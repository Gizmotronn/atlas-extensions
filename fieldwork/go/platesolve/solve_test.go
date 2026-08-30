package platesolve

import (
	"image"
	"image/color"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/signal-k/fieldwork/starcat"
)

// projectSyntheticPhoto builds a fake night-sky photo by taking real
// catalog stars within fovDeg of a known true pointing, projecting them
// through a known WCS (rotation, scale, translation), and drawing them as
// bright blobs. This is the ground truth Solve is asked to recover.
func projectSyntheticPhoto(t *testing.T, lat, lon float64, when time.Time, headingDeg, pitchDeg, fovDeg float64, imgW, imgH int, arcsecPerPixel, rollDeg float64) (image.Image, int) {
	t.Helper()

	centerRA, centerDec := equatorialFromHorizontal(pitchDeg, headingDeg, when, lat, lon)

	stars := starcat.VisibleInDirection(lat, lon, when, -2, headingDeg, pitchDeg, fovDeg)
	sort.Slice(stars, func(i, j int) bool { return stars[i].Mag < stars[j].Mag })

	scale := 206265 / arcsecPerPixel // pixels per radian
	roll := rollDeg * deg2rad
	cosR, sinR := math.Cos(roll), math.Sin(roll)

	img := image.NewGray(image.Rect(0, 0, imgW, imgH))
	for y := range imgH {
		for x := range imgW {
			img.SetGray(x, y, color.Gray{Y: 6})
		}
	}

	drawn := 0
	for _, s := range stars {
		x, y, ok := gnomonicProject(s.RADeg, s.DecDeg, centerRA, centerDec)
		if !ok {
			continue
		}
		// apply rotation + scale + translate to image center
		px := (x*cosR-y*sinR)*scale + float64(imgW)/2
		py := (x*sinR+y*cosR)*scale + float64(imgH)/2
		if px < 4 || px > float64(imgW)-4 || py < 4 || py > float64(imgH)-4 {
			continue
		}

		peak := 250 - clamp((s.Mag+2)*20, 0, 200)
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

	return img, drawn
}

func TestSolveRecoversKnownPointingWithHeadingHint(t *testing.T) {
	lat, lon := 40.0, -105.0
	when := time.Date(2026, 3, 20, 6, 0, 0, 0, time.UTC)
	heading, pitch := 90.0, 40.0 // due east, 40 degrees up
	imgW, imgH := 800, 600
	arcsecPerPixel := 120.0 // ~ a 33-degree-diagonal FOV over an 800x600 frame
	roll := 12.0
	assumedFOV := 35.0 // caller's rough FOV estimate, close to (but not exactly) the truth above

	starRadius := assumedFOV/2 + 6
	img, drawn := projectSyntheticPhoto(t, lat, lon, when, heading, pitch, starRadius, imgW, imgH, arcsecPerPixel, roll)
	if drawn < minMatchedStars {
		t.Fatalf("synthetic fixture too sparse (%d stars drawn); adjust test setup", drawn)
	}

	detections := DetectStars(img)
	if len(detections) < minMatchedStars {
		t.Fatalf("expected enough detections to solve, got %d", len(detections))
	}

	h := heading
	wcs, err := Solve(detections, imgW, imgH, CandidateSky{
		LatDeg: lat, LonDeg: lon, Time: when, HeadingDeg: &h, AssumedFOVDeg: assumedFOV,
	})
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	trueCenterRA, trueCenterDec := equatorialFromHorizontal(pitch, heading, when, lat, lon)
	sep := angularSeparationDeg(wcs.CenterRADeg, wcs.CenterDecDeg, trueCenterRA, trueCenterDec)
	if sep > 1.0 {
		t.Errorf("solved center too far from truth: %.3f deg off (got RA=%.2f Dec=%.2f, want RA=%.2f Dec=%.2f)",
			sep, wcs.CenterRADeg, wcs.CenterDecDeg, trueCenterRA, trueCenterDec)
	}

	scaleErr := math.Abs(wcs.ArcsecPerPixel-arcsecPerPixel) / arcsecPerPixel
	if scaleErr > 0.15 {
		t.Errorf("solved scale too far from truth: got %.2f arcsec/px, want ~%.2f", wcs.ArcsecPerPixel, arcsecPerPixel)
	}

	if wcs.MatchedStars < minMatchedStars {
		t.Errorf("expected at least %d matched stars, got %d", minMatchedStars, wcs.MatchedStars)
	}

	recall := float64(wcs.MatchedStars) / float64(drawn)
	if recall < 0.51 {
		t.Errorf("recall %.2f below the 51%% target (%d/%d matched)", recall, wcs.MatchedStars, drawn)
	}
}

func TestSolveBlindSearchFindsOnGridPointing(t *testing.T) {
	lat, lon := 40.0, -105.0
	when := time.Date(2026, 3, 20, 6, 0, 0, 0, time.UTC)
	// A direction that lands exactly on the blind-search grid (az step 30,
	// alt in {20,40,60,80}) so this is a deterministic, fast check that the
	// no-heading path can still solve rather than a test of grid density.
	heading, pitch := 90.0, 40.0
	imgW, imgH := 640, 480
	arcsecPerPixel := 150.0
	assumedFOV := 35.0

	starRadius := assumedFOV/2 + 6
	img, drawn := projectSyntheticPhoto(t, lat, lon, when, heading, pitch, starRadius, imgW, imgH, arcsecPerPixel, 0)
	if drawn < minMatchedStars {
		t.Fatalf("synthetic fixture too sparse (%d stars drawn)", drawn)
	}

	detections := DetectStars(img)
	wcs, err := Solve(detections, imgW, imgH, CandidateSky{LatDeg: lat, LonDeg: lon, Time: when, AssumedFOVDeg: assumedFOV})
	if err != nil {
		t.Fatalf("blind Solve failed: %v", err)
	}

	trueCenterRA, trueCenterDec := equatorialFromHorizontal(pitch, heading, when, lat, lon)
	sep := angularSeparationDeg(wcs.CenterRADeg, wcs.CenterDecDeg, trueCenterRA, trueCenterDec)
	if sep > 2.0 {
		t.Errorf("blind solve center too far from truth: %.3f deg off", sep)
	}
}

func angularSeparationDeg(ra1, dec1, ra2, dec2 float64) float64 {
	r1, d1 := ra1*deg2rad, dec1*deg2rad
	r2, d2 := ra2*deg2rad, dec2*deg2rad
	cosC := math.Sin(d1)*math.Sin(d2) + math.Cos(d1)*math.Cos(d2)*math.Cos(r1-r2)
	return math.Acos(clamp(cosC, -1, 1)) * rad2deg
}
