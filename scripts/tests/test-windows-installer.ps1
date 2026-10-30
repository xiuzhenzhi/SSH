# Test only the installer ACL helpers, using a new workspace directory. No service is installed.
$ErrorActionPreference='Stop'
$root=(Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$errors=$null; $tokens=$null
$ast=[System.Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot '../install-agent.ps1'),[ref]$tokens,[ref]$errors)
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
foreach ($scriptName in @('install-agent.ps1','uninstall-agent.ps1')) {
    $parseErrors=$null; $parseTokens=$null
    $scriptAst=[System.Management.Automation.Language.Parser]::ParseFile((Join-Path (Join-Path $PSScriptRoot "..") $scriptName),[ref]$parseTokens,[ref]$parseErrors)
    if($parseErrors){throw $parseErrors}
    $taskCheck=$scriptAst.Find({param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Test-FixedUpdaterTask'},$true)
    if(-not $taskCheck){throw 'Missing updater task ownership check'}
    . ([ScriptBlock]::Create($taskCheck.Extent.Text))
    $testShell='C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe'
    $testRunner='C:\Program Files\SSHGuardAgent\poll-update.ps1'
    foreach ($testArguments in @(
        ('-NoProfile -NonInteractive -File "{0}"' -f $testRunner)
    )) {
        $testTask=[PSCustomObject]@{Actions=@([PSCustomObject]@{Execute=$testShell;Arguments=$testArguments})}
        if(-not (Test-FixedUpdaterTask $testTask $testShell $testRunner)){throw 'Owned current updater was rejected'}
        if(Test-FixedUpdaterTask $testTask $testShell 'C:\unrelated\poll-update.ps1'){throw 'Unrelated updater path was accepted'}
        $testTask.Actions[0].Arguments = ('-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "{0}"' -f $testRunner)
        if(Test-FixedUpdaterTask $testTask $testShell $testRunner){throw 'Obsolete bypass task accepted'}
        $testTask.Actions[0].Arguments+=' -Command evil'
        if(Test-FixedUpdaterTask $testTask $testShell $testRunner){throw 'Unexpected updater command was accepted'}
    }
}
Write-Output 'Windows installer checks passed: protected ACLs, current updater ownership and command tampering rejection.'
