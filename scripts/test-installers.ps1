$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$Work = Join-Path ([IO.Path]::GetTempPath()) ("happy-memory-installer-test-" + [guid]::NewGuid().ToString('N'))
$Release = Join-Path $Work 'releases\download\v1.2.3'
$OriginalUserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$OriginalMachinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')

function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    if ($Uri -notmatch '^mock://(.+)$') { throw "unexpected URL: $Uri" }
    Copy-Item -LiteralPath $Matches[1] -Destination $OutFile
}

function Invoke-RestMethod {
    param([string]$Uri, [switch]$UseBasicParsing)
    if ($Uri -ne "mock://$Work/api/releases/latest") { throw "unexpected API URL: $Uri" }
    [pscustomobject]@{ tag_name = 'v1.2.3' }
}

try {
    New-Item -ItemType Directory -Force -Path $Release | Out-Null
    $FixtureArch = switch ($env:PROCESSOR_ARCHITECTURE.ToUpperInvariant()) { 'AMD64' { 'amd64' } 'ARM64' { 'arm64' } default { throw 'unsupported test architecture' } }
    $FixtureExe = Join-Path $Work 'happy-memory.exe'
    Copy-Item -LiteralPath (Join-Path $Root 'happy-memory.exe') -Destination $FixtureExe
    $ArchiveName = "happy-memory_1.2.3_windows_$FixtureArch.zip"
    $Archive = Join-Path $Release $ArchiveName
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $Zip = [IO.Compression.ZipFile]::Open($Archive, [IO.Compression.ZipArchiveMode]::Create)
    try { [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($Zip, $FixtureExe, 'happy-memory.exe') | Out-Null } finally { $Zip.Dispose() }
    $Hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $Archive).Hash
    Set-Content -LiteralPath (Join-Path $Release 'checksums.txt') -Value "$Hash  $ArchiveName" -Encoding ASCII

    $env:HAPPY_MEMORY_API_BASE = "mock://$Work/api"
    $env:HAPPY_MEMORY_DOWNLOAD_BASE = "mock://$Work/releases/download"
    $env:HAPPY_MEMORY_ARCHITECTURE = $env:PROCESSOR_ARCHITECTURE
    $BinDir = Join-Path $Work 'path with spaces\bin'
    & (Join-Path $Root 'install.ps1') -BinDir $BinDir
    & (Join-Path $Root 'install.ps1') -Version v1.2.3 -BinDir $BinDir

    $UserEntries = @([Environment]::GetEnvironmentVariable('Path', 'User') -split ';' | Where-Object { $_.TrimEnd('\') -ieq $BinDir.TrimEnd('\') })
    if ($UserEntries.Count -ne 1) { throw "user PATH contains $($UserEntries.Count) installer entries" }
    if ([Environment]::GetEnvironmentVariable('Path', 'Machine') -cne $OriginalMachinePath) { throw 'machine PATH changed' }
    $Before = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $BinDir 'happy-memory.exe')).Hash

    Set-Content -LiteralPath (Join-Path $Release 'checksums.txt') -Value "$('0' * 64)  $ArchiveName" -Encoding ASCII
    $Failed = $false
    try { & (Join-Path $Root 'install.ps1') -Version v1.2.3 -BinDir $BinDir } catch { $Failed = $true }
    if (-not $Failed) { throw 'bad checksum unexpectedly succeeded' }
    $After = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $BinDir 'happy-memory.exe')).Hash
    if ($After -cne $Before) { throw 'failed installation replaced existing executable' }

    Write-Output 'Windows installer fixtures passed'
} finally {
    [Environment]::SetEnvironmentVariable('Path', $OriginalUserPath, 'User')
    Remove-Item Env:HAPPY_MEMORY_API_BASE, Env:HAPPY_MEMORY_DOWNLOAD_BASE, Env:HAPPY_MEMORY_ARCHITECTURE -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $Work -Recurse -Force -ErrorAction SilentlyContinue
}
