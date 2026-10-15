# Generate/run the browser code privately; Playwright echoes executed source.
$ErrorActionPreference='Stop'
$runtime=docker inspect energything-preview --format '{{json .Config.Env}}' | ConvertFrom-Json
$user=($runtime | Where-Object {$_.StartsWith('ENERGYTHING_USERNAME=')}) -replace '^ENERGYTHING_USERNAME=',''
$password=($runtime | Where-Object {$_.StartsWith('ENERGYTHING_PASSWORD=')}) -replace '^ENERGYTHING_PASSWORD=',''
$credentialJSON=@{username=$user;password=$password}|ConvertTo-Json -Compress
$script=(Get-Content scripts/task05-preview-browser.js -Raw).Replace('__PRIVATE_AUTH__',$credentialJSON)
$path=[IO.Path]::GetFullPath('.cache/task05-private/preview-browser.js')
$output=[IO.Path]::GetFullPath('.cache/task05-private/preview-browser-output.txt')
if(!$path.StartsWith('D:\projects\energything\.cache\task05-private\') -or !$output.StartsWith('D:\projects\energything\.cache\task05-private\')){throw 'Private browser path mismatch'}
try {
 [IO.File]::WriteAllText($path,$script)
 & playwright-cli -s=energything-task05 run-code --filename=$path *> $output
 $passed=Select-String -LiteralPath $output -SimpleMatch '"previewBuildVerified":true,"changedUIVerified":true,"octopusBrowsing":true' -Quiet
 if(!$passed){throw 'Private preview browser verification failed; output suppressed to protect credentials'}
 Write-Output 'Private preview browser build/changed UI/Octopus browsing checks passed'
}finally{
 Remove-Item -LiteralPath $path,$output -ErrorAction SilentlyContinue
 $script=$null;$credentialJSON=$null;$runtime=$null;$user=$null;$password=$null
}
