package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/jayrajadeja/chart/chart"
	"github.com/jayrajadeja/chart/upstream"
)

// serveStream renders each upstream candle snapshot to an ASCII frame and pushes
// it as an SSE "event: frame" (one data: line per chart row). A frame equals
// chart.Render of the current full candle set, i.e. what /v1/chart would return
// at that instant. It requires src to also implement upstream.StreamSource.
func serveStream(w http.ResponseWriter, r *http.Request, src upstream.Source, def Defaults) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := parseRender(w, r.URL.Query(), def)
	if !ok {
		return
	}
	ss, ok := src.(upstream.StreamSource)
	if !ok {
		writeErr(w, http.StatusNotImplemented, "streaming not supported by upstream")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ctx := r.Context()
	snaps, errs := ss.Stream(ctx, p.symbol, p.width)
	opts := chart.Options{Height: p.height, UpDown: p.updown}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errs:
			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
				flusher.Flush()
			}
			return
		case cs, open := <-snaps:
			if !open {
				return // upstream ended
			}
			cs = drainLatest(snaps, cs)
			writeFrame(w, renderFrame(cs, opts))
			flusher.Flush()
		}
	}
}

// drainLatest coalesces a burst of pending snapshots into the newest one so a slow
// render never falls behind a fast upstream.
func drainLatest(snaps <-chan []chart.Candle, cur []chart.Candle) []chart.Candle {
	for {
		select {
		case next, open := <-snaps:
			if !open {
				return cur
			}
			cur = next
		default:
			return cur
		}
	}
}

// renderFrame renders candles to chart lines, degrading an empty set to a single
// "(no candles)" line (matching /v1/chart) and a render error to an error marker.
func renderFrame(candles []chart.Candle, opts chart.Options) []string {
	lines, err := chart.Render(candles, opts)
	if err != nil {
		if errors.Is(err, chart.ErrNoCandles) {
			return []string{"(no candles)"}
		}
		return []string{"(render error)"}
	}
	return lines
}

// writeFrame emits one SSE frame event: one data: line per rendered chart row, so
// a standard SSE client rejoins the multi-line frame with \n.
func writeFrame(w http.ResponseWriter, lines []string) {
	fmt.Fprint(w, "event: frame\n")
	for _, ln := range lines {
		fmt.Fprintf(w, "data: %s\n", ln)
	}
	fmt.Fprint(w, "\n")
}
