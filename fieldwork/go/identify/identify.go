package identify

import (
	"sort"
	"time"
)

// Input describes a scan request: where and when the observer is, and
// optionally which direction the device is pointed.
//
// Pointing (HeadingDeg/PitchDeg) is optional: when both are zero-value and
// PointingKnown is false, Resolve falls back to "what's up right now" mode
// and returns every object currently above the horizon, sorted by altitude,
// instead of filtering by direction. This covers the upload-a-photo path
// where heading usually isn't in the EXIF data.
type Input struct {
	LatDeg  float64
	LonDeg  float64
	Time    time.Time
	Heading float64 // compass heading in degrees, 0 = north, clockwise
	Pitch   float64 // device tilt above horizon in degrees, 0 = horizon, 90 = zenith

	PointingKnown bool // true when Heading/Pitch come from a live sensor reading

	// FOVDeg is the half-angle tolerance (degrees) used to decide whether an
	// object counts as "in view" when PointingKnown is true. Defaults to 15
	// degrees (a generous phone-camera field of view) when zero.
	FOVDeg float64
}

// Object is one identified sky object in a scan result.
type Object struct {
	Key           string  `json:"key"`
	Name          string  `json:"name"`
	Type          string  `json:"type"` // "star" | "planet" | "moon" | "deep_sky"
	Context       string  `json:"context"`
	AltitudeDeg   float64 `json:"altitudeDeg"`
	AzimuthDeg    float64 `json:"azimuthDeg"`
	SeparationDeg float64 `json:"separationDeg,omitempty"` // angular distance from pointing direction, when known
	Confidence    float64 `json:"confidence"`              // 0-1
}

// Horizontal converts an RA/Dec position to altitude/azimuth (degrees,
// azimuth clockwise from true north) for the observer/time described by in.
// Exposed so callers can turn a solved photo pointing (e.g. a plate-solved
// WCS center) into the Heading/Pitch an Input expects.
func Horizontal(raDeg, decDeg float64, in Input) (altDeg, azDeg float64) {
	h := equatorial{RADeg: raDeg, DecDeg: decDeg}.toHorizontal(in.Time, in.LatDeg, in.LonDeg)
	return h.AltDeg, h.AzDeg
}

// Locate computes the Object entry for a fixed RA/Dec position with respect
// to in — the alt/az conversion, FOV filtering (when in.PointingKnown), and
// confidence scoring Resolve uses for its own catalog, exposed so other
// callers that already know an object's celestial coordinates (notably the
// photo plate-solving path, which has real RA/Dec positions for thousands
// of matched stars rather than a small fixed catalog) get identical,
// consistent results. ok is false when the object is below the horizon, or
// outside in.FOVDeg when pointing is known.
func Locate(key, name, objType, context string, raDeg, decDeg float64, in Input) (Object, bool) {
	if in.FOVDeg <= 0 {
		in.FOVDeg = 15
	}
	pointing := horizontal{AltDeg: in.Pitch, AzDeg: in.Heading}

	h := equatorial{RADeg: raDeg, DecDeg: decDeg}.toHorizontal(in.Time, in.LatDeg, in.LonDeg)
	if h.AltDeg < -1 { // a degree of margin for atmospheric refraction near the horizon
		return Object{}, false
	}

	obj := Object{
		Key:         key,
		Name:        name,
		Type:        objType,
		Context:     context,
		AltitudeDeg: h.AltDeg,
		AzimuthDeg:  h.AzDeg,
	}

	if in.PointingKnown {
		sep := angularSeparationDeg(h, pointing)
		if sep > in.FOVDeg {
			return Object{}, false
		}
		obj.SeparationDeg = sep
		obj.Confidence = clamp(1-sep/in.FOVDeg, 0, 1)
	} else {
		// Without a pointing direction, confidence just reflects how easy
		// the object is to spot (higher in the sky = easier).
		obj.Confidence = clamp(h.AltDeg/90, 0, 1)
	}

	return obj, true
}

// Resolve returns the sky objects visible/relevant to the given input,
// sorted by relevance (in-view separation when pointing is known, otherwise
// altitude — higher/more overhead objects first).
func Resolve(in Input) []Object {
	if in.FOVDeg <= 0 {
		in.FOVDeg = 15
	}

	var results []Object

	appendCandidate := func(key, name, objType, context string, eq equatorial) {
		if obj, ok := Locate(key, name, objType, context, eq.RADeg, eq.DecDeg, in); ok {
			results = append(results, obj)
		}
	}

	appendCandidate("sun", "the Sun", "star", "Never look directly at the Sun, even through a phone camera.", sunPosition(in.Time))
	appendCandidate("moon", "the Moon", "moon", solarSystemObjects[0].Context, moonPosition(in.Time))

	for _, p := range solarSystemObjects[1:] {
		if eq, ok := planetPosition(p.Key, in.Time); ok {
			appendCandidate(p.Key, p.Name, "planet", p.Context, eq)
		}
	}

	for _, c := range catalog {
		appendCandidate(c.Key, c.Name, c.Type, c.Context, equatorial{RADeg: c.RADeg, DecDeg: c.DecDeg})
	}

	sort.Slice(results, func(i, j int) bool {
		if in.PointingKnown {
			return results[i].SeparationDeg < results[j].SeparationDeg
		}
		return results[i].AltitudeDeg > results[j].AltitudeDeg
	})

	return results
}

// MoonPhase returns the Moon's illuminated fraction (0 = new, 1 = full) at t.
func MoonPhase(t time.Time) float64 {
	return moonPhaseFraction(t)
}
