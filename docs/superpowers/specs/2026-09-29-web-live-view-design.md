# Browser live view for the chart stream (design)

Project **CH.4**, a thin UI increment on `chart serve`. It serves one embedded static
HTML page that opens the existing `/v1/stream` SSE feed with the browser's native
`EventSource` and paints each frame into a monospace `<pre>` — so the live chart is
visible in a browser, not only a terminal `curl -N`.

## Problem

`/v1/stream` already pushes rendered ASCII frames, but the only client so far is
`curl -N`. To *see* it live you need a terminal and the exact URL. A browser view removes
that friction and makes the whole `lob → tickstore → candle → chart` pipeline demoable to
anyone with the serve URL.

## Why this is nearly free

A browser `EventSource` is an SSE client that already speaks our wire format: for an
`event: frame` block with several `data:` lines it **joins them with `\n`** into a single
`event.data` string — precisely the multi-line frame the server emits. So the client is:

```js
const es = new EventSource(`/v1/stream?symbol=${s}&width=${w}&height=${h}`);
es.addEventListener("frame", e => { pre.textContent = e.data; });
es.addEventListener("error", e => { status.textContent = "reconnecting…"; });
```

No frame parsing, no dependency, no build step. The ASCII grid in a monospace `<pre>` is
already the same picture the terminal shows.

## Goal

`GET /` returns a small self-contained HTML page (HTML + inline CSS + inline JS, no external
assets) with a symbol/width/height form and a `<pre>` that shows the latest frame. It
connects to `/v1/stream` on the same origin, updates the `<pre>` on every `frame` event, and
shows a small status line (connecting / live / reconnecting) driven by `EventSource`'s
built-in reconnect. The page is embedded in the binary with `go:embed`, so `chart serve`
stays a single self-contained binary.

Non-goals (YAGNI): no graphical candlesticks on a canvas (that needs a separate numeric
stream and a new contract — a different project); no historical scrollback; no multi-symbol
dashboard; no styling framework; no websockets.

## Design

### One embedded asset

`server/live.html` — a static page committed alongside the handler and embedded via
`//go:embed live.html` into a `[]byte`. It contains everything: markup, a little CSS
(dark, monospace, centered `<pre>`), and the `EventSource` script. The symbol/width/height
inputs default to the common demo values and, on change, tear down the old `EventSource`
and open a new one so switching symbols is instant.

### One route

`Handler` gains `GET /` → serve the embedded page with `Content-Type: text/html;
charset=utf-8`. Method other than GET → `405`, consistent with the other routes. Because
`/` is the mux root, it must not shadow the specific routes (`/healthz`, `/v1/chart`,
`/v1/stream`) — Go's `ServeMux` already prefers the longest matching pattern, so the exact
routes win and `/` catches only the bare root. To avoid `/` acting as a catch-all `404`
handler for unknown paths, the handler checks `r.URL.Path == "/"` and returns `404` for
anything else it receives.

### No change to the stream

`/v1/stream` is untouched; the page is purely an additional consumer. The frame invariant
(a frame equals `chart.Render` of the full set) already guarantees the browser shows exactly
what `/v1/chart` would return — the UI adds display only, no logic.

## Testing

- `GET /` returns `200`, `text/html`, and a body containing the key wiring markers
  (`EventSource`, `/v1/stream`, the `frame` event name, and the `<pre>` element) so a
  refactor that breaks the client contract fails a test.
- `GET /` with a non-GET method returns `405`.
- An unknown path (e.g. `/nope`) still returns `404` (the root handler doesn't swallow it).
- Existing `/healthz`, `/v1/chart`, `/v1/stream` behavior is unchanged (regression: they
  still route correctly with `/` registered).
- Manual smoke: run candle serve + chart serve, open `http://127.0.0.1:8139/` in a browser,
  append ticks, watch the `<pre>` redraw live.

## Layout additions

| unit | job |
|------|-----|
| `server/live.html` | embedded static page: form + `<pre>` + `EventSource` client for `/v1/stream` |
| `server` `GET /` | serve the embedded page (text/html); `404` for other root-mux paths |
