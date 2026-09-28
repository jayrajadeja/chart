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
| `cmd/chart/` | flags + file/stdin plumbing, writes the chart — the only I/O layer |

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
