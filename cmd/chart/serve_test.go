package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeUpstream serves a canned candle-serve JSON response.
func fakeUpstream() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/candles" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"symbol":"SYNTH","width":60,"from":0,"to":100,"count":2,"candles":[
			{"start":0,"open":10001,"high":10005,"low":9996,"close":9998},
			{"start":60,"open":9997,"high":10004,"low":9995,"close":10000}
		]}`)
	}))
}

func TestServeSmoke(t *testing.T) {
	up := fakeUpstream()
	defer up.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan string, 1)
	errc := make(chan error, 1)
	go func() { errc <- runServe("127.0.0.1:0", up.URL, 20, true, ctx, ready) }()

	var addr string
	select {
	case addr = <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not become ready")
	}
	base := "http://" + addr

	if body := get(t, base+"/healthz"); strings.TrimSpace(body) != "ok" {
		t.Fatalf("healthz body=%q", body)
	}
	body := get(t, base+"/v1/chart?symbol=SYNTH&width=60")
	if body == "" {
		t.Fatal("empty chart body")
	}

	cancel()
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("runServe returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func get(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", url, resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
