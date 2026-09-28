# Contributing to chart

Thanks for helping improve **chart** — ASCII candlestick charts over the CSV that
the `candle` tool emits.

## Prerequisites

- Go 1.26 or newer (see `go.mod`).
- Standard library only. **Do not add third-party dependencies** without discussion;
  keeping the module dependency-free is a design goal of this series.

## Local checks

Run the full gate from the repository root before opening a pull request:

```bash
go build ./... && go vet ./... && go test ./...
```

## Coding expectations

- **Test-first.** Add or update a failing test before the change that makes it pass.
- **No floats.** Price-to-row scaling is exact integer math; the chart must be
  byte-identical for identical input. Never introduce floating-point rendering.
- **Keep the core pure.** `chart` and `parse` return errors and never `os.Exit`,
  print, or panic on bad input. All I/O (files/stdin/stdout/argv) lives in
  `cmd/chart`, behind the `run(args, stdin, stdout, stderr) int` seam.
- **CSV is the contract with candle.** chart binds columns by header name and must
  not import the candle module. Track the format through the text, not a Go
  dependency.
- **Minimal, surgical changes.** No speculative features, no unrelated refactors.
- Update documentation when behavior or flags change.
- Never commit secrets.

## Pull-request checklist

- `go build ./... && go vet ./... && go test ./...` is green.
- New behavior is covered by tests.
- Docs (README, specs) reflect the change.
- No unrelated files are modified.

## Design docs

Specs and implementation plans live under `docs/superpowers/`. For a non-trivial
change, add or update the relevant spec before implementing.
