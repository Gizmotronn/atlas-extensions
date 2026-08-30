# Fieldwork interface contract

Fieldwork is meant to be portable beyond Swift/Go. This document is the
canonical description of the wire contract between a Fieldwork client (iOS
today, Kotlin/web later) and the Fieldwork service, so a future port only
has to match this JSON shape — not read Go source.

Only the HTTP contract is specified here. Sky-position math itself
(`fieldwork/go/identify`) is a reference implementation; a future Kotlin/JS
port may reimplement it locally on-device rather than calling the service,
as long as the object list it produces fits the `objects` shape below.

## Auth

Every request (except `GET /healthz`) requires an `Authorization` header
carrying a **PocketBase user auth token** — the same token you get back
from Atlas's `POST {PB_URL}/auth/clerk-exchange` endpoint after signing in
with Clerk. Fieldwork's service never issues its own tokens; it only
verifies PocketBase ones.

## `POST /v1/scans`

**Live mode** (`inputMode: "live"`) sends a plain JSON body:

```jsonc
{
  "latitude": 51.5,
  "longitude": -0.12,
  "timestamp": "2026-08-30T21:00:00Z", // optional, RFC3339; defaults to server receipt time
  "inputMode": "live",
  "heading": 42.0,                     // degrees, 0 = north, clockwise
  "pitch": 51.5,                       // degrees above horizon, 0 = horizon, 90 = zenith
  "deviceModel": "iphone_16_pro"       // same enum as PocketBase users.device_models
}
```

**Photo mode** (`inputMode: "photo"`) sends `multipart/form-data` instead,
so the actual image bytes can travel alongside the same fields (as form
values, not JSON):

| field | required | notes |
| --- | --- | --- |
| `inputMode` | yes | `"photo"` |
| `latitude`, `longitude` | yes | decimal degrees |
| `timestamp` | no | RFC3339; defaults to server receipt time |
| `deviceModel` | no | same enum as live mode |
| `heading`, `pitch` | no | rarely known for a photo; omit unless the client has a real reading |
| `image` | no | the photo file (JPEG/PNG). Without it, photo mode behaves exactly like an empty live scan — the omnidirectional listing below |

The client's `latitude`/`longitude`/`timestamp` fields are always the
source of truth — the server does **not** require or trust EXIF GPS data,
since many photo-library exports strip it. It does read the photo's own
EXIF `GPSImgDirection`/`GPSDestBearing` (when present) purely as a
direction *hint* to speed up matching; nothing in the request needs to
carry that separately.

On the server, photo mode runs one of three ways, cheapest/most-precise
first, each degrading to the next when it can't produce a result:

1. **Plate-solving** — matches star positions detected in the image against
   a ~83k-star catalog constrained to the hemisphere visible from
   `latitude`/`longitude`/`timestamp`, recovering the photo's exact
   pointing (`fieldwork/go/platesolve`). Produces the richest `objects`
   list — every visible star the catalog has, not just a handful of named
   ones.
2. **EXIF direction** — if plate-solving doesn't find a confident match but
   the photo's EXIF recorded a compass heading, falls back to the same
   direction-cone listing live mode would produce for that heading with an
   assumed device pitch.
3. **Omnidirectional** — no image, no solve, no EXIF direction: everything
   currently above the horizon, sorted by altitude (`identify.Input.PointingKnown
   = false` in the Go reference).

Response body:

```jsonc
{
  "scanId": "abc123",
  "objects": [
    {
      "key": "polaris",
      "name": "Polaris",
      "type": "star",              // "star" | "planet" | "moon" | "deep_sky"
      "context": "The North Star — within about a degree of true celestial north.",
      "altitudeDeg": 51.4,
      "azimuthDeg": 359.8,
      "separationDeg": 2.1,        // only present when pointing was known (live, or a solved/EXIF-direction photo)
      "confidence": 0.86           // 0-1
    },
    {
      "key": "hip_91262",
      "name": "HIP 91262",
      "type": "star",
      "context": "Magnitude 0.0 star, identified by plate-solving this photo.",
      "altitudeDeg": 48.9,
      "azimuthDeg": 12.3,
      "separationDeg": 4.7,
      "confidence": 0.71
    }
  ],
  "events": [
    {
      "id": "sky_events_record_id",
      "kind": "meteor_shower",
      "target": "perseids",
      "title": "Perseids peak",
      "description": "...",
      "starts_at": "2026-08-30T20:00:00Z",
      "ends_at": "2026-08-31T04:00:00Z",
      "latitude": null,
      "longitude": null
    }
  ],
  "moonPhase": 0.42,               // 0 = new, 1 = full
  "advice": {
    "value": "iphone_16_pro",
    "label": "iPhone 16 Pro",
    "instructions": ["..."]
  },
  "solveMethod": "plate_solve"     // "live_pointing" | "plate_solve" | "exif_direction" | "omnidirectional"
}
```

`objects` entries from the star catalog (rather than the small named
catalog) use `"hip_<number>"` as their key, or `"star_<ra>_<dec>"` when the
catalog has no Hipparcos number for that star — either way, treat `key` as
opaque; don't parse it.

## Field/enum sources of truth

- `deviceModel` values: `backend/migrations/33_atlas_device_models.go`
  (`users.device_models` select field) — keep any new device value in sync
  across that migration, `fieldwork/go/advice/presets.json`, and this doc.
- `events[].kind` values: `backend/migrations/10_atlas_collections.go`
  (`sky_events.kind`).
- `objects[].type` values: fixed to `star | planet | moon | deep_sky` —
  extend `fieldwork/go/identify/catalog.go` rather than adding new types
  unless the taxonomy itself needs to grow.
- Plate-solved photo stars come from `fieldwork/go/starcat` (see its
  `README.md` for the catalog's source/regeneration steps), not from
  `identify/catalog.go` — the two are merged (with duplicates removed) by
  `service/internal/api.objectsFromWCS`.

## Porting notes

- **Kotlin**: see `docs/porting-kotlin.md` (not yet implemented).
- **Web/Vite**: see `docs/porting-web.md` (not yet implemented).
