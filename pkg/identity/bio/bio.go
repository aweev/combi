package bio

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"

	"golang.org/x/image/draw"
)

var (
	ErrUnsupportedImage = errors.New("bio: unsupported image format")
	ErrEmptyImage       = errors.New("bio: image data is empty")
)

type Options struct {
	MaxWidth   int
	Quality    int
	TargetSize int
	Grayscale  bool
}

func Process(input io.Reader, opts Options) ([]byte, error) {
	if opts.MaxWidth <= 0 {
		opts.MaxWidth = 720
	}
	if opts.Quality <= 0 || opts.Quality > 100 {
		opts.Quality = 80
	}
	if opts.TargetSize <= 0 {
		opts.TargetSize = 51200
	}

	img, format, err := image.Decode(input)
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, ErrEmptyImage
	}

	scaled := scaleToWidth(img, opts.MaxWidth)
	if opts.Grayscale {
		scaled = toGray(scaled)
	}

	buf := new(bytes.Buffer)
	quality := opts.Quality
	for {
		buf.Reset()
		if err := jpeg.Encode(buf, scaled, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
		if buf.Len() <= opts.TargetSize || quality <= 30 {
			break
		}
		quality -= 10
	}

	if format == "png" {
		return buf.Bytes(), nil
	}
	return buf.Bytes(), nil
}

func scaleToWidth(src image.Image, maxWidth int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	if width <= maxWidth {
		return src
	}
	ratio := float64(maxWidth) / float64(width)
	height := int(math.Round(float64(bounds.Dy()) * ratio))

	dst := image.NewRGBA(image.Rect(0, 0, maxWidth, height))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}

func toGray(src image.Image) image.Image {
	bounds := src.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, color.GrayModel.Convert(src.At(x, y)))
		}
	}
	return gray
}

func AverageLuminance(img image.Image) float64 {
	bounds := img.Bounds()
	if bounds.Empty() {
		return 0
	}
	var sum float64
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 5 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 5 {
			r, g, b, _ := color.RGBAModel.Convert(img.At(x, y)).RGBA()
			sum += float64(r+g+b) / 3.0
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count*65535) * 100
}

func IsTooDark(img image.Image) bool {
	return AverageLuminance(img) < 20
}

func IsTooBright(img image.Image) bool {
	return AverageLuminance(img) > 90
}

func DecodeImage(input io.Reader) (image.Image, string, error) {
	img, format, err := image.Decode(input)
	if err != nil {
		return nil, "", err
	}
	return img, format, nil
}

func EncodeJPEG(img image.Image, quality int) ([]byte, error) {
	if quality < 1 || quality > 100 {
		quality = 80
	}
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func EncodePNG(img image.Image) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecodePNG(input io.Reader) (image.Image, error) {
	img, err := png.Decode(input)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func DecodeJPEG(input io.Reader) (image.Image, error) {
	img, err := jpeg.Decode(input)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func CopyImage(src image.Image) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
	return dst
}
