package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jayrajadeja/chart/chart"
)

// fakeStream feeds pre-scripted snapshots (and an optional terminal error) into a
// StreamSource. Each snapshot is delivered in order; the channel then stays open
// until ctx is cancelled, mirroring a live upstream.
type fakeStream struct {
	snapshots [][]chart.Candle
	err       error
	notStream bool // when true, Handler is built with a plain Source (no Stream)
}

func (f *fakeStream) Candles(symbol string, width, from, to int64) ([]chart.Candle, error) {
	return nil, nil
}

func (f *fakeStream) Stream(ctx context.Context, symbol string, width int64) (<-chan []chart.Candle, <-chan error) {
	snaps := make(chan []chart.Candle)
	errs := make(chan error, 1)
	go func() {
		defer close(snaps)
		for _, s := range f.snapshots {
			select {
			case snaps <- s:
			case <-ctx.Done():
				return
			}
		}
		if f.err != nil {
			errs <- f.err
			return
		}
		<-ctx.Done()
	}()
	return snaps, errs
}

// plainSource implements only upstream.Source, not StreamSource.
type plainSource struct{}

func (plainSource) Candles(symbol string, width, from, to int64) ([]chart.Candle, error) {
	return nil, nil
}

// waitForFrame scans SSE "event: frame" blocks (joining each block's data lines
// with \n) until one satisfies match, then cancels the request and returns it. A
// safety timer cancels the request if no matching frame arrives, so a coalesced
// or missing frame fails fast instead of hanging. Coalescing (drainLatest) means
// the frame *count* is nondeterministic, so tests assert on frame content, never
// a fixed count.
func waitForFrame(t *testing.T, sc *bufio.Scanner, cancel context.CancelFunc, match func(string) bool) string {
	t.Helper()
	timer := time.AfterFunc(5*time.Second, cancel)
	defer timer.Stop()
	var event string
	var data []string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if event == "frame" && match(strings.Join(data, "\n")) {
				cancel()
				return strings.Join(data, "\n")
			}
			event, data = "", nil
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(line[len("data:"):], " "))
		}
	}
	t.Fatal("stream ended before a matching frame")
	return ""
}

func streamGet(t *testing.T, srv *httptest.Server, target string) (*http.Response, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+target, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("stream GET: %v", err)
	}
	return resp, cancel
}

func TestStreamFrameMatchesRender(t *testing.T) {
	final := sampleCandles()
	src := &fakeStream{snapshots: [][]chart.Candle{
		{final[0]},
		final,
	}}
	srv := httptest.NewServer(Handler(src, Defaults{Height: 20, UpDown: true}))
	defer srv.Close()

	resp, cancel := streamGet(t, srv, "/v1/stream?symbol=SYNTH&width=60&height=8&updown=true")
	defer resp.Body.Close()
	defer cancel()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	want, err := chart.Render(final, chart.Options{Height: 8, UpDown: true})
	if err != nil {
		t.Fatalf("render oracle: %v", err)
	}
	wantFrame := strings.Join(want, "\n")

	// The final frame must equal chart.Render(final); coalescing may merge the
	// intermediate {final[0]} frame away, so wait for the matching frame rather
	// than a fixed count.
	sc := bufio.NewScanner(resp.Body)
	got := waitForFrame(t, sc, cancel, func(f string) bool { return f == wantFrame })
	if got != wantFrame {
		t.Fatalf("frame != chart.Render(final)\n got:\n%s\nwant:\n%s", got, wantFrame)
	}
}

func TestStreamNoCandlesFrame(t *testing.T) {
	src := &fakeStream{snapshots: [][]chart.Candle{{}}}
	srv := httptest.NewServer(Handler(src, Defaults{Height: 20, UpDown: true}))
	defer srv.Close()

	resp, cancel := streamGet(t, srv, "/v1/stream?symbol=SYNTH&width=60")
	defer resp.Body.Close()
	defer cancel()

	sc := bufio.NewScanner(resp.Body)
	waitForFrame(t, sc, cancel, func(f string) bool { return strings.Contains(f, "(no candles)") })
}

func TestStreamValidation(t *testing.T) {
	src := &fakeStream{}
	h := Handler(src, Defaults{Height: 20, UpDown: true})
	cases := []string{
		"/v1/stream",                                    // missing symbol
		"/v1/stream?symbol=SYNTH",                       // missing width
		"/v1/stream?symbol=SYNTH&width=0",               // non-positive width
		"/v1/stream?symbol=SYNTH&width=x",               // bad width
		"/v1/stream?symbol=SYNTH&width=60&height=0",     // bad height
		"/v1/stream?symbol=SYNTH&width=60&updown=maybe", // bad updown
	}
	for _, target := range cases {
		rec := do(h, http.MethodGet, target)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: code=%d, want 400", target, rec.Code)
		}
	}
}

func TestStreamMethodNotAllowed(t *testing.T) {
	rec := do(Handler(&fakeStream{}, Defaults{Height: 20, UpDown: true}), http.MethodPost, "/v1/stream?symbol=SYNTH&width=60")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d, want 405", rec.Code)
	}
}

func TestStreamNotImplemented(t *testing.T) {
	h := Handler(plainSource{}, Defaults{Height: 20, UpDown: true})
	rec := do(h, http.MethodGet, "/v1/stream?symbol=SYNTH&width=60")
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("code=%d, want 501", rec.Code)
	}
}

func TestStreamErrorReachesClient(t *testing.T) {
	// The upstream goroutine sends on errs and closes snaps at the same instant.
	// The server must still emit an event: error, deterministically, not ~half
	// the time (select race between errs and the closed snaps channel).
	for i := 0; i < 25; i++ {
		src := &fakeStream{
			snapshots: [][]chart.Candle{sampleCandles()},
			err:       errors.New("upstream boom"),
		}
		srv := httptest.NewServer(Handler(src, Defaults{Height: 20, UpDown: true}))

		resp, cancel := streamGet(t, srv, "/v1/stream?symbol=SYNTH&width=60")
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		srv.Close()

		if !strings.Contains(string(body), "event: error") ||
			!strings.Contains(string(body), "upstream boom") {
			t.Fatalf("iter %d: error not delivered to client; body=%q", i, body)
		}
	}
}

func TestStreamClientDisconnectReturns(t *testing.T) {
	// Many snapshots pending; cancel after the first frame and ensure the server
	// goroutine unwinds (no hang). The deferred srv.Close would block on a leaked
	// handler, so reaching the end is the assertion.
	snaps := make([][]chart.Candle, 50)
	for i := range snaps {
		snaps[i] = sampleCandles()
	}
	src := &fakeStream{snapshots: snaps}
	srv := httptest.NewServer(Handler(src, Defaults{Height: 20, UpDown: true}))
	defer srv.Close()

	resp, cancel := streamGet(t, srv, "/v1/stream?symbol=SYNTH&width=60")
	sc := bufio.NewScanner(resp.Body)
	waitForFrame(t, sc, cancel, func(string) bool { return true })
	cancel()
	resp.Body.Close()
	time.Sleep(50 * time.Millisecond) // let the handler observe the closed conn
}
