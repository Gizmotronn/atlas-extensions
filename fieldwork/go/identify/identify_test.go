package identify

import (
	"testing"
	"time"
)

// Full moon on 2026-08-28 ~04:19 UTC (published almanac data). Illuminated
// fraction should be close to 1.
func TestMoonPhaseNearFull(t *testing.T) {
	full := time.Date(2026, 8, 28, 4, 19, 0, 0, time.UTC)
	frac := MoonPhase(full)
	if frac < 0.97 {
		t.Fatalf("expected near-full moon (>=0.97), got %.3f", frac)
	}
}

// New moon on 2026-08-13 ~00:37 UTC. Illuminated fraction should be close
// to 0.
func TestMoonPhaseNearNew(t *testing.T) {
	new := time.Date(2026, 8, 13, 0, 37, 0, 0, time.UTC)
	frac := MoonPhase(new)
	if frac > 0.03 {
		t.Fatalf("expected near-new moon (<=0.03), got %.3f", frac)
	}
}

// Polaris should always sit within a couple of degrees of true north at any
// latitude in the northern hemisphere, and its altitude should roughly
// track observer latitude.
func TestResolvePolarisAltitudeTracksLatitude(t *testing.T) {
	now := time.Date(2026, 6, 1, 22, 0, 0, 0, time.UTC)
	in := Input{LatDeg: 51.5, LonDeg: -0.12, Time: now}

	results := Resolve(in)

	var polaris *Object
	for i := range results {
		if results[i].Key == "polaris" {
			polaris = &results[i]
			break
		}
	}
	if polaris == nil {
		t.Fatal("expected polaris in results")
	}
	if diff := polaris.AltitudeDeg - 51.5; diff < -3 || diff > 3 {
		t.Fatalf("expected polaris altitude near latitude (51.5), got %.2f", polaris.AltitudeDeg)
	}
}

func TestResolvePointingFiltersToFOV(t *testing.T) {
	now := time.Date(2026, 6, 1, 22, 0, 0, 0, time.UTC)
	in := Input{
		LatDeg:        51.5,
		LonDeg:        -0.12,
		Time:          now,
		Heading:       0,
		Pitch:         51.5, // pointed roughly at Polaris
		PointingKnown: true,
		FOVDeg:        10,
	}

	results := Resolve(in)
	for _, r := range results {
		if r.SeparationDeg > 10 {
			t.Fatalf("object %s outside requested FOV: separation=%.2f", r.Key, r.SeparationDeg)
		}
	}

	found := false
	for _, r := range results {
		if r.Key == "polaris" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected polaris to be in view when pointed north at latitude altitude")
	}
}
