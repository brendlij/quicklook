package history

import (
	"github.com/quicklook/quicklook/internal/metrics"
	"sync"
)

type Ring struct {
	mu     sync.RWMutex
	points []metrics.Point
	next   int
	full   bool
}

func New(capacity int) *Ring { return &Ring{points: make([]metrics.Point, capacity)} }

func (r *Ring) Add(p metrics.Point) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.points[r.next] = p
	r.next = (r.next + 1) % len(r.points)
	if r.next == 0 {
		r.full = true
	}
}

func (r *Ring) Points() []metrics.Point {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.full {
		return append([]metrics.Point(nil), r.points[:r.next]...)
	}
	out := make([]metrics.Point, 0, len(r.points))
	out = append(out, r.points[r.next:]...)
	out = append(out, r.points[:r.next]...)
	return out
}
