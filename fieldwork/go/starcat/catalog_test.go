package starcat

import (
	"testing"
	"time"
)

func TestAllLoadsCatalog(t *testing.T) {
	stars := All()
	if len(stars) < 50000 {
		t.Fatalf("expected tens of thousands of stars, got %d", len(stars))
	}
	for _, s := range stars[:100] {
		if s.RADeg < 0 || s.RADeg > 360 {
			t.Fatalf("RA out of range: %+v", s)
		}
		if s.DecDeg < -90 || s.DecDeg > 90 {
			t.Fatalf("Dec out of range: %+v", s)
		}
	}
}

func TestVisibleFiltersByAltitude(t *testing.T) {
	// Same fixture location/time style as identify's tests: a fixed
	// mid-latitude observer at a fixed instant.
	loc := struct{ lat, lon float64 }{lat: 40.0, lon: -105.0}
	when := time.Date(2026, 3, 20, 6, 0, 0, 0, time.UTC)

	above := Visible(loc.lat, loc.lon, when, 0)
	aboveHigh := Visible(loc.lat, loc.lon, when, 60)

	if len(above) == 0 {
		t.Fatal("expected some stars above the horizon")
	}
	if len(aboveHigh) >= len(above) {
		t.Fatalf("raising the altitude cutoff should shrink the result set: %d vs %d", len(aboveHigh), len(above))
	}
}

func TestVisibleInDirectionNarrowsFurther(t *testing.T) {
	when := time.Date(2026, 3, 20, 6, 0, 0, 0, time.UTC)

	wide := Visible(40.0, -105.0, when, 0)
	narrow := VisibleInDirection(40.0, -105.0, when, 0, 90, 30, 20)

	if len(narrow) >= len(wide) {
		t.Fatalf("directional cone should be a strict subset: narrow=%d wide=%d", len(narrow), len(wide))
	}
	for _, s := range narrow {
		found := false
		for _, w := range wide {
			if w.HIP == s.HIP && w.RADeg == s.RADeg {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("narrow result not present in wide result: %+v", s)
		}
	}
}
