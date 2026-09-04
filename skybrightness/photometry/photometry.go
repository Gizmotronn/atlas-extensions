// Package photometry turns a plate-solved night-sky photo
// (fieldwork/go/platesolve) into a sky-brightness estimate, using the same
// technique real dark-sky photometry does: differential (relative)
// photometry against catalog stars visible in the frame, rather than
// requiring a lab-calibrated device.
//
// The method, and its caveats, come from the smartphone/DSLR sky-brightness
// literature (Kyba et al., "Measuring night sky brightness: methods and
// challenges", arXiv:1709.09558):
//
//  1. Match detected bright points (platesolve.Detection) to catalog stars
//     (starcat.Star) by projecting each catalog star through the solved WCS
//     into pixel space and pairing it with the nearest detection.
//  2. Derive a zero-point: the constant that converts a detection's raw
//     pixel flux into a calibrated magnitude, from the matched pairs'
//     already-known catalog magnitudes (median, not mean, so one badly
//     mismatched pair can't drag the whole frame's calibration off).
//  3. Estimate the limiting magnitude two ways -- the faintest confidently
//     matched catalog star, and a "turnover" read of the full detected
//     (matched + unmatched) flux histogram -- and report the more
//     conservative (brighter) of the two, since overclaiming how deep the
//     photo actually sees is worse than underclaiming it.
//
// A phone's computational "night mode" (multi-frame stacking, aggressive
// denoise) breaks the linear photon-count-to-pixel-value relationship this
// depends on -- see SourceFormat on Estimate and the caller-supplied flag on
// Estimate's input. This package does not attempt to correct for that; it
// only lowers Confidence and records that the input wasn't known-linear.
package photometry

import (
	"math"
	"sort"

	"github.com/signal-k/fieldwork/platesolve"
	"github.com/signal-k/fieldwork/starcat"
)

const deg2rad = math.Pi / 180
const rad2deg = 180 / math.Pi
const arcsecPerRadian = 206265

// projectToPixel maps a catalog star's RA/Dec onto the pixel space of a
// solved WCS. This is a from-scratch re-derivation of platesolve's internal
// projection (its gnomonicProject/similarity-fit machinery is unexported),
// using only WCS's public contract: CenterRADeg/CenterDecDeg is the sky
// position at the image's exact pixel center ((imgW/2, imgH/2)) -- true by
// construction, since that's literally how platesolve.Solve derives those
// two fields -- and RollDeg/ArcsecPerPixel describe the rotation and scale
// of the fitted transform from tangent-plane radians to pixels.
func projectToPixel(wcs *platesolve.WCS, imgW, imgH int, star starcat.Star) (x, y float64, ok bool) {
	gx, gy, projected := gnomonicProject(star.RADeg, star.DecDeg, wcs.CenterRADeg, wcs.CenterDecDeg)
	if !projected {
		return 0, 0, false
	}

	scale := arcsecPerRadian / wcs.ArcsecPerPixel // pixels per radian
	roll := wcs.RollDeg * deg2rad
	cosR, sinR := math.Cos(roll), math.Sin(roll)

	rx := scale * (gx*cosR - gy*sinR)
	ry := scale * (gx*sinR + gy*cosR)

	return float64(imgW)/2 + rx, float64(imgH)/2 + ry, true
}

func gnomonicProject(raDeg, decDeg, ra0Deg, dec0Deg float64) (x, y float64, ok bool) {
	ra, dec := raDeg*deg2rad, decDeg*deg2rad
	ra0, dec0 := ra0Deg*deg2rad, dec0Deg*deg2rad

	cosC := math.Sin(dec0)*math.Sin(dec) + math.Cos(dec0)*math.Cos(dec)*math.Cos(ra-ra0)
	if cosC <= 0.01 {
		return 0, 0, false
	}
	x = math.Cos(dec) * math.Sin(ra-ra0) / cosC
	y = (math.Cos(dec0)*math.Sin(dec) - math.Sin(dec0)*math.Cos(dec)*math.Cos(ra-ra0)) / cosC
	return x, y, true
}

// MatchedStar pairs a catalog star with the detection nearest its projected
// pixel position, within the caller's tolerance.
type MatchedStar struct {
	Star          starcat.Star
	Detection     platesolve.Detection
	PixelDistance float64
}

// matchTolerancePx mirrors platesolve's own matchPixelFrac (2% of the image
// diagonal) -- same reasoning: too tight and real matches from solver noise
// are missed, too loose and an unrelated bright pixel gets paired with the
// wrong star and corrupts the zero-point.
const matchTolerancePxFrac = 0.02

// MatchDetections pairs catalog stars against image detections via the
// solved WCS, greedily nearest-first so no detection is claimed twice.
func MatchDetections(wcs *platesolve.WCS, imgW, imgH int, detections []platesolve.Detection, catalogStars []starcat.Star) []MatchedStar {
	tolerance := math.Hypot(float64(imgW), float64(imgH)) * matchTolerancePxFrac
	used := make([]bool, len(detections))
	var matches []MatchedStar

	type projected struct {
		star starcat.Star
		x, y float64
	}
	var candidates []projected
	for _, s := range catalogStars {
		x, y, ok := projectToPixel(wcs, imgW, imgH, s)
		if !ok || x < -tolerance || x > float64(imgW)+tolerance || y < -tolerance || y > float64(imgH)+tolerance {
			continue
		}
		candidates = append(candidates, projected{s, x, y})
	}
	// Brightest catalog stars claim their nearest detection first -- a
	// fainter star's weaker/noisier projection is less likely to steal the
	// correct detection out from under a brighter, better-constrained one.
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].star.Mag < candidates[j].star.Mag })

	for _, c := range candidates {
		best, bestDist := -1, tolerance
		for di, d := range detections {
			if used[di] {
				continue
			}
			dist := math.Hypot(d.X-c.x, d.Y-c.y)
			if dist < bestDist {
				best, bestDist = di, dist
			}
		}
		if best >= 0 {
			used[best] = true
			matches = append(matches, MatchedStar{Star: c.star, Detection: detections[best], PixelDistance: bestDist})
		}
	}

	return matches
}

// instrumentalMag converts raw detection flux to an uncalibrated magnitude
// on the standard astronomical (higher flux = lower/brighter magnitude)
// scale. Flux <= 0 is treated as unmeasurable.
func instrumentalMag(flux float64) (float64, bool) {
	if flux <= 0 {
		return 0, false
	}
	return -2.5 * math.Log10(flux), true
}

// ZeroPoint derives the additive constant relating instrumental magnitude to
// true catalog magnitude, as the median offset across matched pairs (median
// rather than mean so one mismatched pair -- e.g. a plane or satellite trail
// wrongly detected as a star -- can't skew the whole frame's calibration).
func ZeroPoint(matches []MatchedStar) (zeroPoint float64, ok bool) {
	var offsets []float64
	for _, m := range matches {
		instMag, measurable := instrumentalMag(m.Detection.Flux)
		if !measurable {
			continue
		}
		offsets = append(offsets, m.Star.Mag-instMag)
	}
	if len(offsets) == 0 {
		return 0, false
	}
	sort.Float64s(offsets)
	return median(offsets), true
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// Confidence is how much an Estimate's LimitingMagnitude should be trusted,
// mirroring the three-tier vocabulary atlas_light_pollution_samples already
// uses for externally-fetched brightness data (estimated/modelled/measured)
// so both sources of atlas_observations.sky_brightness_confidence mean the
// same thing to a downstream reader.
type Confidence string

const (
	ConfidenceMeasured  Confidence = "measured"
	ConfidenceModelled  Confidence = "modelled"
	ConfidenceEstimated Confidence = "estimated"
)

// Estimate is the processor's output for one submitted photo.
type Estimate struct {
	ZeroPoint         float64
	LimitingMagnitude float64
	Confidence        Confidence
	MatchedStars      int
	DetectedStars     int
	FlaggedForReview  bool
	FlagReason        string
}

// minMatchesForConfidentEstimate mirrors platesolve's own minMatchedStars --
// below this, a "solved" WCS is geometrically valid but too thinly matched
// for its zero-point to be trustworthy.
const minMatchesForConfidentEstimate = 6

// plausibleLimitingMagnitudeRange bounds what a phone camera can plausibly
// report. Real naked-eye/phone limiting magnitudes run roughly 1 (a
// floodlit city center) to about 7 (a truly dark site, near the edge of
// human vision) -- anything outside that is almost certainly a bad match or
// a corrupted zero-point, not a real reading (eBird's automated-flag model:
// don't discard it, but don't accept it silently either).
const (
	minPlausibleLimitingMag = 1.0
	maxPlausibleLimitingMag = 8.0
)

// magBinWidth buckets instrumental magnitudes for the turnover method: the
// point where the detected-star count histogram stops rising and starts
// falling marks the practical detection limit, independent of the
// catalog-matched estimate below.
const magBinWidth = 0.5

// Estimate computes a sky-brightness estimate from a solved WCS and the raw
// detections it was solved from. rawInput=false (a processed JPEG rather
// than RAW) lowers Confidence a tier, per the package doc's phone
// night-mode caveat, rather than silently claiming calibrated precision the
// input can't support.
func EstimateLimitingMagnitude(matches []MatchedStar, allDetections []platesolve.Detection, wcsConfidence float64, rawInput bool) Estimate {
	zp, ok := ZeroPoint(matches)
	if !ok || len(matches) < 4 {
		return Estimate{
			MatchedStars:     len(matches),
			DetectedStars:    len(allDetections),
			FlaggedForReview: true,
			FlagReason:       "too few catalog matches to derive a zero-point",
		}
	}

	faintestMatched := faintestMagnitude(matches)
	turnover := turnoverLimitingMagnitude(allDetections, zp)

	// The more conservative (numerically smaller / brighter-limit) of the
	// two methods -- see the package doc for why underclaiming beats
	// overclaiming here.
	limiting := faintestMatched
	if turnover < limiting {
		limiting = turnover
	}

	confidence := ConfidenceMeasured
	flagged := false
	var flagReason string

	if len(matches) < minMatchesForConfidentEstimate || wcsConfidence < 0.5 {
		confidence = ConfidenceModelled
	}
	if !rawInput {
		if confidence == ConfidenceMeasured {
			confidence = ConfidenceModelled
		} else {
			confidence = ConfidenceEstimated
		}
	}
	if limiting < minPlausibleLimitingMag || limiting > maxPlausibleLimitingMag {
		flagged = true
		flagReason = "limiting magnitude outside the plausible range for a phone-camera observation"
	}

	return Estimate{
		ZeroPoint:         zp,
		LimitingMagnitude: limiting,
		Confidence:        confidence,
		MatchedStars:      len(matches),
		DetectedStars:     len(allDetections),
		FlaggedForReview:  flagged,
		FlagReason:        flagReason,
	}
}

func faintestMagnitude(matches []MatchedStar) float64 {
	faintest := matches[0].Star.Mag
	for _, m := range matches[1:] {
		if m.Star.Mag > faintest {
			faintest = m.Star.Mag
		}
	}
	return faintest
}

// turnoverLimitingMagnitude buckets every detection's calibrated magnitude
// (matched or not -- a real but unmatched faint star still counts as
// "detected") into fixed-width bins and returns the faint edge of the last
// bin before the count stops increasing. A true stellar luminosity function
// keeps rising toward fainter magnitudes; a detector's actual sensitivity
// limit shows up as that rise flattening or reversing.
func turnoverLimitingMagnitude(detections []platesolve.Detection, zeroPoint float64) float64 {
	var mags []float64
	for _, d := range detections {
		instMag, ok := instrumentalMag(d.Flux)
		if !ok {
			continue
		}
		mags = append(mags, zeroPoint+instMag)
	}
	if len(mags) == 0 {
		return 0
	}
	sort.Float64s(mags)

	minMag, maxMag := mags[0], mags[len(mags)-1]
	bins := int((maxMag-minMag)/magBinWidth) + 1
	counts := make([]int, bins+1)
	for _, m := range mags {
		bin := int((m - minMag) / magBinWidth)
		counts[bin]++
	}

	turnoverBin := 0
	for i := 1; i < len(counts); i++ {
		if counts[i] >= counts[i-1] {
			turnoverBin = i
		} else {
			break
		}
	}
	return minMag + float64(turnoverBin)*magBinWidth
}
