package platesolve

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// synthImage draws a dark background with Gaussian-ish bright blobs at the
// given centers, simulating stars in a night-sky photo.
func synthImage(w, h int, centers [][2]float64, peak uint8, sigma float64) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetGray(x, y, color.Gray{Y: 8}) // faint uniform background
		}
	}
	for _, c := range centers {
		cx, cy := c[0], c[1]
		for dy := -6; dy <= 6; dy++ {
			for dx := -6; dx <= 6; dx++ {
				x, y := int(cx)+dx, int(cy)+dy
				if x < 0 || x >= w || y < 0 || y >= h {
					continue
				}
				r2 := (float64(x)-cx)*(float64(x)-cx) + (float64(y)-cy)*(float64(y)-cy)
				v := float64(peak) * math.Exp(-r2/(2*sigma*sigma))
				existing := img.GrayAt(x, y).Y
				if uint8(v) > existing {
					img.SetGray(x, y, color.Gray{Y: uint8(v) + 8})
				}
			}
		}
	}
	return img
}

func TestDetectStarsFindsKnownCenters(t *testing.T) {
	centers := [][2]float64{{50, 50}, {150, 80}, {200, 200}}
	img := synthImage(256, 256, centers, 220, 1.2)

	detections := DetectStars(img)
	if len(detections) < len(centers) {
		t.Fatalf("expected at least %d detections, got %d", len(centers), len(detections))
	}

	for _, want := range centers {
		found := false
		for _, d := range detections {
			if math.Hypot(d.X-want[0], d.Y-want[1]) < 0.75 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no detection near expected center %v", want)
		}
	}
}

func TestDetectStarsRejectsUniformNoise(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.SetGray(x, y, color.Gray{Y: 10})
		}
	}
	detections := DetectStars(img)
	if len(detections) != 0 {
		t.Fatalf("expected no detections on a uniform image, got %d", len(detections))
	}
}

func TestDetectStarsRejectsOversizedBlob(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			img.SetGray(x, y, color.Gray{Y: 10})
		}
	}
	// A big bright rectangle covering most of the frame — e.g. cloud glow,
	// not a point star.
	for y := 10; y < 90; y++ {
		for x := 10; x < 90; x++ {
			img.SetGray(x, y, color.Gray{Y: 250})
		}
	}
	detections := DetectStars(img)
	if len(detections) != 0 {
		t.Fatalf("expected the oversized blob to be rejected, got %d detections", len(detections))
	}
}
