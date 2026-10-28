#Requires -Version 5.1
#Requires -RunAsAdministrator
param(
    [Parameter(Mandatory=$true)][string]$Endpoint,
    [ValidatePattern('^[a-f0-9]{64}$')][string]$EnrollmentToken,
    [switch]$Update
)
$ErrorActionPreference = 'Stop'
if ($Update -and $EnrollmentToken) { throw 'Choose Update or EnrollmentToken, not both.' }
if (-not $Update -and -not $EnrollmentToken) { throw 'EnrollmentToken is required for installation.' }
$Endpoint = $Endpoint.TrimEnd('/')
$origin = [Uri]$Endpoint
if ($origin.Scheme -ne 'https' -or $origin.UserInfo -or $origin.Query -or $origin.Fragment -or $origin.AbsolutePath -ne '/') { throw 'Endpoint must be an HTTPS origin.' }
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$osArchitecture = $env:PROCESSOR_ARCHITEW6432
if (-not $osArchitecture) { $osArchitecture = $env:PROCESSOR_ARCHITECTURE }
switch ($osArchitecture) { 'AMD64' { $arch='amd64' } 'ARM64' { $arch='arm64' } default { throw 'Windows amd64 / arm64 only.' } }
$installDir = Join-Path $env:ProgramFiles 'SSHGuardAgent'
$configDir = Join-Path $env:ProgramData 'SSHGuard'
$exe = Join-Path $installDir 'sshguard-agent.exe'
$config = Join-Path $configDir 'agent.json'

function Assert-NoReparsePoint([string]$Path) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
    if ($item -and ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw "Refusing reparse point: $Path" }
}
function Protect-Directory([string]$Path) {
    Assert-NoReparsePoint $Path
    New-Item -ItemType Directory -Force -Path $Path | Out-Null
    $acl = New-Object Security.AccessControl.DirectorySecurity
    $acl.SetAccessRuleProtection($true, $false)
    $admins = New-Object Security.Principal.SecurityIdentifier('S-1-5-32-544')
    $acl.SetOwner($admins)
    foreach ($sid in @('S-1-5-32-544','S-1-5-18','S-1-5-19')) {
        $rights = if ($sid -eq 'S-1-5-19') { 'ReadAndExecute' } else { 'FullControl' }
        $identity = New-Object Security.Principal.SecurityIdentifier($sid)
        $rule = New-Object Security.AccessControl.FileSystemAccessRule($identity,$rights,'ContainerInherit,ObjectInherit','None','Allow')
        $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $Path -AclObject $acl
}
function Protect-File([string]$Path) {
    $acl = New-Object Security.AccessControl.FileSecurity
    $acl.SetAccessRuleProtection($true,$false)
    $acl.SetOwner((New-Object Security.Principal.SecurityIdentifier('S-1-5-32-544')))
    foreach ($sid in @('S-1-5-32-544','S-1-5-18','S-1-5-19')) {
        $rights = if ($sid -eq 'S-1-5-19') { 'ReadAndExecute' } else { 'FullControl' }
        $identity = New-Object Security.Principal.SecurityIdentifier($sid)
        $acl.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($identity,$rights,'Allow')))
    }
    Set-Acl -LiteralPath $Path -AclObject $acl
}
Protect-Directory $installDir
Protect-Directory $configDir
Assert-NoReparsePoint $exe
Assert-NoReparsePoint $config
$lockPath = Join-Path $installDir 'install.lock'
Assert-NoReparsePoint $lockPath
# Exclusive handle prevents concurrent installers from stopping/replacing each other.
$installLock = [IO.File]::Open($lockPath, 'OpenOrCreate', 'ReadWrite', 'None')
$staging = Join-Path $installDir ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $staging | Out-Null
$download = Join-Path $staging 'sshguard-agent.exe'
$checksum = Join-Path $staging 'agent.sha256'
$service = Get-Service -Name SSHGuardAgent -ErrorAction SilentlyContinue
$wasRunning = $service -and $service.Status -eq 'Running'
$enrolled = $false
$replaced = $false
$backup = Join-Path $staging 'previous.exe'
$keepBackup = $false
try {
    if ($Update -and (-not $service -or -not (Test-Path -LiteralPath $config) -or -not (Test-Path -LiteralPath $exe))) { throw 'Agent not installed; generate an installation command.' }
    Invoke-WebRequest -UseBasicParsing -TimeoutSec 180 -MaximumRedirection 0 -Uri "$Endpoint/downloads/sshguard-agent-windows-$arch.exe" -OutFile $download
    Invoke-WebRequest -UseBasicParsing -TimeoutSec 30 -MaximumRedirection 0 -Uri "$Endpoint/downloads/sshguard-agent-windows-$arch.exe.sha256" -OutFile $checksum
    $expected = ((Get-Content -LiteralPath $checksum -Raw).Trim() -split '\s+')[0]
    if ($expected -notmatch '^[a-f0-9]{64}$' -or (Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash -ne $expected) { throw 'Agent checksum mismatch.' }
    & $download -version
    if ($LASTEXITCODE -ne 0) { throw 'Downloaded Agent cannot run on this machine.' }
    if ($Update) {
        & $download -check -endpoint $Endpoint -config $config
        if ($LASTEXITCODE -ne 0) { throw 'Existing credentials or endpoint check failed; service unchanged.' }
        Copy-Item -LiteralPath $exe -Destination $backup
    }
    if ($service) {
        # Reject a colliding service name before touching the existing service.
        $installed = Get-CimInstance Win32_Service -Filter "Name='SSHGuardAgent'"
        $quotedExe = if ($exe.Contains(' ')) { '"{0}"' -f $exe } else { $exe }
        $quotedConfig = if ($config.Contains(' ')) { '"{0}"' -f $config } else { $config }
        $expectedCommand = '{0} -config {1}' -f $quotedExe, $quotedConfig
        if ($installed.PathName -ne $expectedCommand) { throw 'Existing SSHGuardAgent service belongs to a different installation.' }
        if ($Update -and (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash -eq $expected) {
            Protect-File $config
            if (-not $wasRunning) { Start-Service -Name SSHGuardAgent; (Get-Service SSHGuardAgent).WaitForStatus('Running',[TimeSpan]::FromSeconds(30)) }
            Write-Output 'Already up to date; credentials verified and service running.'
            return
        }
        Stop-Service -Name SSHGuardAgent
        (Get-Service SSHGuardAgent).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(30))
    }
    # Token travels over stdin; only the short-lived install command contains it.
    if (-not $Update) {
        $EnrollmentToken | & $download -enroll -endpoint $Endpoint -config $config
        if ($LASTEXITCODE -ne 0) { throw 'Enrollment failed. Generate a new installation command and retry.' }
        $enrolled = $true
        $EnrollmentToken = $null
    }
    # Replace the file ACL too; do not retain explicit permissions from an older config.
    Protect-File $config
    if ($Update) { $replaced = $true }
    if (Test-Path -LiteralPath $exe) { [IO.File]::Replace($download,$exe,$null) }
    else { Move-Item -LiteralPath $download -Destination $exe }
    Protect-File $exe
    & $exe -service install -config $config
    if ($LASTEXITCODE -ne 0) { throw 'Windows service registration failed; see the preceding error.' }
    Start-Service -Name SSHGuardAgent
    (Get-Service SSHGuardAgent).WaitForStatus('Running',[TimeSpan]::FromSeconds(30))
    Start-Sleep -Seconds 2
    if ((Get-Service SSHGuardAgent).Status -ne 'Running') { throw 'Agent stopped unexpectedly.' }
    & $exe -check -endpoint $Endpoint -config $config
    if ($LASTEXITCODE -ne 0) { throw 'Post-install heartbeat check failed.' }
    $replaced = $false
    Write-Output 'Installed/updated as LocalService. Registration + heartbeat only. Update preserves credentials.'
    Write-Output 'Logs: Windows Event Viewer / Application / SSHGuardAgent.'
} catch {
    if ($replaced -and (Test-Path -LiteralPath $backup)) {
        $keepBackup = $true
        Stop-Service -Name SSHGuardAgent -ErrorAction Continue
        (Get-Service SSHGuardAgent).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(30))
        Copy-Item -LiteralPath $backup -Destination $exe -Force
        Protect-File $exe
        $keepBackup = $false
        Write-Warning 'Update failed; previous Agent binary restored.'
    }
    if ($wasRunning -and -not $enrolled) { Start-Service -Name SSHGuardAgent -ErrorAction Continue }
    throw
} finally {
    # Only delete the exact temporary files created by this invocation.
    $files = @($download,$checksum)
    if (-not $keepBackup) { $files += $backup }
    foreach ($file in $files) { if (Test-Path -LiteralPath $file) { Remove-Item -LiteralPath $file -Force } }
    if (-not $keepBackup -and (Test-Path -LiteralPath $staging)) { Remove-Item -LiteralPath $staging }
    if ($keepBackup) { Write-Warning "Rollback failed; backup retained at $backup" }
    $installLock.Dispose()
}
