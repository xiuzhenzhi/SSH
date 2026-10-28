#Requires -Version 5.1
#Requires -RunAsAdministrator
# Real SCM/LocalService test: disposable GitHub Windows runner ONLY.
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
if ($env:GITHUB_ACTIONS -ne 'true') { throw 'Disposable GitHub Actions VM only.' }
if (Get-Service SSHGuardAgent -ErrorAction SilentlyContinue) { throw 'Refusing to replace an existing service.' }
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$temp = Join-Path $root ('.local\windows-service-' + [Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $temp | Out-Null
$download = Join-Path $temp 'sshguard-agent-windows-amd64.exe'
$checksum = "$download.sha256"
$exe = Join-Path $env:ProgramFiles 'SSHGuardAgent\sshguard-agent.exe'
$config = Join-Path $env:ProgramData 'SSHGuard\agent.json'
$cert = $null
$server = $null
function Write-Checksum { [IO.File]::WriteAllText($checksum, ((Get-FileHash $download -Algorithm SHA256).Hash.ToLower() + '  sshguard-agent-windows-amd64.exe')) }
try {
    go build -trimpath -o $download ./cmd/agent
    if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
    Write-Checksum
    Copy-Item $download (Join-Path $temp 'good.exe')
    $cert = New-SelfSignedCertificate -DnsName localhost -CertStoreLocation Cert:\LocalMachine\My -KeyExportPolicy Exportable -NotAfter (Get-Date).AddDays(2)
    $pfxPassword = [Guid]::NewGuid().ToString('N')
    Export-PfxCertificate -Cert $cert -FilePath (Join-Path $temp 'server.pfx') -CryptoAlgorithmOption AES256_SHA256 -Password (ConvertTo-SecureString $pfxPassword -AsPlainText -Force) | Out-Null
    Export-Certificate -Cert $cert -FilePath (Join-Path $temp 'server.cer') | Out-Null
    Import-Certificate -FilePath (Join-Path $temp 'server.cer') -CertStoreLocation Cert:\LocalMachine\Root | Out-Null
    $env:SSHGUARD_TEST_PFX = Join-Path $temp 'server.pfx'
    $env:SSHGUARD_TEST_PFX_PASSWORD = $pfxPassword
    $env:SSHGUARD_TEST_DOWNLOAD_ROOT = $temp
    New-Item -ItemType Directory -Force -Path (Split-Path $config) | Out-Null
    $env:SSHGUARD_TEST_LOG = Join-Path (Split-Path $config) 'fixture-auth.log'
    [IO.File]::WriteAllText($env:SSHGUARD_TEST_LOG,'')
    $fixture = Join-Path $PSScriptRoot 'test-agent-server.mjs'
    $server = Start-Process -FilePath (Get-Command node).Source -ArgumentList @(('"{0}"' -f $fixture)) -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $temp 'server.log') -RedirectStandardError (Join-Path $temp 'server-error.log')
    $ready = $false
    for ($i=0; $i -lt 20; $i++) {
        try { Invoke-WebRequest -UseBasicParsing -TimeoutSec 2 https://localhost:9443/ready | Out-Null; $ready=$true; break } catch { $lastError=$_.Exception.Message; Start-Sleep -Seconds 1 }
    }
    if (-not $ready) { Get-Content (Join-Path $temp 'server-error.log'); throw "HTTPS fixture failed: $lastError" }
    & "$PSScriptRoot/install-agent.ps1" -Endpoint https://localhost:9443 -EnrollmentToken ('a'*64)
    if ((Get-Service SSHGuardAgent).Status -ne 'Running') { throw 'Service not running' }
    $service = Get-CimInstance Win32_Service -Filter "Name='SSHGuardAgent'"
    if ($service.StartName -ne 'NT AUTHORITY\LocalService') { throw 'Unexpected service identity' }
    Start-Sleep -Seconds 3
    $line=(Get-Date).ToUniversalTime().ToString('o')+' host sshd[1]: Accepted publickey for ci-fixture from 192.0.2.10 port 2222 ssh2'+"`n"
    [IO.File]::AppendAllText($env:SSHGUARD_TEST_LOG,$line,(New-Object Text.UTF8Encoding($false)))
    $collected=$false
    for($i=0;$i -lt 15;$i++) {
        $result=Invoke-RestMethod -Uri https://localhost:9443/collected
        if($result.count -eq 1){$collected=$true;break}
        Start-Sleep -Seconds 1
    }
    if(-not $collected){throw 'LocalService did not collect and upload the fixture event over HTTPS fallback'}
    if(-not (Test-Path (Join-Path (Split-Path $config) 'state\state.db'))){throw 'Durable state missing'}
    $configHash = (Get-FileHash $config -Algorithm SHA256).Hash
    Stop-Service SSHGuardAgent
    (Get-Service SSHGuardAgent).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(30))
    $stream=[IO.File]::Open($exe,'Append','Write'); $stream.WriteByte(10); $stream.Dispose()
    Start-Service SSHGuardAgent
    & "$PSScriptRoot/install-agent.ps1" -Endpoint https://localhost:9443 -Update
    & "$PSScriptRoot/install-agent.ps1" -Endpoint https://localhost:9443 -Update
    if ((Get-FileHash $config -Algorithm SHA256).Hash -ne $configHash) { throw 'Update changed credentials' }
    if ((Get-FileHash $exe -Algorithm SHA256).Hash -ne (Get-FileHash $download -Algorithm SHA256).Hash) { throw 'Replacement failed' }
    [IO.File]::WriteAllText($checksum, ('0'*64))
    $rejected=$false
    try { & "$PSScriptRoot/install-agent.ps1" -Endpoint https://localhost:9443 -Update } catch { $rejected=$true }
    if (-not $rejected -or (Get-Service SSHGuardAgent).Status -ne 'Running') { throw 'Checksum failure interrupted running service' }
    # This signed-by-checksum test payload runs version/check but cannot register a service.
    $broken = Join-Path $temp 'broken.go'
    [IO.File]::WriteAllText($broken,'package main; import "os"; func main(){if len(os.Args)>1 && (os.Args[1]=="-version" || os.Args[1]=="-check"){return}; os.Exit(1)}')
    go build -o $download $broken
    if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed' }
    Write-Checksum
    $rejected=$false
    try { & "$PSScriptRoot/install-agent.ps1" -Endpoint https://localhost:9443 -Update } catch { $rejected=$true }
    if (-not $rejected) { throw 'Broken update unexpectedly succeeded' }
    if ((Get-Service SSHGuardAgent).Status -ne 'Running') { throw 'Rollback did not restart old service' }
    if ((Get-FileHash $config -Algorithm SHA256).Hash -ne $configHash) { throw 'Rollback changed credentials' }
    if ((Get-FileHash $exe -Algorithm SHA256).Hash -ne (Get-FileHash (Join-Path $temp 'good.exe') -Algorithm SHA256).Hash) { throw 'Rollback binary mismatch' }
    Write-Output 'Windows real LocalService install, update, idempotence, checksum rejection and rollback passed.'
} finally {
    if (Get-Service SSHGuardAgent -ErrorAction SilentlyContinue) { & "$PSScriptRoot/uninstall-agent.ps1" }
    if ($server -and -not $server.HasExited) { Stop-Process -Id $server.Id }
    if ($cert) {
        Remove-Item -LiteralPath ("Cert:\LocalMachine\Root\"+$cert.Thumbprint) -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath ("Cert:\LocalMachine\My\"+$cert.Thumbprint) -ErrorAction SilentlyContinue
    }
    $env:SSHGUARD_TEST_PFX_PASSWORD=$null
}
