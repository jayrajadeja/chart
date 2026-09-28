// Package upstream fetches candles from a candle-serve JSON API so chart serve
// can render them. chart owns no data of its own; it renders whatever an upstream
// that speaks /v1/candles returns.
package upstream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jayrajadeja/chart/chart"
)

// Source supplies candles for a symbol and window.
type Source interface {
	Candles(symbol string, width, from, to int64) ([]chart.Candle, error)
}

// HTTPSource fetches candles from a candle-serve /v1/candles endpoint.
type HTTPSource struct {
	base   string
	client *http.Client
}

// NewHTTP returns an HTTPSource over baseURL using client (defaulting to
// http.DefaultClient when nil).
func NewHTTP(baseURL string, client *http.Client) *HTTPSource {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPSource{base: strings.TrimRight(baseURL, "/"), client: client}
}

// candlesResponse mirrors candle serve's wire shape; only OHLC+start are read.
type candlesResponse struct {
	Candles []struct {
		Start int64 `json:"start"`
		Open  int64 `json:"open"`
		High  int64 `json:"high"`
		Low   int64 `json:"low"`
		Close int64 `json:"close"`
	} `json:"candles"`
}

// Candles GETs <base>/v1/candles?symbol=&width=&from=&to= and decodes the result.
func (s *HTTPSource) Candles(symbol string, width, from, to int64) ([]chart.Candle, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("width", strconv.FormatInt(width, 10))
	q.Set("from", strconv.FormatInt(from, 10))
	q.Set("to", strconv.FormatInt(to, 10))
	reqURL := s.base + "/v1/candles?" + q.Encode()

	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded candlesResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode upstream: %w", err)
	}

	out := make([]chart.Candle, len(decoded.Candles))
	for i, c := range decoded.Candles {
		out[i] = chart.Candle{Start: c.Start, Open: c.Open, High: c.High, Low: c.Low, Close: c.Close}
	}
	return out, nil
}
