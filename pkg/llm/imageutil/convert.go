package imageutil

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // Register PNG decoder
	"os"

	"golang.org/x/image/draw"
)

const (
	maxWidth    = 1920
	maxHeight   = 1080
	cropMargin  = 0.2 // Remove 20% from each edge
	jpegQuality = 85
)

// PrepareForLLM loads an image from disk, crops the center 60%,
// scales it to fit within 1920x1080, and returns JPEG bytes.
// The original file on disk is not modified.
func PrepareForLLM(imagePath string) (data []byte, mimeType string, err error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	// 1. Crop to center 60%
	cropped := cropCenter(src)

	// 2. Scale down to fit within 1920x1080 (no upscaling)
	scaled := scaleToFit(cropped)

	// 3. Encode as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, "", fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), "image/jpeg", nil
}

// cropCenter returns a sub-image of the center 60% (20% removed from each edge).
func cropCenter(img image.Image) image.Image {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()

	marginX := int(float64(w) * cropMargin)
	marginY := int(float64(h) * cropMargin)

	cropRect := image.Rect(
		b.Min.X+marginX,
		b.Min.Y+marginY,
		b.Max.X-marginX,
		b.Max.Y-marginY,
	)

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}

	if si, ok := img.(subImager); ok {
		return si.SubImage(cropRect)
	}

	// Fallback: copy pixels (shouldn't happen with standard decoders)
	dst := image.NewRGBA(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
	draw.Copy(dst, image.Point{}, img, cropRect, draw.Src, nil)
	return dst
}

// scaleToFit scales the image to fit within maxWidth x maxHeight, preserving aspect ratio.
// Does not upscale.
func scaleToFit(img image.Image) image.Image {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()

	if w <= maxWidth && h <= maxHeight {
		return img
	}

	ratio := float64(maxWidth) / float64(w)
	if rh := float64(maxHeight) / float64(h); rh < ratio {
		ratio = rh
	}

	newW := int(float64(w) * ratio)
	newH := int(float64(h) * ratio)

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}
