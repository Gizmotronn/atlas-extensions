package starcat

// A small, self-contained copy of the alt/az conversion math in
// identify/astro.go. Kept separate (rather than importing identify) so the
// dependency runs starcat -> (nothing) and identify/platesolve -> starcat,
// not the other way around.

import (
	"math"
	"time"
)

const deg2rad = math.Pi / 180
const rad2deg = 180 / math.Pi

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

func normalizeDeg(deg float64) float64 {
	deg = math.Mod(deg, 360)
	if deg < 0 {
		deg += 360
	}
	return deg
}

func localSiderealTimeDeg(t time.Time, lonDeg float64) float64 {
	d := julianDate(t) - 2451545.0
	gmst := 280.46061837 + 360.98564736629*d
	return normalizeDeg(gmst + lonDeg)
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

// toHorizontal converts an RA/Dec position (degrees) to altitude/azimuth
// (degrees, azimuth clockwise from true north) for an observer at
// (latDeg, lonDeg) at time t.
func toHorizontal(raDeg, decDeg float64, t time.Time, latDeg, lonDeg float64) (altDeg, azDeg float64) {
	lst := localSiderealTimeDeg(t, lonDeg)
	haDeg := normalizeDeg(lst - raDeg)
	ha := haDeg * deg2rad
	dec := decDeg * deg2rad
	lat := latDeg * deg2rad

	sinAlt := math.Sin(dec)*math.Sin(lat) + math.Cos(dec)*math.Cos(lat)*math.Cos(ha)
	alt := math.Asin(clamp(sinAlt, -1, 1))

	cosAz := (math.Sin(dec) - math.Sin(alt)*math.Sin(lat)) / (math.Cos(alt) * math.Cos(lat))
	az := math.Acos(clamp(cosAz, -1, 1))
	if math.Sin(ha) > 0 {
		az = 2*math.Pi - az
	}

	return alt * rad2deg, az * rad2deg
}

// angularSeparationDeg returns the great-circle distance in degrees between
// two alt/az directions.
func angularSeparationDeg(alt1, az1, alt2, az2 float64) float64 {
	a1, a2 := alt1*deg2rad, alt2*deg2rad
	dAz := (az1 - az2) * deg2rad
	cosC := math.Sin(a1)*math.Sin(a2) + math.Cos(a1)*math.Cos(a2)*math.Cos(dAz)
	return math.Acos(clamp(cosC, -1, 1)) * rad2deg
}
