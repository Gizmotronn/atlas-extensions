package api

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"math"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/signal-k/fieldwork/starcat"
)

// buildSyntheticSkyJPEG projects real catalog stars visible from (lat, lon)
// at `when`, in the direction (headingDeg, pitchDeg), into a JPEG-encoded
// image at the given plate scale — the same technique platesolve's own
// tests use, duplicated here (rather than imported) since it's test-only
// and small. This is what lets resolvePhotoScan be exercised end-to-end
// without a real photo or a running PocketBase.
func buildSyntheticSkyJPEG(t *testing.T, lat, lon float64, when time.Time, headingDeg, pitchDeg, fovDeg float64, imgW, imgH int, arcsecPerPixel float64) []byte {
	t.Helper()

	deg2rad := math.Pi / 180
	stars := starcat.VisibleInDirection(lat, lon, when, -2, headingDeg, pitchDeg, fovDeg)
	sort.Slice(stars, func(i, j int) bool { return stars[i].Mag < stars[j].Mag })

	// Reproduce identify.Horizontal's underlying equatorial-from-horizontal
	// conversion just enough to get a projection center; platesolve does
	// the equivalent internally when solving, so this only needs to be
	// good enough to place stars in the frame, not bit-exact.
	centerRA, centerDec := equatorialFromHorizontalForTest(headingDeg, pitchDeg, when, lat, lon)

	scale := 206265 / arcsecPerPixel
	img := image.NewGray(image.Rect(0, 0, imgW, imgH))
	for y := range imgH {
		for x := range imgW {
			img.SetGray(x, y, color.Gray{Y: 6})
		}
	}

	drawn := 0
	for _, s := range stars {
		ra, dec := s.RADeg*deg2rad, s.DecDeg*deg2rad
		ra0, dec0 := centerRA*deg2rad, centerDec*deg2rad
		cosC := math.Sin(dec0)*math.Sin(dec) + math.Cos(dec0)*math.Cos(dec)*math.Cos(ra-ra0)
		if cosC <= 0.01 {
			continue
		}
		x := math.Cos(dec) * math.Sin(ra-ra0) / cosC
		y := (math.Cos(dec0)*math.Sin(dec) - math.Sin(dec0)*math.Cos(dec)*math.Cos(ra-ra0)) / cosC

		px := x*scale + float64(imgW)/2
		py := y*scale + float64(imgH)/2
		if px < 4 || px > float64(imgW)-4 || py < 4 || py > float64(imgH)-4 {
			continue
		}

		peak := 250 - clampTest((s.Mag+2)*20, 0, 200)
		for dy := -3; dy <= 3; dy++ {
			for dx := -3; dx <= 3; dx++ {
				ix, iy := int(px)+dx, int(py)+dy
				if ix < 0 || ix >= imgW || iy < 0 || iy >= imgH {
					continue
				}
				r2 := (float64(ix)-px)*(float64(ix)-px) + (float64(iy)-py)*(float64(iy)-py)
				v := peak * math.Exp(-r2/(2*1.1*1.1))
				existing := img.GrayAt(ix, iy).Y
				if uint8(v) > existing {
					img.SetGray(ix, iy, color.Gray{Y: uint8(v) + 6})
				}
			}
		}
		drawn++
		if drawn >= 60 {
			break
		}
	}
	if drawn < 10 {
		t.Fatalf("synthetic fixture too sparse (%d stars drawn)", drawn)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encoding synthetic JPEG failed: %v", err)
	}
	return buf.Bytes()
}

func clampTest(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// equatorialFromHorizontalForTest mirrors platesolve's unexported
// equatorialFromHorizontal (same formula, duplicated for this
// package's own test fixture — see platesolve/geometry.go for the
// canonical version and its derivation).
func equatorialFromHorizontalForTest(azDeg, altDeg float64, t time.Time, latDeg, lonDeg float64) (raDeg, decDeg float64) {
	deg2rad := math.Pi / 180
	rad2deg := 180 / math.Pi
	alt, az := altDeg*deg2rad, azDeg*deg2rad
	lat := latDeg * deg2rad

	sinDec := math.Sin(lat)*math.Sin(alt) + math.Cos(lat)*math.Cos(alt)*math.Cos(az)
	dec := math.Asin(clampTest(sinDec, -1, 1))
	cosHA := (math.Sin(alt) - math.Sin(lat)*math.Sin(dec)) / (math.Cos(lat) * math.Cos(dec))
	ha := math.Acos(clampTest(cosHA, -1, 1))
	if math.Sin(az) > 0 {
		ha = 2*math.Pi - ha
	}

	d := t.UTC()
	y, m := int(d.Year()), int(d.Month())
	day := float64(d.Day()) + (float64(d.Hour())+float64(d.Minute())/60+float64(d.Second())/3600)/24
	if m <= 2 {
		y--
		m += 12
	}
	a := y / 100
	b := 2 - a + a/4
	jd := math.Floor(365.25*float64(y+4716)) + math.Floor(30.6001*float64(m+1)) + day + float64(b) - 1524.5
	gmst := 280.46061837 + 360.98564736629*(jd-2451545.0)
	lst := math.Mod(gmst+lonDeg, 360)
	if lst < 0 {
		lst += 360
	}

	ra := math.Mod(lst-ha*rad2deg, 360)
	if ra < 0 {
		ra += 360
	}
	return ra, dec * rad2deg
}

func TestResolvePhotoScanPlateSolves(t *testing.T) {
	lat, lon := 40.0, -105.0
	when := time.Date(2026, 3, 20, 6, 0, 0, 0, time.UTC)
	heading, pitch := 90.0, 40.0
	assumedFOV := 35.0
	starRadius := assumedFOV/2 + 6
	imgW, imgH := 800, 600
	arcsecPerPixel := 120.0

	jpegBytes := buildSyntheticSkyJPEG(t, lat, lon, when, heading, pitch, starRadius, imgW, imgH, arcsecPerPixel)

	s := &Server{Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}
	req := scanRequest{
		Latitude: lat, Longitude: lon, InputMode: "photo",
	}
	r := httptest.NewRequest("POST", "/v1/scans", nil)

	objects, method := s.resolvePhotoScan(r, jpegBytes, req, when)
	if method != "plate_solve" {
		t.Fatalf("expected plate_solve, got %q (objects=%d)", method, len(objects))
	}
	if len(objects) == 0 {
		t.Fatal("expected at least one identified object from a solved photo")
	}
	for _, o := range objects {
		if o.Type != "star" && o.Type != "planet" && o.Type != "moon" && o.Type != "deep_sky" {
			t.Errorf("unexpected object type %q", o.Type)
		}
	}

	starCount := 0
	for _, o := range objects {
		if o.Type == "star" {
			starCount++
		}
	}
	if starCount < 6 {
		t.Errorf("expected several matched stars in the result, got %d", starCount)
	}
}

func TestResolvePhotoScanFallsBackWithoutStars(t *testing.T) {
	// A blank frame has nothing to plate-solve — this should degrade to
	// the omnidirectional path rather than error.
	img := image.NewGray(image.Rect(0, 0, 100, 100))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encoding blank JPEG failed: %v", err)
	}

	s := &Server{Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}
	req := scanRequest{Latitude: 40.0, Longitude: -105.0, InputMode: "photo"}
	r := httptest.NewRequest("POST", "/v1/scans", nil)

	objects, method := s.resolvePhotoScan(r, buf.Bytes(), req, time.Date(2026, 3, 20, 6, 0, 0, 0, time.UTC))
	if method != "omnidirectional" {
		t.Fatalf("expected omnidirectional fallback, got %q", method)
	}
	if len(objects) == 0 {
		t.Fatal("expected the omnidirectional fallback to still return something (Sun/Moon/planets/catalog)")
	}
}
