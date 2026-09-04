# skybrightness

Processes citizen-science sky-brightness submissions from Atlas's Journal
(currently Globe at Night-style light-pollution campaigns) into structured
data, automatically -- a user uploads a night-sky photo plus a comment, and
this service does the estimation work Globe at Night's own manual
star-chart-matching form would otherwise ask them to do by hand.

## How it fits together

1. In `signal-k/atlas`, a user signs up for a `light_pollution_campaign`
   event (an ordinary `atlas_tagged_events` row -- see `eventTags.ts`). Once
   it's ongoing or starting within 48h, it surfaces on the Hub
   (`getSignedUpEventsDueSoon`), and "Submit" opens the Journal capture flow
   pre-tagged with a `citizenScienceProject`.
2. The client saves the submission as a normal `atlas_observations` row
   (a Journal entry) with `citizen_science_project` set, a photo attached,
   and -- new as of this feature -- real `latitude`/`longitude` (see
   `20260904122000_atlas_observation_coordinates.js`; the existing
   `location_label` field is free text, not enough to plate-solve against).
   The `sky_brightness_*` fields are left empty at this point.
3. This service polls for exactly those rows (`citizen_science_project != ""
   && sky_brightness_limiting_magnitude = null`), downloads the photo,
   plate-solves it against Fieldwork's existing ~83k-star catalog
   (`github.com/signal-k/fieldwork/platesolve` + `starcat` -- the same code
   Fieldwork's "point your phone at the sky" feature already uses, reused
   rather than reimplemented), runs differential photometry
   (`photometry/`), and writes the result back onto the same row.
4. The next time the client pulls Journal entries, the submission shows the
   processed `sky_brightness_limiting_magnitude`/`confidence`/etc.

Same architecture decision as Fieldwork: this only ever talks to the shared
PocketBase instance as an API client (`github.com/signal-k/fieldwork/pbclient`),
never as a Go import of Atlas's own backend -- see the root
[`README.md`](../README.md) and [`docs/architecture.md`](../docs/architecture.md).

## Layout

```
photometry/            The actual estimation math -- zero-point calibration
                        and limiting-magnitude estimation from a solved WCS
                        plus raw detections. Pure, well-tested, no PocketBase
                        or network code.
internal/processor/     Orchestration: poll -> download -> plate-solve ->
                        photometry -> write back.
cmd/skybrightnessd/      Poll-loop entrypoint (not an HTTP server -- there's
                        no client-facing request to answer, unlike fieldworkd).
```

## The estimation method

See `photometry/photometry.go`'s package doc for the full method and its
sourcing (Kyba et al., arXiv:1709.09558). In short: catalog stars the
plate-solver already identified in the frame, with known true magnitudes,
calibrate a per-photo zero-point (median-based, so one bad match can't skew
it); limiting magnitude is then the more conservative of two independent
reads -- the faintest confidently matched star, and a "turnover" read of the
full detected-magnitude histogram -- so the service never overclaims how
deep a photo actually sees.

## Known gaps (not stubbed out, genuinely unimplemented)

- **R2-hosted photos aren't fetched yet.** Atlas's newer uploads go through
  a private Cloudflare R2 path (`photo_r2_key`), not PocketBase's plain file
  API. This service currently only handles the legacy same-instance
  attachment path (`sync.ts`'s `!useR2` fallback) -- see the `TODO` in
  `internal/processor/processor.go`. Submissions on the R2 path are logged
  and skipped, not silently mis-processed.
- **No Bortle-scale conversion.** `sky_brightness_bortle_estimate` is left
  unset; converting a limiting-magnitude estimate to a Bortle class is a
  well-known but separate step, not yet wired in.
- **RAW capture isn't actually requested from the client.** The
  `sky_brightness_source_format` the client sends today is always
  `'unknown'` (Atlas's `<input type="file">` capture has no way to request
  RAW) -- so `rawInput` is effectively always `false` in practice right now,
  which correctly downgrades confidence per the package doc, but a real RAW
  capture path (if one gets built client-side) is what would let this
  service report `measured` confidence instead of `modelled`/`estimated`.

## Running

```bash
cd photometry && go test ./...

PB_URL=https://signal-k-starsailors.fly.dev \
PB_ADMIN_EMAIL=liam@skinetics.tech \
PB_ADMIN_PASSWORD=ThisIsATestPassword \
  go run ./cmd/skybrightnessd
```
