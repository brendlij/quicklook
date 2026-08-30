package metrics

import "time"

type CPU struct {
	Usage       float64  `json:"usage"`
	Model       string   `json:"model"`
	Cores       int      `json:"cores"`
	Threads     int      `json:"threads"`
	Temperature *float64 `json:"temperature"`
}

type Memory struct {
	Used      uint64  `json:"used"`
	Total     uint64  `json:"total"`
	Available uint64  `json:"available"`
	Usage     float64 `json:"usage"`
	SwapUsed  uint64  `json:"swap_used"`
	SwapTotal uint64  `json:"swap_total"`
}

type Load struct{ One, Five, Fifteen float64 }

func (l Load) MarshalJSON() ([]byte, error) {
	return []byte(fmtLoad(l)), nil
}

type Filesystem struct {
	Device     string  `json:"device"`
	MountPoint string  `json:"mount_point"`
	Type       string  `json:"type"`
	Used       uint64  `json:"used"`
	Total      uint64  `json:"total"`
	Usage      float64 `json:"usage"`
}

type DiskIO struct{ ReadBPS, WriteBPS float64 }

func (d DiskIO) MarshalJSON() ([]byte, error) { return []byte(fmtDisk(d)), nil }

type Interface struct {
	Name  string   `json:"name"`
	IPv4  []string `json:"ipv4"`
	IPv6  []string `json:"ipv6"`
	RX    uint64   `json:"rx"`
	TX    uint64   `json:"tx"`
	RXBPS float64  `json:"rx_bps"`
	TXBPS float64  `json:"tx_bps"`
	State string   `json:"state"`
}

type Network struct {
	RXBPS      float64     `json:"rx_bps"`
	TXBPS      float64     `json:"tx_bps"`
	Interfaces []Interface `json:"interfaces"`
}

type Host struct {
	Hostname     string  `json:"hostname"`
	Distribution string  `json:"distribution"`
	Kernel       string  `json:"kernel"`
	Architecture string  `json:"architecture"`
	Uptime       float64 `json:"uptime"`
}

type ContainerPort struct {
	IP          string `json:"ip"`
	Type        string `json:"type"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port"`
}

type Container struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Image        string          `json:"image"`
	State        string          `json:"state"`
	Status       string          `json:"status"`
	Health       string          `json:"health,omitempty"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	RestartCount int             `json:"restart_count"`
	CPU          float64         `json:"cpu"`
	Memory       uint64          `json:"memory"`
	MemoryLimit  uint64          `json:"memory_limit"`
	NetworkRX    uint64          `json:"network_rx"`
	NetworkTX    uint64          `json:"network_tx"`
	Ports        []ContainerPort `json:"ports,omitempty"`
}

type Docker struct {
	Available  bool        `json:"available"`
	Error      string      `json:"error,omitempty"`
	Running    int         `json:"running"`
	Stopped    int         `json:"stopped"`
	Containers []Container `json:"containers"`
}

type Point struct {
	Time      int64   `json:"time"`
	CPU       float64 `json:"cpu"`
	Memory    float64 `json:"memory"`
	NetworkRX float64 `json:"network_rx"`
	NetworkTX float64 `json:"network_tx"`
	DiskRead  float64 `json:"disk_read"`
	DiskWrite float64 `json:"disk_write"`
}

type Snapshot struct {
	Timestamp time.Time    `json:"timestamp"`
	Host      Host         `json:"host"`
	CPU       CPU          `json:"cpu"`
	Memory    Memory       `json:"memory"`
	Load      Load         `json:"load"`
	Storage   []Filesystem `json:"storage"`
	DiskIO    DiskIO       `json:"disk_io"`
	Network   Network      `json:"network"`
	Docker    Docker       `json:"docker"`
	History   []Point      `json:"history"`
}
