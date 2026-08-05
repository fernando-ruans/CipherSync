Add-Type -AssemblyName System.Drawing

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$logo = [System.Drawing.Image]::FromFile((Join-Path $root 'ciphersync-logo.png'))

function New-SquareIcon([System.Drawing.Image]$src, [int]$size) {
    $canvas = New-Object System.Drawing.Bitmap($size, $size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($canvas)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $g.Clear([System.Drawing.Color]::Transparent)

    # fit-contain preserving aspect ratio
    $ratio = [Math]::Min($size / $src.Width, $size / $src.Height)
    $w = [int]($src.Width * $ratio)
    $h = [int]($src.Height * $ratio)
    $x = [int](($size - $w) / 2)
    $y = [int](($size - $h) / 2)
    $g.DrawImage($src, $x, $y, $w, $h)
    $g.Dispose()
    return $canvas
}

# 1) Square appicon.png (512x512)
$icon512 = New-SquareIcon $logo 512
$icon512.Save((Join-Path $root 'build\appicon.png'), [System.Drawing.Imaging.ImageFormat]::Png)
$icon512.Dispose()
Write-Output "appicon.png -> 512x512"

# 2) Multi-size icon.ico (Vista+ PNG-in-ICO)
function ConvertTo-PngBytes([System.Drawing.Bitmap]$bmp) {
    $ms = New-Object System.IO.MemoryStream
    $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    $bytes = $ms.ToArray()
    $ms.Dispose()
    return $bytes
}

$sizes = @(16, 24, 32, 48, 64, 128, 256)
$images = @()
foreach ($s in $sizes) {
    $bmp = New-SquareIcon $logo $s
    $png = ConvertTo-PngBytes $bmp
    $bmp.Dispose()
    $images += ,@($s, $png)
}

$ms = New-Object System.IO.MemoryStream
$bw = New-Object System.IO.BinaryWriter($ms)
# ICONDIR
$bw.Write([UInt16]0); $bw.Write([UInt16]1); $bw.Write([UInt16]$images.Count)
$offset = 6 + (16 * $images.Count)
foreach ($img in $images) {
    $s = $img[0]; $bytes = $img[1]
    $bw.Write([Byte]($(if ($s -ge 256) { 0 } else { $s })))
    $bw.Write([Byte]($(if ($s -ge 256) { 0 } else { $s })))
    $bw.Write([Byte]0)          # palette
    $bw.Write([Byte]0)          # reserved
    $bw.Write([UInt16]1)        # planes
    $bw.Write([UInt16]32)       # bpp
    $bw.Write([UInt32]$bytes.Length)
    $bw.Write([UInt32]$offset)
    $offset += $bytes.Length
}
foreach ($img in $images) {
    $bw.Write([Byte[]]$img[1])
}
$bw.Flush()
$icoBytes = $ms.ToArray()
$bw.Dispose(); $ms.Dispose()
[System.IO.File]::WriteAllBytes((Join-Path $root 'build\windows\icon.ico'), $icoBytes)
Write-Output "icon.ico -> $($sizes.Count) sizes"

$logo.Dispose()
Write-Output "done"
