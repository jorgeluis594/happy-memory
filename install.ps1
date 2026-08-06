[CmdletBinding()]
param(
    [ValidatePattern('^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')]
    [string]$Version,
    [string]$BinDir,
    [switch]$Help
)

$ErrorActionPreference = 'Stop'
$Repository = if ($env:HAPPY_MEMORY_REPOSITORY) { $env:HAPPY_MEMORY_REPOSITORY } else { 'jorgeluis594/happy-memory' }
$ApiBase = if ($env:HAPPY_MEMORY_API_BASE) { $env:HAPPY_MEMORY_API_BASE } else { "https://api.github.com/repos/$Repository" }
$DownloadBase = if ($env:HAPPY_MEMORY_DOWNLOAD_BASE) { $env:HAPPY_MEMORY_DOWNLOAD_BASE } else { "https://github.com/$Repository/releases/download" }
$TempDir = $null
$Target = $null
$Backup = $null
$Installed = $false
$PathChanged = $false
$OldUserPath = $null
$Stage = $null

if ($Help) {
    Write-Output 'Usage: install.ps1 [-Version vMAJOR.MINOR.PATCH] [-BinDir DIRECTORY] [-Help]'
    return
}

try {
    $Architecture = if ($env:HAPPY_MEMORY_ARCHITECTURE) { $env:HAPPY_MEMORY_ARCHITECTURE } else { $env:PROCESSOR_ARCHITECTURE }
    switch ($Architecture.ToUpperInvariant()) {
        'AMD64' { $Arch = 'amd64' }
        'ARM64' { $Arch = 'arm64' }
        default { throw 'Unsupported architecture; supported architectures are AMD64 and ARM64.' }
    }

    $TempDir = Join-Path ([IO.Path]::GetTempPath()) ("happy-memory-install-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $TempDir | Out-Null
    if (-not $Version) {
        Write-Output 'Resolving latest stable release...'
        $Latest = Invoke-RestMethod -UseBasicParsing -Uri "$ApiBase/releases/latest"
        $Version = [string]$Latest.tag_name
        if ($Version -notmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
            throw 'Latest release returned an invalid version.'
        }
    }
    if (-not $BinDir) {
        $BinDir = Join-Path $env:LOCALAPPDATA 'Programs\happy-memory\bin'
    }

    $PlainVersion = $Version.Substring(1)
    $Archive = "happy-memory_${PlainVersion}_windows_${Arch}.zip"
    $ReleaseUrl = "$DownloadBase/$Version"
    $ArchivePath = Join-Path $TempDir $Archive
    $ChecksumsPath = Join-Path $TempDir 'checksums.txt'
    Write-Output "Downloading $Archive..."
    Invoke-WebRequest -UseBasicParsing -Uri "$ReleaseUrl/$Archive" -OutFile $ArchivePath
    Invoke-WebRequest -UseBasicParsing -Uri "$ReleaseUrl/checksums.txt" -OutFile $ChecksumsPath

    $ChecksumMatches = @(Get-Content -LiteralPath $ChecksumsPath | ForEach-Object {
        if ($_ -match '^([0-9a-fA-F]{64})\s+\*?(.+)$' -and $Matches[2] -ceq $Archive) { $Matches[1] }
    })
    if ($ChecksumMatches.Count -ne 1) { throw "checksums.txt must contain exactly one entry for $Archive." }
    $Actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $ArchivePath).Hash
    if ($Actual -cne $ChecksumMatches[0]) { throw "Checksum mismatch for $Archive." }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $Zip = [IO.Compression.ZipFile]::OpenRead($ArchivePath)
    try {
        if ($Zip.Entries.Count -ne 1 -or $Zip.Entries[0].FullName -cne 'happy-memory.exe' -or -not $Zip.Entries[0].Name) {
            throw 'Release archive must contain only happy-memory.exe.'
        }
        $Extracted = Join-Path $TempDir 'happy-memory.exe'
        [IO.Compression.ZipFileExtensions]::ExtractToFile($Zip.Entries[0], $Extracted, $true)
    } finally {
        $Zip.Dispose()
    }
    & $Extracted version | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Downloaded executable failed validation.' }

    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    $Target = Join-Path $BinDir 'happy-memory.exe'
    $Stage = Join-Path $BinDir ('.happy-memory.new.' + [guid]::NewGuid().ToString('N'))
    Move-Item -LiteralPath $Extracted -Destination $Stage
    & $Stage version | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Staged executable failed validation.' }
    if (Test-Path -LiteralPath $Target) {
        $Backup = Join-Path $BinDir ('.happy-memory.backup.' + [guid]::NewGuid().ToString('N'))
        Move-Item -LiteralPath $Target -Destination $Backup
    }
    $Installed = $true
    Move-Item -LiteralPath $Stage -Destination $Target
    $Stage = $null
    & $Target version | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Installed executable failed validation.' }

    $OldUserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $PathEntries = @($OldUserPath -split ';' | Where-Object { $_ })
    if (-not ($PathEntries | Where-Object { $_.TrimEnd('\') -ieq $BinDir.TrimEnd('\') })) {
        $NewUserPath = (@($PathEntries) + $BinDir) -join ';'
        [Environment]::SetEnvironmentVariable('Path', $NewUserPath, 'User')
        $PathChanged = $true
    }

    if ($Backup) { Remove-Item -LiteralPath $Backup -Force; $Backup = $null }
    $Installed = $false
    Write-Output "Installed happy-memory $Version at $Target"
    if ($PathChanged) { Write-Output 'The user PATH was updated. Open a new terminal to use happy-memory.' }
} catch {
    if ($PathChanged) {
        try { [Environment]::SetEnvironmentVariable('Path', $OldUserPath, 'User') } catch { }
    }
    if ($Installed -and $Target) {
        Remove-Item -LiteralPath $Target -Force -ErrorAction SilentlyContinue
        if ($Backup -and (Test-Path -LiteralPath $Backup)) { Move-Item -LiteralPath $Backup -Destination $Target -Force }
    }
    throw "happy-memory installer: $($_.Exception.Message)"
} finally {
    if ($Backup -and (Test-Path -LiteralPath $Backup)) { Remove-Item -LiteralPath $Backup -Force -ErrorAction SilentlyContinue }
    if ($Stage -and (Test-Path -LiteralPath $Stage)) { Remove-Item -LiteralPath $Stage -Force -ErrorAction SilentlyContinue }
    if ($TempDir -and (Test-Path -LiteralPath $TempDir)) { Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue }
}
