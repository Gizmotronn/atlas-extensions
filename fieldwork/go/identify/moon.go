package identify

import (
	"math"
	"time"
)

// moonPosition returns the Moon's geocentric equatorial position using
// Jean Meeus's abbreviated lunar theory (Astronomical Algorithms ch. 47,
// truncated to the largest periodic terms; accurate to a few arc-minutes,
// well within tolerance for a "what am I pointing at" scan).
func moonPosition(t time.Time) equatorial {
	d := daysSinceJ2000(t)

	// Moon's mean longitude, mean elongation, Sun's mean anomaly, Moon's
	// mean anomaly, Moon's argument of latitude (all degrees).
	Lp := normalizeDeg(218.316 + 13.176396*d)
	D := normalizeDeg(297.850+12.190749*d) * deg2rad
	M := normalizeDeg(357.529+0.985600*d) * deg2rad
	Mp := normalizeDeg(134.963+13.064993*d) * deg2rad
	F := normalizeDeg(93.272+13.229350*d) * deg2rad

	// Largest periodic terms for ecliptic longitude/latitude (degrees).
	lon := Lp +
		6.289*math.Sin(Mp) -
		1.274*math.Sin(Mp-2*D) +
		0.658*math.Sin(2*D) -
		0.186*math.Sin(M) -
		0.059*math.Sin(2*Mp-2*D) -
		0.057*math.Sin(Mp-2*D+M)

	lat := 5.128*math.Sin(F) +
		0.281*math.Sin(Mp+F) -
		0.278*math.Sin(F-Mp) -
		0.173*math.Sin(F-2*D)

	lonRad := lon * deg2rad
	latRad := lat * deg2rad
	obliquity := (23.439 - 0.0000004*d) * deg2rad

	sinDec := math.Sin(latRad)*math.Cos(obliquity) + math.Cos(latRad)*math.Sin(obliquity)*math.Sin(lonRad)
	dec := math.Asin(clamp(sinDec, -1, 1))

	y := math.Sin(lonRad)*math.Cos(obliquity) - math.Tan(latRad)*math.Sin(obliquity)
	x := math.Cos(lonRad)
	ra := math.Atan2(y, x)

	return equatorial{RADeg: normalizeDeg(ra * rad2deg), DecDeg: dec * rad2deg}
}

// moonPhaseFraction returns the illuminated fraction of the Moon's disc,
// 0 (new) to 1 (full), using the Sun/Moon elongation.
func moonPhaseFraction(t time.Time) float64 {
	sun := sunPosition(t)
	moon := moonPosition(t)

	sunRad := equatorialToVector(sun)
	moonRad := equatorialToVector(moon)

	cosElong := sunRad[0]*moonRad[0] + sunRad[1]*moonRad[1] + sunRad[2]*moonRad[2]
	elong := math.Acos(clamp(cosElong, -1, 1))
	return (1 - math.Cos(elong)) / 2
}

func equatorialToVector(e equatorial) [3]float64 {
	ra := e.RADeg * deg2rad
	dec := e.DecDeg * deg2rad
	return [3]float64{
		math.Cos(dec) * math.Cos(ra),
		math.Cos(dec) * math.Sin(ra),
		math.Sin(dec),
	}
}
