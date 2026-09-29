# Live chart streaming over SSE — implementation plan

Spec: [`../specs/2026-09-29-live-chart-stream-design.md`](../specs/2026-09-29-live-chart-stream-design.md).
TDD, minimal surgical diffs, gate green after every task. Standard library only. The frame
invariant (an emitted frame equals `chart.Render` of the current full set, == `/v1/chart`)
is the oracle.

## Task 1 — upstream SSE client
- **Test first** `upstream/stream_test.go`: a fake candle-serve `httptest.Server` emitting a
  scripted SSE sequence (initial snapshot, appends, open-bucket mutation, `reset`);
  `HTTPSource.Stream` must yield a final snapshot equal to the expected set, and a `reset`
  must drop stranded candles. Assert prompt end on ctx cancel.
- **Implement** `upstream/stream.go`: `StreamSource` interface + `HTTPSource.Stream` — a
  timeout-free client, SSE line parser (accumulate `data:` per block, dispatch on blank
  line), `map[int64]chart.Candle` upsert / `reset` clear, sorted-snapshot push, close on
  end, one terminal error.
- Gate (`go test -race ./upstream/...`).

## Task 2 — server frame loop
- **Test first** `server/stream_test.go`: a fake `StreamSource` (channel-driven) fed known
  sets; `httptest.Server`; collect `event: frame` blocks (join `data:` lines) and assert the
  last frame equals `chart.Render(finalSet, opts)`. Plus: validation parity with `/v1/chart`
  (symbol/width/height/updown), `501` when the source isn't a `StreamSource`, `(no candles)`
  on empty, prompt return on client disconnect.
- **Implement** `server/stream.go`: `/v1/stream` route in `Handler`; parse render params;
  `http.Flusher` assertion; `src.(upstream.StreamSource)` or `501`; select loop over
  snapshots/errors/ctx with `drainLatest` coalescing; SSE frame writer (one `data:` per
  line). Register route in `handler.go`.
- Gate (`go test -race ./server/...`).

## Task 3 — live smoke + docs
- Live smoke: build chart + a candle serve (from ~/Personal/candle), pipe lob→tickstore or
  seed a `.log`, `curl -N .../v1/stream`, append ticks, watch frames redraw. (Manual; not a
  committed test.)
- README: a "Live stream" subsection under Serve (SSE `/v1/stream`, frame = full re-render,
  event shapes, upstream `/v1/stream` dependency) + a layout row. AGENTS: one line — chart
  `/v1/stream` renders each upstream candle snapshot to a frame; the frame equals `/v1/chart`.

## Task 4 — gate + review + PR
- Full gate (`go build/vet ./...`, `go test -race ./...`, gofmt).
- Fresh code-review agent (focus: no client timeout on the stream, SSE parse correctness incl.
  multi-line data + reset, snapshot maintenance vs upstream, frame-loop ctx cancel / goroutine
  leak, `501` path, empty-render handling).
- Fix Critical/Important. Open PR base main via `as-personal gh pr create`, disclose model +
  plugins, state the frame invariant. Never self-merge.
