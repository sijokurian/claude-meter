package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"runtime"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed claude_icon.png
var claudeIconPNG []byte

//go:embed claude_symbol.png
var claudeSymbolPNG []byte

var (
	symbolImage *image.RGBA
	whiteSymbol *image.RGBA
)

func initIcons() {
	symbolImage = loadSymbol()
	whiteSymbol = recolorWhite(symbolImage)
}

func loadSymbol() *image.RGBA {
	img, err := png.Decode(bytes.NewReader(claudeSymbolPNG))
	if err != nil {
		img, err = png.Decode(bytes.NewReader(claudeIconPNG))
		if err != nil {
			return fallbackCircle(44)
		}
	}
	return resizeToRGBA(img, 44)
}

func fallbackCircle(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	cx, cy, r := size/2, size/2, size/2-2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, color.Black)
			}
		}
	}
	return img
}

func resizeToRGBA(src image.Image, size int) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	sx := float64(bounds.Dx()) / float64(size)
	sy := float64(bounds.Dy()) / float64(size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			srcX := bounds.Min.X + int(float64(x)*sx)
			srcY := bounds.Min.Y + int(float64(y)*sy)
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func recolorWhite(img *image.RGBA) *image.RGBA {
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			out.Set(x, y, color.NRGBA{255, 255, 255, uint8(a >> 8)})
		}
	}
	return out
}

var systemFontPaths = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
	"/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/TTF/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf",
}

func loadSystemFont(size float64) font.Face {
	for _, path := range systemFontPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		f, err := opentype.Parse(data)
		if err != nil {
			continue
		}
		face, err := opentype.NewFace(f, &opentype.FaceOptions{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			continue
		}
		return face
	}
	return nil
}

func renderLinuxIcon(pct float64) []byte {
	text := fmt.Sprintf("%d%%", int(pct+0.5))
	height := 44
	symW := 38
	pad := 6

	face := loadSystemFont(28)
	if face == nil {
		return iconToBytes(whiteSymbol)
	}
	defer face.Close()

	tw := font.MeasureString(face, text).Ceil()
	metrics := face.Metrics()

	width := symW + pad + tw + 4
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	sym := resizeToRGBA(whiteSymbol, symW)
	yOff := (height - symW) / 2
	draw.Draw(img, image.Rect(0, yOff, symW, yOff+symW), sym, image.Point{}, draw.Over)

	ascent := metrics.Ascent.Ceil()
	ty := (height+ascent)/2 - 2
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot:  fixed.P(symW+pad, ty),
	}
	d.DrawString(text)

	return iconToBytes(img)
}

func makeIcon(pct float64) []byte {
	if runtime.GOOS == "darwin" {
		return iconToBytes(symbolImage)
	}
	return renderLinuxIcon(pct)
}

func iconToBytes(img image.Image) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
