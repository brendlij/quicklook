package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/quicklook/quicklook/internal/metrics"
	"github.com/quicklook/quicklook/internal/state"
	webassets "github.com/quicklook/quicklook/web"
)

type Server struct {
	http      *http.Server
	store     *state.Store
	shutdown  chan struct{}
	closeOnce sync.Once
}

func New(address string, store *state.Store, version string) *Server {
	s := &Server{store: store, shutdown: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("GET /api/v1/status", s.status)
	mux.HandleFunc("GET /api/v1/cpu", s.part(func(v metrics.Snapshot) any { return v.CPU }))
	mux.HandleFunc("GET /api/v1/memory", s.part(func(v metrics.Snapshot) any { return v.Memory }))
	mux.HandleFunc("GET /api/v1/storage", s.part(func(v metrics.Snapshot) any {
		return struct {
			Filesystems []metrics.Filesystem `json:"filesystems"`
			IO          metrics.DiskIO       `json:"io"`
		}{v.Storage, v.DiskIO}
	}))
	mux.HandleFunc("GET /api/v1/network", s.part(func(v metrics.Snapshot) any { return v.Network }))
	mux.HandleFunc("GET /api/v1/containers", s.part(func(v metrics.Snapshot) any { return v.Docker }))
	mux.HandleFunc("GET /api/v1/history", s.part(func(v metrics.Snapshot) any { return v.History }))
	mux.HandleFunc("GET /api/v1/events", s.events)
	assets, _ := fs.Sub(webassets.Files, ".")
	mux.Handle("/", http.FileServer(http.FS(assets)))
	s.http = &http.Server{Addr: address, Handler: securityHeaders(mux, version), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	return s
}

func (s *Server) ListenAndServe() error { return s.http.ListenAndServe() }
func (s *Server) Shutdown(ctx context.Context) error {
	s.closeOnce.Do(func() { close(s.shutdown) })
	return s.http.Shutdown(ctx)
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) { writeJSON(w, s.store.Get()) }
func (s *Server) part(selectValue func(metrics.Snapshot) any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, selectValue(s.store.Get())) }
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	updates, unsubscribe := s.store.Subscribe()
	defer unsubscribe()
	initial := s.store.Get()
	if err := writeEvent(w, initial); err != nil {
		return
	}
	flusher.Flush()
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-s.shutdown:
			return
		case <-r.Context().Done():
			return
		case value := <-updates:
			if err := writeEvent(w, value); err != nil {
				return
			}
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func writeEvent(w http.ResponseWriter, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", data)
	return err
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
func securityHeaders(next http.Handler, version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:")
		w.Header().Set("X-Quicklook-Version", version)
		next.ServeHTTP(w, r)
	})
}
