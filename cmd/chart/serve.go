package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jayrajadeja/chart/server"
	"github.com/jayrajadeja/chart/upstream"
)

// runServeCmd parses the `serve` subcommand flags and runs the HTTP chart proxy
// until an interrupt/termination signal, then shuts down gracefully.
func runServeCmd(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("chart serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8139", "listen address")
	candlesURL := fs.String("candles-url", "http://127.0.0.1:8138", "base URL of an upstream candle serve")
	height := fs.Int("height", 20, "default number of price rows (> 0)")
	updown := fs.Bool("updown", true, "default: distinct glyph for down candles")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *height <= 0 {
		fmt.Fprintln(stderr, "chart serve: --height must be a positive integer")
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runServe(*addr, *candlesURL, *height, *updown, ctx, nil); err != nil {
		fmt.Fprintf(stderr, "chart serve: %v\n", err)
		return 1
	}
	return 0
}

// runServe starts the HTTP chart proxy against the upstream candle serve at
// candlesURL and blocks until ctx is cancelled, then shuts down gracefully. When
// ready is non-nil, the bound listen address is sent on it once listening (useful
// with :0 in tests).
func runServe(addr, candlesURL string, height int, updown bool, ctx context.Context, ready chan<- string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	src := upstream.NewHTTP(candlesURL, &http.Client{Timeout: 10 * time.Second})
	handler := server.Handler(src, server.Defaults{Height: height, UpDown: updown})
	srv := &http.Server{Handler: handler}
	if ready != nil {
		ready <- ln.Addr().String()
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	fmt.Fprintf(os.Stderr, "chart serve: listening on %s (candles-url=%s)\n", ln.Addr(), candlesURL)
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
