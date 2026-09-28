package chart

import (
	"strings"
	"testing"
)

func TestRenderBadHeight(t *testing.T) {
	_, err := Render([]Candle{{High: 2, Low: 1}}, Options{Height: 0})
	if err != ErrBadHeight {
		t.Fatalf("height 0: got %v, want ErrBadHeight", err)
	}
}

func TestRenderNoCandles(t *testing.T) {
	_, err := Render(nil, Options{Height: 5})
	if err != ErrNoCandles {
		t.Fatalf("empty: got %v, want ErrNoCandles", err)
	}
}

// A single candle O=2,H=4,L=1,C=3 over height 4, prices 1..4.
// rowOf(p) = (4-p)*(3)/(3): p4->0, p3->1, p2->2, p1->3.
// body spans open(2,row2)..close(3,row1) => rows 1,2. wick high(row0)..low(row3).
// Expected column top..bottom: row0 wick, row1 body, row2 body, row3 wick.
func TestRenderSingleCandleGrid(t *testing.T) {
	lines, err := Render([]Candle{{Start: 0, Open: 2, High: 4, Low: 1, Close: 3}}, Options{Height: 4})
	if err != nil {
		t.Fatal(err)
	}
	col := lastRune(t, lines[:4])
	want := []rune{glyphWick, glyphUp, glyphUp, glyphWick}
	for r := range want {
		if col[r] != want[r] {
			t.Fatalf("row %d: got %q, want %q (full:\n%s)", r, col[r], want[r], strings.Join(lines, "\n"))
		}
	}
	// Price axis: top row labels pMax=4, bottom row labels pMin=1.
	if !strings.HasPrefix(lines[0], "4 │ ") {
		t.Fatalf("top axis: %q", lines[0])
	}
	if !strings.HasPrefix(lines[3], "1 │ ") {
		t.Fatalf("bottom axis: %q", lines[3])
	}
}

func TestRenderUpDownGlyphs(t *testing.T) {
	up := Candle{Open: 1, High: 3, Low: 1, Close: 3}   // close>open
	down := Candle{Open: 3, High: 3, Low: 1, Close: 1} // close<open
	lines, err := Render([]Candle{up, down}, Options{Height: 3, UpDown: true})
	if err != nil {
		t.Fatal(err)
	}
	// Middle row (row1) is inside both bodies (both span rows 0..2 here).
	body := []rune(stripAxis(t, lines[1]))
	if body[0] != glyphUp {
		t.Fatalf("up candle body: got %q want %q", body[0], glyphUp)
	}
	if body[1] != glyphDown {
		t.Fatalf("down candle body: got %q want %q", body[1], glyphDown)
	}
	// Without UpDown, both are glyphUp.
	lines2, _ := Render([]Candle{up, down}, Options{Height: 3, UpDown: false})
	body2 := []rune(stripAxis(t, lines2[1]))
	if body2[0] != glyphUp || body2[1] != glyphUp {
		t.Fatalf("no-updown: got %q%q want both %q", body2[0], body2[1], glyphUp)
	}
}

func TestRenderFlatRange(t *testing.T) {
	// All prices equal: single meaningful row, body glyph, no panic.
	lines, err := Render([]Candle{{Open: 5, High: 5, Low: 5, Close: 5}}, Options{Height: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 4 { // 3 rows + footer
		t.Fatalf("want 4 lines, got %d", len(lines))
	}
	// Flat collapses to the bottom row.
	if stripAxis(t, lines[2]) != string(glyphUp) {
		t.Fatalf("flat bottom row: %q", stripAxis(t, lines[2]))
	}
}

func TestRenderFooter(t *testing.T) {
	lines, err := Render([]Candle{
		{Start: 0, Open: 1, High: 4, Low: 1, Close: 2},
		{Start: 60, Open: 2, High: 5, Low: 2, Close: 3},
	}, Options{Height: 5})
	if err != nil {
		t.Fatal(err)
	}
	foot := lines[len(lines)-1]
	for _, want := range []string{"candles=2", "start=[0..60]", "price=[1..5]", "height=5"} {
		if !strings.Contains(foot, want) {
			t.Fatalf("footer missing %q: %s", want, foot)
		}
	}
}

func TestRenderRowsAndWidth(t *testing.T) {
	cs := []Candle{{Open: 1, High: 9, Low: 1, Close: 9}, {Open: 2, High: 8, Low: 2, Close: 8}}
	lines, err := Render(cs, Options{Height: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 8 { // 7 grid + footer
		t.Fatalf("want 8 lines, got %d", len(lines))
	}
	for r := 0; r < 7; r++ {
		if got := len([]rune(stripAxis(t, lines[r]))); got != len(cs) {
			t.Fatalf("row %d width: got %d want %d", r, got, len(cs))
		}
	}
}

// lastRune returns the final grid rune (last candle column) of each line.
func lastRune(t *testing.T, lines []string) []rune {
	t.Helper()
	out := make([]rune, len(lines))
	for i, ln := range lines {
		rs := []rune(stripAxis(t, ln))
		out[i] = rs[len(rs)-1]
	}
	return out
}

// stripAxis removes the "<label> │ " gutter, returning just the grid columns.
func stripAxis(t *testing.T, line string) string {
	t.Helper()
	const sep = " │ "
	i := strings.Index(line, sep)
	if i < 0 {
		t.Fatalf("no axis separator in %q", line)
	}
	return line[i+len(sep):]
}
