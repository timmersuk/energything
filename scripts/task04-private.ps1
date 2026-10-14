# Private live verification. No .env reads, raw account output or credential logs.
$ErrorActionPreference='Stop'
$taskRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$privateRoot=Join-Path $taskRoot ('.cache/task04-private-'+[guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory $privateRoot | Out-Null
$runtime=docker inspect energything-preview --format '{{json .Config.Env}}' | ConvertFrom-Json
$allowed=@('OCTOPUS_API_KEY','OCTOPUS_ACCOUNT_NUMBER','OCTOPUS_IMPORT_MPAN','OCTOPUS_IMPORT_SERIAL','OCTOPUS_EXPORT_MPAN','OCTOPUS_EXPORT_SERIAL')
$previous=@{}
$container='energything-task04-private-'+[guid]::NewGuid().ToString('N')
try {
 foreach($name in $allowed){$previous[$name]=[Environment]::GetEnvironmentVariable($name,'Process')}
 foreach($entry in $runtime){$parts=$entry.Split('=',2);if($allowed -contains $parts[0]){[Environment]::SetEnvironmentVariable($parts[0],$parts[1],'Process')}}
 if(!$env:OCTOPUS_API_KEY -or !$env:OCTOPUS_ACCOUNT_NUMBER){throw 'Live gate unavailable: runtime credentials missing'}
 $taskDockerArgs=@('run','-d','--name',$container,'-p','127.0.0.1:8086:8080','--mount',"type=bind,source=$privateRoot,target=/data",'-e','ENERGYTHING_USERNAME=synthetic','-e','ENERGYTHING_PASSWORD=synthetic','-e','ENERGYTHING_OCTOPUS_ONCE=true')
 foreach($name in $allowed){$taskDockerArgs+=@('-e',$name)}
 $taskDockerArgs+='energything:task04'
 $id=& docker @taskDockerArgs
 if($LASTEXITCODE -ne 0){throw 'Private task container start failed'}
 $basic=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('synthetic:synthetic'))
 $status=$null
 for($i=0;$i -lt 60;$i++){
  Start-Sleep -Seconds 1
  try{$status=Invoke-RestMethod 'http://127.0.0.1:8086/api/v1/status' -Headers @{Authorization="Basic $basic"}}catch{continue}
  $provider=$status.providers | Where-Object source -eq 'octopus'
  if($provider.streams.Count -eq 2 -and !($provider.streams.state -contains 'discovering') -and !(($provider.sources | Where-Object selected).last_attempt.outcome -contains 'running') -and (($provider.sources | Where-Object selected).Count -eq 0 -or @($provider.sources | Where-Object { $_.selected -and !$_.last_attempt }).Count -eq 0)){break}
 }
 # Status is already masked. Keep even this evidence private; print gate booleans only.
 $status | ConvertTo-Json -Depth 20 | Set-Content (Join-Path $privateRoot 'masked-status.json')
 $provider=$status.providers | Where-Object source -eq 'octopus'
 $import=$provider.streams | Where-Object stream -eq 'import';$export=$provider.streams | Where-Object stream -eq 'export'
 $importSource=$provider.sources | Where-Object { $_.stream -eq 'import' -and $_.selected }
 $exportSource=$provider.sources | Where-Object { $_.stream -eq 'export' -and $_.selected }
 $summary=[ordered]@{build_verified=($status.build_id -eq 'task04-octopus');import_discovered=(@($provider.sources | Where-Object stream -eq 'import').Count -gt 0);export_discovered=(@($provider.sources | Where-Object stream -eq 'export').Count -gt 0);import_selected=($import.state -eq 'selected');export_selected=($export.state -eq 'selected');import_ingested=($importSource.last_attempt.outcome -in @('success','empty'));export_ingested=($exportSource.last_attempt.outcome -in @('success','empty'));browsing_responsive=($null -ne $status)}
 $summary | ConvertTo-Json | Set-Content (Join-Path $privateRoot 'gate-summary.json')
 $summary | ConvertTo-Json -Compress | Write-Output
 Write-Output "Private evidence directory: $privateRoot"
}finally{
 if($id){docker stop -t 15 $container | Out-Null; docker rm $container | Out-Null}
 foreach($name in $allowed){[Environment]::SetEnvironmentVariable($name,$previous[$name],'Process')}
}
