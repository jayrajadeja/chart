package parse

import (
	"strings"
	"testing"

	"github.com/jayrajadeja/chart/chart"
)

const candleHeader = "start,open,high,low,close,volume,vwap,trades,buy_vol,sell_vol"

func TestReadCSVHappyPath(t *testing.T) {
	in := candleHeader + "\n" +
		"0,10001,10005,9996,9998,134,10000,41,76,58\n" +
		"60,9997,10004,9995,10000,161,9999,47,58,103\n"
	got, err := ReadCSV(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []chart.Candle{
		{Start: 0, Open: 10001, High: 10005, Low: 9996, Close: 9998},
		{Start: 60, Open: 9997, High: 10004, Low: 9995, Close: 10000},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d candles, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candle %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestReadCSVReorderedAndExtraColumns(t *testing.T) {
	in := "open,close,low,high,start,extra\n" +
		"100,105,99,110,7,ignored\n"
	got, err := ReadCSV(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := chart.Candle{Start: 7, Open: 100, High: 110, Low: 99, Close: 105}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestReadCSVEmptyInput(t *testing.T) {
	got, err := ReadCSV(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want no candles, got %d", len(got))
	}
}

func TestReadCSVHeaderOnly(t *testing.T) {
	got, err := ReadCSV(strings.NewReader(candleHeader + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want no candles, got %d", len(got))
	}
}

func TestReadCSVMissingColumn(t *testing.T) {
	in := "start,open,high,close\n0,1,2,1\n" // no "low"
	if _, err := ReadCSV(strings.NewReader(in)); err == nil {
		t.Fatal("want error for missing low column")
	}
}

func TestReadCSVBadNumber(t *testing.T) {
	in := candleHeader + "\n0,abc,10005,9996,9998,1,1,1,1,1\n"
	if _, err := ReadCSV(strings.NewReader(in)); err == nil {
		t.Fatal("want error for non-numeric open")
	}
}

func TestReadCSVHighBelowLow(t *testing.T) {
	in := "start,open,high,low,close\n0,5,4,9,5\n"
	if _, err := ReadCSV(strings.NewReader(in)); err == nil {
		t.Fatal("want error for high < low")
	}
}
