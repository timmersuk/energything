$ErrorActionPreference='Stop'
$runtime=docker inspect energything-preview --format '{{json .Config.Env}}' | ConvertFrom-Json
$user=($runtime|Where-Object {$_.StartsWith('ENERGYTHING_USERNAME=')}) -replace '^ENERGYTHING_USERNAME=',''
$password=($runtime|Where-Object {$_.StartsWith('ENERGYTHING_PASSWORD=')}) -replace '^ENERGYTHING_PASSWORD=',''
$auth=@{username=$user;password=$password}|ConvertTo-Json -Compress
$script=(Get-Content scripts/task06-preview-browser.js -Raw).Replace('__PRIVATE_AUTH__',$auth)
$dir=[IO.Path]::GetFullPath('.cache/task06-private');[IO.Directory]::CreateDirectory($dir)|Out-Null
$path=Join-Path $dir 'preview-browser.js';$output=Join-Path $dir 'preview-browser-output.txt'
try{
 [IO.File]::WriteAllText($path,$script)
 & playwright-cli -s=energything-task06 run-code --filename=$path *> $output
 if(!(Select-String -LiteralPath $output -SimpleMatch '"previewBuildVerified":true,"retainedBrowsingVerified":true' -Quiet)){throw 'Private preview browser verification failed; output withheld'}
 Write-Output 'Private preview actual build, status and retained record browsing passed'
}finally{Remove-Item -LiteralPath $path,$output -ErrorAction SilentlyContinue;$script=$null;$auth=$null;$runtime=$null}
