$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'));Set-Location $root
foreach($arch in @('amd64','arm64')) {
 $image=if($arch -eq 'amd64'){'energything:local'}else{'energything:task06-arm64'}
 $name='energything-task06-'+$arch+'-'+[guid]::NewGuid().ToString('N')
 $dir=Join-Path $root ('.cache/task06-runtime-'+$arch+'-'+[guid]::NewGuid().ToString('N'))
 [IO.Directory]::CreateDirectory($dir)|Out-Null
 try {
  & docker run -d --name $name --platform "linux/$arch" -p '127.0.0.1::8080' --mount "type=bind,source=$dir,target=/data" -e ENERGYTHING_USERNAME=synthetic -e ENERGYTHING_PASSWORD=synthetic $image | Out-Null
  if($LASTEXITCODE -ne 0){throw 'Runtime start failed'}
  $base='http://'+(docker port $name 8080/tcp).Trim()
  $auth=@{Authorization='Basic '+[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('synthetic:synthetic'))}
  for($n=0;$n -lt 40;$n++){try{$status=Invoke-RestMethod "$base/api/v1/status" -Headers $auth -NoProxy;break}catch{Start-Sleep -Milliseconds 200}}
  if($status.build_id -ne 'task06-scheduling'){throw 'Wrong runtime build'}
  if((docker exec $name id -u).Trim() -ne '10001'){throw 'Wrong UID'}
  $cli=& docker exec $name energything-enqueue 2>&1
  if($LASTEXITCODE -ne 1 -or !(($cli|Out-String).Contains('require existing data-dir'))){throw 'Repair CLI runtime failed'}
  & docker exec $name sh -c 'test -s /etc/ssl/certs/ca-certificates.crt && test -f /usr/share/zoneinfo/Europe/London'
  if($LASTEXITCODE -ne 0){throw 'Missing CA/timezone'}
  & docker stop -t 12 $name | Out-Null
  if((docker inspect $name --format '{{.State.ExitCode}}').Trim() -ne '0'){throw 'Unclean shutdown'}
  & docker start $name | Out-Null
  $base='http://'+(docker port $name 8080/tcp).Trim()
  $restarted=$false
  for($n=0;$n -lt 40;$n++){try{$again=Invoke-RestMethod "$base/api/v1/status" -Headers $auth -NoProxy;if($again.build_id -eq 'task06-scheduling'){$restarted=$true;break}}catch{};Start-Sleep -Milliseconds 200}
  if(!$restarted -or !(Test-Path (Join-Path $dir 'energything.sqlite'))){throw 'Database restart failed'}
  Write-Output "$arch actual image: non-root, CA/timezone, schema migration, status, clean shutdown and persisted database restart passed"
 }finally{& docker rm -f $name 2>$null | Out-Null}
}
