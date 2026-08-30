# Porting Fieldwork to web/Vite (deferred)

Not implemented yet, and deliberately so: the scan feature depends on
device camera/compass/motion sensors that don't test well in a browser,
and Atlas is expected to become primarily an installed mobile app with the
web app as a secondary surface. Revisit this once the iOS scan flow has
been validated with real users.

When it's time to add a web client (either into `~/Navigation/atlas`
directly, or a standalone `fieldwork/web` package):

1. Start from [`../fieldwork/CONTRACT.md`](../fieldwork/CONTRACT.md) for
   the `POST /v1/scans` request/response shape and auth model — it's
   framework-agnostic on purpose.
2. Atlas's web client already has an astronomy-engine-based sky-position
   layer (see `atlas/src/lib` — the planetarium/full-screen map code
   referenced in `atlas/README.md`). Before porting `fieldwork/go/identify`
   to TypeScript, check whether that existing code can be reused or
   adapted instead of duplicating a third implementation of the same math.
3. Browser sensor access (`DeviceOrientationEvent`, geolocation) needs a
   permissions/HTTPS story and graceful degradation — likely photo-upload
   mode (EXIF-only, no live pointing) is the more realistic web entry
   point rather than live compass scanning.
