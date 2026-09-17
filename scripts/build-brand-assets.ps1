param(
    [string]$Source = 'brand\shelf-keeper.png'
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$sourcePath = Join-Path $repositoryRoot $Source
$brandDirectory = Join-Path $repositoryRoot 'brand'
$iconDirectory = Join-Path $repositoryRoot 'internal\brand\icons'
$frontendIcon = Join-Path $repositoryRoot 'frontend\public\brand-mark.png'
$previewDirectory = Join-Path $repositoryRoot 'build\tools'
$previewPath = Join-Path $previewDirectory 'brand-size-preview.png'
$sizes = @(16, 24, 32, 48, 64, 128, 256)

if (-not (Test-Path -LiteralPath $sourcePath)) {
    throw "Brand source was not found: $sourcePath"
}

Add-Type -AssemblyName System.Drawing
New-Item -ItemType Directory -Force -Path $brandDirectory, $iconDirectory, $previewDirectory | Out-Null

function New-ScaledBitmap([System.Drawing.Image]$image, [int]$size) {
    $bitmap = [System.Drawing.Bitmap]::new($size, $size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $attributes = [System.Drawing.Imaging.ImageAttributes]::new()
    try {
        $graphics.CompositingMode = [System.Drawing.Drawing2D.CompositingMode]::SourceCopy
        $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
        $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
        $attributes.SetWrapMode([System.Drawing.Drawing2D.WrapMode]::TileFlipXY)
        $graphics.DrawImage($image, [System.Drawing.Rectangle]::new(0, 0, $size, $size), 0, 0, $image.Width, $image.Height, [System.Drawing.GraphicsUnit]::Pixel, $attributes)
    } finally {
        $attributes.Dispose()
        $graphics.Dispose()
    }
    return $bitmap
}

$sourceImage = [System.Drawing.Image]::FromFile($sourcePath)
try {
    if ($sourceImage.Width -ne $sourceImage.Height) {
        throw "Brand source must be square: $($sourceImage.Width)x$($sourceImage.Height)"
    }

    # Resize directly from the transparent master: no frame, crop, or repeated scaling.
    $master = New-ScaledBitmap $sourceImage 512
    try {
        $master.Save((Join-Path $brandDirectory 'app-icon.png'), [System.Drawing.Imaging.ImageFormat]::Png)
    } finally {
        $master.Dispose()
    }

    foreach ($size in $sizes) {
        $scaled = New-ScaledBitmap $sourceImage $size
        try {
            $scaled.Save((Join-Path $iconDirectory "app-icon-$size.png"), [System.Drawing.Imaging.ImageFormat]::Png)
        } finally {
            $scaled.Dispose()
        }
    }
    $icon256 = Join-Path $iconDirectory 'app-icon-256.png'
    Copy-Item -LiteralPath $icon256 -Destination (Join-Path $brandDirectory 'brand-mark.png') -Force
    Copy-Item -LiteralPath $icon256 -Destination (Join-Path $iconDirectory 'brand-mark-256.png') -Force
    Copy-Item -LiteralPath $icon256 -Destination $frontendIcon -Force
} finally {
    $sourceImage.Dispose()
}

$preview = [System.Drawing.Bitmap]::new(1080, 660, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
try {
    $graphics = [System.Drawing.Graphics]::FromImage($preview)
    try {
        $graphics.Clear([System.Drawing.ColorTranslator]::FromHtml('#0d0f12'))
        $graphics.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::ClearTypeGridFit
        $lightBackground = [System.Drawing.SolidBrush]::new([System.Drawing.ColorTranslator]::FromHtml('#f4f1eb'))
        $font = [System.Drawing.Font]::new('Segoe UI', 10)
        $labelBrush = [System.Drawing.SolidBrush]::new([System.Drawing.ColorTranslator]::FromHtml('#808080'))
        try {
            $graphics.FillRectangle($lightBackground, 0, 330, 1080, 330)
            foreach ($row in @(0, 330)) {
                $x = 34
                foreach ($size in $sizes) {
                    $icon = [System.Drawing.Image]::FromFile((Join-Path $iconDirectory "app-icon-$size.png"))
                    try {
                        $graphics.DrawImage($icon, $x, $row + 30, $size, $size)
                        $graphics.DrawString("${size}px", $font, $labelBrush, $x, $row + 42 + $size)
                    } finally {
                        $icon.Dispose()
                    }
                    $x += [Math]::Max($size + 28, 66)
                }
            }
        } finally {
            $labelBrush.Dispose()
            $font.Dispose()
            $lightBackground.Dispose()
        }
    } finally {
        $graphics.Dispose()
    }
    $preview.Save($previewPath, [System.Drawing.Imaging.ImageFormat]::Png)
} finally {
    $preview.Dispose()
}

Write-Host "Built Tavern Shelf brand assets from $sourcePath"
Write-Host "Size preview: $previewPath"
