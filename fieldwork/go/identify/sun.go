package identify

import (
	"math"
	"time"
)

// sunPosition returns the Sun's geocentric equatorial position using the
// standard low-precision solar formulas (mean longitude + equation of
// center; accurate to about 0.01 degrees).
func sunPosition(t time.Time) equatorial {
	d := daysSinceJ2000(t)

	meanLon := normalizeDeg(280.460 + 0.9856474*d)
	meanAnom := normalizeDeg(357.528+0.9856003*d) * deg2rad

	eclipticLon := meanLon + 1.915*math.Sin(meanAnom) + 0.020*math.Sin(2*meanAnom)
	eclipticLonRad := eclipticLon * deg2rad

	obliquity := (23.439 - 0.0000004*d) * deg2rad

	ra := math.Atan2(math.Cos(obliquity)*math.Sin(eclipticLonRad), math.Cos(eclipticLonRad))
	dec := math.Asin(math.Sin(obliquity) * math.Sin(eclipticLonRad))

	return equatorial{RADeg: normalizeDeg(ra * rad2deg), DecDeg: dec * rad2deg}
}
