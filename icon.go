package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed claude_icon.png
var claudeIconPNG []byte

//go:embed claude_symbol.png
var claudeSymbolPNG []byte

var (
	symbolImage   *image.RGBA
	trayIconData  []byte
	errorIconData []byte
	cachedFont    *opentype.Font
	fontOnce      sync.Once
	iconMu        sync.Mutex
	lastIconPct   int = -1
	lastIconBuf   []byte
)

func initIcons() {
	symbolImage = loadImage(claudeSymbolPNG, claudeIconPNG, 128)
	trayIconData = prepareTrayIcon()
	errorIconData = prepareErrorIcon()
}

func loadImage(primary, fallback []byte, size int) *image.RGBA {
	img, err := png.Decode(bytes.NewReader(primary))
	if err != nil {
		img, err = png.Decode(bytes.NewReader(fallback))
		if err != nil {
			return fallbackCircle(size)
		}
	}
	return bilinearResize(img, size)
}

func prepareTrayIcon() []byte {
	img, err := png.Decode(bytes.NewReader(claudeIconPNG))
	if err != nil {
		return iconToBytes(fallbackCircle(64))
	}
	resized := bilinearResize(img, 64)
	return iconToBytes(resized)
}

func prepareErrorIcon() []byte {
	img, err := png.Decode(bytes.NewReader(claudeIconPNG))
	if err != nil {
		return iconToBytes(fallbackCircle(64))
	}
	resized := bilinearResize(img, 64)
	// Desaturate to gray
	bounds := resized.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := resized.At(x, y).RGBA()
			gray := uint8((r*299 + g*587 + b*114) / 1000 / 256)
			resized.SetRGBA(x, y, color.RGBA{gray, gray, gray, uint8(a >> 8)})
		}
	}
	return iconToBytes(resized)
}

func fallbackCircle(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size)/2, float64(size)/2
	r := float64(size)/2 - 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, color.RGBA{0xE8, 0x73, 0x4A, 0xFF})
			}
		}
	}
	return img
}

func bilinearResize(src image.Image, size int) *image.RGBA {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	scaleX := float64(srcW) / float64(size)
	scaleY := float64(srcH) / float64(size)

	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size; dx++ {
			sx := float64(dx)*scaleX + float64(bounds.Min.X)
			sy := float64(dy)*scaleY + float64(bounds.Min.Y)

			x0 := int(math.Floor(sx))
			y0 := int(math.Floor(sy))
			x1 := x0 + 1
			y1 := y0 + 1
			fx := sx - float64(x0)
			fy := sy - float64(y0)

			if x1 >= bounds.Max.X {
				x1 = bounds.Max.X - 1
			}
			if y1 >= bounds.Max.Y {
				y1 = bounds.Max.Y - 1
			}

			r00, g00, b00, a00 := src.At(x0, y0).RGBA()
			r10, g10, b10, a10 := src.At(x1, y0).RGBA()
			r01, g01, b01, a01 := src.At(x0, y1).RGBA()
			r11, g11, b11, a11 := src.At(x1, y1).RGBA()

			lerp := func(v00, v10, v01, v11 uint32) uint8 {
				top := float64(v00)*(1-fx) + float64(v10)*fx
				bot := float64(v01)*(1-fx) + float64(v11)*fx
				return uint8((top*(1-fy) + bot*fy) / 256)
			}

			dst.SetRGBA(dx, dy, color.RGBA{
				R: lerp(r00, r10, r01, r11),
				G: lerp(g00, g10, g01, g11),
				B: lerp(b00, b10, b01, b11),
				A: lerp(a00, a10, a01, a11),
			})
		}
	}
	return dst
}


var systemFontPaths = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
	"/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/TTF/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf",
}

func getCachedFont() *opentype.Font {
	fontOnce.Do(func() {
		for _, path := range systemFontPaths {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			f, err := opentype.Parse(data)
			if err != nil {
				continue
			}
			cachedFont = f
			return
		}
	})
	return cachedFont
}

func makeFace(size float64) font.Face {
	f := getCachedFont()
	if f == nil {
		return nil
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil
	}
	return face
}

func renderLinuxIcon(pct float64) []byte {
	iconMu.Lock()
	defer iconMu.Unlock()

	rounded := int(pct + 0.5)
	if rounded == lastIconPct && lastIconBuf != nil {
		return lastIconBuf
	}

	text := fmt.Sprintf("%d%%", rounded)
	height := 64
	fontSize := 56.0

	face := makeFace(fontSize)
	if face == nil {
		return trayIconData
	}
	defer face.Close()

	tw := font.MeasureString(face, text).Ceil()
	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	textH := (metrics.Ascent + metrics.Descent).Ceil()

	width := tw + 8
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	tx := (width - tw) / 2
	ty := (height-textH)/2 + ascent

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot:  fixed.P(tx, ty),
	}
	d.DrawString(text)

	buf := iconToBytes(img)
	lastIconPct = rounded
	lastIconBuf = buf
	return buf
}

var iconTempDir string

func getIconTempDir() string {
	if iconTempDir == "" {
		dir := filepath.Join(os.TempDir(), fmt.Sprintf("claude-meter-%d", os.Getpid()))
		os.MkdirAll(dir, 0755)
		iconTempDir = dir
	}
	return iconTempDir
}

func makeIcon(pct float64) []byte {
	if runtime.GOOS == "darwin" {
		return iconToBytes(symbolImage)
	}
	return renderLinuxIcon(pct)
}

// writeIconFile writes the Claude icon (orange bg + white sparkle) to a temp file.
// Returns (iconName, themePath) for systray.SetIconByName on Linux.
func writeIconFile(pct float64) (string, string) {
	dir := getIconTempDir()
	iconName := "claude-meter-icon"
	path := filepath.Join(dir, iconName+".png")
	os.WriteFile(path, trayIconData, 0644)
	return iconName, dir
}

func writeErrorIconFile() (string, string) {
	dir := getIconTempDir()
	iconName := "claude-meter-error"
	path := filepath.Join(dir, iconName+".png")
	os.WriteFile(path, errorIconData, 0644)
	return iconName, dir
}

func iconToBytes(img image.Image) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
