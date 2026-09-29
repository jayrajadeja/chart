# Live chart streaming over SSE (design)

Project **CH.3**, the streaming increment on `chart serve`. It consumes candle's new
`/v1/stream` and pushes **re-rendered ASCII frames** as Server-Sent Events, so a terminal
or browser sees the candlestick chart redraw as ticks flow — the visible capstone of the
`lob → tickstore → candle → chart` pipeline.

## Problem

`chart serve` is request/response: `GET /v1/chart` fetches a candle snapshot from
`/v1/candles`, renders it once, and returns text. A live chart must re-request on a timer
and redraw client-side. candle now *pushes* candle changes over `/v1/stream`; chart should
consume that and push finished frames, so the client just prints what it receives.

## Goal

`GET /v1/stream?symbol=X&width=W[&height=&updown=]` on chart serve: connect to the upstream
candle `/v1/stream`, maintain the candle set, and emit an `event: frame` (the rendered
chart) whenever the set changes. A frame equals `chart.Render` of the current full candle
set — i.e. what `GET /v1/chart` would return at that instant. That equality is the
invariant and the test oracle; because both paths call the same `chart.Render`, it reduces
to *maintaining the upstream candle set correctly* (upsert by `start`, honor `reset`).

Non-goals (YAGNI): no windowed streaming (full-range tail, matching candle); no diffing of
frame *contents* (re-render whole, it's ASCII and cheap); no reconnect/backoff to a dropped
upstream (surface the error and end — the client reconnects); no auth.

## Design

### Upstream: an SSE client (`upstream` package)

A new `StreamSource` alongside the existing `Source`:

```go
type StreamSource interface {
    // Stream connects to <base>/v1/stream and emits the full candle set on every
    // change until ctx is cancelled or the stream ends. The snapshot channel closes
    // on a clean end; the error channel carries at most one terminal error.
    Stream(ctx context.Context, symbol string, width int64) (<-chan []chart.Candle, <-chan error)
}
```

`HTTPSource` implements it. **Critical:** streaming must not use the 10s-timeout client the
snapshot path uses — a total client timeout would sever the connection mid-stream. `Stream`
uses a dedicated timeout-free `http.Client` and relies on `ctx` (the request context) for
lifetime.

It reads the SSE body line-by-line, accumulating `data:` lines per event block (SSE joins
multi-line data with `\n`) and dispatching on the blank-line terminator:

- `event: candle` → parse the JSON DTO (`start,open,high,low,close`), upsert into
  `map[int64]chart.Candle` by `start`.
- `event: reset` → clear the map (candle sends this when its series was rebuilt).
- `event: error` → deliver the error and stop.

After each applied event it pushes a fresh sorted snapshot on the channel (latest-wins is
fine; the consumer coalesces bursts). The channel is closed when the body ends.

### Server: the frame loop (`server` package)

`Handler` gains a `/v1/stream` route. Because `chart.Render` and the query parsing already
exist, the handler is thin: validate `symbol`/`width`/`height`/`updown` exactly like
`/v1/chart`; assert the writer is an `http.Flusher`; type-assert the source to
`upstream.StreamSource` (else `501`); then:

```
snaps, errs := ss.Stream(ctx, symbol, width)
for {
    select {
    case <-ctx.Done(): return
    case err := <-errs: emit `event: error`; return
    case cs, ok := <-snaps:
        if !ok { return }             // upstream ended
        cs = drainLatest(snaps, cs)   // coalesce a burst into one render
        lines := render(cs, opts)     // chart.Render, or "(no candles)"
        emit `event: frame` with one `data:` line per rendered line
        flush
    }
}
```

A frame is emitted as multiple `data:` lines (one per chart row) so a standard SSE client
reconstructs the multi-line frame by joining on `\n`. `ErrNoCandles` renders a single
`(no candles)` line rather than erroring, matching `/v1/chart`.

### Wiring

`cmd/chart serve` is unchanged: `HTTPSource` already becomes the `/v1/stream` backend via
the type assertion, and its stream path builds its own timeout-free client. The default
`--candles-url`, `--height`, `--updown` apply.

## Testing

- `upstream/stream_test.go`: point `HTTPSource.Stream` at a fake candle-serve
  `httptest.Server` that emits a scripted SSE sequence (snapshot, appends, an open-bucket
  mutation, a `reset`); assert the final snapshot equals the expected candle set and that a
  `reset` drops stranded candles. Covers the real SSE-parsing client.
- `server/stream_test.go`: a fake `StreamSource` feeding candle sets; drive an
  `httptest.Server`, collect `event: frame` blocks, and assert the last frame equals
  `chart.Render(finalSet, opts)` (the oracle). Plus validation parity, `501` when the
  source can't stream, `(no candles)` handling, and prompt return on client disconnect.
- All under `-race`.

## Layout additions

| unit | job |
|------|-----|
| `upstream.StreamSource` / `HTTPSource.Stream` | SSE client: parse candle `/v1/stream`, maintain the candle set, emit snapshots |
| `server` `/v1/stream` | render each upstream snapshot to an ASCII frame, push as SSE |
