#Requires -Version 5.1
#Requires -RunAsAdministrator
param(
    [Parameter(Mandatory=$true)][string]$Endpoint,
    [Parameter(Mandatory=$true)][ValidatePattern('^[a-f0-9]{64}$')][string]$EnrollmentToken
)
$ErrorActionPreference = 'Stop'
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
$staging = Join-Path $installDir ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $staging | Out-Null
$download = Join-Path $staging 'sshguard-agent.exe'
$checksum = Join-Path $staging 'agent.sha256'
$service = Get-Service -Name SSHGuardAgent -ErrorAction SilentlyContinue
$wasRunning = $service -and $service.Status -eq 'Running'
$enrolled = $false
try {
    Invoke-WebRequest -UseBasicParsing -MaximumRedirection 0 -Uri "$Endpoint/downloads/sshguard-agent-windows-$arch.exe" -OutFile $download
    Invoke-WebRequest -UseBasicParsing -MaximumRedirection 0 -Uri "$Endpoint/downloads/sshguard-agent-windows-$arch.exe.sha256" -OutFile $checksum
    $expected = ((Get-Content -LiteralPath $checksum -Raw).Trim() -split '\s+')[0]
    if ($expected -notmatch '^[a-f0-9]{64}$' -or (Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash -ne $expected) { throw 'Agent checksum mismatch.' }
    if ($service) {
        # Reject a colliding service name before touching the existing service.
        $installed = Get-CimInstance Win32_Service -Filter "Name='SSHGuardAgent'"
        $quotedExe = if ($exe.Contains(' ')) { '"{0}"' -f $exe } else { $exe }
        $quotedConfig = if ($config.Contains(' ')) { '"{0}"' -f $config } else { $config }
        $expectedCommand = '{0} -config {1}' -f $quotedExe, $quotedConfig
        if ($installed.PathName -ne $expectedCommand) { throw 'Existing SSHGuardAgent service belongs to a different installation.' }
        Stop-Service -Name SSHGuardAgent
        (Get-Service SSHGuardAgent).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(30))
    }
    # Token travels over stdin; only the short-lived install command contains it.
    $EnrollmentToken | & $download -enroll -endpoint $Endpoint -config $config
    if ($LASTEXITCODE -ne 0) { throw 'Enrollment failed. Generate a new installation command and retry.' }
    $enrolled = $true
    $EnrollmentToken = $null
    # Replace the file ACL too; do not retain explicit permissions from an older config.
    Protect-File $config
    Copy-Item -LiteralPath $download -Destination $exe -Force
    Protect-File $exe
    & $exe -service install -config $config
    if ($LASTEXITCODE -ne 0) { throw 'Windows service registration failed; see the preceding error.' }
    Start-Service -Name SSHGuardAgent
    (Get-Service SSHGuardAgent).WaitForStatus('Running',[TimeSpan]::FromSeconds(30))
    Write-Output 'Installed as LocalService. Phase 1: registration + heartbeat only.'
    Write-Output 'Logs: Windows Event Viewer / Application / SSHGuardAgent.'
} catch {
    if ($wasRunning -and -not $enrolled) { Start-Service -Name SSHGuardAgent -ErrorAction Continue }
    throw
} finally {
    # Only delete the exact temporary files created by this invocation.
    foreach ($file in @($download,$checksum)) { if (Test-Path -LiteralPath $file) { Remove-Item -LiteralPath $file -Force } }
    if (Test-Path -LiteralPath $staging) { Remove-Item -LiteralPath $staging }
}
