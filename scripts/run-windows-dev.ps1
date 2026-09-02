param(
    [string]$DataDir = '',
    [switch]$Background
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $repositoryRoot 'build\dev'
$desktopExecutable = Join-Path $outputDirectory 'TavernShelf-dev.exe'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

Push-Location $repositoryRoot
try {
    go build -gcflags 'all=-N -l' -o $desktopExecutable ./cmd/tavern-shelf-desktop
    if ($LASTEXITCODE -ne 0) { throw 'Desktop development build failed. Exit the existing tray process before rebuilding.' }

    $applicationArguments = @()
    if ($DataDir) { $applicationArguments += @('-data-dir', $DataDir) }
    if ($Background) { $applicationArguments += '-background' }
    & $desktopExecutable @applicationArguments
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
