param(
    [string]$Version,
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\BubbleWorldClock')
)
$ErrorActionPreference = 'Stop'
$repo = 'thomasjdelaney/BubbleWorldClock'

function Get-AssetName([string]$Version, [string]$Architecture) {
    $Version = $Version -replace '^v', ''
    switch ($Architecture.ToLowerInvariant()) {
        { $_ -in 'amd64', 'x86_64' } { $archName = 'x86_64'; break }
        { $_ -in 'arm64', 'aarch64' } { $archName = 'arm64'; break }
        default { throw "Unsupported Windows architecture: $Architecture" }
    }
    return "BubbleWorldClock_${Version}_Windows_${archName}.zip"
}

function Test-ReleaseChecksum([string]$ChecksumFile, [string]$Archive) {
    $name = [IO.Path]::GetFileName($Archive)
    $match = Get-Content $ChecksumFile | Where-Object { $_ -match "^([0-9a-fA-F]{64})\s+\*?$([regex]::Escape($name))$" } | Select-Object -First 1
    if (-not $match) { throw "No checksum entry for $name" }
    $expected = [regex]::Match($match, '^([0-9a-fA-F]{64})').Groups[1].Value
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $Archive).Hash
    if ($actual -ne $expected) { throw "Checksum mismatch for $name" }
}

$nativeArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
if ($nativeArch -eq 'ARM64') { throw 'Windows ARM64 is not currently supported; no Windows ARM64 release is published.' }
if ($nativeArch -in 'AMD64', 'x86_64') { $arch = 'amd64' }
else { throw "Unsupported Windows architecture: $nativeArch" }

if ($Version) {
    if ($Version -notmatch '^v') { $Version = "v$Version" }
    $releaseUrl = "https://api.github.com/repos/$repo/releases/tags/$Version"
} else {
    $releaseUrl = "https://api.github.com/repos/$repo/releases/latest"
}
$tmp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    $release = Invoke-RestMethod -Uri $releaseUrl -Headers @{ 'User-Agent' = 'BubbleWorldClock-installer' }
    $Version = $release.tag_name
    if (-not $Version) { throw 'Could not read release version' }
    $asset = Get-AssetName $Version $arch
    $base = "https://github.com/$repo/releases/download/$Version"
    $archive = Join-Path $tmp $asset
    $checksums = Join-Path $tmp 'checksums.txt'
    Invoke-WebRequest -Uri "$base/$asset" -OutFile $archive
    Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $checksums
    Test-ReleaseChecksum $checksums $archive

    $extract = Join-Path $tmp 'extract'
    Expand-Archive -LiteralPath $archive -DestinationPath $extract
    $binary = Join-Path $extract 'bubble-world-clock.exe'
    if (-not (Test-Path -LiteralPath $binary)) { throw 'Archive does not contain bubble-world-clock.exe' }
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -LiteralPath $binary -Destination (Join-Path $InstallDir 'bubble-world-clock.exe') -Force

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = @($userPath -split ';' | Where-Object { $_ })
    if (-not ($entries | Where-Object { [IO.Path]::GetFullPath($_).TrimEnd('\') -ieq [IO.Path]::GetFullPath($InstallDir).TrimEnd('\') })) {
        $newPath = (@($entries) + $InstallDir) -join ';'
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    }
    Write-Output "Installed bubble-world-clock.exe to $InstallDir"
    Write-Output 'Open a new terminal to use the updated PATH.'
} finally {
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
