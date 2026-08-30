package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/quicklook/quicklook/internal/metrics"
)

type Client struct {
	socket  string
	enabled bool
	http    *http.Client
}

func New(socket string, enabled bool) *Client {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", socket)
	}}
	return &Client{socket: socket, enabled: enabled, http: &http.Client{Transport: transport, Timeout: 5 * time.Second}}
}

func (c *Client) Collect(ctx context.Context) metrics.Docker {
	if !c.enabled {
		return metrics.Docker{Error: "disabled by configuration"}
	}
	if _, err := os.Stat(c.socket); err != nil {
		return metrics.Docker{Error: "Docker socket not available"}
	}
	var summaries []containerSummary
	if err := c.get(ctx, "/containers/json?all=1", &summaries); err != nil {
		return metrics.Docker{Error: err.Error()}
	}
	result := metrics.Docker{Available: true, Containers: make([]metrics.Container, len(summaries))}
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 6)
	for i, summary := range summaries {
		i, summary := i, summary
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			result.Containers[i] = c.container(ctx, summary)
		}()
	}
	wg.Wait()
	for _, item := range result.Containers {
		if item.State == "running" {
			result.Running++
		} else {
			result.Stopped++
		}
	}
	return result
}

func (c *Client) container(ctx context.Context, summary containerSummary) metrics.Container {
	item := metrics.Container{ID: summary.ID, Name: strings.TrimPrefix(first(summary.Names), "/"), Image: summary.Image, State: summary.State, Status: summary.Status}
	for _, p := range summary.Ports {
		item.Ports = append(item.Ports, metrics.ContainerPort{IP: p.IP, Type: p.Type, PrivatePort: p.PrivatePort, PublicPort: p.PublicPort})
	}
	var detail containerDetail
	if c.get(ctx, "/containers/"+url.PathEscape(summary.ID)+"/json", &detail) == nil {
		item.RestartCount = detail.RestartCount
		item.Health = detail.State.Health.Status
		if started, err := time.Parse(time.RFC3339Nano, detail.State.StartedAt); err == nil && !started.IsZero() {
			item.StartedAt = &started
		}
	}
	if summary.State == "running" {
		var stat containerStats
		if c.get(ctx, "/containers/"+url.PathEscape(summary.ID)+"/stats?stream=false", &stat) == nil {
			cpuDelta := float64(counterDelta(stat.CPUStats.CPUUsage.TotalUsage, stat.PreCPUStats.CPUUsage.TotalUsage))
			systemDelta := float64(counterDelta(stat.CPUStats.SystemCPUUsage, stat.PreCPUStats.SystemCPUUsage))
			cpus := stat.CPUStats.OnlineCPUs
			if cpus == 0 {
				cpus = len(stat.CPUStats.CPUUsage.PercpuUsage)
			}
			if systemDelta > 0 && cpuDelta >= 0 {
				item.CPU = cpuDelta / systemDelta * float64(cpus) * 100
			}
			item.Memory = counterDelta(stat.MemoryStats.Usage, stat.MemoryStats.Stats.Cache)
			item.MemoryLimit = stat.MemoryStats.Limit
			for _, network := range stat.Networks {
				item.NetworkRX += network.RxBytes
				item.NetworkTX += network.TxBytes
			}
		}
	}
	return item
}

func (c *Client) get(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Docker unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Docker API returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func first(values []string) string {
	if len(values) == 0 {
		return "unknown"
	}
	return values[0]
}

func counterDelta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}

type containerSummary struct {
	ID                   string
	Names                []string
	Image, State, Status string
	Ports                []struct {
		IP, Type                string
		PrivatePort, PublicPort uint16
	}
}
type containerDetail struct {
	RestartCount int
	State        struct {
		StartedAt string
		Health    struct{ Status string }
	}
}
type containerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     int    `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
		Stats struct {
			Cache uint64 `json:"cache"`
		} `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}
