// Package api implements Fieldwork's HTTP surface: the only piece of the
// scaffold that talks to PocketBase with real user auth. See
// fieldwork/CONTRACT.md for the request/response shapes any future
// Kotlin/web port must match.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/signal-k/fieldwork/advice"
	"github.com/signal-k/fieldwork/events"
	"github.com/signal-k/fieldwork/identify"
	"github.com/signal-k/fieldwork/pbclient"
	"github.com/signal-k/fieldwork/photoexif"
	"github.com/signal-k/fieldwork/platesolve"
	"github.com/signal-k/fieldwork/starcat"
)

// Server holds the dependencies HTTP handlers need.
type Server struct {
	PB     *pbclient.Client
	Logger *slog.Logger
}

func NewServer(pb *pbclient.Client, logger *slog.Logger) *Server {
	return &Server{PB: pb, Logger: logger}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /v1/scans", s.handleCreateScan)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// scanRequest is the client-facing scan input. See fieldwork/CONTRACT.md.
type scanRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	// Timestamp is RFC3339; when empty the server uses request receipt time.
	Timestamp string `json:"timestamp,omitempty"`

	InputMode string `json:"inputMode"` // "live" | "photo"

	// Present when InputMode == "live" (from CoreMotion/CoreLocation on iOS).
	Heading *float64 `json:"heading,omitempty"`
	Pitch   *float64 `json:"pitch,omitempty"`

	DeviceModel string `json:"deviceModel,omitempty"`
}

type scanResponse struct {
	ScanID      string            `json:"scanId"`
	Objects     []identify.Object `json:"objects"`
	Events      []events.SkyEvent `json:"events"`
	MoonPhase   float64           `json:"moonPhase"`
	Advice      advice.Preset     `json:"advice"`
	SolveMethod string            `json:"solveMethod"` // "plate_solve" | "exif_direction" | "live_pointing" | "omnidirectional"
}

func (s *Server) handleCreateScan(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing Authorization header (PocketBase user token)")
		return
	}

	auth, err := s.PB.VerifyUserToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	req, imageBytes, err := parseScanRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.InputMode != "live" && req.InputMode != "photo" {
		writeError(w, http.StatusBadRequest, "inputMode must be \"live\" or \"photo\"")
		return
	}

	ts := time.Now().UTC()
	if req.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339, req.Timestamp)
		if err != nil {
			writeError(w, http.StatusBadRequest, "timestamp must be RFC3339")
			return
		}
		ts = parsed
	}

	var objects []identify.Object
	solveMethod := "omnidirectional"

	switch {
	case req.InputMode == "live" && req.Heading != nil && req.Pitch != nil:
		in := identify.Input{
			LatDeg: req.Latitude, LonDeg: req.Longitude, Time: ts,
			Heading: *req.Heading, Pitch: *req.Pitch, PointingKnown: true,
		}
		objects = identify.Resolve(in)
		solveMethod = "live_pointing"

	case req.InputMode == "photo" && len(imageBytes) > 0:
		objects, solveMethod = s.resolvePhotoScan(r, imageBytes, req, ts)

	default:
		objects = identify.Resolve(identify.Input{LatDeg: req.Latitude, LonDeg: req.Longitude, Time: ts})
	}

	nearby, err := events.NearbyEvents(r.Context(), s.PB, req.Latitude, req.Longitude, ts)
	if err != nil {
		// Events are supplementary context — degrade gracefully rather than
		// failing the whole scan if the shared PocketBase is unreachable.
		s.Logger.WarnContext(r.Context(), "fetching nearby sky_events failed", "error", err)
		nearby = nil
	}

	preset := advice.ForDevice(req.DeviceModel)

	scanRecord := map[string]any{
		"user":         auth.Record.ID,
		"captured_at":  ts.Format(time.RFC3339),
		"latitude":     req.Latitude,
		"longitude":    req.Longitude,
		"heading":      req.Heading,
		"pitch":        req.Pitch,
		"input_mode":   req.InputMode,
		"device_model": req.DeviceModel,
		"solve_method": solveMethod,
		"status":       "processed",
	}
	scanRaw, err := s.PB.CreateRecord(r.Context(), "fieldwork_scans", scanRecord, token)
	if err != nil {
		s.Logger.ErrorContext(r.Context(), "creating fieldwork_scans record failed", "error", err)
		writeError(w, http.StatusBadGateway, "could not persist scan")
		return
	}

	var created struct {
		ID string `json:"id"`
	}
	scanID := ""
	if err := json.Unmarshal(scanRaw, &created); err == nil {
		scanID = created.ID
	}

	if scanID != "" {
		s.persistResults(r.Context(), scanID, objects, token)
	}

	writeJSON(w, http.StatusOK, scanResponse{
		ScanID:      scanID,
		Objects:     objects,
		Events:      nearby,
		MoonPhase:   identify.MoonPhase(ts),
		Advice:      preset,
		SolveMethod: solveMethod,
	})
}

// parseScanRequest reads a scan request from either a plain JSON body (live
// mode, as today) or multipart/form-data (photo mode, when an image is
// attached) — see fieldwork/CONTRACT.md for the exact multipart field
// names. imageBytes is nil unless the request was multipart and included an
// "image" file part.
func parseScanRequest(r *http.Request) (scanRequest, []byte, error) {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/form-data") {
		var req scanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return scanRequest{}, nil, fmt.Errorf("invalid request body")
		}
		return req, nil, nil
	}

	const maxUploadBytes = 20 << 20 // 20MB — comfortably covers a phone camera JPEG
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		return scanRequest{}, nil, fmt.Errorf("invalid multipart form: %w", err)
	}

	var req scanRequest
	req.InputMode = r.FormValue("inputMode")
	req.Timestamp = r.FormValue("timestamp")
	req.DeviceModel = r.FormValue("deviceModel")

	lat, err := strconv.ParseFloat(r.FormValue("latitude"), 64)
	if err != nil {
		return scanRequest{}, nil, fmt.Errorf("latitude must be a number")
	}
	req.Latitude = lat

	lon, err := strconv.ParseFloat(r.FormValue("longitude"), 64)
	if err != nil {
		return scanRequest{}, nil, fmt.Errorf("longitude must be a number")
	}
	req.Longitude = lon

	if v := r.FormValue("heading"); v != "" {
		h, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return scanRequest{}, nil, fmt.Errorf("heading must be a number")
		}
		req.Heading = &h
	}
	if v := r.FormValue("pitch"); v != "" {
		p, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return scanRequest{}, nil, fmt.Errorf("pitch must be a number")
		}
		req.Pitch = &p
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		// No image part — the caller degrades to the omnidirectional path,
		// same as photo mode has always done without a real photo.
		return req, nil, nil
	}
	defer file.Close()

	imgBytes, err := io.ReadAll(file)
	if err != nil {
		return scanRequest{}, nil, fmt.Errorf("reading uploaded image failed: %w", err)
	}
	return req, imgBytes, nil
}

// resolvePhotoScan runs the photo-mode identification pipeline: plate-solve
// the uploaded image against the star catalog (constrained to the
// hemisphere visible from req's location/time), falling back to an EXIF
// direction hint, and finally to the plain omnidirectional listing if
// neither pans out. See docs/architecture.md for why this ordering exists.
func (s *Server) resolvePhotoScan(r *http.Request, imageBytes []byte, req scanRequest, ts time.Time) ([]identify.Object, string) {
	hints := photoexif.Extract(bytes.NewReader(imageBytes))

	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		s.Logger.WarnContext(r.Context(), "decoding uploaded photo failed", "error", err)
		return s.fallbackPhotoObjects(req, ts, hints), fallbackSolveMethod(hints)
	}

	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()

	detections := platesolve.DetectStars(img)

	candidate := platesolve.CandidateSky{LatDeg: req.Latitude, LonDeg: req.Longitude, Time: ts}
	if hints.HasDirection {
		h := hints.HeadingDeg
		candidate.HeadingDeg = &h
	}

	wcs, err := platesolve.Solve(detections, imgW, imgH, candidate)
	if err != nil {
		s.Logger.InfoContext(r.Context(), "plate-solve did not find a confident match", "error", err, "detections", len(detections))
		return s.fallbackPhotoObjects(req, ts, hints), fallbackSolveMethod(hints)
	}

	return objectsFromWCS(wcs, req, ts, imgW, imgH), "plate_solve"
}

// fallbackPhotoObjects is what a photo scan falls back to when plate-solving
// doesn't produce a confident match: an EXIF compass-direction cone if the
// photo recorded one, otherwise the same omnidirectional "what's up right
// now" listing photo mode has always used.
func (s *Server) fallbackPhotoObjects(req scanRequest, ts time.Time, hints photoexif.Hints) []identify.Object {
	if !hints.HasDirection {
		return identify.Resolve(identify.Input{LatDeg: req.Latitude, LonDeg: req.Longitude, Time: ts})
	}
	return identify.Resolve(identify.Input{
		LatDeg: req.Latitude, LonDeg: req.Longitude, Time: ts,
		Heading: hints.HeadingDeg, Pitch: 35, // pitch is unknown; a moderate default keeps the FOV cone above the horizon
		PointingKnown: true, FOVDeg: 35,
	})
}

func fallbackSolveMethod(hints photoexif.Hints) string {
	if hints.HasDirection {
		return "exif_direction"
	}
	return "omnidirectional"
}

// objectsFromWCS turns a solved photo pointing into the same Object shape
// live-pointing scans use: the small named catalog (Sun/Moon/planets/bright
// named stars) via identify.Resolve, plus every starcat star that actually
// falls inside the solved frame, labeled by Hipparcos number when it has no
// friendly name. Stars already present via the named catalog are skipped by
// comparing alt/az (RA/Dec differ slightly between the two catalogs' star
// positions, so alt/az proximity is the more forgiving dedupe key).
func objectsFromWCS(wcs *platesolve.WCS, req scanRequest, ts time.Time, imgW, imgH int) []identify.Object {
	altDeg, azDeg := identify.Horizontal(wcs.CenterRADeg, wcs.CenterDecDeg, identify.Input{
		LatDeg: req.Latitude, LonDeg: req.Longitude, Time: ts,
	})

	diagDeg := (wcs.ArcsecPerPixel * math.Hypot(float64(imgW), float64(imgH))) / 3600
	fovHalfDiag := diagDeg / 2
	if fovHalfDiag <= 0 {
		fovHalfDiag = 20
	}

	in := identify.Input{
		LatDeg: req.Latitude, LonDeg: req.Longitude, Time: ts,
		Heading: azDeg, Pitch: altDeg, PointingKnown: true, FOVDeg: fovHalfDiag,
	}

	objects := identify.Resolve(in)

	const dedupeToleranceDeg = 0.2
	isDuplicate := func(altD, azD float64) bool {
		for _, o := range objects {
			dAlt := o.AltitudeDeg - altD
			dAz := o.AzimuthDeg - azD
			if dAlt*dAlt+dAz*dAz < dedupeToleranceDeg*dedupeToleranceDeg {
				return true
			}
		}
		return false
	}

	stars := starcat.VisibleInDirection(req.Latitude, req.Longitude, ts, -2, azDeg, altDeg, fovHalfDiag+2)
	for _, star := range stars {
		obj, ok := identify.Locate(starKey(star), starName(star), "star", starContext(star), star.RADeg, star.DecDeg, in)
		if !ok || isDuplicate(obj.AltitudeDeg, obj.AzimuthDeg) {
			continue
		}
		objects = append(objects, obj)
	}

	sort.Slice(objects, func(i, j int) bool { return objects[i].SeparationDeg < objects[j].SeparationDeg })
	return objects
}

func starKey(s starcat.Star) string {
	if s.HIP != 0 {
		return fmt.Sprintf("hip_%d", s.HIP)
	}
	return fmt.Sprintf("star_%.4f_%.4f", s.RADeg, s.DecDeg)
}

func starName(s starcat.Star) string {
	if s.HIP != 0 {
		return fmt.Sprintf("HIP %d", s.HIP)
	}
	return fmt.Sprintf("Star at RA %.2f°, Dec %.2f°", s.RADeg, s.DecDeg)
}

func starContext(s starcat.Star) string {
	return fmt.Sprintf("Magnitude %.1f star, identified by plate-solving this photo.", s.Mag)
}

// persistResults writes each identified object as a fieldwork_scan_results
// record. Failures are logged, not fatal — the caller already has the scan
// result in the response even if a couple of rows fail to persist.
func (s *Server) persistResults(ctx context.Context, scanID string, objects []identify.Object, token string) {
	for _, obj := range objects {
		record := map[string]any{
			"scan":         scanID,
			"object_key":   obj.Key,
			"object_type":  obj.Type,
			"display_name": obj.Name,
			"confidence":   obj.Confidence,
			"context_text": obj.Context,
		}
		if _, err := s.PB.CreateRecord(ctx, "fieldwork_scan_results", record, token); err != nil {
			s.Logger.WarnContext(ctx, "creating fieldwork_scan_results record failed", "error", err, "object", obj.Key)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
