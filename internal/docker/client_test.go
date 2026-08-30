package docker

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestContainerStatsJSON(t *testing.T) {
	const payload = `{
		"cpu_stats":{"cpu_usage":{"total_usage":240,"percpu_usage":[120,120]},"system_cpu_usage":1000,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":900},
		"memory_stats":{"usage":1048576,"limit":2097152,"stats":{"cache":4096}},
		"networks":{"eth0":{"rx_bytes":1200,"tx_bytes":500}}
	}`
	var stats containerStats
	if err := json.NewDecoder(strings.NewReader(payload)).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.CPUStats.CPUUsage.TotalUsage != 240 || stats.CPUStats.OnlineCPUs != 2 {
		t.Fatalf("CPU stats not decoded: %+v", stats.CPUStats)
	}
	if stats.MemoryStats.Usage != 1048576 || stats.Networks["eth0"].RxBytes != 1200 {
		t.Fatalf("memory/network stats not decoded: %+v", stats)
	}
}

func TestCounterDelta(t *testing.T) {
	if got := counterDelta(20, 5); got != 15 {
		t.Fatalf("delta = %d", got)
	}
	if got := counterDelta(5, 20); got != 0 {
		t.Fatalf("wrapped delta = %d", got)
	}
}
