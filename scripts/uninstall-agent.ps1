#Requires -Version 5.1
#Requires -RunAsAdministrator
$ErrorActionPreference = 'Stop'
$exe = Join-Path $env:ProgramFiles 'SSHGuardAgent\sshguard-agent.exe'
$config = Join-Path $env:ProgramData 'SSHGuard\agent.json'
$installLock = $null
$installDir = Split-Path -Parent $exe
foreach ($path in @($installDir,$exe)) {
    if ((Get-Item -LiteralPath $path -Force -ErrorAction SilentlyContinue).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Refusing reparse point: $path" }
}
if (Test-Path -LiteralPath $installDir) {
    $lockPath = Join-Path $installDir 'install.lock'
    if ((Get-Item -LiteralPath $lockPath -Force -ErrorAction SilentlyContinue).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Refusing reparse point lock.' }
    $installLock = [IO.File]::Open($lockPath, 'OpenOrCreate', 'ReadWrite', 'None')
}
try {
$runner = Join-Path $env:ProgramFiles 'SSHGuardAgent\poll-update.ps1'
$task = Get-ScheduledTask -TaskName SSHGuardAgentUpdate -ErrorAction SilentlyContinue
if ($task) {
    $shell = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    $arguments = '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "{0}"' -f $runner
    if ($task.Actions.Count -ne 1 -or $task.Actions[0].Execute -ne $shell -or $task.Actions[0].Arguments -ne $arguments) { throw 'Updater task belongs to a different installation.' }
    Disable-ScheduledTask -TaskName SSHGuardAgentUpdate | Out-Null
    if ($task.State -eq 'Running') { throw 'Updater is running; wait for it to finish and rerun uninstall.' }
    Unregister-ScheduledTask -TaskName SSHGuardAgentUpdate -Confirm:$false
}
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
foreach ($path in @($runner,(Join-Path $env:ProgramFiles 'SSHGuardAgent\update-agent.ps1'),(Join-Path $env:ProgramFiles 'SSHGuardAgent\uninstall-agent.ps1'))) {
    if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path }
}
Write-Output 'Service removed. Config, event source and historical logs retained; revoke the credential in the dashboard.'

} finally { if ($installLock) { $installLock.Dispose() } }
