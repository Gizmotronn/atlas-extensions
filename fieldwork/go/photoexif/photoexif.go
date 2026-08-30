// Package photoexif pulls the handful of EXIF fields Fieldwork's photo scan
// mode can use as hints: where and when a photo was taken, and — when the
// device wrote it — which direction it was pointed. It's a thin layer over
// github.com/rwcarlsen/goexif; none of these fields are required, since the
// client-supplied lat/lon/timestamp in the scan request remain the source
// of truth (many photo-library exports strip GPS EXIF entirely).
package photoexif

import (
	"io"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// Hints is what could be read from a photo's EXIF data. Each field has a
// paired "Has*" flag rather than using a pointer/zero-value convention,
// since 0 is a valid heading (due north) and a valid lat/lon component.
type Hints struct {
	HasLocation bool
	LatDeg      float64
	LonDeg      float64

	HasTime bool
	Time    time.Time

	// HasDirection is true when the image itself records which way the
	// camera was pointed (GPSImgDirection, falling back to GPSDestBearing).
	// This is the strongest available hint for narrowing plate-solving's
	// candidate search — see fieldwork/go/platesolve.
	HasDirection bool
	HeadingDeg   float64
}

// Extract reads whatever EXIF hints are present in r (a JPEG's bytes).
// A decode failure (no EXIF segment at all, or a stripped/corrupt one) is
// not treated as an error worth surfacing — it just means an empty Hints,
// which callers degrade gracefully around.
func Extract(r io.Reader) Hints {
	x, err := exif.Decode(r)
	if err != nil {
		return Hints{}
	}

	var h Hints

	if lat, lon, err := x.LatLong(); err == nil {
		h.HasLocation = true
		h.LatDeg, h.LonDeg = lat, lon
	}

	if t, err := x.DateTime(); err == nil {
		h.HasTime = true
		h.Time = t
	}

	if deg, ok := rationalDeg(x, exif.GPSImgDirection); ok {
		h.HasDirection = true
		h.HeadingDeg = deg
	} else if deg, ok := rationalDeg(x, exif.GPSDestBearing); ok {
		h.HasDirection = true
		h.HeadingDeg = deg
	}

	return h
}

func rationalDeg(x *exif.Exif, name exif.FieldName) (float64, bool) {
	tag, err := x.Get(name)
	if err != nil {
		return 0, false
	}
	num, den, err := tag.Rat2(0)
	if err != nil || den == 0 {
		return 0, false
	}
	return float64(num) / float64(den), true
}
