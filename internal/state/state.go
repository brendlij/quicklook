package state

import (
	"context"
	"sync"
	"time"

	"github.com/quicklook/quicklook/internal/docker"
	"github.com/quicklook/quicklook/internal/history"
	"github.com/quicklook/quicklook/internal/metrics"
)

type Store struct {
	mu          sync.RWMutex
	snapshot    metrics.Snapshot
	subscribers map[chan metrics.Snapshot]struct{}
}

func New() *Store { return &Store{subscribers: map[chan metrics.Snapshot]struct{}{}} }

func (s *Store) Get() metrics.Snapshot { s.mu.RLock(); defer s.mu.RUnlock(); return s.snapshot }

func (s *Store) Set(value metrics.Snapshot) {
	s.mu.Lock()
	s.snapshot = value
	for ch := range s.subscribers {
		select {
		case ch <- value:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *Store) Subscribe() (<-chan metrics.Snapshot, func()) {
	ch := make(chan metrics.Snapshot, 1)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() { s.mu.Lock(); delete(s.subscribers, ch); close(ch); s.mu.Unlock() }
}

type Sampler struct {
	collector *metrics.Collector
	docker    *docker.Client
	history   *history.Ring
	store     *Store
	interval  time.Duration
}

func NewSampler(collector *metrics.Collector, dockerClient *docker.Client, h *history.Ring, store *Store, interval time.Duration) *Sampler {
	return &Sampler{collector: collector, docker: dockerClient, history: h, store: store, interval: interval}
}

func (s *Sampler) Run(ctx context.Context) {
	s.collect(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collect(ctx)
		}
	}
}

func (s *Sampler) collect(ctx context.Context) {
	host, cpu, memory, load, storage, diskIO, network, _ := s.collector.Collect()
	dockerState := s.docker.Collect(ctx)
	now := time.Now()
	s.history.Add(metrics.Point{Time: now.UnixMilli(), CPU: cpu.Usage, Memory: memory.Usage, NetworkRX: network.RXBPS, NetworkTX: network.TXBPS, DiskRead: diskIO.ReadBPS, DiskWrite: diskIO.WriteBPS})
	s.store.Set(metrics.Snapshot{Timestamp: now, Host: host, CPU: cpu, Memory: memory, Load: load, Storage: storage, DiskIO: diskIO, Network: network, Docker: dockerState, History: s.history.Points()})
}
