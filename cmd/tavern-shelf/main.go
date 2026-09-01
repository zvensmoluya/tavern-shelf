package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openai/tavern-shelf/internal/app"
	"github.com/openai/tavern-shelf/internal/httpapi"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tavern-shelf:", err)
		os.Exit(1)
	}
}

func run() error {
	dataDir := flag.String("data-dir", "", "managed data directory (default: user config directory)")
	listen := flag.String("listen", "127.0.0.1:8787", "HTTP listen address")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	shelf, err := app.Open(app.Options{DataDir: *dataDir, Logger: logger})
	if err != nil {
		return err
	}
	defer shelf.Close()
	handler, err := httpapi.Handler(shelf)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	scanDone := make(chan error, 1)
	go func() { scanDone <- shelf.Scanner.Run(ctx) }()

	server := &http.Server{
		Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second,
	}
	serveDone := make(chan error, 1)
	go func() {
		logger.Info("Tavern Shelf is ready", "url", "http://"+*listen, "inbox", shelf.Paths.Inbox)
		serveDone <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		<-scanDone
		return nil
	case err := <-serveDone:
		stop()
		<-scanDone
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	}
}
