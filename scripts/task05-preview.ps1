# Safe standard preview replacement; never prints resolved config or runtime values.
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
Set-Location $root
$container='energything-preview'
$mount=docker inspect $container --format '{{json .Mounts}}' | ConvertFrom-Json
$data=Join-Path $root '.cache/preview-data'
if(@($mount).Count -ne 1 -or $mount[0].Destination -ne '/data' -or $mount[0].Source -ne $data){throw 'Original preview mount mismatch'}
$runtime=docker inspect $container --format '{{json .Config.Env}}' | ConvertFrom-Json
$user=($runtime | Where-Object {$_.StartsWith('ENERGYTHING_USERNAME=')}) -replace '^ENERGYTHING_USERNAME=',''
$password=($runtime | Where-Object {$_.StartsWith('ENERGYTHING_PASSWORD=')}) -replace '^ENERGYTHING_PASSWORD=',''
$basic=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($user+':'+$password))
$before=Invoke-RestMethod 'http://localhost:8080/api/v1/status' -Headers @{Authorization="Basic $basic"}
$env:ENERGYTHING_IMAGE='energything:local';$env:ENERGYTHING_OCTOPUS_ONCE='false'
$compose=@('compose','-p','energything','-f','docker-compose.yml','-f','.cache/preview-compose.override.json')
& docker @compose config --quiet
if($LASTEXITCODE -ne 0){throw 'Saved Compose configuration invalid'}
& docker @compose stop -t 15 energything | Out-Null
if($LASTEXITCODE -ne 0){throw 'Clean preview stop failed'}
$backup=Join-Path $root ('.cache/task05-preview-backup-'+[guid]::NewGuid().ToString('N'))
Copy-Item -LiteralPath $data -Destination $backup -Recurse
& docker @compose up -d --no-deps --force-recreate energything | Out-Null
if($LASTEXITCODE -ne 0){throw 'Preview recreation failed; original mount and private backup preserved'}
for($i=0;$i -lt 30;$i++){try{$health=Invoke-WebRequest 'http://localhost:8080/healthz';$after=Invoke-RestMethod 'http://localhost:8080/api/v1/status' -Headers @{Authorization="Basic $basic"};break}catch{Start-Sleep -Seconds 1}}
$mountAfter=docker inspect $container --format '{{json .Mounts}}' | ConvertFrom-Json
$oldCount=($before.providers.sources.retained_count | Measure-Object -Sum).Sum
$newCount=($after.providers.sources.retained_count | Measure-Object -Sum).Sum
$summary=[ordered]@{build_verified=($after.build_id -eq 'task05-growatt');health_ok=($health.StatusCode -eq 200);original_mount=($mountAfter[0].Source -eq $data);retained_data_preserved=($oldCount -eq $newCount);one_shot_disabled=$true;clean_backup=$true}
$summary | ConvertTo-Json | Set-Content .cache/task05-preview-summary.json
$summary | ConvertTo-Json -Compress | Write-Output
if($summary.Values -contains $false){throw 'Preview verification failed'}
