package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleCSV = "start,open,high,low,close,volume,vwap,trades,buy_vol,sell_vol\n" +
	"0,10001,10005,9996,9998,134,10000,41,76,58\n" +
	"60,9997,10004,9995,10000,161,9999,47,58,103\n" +
	"120,9999,10002,9995,10000,116,9999,39,72,44\n"

func TestRunStdin(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"--height", "8"}, strings.NewReader(sampleCSV), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errBuf.String())
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 9 { // 8 grid rows + footer
		t.Fatalf("want 9 lines, got %d:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[len(lines)-1], "candles=3") {
		t.Fatalf("footer: %s", lines[len(lines)-1])
	}
	if !strings.Contains(out.String(), "│") {
		t.Fatal("expected candlestick glyphs in output")
	}
}

func TestRunFileArg(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "candles.csv")
	if err := os.WriteFile(path, []byte(sampleCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	if code := run([]string{path}, strings.NewReader(""), &out, &errBuf); code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "candles=3") {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestRunBadHeight(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"--height", "0"}, strings.NewReader(sampleCSV), &out, &errBuf); code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "height") {
		t.Fatalf("stderr: %s", errBuf.String())
	}
}

func TestRunEmptyInputNoCandles(t *testing.T) {
	var out, errBuf bytes.Buffer
	// No rows -> Render returns ErrNoCandles -> exit 1.
	if code := run(nil, strings.NewReader(""), &out, &errBuf); code != 1 {
		t.Fatalf("want exit 1 on empty, got %d", code)
	}
}

func TestRunMissingFile(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"/no/such/file.csv"}, strings.NewReader(""), &out, &errBuf); code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
}

func TestRunBadFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"--nope"}, strings.NewReader(sampleCSV), &out, &errBuf); code != 2 {
		t.Fatalf("want exit 2 on bad flag, got %d", code)
	}
}
