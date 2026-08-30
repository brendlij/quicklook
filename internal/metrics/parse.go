package metrics

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type CPUTimes struct{ Total, Idle uint64 }

func ParseCPUStat(r io.Reader) (CPUTimes, error) {
	s := bufio.NewScanner(r)
	if !s.Scan() {
		return CPUTimes{}, fmt.Errorf("read cpu stat: %w", s.Err())
	}
	f := strings.Fields(s.Text())
	if len(f) < 5 || f[0] != "cpu" {
		return CPUTimes{}, fmt.Errorf("invalid cpu stat")
	}
	var values []uint64
	for _, raw := range f[1:] {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return CPUTimes{}, err
		}
		values = append(values, n)
	}
	var total uint64
	for _, n := range values {
		total += n
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return CPUTimes{Total: total, Idle: idle}, nil
}

func CPUUsage(prev, current CPUTimes) float64 {
	if current.Total <= prev.Total {
		return 0
	}
	total := current.Total - prev.Total
	idle := delta(current.Idle, prev.Idle)
	if idle >= total {
		return 0
	}
	return clamp(float64(total-idle) / float64(total) * 100)
}

func ParseMeminfo(r io.Reader) (Memory, error) {
	values := map[string]uint64{}
	s := bufio.NewScanner(r)
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) < 2 {
			continue
		}
		n, err := strconv.ParseUint(f[1], 10, 64)
		if err == nil {
			values[strings.TrimSuffix(f[0], ":")] = n * 1024
		}
	}
	if err := s.Err(); err != nil {
		return Memory{}, err
	}
	total := values["MemTotal"]
	if total == 0 {
		return Memory{}, fmt.Errorf("MemTotal missing")
	}
	available := values["MemAvailable"]
	if available == 0 {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}
	if available > total {
		available = total
	}
	used := total - available
	swapTotal, swapFree := values["SwapTotal"], values["SwapFree"]
	if swapFree > swapTotal {
		swapFree = swapTotal
	}
	return Memory{Used: used, Total: total, Available: available, Usage: clamp(float64(used) / float64(total) * 100), SwapUsed: swapTotal - swapFree, SwapTotal: swapTotal}, nil
}

func ParseNetDev(r io.Reader) (map[string][2]uint64, error) {
	out := map[string][2]uint64{}
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		f := strings.Fields(line[colon+1:])
		if len(f) < 9 {
			continue
		}
		rx, e1 := strconv.ParseUint(f[0], 10, 64)
		tx, e2 := strconv.ParseUint(f[8], 10, 64)
		if e1 == nil && e2 == nil {
			out[name] = [2]uint64{rx, tx}
		}
	}
	return out, s.Err()
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
