package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestAccentTint(t *testing.T) {
	a := color.RGBA{0, 0, 255, 255}
	tinted := tintPalette(gameboy, a)
	if len(tinted) != len(gameboy) {
		t.Fatal("length changed")
	}
	if tinted[0] == gameboy[0] {
		t.Fatal("no tint applied")
	}
	// blue channel must move toward 255
	if tinted[0].B <= gameboy[0].B {
		t.Fatalf("blue did not increase: %v -> %v", gameboy[0], tinted[0])
	}
	if _, err := parseHex("zzzzzz"); err == nil {
		t.Fatal("expected hex error")
	}
}

func TestPixelate(t *testing.T) {
	// 64x64 gradient, no network needed.
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			src.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 4), 128, 255})
		}
	}
	f, _ := os.Create("/tmp/px-in.png")
	png.Encode(f, src)
	f.Close()

	out := pixelate(src, 8, gameboy)
	if out.Bounds().Dx() != 64 || out.Bounds().Dy() != 64 {
		t.Fatalf("bad dims: %v", out.Bounds())
	}
	// every pixel must be a palette color
	allowed := map[color.RGBA]bool{}
	for _, p := range gameboy {
		allowed[p] = true
	}
	for y := 0; y < 64; y += 8 {
		for x := 0; x < 64; x += 8 {
			c := out.RGBAAt(x, y)
			if !allowed[c] {
				t.Fatalf("non-palette color at %d,%d: %v", x, y, c)
			}
			// block uniformity (nearest-neighbor upscale)
			if out.RGBAAt(x, y) != out.RGBAAt(x+7, y+7) {
				t.Fatalf("block not uniform at %d,%d", x, y)
			}
		}
	}
}
