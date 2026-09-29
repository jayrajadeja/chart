package upstream

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jayrajadeja/chart/chart"
)

// fakeCandleStream is an httptest handler that emits a scripted candle-serve
// /v1/stream SSE sequence, then blocks until the client disconnects.
func fakeCandleStream(events []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flush", 500)
			return
		}
		w.WriteHeader(200)
		for _, ev := range events {
			fmt.Fprint(w, ev)
			fl.Flush()
			time.Sleep(3 * time.Millisecond)
		}
		<-r.Context().Done() // hold the stream open like real candle serve
	}
}

func candleEvent(start, open, high, low, cls int64) string {
	return fmt.Sprintf("event: candle\ndata: {\"start\":%d,\"open\":%d,\"high\":%d,\"low\":%d,\"close\":%d,\"volume\":1,\"vwap\":%d}\n\n",
		start, open, high, low, cls, open)
}

// collectFinal drains the snapshot channel until it settles (no new snapshot for a
// short quiet period) or ctx/errs fire, returning the last snapshot seen.
func collectFinal(t *testing.T, snaps <-chan []chart.Candle, errs <-chan error) []chart.Candle {
	t.Helper()
	var last []chart.Candle
	quiet := time.NewTimer(80 * time.Millisecond)
	defer quiet.Stop()
	for {
		select {
		case s, ok := <-snaps:
			if !ok {
				return last
			}
			last = s
			if !quiet.Stop() {
				select {
				case <-quiet.C:
				default:
				}
			}
			quiet.Reset(80 * time.Millisecond)
		case err := <-errs:
			if err != nil {
				t.Fatalf("stream error: %v", err)
			}
		case <-quiet.C:
			return last
		}
	}
}

func TestStreamUpsertAndAppend(t *testing.T) {
	events := []string{
		candleEvent(0, 100, 105, 99, 104), // initial open bucket
		candleEvent(0, 100, 108, 99, 107), // same bucket mutates (replace, not add)
		candleEvent(100, 107, 110, 106, 109),
		candleEvent(200, 109, 115, 108, 114),
	}
	srv := httptest.NewServer(fakeCandleStream(events))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	snaps, errs := NewHTTP(srv.URL, nil).Stream(ctx, "SYNTH", 100)
	got := collectFinal(t, snaps, errs)

	want := []chart.Candle{
		{Start: 0, Open: 100, High: 108, Low: 99, Close: 107},
		{Start: 100, Open: 107, High: 110, Low: 106, Close: 109},
		{Start: 200, Open: 109, High: 115, Low: 108, Close: 114},
	}
	assertCandles(t, got, want)
}

func TestStreamResetDropsStranded(t *testing.T) {
	events := []string{
		candleEvent(0, 100, 105, 99, 104),
		candleEvent(100, 104, 109, 103, 108),
		candleEvent(200, 108, 112, 107, 111),
		"event: reset\ndata: {\"reset\":true}\n\n", // series rebuilt
		candleEvent(0, 200, 205, 199, 204),
		candleEvent(100, 204, 209, 203, 208),
	}
	srv := httptest.NewServer(fakeCandleStream(events))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	snaps, errs := NewHTTP(srv.URL, nil).Stream(ctx, "SYNTH", 100)
	got := collectFinal(t, snaps, errs)

	want := []chart.Candle{
		{Start: 0, Open: 200, High: 205, Low: 199, Close: 204},
		{Start: 100, Open: 204, High: 209, Low: 203, Close: 208},
	}
	assertCandles(t, got, want)
}

func TestStreamHeartbeatsIgnored(t *testing.T) {
	events := []string{
		": ping\n\n",
		candleEvent(0, 100, 105, 99, 104),
		": ping\n\n",
		candleEvent(100, 104, 109, 103, 108),
	}
	srv := httptest.NewServer(fakeCandleStream(events))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	snaps, errs := NewHTTP(srv.URL, nil).Stream(ctx, "SYNTH", 100)
	got := collectFinal(t, snaps, errs)
	assertCandles(t, got, []chart.Candle{
		{Start: 0, Open: 100, High: 105, Low: 99, Close: 104},
		{Start: 100, Open: 104, High: 109, Low: 103, Close: 108},
	})
}

func TestStreamContextCancelEnds(t *testing.T) {
	srv := httptest.NewServer(fakeCandleStream([]string{candleEvent(0, 100, 105, 99, 104)}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	snaps, errs := NewHTTP(srv.URL, nil).Stream(ctx, "SYNTH", 100)
	// consume the first snapshot, then cancel
	select {
	case <-snaps:
	case <-time.After(time.Second):
		t.Fatal("no first snapshot")
	}
	cancel()
	// snaps must close promptly
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-snaps:
			if !ok {
				return
			}
		case <-errs:
		case <-deadline:
			t.Fatal("stream did not end after cancel")
		}
	}
}

func TestStreamBadStatusErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	snaps, errs := NewHTTP(srv.URL, nil).Stream(ctx, "SYNTH", 100)
	select {
	case err := <-errs:
		if err == nil || !strings.Contains(err.Error(), "502") {
			t.Fatalf("err = %v, want 502", err)
		}
	case <-snaps:
		t.Fatal("expected an error, got a snapshot")
	case <-time.After(time.Second):
		t.Fatal("no error delivered")
	}
}

func assertCandles(t *testing.T, got, want []chart.Candle) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d candles, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candle %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
