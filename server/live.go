package server

import (
	_ "embed"
	"net/http"
)

//go:embed live.html
var livePage []byte

// serveLive serves the embedded browser live-view page at the mux root. It is a
// pure additional consumer of /v1/stream (a browser EventSource client), adding
// display only — no stream logic. Only the exact root path is served; any other
// path reaching the root pattern is a 404, so "/" never acts as a catch-all.
func serveLive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if r.URL.Path != "/" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(livePage)
}
