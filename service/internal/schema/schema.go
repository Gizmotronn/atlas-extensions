// Package schema idempotently ensures Fieldwork's own PocketBase
// collections exist on the shared instance. Fieldwork owns this schema
// itself (rather than requiring a migration PR into ~/Navigation/backend)
// so the service stays decoupled, per the project's Fieldwork architecture
// decision: backend logic lives in this repo, talking to PocketBase purely
// as a client.
package schema

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// field mirrors the subset of PocketBase's collection-import field schema
// that Fieldwork's collections use (PocketBase v0.39 JSON collection
// import format).
type field struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Required      bool     `json:"required,omitempty"`
	CollectionID  string   `json:"collectionId,omitempty"`
	CascadeDelete bool     `json:"cascadeDelete,omitempty"`
	MaxSelect     int      `json:"maxSelect,omitempty"`
	Values        []string `json:"values,omitempty"`
}

type collectionDef struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Fields     []field `json:"fields"`
	ListRule   *string `json:"listRule"`
	ViewRule   *string `json:"viewRule"`
	CreateRule *string `json:"createRule"`
	UpdateRule *string `json:"updateRule"`
	DeleteRule *string `json:"deleteRule"`
}

func rule(expr string) *string { return &expr }

// Ensurer authenticates once as a PocketBase superuser and can then ensure
// Fieldwork's collections exist.
type Ensurer struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// NewEnsurer authenticates against PocketBase's superuser auth endpoint
// (POST /api/collections/_superusers/auth-with-password, the PocketBase
// v0.39 convention this ecosystem's backend already uses).
func NewEnsurer(ctx context.Context, baseURL, email, password string) (*Ensurer, error) {
	e := &Ensurer{baseURL: baseURL, httpClient: &http.Client{Timeout: 10 * time.Second}}

	body, _ := json.Marshal(map[string]string{"identity": email, "password": password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/collections/_superusers/auth-with-password", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("schema: superuser auth failed: %s: %s", resp.Status, string(b))
	}

	var parsed struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	e.token = parsed.Token
	return e, nil
}

// EnsureAll creates fieldwork_scans and fieldwork_scan_results if they
// don't already exist. It does not update an existing collection's schema
// — this is a bootstrap step, not a migration system.
func (e *Ensurer) EnsureAll(ctx context.Context) error {
	scans := collectionDef{
		Name: "fieldwork_scans",
		Type: "base",
		Fields: []field{
			{Name: "user", Type: "relation", Required: true, CollectionID: "_pb_users_auth_", CascadeDelete: true, MaxSelect: 1},
			{Name: "captured_at", Type: "date", Required: true},
			{Name: "latitude", Type: "number", Required: true},
			{Name: "longitude", Type: "number", Required: true},
			{Name: "heading", Type: "number"},
			{Name: "pitch", Type: "number"},
			{Name: "input_mode", Type: "select", Required: true, MaxSelect: 1, Values: []string{"live", "photo"}},
			{Name: "device_model", Type: "text"},
			{Name: "status", Type: "select", Required: true, MaxSelect: 1, Values: []string{"pending", "processed", "failed"}},
		},
		ListRule:   rule("user = @request.auth.id"),
		ViewRule:   rule("user = @request.auth.id"),
		CreateRule: rule("@request.auth.id != '' && user = @request.auth.id"),
		UpdateRule: rule("user = @request.auth.id"),
		DeleteRule: rule("user = @request.auth.id"),
	}
	if err := e.ensureCollection(ctx, scans); err != nil {
		return fmt.Errorf("schema: ensure fieldwork_scans: %w", err)
	}

	scanID, err := e.collectionID(ctx, "fieldwork_scans")
	if err != nil {
		return fmt.Errorf("schema: resolve fieldwork_scans id: %w", err)
	}

	results := collectionDef{
		Name: "fieldwork_scan_results",
		Type: "base",
		Fields: []field{
			{Name: "scan", Type: "relation", Required: true, CollectionID: scanID, CascadeDelete: true, MaxSelect: 1},
			{Name: "object_key", Type: "text", Required: true},
			{Name: "object_type", Type: "select", Required: true, MaxSelect: 1, Values: []string{"star", "planet", "moon", "deep_sky", "event"}},
			{Name: "display_name", Type: "text", Required: true},
			{Name: "confidence", Type: "number"},
			{Name: "context_text", Type: "text"},
			{Name: "advice_text", Type: "text"},
		},
		ListRule:   rule("scan.user = @request.auth.id"),
		ViewRule:   rule("scan.user = @request.auth.id"),
		CreateRule: rule("@request.auth.id != '' && scan.user = @request.auth.id"),
		UpdateRule: rule("scan.user = @request.auth.id"),
		DeleteRule: rule("scan.user = @request.auth.id"),
	}
	if err := e.ensureCollection(ctx, results); err != nil {
		return fmt.Errorf("schema: ensure fieldwork_scan_results: %w", err)
	}

	return nil
}

func (e *Ensurer) collectionID(ctx context.Context, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/api/collections/"+name, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", e.token)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get collection %s: %s: %s", name, resp.Status, string(b))
	}

	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	return parsed.ID, nil
}

func (e *Ensurer) ensureCollection(ctx context.Context, def collectionDef) error {
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+"/api/collections/"+def.Name, nil)
	if err != nil {
		return err
	}
	getReq.Header.Set("Authorization", e.token)

	getResp, err := e.httpClient.Do(getReq)
	if err != nil {
		return err
	}
	getResp.Body.Close()

	if getResp.StatusCode == http.StatusOK {
		return nil // already exists; not diffing/updating schema on boot
	}

	body, err := json.Marshal(def)
	if err != nil {
		return err
	}

	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/collections", bytes.NewReader(body))
	if err != nil {
		return err
	}
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("Authorization", e.token)

	postResp, err := e.httpClient.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK && postResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(postResp.Body)
		return fmt.Errorf("create collection %s failed: %s: %s", def.Name, postResp.Status, string(b))
	}
	return nil
}
