// Package identify resolves which sky objects are visible for a given
// location, time, and (optionally) pointing direction.
//
// The position math here is deliberately low-precision (good to roughly
// arc-minute to sub-degree accuracy, plenty for "what am I looking at"),
// using the same style of approximate orbital-element formulas popularized
// by Paul Schlyter's "How to compute planetary positions"
// (stjarnhimlen.se/comp/ppcomp.html) rather than a full ephemeris library.
// Keeping it small and dependency-free is deliberate: this same approach
// needs to be portable to Kotlin/JS later (see fieldwork/CONTRACT.md).
package identify

import (
	"math"
	"time"
)

const deg2rad = math.Pi / 180
const rad2deg = 180 / math.Pi

// julianDate returns the Julian Date for t (UTC).
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

// d is "days since J2000.0", the time variable most of the low-precision
// formulas below are expressed in.
func daysSinceJ2000(t time.Time) float64 {
	return julianDate(t) - 2451545.0
}

func normalizeDeg(deg float64) float64 {
	deg = math.Mod(deg, 360)
	if deg < 0 {
		deg += 360
	}
	return deg
}

// equatorial is a Right Ascension / Declination position, in degrees.
type equatorial struct {
	RADeg  float64
	DecDeg float64
}

// horizontal is an Altitude / Azimuth position, in degrees. Azimuth is
// measured clockwise from true north (matching a phone compass heading).
type horizontal struct {
	AltDeg float64
	AzDeg  float64
}

// localSiderealTimeDeg returns the local mean sidereal time in degrees for
// the given time and longitude (east-positive).
func localSiderealTimeDeg(t time.Time, lonDeg float64) float64 {
	d := daysSinceJ2000(t)
	gmst := 280.46061837 + 360.98564736629*d
	return normalizeDeg(gmst + lonDeg)
}

// toHorizontal converts an equatorial position to alt/az for an observer at
// (latDeg, lonDeg) at time t.
func (e equatorial) toHorizontal(t time.Time, latDeg, lonDeg float64) horizontal {
	lst := localSiderealTimeDeg(t, lonDeg)
	haDeg := normalizeDeg(lst - e.RADeg)
	ha := haDeg * deg2rad
	dec := e.DecDeg * deg2rad
	lat := latDeg * deg2rad

	sinAlt := math.Sin(dec)*math.Sin(lat) + math.Cos(dec)*math.Cos(lat)*math.Cos(ha)
	alt := math.Asin(clamp(sinAlt, -1, 1))

	cosAz := (math.Sin(dec) - math.Sin(alt)*math.Sin(lat)) / (math.Cos(alt) * math.Cos(lat))
	az := math.Acos(clamp(cosAz, -1, 1))
	if math.Sin(ha) > 0 {
		az = 2*math.Pi - az
	}

	return horizontal{AltDeg: alt * rad2deg, AzDeg: az * rad2deg}
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

// angularSeparationDeg returns the great-circle angular distance in degrees
// between two horizontal positions.
func angularSeparationDeg(a, b horizontal) float64 {
	a1, a2 := a.AltDeg*deg2rad, b.AltDeg*deg2rad
	dAz := (a.AzDeg - b.AzDeg) * deg2rad
	cosC := math.Sin(a1)*math.Sin(a2) + math.Cos(a1)*math.Cos(a2)*math.Cos(dAz)
	return math.Acos(clamp(cosC, -1, 1)) * rad2deg
}
