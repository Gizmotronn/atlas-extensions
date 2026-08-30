package api

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func TestParseScanRequestMultipartWithImage(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("inputMode", "photo")
	_ = w.WriteField("latitude", "40.5")
	_ = w.WriteField("longitude", "-105.25")
	_ = w.WriteField("timestamp", "2026-03-20T06:00:00Z")
	_ = w.WriteField("deviceModel", "iphone_15_pro")
	fw, err := w.CreateFormFile("image", "photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("fake-jpeg-bytes")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/v1/scans", &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())

	req, img, err := parseScanRequest(r)
	if err != nil {
		t.Fatalf("parseScanRequest failed: %v", err)
	}
	if req.InputMode != "photo" || req.Latitude != 40.5 || req.Longitude != -105.25 {
		t.Fatalf("unexpected parsed request: %+v", req)
	}
	if req.DeviceModel != "iphone_15_pro" {
		t.Fatalf("expected deviceModel to round-trip, got %q", req.DeviceModel)
	}
	if string(img) != "fake-jpeg-bytes" {
		t.Fatalf("expected image bytes to round-trip, got %q", string(img))
	}
}

func TestParseScanRequestJSONStillWorks(t *testing.T) {
	body := `{"latitude":10,"longitude":20,"inputMode":"live","heading":90,"pitch":45}`
	r := httptest.NewRequest("POST", "/v1/scans", bytes.NewReader([]byte(body)))
	r.Header.Set("Content-Type", "application/json")

	req, img, err := parseScanRequest(r)
	if err != nil {
		t.Fatalf("parseScanRequest failed: %v", err)
	}
	if img != nil {
		t.Fatalf("expected no image for a JSON request, got %d bytes", len(img))
	}
	if req.Latitude != 10 || req.Longitude != 20 || req.InputMode != "live" {
		t.Fatalf("unexpected parsed request: %+v", req)
	}
	if req.Heading == nil || *req.Heading != 90 {
		t.Fatalf("expected heading to round-trip, got %+v", req.Heading)
	}
}

func TestParseScanRequestMultipartWithoutImageDegrades(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("inputMode", "photo")
	_ = w.WriteField("latitude", "1")
	_ = w.WriteField("longitude", "2")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("POST", "/v1/scans", &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())

	req, img, err := parseScanRequest(r)
	if err != nil {
		t.Fatalf("expected no error for a photo request missing an image, got %v", err)
	}
	if img != nil {
		t.Fatalf("expected nil image bytes, got %d", len(img))
	}
	if req.InputMode != "photo" {
		t.Fatalf("expected inputMode to still parse, got %q", req.InputMode)
	}
}
