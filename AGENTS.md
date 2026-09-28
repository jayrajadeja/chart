# AGENTS.md — chart

Guidance for AI coding agents working in this repository. Human contributors: see
[`CONTRIBUTING.md`](CONTRIBUTING.md).

**What this is:** an ASCII candlestick renderer. It reads the CSV that `candle`
prints (`start,open,high,low,close,...`) and draws one column per candle — a
low-high wick with an open-close body — scaled to the terminal with pure integer
math. Standard library only, deterministic. **Project D** in the
`lob → tickstore → candle → chart` series, the visualisation stage.

## Commands you must run

```bash
go build ./...            # compile
go build -o chart ./cmd/chart
go vet ./...              # static checks
go test ./...             # full test suite
```

**Never claim a change is done until `go build ./... && go vet ./... && go test ./...`
passes.** Show the output; don't assert.

## Non-negotiable conventions

- **No floats.** Price-to-row scaling is exact integer arithmetic
  (`row = (priceMax - price) * (height - 1) / (priceMax - priceMin)`). The chart must
  be byte-identical for identical input. Never render with floating-point.
- **CSV is the cross-repo contract.** chart cooperates with `candle` through its CSV
  output only; it binds columns by header name (`start/open/high/low/close` required)
  and must **not** import the candle Go module. Track the format via text.
- **Body over wick.** For each candle/row, fill the open-close body first, then the
  high-low wick, then blank — bodies always sit on top of wicks.
- **I/O is isolated.** `chart` and `parse` are pure (return errors, no `os.Exit`/
  print/panic). Only `cmd/chart` touches files/stdin/stdout/argv, through a testable
  `run(args, stdin, stdout, stderr) int` seam.
- **Determinism.** Same input ⇒ byte-identical chart.
- **Minimal, surgical diffs.** Keep every safety guard; write the failing test first.

## Workflow

- Design specs and implementation plans live under `docs/superpowers/`. Read the
  relevant spec before a non-trivial change; add one for new work.
- **Commits:** include the trailer
  `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`.
  Git identity is machine-level — do not hard-code author info.
- **Pull requests:** open a PR and let a human merge it. **Never self-merge.**

## Layout

| package | job |
|---------|-----|
| `chart/` | `Render` — candles + height → ASCII grid lines (pure) |
| `parse/` | `ReadCSV` — candle CSV → `[]chart.Candle`, columns bound by name (pure) |
| `cmd/chart/` | flags + file/stdin plumbing, writes the chart — the only I/O layer |
