# chart — ASCII candlestick renderer (design)

Project **D** in the `lob → tickstore → candle → chart` series: the visualisation
stage. A small Go tool that turns `candle --csv` output into a terminal candlestick
chart. Standard library only, pure integer rendering, deterministic.

## Problem

The pipeline can generate, store, and aggregate a trade stream, but the only way to
*look* at the result is to read rows of numbers. A candlestick chart is the native
way humans read OHLC data. chart closes the loop: pipe candles in, see the shape.

## Goal

Read candle CSV (`start,open,high,low,close,...`) from stdin or a file and draw an
ASCII candlestick chart to stdout: one column per candle, a low-high wick with an
open-close body, a left price axis, and a summary footer. Byte-identical output for
identical input.

Non-goals (cut hard, YAGNI): no color, no interactivity/scrolling, no real-time
follow mode, no volume panel, no wall-clock time axis, no image/SVG output, no
technical indicators. One static chart, drawn honestly.

## Design

### Decoupling: the CSV is the contract

chart cooperates with `candle` through candle's **text CSV output**, not a Go
import — the same "share a data format, not a package" principle the rest of the
series follows (candle ↔ tickstore share the 25-byte record; chart ↔ candle share
the CSV). `parse` binds columns by header **name**, so extra or reordered columns
are tolerated as long as `start/open/high/low/close` are present.

### Components

- **`chart` (pure core).** `Render(candles []chart.Candle, opts Options) ([]string,
  error)`. `Candle` is the minimal OHLC bar (`Start,Open,High,Low,Close int64`).
  `Options{Height int, UpDown bool}`. Returns the chart as lines (no trailing
  newline). Errors: `ErrBadHeight` (height ≤ 0), `ErrNoCandles` (empty input).
- **`parse` (pure).** `ReadCSV(r io.Reader) ([]chart.Candle, error)` using
  `encoding/csv`, columns bound by header name, ragged rows tolerated. Empty input
  and header-only input yield no candles, no error. Validates `high >= low`.
- **`cmd/chart` (I/O only).** `run(args, stdin, stdout, stderr) int` seam: parse
  `--height`/`--updown`, read stdin or a file arg, `parse.ReadCSV`, `chart.Render`,
  print. `main` is just `os.Exit(run(...))`.

### Rendering (pure integer)

1. `priceMax = max High`, `priceMin = min Low` over all candles.
2. Row scaling, higher price → upper row:
   `rowOf(p) = (priceMax - p) * (height - 1) / (priceMax - priceMin)`.
   A flat series (`priceMax == priceMin`) collapses every price to the bottom row.
3. For each candle and row `r`: if `r` is within the open-close body rows, draw the
   **body** glyph; else if within the high-low wick rows, draw `│`; else blank.
   Body is drawn before wick so bodies sit on top.
4. Body glyph: `█` for up (`Close >= Open`); with `--updown`, `▒` for down
   (`Close < Open`); without it, always `█`.
5. Each row line is `<price label> │ <one glyph per candle>`, the label right-aligned
   to the width of the widest of `priceMax`/`priceMin`. `priceAt(r)` gives the row's
   label by the inverse mapping.
6. A footer line (aligned past the gutter) reports
   `candles=N  start=[first..last]  price=[min..max]  height=H`.

## Error handling

- `height <= 0` -> `ErrBadHeight` (cmd prints, exit 1).
- no candles (empty or header-only input) -> `ErrNoCandles` (exit 1).
- bad CSV: missing required column, non-numeric field, or `high < low` -> a wrapped
  error naming the row (exit 1).
- missing file / bad flag -> exit 1 / exit 2.

## Testing

- `chart/chart_test.go` — hand-computed grids: a single candle (exact per-row
  glyphs), up/down glyph selection (with and without `--updown`), flat range,
  footer contents, and row-count/width invariants. `ErrBadHeight`/`ErrNoCandles`.
- `parse/parse_test.go` — happy path, reordered + extra columns, empty input,
  header-only, missing column, non-numeric field, `high < low`.
- `cmd/chart/main_test.go` — `run` over stdin and a file arg (exit 0, footer, glyphs
  present), bad height (exit 1), empty input (exit 1), missing file (exit 1), bad
  flag (exit 2).
- Manual e2e: `lob-replay --emit | candle --width W --csv | chart --height H` renders
  a sensible chart.

## Layout

| package | job |
|---------|-----|
| `chart/` | `Render` — candles + height → ASCII grid lines (pure) |
| `parse/` | `ReadCSV` — candle CSV → `[]chart.Candle`, columns bound by name (pure) |
| `cmd/chart/` | flags + file/stdin plumbing — the only I/O layer |
