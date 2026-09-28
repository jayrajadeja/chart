package upstream

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jayrajadeja/chart/chart"
)

func TestHTTPSourceDecodesCandles(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/candles" {
			http.NotFound(w, r)
			return
		}
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"symbol":"SYNTH","width":60,"from":0,"to":100,"count":2,"candles":[
			{"start":0,"open":10001,"high":10005,"low":9996,"close":9998,"volume":134,"vwap":10000,"trades":41,"buyVol":76,"sellVol":58},
			{"start":60,"open":9997,"high":10004,"low":9995,"close":10000,"volume":161,"vwap":9999,"trades":47,"buyVol":58,"sellVol":103}
		]}`))
	}))
	defer srv.Close()

	src := NewHTTP(srv.URL, srv.Client())
	got, err := src.Candles("SYNTH", 60, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []chart.Candle{
		{Start: 0, Open: 10001, High: 10005, Low: 9996, Close: 9998},
		{Start: 60, Open: 9997, High: 10004, Low: 9995, Close: 10000},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d candles, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candle %d: got %+v want %+v", i, got[i], want[i])
		}
	}
	for k, v := range map[string]string{"symbol": "SYNTH", "width": "60", "from": "0", "to": "100"} {
		if gotQuery.Get(k) != v {
			t.Fatalf("query %s: got %q want %q", k, gotQuery.Get(k), v)
		}
	}
}

func TestHTTPSourceEmptyCandles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"symbol":"X","width":1,"from":0,"to":0,"count":0,"candles":[]}`))
	}))
	defer srv.Close()
	got, err := NewHTTP(srv.URL, srv.Client()).Candles("X", 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %d", len(got))
	}
}

func TestHTTPSourceUpstreamNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusBadRequest)
	}))
	defer srv.Close()
	_, err := NewHTTP(srv.URL, srv.Client()).Candles("X", 1, 0, 0)
	if err == nil {
		t.Fatal("want error on upstream 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("error should mention status: %v", err)
	}
}

func TestHTTPSourceMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	if _, err := NewHTTP(srv.URL, srv.Client()).Candles("X", 1, 0, 0); err == nil {
		t.Fatal("want error on malformed JSON")
	}
}

func TestHTTPSourceUnreachable(t *testing.T) {
	src := NewHTTP("http://127.0.0.1:0", http.DefaultClient)
	if _, err := src.Candles("X", 1, 0, 0); err == nil {
		t.Fatal("want error on unreachable upstream")
	}
}
