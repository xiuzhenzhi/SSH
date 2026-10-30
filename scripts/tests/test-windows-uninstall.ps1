$ErrorActionPreference='Stop'
$root=Join-Path ([IO.Path]::GetTempPath()) ('sshguard-uninstall-'+[guid]::NewGuid())
$programs=Join-Path $root 'programs'
$data=Join-Path $root 'data'
$install=Join-Path $programs 'SSHGuardAgent'
$state=Join-Path $data 'SSHGuard\state'
New-Item -ItemType Directory -Path $install,$state -Force | Out-Null
foreach($path in @((Join-Path $install 'sshguard-agent.exe'),(Join-Path $install 'leftover.tmp'),(Join-Path $state 'state.db'),(Join-Path $data 'SSHGuard\agent.json'))) { Set-Content -LiteralPath $path -Value 'secret' }
Set-Content -LiteralPath (Join-Path $data 'SSHGuard\unrelated.txt') -Value 'keep'
$source=Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '../uninstall-agent.ps1')
$source=$source.Replace('$env:ProgramFiles',("'{0}'" -f $programs.Replace("'","''"))).Replace('$env:ProgramData',("'{0}'" -f $data.Replace("'","''")))
$source=$source.Replace("[Diagnostics.EventLog]::SourceExists('SSHGuardAgent')",'$false')
function Get-ScheduledTask { return $null }
function Get-Service { return $null }
& ([ScriptBlock]::Create($source))
foreach($path in @($install,$state,(Join-Path $data 'SSHGuard\agent.json'))) { if(Test-Path -LiteralPath $path){throw "Residual Agent data: $path"} }
if((Get-Content -LiteralPath (Join-Path $data 'SSHGuard\unrelated.txt')) -ne 'keep'){throw 'Unrelated data changed'}
$ast=[System.Management.Automation.Language.Parser]::ParseInput($source,[ref]$null,[ref]$null)
$fn=$ast.Find({param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq 'Assert-CleanupPath'},$true)
. ([ScriptBlock]::Create($fn.Extent.Text))
$rejected=$false
try { Assert-CleanupPath (Join-Path $root 'outside') $data } catch { $rejected=$true }
if(-not $rejected){throw 'Out-of-root target accepted'}
Write-Output 'Windows uninstall purge and path boundary checks passed.'
