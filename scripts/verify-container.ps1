param([string]$Image = 'energything:local', [string]$Platform = 'linux/amd64')
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$probeRoot = Join-Path $root '.cache/container-verification'
New-Item -ItemType Directory -Force $probeRoot | Out-Null
$data = Join-Path $probeRoot ([Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory $data | Out-Null
$name = 'energything-task01-' + [Guid]::NewGuid().ToString('N')
function Assert-True($condition, $message) { if (!$condition) { throw $message } }
function Startup-Failure([string[]]$Extra, [string]$Expected) {
    $output = & docker run --rm --platform $Platform @Extra $Image 2>&1
    $code = $LASTEXITCODE
    Assert-True ($code -ne 0) 'Expected nonzero startup exit'
    Assert-True (($output | Out-String).Contains($Expected)) "Expected actionable error: $Expected"
    Assert-True (!(($output | Out-String).Contains('synthetic-password'))) 'Secret leaked'
}
Startup-Failure @() 'ENERGYTHING_USERNAME and ENERGYTHING_PASSWORD are required'
Startup-Failure @('-e','ENERGYTHING_USERNAME=synthetic-user') 'ENERGYTHING_USERNAME and ENERGYTHING_PASSWORD are required'
Startup-Failure @('-e','ENERGYTHING_PASSWORD=synthetic-password') 'ENERGYTHING_USERNAME and ENERGYTHING_PASSWORD are required'
$authArgs = @('-e','ENERGYTHING_USERNAME=synthetic-user','-e','ENERGYTHING_PASSWORD=synthetic-password')
Startup-Failure ($authArgs + @('-e','ENERGYTHING_TIMEZONE=invalid/synthetic')) 'ENERGYTHING_TIMEZONE'
Startup-Failure ($authArgs + @('-e','ENERGYTHING_HTTP_ADDR=bad-address')) 'ENERGYTHING_HTTP_ADDR'
Startup-Failure ($authArgs + @('--mount',"type=bind,source=$data,target=/data,readonly")) 'must be writable'
Write-Output "$Platform startup failures: missing credentials, invalid timezone/address, read-only bind mount passed"
try {
    & docker run -d --name $name --platform $Platform -p '127.0.0.1::8080' --mount "type=bind,source=$data,target=/data" @authArgs -e 'OCTOPUS_API_KEY=synthetic-provider-key' $Image | Out-Null
    Assert-True ($LASTEXITCODE -eq 0) 'Container failed to start'
    $binding = (& docker port $name '8080/tcp').Trim()
    $base = 'http://' + $binding
    $ready = $false
    for ($i=0; $i -lt 50; $i++) {
        try { $response=Invoke-WebRequest "$base/healthz" -SkipHttpErrorCheck -NoProxy; if ($response.StatusCode -eq 200) { $ready=$true;break } } catch {}
        Start-Sleep -Milliseconds 100
    }
    Assert-True $ready 'Liveness did not become ready'
    Assert-True ($response.Content -eq "ok`n") 'Liveness changed or leaked detail'
    foreach ($path in @('/','/static/style.css')) {
        $response=Invoke-WebRequest "$base$path" -SkipHttpErrorCheck -NoProxy
        Assert-True ($response.StatusCode -eq 401) "Unprotected $path"
        Assert-True ($response.Headers.ContainsKey('WWW-Authenticate')) 'Missing Basic Auth challenge'
        $wrong=@{Authorization='Basic '+[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('synthetic-user:wrong'))}
        $response=Invoke-WebRequest "$base$path" -Headers $wrong -SkipHttpErrorCheck -NoProxy
        Assert-True ($response.StatusCode -eq 401) "Wrong credentials accepted for $path"
    }
    $auth=@{Authorization='Basic '+[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('synthetic-user:synthetic-password'))}
    $response=Invoke-WebRequest "$base/" -Headers $auth -SkipHttpErrorCheck -NoProxy
    Assert-True ($response.StatusCode -eq 200) 'Authenticated page failed'
    Assert-True ($response.Content.Contains('credentials are incomplete')) 'Missing partial source card'
    Assert-True ($response.Content.Contains('Unconfigured')) 'Missing empty source card'
    Assert-True (!$response.Content.Contains('synthetic-provider-key')) 'Provider secret leaked'
    $response=Invoke-WebRequest "$base/static/style.css" -Headers $auth -SkipHttpErrorCheck -NoProxy
    Assert-True ($response.StatusCode -eq 200 -and $response.Content.Contains(':root')) 'Embedded CSS failed'
    $response=Invoke-WebRequest "$base/api/v1/status" -Headers $auth -SkipHttpErrorCheck -NoProxy
    Assert-True ($response.StatusCode -eq 404) 'Unimplemented API did not return 404'
    $uid=(& docker exec $name id -u).Trim()
    Assert-True ($uid -eq '10001') 'Container is not UID 10001'
    & docker exec $name sh -c 'test -s /etc/ssl/certs/ca-certificates.crt && test -f /usr/share/zoneinfo/Europe/London'
    Assert-True ($LASTEXITCODE -eq 0) 'Missing CA bundle/timezone data'
    $bindFailure = & docker exec $name /usr/local/bin/energything 2>&1
    Assert-True ($LASTEXITCODE -ne 0) 'Second process accepted occupied port'
    Assert-True (($bindFailure | Out-String).Contains('HTTP listen failed')) 'Missing bind failure message'
    Assert-True (!(($bindFailure | Out-String).Contains('synthetic-password'))) 'Bind failure leaked credentials'
    & docker stop -t 12 $name | Out-Null
    $exitCode=(& docker inspect --format '{{.State.ExitCode}}' $name).Trim()
    Assert-True ($exitCode -eq '0') "SIGTERM exit code $exitCode"
    Assert-True (Test-Path (Join-Path $data 'energything.sqlite')) 'Bind mount database missing'
    Write-Output "$Platform non-root writable bind mount, embedded page/CSS, auth, empty/error cards, liveness, 404, CA/timezone files, occupied-port exit, SIGTERM exit 0 passed"
} finally {
    & docker rm -f $name 2>$null | Out-Null
}
