# atlas-extensions

A playground for Star Sailors/Atlas concepts and experiments that don't
belong in the main `signal-k/atlas` web app -- each module here is
decoupled, talking to the shared PocketBase instance only as an API client.

**Fieldwork** — a Star Sailors–wide module: point your phone at the sky (or
upload a photo) with your location and time, and get back what you're
looking at, any special events happening, and device-specific photography
advice. Atlas is the first consumer; the module is built to stay portable
to other Star Sailors games and other client languages.

**skybrightness** — processes Atlas's citizen-science Journal submissions
(currently Globe at Night-style light-pollution campaigns): a user uploads
a night-sky photo and a comment, and this service plate-solves it (reusing
Fieldwork's star catalog and plate-solving code) to estimate sky brightness
automatically, rather than asking the user to manually match star charts
the way Globe at Night's own form does. See
[`skybrightness/README.md`](skybrightness/README.md).

Forked from the "Pokedex Field OS" idea
([Jastman/Scout-Pokedex-Field-OS](https://github.com/Jastman/Scout-Pokedex-Field-OS)),
with Atlas's own look and feel (`~/Navigation/atlas-design`) and Atlas's
existing Clerk-backed account against the shared Star Sailors PocketBase
instance.

## Layout

```
fieldwork/            The portable module family
  go/                 Go module (github.com/signal-k/fieldwork): sky-position
                       math, device advice, PocketBase sky_events client
  swift/FieldworkKit/  Swift package: scan models, networking, sensors
  CONTRACT.md          Wire contract for any future Kotlin/web port
service/               Standalone Go HTTP service (fieldworkd) — the only
                       piece that talks to PocketBase with real user auth
ios/Fieldwork/          Minimal SwiftUI app (xcodegen), the current client
docs/                  Architecture notes, deferred-port stubs
skybrightness/         Citizen-science photo processor (poll loop, not an
                       HTTP service) — see skybrightness/README.md
```

This repo is **decoupled from `~/Navigation/backend`**: Fieldwork owns its
own PocketBase collections (created idempotently on service boot — see
`service/internal/schema`) and only ever talks to the shared PocketBase
instance as an API client, never as a Go import. See
[`docs/architecture.md`](docs/architecture.md) for the full picture and
[`fieldwork/CONTRACT.md`](fieldwork/CONTRACT.md) for the client/service
wire contract.

## Scope

This is a working **vertical-slice scaffold**: sign in with an Atlas
account → capture a scan (live pointing or a photo) → the service
identifies visible objects, cross-references live Atlas `sky_events`, and
returns device-specific camera advice → result screen. Full Atlas parity
(journal, calendar, watchlist) is intentionally out of scope — see
`docs/architecture.md` for what's deferred and why.

## Getting started

### Go module + service

```bash
cd fieldwork/go && go test ./...
cd ../../service && go build ./...

# Run the service locally (schema bootstrap needs PocketBase superuser creds
# — see the repo-standard local superuser login in the monorepo's CLAUDE.md)
PB_URL=http://localhost:8090 \
PB_ADMIN_EMAIL=liam@skinetics.tech \
PB_ADMIN_PASSWORD=ThisIsATestPassword \
  go run ./cmd/fieldworkd
```

### iOS app

```bash
cd ios/Fieldwork
xcodegen generate
open Fieldwork.xcodeproj   # or: xcodebuild -scheme Fieldwork -sdk iphonesimulator build
```

`FieldworkKit` is wired in as a local Swift package (`../../fieldwork/swift/FieldworkKit`),
so changes to the shared Swift models/networking/sensors code show up in the
app immediately without a separate publish step.

Sign-in currently accepts a Clerk session JWT directly (`SignInView`) as a
development shortcut — see the `TODO` there for wiring up the real
`ClerkSwift` SDK.
