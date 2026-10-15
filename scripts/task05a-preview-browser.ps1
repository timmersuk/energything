$ErrorActionPreference='Stop'
$runtime=docker inspect energything-preview --format '{{json .Config.Env}}' | ConvertFrom-Json
$user=($runtime|Where-Object {$_.StartsWith('ENERGYTHING_USERNAME=')}) -replace '^ENERGYTHING_USERNAME=',''
$password=($runtime|Where-Object {$_.StartsWith('ENERGYTHING_PASSWORD=')}) -replace '^ENERGYTHING_PASSWORD=',''
$auth=@{username=$user;password=$password}|ConvertTo-Json -Compress
$script=(Get-Content scripts/task05a-preview-browser.js -Raw).Replace('__PRIVATE_AUTH__',$auth)
$dir=[IO.Path]::GetFullPath('.cache/task05a-private');[IO.Directory]::CreateDirectory($dir)|Out-Null
$path=Join-Path $dir 'preview-browser.js';$output=Join-Path $dir 'preview-browser-output.txt'
try{
 [IO.File]::WriteAllText($path,$script)
 & playwright-cli -s=energything-task05a run-code --filename=$path *> $output
 if(!(Select-String -LiteralPath $output -SimpleMatch '"previewBuildVerified":true,"olderAggregateVerified":true,"existingBrowsingVerified":true' -Quiet)){throw 'Private browser verification failed; output kept private'}
 Write-Output 'Private preview build/older aggregate/existing browsing verified'
}finally{Remove-Item -LiteralPath $path,$output -ErrorAction SilentlyContinue;$script=$null;$auth=$null;$runtime=$null}
