# Run after building energything:local with task05a-growatt-aggregates.
# Runtime credentials stay in memory. Never print resolved Compose configuration.
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'));Set-Location $root
$container='energything-preview';$data=Join-Path $root '.cache/preview-data'
$mount=docker inspect $container --format '{{json .Mounts}}' | ConvertFrom-Json
$labels=docker inspect $container --format '{{json .Config.Labels}}' | ConvertFrom-Json
if(@($mount).Count -ne 1 -or $mount[0].Destination -ne '/data' -or $mount[0].Source -ne $data -or $labels.'com.docker.compose.project' -ne 'energything'){throw 'Original preview setup mismatch'}
$runtime=docker inspect $container --format '{{json .Config.Env}}' | ConvertFrom-Json
$user=($runtime|Where-Object {$_.StartsWith('ENERGYTHING_USERNAME=')}) -replace '^ENERGYTHING_USERNAME=',''
$password=($runtime|Where-Object {$_.StartsWith('ENERGYTHING_PASSWORD=')}) -replace '^ENERGYTHING_PASSWORD=',''
$basic=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($user+':'+$password))
$headers=@{Authorization="Basic $basic"}
$before=Invoke-RestMethod 'http://localhost:8080/api/v1/status' -Headers $headers
$compose=@('compose','-p','energything','-f','docker-compose.yml','-f','.cache/preview-compose.override.json')
$env:ENERGYTHING_IMAGE='energything:local';$env:ENERGYTHING_OCTOPUS_ONCE='false'
& docker @compose config --quiet;if($LASTEXITCODE -ne 0){throw 'Saved Compose configuration invalid'}
& docker @compose stop -t 15 energything | Out-Null;if($LASTEXITCODE -ne 0){throw 'Clean stop failed'}
if((docker inspect $container --format '{{.State.Running}}') -ne 'false'){throw 'Preview is still running'}
$backup=Join-Path $root ('.cache/task05a-preview-backup-'+[guid]::NewGuid().ToString('N'))
Copy-Item -LiteralPath $data -Destination $backup -Recurse
$collectionPassed=$false
try {
 $env:GROWATT_API_TOKEN=($runtime|Where-Object {$_.StartsWith('GROWATT_API_TOKEN=')}) -replace '^GROWATT_API_TOKEN=',''
 & go run ./cmd/growatt-aggregate -data-dir $data -discovery .cache/task05-private/preview-populate-discovery.json -day 2026-01-15
 $collectionPassed=$LASTEXITCODE -eq 0
} finally {
 Remove-Item Env:GROWATT_API_TOKEN -ErrorAction SilentlyContinue
 & docker @compose up -d --no-deps --force-recreate energything | Out-Null
 if($LASTEXITCODE -ne 0){throw 'Preview recreation failed; original mount and backup retained'}
}
$after=$null
for($i=0;$i -lt 30;$i++){try{$health=Invoke-WebRequest 'http://localhost:8080/healthz';$after=Invoke-RestMethod 'http://localhost:8080/api/v1/status' -Headers $headers;break}catch{Start-Sleep -Seconds 1}}
if($null -eq $after){throw 'Preview health unavailable'}
$mountAfter=docker inspect $container --format '{{json .Mounts}}' | ConvertFrom-Json
$actualImage=docker inspect $container --format '{{.Image}}';$expectedImage=docker image inspect energything:local --format '{{.Id}}'
$preserved=$true
foreach($old in $before.providers.sources){$new=@($after.providers.sources|Where-Object {$_.identity_id -eq $old.identity_id});if($new.Count -ne 1 -or $new[0].retained_count -ne $old.retained_count){$preserved=$false}}
$aggregates=Invoke-RestMethod 'http://localhost:8080/api/v1/growatt/aggregates?from=2026-01-15&to=2026-01-15' -Headers $headers
$summary=[ordered]@{build_verified=($after.build_id -eq 'task05a-growatt-aggregates');health_ok=($health.StatusCode -eq 200);original_mount=($mountAfter[0].Source -eq $data);actual_image=($actualImage -eq $expectedImage);prior_counts_preserved=$preserved;clean_backup=$true;older_daily_aggregate_visible=($aggregates.records.Count -eq 1);one_off_collection=$collectionPassed}
$summary | ConvertTo-Json | Set-Content .cache/task05a-preview-summary.json
$summary | ConvertTo-Json -Compress | Write-Output
if($summary.Values -contains $false){throw 'Preview verification failed'}
