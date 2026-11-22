// Command pixelate converts an image to retro pixel art.
// 3-step pipeline: nearest-neighbor downsample, fixed-palette quantize,
// nearest-neighbor upscale. Zero dependencies (stdlib only).
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	_ "image/jpeg"
	"math"
	"os"
	"strconv"
	"strings"
)

var gameboy = []color.RGBA{
	{15, 56, 15, 255},
	{48, 98, 48, 255},
	{139, 172, 15, 255},
	{155, 188, 15, 255},
}

// tintPalette shifts every palette color 30% toward the accent color,
// so pixel art follows the system accent.
func tintPalette(pal []color.RGBA, accent color.RGBA) []color.RGBA {
	out := make([]color.RGBA, len(pal))
	for i, p := range pal {
		out[i] = color.RGBA{
			uint8((int(p.R)*7 + int(accent.R)*3) / 10),
			uint8((int(p.G)*7 + int(accent.G)*3) / 10),
			uint8((int(p.B)*7 + int(accent.B)*3) / 10),
			255,
		}
	}
	return out
}

func parseHex(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return color.RGBA{}, fmt.Errorf("want RRGGBB, got %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
}

func closestColor(c color.Color, pal []color.RGBA) color.RGBA {
	r1, g1, b1, _ := c.RGBA()
	minDist := math.MaxFloat64
	var best color.RGBA
	for _, p := range pal {
		r2, g2, b2, _ := p.RGBA()
		dist := math.Pow(float64(r1>>8)-float64(r2>>8), 2) +
			math.Pow(float64(g1>>8)-float64(g2>>8), 2) +
			math.Pow(float64(b1>>8)-float64(b2>>8), 2)
		if dist < minDist {
			minDist = dist
			best = p
		}
	}
	return best
}

func pixelate(src image.Image, pixelSize int, pal []color.RGBA) *image.RGBA {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	smallW, smallH := w/pixelSize, h/pixelSize
	if smallW < 1 {
		smallW = 1
	}
	if smallH < 1 {
		smallH = 1
	}
	small := image.NewRGBA(image.Rect(0, 0, smallW, smallH))
	for y := 0; y < smallH; y++ {
		for x := 0; x < smallW; x++ {
			small.Set(x, y, closestColor(src.At(bounds.Min.X+x*pixelSize, bounds.Min.Y+y*pixelSize), pal))
		}
	}
	out := image.NewRGBA(image.Rect(0, 0, smallW*pixelSize, smallH*pixelSize))
	for y := 0; y < smallH*pixelSize; y++ {
		for x := 0; x < smallW*pixelSize; x++ {
			out.Set(x, y, small.At(x/pixelSize, y/pixelSize))
		}
	}
	return out
}

func main() {
	in := flag.String("in", "input.png", "input image")
	out := flag.String("out", "pixel_art.png", "output image")
	px := flag.Int("px", 8, "pixel block size")
	accent := flag.String("accent", "", "tint palette toward accent RRGGBB (system color)")
	flag.Parse()

	pal := gameboy
	if *accent != "" {
		a, err := parseHex(*accent)
		if err != nil {
			fatal(err)
		}
		pal = tintPalette(gameboy, a)
	}

	f, err := os.Open(*in)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		fatal(err)
	}
	o, err := os.Create(*out)
	if err != nil {
		fatal(err)
	}
	defer o.Close()
	if err := png.Encode(o, pixelate(src, *px, pal)); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	os.Stderr.WriteString("pixelate: " + err.Error() + "\n")
	os.Exit(1)
}
