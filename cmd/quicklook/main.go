package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quicklook/quicklook/internal/config"
	"github.com/quicklook/quicklook/internal/docker"
	"github.com/quicklook/quicklook/internal/history"
	"github.com/quicklook/quicklook/internal/metrics"
	"github.com/quicklook/quicklook/internal/server"
	"github.com/quicklook/quicklook/internal/state"
)

var version = "dev"

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store := state.New()
	sampler := state.NewSampler(metrics.NewCollector(cfg.HostProc, cfg.HostSys, cfg.HostRoot), docker.New(cfg.DockerSocket, cfg.DockerEnabled), history.New(cfg.HistoryCapacity()), store, cfg.Interval)
	go sampler.Run(ctx)
	app := server.New(":"+cfg.Port, store, version)
	errCh := make(chan error, 1)
	go func() {
		log.Printf("quicklook listening on :%s (version %s)", cfg.Port, version)
		errCh <- app.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server stopped: %v", err)
			os.Exit(1)
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown: %v", err)
	}
}
