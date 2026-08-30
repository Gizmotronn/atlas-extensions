package identify

import (
	"math"
	"time"
)

// planetElements holds the linear-in-time orbital elements for a planet, in
// the style of Paul Schlyter's "How to compute planetary positions"
// (stjarnhimlen.se/comp/ppcomp.html): each element is `base + rate*d`, d
// being days since J2000.0, angles in degrees, distances in AU.
type planetElements struct {
	name      string
	n0, nRate float64 // longitude of ascending node
	i0, iRate float64 // inclination
	w0, wRate float64 // argument of perihelion
	a0, aRate float64 // semi-major axis
	e0, eRate float64 // eccentricity
	m0, mRate float64 // mean anomaly
}

// Elements for the naked-eye planets, epoch J2000.0. Coefficients are the
// well-known low-precision set popularized by Schlyter's page (public
// domain), sufficient for degree-level accuracy in apparent position.
var planetTable = []planetElements{
	{name: "mercury", n0: 48.3313, nRate: 3.24587e-5, i0: 7.0047, iRate: 5.00e-8, w0: 29.1241, wRate: 1.01444e-5, a0: 0.387098, aRate: 0, e0: 0.205635, eRate: 5.59e-10, m0: 168.6562, mRate: 4.0923344368},
	{name: "venus", n0: 76.6799, nRate: 2.46590e-5, i0: 3.3946, iRate: 2.75e-8, w0: 54.8910, wRate: 1.38374e-5, a0: 0.723330, aRate: 0, e0: 0.006773, eRate: -1.302e-9, m0: 48.0052, mRate: 1.6021302244},
	{name: "mars", n0: 49.5574, nRate: 2.11081e-5, i0: 1.8497, iRate: -1.78e-8, w0: 286.5016, wRate: 2.92961e-5, a0: 1.523688, aRate: 0, e0: 0.093405, eRate: 2.516e-9, m0: 18.6021, mRate: 0.5240207766},
	{name: "jupiter", n0: 100.4542, nRate: 2.76854e-5, i0: 1.3030, iRate: -1.557e-7, w0: 273.8777, wRate: 1.64505e-5, a0: 5.20256, aRate: 0, e0: 0.048498, eRate: 4.469e-9, m0: 19.8950, mRate: 0.0830853001},
	{name: "saturn", n0: 113.6634, nRate: 2.38980e-5, i0: 2.4886, iRate: -1.081e-7, w0: 339.3939, wRate: 2.97661e-5, a0: 9.55475, aRate: 0, e0: 0.055546, eRate: -9.499e-9, m0: 316.9670, mRate: 0.0334442282},
}

// planetPosition returns the geocentric apparent equatorial position of the
// named planet at time t (name must be one of planetTable's `name` values).
func planetPosition(name string, t time.Time) (equatorial, bool) {
	var el *planetElements
	for i := range planetTable {
		if planetTable[i].name == name {
			el = &planetTable[i]
			break
		}
	}
	if el == nil {
		return equatorial{}, false
	}

	d := daysSinceJ2000(t)

	heliocentric := func(el *planetElements, d float64) (x, y, z float64) {
		N := normalizeDeg(el.n0+el.nRate*d) * deg2rad
		i := normalizeDeg(el.i0+el.iRate*d) * deg2rad
		w := normalizeDeg(el.w0+el.wRate*d) * deg2rad
		a := el.a0 + el.aRate*d
		e := el.e0 + el.eRate*d
		M := normalizeDeg(el.m0+el.mRate*d) * deg2rad

		// Solve Kepler's equation E - e*sin(E) = M by iteration.
		E := M
		for range 8 {
			E = E - (E-e*math.Sin(E)-M)/(1-e*math.Cos(E))
		}

		xv := a * (math.Cos(E) - e)
		yv := a * (math.Sqrt(1-e*e) * math.Sin(E))
		v := math.Atan2(yv, xv)
		r := math.Sqrt(xv*xv + yv*yv)

		xh := r * (math.Cos(N)*math.Cos(v+w) - math.Sin(N)*math.Sin(v+w)*math.Cos(i))
		yh := r * (math.Sin(N)*math.Cos(v+w) + math.Cos(N)*math.Sin(v+w)*math.Cos(i))
		zh := r * (math.Sin(v+w) * math.Sin(i))
		return xh, yh, zh
	}

	// Earth's heliocentric position, approximated as the Sun's position
	// negated (Earth-Sun elements are the same order of precision as the
	// solar formulas already used in sun.go).
	sun := sunPosition(t)
	earthDist := 1.00014 // AU, good enough at this precision
	sunRA := sun.RADeg * deg2rad
	sunDec := sun.DecDeg * deg2rad
	xsE := earthDist * math.Cos(sunDec) * math.Cos(sunRA)
	ysE := earthDist * math.Cos(sunDec) * math.Sin(sunRA)
	// Earth is opposite the Sun as seen from itself.
	xEarth, yEarth, zEarth := -xsE, -ysE, 0.0

	xh, yh, zh := heliocentric(el, d)

	xg := xh - xEarth
	yg := yh - yEarth
	zg := zh - zEarth

	obliquity := (23.439 - 0.0000004*d) * deg2rad
	xeq := xg
	yeq := yg*math.Cos(obliquity) - zg*math.Sin(obliquity)
	zeq := yg*math.Sin(obliquity) + zg*math.Cos(obliquity)

	ra := normalizeDeg(math.Atan2(yeq, xeq) * rad2deg)
	dec := math.Atan2(zeq, math.Sqrt(xeq*xeq+yeq*yeq)) * rad2deg

	return equatorial{RADeg: ra, DecDeg: dec}, true
}
