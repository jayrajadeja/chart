# chart — ASCII candlestick charts from a candle stream (Go)

Turns **OHLCV candles** into a candlestick chart you can read in a terminal. It
consumes the CSV that `candle` prints, maps prices to rows with pure integer
scaling, and draws one column per candle: a low-high wick with an open-close body.
Standard library only, no floats, deterministic (same input, byte-identical chart).

This is **project D** in the pipeline, the part that finally *shows* the data:

```
A  lob        generate   — match orders, emit a trade stream
B  tickstore  store      — append-only, indexed tick store
C  candle     aggregate  — OHLCV candles over the tick stream
D  chart      visualise  — ASCII candlesticks from candle CSV        (this repo)
```

It cooperates with `candle` through **one contract: the CSV**. There is no Go
dependency on the other tools; chart reads text, exactly as a human would pipe it.

## Build

```bash
go build ./...
go build -o chart ./cmd/chart
```

Requires Go 1.26+ (see `go.mod`). No third-party dependencies.

## Usage

```bash
# live: generate -> aggregate -> chart
lob-replay --emit | candle --width 120 --csv | chart --height 16

# from a saved candle CSV file
chart --height 12 candles.csv

# all bodies the same glyph (no up/down distinction)
candle --width 60 --csv < ticks | chart --updown=false
```

Flags:

| flag | default | meaning |
|------|---------|---------|
| `--height` | `20` | number of price rows (**> 0**) |
| `--updown` | `true` | distinct body glyph for up (`█`) vs down (`▒`) candles |

Input is read from the file named as the first positional argument, or from stdin
when none is given. The input must be `candle --csv` output (a header row, then one
row per candle); `start`, `open`, `high`, `low`, and `close` columns are required
and bound by name, so extra or reordered columns are fine.

## Serve

`chart serve` renders charts over HTTP as a **rendering proxy** over a running
`candle serve`. chart owns no data: it fetches candles from candle serve's JSON API,
renders them with the same pure renderer, and returns `text/plain`.

```bash
# terminal 1: aggregate over an ingested tickstore log
candle serve --dir data            # :8138, JSON /v1/candles

# terminal 2: render those candles over HTTP
chart serve --candles-url http://127.0.0.1:8138   # :8139, text /v1/chart

curl 'http://127.0.0.1:8139/v1/chart?symbol=SYNTH&width=120&height=16'
```

| method + path | returns |
|---------------|---------|
| `GET /` | an embedded browser live-view page (`EventSource` over `/v1/stream`) |
| `GET /healthz` | `ok` |
| `GET /v1/chart?symbol=&width=&from=&to=&height=&updown=` | the rendered chart, `text/plain` |
| `GET /v1/stream?symbol=&width=&height=&updown=` | a live SSE feed of re-rendered chart frames |

`symbol` and `width` (> 0) are required; `from`/`to` default to the full range;
`height`/`updown` default to the serve flags. Serve flags:

| flag | default | meaning |
|------|---------|---------|
| `--addr` | `127.0.0.1:8139` | listen address |
| `--candles-url` | `http://127.0.0.1:8138` | base URL of an upstream `candle serve` |
| `--height` | `20` | default price rows when a request omits `height` |
| `--updown` | `true` | default up/down glyph distinction |

### Live stream

`GET /v1/stream` is the streaming twin of `/v1/chart`: chart consumes candle
serve's own `/v1/stream` SSE feed, maintains the candle set (upsert by `start`,
clear on `reset`), and pushes a freshly **re-rendered** ASCII frame whenever the
set changes — so a terminal redraws the chart live as ticks flow.

```bash
curl -N 'http://127.0.0.1:8139/v1/stream?symbol=SYNTH&width=120&height=16'
```

Each frame is one SSE `event: frame` with one `data:` line per chart row (a client
rejoins them with `\n`); an empty set yields a `(no candles)` frame and an upstream
failure an `event: error`. The **invariant**: a frame equals `chart.Render` of the
current full candle set — exactly what `/v1/chart` would return at that instant.
Requires the upstream to speak `/v1/stream`; otherwise the endpoint returns `501`.

### Browser view

`GET /` serves a small embedded page (`go:embed`, no build step, no external assets)
that opens `/v1/stream` with the browser's native `EventSource` and paints each frame
into a monospace `<pre>` — the same live chart as `curl -N`, in a browser, with a
symbol/width/height form:

```bash
chart serve   # then open http://127.0.0.1:8139/ in a browser
```

It's a pure additional consumer of `/v1/stream` (`EventSource` joins the multi-line
frame's `data:` lines with `\n` for free), so the page adds display only — the frame
invariant still holds.


Status codes: bad params (missing `symbol`, `width<=0`, bad `height`) → `400`;
unknown path → `404`; non-GET → `405`; upstream unreachable / non-200 / undecodable
→ `502`; an empty candle set is `200` with body `(no candles)`. This makes the
series' "cooperate through a contract" theme explicit at the network boundary: chart
consumes candle's **JSON** and adds only rendering.

## What it draws

```
10005 │  ▒             │   █│
10004 │  ▒││ ││  │█││││││ ██▒
 ...  │  ...
 9996 │  ││││  │
      │  candles=31  start=[0..3600]  price=[9996..10005]  height=16
```

- **One column per candle.** `│` is the high-low wick; the body spans open-close.
- **Bodies show direction.** `█` when the candle closed up (`close >= open`), `▒`
  when it closed down (with `--updown`; otherwise every body is `█`).
- **Left axis is price.** The top row is the highest high, the bottom row the lowest
  low, labelled by integer price. A flat series collapses to a single body row.
- **Footer summarises** the candle count, the logical-time span, the price range, and
  the height.

## How it works

- **Integer row scaling.** A price maps to a row by
  `row = (priceMax - price) * (height - 1) / (priceMax - priceMin)` — no floats, so
  the chart is exactly reproducible. Higher prices sit in upper rows.
- **Body then wick.** For each candle and row, the renderer fills the open-close body
  first, then the high-low wick, then blank — so bodies always sit on top of wicks.
- **CSV is the contract.** `parse` binds columns by header name (needs
  `start/open/high/low/close`), so chart tracks candle's format without importing it.
- **I/O at the edge.** The `chart` and `parse` cores are pure; only `cmd/chart`
  touches files/stdin/stdout/argv, through a testable `run(args, stdin, stdout,
  stderr) int` seam.

## Layout

| package | job |
|---------|-----|
| `chart/` | `Render` — candles + height → ASCII grid lines (pure) |
| `parse/` | `ReadCSV` — candle CSV → `[]chart.Candle`, columns bound by name (pure) |
| `upstream/` | `Source` + `HTTPSource` — fetch candles from a `candle serve` JSON API; `StreamSource` — consume candle's `/v1/stream` SSE and maintain the candle set |
| `server/` | transport-only `/v1/chart` handler + `/v1/stream` live frame loop rendering over a `Source`; `GET /` serves an embedded browser live-view page (`server/live.html`) |
| `cmd/chart/` | flags + file/stdin plumbing and the `serve` subcommand — the only I/O layer |

## Development

```bash
go build ./... && go vet ./... && go test ./...
```

Design specs and implementation plans live under `docs/superpowers/`.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md). Agent contributors: read
[`AGENTS.md`](AGENTS.md).

## License

MIT — see [`LICENSE`](LICENSE).
