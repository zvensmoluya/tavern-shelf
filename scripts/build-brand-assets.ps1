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

function New-RoundedPath([float]$x, [float]$y, [float]$width, [float]$height, [float]$radius) {
    $path = [System.Drawing.Drawing2D.GraphicsPath]::new()
    $diameter = $radius * 2
    $path.AddArc($x, $y, $diameter, $diameter, 180, 90)
    $path.AddArc($x + $width - $diameter, $y, $diameter, $diameter, 270, 90)
    $path.AddArc($x + $width - $diameter, $y + $height - $diameter, $diameter, $diameter, 0, 90)
    $path.AddArc($x, $y + $height - $diameter, $diameter, $diameter, 90, 90)
    $path.CloseFigure()
    return $path
}

function New-ScaledBitmap([System.Drawing.Image]$image, [int]$size) {
    $bitmap = [System.Drawing.Bitmap]::new($size, $size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    try {
        $graphics.CompositingMode = [System.Drawing.Drawing2D.CompositingMode]::SourceCopy
        $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
        $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
        $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
        $graphics.DrawImage($image, [System.Drawing.Rectangle]::new(0, 0, $size, $size))
    } finally {
        $graphics.Dispose()
    }
    return $bitmap
}

$sourceImage = [System.Drawing.Image]::FromFile($sourcePath)
try {
    if ($sourceImage.Width -ne $sourceImage.Height) {
        throw "Brand source must be square: $($sourceImage.Width)x$($sourceImage.Height)"
    }

    $masterSize = 512
    $master = [System.Drawing.Bitmap]::new($masterSize, $masterSize, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $graphics = [System.Drawing.Graphics]::FromImage($master)
    $clipPath = New-RoundedPath 5 5 502 502 112
    try {
        $graphics.Clear([System.Drawing.Color]::Transparent)
        $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
        $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
        $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
        $graphics.SetClip($clipPath)
        $graphics.DrawImage($sourceImage, [System.Drawing.Rectangle]::new(5, 5, 502, 502))
        $graphics.ResetClip()
        $border = [System.Drawing.Pen]::new([System.Drawing.ColorTranslator]::FromHtml('#59606c'), 3)
        try {
            $graphics.DrawPath($border, $clipPath)
        } finally {
            $border.Dispose()
        }
    } finally {
        $clipPath.Dispose()
        $graphics.Dispose()
    }

    $masterPath = Join-Path $brandDirectory 'app-icon.png'
    $master.Save($masterPath, [System.Drawing.Imaging.ImageFormat]::Png)

    $compactMaster = [System.Drawing.Bitmap]::new($masterSize, $masterSize, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $compactGraphics = [System.Drawing.Graphics]::FromImage($compactMaster)
    $compactPath = New-RoundedPath 5 5 502 502 112
    try {
        $compactGraphics.Clear([System.Drawing.Color]::Transparent)
        $compactGraphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
        $compactGraphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        $compactGraphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
        $compactGraphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
        $compactGraphics.SetClip($compactPath)
        $compactGraphics.DrawImage(
            $sourceImage,
            [System.Drawing.Rectangle]::new(5, 5, 502, 502),
            [System.Drawing.Rectangle]::new(190, 0, 874, 874),
            [System.Drawing.GraphicsUnit]::Pixel
        )
        $compactGraphics.ResetClip()
        $compactBorder = [System.Drawing.Pen]::new([System.Drawing.ColorTranslator]::FromHtml('#59606c'), 3)
        try {
            $compactGraphics.DrawPath($compactBorder, $compactPath)
        } finally {
            $compactBorder.Dispose()
        }
    } finally {
        $compactPath.Dispose()
        $compactGraphics.Dispose()
    }
    try {
        $brandMark = New-ScaledBitmap $compactMaster 256
        try {
            $brandMarkPath = Join-Path $brandDirectory 'brand-mark.png'
            $embeddedBrandMark = Join-Path $iconDirectory 'brand-mark-256.png'
            $brandMark.Save($brandMarkPath, [System.Drawing.Imaging.ImageFormat]::Png)
            $brandMark.Save($embeddedBrandMark, [System.Drawing.Imaging.ImageFormat]::Png)
            Copy-Item -LiteralPath $embeddedBrandMark -Destination $frontendIcon -Force
        } finally {
            $brandMark.Dispose()
        }

        foreach ($size in $sizes) {
            $iconSource = if ($size -le 48) { $compactMaster } else { $master }
            $scaled = New-ScaledBitmap $iconSource $size
            try {
                $scaled.Save((Join-Path $iconDirectory "app-icon-$size.png"), [System.Drawing.Imaging.ImageFormat]::Png)
            } finally {
                $scaled.Dispose()
            }
        }
        $preview = [System.Drawing.Bitmap]::new(1080, 330, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
        $previewGraphics = [System.Drawing.Graphics]::FromImage($preview)
        try {
            $previewGraphics.Clear([System.Drawing.ColorTranslator]::FromHtml('#0d0f12'))
            $previewGraphics.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::ClearTypeGridFit
            $font = [System.Drawing.Font]::new('Segoe UI', 10)
            $brush = [System.Drawing.SolidBrush]::new([System.Drawing.ColorTranslator]::FromHtml('#8e96a3'))
            try {
                $x = 34
                foreach ($size in $sizes) {
                    $icon = [System.Drawing.Image]::FromFile((Join-Path $iconDirectory "app-icon-$size.png"))
                    try {
                        $previewGraphics.DrawImage($icon, $x, 46, $size, $size)
                        $previewGraphics.DrawString("${size}px", $font, $brush, $x, 58 + $size)
                    } finally {
                        $icon.Dispose()
                    }
                    $x += [Math]::Max($size + 28, 66)
                }
            } finally {
                $font.Dispose()
                $brush.Dispose()
            }
        } finally {
            $previewGraphics.Dispose()
        }
        try {
            $preview.Save($previewPath, [System.Drawing.Imaging.ImageFormat]::Png)
        } finally {
            $preview.Dispose()
        }
    } finally {
        $compactMaster.Dispose()
        $master.Dispose()
    }
} finally {
    $sourceImage.Dispose()
}

Write-Host "Built Tavern Shelf brand assets from $sourcePath"
Write-Host "Size preview: $previewPath"
