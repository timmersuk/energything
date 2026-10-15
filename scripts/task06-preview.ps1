param([string]$BuildID='task06-scheduling')
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'));Set-Location $root
$container='energything-preview';$data=Join-Path $root '.cache/preview-data'
$mount=docker inspect $container --format '{{json .Mounts}}' | ConvertFrom-Json
$labels=docker inspect $container --format '{{json .Config.Labels}}' | ConvertFrom-Json
if(@($mount).Count -ne 1 -or $mount[0].Destination -ne '/data' -or $mount[0].Source -ne $data -or $labels.'com.docker.compose.project' -ne 'energything'){throw 'Original preview setup mismatch'}
$runtime=docker inspect $container --format '{{json .Config.Env}}' | ConvertFrom-Json
$user=($runtime|Where-Object {$_.StartsWith('ENERGYTHING_USERNAME=')}) -replace '^ENERGYTHING_USERNAME=',''
$password=($runtime|Where-Object {$_.StartsWith('ENERGYTHING_PASSWORD=')}) -replace '^ENERGYTHING_PASSWORD=',''
$headers=@{Authorization='Basic '+[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($user+':'+$password))}
$before=Invoke-RestMethod 'http://localhost:8080/api/v1/status' -Headers $headers -NoProxy
$compose=@('compose','-p','energything','-f','docker-compose.yml','-f','.cache/preview-compose.override.json')
$env:ENERGYTHING_IMAGE='energything:local'
& docker @compose config --quiet;if($LASTEXITCODE -ne 0){throw 'Saved Compose configuration invalid'}
& docker @compose stop -t 15 energything | Out-Null;if($LASTEXITCODE -ne 0){throw 'Clean stop failed'}
if((docker inspect $container --format '{{.State.Running}}') -ne 'false'){throw 'Preview is still running'}
$backup=Join-Path $root ('.cache/task06-preview-backup-'+[guid]::NewGuid().ToString('N'))
Copy-Item -LiteralPath $data -Destination $backup -Recurse
# Starting this image intentionally enables automatic recent collection and bounded
# history exploration with the configured selected identities. No obsolete gate.
& docker @compose up -d --no-deps --force-recreate energything | Out-Null
if($LASTEXITCODE -ne 0){throw 'Preview recreation failed; original mount and backup preserved'}
$after=$null
for($n=0;$n -lt 40;$n++){try{$health=Invoke-WebRequest 'http://localhost:8080/healthz' -NoProxy;$after=Invoke-RestMethod 'http://localhost:8080/api/v1/status' -Headers $headers -NoProxy;break}catch{Start-Sleep -Milliseconds 250}}
if($null -eq $after){throw 'Updated preview unavailable'}
$mountAfter=docker inspect $container --format '{{json .Mounts}}' | ConvertFrom-Json
$actual=docker inspect $container --format '{{.Image}}';$expected=docker image inspect energything:local --format '{{.Id}}'
$preserved=$true
foreach($old in $before.providers.sources){$new=@($after.providers.sources|Where-Object {$_.identity_id -eq $old.identity_id});if($new.Count -ne 1 -or $new[0].retained_count -lt $old.retained_count){$preserved=$false}}
$frontiersPreserved=$true
foreach($old in @($before.schedules|Where-Object policy -eq 'backfill')){
 $new=@($after.schedules|Where-Object {$_.identity_id -eq $old.identity_id -and $_.kind -eq $old.kind -and $_.policy -eq 'backfill'})
 if($new.Count -ne 1 -or [DateTimeOffset]$new[0].frontier -gt [DateTimeOffset]$old.frontier){$frontiersPreserved=$false}
}
$growattIdentityIds=@(($after.providers|Where-Object source -eq 'growatt').sources.identity_id)
$growattPlans=@($after.schedules|Where-Object {$_.policy -eq 'backfill' -and $_.identity_id -in $growattIdentityIds})
$continuous=($growattPlans.Count -gt 0 -and @($growattPlans|Where-Object search_policy -ne 'calendar').Count -eq 0)
$summary=[ordered]@{build_verified=($after.build_id -eq $BuildID);health_ok=($health.StatusCode -eq 200);original_mount=($mountAfter[0].Source -eq $data);actual_image=($actual -eq $expected);prior_counts_preserved=$preserved;backfill_frontiers_preserved=$frontiersPreserved;growatt_history_continues=$continuous;clean_full_backup=$true;automatic_collection_enabled=$true}
$summary|ConvertTo-Json|Set-Content .cache/task06-preview-summary.json
$summary|ConvertTo-Json -Compress|Write-Output
if($summary.Values -contains $false){throw 'Preview verification failed'}
