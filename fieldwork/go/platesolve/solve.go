package platesolve

import (
	"errors"
	"math"
	"sort"
	"time"

	"github.com/signal-k/fieldwork/starcat"
)

// ErrNoConfidentMatch is returned by Solve when no candidate direction
// produced a geometrically confirmed star match. Callers should degrade to
// the EXIF-direction (or fully omnidirectional) identification path rather
// than treat this as a hard failure.
var ErrNoConfidentMatch = errors.New("platesolve: no confident match found")

// CandidateSky is what's known about where/when a photo was taken, before
// any image analysis: this is what narrows plate-solving from a blind
// all-sky search down to a constrained local one.
type CandidateSky struct {
	LatDeg float64
	LonDeg float64
	Time   time.Time

	// HeadingDeg/PitchDeg are an optional hint (from EXIF GPSImgDirection,
	// or the live sensor path) at which direction the camera was pointed.
	// Nil means "unknown" — Solve falls back to a coarser blind search of
	// the visible hemisphere.
	HeadingDeg *float64
	PitchDeg   *float64

	// AssumedFOVDeg is a rough guess at the photo's diagonal field of view
	// (e.g. from EXIF focal length + sensor size, or a phone-camera
	// default). It sizes how far around each candidate direction to look
	// for stars to match — the search grid above handles pointing
	// uncertainty, this handles scale/zoom uncertainty. Defaults to 65
	// degrees (a generous phone main-camera FOV) when zero.
	AssumedFOVDeg float64
}

// WCS ("World Coordinate System") is a solved photo pointing: where the
// image center is aimed, how it's rotated, and its plate scale.
type WCS struct {
	CenterRADeg    float64
	CenterDecDeg   float64
	RollDeg        float64
	ArcsecPerPixel float64
	MatchedStars   int
	Confidence     float64
}

const (
	maxQuadStars       = 12   // brightest stars used for quad generation, per side
	maxVerifyStars     = 150  // brightest catalog stars considered when scoring a proposed match
	minMatchedStars    = 6    // minimum verified correspondences to accept a solve
	hashTolerance      = 0.03 // max Euclidean distance between quad hashes to consider a match
	matchPixelFrac     = 0.02 // matched-star pixel tolerance, as a fraction of image diagonal
	defaultFOVDeg      = 65   // fallback AssumedFOVDeg when the caller doesn't supply one
	fovSearchMarginDeg = 6    // extra radius beyond half the assumed FOV, to absorb pointing slop
)

// Solve attempts to determine a photo's exact pointing (WCS) from detected
// star positions and what's already known about where/when it was taken.
func Solve(detections []Detection, imgW, imgH int, candidate CandidateSky) (*WCS, error) {
	if len(detections) < 4 || imgW <= 0 || imgH <= 0 {
		return nil, ErrNoConfidentMatch
	}

	bright := detections
	if len(bright) > maxQuadStars {
		bright = bright[:maxQuadStars] // detections are flux-sorted descending
	}
	imgPts := make([]point2, len(bright))
	for i, d := range bright {
		imgPts[i] = point2{X: d.X, Y: d.Y}
	}

	var imgQuads []quadHash
	combinations4(len(imgPts), func(idx [4]int) bool {
		var pts [4]point2
		for k, ix := range idx {
			pts[k] = imgPts[ix]
		}
		if qh, ok := buildQuadHash(pts, idx); ok {
			imgQuads = append(imgQuads, qh)
		}
		return true
	})
	if len(imgQuads) == 0 {
		return nil, ErrNoConfidentMatch
	}

	diag := math.Hypot(float64(imgW), float64(imgH))
	pixelTolerance := diag * matchPixelFrac

	assumedFOV := candidate.AssumedFOVDeg
	if assumedFOV <= 0 {
		assumedFOV = defaultFOVDeg
	}
	starSearchRadiusDeg := assumedFOV/2 + fovSearchMarginDeg

	var best *WCS
	for _, cand := range candidateDirections(candidate) {
		wcs, ok := trySolveAt(candidate, cand, imgPts, imgQuads, detections, imgW, imgH, pixelTolerance, starSearchRadiusDeg)
		if !ok {
			continue
		}
		// Several candidate directions can pass the match-count bar (quad
		// hashing has a nonzero false-positive rate at low star counts) —
		// take the one with the most verified matches, not just the first,
		// since a correct solve should out-match a coincidental one once
		// checked against the full catalog subset.
		if best == nil || wcs.MatchedStars > best.MatchedStars {
			best = wcs
		}
	}
	if best == nil {
		return nil, ErrNoConfidentMatch
	}
	return best, nil
}

// candidateDir is one direction to attempt a solve around.
type candidateDir struct {
	HeadingDeg, PitchDeg float64 // alt/az, for querying starcat
	RADeg, DecDeg        float64 // equivalent equatorial position, for projection
}

// candidateDirections builds the grid of directions Solve will try. When a
// heading hint is present, it's a tight jitter grid around that hint
// (compass/orientation readings are approximate, not exact). Without one,
// it's a coarser grid covering the visible hemisphere — bounded so solving
// stays fast, at the cost of only reliably landing on directions close to
// a grid point (see platesolve package doc).
func candidateDirections(c CandidateSky) []candidateDir {
	var out []candidateDir
	add := func(headingDeg, pitchDeg float64) {
		if pitchDeg < 5 || pitchDeg > 89 {
			return
		}
		ra, dec := equatorialFromHorizontal(pitchDeg, headingDeg, c.Time, c.LatDeg, c.LonDeg)
		out = append(out, candidateDir{
			HeadingDeg: normalizeDeg(headingDeg), PitchDeg: pitchDeg,
			RADeg: ra, DecDeg: dec,
		})
	}

	if c.HeadingDeg != nil {
		pitchBase := 45.0
		if c.PitchDeg != nil {
			pitchBase = *c.PitchDeg
		}
		for _, dh := range []float64{-30, -15, 0, 15, 30} {
			for _, dp := range []float64{-20, 0, 20} {
				add(*c.HeadingDeg+dh, pitchBase+dp)
			}
		}
		return out
	}

	for az := 0.0; az < 360; az += 30 {
		for _, alt := range []float64{20, 40, 60, 80} {
			add(az, alt)
		}
	}
	return out
}

func trySolveAt(c CandidateSky, cand candidateDir, imgPts []point2, imgQuads []quadHash, allDetections []Detection, imgW, imgH int, pixelTolerance, starSearchRadiusDeg float64) (*WCS, bool) {
	stars := starcat.VisibleInDirection(c.LatDeg, c.LonDeg, c.Time, -2, cand.HeadingDeg, cand.PitchDeg, starSearchRadiusDeg)
	if len(stars) < 4 {
		return nil, false
	}
	sort.Slice(stars, func(i, j int) bool { return stars[i].Mag < stars[j].Mag })
	if len(stars) > maxVerifyStars {
		stars = stars[:maxVerifyStars]
	}

	// catPts/catStars is the full verification set (used to score how many
	// stars actually line up once a candidate transform is proposed).
	// catPtsBright is the smaller subset quad hashes are built from — kept
	// small to bound the O(n^4) combination count.
	catPts := make([]point2, 0, len(stars))
	catStars := make([]starcat.Star, 0, len(stars))
	for _, s := range stars {
		x, y, ok := gnomonicProject(s.RADeg, s.DecDeg, cand.RADeg, cand.DecDeg)
		if !ok {
			continue
		}
		catPts = append(catPts, point2{X: x, Y: y})
		catStars = append(catStars, s)
	}
	if len(catPts) < 4 {
		return nil, false
	}

	brightCount := min(len(catPts), maxQuadStars)
	catPtsBright := catPts[:brightCount]

	var catQuads []quadHash
	combinations4(len(catPtsBright), func(idx [4]int) bool {
		var pts [4]point2
		for k, ix := range idx {
			pts[k] = catPtsBright[ix]
		}
		if qh, ok := buildQuadHash(pts, idx); ok {
			catQuads = append(catQuads, qh)
		}
		return true
	})

	var best *WCS
	for _, iq := range imgQuads {
		for _, cq := range catQuads {
			if hashDistance(iq.Hash, cq.Hash) > hashTolerance {
				continue
			}

			src := [4]point2{catPtsBright[cq.Order[0]], catPtsBright[cq.Order[1]], catPtsBright[cq.Order[2]], catPtsBright[cq.Order[3]]}
			dst := [4]point2{imgPts[iq.Order[0]], imgPts[iq.Order[1]], imgPts[iq.Order[2]], imgPts[iq.Order[3]]}

			a, b, ok := similarityFit(src[:], dst[:])
			if !ok {
				continue
			}

			wcs, ok := verifyAndRefine(a, b, catPts, catStars, allDetections, cand, imgW, imgH, pixelTolerance)
			if !ok {
				continue
			}
			if best == nil || wcs.MatchedStars > best.MatchedStars {
				best = wcs
			}
		}
	}

	return best, best != nil
}

// verifyAndRefine projects every candidate catalog star through the
// similarity transform (a, b), greedily matches projected positions to
// actual detections, and — if enough of them agree — refits the transform
// using every matched pair (more accurate than the seed quad alone) before
// producing the final WCS.
func verifyAndRefine(a, b point2, catPts []point2, catStars []starcat.Star, detections []Detection, cand candidateDir, imgW, imgH int, pixelTolerance float64) (*WCS, bool) {
	usedDetection := make([]bool, len(detections))
	var matchedSrc, matchedDst []point2

	for i, cp := range catPts {
		pred := applySimilarity(a, b, cp)
		if pred.X < -pixelTolerance || pred.X > float64(imgW)+pixelTolerance ||
			pred.Y < -pixelTolerance || pred.Y > float64(imgH)+pixelTolerance {
			continue
		}

		best, bestDist := -1, pixelTolerance
		for di, d := range detections {
			if usedDetection[di] {
				continue
			}
			dist := math.Hypot(d.X-pred.X, d.Y-pred.Y)
			if dist < bestDist {
				best, bestDist = di, dist
			}
		}
		if best >= 0 {
			usedDetection[best] = true
			matchedSrc = append(matchedSrc, cp)
			matchedDst = append(matchedDst, point2{X: detections[best].X, Y: detections[best].Y})
			_ = catStars[i] // reserved for future named-match enrichment
		}
	}

	if len(matchedSrc) < minMatchedStars {
		return nil, false
	}

	fa, fb, ok := similarityFit(matchedSrc, matchedDst)
	if !ok {
		fa, fb = a, b
	}

	var residual float64
	for i := range matchedSrc {
		pred := applySimilarity(fa, fb, matchedSrc[i])
		residual += math.Hypot(pred.X-matchedDst[i].X, pred.Y-matchedDst[i].Y)
	}
	residual /= float64(len(matchedSrc))

	center := point2{X: float64(imgW) / 2, Y: float64(imgH) / 2}
	srcCenter := invertSimilarity(fa, fb, center)
	centerRA, centerDec := gnomonicUnproject(srcCenter.X, srcCenter.Y, cand.RADeg, cand.DecDeg)

	scale := complexMagnitude(fa)
	if scale < 1e-12 {
		return nil, false
	}

	confidence := clamp(float64(len(matchedSrc))/15, 0, 1) * clamp(1-residual/pixelTolerance, 0, 1)

	return &WCS{
		CenterRADeg:    centerRA,
		CenterDecDeg:   centerDec,
		RollDeg:        normalizeDeg(complexAngleDeg(fa)),
		ArcsecPerPixel: 206265 / scale,
		MatchedStars:   len(matchedSrc),
		Confidence:     confidence,
	}, true
}

// invertSimilarity solves dst = a*src + b for src, given dst (complex
// division: src = (dst-b) / a).
func invertSimilarity(a, b, dst point2) point2 {
	dx, dy := dst.X-b.X, dst.Y-b.Y
	denom := a.X*a.X + a.Y*a.Y
	return point2{
		X: (dx*a.X + dy*a.Y) / denom,
		Y: (dy*a.X - dx*a.Y) / denom,
	}
}
