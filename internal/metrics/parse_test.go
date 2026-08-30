package metrics

import (
	"strings"
	"testing"
)

func TestParseCPUStat(t *testing.T) {
	p, err := ParseCPUStat(strings.NewReader("cpu  100 10 20 400 5 2 3 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 540 || p.Idle != 405 {
		t.Fatalf("unexpected: %+v", p)
	}
	if got := CPUUsage(p, CPUTimes{Total: 640, Idle: 425}); got != 80 {
		t.Fatalf("usage = %v", got)
	}
}

func TestParseMeminfo(t *testing.T) {
	m, err := ParseMeminfo(strings.NewReader("MemTotal: 1000 kB\nMemAvailable: 250 kB\nSwapTotal: 100 kB\nSwapFree: 40 kB\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Used != 750*1024 || m.Usage != 75 || m.SwapUsed != 60*1024 {
		t.Fatalf("unexpected: %+v", m)
	}
}

func TestParseNetDev(t *testing.T) {
	n, err := ParseNetDev(strings.NewReader("Inter-| Receive\n eth0: 123 0 0 0 0 0 0 0 456 0 0 0 0 0 0 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n["eth0"] != [2]uint64{123, 456} {
		t.Fatalf("unexpected: %+v", n)
	}
}
