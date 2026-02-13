package imageutil

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func createTestPNG(t *testing.T, w, h int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.White)
		}
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPrepareForLLM_CropsAndScales4K(t *testing.T) {
	path := createTestPNG(t, 3840, 2160)

	data, mime, err := PrepareForLLM(path)
	if err != nil {
		t.Fatalf("PrepareForLLM failed: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JPEG data")
	}

	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode output JPEG: %v", err)
	}

	b := img.Bounds()
	if b.Dx() > maxWidth || b.Dy() > maxHeight {
		t.Errorf("output too large: %dx%d", b.Dx(), b.Dy())
	}
}

func TestPrepareForLLM_SmallImageNoUpscale(t *testing.T) {
	path := createTestPNG(t, 800, 600)

	data, _, err := PrepareForLLM(path)
	if err != nil {
		t.Fatalf("PrepareForLLM failed: %v", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode output JPEG: %v", err)
	}

	b := img.Bounds()
	// 800*0.6 = 480, 600*0.6 = 360 — both under 1920x1080, no scaling
	if b.Dx() != 480 || b.Dy() != 360 {
		t.Errorf("expected 480x360, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestPrepareForLLM_NonexistentFile(t *testing.T) {
	_, _, err := PrepareForLLM("/nonexistent/file.png")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestCropCenter(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	cropped := cropCenter(img)
	b := cropped.Bounds()

	expectedW := 600
	expectedH := 300
	if b.Dx() != expectedW || b.Dy() != expectedH {
		t.Errorf("expected %dx%d, got %dx%d", expectedW, expectedH, b.Dx(), b.Dy())
	}
}

func TestScaleToFit_NoUpscale(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	result := scaleToFit(img)
	b := result.Bounds()
	if b.Dx() != 100 || b.Dy() != 100 {
		t.Errorf("should not upscale, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestScaleToFit_DownscalesLargeImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3840, 2160))
	result := scaleToFit(img)
	b := result.Bounds()
	if b.Dx() > maxWidth || b.Dy() > maxHeight {
		t.Errorf("should fit within %dx%d, got %dx%d", maxWidth, maxHeight, b.Dx(), b.Dy())
	}
}
