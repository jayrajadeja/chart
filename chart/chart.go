// Package chart renders OHLC candles as a deterministic ASCII candlestick grid.
// It is a pure, integer-only core with no IO: prices map to rows by integer
// scaling, each candle occupies one column (a low..high wick with an open..close
// body). It shares nothing with the candle tool but the text CSV it consumes.
package chart

import "errors"

// ErrBadHeight is returned when the requested row count is not positive.
var ErrBadHeight = errors.New("chart: height must be positive")

// ErrNoCandles is returned when there is nothing to draw.
var ErrNoCandles = errors.New("chart: no candles to render")

// Candle is the minimal OHLC bar chart needs. Start is the candle's logical-time
// start (for the footer); prices are integer ticks. Callers are expected to pass
// consistent bars (Low <= Open,Close <= High), as the candle tool emits.
type Candle struct {
	Start int64
	Open  int64
	High  int64
	Low   int64
	Close int64
}

// Options controls rendering.
type Options struct {
	Height int  // number of price rows (> 0)
	UpDown bool // distinct body glyph for up (Close>=Open) vs down candles
}

// Glyphs used in the grid.
const (
	glyphUp   = '█' // body, Close >= Open
	glyphDown = '▒' // body, Close <  Open (only when Options.UpDown)
	glyphWick = '│' // high..low outside the body
	glyphGap  = ' '
)

// Render returns the chart as a slice of lines (no trailing newline). Each line is
// a price label, an axis separator, then one column per candle. The first line is
// the highest price row, the last the lowest; a footer line summarises the series.
func Render(candles []Candle, opts Options) ([]string, error) {
	if opts.Height <= 0 {
		return nil, ErrBadHeight
	}
	if len(candles) == 0 {
		return nil, ErrNoCandles
	}

	pMax, pMin := candles[0].High, candles[0].Low
	for _, c := range candles {
		if c.High > pMax {
			pMax = c.High
		}
		if c.Low < pMin {
			pMin = c.Low
		}
	}

	h := opts.Height
	// rowOf maps a price to a row index: higher price -> smaller (upper) row.
	// A flat range (pMax == pMin) collapses to the bottom row.
	span := pMax - pMin
	rowOf := func(p int64) int {
		if span == 0 {
			return h - 1
		}
		return int((pMax - p) * int64(h-1) / span)
	}
	// priceAt is the inverse used for the left axis label of a row.
	priceAt := func(r int) int64 {
		if span == 0 || h == 1 {
			return pMax
		}
		return pMax - int64(r)*span/int64(h-1)
	}

	labelW := max(numWidth(pMax), numWidth(pMin))
	lines := make([]string, 0, h+1)
	for r := 0; r < h; r++ {
		row := make([]rune, 0, len(candles))
		for _, c := range candles {
			row = append(row, cell(c, r, rowOf, opts.UpDown))
		}
		lines = append(lines, padLeft(priceAt(r), labelW)+" │ "+string(row))
	}
	lines = append(lines, footer(candles, pMin, pMax, h, labelW))
	return lines, nil
}

// cell returns the glyph for candle c at row r.
func cell(c Candle, r int, rowOf func(int64) int, upDown bool) rune {
	bodyHi, bodyLo := c.Open, c.Close
	if bodyLo > bodyHi {
		bodyHi, bodyLo = bodyLo, bodyHi
	}
	bodyTop, bodyBot := rowOf(bodyHi), rowOf(bodyLo) // top row <= bottom row
	if r >= bodyTop && r <= bodyBot {
		if upDown && c.Close < c.Open {
			return glyphDown
		}
		return glyphUp
	}
	wickTop, wickBot := rowOf(c.High), rowOf(c.Low)
	if r >= wickTop && r <= wickBot {
		return glyphWick
	}
	return glyphGap
}

// footer summarises the series under the grid, aligned past the price gutter.
func footer(candles []Candle, pMin, pMax int64, h int, labelW int) string {
	gutter := spaces(labelW) + " │ "
	return gutter + "candles=" + itoa(int64(len(candles))) +
		"  start=[" + itoa(candles[0].Start) + ".." + itoa(candles[len(candles)-1].Start) + "]" +
		"  price=[" + itoa(pMin) + ".." + itoa(pMax) + "]" +
		"  height=" + itoa(int64(h))
}
