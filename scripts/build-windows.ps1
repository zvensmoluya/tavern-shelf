param(
    [switch]$Installer
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$frontendDirectory = Join-Path $repositoryRoot 'frontend'
$outputDirectory = Join-Path $repositoryRoot 'build\bin'
$desktopExecutable = Join-Path $outputDirectory 'TavernShelf.exe'
$headlessExecutable = Join-Path $outputDirectory 'tavern-shelf-server.exe'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

Push-Location $repositoryRoot
try {
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

    go build -trimpath -ldflags '-s -w -H=windowsgui' -o $desktopExecutable ./cmd/tavern-shelf-desktop
    if ($LASTEXITCODE -ne 0) { throw 'Desktop build failed' }

    go build -trimpath -ldflags '-s -w' -o $headlessExecutable ./cmd/tavern-shelf
    if ($LASTEXITCODE -ne 0) { throw 'Headless build failed' }

    if ($Installer) {
        $compiler = Get-Command ISCC.exe -ErrorAction SilentlyContinue
        if (-not $compiler) {
            $defaultCompiler = Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe'
            if (Test-Path -LiteralPath $defaultCompiler) { $compiler = Get-Item $defaultCompiler }
        }
        if (-not $compiler) { throw 'Inno Setup 6 was not found' }
        & $compiler.Source (Join-Path $repositoryRoot 'build\windows\TavernShelf.iss')
        if ($LASTEXITCODE -ne 0) { throw 'Installer build failed' }
    }
} finally {
    Pop-Location
}

Write-Host "Built $desktopExecutable"
