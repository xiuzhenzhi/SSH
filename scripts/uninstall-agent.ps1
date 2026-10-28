#Requires -Version 5.1
#Requires -RunAsAdministrator
$ErrorActionPreference = 'Stop'
$exe = Join-Path $env:ProgramFiles 'SSHGuardAgent\sshguard-agent.exe'
$config = Join-Path $env:ProgramData 'SSHGuard\agent.json'
if (Get-Service SSHGuardAgent -ErrorAction SilentlyContinue) {
    $installed = Get-CimInstance Win32_Service -Filter "Name='SSHGuardAgent'"
    $quotedExe = if ($exe.Contains(' ')) { '"{0}"' -f $exe } else { $exe }
    $quotedConfig = if ($config.Contains(' ')) { '"{0}"' -f $config } else { $config }
    if ($installed.PathName -ne ('{0} -config {1}' -f $quotedExe, $quotedConfig)) { throw 'Existing service belongs to a different installation.' }
    Stop-Service SSHGuardAgent
    (Get-Service SSHGuardAgent).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(30))
    & $exe -service uninstall -config $config
    if ($LASTEXITCODE -ne 0) { throw 'Service removal failed; executable and config retained.' }
}
if (Test-Path -LiteralPath $exe) { Remove-Item -LiteralPath $exe }
Write-Output 'Service removed. Config, event source and historical logs retained; revoke the credential in the dashboard.'
