[CmdletBinding()]
param(
    [ValidatePattern('^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')]
    [string]$Version = $env:TWIKKO_VERSION,
    [string]$InstallDir = $(if ($env:TWIKKO_INSTALL_DIR) { $env:TWIKKO_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'twikko' })
)
$ErrorActionPreference = 'Stop'
$repo = 'bettertomorrow-dev/twikko'
$machine = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$arch = switch ($machine) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "Unsupported architecture: $machine" }
}
if (-not $Version) {
    $Version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
}
if ($Version -notmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') { throw 'Expected version vX.Y.Z.' }
$asset = "twikko_$($Version.Substring(1))_windows_$arch.zip"
$base = "https://github.com/$repo/releases/download/$Version"
$temp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $archive = Join-Path $temp $asset
    $checksums = Join-Path $temp 'checksums.txt'
    Invoke-WebRequest "$base/$asset" -OutFile $archive -UseBasicParsing
    Invoke-WebRequest "$base/checksums.txt" -OutFile $checksums -UseBasicParsing
    $pattern = '^([a-fA-F0-9]{64})\s+' + [regex]::Escape($asset) + '$'
    $expected = Get-Content -LiteralPath $checksums | ForEach-Object {
        if ($_ -match $pattern) { $Matches[1] }
    } | Select-Object -First 1
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash
    if (-not $expected -or $expected -ne $actual) { throw 'SHA-256 verification failed.' }
    Expand-Archive -LiteralPath $archive -DestinationPath (Join-Path $temp 'expanded')
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -LiteralPath (Join-Path $temp 'expanded/twikko.exe') -Destination (Join-Path $InstallDir 'twikko.exe') -Force
    $InstallDir = (Resolve-Path -LiteralPath $InstallDir).Path
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($InstallDir -notin @($userPath -split ';')) {
        [Environment]::SetEnvironmentVariable('Path', "$InstallDir;$userPath", 'User')
    }
    if ($InstallDir -notin @($env:Path -split ';')) { $env:Path = "$InstallDir;$env:Path" }
    Write-Host "Installed twikko $Version to $InstallDir"
} finally {
    Remove-Item -LiteralPath $temp -Recurse -Force
}
