param(
    [switch]$Installer,
    [string]$Version
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$frontendDirectory = Join-Path $repositoryRoot 'frontend'
$outputDirectory = Join-Path $repositoryRoot 'build\bin'
$desktopExecutable = Join-Path $outputDirectory 'TavernShelf.exe'
$headlessExecutable = Join-Path $outputDirectory 'tavern-shelf-server.exe'
$windowsIcon = Join-Path $repositoryRoot 'build\windows\TavernShelf.ico'
$versionFile = Join-Path $repositoryRoot 'VERSION'
$releaseVersion = if ($Version) { $Version.TrimStart('v') } else { (Get-Content -Raw -LiteralPath $versionFile).Trim() }
if ($releaseVersion -notmatch '^\d+\.\d+\.\d+$') { throw "Invalid release version: $releaseVersion" }
$commonLDFlags = "-s -w -X main.version=$releaseVersion"

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

Push-Location $repositoryRoot
try {
    go run ./scripts/generate-windows-icon.go $windowsIcon
    if ($LASTEXITCODE -ne 0) { throw 'Windows icon generation failed' }

    Push-Location $frontendDirectory
    try {
        npm ci
        if ($LASTEXITCODE -ne 0) { throw 'Frontend dependency install failed' }
        npm run build
        if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' }
    } finally {
        Pop-Location
    }

    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }

    go build -trimpath -ldflags "$commonLDFlags -H=windowsgui" -o $desktopExecutable ./cmd/tavern-shelf-desktop
    if ($LASTEXITCODE -ne 0) { throw 'Desktop build failed' }

    go build -trimpath -ldflags $commonLDFlags -o $headlessExecutable ./cmd/tavern-shelf
    if ($LASTEXITCODE -ne 0) { throw 'Headless build failed' }

    if ($Installer) {
        $compilerPath = if ($env:TAVERN_SHELF_ISCC -and (Test-Path -LiteralPath $env:TAVERN_SHELF_ISCC)) { $env:TAVERN_SHELF_ISCC } else { $null }
        $compiler = if ($compilerPath) { $null } else { Get-Command ISCC.exe -ErrorAction SilentlyContinue }
        if ($compiler) { $compilerPath = $compiler.Source }
        if (-not $compilerPath) {
            $defaultCompiler = Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'
            if (Test-Path -LiteralPath $defaultCompiler) { $compilerPath = $defaultCompiler }
        }
        if (-not $compilerPath) { throw 'Inno Setup 6 was not found' }
        & $compilerPath "/DAppVersion=$releaseVersion" (Join-Path $repositoryRoot 'build\windows\TavernShelf.iss')
        if ($LASTEXITCODE -ne 0) { throw 'Installer build failed' }
    }
} finally {
    Pop-Location
}

Write-Host "Built Tavern Shelf $releaseVersion at $outputDirectory"
