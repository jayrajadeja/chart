package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestLivePageServed(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{}), http.MethodGet, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type=%q, want text/html", ct)
	}
	body := rec.Body.String()
	for _, marker := range []string{"EventSource", "/v1/stream", "\"frame\"", "<pre"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("live page missing %q", marker)
		}
	}
}

func TestLivePageMethodNotAllowed(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{}), http.MethodPost, "/")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d, want 405", rec.Code)
	}
}

func TestUnknownRootPathIsNotFound(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{}), http.MethodGet, "/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d, want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "EventSource") {
		t.Fatal("unknown path served the live page")
	}
}

// Registering "/" must not break routing to the specific endpoints.
func TestSpecificRoutesStillRouteWithRoot(t *testing.T) {
	h := newTestHandler(&fakeSource{candles: sampleCandles()})

	if rec := do(h, http.MethodGet, "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("/healthz code=%d, want 200", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/v1/chart?symbol=SYNTH&width=60"); rec.Code != http.StatusOK {
		t.Fatalf("/v1/chart code=%d, want 200", rec.Code)
	}
	// /v1/stream over a plain fakeSource (no StreamSource) → 501, proving the
	// request reached the stream handler and not the root page.
	if rec := do(h, http.MethodGet, "/v1/stream?symbol=SYNTH&width=60"); rec.Code != http.StatusNotImplemented {
		t.Fatalf("/v1/stream code=%d, want 501", rec.Code)
	}
}
