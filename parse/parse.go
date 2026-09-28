// Package parse reads the candle tool's CSV output into chart.Candle values. It
// binds columns by header name, so extra or reordered columns are tolerated as
// long as start/open/high/low/close are present. This CSV is the only contract
// between candle and chart.
package parse

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/jayrajadeja/chart/chart"
)

// required header columns, in the order chart needs them.
var required = []string{"start", "open", "high", "low", "close"}

// ReadCSV parses candle CSV from r. The first record must be the header row. An
// empty input (no rows) yields no candles and no error.
func ReadCSV(r io.Reader) ([]chart.Candle, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // tolerate ragged rows; we index by name

	header, err := cr.Read()
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	idx, err := columnIndex(header)
	if err != nil {
		return nil, err
	}

	var out []chart.Candle
	for line := 2; ; line++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row %d: %w", line, err)
		}
		c, err := rowToCandle(rec, idx, line)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// columnIndex maps each required column name to its position in the header.
func columnIndex(header []string) (map[string]int, error) {
	pos := make(map[string]int, len(header))
	for i, name := range header {
		pos[name] = i
	}
	idx := make(map[string]int, len(required))
	for _, name := range required {
		i, ok := pos[name]
		if !ok {
			return nil, fmt.Errorf("missing required column %q in header", name)
		}
		idx[name] = i
	}
	return idx, nil
}

// rowToCandle parses one CSV record using the resolved column indices.
func rowToCandle(rec []string, idx map[string]int, line int) (chart.Candle, error) {
	field := func(name string) (int64, error) {
		i := idx[name]
		if i >= len(rec) {
			return 0, fmt.Errorf("row %d: missing column %q", line, name)
		}
		v, err := strconv.ParseInt(rec[i], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("row %d: bad %q value %q: %w", line, name, rec[i], err)
		}
		return v, nil
	}
	var c chart.Candle
	var err error
	if c.Start, err = field("start"); err != nil {
		return c, err
	}
	if c.Open, err = field("open"); err != nil {
		return c, err
	}
	if c.High, err = field("high"); err != nil {
		return c, err
	}
	if c.Low, err = field("low"); err != nil {
		return c, err
	}
	if c.Close, err = field("close"); err != nil {
		return c, err
	}
	if c.High < c.Low {
		return c, fmt.Errorf("row %d: high %d < low %d", line, c.High, c.Low)
	}
	return c, nil
}
