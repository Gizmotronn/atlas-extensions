// Package events cross-references a scan against Atlas's shared PocketBase
// `sky_events` collection (see backend/migrations/10_atlas_collections.go),
// so Fieldwork surfaces real, currently-scheduled events — meteor showers,
// eclipses, ISS passes, conjunctions — instead of reimplementing event
// detection. This package only reads that collection; it never writes it.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/signal-k/fieldwork/pbclient"
)

// SkyEvent mirrors the fields of a PocketBase `sky_events` record that
// Fieldwork cares about.
type SkyEvent struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"` // meteor_shower | moon_phase | iss_pass | eclipse | conjunction
	Target      string    `json:"target"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Latitude    *float64  `json:"latitude"`
	Longitude   *float64  `json:"longitude"`
}

type listResponse struct {
	Items []SkyEvent `json:"items"`
}

// NearbyEvents returns sky_events active at t, within a window bucket
// (starts_at <= t <= ends_at). Location-scoped events (iss_pass) are
// further filtered client-side to roughly match the observer's position;
// global events (meteor showers, eclipses, moon phases) have no
// latitude/longitude and are always included.
func NearbyEvents(ctx context.Context, client *pbclient.Client, latDeg, lonDeg float64, t time.Time) ([]SkyEvent, error) {
	filter := fmt.Sprintf("starts_at <= %q && ends_at >= %q", t.UTC().Format(time.RFC3339), t.UTC().Format(time.RFC3339))

	params := url.Values{}
	params.Set("filter", filter)
	params.Set("sort", "starts_at")
	params.Set("perPage", "50")

	raw, err := client.ListRecords(ctx, "sky_events", params, "")
	if err != nil {
		return nil, err
	}

	var parsed listResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("events: decoding sky_events response: %w", err)
	}

	const locationToleranceDeg = 5.0 // roughly "same region", generous for a naked-eye event like an ISS pass
	out := make([]SkyEvent, 0, len(parsed.Items))
	for _, e := range parsed.Items {
		if e.Latitude != nil && e.Longitude != nil {
			if absDiff(*e.Latitude, latDeg) > locationToleranceDeg || absDiff(*e.Longitude, lonDeg) > locationToleranceDeg {
				continue
			}
		}
		out = append(out, e)
	}
	return out, nil
}

func absDiff(a, b float64) float64 {
	d := a - b
	if d < 0 {
		return -d
	}
	return d
}
