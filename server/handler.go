// Package server exposes chart rendering over HTTP. It is transport-only: it
// parses requests, calls an upstream.Source for candles, renders them with the
// pure chart.Render, and writes text/plain. It owns no data and no aggregation.
package server

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/jayrajadeja/chart/chart"
	"github.com/jayrajadeja/chart/upstream"
)

const (
	minInt64 = math.MinInt64
	maxInt64 = math.MaxInt64
)

// Defaults seed optional render params when a request omits them.
type Defaults struct {
	Height int
	UpDown bool
}

// Handler returns an http.Handler rendering charts from src.
func Handler(src upstream.Source, def Defaults) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeText(w, "ok\n")
	})
	mux.HandleFunc("/v1/chart", func(w http.ResponseWriter, r *http.Request) {
		serveChart(w, r, src, def)
	})
	return mux
}

func serveChart(w http.ResponseWriter, r *http.Request, src upstream.Source, def Defaults) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()

	symbol := q.Get("symbol")
	if symbol == "" {
		writeErr(w, http.StatusBadRequest, "missing symbol")
		return
	}
	width, err := parseInt64Required(q, "width")
	if err != nil || width <= 0 {
		writeErr(w, http.StatusBadRequest, "width must be a positive integer")
		return
	}
	from, err := parseInt64Default(q, "from", minInt64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid from")
		return
	}
	to, err := parseInt64Default(q, "to", maxInt64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid to")
		return
	}
	height := def.Height
	if raw := q.Get("height"); raw != "" {
		h, err := strconv.Atoi(raw)
		if err != nil || h <= 0 {
			writeErr(w, http.StatusBadRequest, "height must be a positive integer")
			return
		}
		height = h
	}
	updown := def.UpDown
	if raw := q.Get("updown"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "updown must be a boolean")
			return
		}
		updown = b
	}

	candles, err := src.Candles(symbol, width, from, to)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "upstream error")
		return
	}

	lines, err := chart.Render(candles, chart.Options{Height: height, UpDown: updown})
	if err != nil {
		if errors.Is(err, chart.ErrNoCandles) {
			writeText(w, "(no candles)\n")
			return
		}
		writeErr(w, http.StatusInternalServerError, "render error")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, ln := range lines {
		w.Write([]byte(ln))
		w.Write([]byte{'\n'})
	}
}

func parseInt64Required(q map[string][]string, name string) (int64, error) {
	raw := first(q, name)
	if raw == "" {
		return 0, errors.New("missing " + name)
	}
	return strconv.ParseInt(raw, 10, 64)
}

func parseInt64Default(q map[string][]string, name string, def int64) (int64, error) {
	raw := first(q, name)
	if raw == "" {
		return def, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

func first(q map[string][]string, name string) string {
	if v := q[name]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func writeText(w http.ResponseWriter, s string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(s))
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	w.Write([]byte(msg + "\n"))
}
