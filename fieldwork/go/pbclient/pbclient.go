// Package pbclient is a minimal PocketBase REST client. It deliberately
// doesn't depend on the pocketbase Go SDK or a superuser session by
// default — most calls pass through a caller-supplied bearer token so
// requests run as the authenticated user and are subject to that user's
// collection API rules, matching how the Atlas web client behaves.
package pbclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a single PocketBase instance.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New returns a Client for the given PocketBase base URL (e.g.
// "https://signal-k-starsailors.fly.dev").
func New(baseURL string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// AuthRefreshResult is the response shape of PocketBase's
// /api/collections/users/auth-refresh endpoint.
type AuthRefreshResult struct {
	Token  string `json:"token"`
	Record struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"record"`
}

// VerifyUserToken confirms a bearer token is a currently-valid PocketBase
// user auth token, returning the user id it belongs to. This is the
// Fieldwork service's substitute for its own session store: it never
// mints tokens, it only validates ones issued by PocketBase (via Atlas's
// existing /auth/clerk-exchange flow on the client side).
func (c *Client) VerifyUserToken(ctx context.Context, token string) (AuthRefreshResult, error) {
	var out AuthRefreshResult
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/collections/users/auth-refresh", nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return out, fmt.Errorf("pbclient: auth-refresh failed: %s: %s", resp.Status, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("pbclient: decoding auth-refresh response: %w", err)
	}
	return out, nil
}

// ListRecords fetches records from a collection, as caller-supplied query
// params (e.g. "filter", "sort", "page", "perPage"). token may be empty to
// use no auth (only works against collections with a public list rule).
func (c *Client) ListRecords(ctx context.Context, collection string, params url.Values, token string) (json.RawMessage, error) {
	u := fmt.Sprintf("%s/api/collections/%s/records?%s", c.BaseURL, collection, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pbclient: list %s failed: %s: %s", collection, resp.Status, string(body))
	}
	return body, nil
}

// SuperuserAuthResult is the response shape of PocketBase's
// /api/collections/_superusers/auth-with-password endpoint.
type SuperuserAuthResult struct {
	Token string `json:"token"`
}

// AuthWithPassword authenticates as a PocketBase superuser, returning a
// bearer token for background jobs that need to read/write across every
// user's records (e.g. skybrightness's processor scanning all pending
// citizen-science submissions) -- the same endpoint schema.NewEnsurer uses
// for Fieldwork's own schema bootstrap.
func (c *Client) AuthWithPassword(ctx context.Context, email, password string) (SuperuserAuthResult, error) {
	var out SuperuserAuthResult
	payload, err := json.Marshal(map[string]string{"identity": email, "password": password})
	if err != nil {
		return out, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/collections/_superusers/auth-with-password", bytes.NewReader(payload))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, err
	}
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("pbclient: superuser auth failed: %s: %s", resp.Status, string(body))
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("pbclient: decoding superuser auth response: %w", err)
	}
	return out, nil
}

// UpdateRecord patches an existing record. Used by the skybrightness
// processor to write plate-solved photometry results back onto an
// atlas_observations row after the client has already created it.
func (c *Client) UpdateRecord(ctx context.Context, collection, id string, payload any, token string) (json.RawMessage, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf("%s/api/collections/%s/records/%s", c.BaseURL, collection, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pbclient: update %s/%s failed: %s: %s", collection, id, resp.Status, string(body))
	}
	return body, nil
}

// GetFile downloads a record's file attachment (PocketBase's legacy
// same-instance file storage, distinct from Atlas's private R2 media path
// -- see the TODO in skybrightness/internal/processor for R2 support).
func (c *Client) GetFile(ctx context.Context, collection, recordID, filename, token string) ([]byte, error) {
	u := fmt.Sprintf("%s/api/files/%s/%s/%s", c.BaseURL, collection, recordID, filename)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pbclient: get file %s/%s/%s failed: %s", collection, recordID, filename, resp.Status)
	}
	return body, nil
}

// CreateRecord creates a record in the given collection, authenticated as
// token (the caller's own user token, or a superuser token for
// schema/admin operations).
func (c *Client) CreateRecord(ctx context.Context, collection string, payload any, token string) (json.RawMessage, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf("%s/api/collections/%s/records", c.BaseURL, collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("pbclient: create %s failed: %s: %s", collection, resp.Status, string(body))
	}
	return body, nil
}
