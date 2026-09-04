package photometry

import (
	"math"
	"testing"

	"github.com/signal-k/fieldwork/platesolve"
	"github.com/signal-k/fieldwork/starcat"
)

func TestProjectToPixelCenterIsImageCenter(t *testing.T) {
	wcs := &platesolve.WCS{CenterRADeg: 83.0, CenterDecDeg: -5.0, RollDeg: 27, ArcsecPerPixel: 12}
	star := starcat.Star{RADeg: 83.0, DecDeg: -5.0, Mag: 1.0}

	x, y, ok := projectToPixel(wcs, 1000, 800, star)
	if !ok {
		t.Fatal("expected projection to succeed for the center star")
	}
	if math.Abs(x-500) > 1e-6 || math.Abs(y-400) > 1e-6 {
		t.Fatalf("star at WCS center should land exactly at image center, got (%v, %v)", x, y)
	}
}

func TestProjectToPixelScaleAndRoll(t *testing.T) {
	// No roll: a star offset purely in RA (at the same declination as
	// center) should displace along the pixel X axis in proportion to
	// ArcsecPerPixel, with no Y displacement.
	arcsecPerPixel := 20.0
	wcs := &platesolve.WCS{CenterRADeg: 100.0, CenterDecDeg: 0.0, RollDeg: 0, ArcsecPerPixel: arcsecPerPixel}
	offsetDeg := 0.01 // small enough for the small-angle approximation below
	star := starcat.Star{RADeg: 100.0 + offsetDeg, DecDeg: 0.0, Mag: 2.0}

	x, y, ok := projectToPixel(wcs, 1000, 1000, star)
	if !ok {
		t.Fatal("expected projection to succeed")
	}
	if math.Abs(y-500) > 0.5 {
		t.Fatalf("a star offset purely in RA at roll=0 should not move in Y, got y=%v", y)
	}
	expectedArcsec := offsetDeg * 3600
	expectedPixels := expectedArcsec / arcsecPerPixel
	gotOffset := x - 500
	if math.Abs(gotOffset-expectedPixels) > 0.5 {
		t.Fatalf("expected roughly %v px offset in X, got %v", expectedPixels, gotOffset)
	}

	// A 90-degree roll should rotate that same RA-offset star to displace in
	// Y instead of X.
	wcsRolled := &platesolve.WCS{CenterRADeg: 100.0, CenterDecDeg: 0.0, RollDeg: 90, ArcsecPerPixel: arcsecPerPixel}
	rx, ry, ok := projectToPixel(wcsRolled, 1000, 1000, star)
	if !ok {
		t.Fatal("expected projection to succeed")
	}
	if math.Abs(rx-500) > 0.5 {
		t.Fatalf("after a 90deg roll, the RA-offset star should not move in X, got x=%v", rx)
	}
	if math.Abs(math.Abs(ry-500)-expectedPixels) > 0.5 {
		t.Fatalf("after a 90deg roll, expected the RA-offset star's displacement to move into Y, got y=%v", ry)
	}
}

// syntheticFrame builds a WCS plus a matched set of catalog stars/detections
// where every detection's flux is chosen so that, given trueZeroPoint, its
// instrumental magnitude reproduces the star's real catalog magnitude
// exactly -- i.e. ZeroPoint(matches) should recover trueZeroPoint.
func syntheticFrame(t *testing.T, trueZeroPoint float64, mags []float64) (*platesolve.WCS, []platesolve.Detection, []starcat.Star) {
	t.Helper()
	wcs := &platesolve.WCS{CenterRADeg: 50.0, CenterDecDeg: 10.0, RollDeg: 0, ArcsecPerPixel: 15, MatchedStars: len(mags), Confidence: 0.9}

	var stars []starcat.Star
	var detections []platesolve.Detection
	for i, mag := range mags {
		// Spread stars out in RA so their projected pixel positions don't
		// collide, small enough to stay well inside a 2000x2000 frame.
		offsetDeg := 0.02 * float64(i+1)
		star := starcat.Star{RADeg: 50.0 + offsetDeg, DecDeg: 10.0, Mag: mag, HIP: i + 1}
		stars = append(stars, star)

		x, y, ok := projectToPixel(wcs, 2000, 2000, star)
		if !ok {
			t.Fatalf("synthetic star %d failed to project", i)
		}
		// instrumentalMag(flux) = -2.5*log10(flux); ZeroPoint computes
		// offset = catalogMag - instMag, so to make it recover
		// trueZeroPoint exactly we need instMag = mag - trueZeroPoint.
		instMag := mag - trueZeroPoint
		flux := math.Pow(10, -instMag/2.5)
		detections = append(detections, platesolve.Detection{X: x, Y: y, Flux: flux})
	}
	return wcs, detections, stars
}

func TestZeroPointRecoversKnownCalibration(t *testing.T) {
	const trueZeroPoint = 18.5
	wcs, detections, stars := syntheticFrame(t, trueZeroPoint, []float64{2.0, 3.5, 4.0, 5.2, 6.0, 6.8})

	matches := MatchDetections(wcs, 2000, 2000, detections, stars)
	if len(matches) != len(stars) {
		t.Fatalf("expected all %d synthetic stars to match, got %d", len(stars), len(matches))
	}

	zp, ok := ZeroPoint(matches)
	if !ok {
		t.Fatal("expected a zero-point to be derivable")
	}
	if math.Abs(zp-trueZeroPoint) > 1e-6 {
		t.Fatalf("expected zero-point %v, got %v", trueZeroPoint, zp)
	}
}

func TestZeroPointMedianIgnoresOutlier(t *testing.T) {
	const trueZeroPoint = 18.5
	wcs, detections, stars := syntheticFrame(t, trueZeroPoint, []float64{2.0, 3.0, 4.0, 5.0, 6.0})

	// Corrupt one detection's flux (e.g. a plane/satellite trail wrongly
	// matched to a real catalog star) -- the median zero-point should still
	// land on the true value rather than drifting toward the bad pair.
	detections[2].Flux *= 50

	matches := MatchDetections(wcs, 2000, 2000, detections, stars)
	zp, ok := ZeroPoint(matches)
	if !ok {
		t.Fatal("expected a zero-point to be derivable")
	}
	if math.Abs(zp-trueZeroPoint) > 1e-6 {
		t.Fatalf("expected the median to recover %v despite one bad pair, got %v", trueZeroPoint, zp)
	}
}

func TestEstimateLimitingMagnitudeUsesFaintestMatch(t *testing.T) {
	const trueZeroPoint = 18.5
	faintestMag := 6.8
	wcs, detections, stars := syntheticFrame(t, trueZeroPoint, []float64{2.0, 3.5, 4.0, 5.2, 6.0, faintestMag})

	matches := MatchDetections(wcs, 2000, 2000, detections, stars)
	estimate := EstimateLimitingMagnitude(matches, detections, wcs.Confidence, true)

	if estimate.FlaggedForReview {
		t.Fatalf("did not expect a plausible estimate to be flagged: %s", estimate.FlagReason)
	}
	if estimate.LimitingMagnitude > faintestMag+1e-6 {
		t.Fatalf("expected limiting magnitude to be at least as conservative as the faintest match (%v), got %v", faintestMag, estimate.LimitingMagnitude)
	}
	if estimate.Confidence != ConfidenceMeasured {
		t.Fatalf("expected a well-matched RAW frame to report measured confidence, got %s", estimate.Confidence)
	}
}

func TestEstimateLimitingMagnitudeDowngradesConfidenceForJpeg(t *testing.T) {
	const trueZeroPoint = 18.5
	wcs, detections, stars := syntheticFrame(t, trueZeroPoint, []float64{2.0, 3.5, 4.0, 5.2, 6.0, 6.8})
	matches := MatchDetections(wcs, 2000, 2000, detections, stars)

	rawEstimate := EstimateLimitingMagnitude(matches, detections, wcs.Confidence, true)
	jpegEstimate := EstimateLimitingMagnitude(matches, detections, wcs.Confidence, false)

	if jpegEstimate.Confidence == rawEstimate.Confidence {
		t.Fatalf("expected a processed-JPEG input to report lower confidence than RAW, got %s for both", jpegEstimate.Confidence)
	}
}

func TestEstimateLimitingMagnitudeFlagsImplausibleResult(t *testing.T) {
	// Every synthetic star here is fainter than any phone camera can
	// plausibly resolve (mag 9-14) -- both the faintest-match and turnover
	// methods should agree the result is implausible, so it gets flagged
	// rather than accepted silently. (A single implausible star among
	// otherwise-plausible ones is deliberately *not* enough to flag -- see
	// TestEstimateLimitingMagnitudeUsesFaintestMatch's conservative-minimum
	// behavior, which already treats that case as "ignore the outlier".)
	const trueZeroPoint = 18.5
	wcs, detections, stars := syntheticFrame(t, trueZeroPoint, []float64{9.0, 10.0, 11.0, 12.0, 13.0, 14.0})
	matches := MatchDetections(wcs, 2000, 2000, detections, stars)

	estimate := EstimateLimitingMagnitude(matches, detections, wcs.Confidence, true)
	if !estimate.FlaggedForReview {
		t.Fatalf("expected an implausible limiting magnitude to be flagged for review, got %v", estimate.LimitingMagnitude)
	}
}

func TestEstimateLimitingMagnitudeTooFewMatches(t *testing.T) {
	wcs, detections, stars := syntheticFrame(t, 18.5, []float64{2.0, 3.0})
	matches := MatchDetections(wcs, 2000, 2000, detections, stars)

	estimate := EstimateLimitingMagnitude(matches, detections, wcs.Confidence, true)
	if !estimate.FlaggedForReview {
		t.Fatal("expected too few matches to be flagged for review rather than produce a confident estimate")
	}
}
