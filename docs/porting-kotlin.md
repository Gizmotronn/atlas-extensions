# Porting Fieldwork to Kotlin (deferred)

Not implemented yet — Android support was explicitly deferred when this
repo was scaffolded, in favor of shipping the Swift/iOS vertical slice
first. This is a placeholder for whoever picks it up next.

When it's time to build a Kotlin client:

1. Start from [`../fieldwork/CONTRACT.md`](../fieldwork/CONTRACT.md) — it's
   the source of truth for the `POST /v1/scans` request/response shape and
   the auth model (PocketBase user token via Atlas's existing
   `/auth/clerk-exchange`). Don't read Go source for the wire format; read
   that doc.
2. Decide whether the Kotlin client calls the existing `fieldworkd` service
   over HTTP (fastest path — no port needed) or reimplements
   `fieldwork/go/identify`'s sky-position math on-device. The Go
   implementation was deliberately written with small, well-known
   low-precision formulas (see `docs/architecture.md`) specifically so a
   line-for-line Kotlin port is feasible if an offline/on-device mode is
   ever needed.
3. `fieldwork/go/advice/presets.json` and the `FieldworkDeviceModel` enum
   in `fieldwork/swift/FieldworkKit/Sources/FieldworkKit/DeviceModel.swift`
   are the two other things worth porting as data (not logic) — same enum
   values as PocketBase's `users.device_models` field.
4. Mirror the module boundary: a `fieldwork/kotlin/` library analogous to
   `fieldwork/swift/FieldworkKit`, kept UI-framework-agnostic so any SS
   Android app can depend on it.
