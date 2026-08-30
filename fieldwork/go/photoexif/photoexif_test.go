package photoexif

import (
	"bytes"
	"testing"
)

func TestExtractGracefulOnNonEXIFData(t *testing.T) {
	// Not a real JPEG at all — Extract should degrade to an empty Hints
	// rather than error or panic, since most callers just want "no hints"
	// for anything it can't parse (screenshots, re-encoded/stripped photos).
	h := Extract(bytes.NewReader([]byte("not a jpeg")))
	if h.HasLocation || h.HasTime || h.HasDirection {
		t.Fatalf("expected no hints from non-EXIF data, got %+v", h)
	}
}
