package upstream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/jayrajadeja/chart/chart"
)

// StreamSource pushes the full candle set on every upstream change until ctx is
// cancelled or the stream ends. The snapshot channel closes on a clean end; the
// error channel carries at most one terminal error.
type StreamSource interface {
	Stream(ctx context.Context, symbol string, width int64) (<-chan []chart.Candle, <-chan error)
}

// streamClient is a dedicated http.Client with no total timeout; a long-lived SSE
// stream must be bounded by context cancellation, not by Client.Timeout (which
// would sever the connection mid-stream). It is a package var so the stream path
// never accidentally borrows HTTPSource.client's timeout.
var streamClient = &http.Client{}

// streamCandle mirrors the fields chart needs from candle serve's stream DTO.
type streamCandle struct {
	Start int64 `json:"start"`
	Open  int64 `json:"open"`
	High  int64 `json:"high"`
	Low   int64 `json:"low"`
	Close int64 `json:"close"`
}

// Stream connects to <base>/v1/stream, parses the candle SSE feed, maintains the
// candle set (upsert by start, clear on reset), and emits a fresh sorted snapshot
// on every applied change.
func (s *HTTPSource) Stream(ctx context.Context, symbol string, width int64) (<-chan []chart.Candle, <-chan error) {
	snaps := make(chan []chart.Candle, 8)
	errs := make(chan error, 1)

	go func() {
		defer close(snaps)

		q := url.Values{}
		q.Set("symbol", symbol)
		q.Set("width", strconv.FormatInt(width, 10))
		reqURL := s.base + "/v1/stream?" + q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			errs <- fmt.Errorf("stream request: %w", err)
			return
		}
		req.Header.Set("Accept", "text/event-stream")

		resp, err := streamClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return // cancellation is a clean end, not an error
			}
			errs <- fmt.Errorf("stream connect: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			errs <- fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
			return
		}

		set := map[int64]chart.Candle{}
		var event string
		var data []string

		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "": // end of an event block: dispatch
				if s.applyEvent(event, data, set, errs) {
					return // terminal error event
				}
				if event == "candle" || event == "reset" {
					if !push(ctx, snaps, snapshot(set)) {
						return
					}
				}
				event, data = "", data[:0]
			case strings.HasPrefix(line, ":"): // heartbeat comment
				continue
			case strings.HasPrefix(line, "event:"):
				event = strings.TrimSpace(line[len("event:"):])
			case strings.HasPrefix(line, "data:"):
				data = append(data, strings.TrimPrefix(line[len("data:"):], " "))
			}
		}
		if err := sc.Err(); err != nil && ctx.Err() == nil {
			errs <- fmt.Errorf("stream read: %w", err)
		}
	}()

	return snaps, errs
}

// applyEvent mutates set per one SSE event block. It returns true when the stream
// should terminate (an error event was delivered).
func (s *HTTPSource) applyEvent(event string, data []string, set map[int64]chart.Candle, errs chan<- error) bool {
	switch event {
	case "candle":
		var c streamCandle
		if err := json.Unmarshal([]byte(strings.Join(data, "\n")), &c); err != nil {
			return false // skip a malformed candle rather than kill the stream
		}
		set[c.Start] = chart.Candle{Start: c.Start, Open: c.Open, High: c.High, Low: c.Low, Close: c.Close}
	case "reset":
		for k := range set {
			delete(set, k)
		}
	case "error":
		errs <- fmt.Errorf("upstream stream: %s", strings.TrimSpace(strings.Join(data, "\n")))
		return true
	}
	return false
}

// snapshot returns the candle set sorted by start.
func snapshot(set map[int64]chart.Candle) []chart.Candle {
	out := make([]chart.Candle, 0, len(set))
	for _, c := range set {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// push sends v unless ctx is cancelled, reporting whether the send happened.
func push(ctx context.Context, ch chan<- []chart.Candle, v []chart.Candle) bool {
	select {
	case ch <- v:
		return true
	case <-ctx.Done():
		return false
	}
}
