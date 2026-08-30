# Fieldwork architecture

## Why this exists

Atlas's web app (`~/Navigation/atlas`) is a full observing companion:
calendar, watchlist, journal, weather, citizen science. Fieldwork is a
narrower, separate idea layered on top: point a device at the sky (or
upload a photo you already took) and get an identification + context,
inspired by the "Pokedex Field OS" concept
([Jastman/Scout-Pokedex-Field-OS](https://github.com/Jastman/Scout-Pokedex-Field-OS)).

Two things drove keeping this in a new repo instead of extending Atlas
directly:

1. **The scan feature can't be tested properly in a browser.** It needs
   real device sensors (compass, motion) and a real camera, so it belongs
   in a native app, not the existing Vite/React web app.
2. **It's meant to outlive Atlas as its only consumer.** Star Sailors has
   multiple games; the "point at something, identify it, get context and
   advice" mechanic is a pattern worth sharing, not an Atlas-only feature.
   Structuring it as its own module (Go core + Swift package + a wire
   contract) means a future SS project can adopt it without depending on
   Atlas's codebase.

## Component map

```
 iOS app (ios/Fieldwork)
   │  uses FieldworkKit (local Swift package) for models/networking/sensors
   │
   ├─ signs in ──────────────► shared PocketBase  (Atlas's existing
   │  (Clerk exchange)          /auth/clerk-exchange endpoint — unchanged)
   │
   └─ POST /v1/scans ────────► fieldworkd (service/)
                                  │
                                  ├─ live mode:  identify.Resolve()       (fieldwork/go)
                                  │
                                  ├─ photo mode: photoexif.Extract()      (fieldwork/go)
                                  │              → platesolve.Solve()     (matches against
                                  │                 starcat, ~83k stars)
                                  │              → identify.Locate()/     (converts solved
                                  │                Resolve() (fallback)    stars + named
                                  │                                        catalog into objects)
                                  │
                                  ├─ events.NearbyEvents() ─► shared PocketBase
                                  │                            (reads sky_events)
                                  ├─ advice.ForDevice()      (fieldwork/go)
                                  └─ writes fieldwork_scans / ─► shared PocketBase
                                     fieldwork_scan_results      (owns its own
                                                                  collections)
```

`fieldworkd` never imports `~/Navigation/backend`'s Go code and is never
imported by it — every interaction with the shared PocketBase instance goes
over its public REST API, using either the caller's own user token (for
everything user-scoped) or a superuser token (only for the one-time
collection-bootstrap step in `service/internal/schema`).

## Design decisions worth knowing

- **Photo identification is real plate-solving, not just EXIF metadata.**
  `fieldwork/go/platesolve` detects star-shaped bright points in the
  uploaded photo and matches their geometric pattern (rotation/scale
  -invariant "quad hashing," in the style of astrometry.net/Tetra3) against
  `fieldwork/go/starcat` — a ~83,000-star catalog (Hipparcos/HYG data,
  mag ≤ 9, embedded as a ~1.1MB binary; see `starcat/README.md` for its
  provenance and regeneration steps) restricted to the hemisphere visible
  from the request's location/time. That location/time constraint is what
  keeps this self-hostable on the existing service (no GPU, no external
  paid solving API, sub-second per scan): it turns blind all-sky
  plate-solving into a bounded local search. EXIF `GPSImgDirection` (when
  present) narrows the search further; if solving still doesn't find a
  confident match, photo mode falls back to that EXIF direction, then to
  the plain omnidirectional listing.
- **Low-precision astronomy, not a big ephemeris library**
  (`fieldwork/go/identify`). Sun/Moon/planet positions use the same style
  of approximate orbital-element formulas Paul Schlyter's classic
  "How to compute planetary positions" page uses — good to roughly
  degree-level accuracy, which is what a naked-eye "what am I pointing at"
  scan needs. This was a deliberate choice over pulling in a full Go
  astronomy package: it keeps the algorithm small enough to read in one
  sitting and easy to port line-for-line to Kotlin or TypeScript later,
  rather than depending on whether an equivalent Go-specific library has a
  Kotlin/JS counterpart.
- **Events are read, not reimplemented.** Meteor showers, eclipses, ISS
  passes, and conjunctions already exist in Atlas's `sky_events`
  PocketBase collection, populated by Atlas's own scheduled ingest
  workflow. Fieldwork reads that collection live (`fieldwork/go/events`)
  instead of building a second event-detection system.
- **Camera advice is a manual data port, not a live sync.** The device →
  instructions mapping in `fieldwork/go/advice/presets.json` is copied from
  `atlas/src/lib/devicePresets.ts`, keyed by the same enum as the shared
  PocketBase `users.device_models` field. There's no PocketBase collection
  for this data (Atlas keeps it client-side), so Fieldwork keeps its own
  copy. Update both together when device guidance changes; a follow-up
  could centralize this as an actual PocketBase collection if it starts
  drifting.
- **Fieldwork owns its schema.** `service/internal/schema` idempotently
  creates `fieldwork_scans`/`fieldwork_scan_results` on boot via
  PocketBase's admin API rather than requiring a migration PR into
  `~/Navigation/backend`. It only creates missing collections — it doesn't
  diff or migrate an existing one, so a schema change to an already-deployed
  collection still needs a manual/admin-UI step for now.

## Deferred (explicitly out of scope for this pass)

- **Android/Kotlin.** Swift only for now, per the initial scope decision.
  `fieldwork/CONTRACT.md` and `docs/porting-kotlin.md` exist so a future
  Kotlin client has an exact target to build against.
- **Web/Vite integration.** The scan feature isn't going into Atlas's
  existing web UI yet — camera/sensor access doesn't test well there, and
  Atlas is expected to become primarily an installed mobile app. See
  `docs/porting-web.md`.
- **Real Clerk iOS SDK.** `ios/Fieldwork`'s sign-in screen takes a Clerk
  session JWT directly as a development shortcut rather than embedding the
  `ClerkSwift` hosted sign-in flow — see the `TODO` in `SignInView.swift`.
- **Full Atlas parity.** No journal, calendar, watchlist, or weather in
  this app. It's a vertical slice proving the scan mechanic end-to-end
  against a real backend, not a mobile Atlas rewrite.
- **Real-photo plate-solving validation.** `fieldwork/go/platesolve`'s
  tests verify recall against synthetic star fields (real catalog stars
  projected through a known transform), including an explicit ≥51% recall
  assertion — that's not the same as confirming a genuine handheld
  night-sky photo hits that bar. Validating against real photos needs
  actual test images, which weren't available in this pass; it's the next
  thing to check before relying on photo mode for real.
- **iOS photo capture UI.** `ios/Fieldwork`'s photo scan flow still needs a
  `PhotosPicker`/camera integration that actually attaches image bytes to
  the multipart upload — `FieldworkClient`/`ScanCaptureView` were built
  before the service had a real photo pipeline to call and haven't been
  updated to send one yet.
