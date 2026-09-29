# Browser live view — implementation plan

Spec: [`../specs/2026-09-29-web-live-view-design.md`](../specs/2026-09-29-web-live-view-design.md).
TDD, minimal surgical diffs, gate green after every task. Stdlib only (`embed`). The page is
a pure additional consumer of `/v1/stream`; no stream logic changes.

## Task 1 — embedded page + root route
- **Test first** `server/live_test.go`:
  - `GET /` → `200`, `Content-Type: text/html`, body contains `EventSource`, `/v1/stream`,
    the `frame` event name, and `<pre`.
  - `GET /` non-GET (POST) → `405`.
  - `GET /nope` (unknown root path) → `404`, not the page.
  - Regression: `/healthz`, `/v1/chart?…`, `/v1/stream?…` still route as before with `/`
    registered.
- **Implement**
  - `server/live.html`: self-contained page (markup + inline CSS + inline JS `EventSource`
    client): symbol/width/height form, a `<pre id="chart">`, a status line; on input change
    reopen the stream. No external assets.
  - `server/live.go`: `//go:embed live.html` → `[]byte`; `serveLive(w, r)` — GET only
    (`405` otherwise), `r.URL.Path == "/"` guard (`404` otherwise), write `text/html`.
  - Register `mux.HandleFunc("/", ...)` in `handler.go`.
- Gate (`go test -race ./server/...`, `go vet`, `gofmt`).

## Task 2 — smoke + docs
- Manual browser smoke: candle serve + chart serve, open `http://127.0.0.1:8139/`, append
  ticks, confirm the `<pre>` redraws and the status line tracks connect/live.
- README: a "Browser view" note under Serve (open `/` for a live page; it's an EventSource
  client over `/v1/stream`, no build step) + a layout row. AGENTS: one line.

## Task 3 — gate + review + PR
- Full gate (`go build/vet ./...`, `go test -race ./...`, gofmt).
- Fresh code-review agent (focus: `/` not shadowing the specific routes / not becoming a
  catch-all `404`, correct content-type, embed wired, method handling, no accidental change
  to stream behavior, no injection risk in the static page).
- Fix Critical/Important. Open PR base main via `as-personal gh pr create`, disclose model +
  plugins. Never self-merge.
