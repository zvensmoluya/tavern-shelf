param(
    [string]$Version
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$versionFile = Join-Path $repositoryRoot 'VERSION'
$releaseVersion = if ($Version) { $Version.TrimStart('v') } else { (Get-Content -Raw -LiteralPath $versionFile).Trim() }
$releaseDirectory = Join-Path $repositoryRoot 'build\release'
$installer = Join-Path $repositoryRoot "build\installer\TavernShelf-Setup-$releaseVersion.exe"
$desktop = Join-Path $repositoryRoot 'build\bin\TavernShelf.exe'
$server = Join-Path $repositoryRoot 'build\bin\tavern-shelf-server.exe'

Push-Location $repositoryRoot
try {
    & (Join-Path $PSScriptRoot 'build-windows.ps1') -Installer -Version $releaseVersion
    if ($LASTEXITCODE -ne 0) { throw 'Windows release build failed' }
    if (-not (Test-Path -LiteralPath $installer)) { throw "Installer was not created: $installer" }

    New-Item -ItemType Directory -Force -Path $releaseDirectory | Out-Null
    Get-ChildItem -File -LiteralPath $releaseDirectory | Remove-Item -Force
    $installerTarget = Join-Path $releaseDirectory "TavernShelf-Setup-$releaseVersion.exe"
    $portableTarget = Join-Path $releaseDirectory "TavernShelf-$releaseVersion-Windows-x64-portable.exe"
    $serverTarget = Join-Path $releaseDirectory "tavern-shelf-server-$releaseVersion-Windows-x64.zip"
    Copy-Item -LiteralPath $installer -Destination $installerTarget -Force
    Copy-Item -LiteralPath $desktop -Destination $portableTarget -Force
    Compress-Archive -LiteralPath $server -DestinationPath $serverTarget -CompressionLevel Optimal -Force

    $artifacts = @($installerTarget, $portableTarget, $serverTarget)
    $checksums = foreach ($artifact in $artifacts) {
        $hash = Get-FileHash -Algorithm SHA256 -LiteralPath $artifact
        "$($hash.Hash.ToLowerInvariant())  $(Split-Path -Leaf $artifact)"
    }
    [System.IO.File]::WriteAllLines((Join-Path $releaseDirectory 'SHA256SUMS.txt'), $checksums, [System.Text.UTF8Encoding]::new($false))
} finally {
    Pop-Location
}

Write-Host "Packaged Tavern Shelf $releaseVersion at $releaseDirectory"
