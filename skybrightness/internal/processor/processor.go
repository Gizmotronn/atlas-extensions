// Package processor is the skybrightness service's orchestration loop: find
// citizen-science Journal submissions waiting to be processed, plate-solve
// their photo, run photometry, and write the result back onto the same
// atlas_observations record the Atlas client already created. Fieldwork's
// architecture decision applies here too -- this only ever talks to
// PocketBase as an API client (github.com/signal-k/fieldwork/pbclient),
// never as a Go import of Atlas's own backend.
package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/url"
	"time"

	"github.com/signal-k/fieldwork/pbclient"
	"github.com/signal-k/fieldwork/platesolve"
	"github.com/signal-k/fieldwork/starcat"
	"github.com/signal-k/skybrightness/photometry"
)

const observationsCollection = "atlas_observations"

// atlasObservation is the subset of atlas_observations fields this
// processor reads or writes. Field names/types must stay in lockstep with
// signal-k/atlas's pocketbase/migrations -- see:
//   - 20260716161000_atlas_write_e2e.js (base collection)
//   - 20260904120000_atlas_citizen_science_fields.js (sky_brightness_* / citizen_science_project)
//   - 20260904122000_atlas_observation_coordinates.js (latitude / longitude)
type atlasObservation struct {
	ID                        string   `json:"id"`
	ObservedAt                string   `json:"observed_at"`
	CitizenScienceProject     string   `json:"citizen_science_project"`
	Latitude                  *float64 `json:"latitude"`
	Longitude                 *float64 `json:"longitude"`
	Photo                     string   `json:"photo"`
	PhotoR2Key                string   `json:"photo_r2_key"`
	SkyBrightnessLimitingMag  *float64 `json:"sky_brightness_limiting_magnitude"`
	SkyBrightnessSourceFormat string   `json:"sky_brightness_source_format"`
}

type listResponse struct {
	Items []atlasObservation `json:"items"`
}

// Processor polls for pending submissions and processes them one at a time.
// It authenticates once as a PocketBase superuser (needed since it reads
// and writes every user's records, not just one) rather than per-request.
type Processor struct {
	client         *pbclient.Client
	superuserToken string
}

// New authenticates against PocketBase and returns a ready-to-run Processor.
func New(ctx context.Context, baseURL, adminEmail, adminPassword string) (*Processor, error) {
	client := pbclient.New(baseURL)
	auth, err := client.AuthWithPassword(ctx, adminEmail, adminPassword)
	if err != nil {
		return nil, fmt.Errorf("processor: superuser auth failed: %w", err)
	}
	return &Processor{client: client, superuserToken: auth.Token}, nil
}

// RunOnce processes every currently-pending submission and returns how many
// it handled. "Pending" means citizen_science_project is set (the client
// tagged it as a citizen-science submission) and
// sky_brightness_limiting_magnitude is still empty (this processor, or an
// earlier run of it, hasn't produced a result yet).
func (p *Processor) RunOnce(ctx context.Context) (int, error) {
	params := url.Values{}
	params.Set("filter", `citizen_science_project != "" && sky_brightness_limiting_magnitude = null`)
	params.Set("sort", "created")
	params.Set("perPage", "50")

	raw, err := p.client.ListRecords(ctx, observationsCollection, params, p.superuserToken)
	if err != nil {
		return 0, fmt.Errorf("processor: listing pending submissions: %w", err)
	}
	var list listResponse
	if err := json.Unmarshal(raw, &list); err != nil {
		return 0, fmt.Errorf("processor: decoding pending submissions: %w", err)
	}

	processed := 0
	for _, obs := range list.Items {
		if err := p.processOne(ctx, obs); err != nil {
			// One bad submission (a corrupt photo, a missing coordinate)
			// must not stop the batch -- log and move on, same reasoning
			// pullObservationsNow uses Promise.allSettled on the Atlas
			// client side for exactly this failure isolation.
			log.Printf("processor: submission %s failed: %v", obs.ID, err)
			continue
		}
		processed++
	}
	return processed, nil
}

func (p *Processor) processOne(ctx context.Context, obs atlasObservation) error {
	if obs.Latitude == nil || obs.Longitude == nil {
		return p.writeResult(ctx, obs.ID, photometry.Estimate{
			FlaggedForReview: true,
			FlagReason:       "submission has no coordinates to plate-solve against",
		}, "unknown")
	}

	filename := obs.Photo
	if filename == "" {
		// TODO: photos uploaded via Atlas's private R2 media path
		// (photo_r2_key) aren't fetchable through PocketBase's plain file
		// API -- this processor currently only handles the legacy
		// same-instance PocketBase attachment path (sync.ts's `!useR2`
		// fallback). Reading R2 objects needs the same signed-URL/worker
		// auth Atlas's own client uses; wiring that up is follow-up work,
		// not something to fake here.
		if obs.PhotoR2Key != "" {
			return fmt.Errorf("submission's photo is in R2 (%s) -- R2 fetch is not yet implemented, see TODO", obs.PhotoR2Key)
		}
		return fmt.Errorf("submission has no photo attached")
	}

	imgBytes, err := p.client.GetFile(ctx, observationsCollection, obs.ID, filename, p.superuserToken)
	if err != nil {
		return fmt.Errorf("downloading photo: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return fmt.Errorf("decoding photo: %w", err)
	}

	observedAt, err := time.Parse("2006-01-02 15:04:05.000Z", obs.ObservedAt)
	if err != nil {
		// PocketBase's raw datetime format is inconsistent about the
		// fractional-second/zone tail across SDKs -- fall back to "now"
		// rather than fail the whole submission over a timestamp parse.
		observedAt = time.Now().UTC()
	}

	detections := platesolve.DetectStars(img)
	bounds := img.Bounds()
	candidate := platesolve.CandidateSky{LatDeg: *obs.Latitude, LonDeg: *obs.Longitude, Time: observedAt}

	wcs, err := platesolve.Solve(detections, bounds.Dx(), bounds.Dy(), candidate)
	if err != nil {
		return p.writeResult(ctx, obs.ID, photometry.Estimate{
			DetectedStars:    len(detections),
			FlaggedForReview: true,
			FlagReason:       "could not plate-solve this photo (no confident star match)",
		}, sourceFormat(obs))
	}

	visibleStars := starcat.Visible(*obs.Latitude, *obs.Longitude, observedAt, -2)
	matches := photometry.MatchDetections(wcs, bounds.Dx(), bounds.Dy(), detections, visibleStars)
	rawInput := sourceFormat(obs) == "raw"
	estimate := photometry.EstimateLimitingMagnitude(matches, detections, wcs.Confidence, rawInput)

	return p.writeResult(ctx, obs.ID, estimate, sourceFormat(obs))
}

func sourceFormat(obs atlasObservation) string {
	if obs.SkyBrightnessSourceFormat == "" {
		return "unknown"
	}
	return obs.SkyBrightnessSourceFormat
}

func (p *Processor) writeResult(ctx context.Context, id string, estimate photometry.Estimate, sourceFormat string) error {
	payload := map[string]any{
		"sky_brightness_stars_detected":     estimate.DetectedStars,
		"sky_brightness_flagged_for_review": estimate.FlaggedForReview,
		"sky_brightness_source_format":      sourceFormat,
	}
	if estimate.LimitingMagnitude != 0 {
		payload["sky_brightness_limiting_magnitude"] = estimate.LimitingMagnitude
	}
	if estimate.Confidence != "" {
		payload["sky_brightness_confidence"] = string(estimate.Confidence)
	}

	_, err := p.client.UpdateRecord(ctx, observationsCollection, id, payload, p.superuserToken)
	if err != nil {
		return fmt.Errorf("writing result back: %w", err)
	}
	return nil
}
