$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$temp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $temp | Out-Null
$env:TWIKKO_TEST_FIXTURE = $temp
$originalPath = $env:Path
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$originalArch = $env:PROCESSOR_ARCHITECTURE
$originalArchOverride = $env:PROCESSOR_ARCHITEW6432
function Invoke-RestMethod { param($Uri) return @{ tag_name = 'v1.2.3' } }
function Invoke-WebRequest {
    param($Uri, $OutFile, [switch]$UseBasicParsing)
    if ($Uri.EndsWith('checksums.txt')) {
        $digest = (Get-FileHash (Join-Path $env:TWIKKO_TEST_FIXTURE 'fixture.zip') -Algorithm SHA256).Hash
        if ($env:TWIKKO_TEST_CORRUPT) { $digest = '0' * 64 }
        Set-Content -LiteralPath $OutFile -Value "$digest  twikko_1.2.3_windows_$($env:PROCESSOR_ARCHITECTURE.ToLowerInvariant()).zip"
    } else {
        if (-not $Uri.EndsWith("twikko_1.2.3_windows_$($env:PROCESSOR_ARCHITECTURE.ToLowerInvariant()).zip")) { throw "Unexpected asset: $Uri" }
        Copy-Item (Join-Path $env:TWIKKO_TEST_FIXTURE 'fixture.zip') $OutFile
    }
}
try {
    Set-Content (Join-Path $temp 'twikko.exe') 'fixture binary'
    Compress-Archive -LiteralPath (Join-Path $temp 'twikko.exe') -DestinationPath (Join-Path $env:TWIKKO_TEST_FIXTURE 'fixture.zip')
    $env:PROCESSOR_ARCHITEW6432 = ''
    $installDir = Join-Path $temp 'installed'
    foreach ($arch in @('amd64', 'arm64')) {
        $env:PROCESSOR_ARCHITECTURE = $arch.ToUpperInvariant()
        & "$root/install.ps1" -InstallDir $installDir
        if ((Get-Content (Join-Path $installDir 'twikko.exe')) -ne 'fixture binary') { throw 'Wrong installed content' }
        if ($installDir -notin @($env:Path -split ';')) { throw 'PATH was not updated' }
    }
    Set-Content (Join-Path $installDir 'twikko.exe') 'existing binary'
    $env:TWIKKO_TEST_CORRUPT = 'yes'
    $rejected = $false
    try { & "$root/install.ps1" -InstallDir $installDir } catch {
        if ($_ -notmatch 'SHA-256 verification failed') { throw }
        $rejected = $true
    }
    if (-not $rejected) { throw 'Accepted corrupt archive' }
    if ((Get-Content (Join-Path $installDir 'twikko.exe')) -ne 'existing binary') { throw 'Replaced binary after failed verification' }
    Write-Host 'PowerShell installer checks passed.'
} finally {
    [Environment]::SetEnvironmentVariable('Path', $userPath, 'User')
    $env:Path = $originalPath
    $env:PROCESSOR_ARCHITECTURE = $originalArch
    $env:PROCESSOR_ARCHITEW6432 = $originalArchOverride
    Remove-Item Env:TWIKKO_TEST_FIXTURE, Env:TWIKKO_TEST_CORRUPT -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $temp -Recurse -Force
}
