package bio_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/combi/pkg/identity/bio"
)

func TestProcessResizesAndEncodes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 1200; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 128, 255})
		}
	}

	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}

	out, err := bio.Process(bytes.NewReader(buf.Bytes()), bio.Options{MaxWidth: 600, Quality: 70, TargetSize: 20000, Grayscale: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("expected output bytes")
	}
}

func TestAverageLuminance(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{10, 10, 10, 255})
		}
	}
	if !bio.IsTooDark(img) {
		t.Fatal("expected too dark image")
	}
}
