package platesolve

import (
	"math"
	"time"
)

const deg2rad = math.Pi / 180
const rad2deg = 180 / math.Pi

// gnomonicProject maps an RA/Dec position (degrees) onto the tangent plane
// centered on (ra0Deg, dec0Deg), returning standard coordinates in radians.
// This is the projection a camera lens approximates: straight lines on the
// sky (great circles) stay straight, which is what lets a similarity
// transform (rotate/scale/translate) relate catalog positions to pixel
// positions near the image center.
func gnomonicProject(raDeg, decDeg, ra0Deg, dec0Deg float64) (x, y float64, ok bool) {
	ra, dec := raDeg*deg2rad, decDeg*deg2rad
	ra0, dec0 := ra0Deg*deg2rad, dec0Deg*deg2rad

	cosC := math.Sin(dec0)*math.Sin(dec) + math.Cos(dec0)*math.Cos(dec)*math.Cos(ra-ra0)
	if cosC <= 0.01 { // more than ~89 degrees from center; projection breaks down
		return 0, 0, false
	}
	x = math.Cos(dec) * math.Sin(ra-ra0) / cosC
	y = (math.Cos(dec0)*math.Sin(dec) - math.Sin(dec0)*math.Cos(dec)*math.Cos(ra-ra0)) / cosC
	return x, y, true
}

// gnomonicUnproject is the inverse of gnomonicProject: given standard
// coordinates (radians) around a tangent point, recover RA/Dec (degrees).
func gnomonicUnproject(x, y, ra0Deg, dec0Deg float64) (raDeg, decDeg float64) {
	ra0, dec0 := ra0Deg*deg2rad, dec0Deg*deg2rad

	rho := math.Hypot(x, y)
	if rho < 1e-12 {
		return ra0Deg, dec0Deg
	}
	c := math.Atan(rho)
	sinC, cosC := math.Sin(c), math.Cos(c)

	dec := math.Asin(cosC*math.Sin(dec0) + y*sinC*math.Cos(dec0)/rho)
	ra := ra0 + math.Atan2(x*sinC, rho*math.Cos(dec0)*cosC-y*math.Sin(dec0)*sinC)

	return normalizeDeg(ra * rad2deg), dec * rad2deg
}

// equatorialFromHorizontal is the inverse of the alt/az conversion used
// elsewhere in fieldwork/go (identify/astro.go, starcat/astro.go): given a
// pointing direction (altDeg, azDeg, azimuth clockwise from true north) for
// an observer at (latDeg, lonDeg) at time t, recover the RA/Dec that
// direction was pointed at. Used to turn a heading/pitch hint (from EXIF or
// the live sensor path) into a candidate plate-solve center.
func equatorialFromHorizontal(altDeg, azDeg float64, t time.Time, latDeg, lonDeg float64) (raDeg, decDeg float64) {
	alt, az := altDeg*deg2rad, azDeg*deg2rad
	lat := latDeg * deg2rad

	dec := math.Asin(clamp(math.Sin(lat)*math.Sin(alt)+math.Cos(lat)*math.Cos(alt)*math.Cos(az), -1, 1))
	cosHA := (math.Sin(alt) - math.Sin(lat)*math.Sin(dec)) / (math.Cos(lat) * math.Cos(dec))
	ha := math.Acos(clamp(cosHA, -1, 1))
	if math.Sin(az) > 0 {
		ha = 2*math.Pi - ha
	}

	lst := localSiderealTimeDeg(t, lonDeg)
	ra := normalizeDeg(lst - ha*rad2deg)
	return ra, dec * rad2deg
}

func julianDate(t time.Time) float64 {
	t = t.UTC()
	y, m := int(t.Year()), int(t.Month())
	d := float64(t.Day()) + (float64(t.Hour())+float64(t.Minute())/60+float64(t.Second())/3600)/24
	if m <= 2 {
		y--
		m += 12
	}
	a := y / 100
	b := 2 - a + a/4
	return math.Floor(365.25*float64(y+4716)) + math.Floor(30.6001*float64(m+1)) + d + float64(b) - 1524.5
}

func localSiderealTimeDeg(t time.Time, lonDeg float64) float64 {
	d := julianDate(t) - 2451545.0
	gmst := 280.46061837 + 360.98564736629*d
	return normalizeDeg(gmst + lonDeg)
}

func normalizeDeg(deg float64) float64 {
	deg = math.Mod(deg, 360)
	if deg < 0 {
		deg += 360
	}
	return deg
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// point2 is a plain 2D point used for both pixel coordinates (image space)
// and gnomonic standard coordinates (catalog space) — the quad-hash math
// below is unit-agnostic, which is what makes it scale-invariant.
type point2 struct{ X, Y float64 }

// quadHash is a rotation/scale/translation-invariant descriptor for 4
// points, in the style of astrometry.net's geometric hashing: the two most
// widely separated points become a normalized reference axis, and the
// other two points' coordinates in that frame are the hash. Two quads of
// the same physical stars — one measured in image pixels, one projected
// from the catalog — produce nearly identical hashes regardless of the
// image's rotation, scale, or the exact catalog projection center used.
type quadHash struct {
	Hash  [4]float64 // Cx, Cy, Dx, Dy in the normalized frame
	Order [4]int     // original indices, in canonical role order A, B, C, D
}

// buildQuadHash computes the canonical hash for 4 points (pts, referenced
// by idx into some caller-owned slice). Returns ok=false for degenerate
// (near-collinear or coincident) quads.
func buildQuadHash(pts [4]point2, idx [4]int) (quadHash, bool) {
	type pair struct {
		i, j int
		dist float64
	}
	var best pair
	for i := range 4 {
		for j := i + 1; j < 4; j++ {
			d := math.Hypot(pts[i].X-pts[j].X, pts[i].Y-pts[j].Y)
			if d > best.dist {
				best = pair{i, j, d}
			}
		}
	}
	if best.dist < 1e-9 {
		return quadHash{}, false
	}

	a, b := pts[best.i], pts[best.j]
	axisLen := best.dist
	ux, uy := (b.X-a.X)/axisLen, (b.Y-a.Y)/axisLen
	vx, vy := -uy, ux // perpendicular unit vector

	var others [2]int
	k := 0
	for i := range 4 {
		if i != best.i && i != best.j {
			others[k] = i
			k++
		}
	}

	local := func(p point2) (float64, float64) {
		rx, ry := p.X-a.X, p.Y-a.Y
		return (rx*ux + ry*uy) / axisLen, (rx*vx + ry*vy) / axisLen
	}

	cx, cy := local(pts[others[0]])
	dx, dy := local(pts[others[1]])

	cIdx, dIdx := idx[others[0]], idx[others[1]]
	// Canonicalize which of the two remaining points is "C" vs "D" so the
	// same physical quad always hashes the same way regardless of input
	// order.
	if cx > dx || (cx == dx && cy > dy) {
		cx, cy, dx, dy = dx, dy, cx, cy
		cIdx, dIdx = dIdx, cIdx
	}

	return quadHash{
		Hash:  [4]float64{cx, cy, dx, dy},
		Order: [4]int{idx[best.i], idx[best.j], cIdx, dIdx},
	}, true
}

func hashDistance(a, b [4]float64) float64 {
	var sum float64
	for i := range 4 {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}

// combinations4 calls fn with every 4-element index combination from
// [0,n), stopping early if fn returns false.
func combinations4(n int, fn func(idx [4]int) bool) {
	if n < 4 {
		return
	}
	for a := 0; a < n; a++ {
		for b := a + 1; b < n; b++ {
			for c := b + 1; c < n; c++ {
				for d := c + 1; d < n; d++ {
					if !fn([4]int{a, b, c, d}) {
						return
					}
				}
			}
		}
	}
}

// similarityFit finds the complex scale+rotation `a` and translation `b`
// minimizing sum |a*src_i + b - dst_i|^2, via the closed-form least-squares
// solution for a 2D similarity transform (treating points as complex
// numbers: rotation+uniform-scale is just complex multiplication). This has
// an exact analytic solution, so no iterative/matrix solver is needed for
// this restricted (non-shear, non-independent-axis-scale) transform class.
func similarityFit(src, dst []point2) (a, b point2, ok bool) {
	n := len(src)
	if n < 2 || len(dst) != n {
		return point2{}, point2{}, false
	}

	var meanSrcX, meanSrcY, meanDstX, meanDstY float64
	for i := range n {
		meanSrcX += src[i].X
		meanSrcY += src[i].Y
		meanDstX += dst[i].X
		meanDstY += dst[i].Y
	}
	meanSrcX, meanSrcY = meanSrcX/float64(n), meanSrcY/float64(n)
	meanDstX, meanDstY = meanDstX/float64(n), meanDstY/float64(n)

	// a = sum(conj(src-meanSrc) * (dst-meanDst)) / sum(|src-meanSrc|^2)
	var numX, numY, den float64
	for i := range n {
		sx, sy := src[i].X-meanSrcX, src[i].Y-meanSrcY
		dx, dy := dst[i].X-meanDstX, dst[i].Y-meanDstY
		// complex multiply conj(s) * d = (sx - i*sy)(dx + i*dy)
		numX += sx*dx + sy*dy
		numY += sx*dy - sy*dx
		den += sx*sx + sy*sy
	}
	if den < 1e-12 {
		return point2{}, point2{}, false
	}
	a = point2{X: numX / den, Y: numY / den}

	// b = meanDst - a*meanSrc (complex multiply)
	axMx := a.X*meanSrcX - a.Y*meanSrcY
	ayMx := a.X*meanSrcY + a.Y*meanSrcX
	b = point2{X: meanDstX - axMx, Y: meanDstY - ayMx}

	return a, b, true
}

func applySimilarity(a, b, p point2) point2 {
	return point2{
		X: a.X*p.X - a.Y*p.Y + b.X,
		Y: a.X*p.Y + a.Y*p.X + b.Y,
	}
}

func complexMagnitude(a point2) float64 {
	return math.Hypot(a.X, a.Y)
}

func complexAngleDeg(a point2) float64 {
	return math.Atan2(a.Y, a.X) * rad2deg
}
