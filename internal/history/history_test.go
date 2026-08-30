package history

import (
	"testing"

	"github.com/quicklook/quicklook/internal/metrics"
)

func TestRingKeepsNewestPointsInOrder(t *testing.T) {
	ring := New(3)
	for i := int64(1); i <= 5; i++ {
		ring.Add(metrics.Point{Time: i})
	}
	points := ring.Points()
	if len(points) != 3 || points[0].Time != 3 || points[2].Time != 5 {
		t.Fatalf("unexpected points: %+v", points)
	}
}
