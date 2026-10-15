$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'));Set-Location $root
# This named container and every fixture directory belong only to this script.
& docker rm -f energything-task06-browser 2>$null | Out-Null
$fixture=go run ./scripts/task06-fixtures
if($LASTEXITCODE -ne 0){throw 'Synthetic seed failed'}
$taskOldGOOS=$env:GOOS;$taskOldGOARCH=$env:GOARCH;$taskOldCGO=$env:CGO_ENABLED
try{$env:GOOS='linux';$env:GOARCH='amd64';$env:CGO_ENABLED='0';go build -o bin/linux/amd64/task06-fixtures ./scripts/task06-fixtures;if($LASTEXITCODE -ne 0){throw 'Synthetic helper build failed'}}finally{$env:GOOS=$taskOldGOOS;$env:GOARCH=$taskOldGOARCH;$env:CGO_ENABLED=$taskOldCGO}
try{
 & docker run -d --name energything-task06-browser -p 127.0.0.1:8087:8080 --add-host api.octopus.energy:127.0.0.1 --add-host openapi.growatt.com:127.0.0.1 --mount "type=bind,source=$fixture,target=/data" -e ENERGYTHING_USERNAME=synthetic -e ENERGYTHING_PASSWORD=synthetic -e OCTOPUS_API_KEY=synthetic -e OCTOPUS_ACCOUNT_NUMBER=synthetic -e GROWATT_API_TOKEN=synthetic energything:local | Out-Null
 if($LASTEXITCODE -ne 0){throw 'Synthetic image failed to start'}
 $auth=@{Authorization='Basic '+[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('synthetic:synthetic'))}
 $failed=$false
 for($n=0;$n -lt 80;$n++){try{$status=Invoke-RestMethod 'http://127.0.0.1:8087/api/v1/status' -Headers $auth -NoProxy;if(@(($status.providers|Where-Object source -eq 'octopus').streams|Where-Object state -eq 'failed').Count -eq 2){$failed=$true;break}}catch{};Start-Sleep -Milliseconds 250}
 if(!$failed){throw 'Injected discovery outage not durable'}
 # All queue mutations while Linux owns the live WAL also run inside Linux.
 & docker cp bin/linux/amd64/task06-fixtures energything-task06-browser:/tmp/task06-fixtures
 & docker exec --user 0 energything-task06-browser chmod 755 /tmp/task06-fixtures
 & docker exec energything-task06-browser /tmp/task06-fixtures -reselect-data-dir /data
 if($LASTEXITCODE -ne 0){throw 'Synthetic selected context restoration failed'}
 & playwright-cli -s=energything-task06 run-code --filename=scripts/task06-browser.js
}finally{& docker rm -f energything-task06-browser 2>$null | Out-Null}
