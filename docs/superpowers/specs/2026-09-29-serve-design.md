# chart serve — HTTP chart rendering over candle serve (design)

Project **D.1**, an increment on chart
([`2026-09-29-chart-design.md`](2026-09-29-chart-design.md)). Adds a `chart serve`
subcommand that renders charts over HTTP, mirroring `candle serve` and `tickstore
serve` — but as a **rendering proxy**, not a data owner.

## Problem

`chart` renders candles it is handed on a pipe. To answer "show me SYNTH at width W
over [F,T]" over HTTP, it needs candles for a symbol/window — which is exactly what
`candle serve` already answers as JSON. chart has, and should have, no aggregation or
storage of its own.

## Goal

`chart serve` exposes `GET /v1/chart?symbol=&width=&from=&to=&height=&updown=` and
returns the rendered ASCII chart as `text/plain`. It obtains the candles by calling an
upstream `candle serve` (`GET /v1/candles`), then renders them with the existing pure
`chart.Render`. It is a thin transport + HTTP-client adapter around the renderer.

Non-goals (YAGNI): no caching, no its own aggregation/storage, no auth/TLS, no HTML,
no websocket/streaming.

## Why a proxy (the architecture point)

Every seam in this series is a **contract, not a codebase**: lob↔tickstore↔candle
share the 25-byte record; chart↔candle share candle's CSV. `chart serve` extends that
theme to the network boundary: it consumes candle serve's **JSON** contract and adds
only rendering. The pure renderer stays pure; the only new I/O is one HTTP client,
isolated behind an interface. chart never learns how candles are made — it renders
whatever an upstream that speaks `/v1/candles` returns.

## Design

### Source interface (the seam)

```go
// package upstream
type Source interface {
    Candles(symbol string, width, from, to int64) ([]chart.Candle, error)
}
```

- **`HTTPSource`** (production): `GET <baseURL>/v1/candles?symbol=&width=&from=&to=`,
  decode the JSON `{..., candles:[{start,open,high,low,close,...}]}` into
  `[]chart.Candle` (only the five fields chart needs). Constructed with a base URL and
  a `*http.Client` (timeout set by cmd).
- The handler and tests depend on the interface, so a fake Source drives handler tests
  with no network.

### Handler (transport-only)

`package server`: `Handler(src upstream.Source, def Defaults) http.Handler` where
`Defaults{Height int, UpDown bool}` seeds the optional params. Routes:

| method + path | behaviour |
|---------------|-----------|
| `GET /healthz` | `ok` |
| `GET /v1/chart?symbol=&width=&from=&to=&height=&updown=` | rendered chart, `text/plain` |
| any other path | `404` |

Params: `symbol` and `width` (> 0) required; `from`/`to` inclusive, default full
range; `height` defaults to the serve flag (else 20), `updown` defaults to true.
Validation happens before the upstream call.

### Serve command

`cmd/chart/serve.go`: `chart serve --addr --candles-url [--height --updown]`. Builds
an `HTTPSource` over `--candles-url` (default `http://127.0.0.1:8138`, candle's
default), listens on `--addr` (default `127.0.0.1:8139`; tickstore 8137, candle 8138),
graceful shutdown, same `runServe(...)` + ready-channel seam as candle for ephemeral
-port tests. `main` dispatches `serve` when `args[0] == "serve"`, else the existing
flag-based render path runs.

## Status codes

- missing `symbol`, non-integer/`<= 0` `width`, bad `height`/`from`/`to` -> `400`.
- non-GET -> `405`; unknown path -> `404`.
- upstream unreachable, upstream non-200, or undecodable body -> `502` (the failure is
  the gateway's, not the client's).
- empty candle set (valid, just no data) -> `200` with body `(no candles)` — rendering
  zero candles is not an error condition to surface as 5xx.

## Testing

- `upstream/http_test.go` — against an `httptest.Server` returning candle JSON: decodes
  to the right `[]chart.Candle`; upstream non-200 -> error; malformed JSON -> error;
  unreachable/base-URL error -> error; query params are forwarded correctly.
- `server/handler_test.go` — `httptest` table over a fake Source: healthz; happy path
  (body has the axis, glyphs, footer); missing symbol / width<=0 / height<=0 -> 400;
  empty candles -> 200 `(no candles)`; source error -> 502; non-GET -> 405; unknown
  path -> 404; `updown=false` honoured.
- `cmd/chart/serve_test.go` — ephemeral-port smoke: a fake upstream `httptest.Server`,
  boot `chart serve`, hit `/healthz` and `/v1/chart`, assert a rendered body and clean
  shutdown.
- Manual e2e: run `candle serve` over a real ingested log, run `chart serve
  --candles-url` at it, and `curl /v1/chart?...` — confirm the chart matches
  `... | candle --csv | chart`.

## Layout additions

| package | job |
|---------|-----|
| `upstream/` | `Source` + `HTTPSource` — fetch candles from a candle-serve JSON API |
| `server/` | transport-only `/v1/chart` handler rendering over a `Source` |
| `cmd/chart/` | gains the `serve` subcommand (client wiring, graceful shutdown) |
