// Command chart renders candle CSV as an ASCII candlestick chart.
//
//	lob-replay --emit | candle --width 60 --csv | chart --height 20
//	chart --height 12 candles.csv
//
// It reads candle CSV (start,open,high,low,close,...) from stdin or a file argument
// and writes the chart to stdout. Pure integer rendering, standard library only.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jayrajadeja/chart/chart"
	"github.com/jayrajadeja/chart/parse"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		os.Exit(runServeCmd(args[1:], os.Stderr))
	}
	os.Exit(run(args, os.Stdin, os.Stdout, os.Stderr))
}

// run is the testable entry point: it returns a process exit code and never calls
// os.Exit itself. Flags are parsed from args; input is read from the first
// positional argument or, when none is given, from stdin.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("chart", flag.ContinueOnError)
	fs.SetOutput(stderr)
	height := fs.Int("height", 20, "number of price rows (> 0)")
	upDown := fs.Bool("updown", true, "distinct body glyph for up vs down candles")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	in := stdin
	if name := fs.Arg(0); name != "" {
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(stderr, "chart: %v\n", err)
			return 1
		}
		defer f.Close()
		in = f
	}

	candles, err := parse.ReadCSV(in)
	if err != nil {
		fmt.Fprintf(stderr, "chart: %v\n", err)
		return 1
	}

	lines, err := chart.Render(candles, chart.Options{Height: *height, UpDown: *upDown})
	if err != nil {
		fmt.Fprintf(stderr, "chart: %v\n", err)
		return 1
	}
	for _, ln := range lines {
		if _, err := fmt.Fprintln(stdout, ln); err != nil {
			fmt.Fprintf(stderr, "chart: %v\n", err)
			return 1
		}
	}
	return 0
}
