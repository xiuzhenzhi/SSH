# Test only the installer ACL helpers, using a new workspace directory. No service is installed.
$ErrorActionPreference='Stop'
$root=(Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$errors=$null; $tokens=$null
$ast=[System.Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'install-agent.ps1'),[ref]$tokens,[ref]$errors)
if($errors){throw $errors}
$functions=$ast.FindAll({param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst]},$true)
foreach($fn in $functions){ . ([ScriptBlock]::Create($fn.Extent.Text)) }
$target=Join-Path $root ('.local\acl-test-'+[Guid]::NewGuid().ToString())
Protect-Directory $target
$file=Join-Path $target 'agent.json'
[IO.File]::WriteAllText($file,'{"test":true}')
Protect-File $file
foreach($path in @($target,$file)) {
    $acl=Get-Acl -LiteralPath $path
    $rules=$acl.GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier])
    $sids=@($rules | ForEach-Object {$_.IdentityReference.Value})
    if($sids.Count -ne 3 -or ($sids | Where-Object {$_ -notin @('S-1-5-32-544','S-1-5-18','S-1-5-19')})){throw 'Unexpected ACL identities'}
    $serviceRule=$rules | Where-Object {$_.IdentityReference.Value -eq 'S-1-5-19'}
    if($serviceRule.FileSystemRights -band [Security.AccessControl.FileSystemRights]::Write){throw 'LocalService can write protected files'}
}
Write-Output 'Windows installer ACL test passed: only Administrators/SYSTEM full control; LocalService read/execute.'
