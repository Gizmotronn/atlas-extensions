// Package starcat is a large (~83k star) RA/Dec/magnitude catalog, embedded
// as compact binary data, used for photo plate-solving matching. It's
// deliberately separate from identify's small named catalog: this data has
// no friendly names/context for most entries, and exists purely so
// fieldwork/go/platesolve has enough stars to match a real photo against.
package starcat

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"time"
)

//go:embed hipparcos.bin
var hipparcosBin []byte

// Star is one catalog entry: position and visual magnitude (lower = brighter).
type Star struct {
	RADeg  float64
	DecDeg float64
	Mag    float64
	HIP    int // Hipparcos catalog number, 0 if unknown
}

var all []Star

func init() {
	r := bytes.NewReader(hipparcosBin)

	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		panic("starcat: corrupt catalog header: " + err.Error())
	}

	all = make([]Star, 0, count)
	for i := uint32(0); i < count; i++ {
		var raBits, decBits float32
		var magTenths int16
		var hip uint32

		if err := binary.Read(r, binary.LittleEndian, &raBits); err != nil {
			panic("starcat: corrupt catalog data: " + err.Error())
		}
		if err := binary.Read(r, binary.LittleEndian, &decBits); err != nil {
			panic("starcat: corrupt catalog data: " + err.Error())
		}
		if err := binary.Read(r, binary.LittleEndian, &magTenths); err != nil {
			panic("starcat: corrupt catalog data: " + err.Error())
		}
		if err := binary.Read(r, binary.LittleEndian, &hip); err != nil {
			panic("starcat: corrupt catalog data: " + err.Error())
		}

		all = append(all, Star{
			RADeg:  float64(raBits),
			DecDeg: float64(decBits),
			Mag:    float64(magTenths) / 10,
			HIP:    int(hip),
		})
	}
}

// All returns every star in the catalog. Callers should treat the returned
// slice as read-only.
func All() []Star {
	return all
}

// Visible returns catalog stars above minAltDeg for an observer at
// (latDeg, lonDeg) at time t. This is the primary way callers narrow the
// ~83k-star catalog down to "what's actually in the sky right now" before
// doing anything more expensive with it (matching, direction filtering).
func Visible(latDeg, lonDeg float64, t time.Time, minAltDeg float64) []Star {
	var out []Star
	for _, s := range all {
		alt, _ := toHorizontal(s.RADeg, s.DecDeg, t, latDeg, lonDeg)
		if alt >= minAltDeg {
			out = append(out, s)
		}
	}
	return out
}

// VisibleInDirection is like Visible, but additionally restricted to a cone
// of half-angle radiusDeg around a compass heading (0 = north, clockwise)
// and altitude pitchDeg — used when an EXIF direction hint narrows the
// search before plate-solving.
func VisibleInDirection(latDeg, lonDeg float64, t time.Time, minAltDeg, headingDeg, pitchDeg, radiusDeg float64) []Star {
	var out []Star
	for _, s := range all {
		alt, az := toHorizontal(s.RADeg, s.DecDeg, t, latDeg, lonDeg)
		if alt < minAltDeg {
			continue
		}
		if angularSeparationDeg(alt, az, pitchDeg, headingDeg) > radiusDeg {
			continue
		}
		out = append(out, s)
	}
	return out
}
