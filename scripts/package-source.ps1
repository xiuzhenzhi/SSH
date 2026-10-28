param([string]$OutputDirectory = 'dist')
$ErrorActionPreference = 'Stop'
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Set-Location $projectRoot
$out = [System.IO.Path]::GetFullPath((Join-Path $projectRoot $OutputDirectory))
New-Item -ItemType Directory -Force -Path $out | Out-Null
Add-Type -AssemblyName System.IO.Compression.FileSystem
$sourceZip = Join-Path $out 'sshguard-source.zip'
if (Test-Path -LiteralPath $sourceZip) { Remove-Item -LiteralPath $sourceZip }
$archive = [System.IO.Compression.ZipFile]::Open($sourceZip, 'Create')
try {
  $inputs = @('.github','cmd','internal','migrations','web','deploy','scripts','docs','go.mod','go.sum','VERSION','README.md','CHANGELOG.md','CONTRIBUTING.md','SECURITY.md','.gitignore','.gitattributes','.dockerignore','.env.example','compose.yaml','compose.mysql.yaml','compose.postgres.yaml','compose.local.yaml')
  foreach ($inputPath in $inputs) {
    Get-ChildItem -LiteralPath (Join-Path $projectRoot $inputPath) -File -Recurse -Force | ForEach-Object {
      $relative = $_.FullName.Substring($projectRoot.Length + 1).Replace('\','/')
      if ($relative -notmatch '(^web/(node_modules|dist)/|^deploy/certs/(?!\.gitkeep))') {
        [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, $_.FullName, $relative, 'Optimal') | Out-Null
      }
    }
  }
} finally { $archive.Dispose() }
# A self-contained project creation script. Refuses a nonempty destination.
$payload = [Convert]::ToBase64String([System.IO.File]::ReadAllBytes($sourceZip))
$script = @'
param([string]$Destination = '.\sshguard')
$ErrorActionPreference = 'Stop'
$destinationPath = [System.IO.Path]::GetFullPath($Destination)
if ((Test-Path -LiteralPath $destinationPath) -and (Get-ChildItem -LiteralPath $destinationPath -Force | Select-Object -First 1)) { throw 'Destination must be empty; existing files will not be overwritten.' }
New-Item -ItemType Directory -Force -Path $destinationPath | Out-Null
Add-Type -AssemblyName System.IO.Compression.FileSystem
Add-Type -AssemblyName System.IO.Compression
$bytes = [Convert]::FromBase64String('PAYLOAD')
$stream = [System.IO.MemoryStream]::new($bytes, $false)
$zip = [System.IO.Compression.ZipArchive]::new($stream)
try {
  foreach ($entry in $zip.Entries) {
    $target = [System.IO.Path]::GetFullPath((Join-Path $destinationPath $entry.FullName))
    if (-not $target.StartsWith($destinationPath.TrimEnd('\') + '\', [System.StringComparison]::OrdinalIgnoreCase)) { throw 'Invalid archive path' }
    New-Item -ItemType Directory -Force -Path ([System.IO.Path]::GetDirectoryName($target)) | Out-Null
    [System.IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $target, $false)
  }
} finally { $zip.Dispose(); $stream.Dispose() }
Write-Output "Created project: $destinationPath"
Write-Output 'Next: read README.md; select native deployment or optional Docker Compose.'
'@
[System.IO.File]::WriteAllText((Join-Path $out 'create-project.ps1'), $script.Replace('PAYLOAD', $payload), [System.Text.UTF8Encoding]::new($false))
Write-Output "Created $sourceZip and create-project.ps1"
