package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jayrajadeja/chart/chart"
)

// fakeSource returns canned candles or an error, and records the last call.
type fakeSource struct {
	candles []chart.Candle
	err     error
	symbol  string
	width   int64
	from    int64
	to      int64
}

func (f *fakeSource) Candles(symbol string, width, from, to int64) ([]chart.Candle, error) {
	f.symbol, f.width, f.from, f.to = symbol, width, from, to
	return f.candles, f.err
}

func sampleCandles() []chart.Candle {
	return []chart.Candle{
		{Start: 0, Open: 10001, High: 10005, Low: 9996, Close: 9998},
		{Start: 60, Open: 9997, High: 10004, Low: 9995, Close: 10000},
	}
}

func newTestHandler(src *fakeSource) http.Handler {
	return Handler(src, Defaults{Height: 20, UpDown: true})
}

func do(h http.Handler, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{}), http.MethodGet, "/healthz")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("healthz: code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestChartHappyPath(t *testing.T) {
	src := &fakeSource{candles: sampleCandles()}
	rec := do(newTestHandler(src), http.MethodGet, "/v1/chart?symbol=SYNTH&width=60&from=0&to=100")
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type=%q", ct)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty body")
	}
	if src.symbol != "SYNTH" || src.width != 60 || src.from != 0 || src.to != 100 {
		t.Fatalf("source call: %+v", src)
	}
}

func TestChartMissingSymbol(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{}), http.MethodGet, "/v1/chart?width=60")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestChartBadWidth(t *testing.T) {
	for _, q := range []string{"symbol=X", "symbol=X&width=0", "symbol=X&width=-1", "symbol=X&width=abc"} {
		rec := do(newTestHandler(&fakeSource{}), http.MethodGet, "/v1/chart?"+q)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%q: want 400, got %d", q, rec.Code)
		}
	}
}

func TestChartBadHeight(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{candles: sampleCandles()}), http.MethodGet, "/v1/chart?symbol=X&width=1&height=0")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestChartEmptyCandles(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{candles: nil}), http.MethodGet, "/v1/chart?symbol=X&width=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "(no candles)" {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestChartUpstreamError(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{err: errors.New("boom")}), http.MethodGet, "/v1/chart?symbol=X&width=1")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", rec.Code)
	}
}

func TestChartNonGet(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{}), http.MethodPost, "/v1/chart?symbol=X&width=1")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rec.Code)
	}
}

func TestUnknownPath(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{}), http.MethodGet, "/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestChartUpDownFalse(t *testing.T) {
	rec := do(newTestHandler(&fakeSource{candles: sampleCandles()}), http.MethodGet, "/v1/chart?symbol=X&width=1&updown=false")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if strings.ContainsRune(rec.Body.String(), '▒') {
		t.Fatal("updown=false should not emit down glyph")
	}
}
