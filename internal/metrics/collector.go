package metrics

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Collector struct {
	proc, sys, root string
	previousCPU     CPUTimes
	previousNet     map[string][2]uint64
	previousDisk    [2]uint64
	previousAt      time.Time
}

func NewCollector(proc, sys, root string) *Collector {
	return &Collector{proc: proc, sys: sys, root: root, previousNet: map[string][2]uint64{}}
}

func (c *Collector) Collect() (Host, CPU, Memory, Load, []Filesystem, DiskIO, Network, error) {
	now := time.Now()
	host := c.host()
	cpu, cpuTimes, cpuErr := c.cpu()
	memory, memErr := c.memory()
	load, loadErr := c.load()
	storage, _ := c.storage()
	disk, diskRaw, _ := c.diskIO()
	network, netRaw, _ := c.network()
	if !c.previousAt.IsZero() {
		seconds := now.Sub(c.previousAt).Seconds()
		cpu.Usage = CPUUsage(c.previousCPU, cpuTimes)
		if seconds > 0 {
			disk.ReadBPS = float64(delta(diskRaw[0], c.previousDisk[0])) * 512 / seconds
			disk.WriteBPS = float64(delta(diskRaw[1], c.previousDisk[1])) * 512 / seconds
			for i := range network.Interfaces {
				if old, ok := c.previousNet[network.Interfaces[i].Name]; ok {
					network.Interfaces[i].RXBPS = float64(delta(network.Interfaces[i].RX, old[0])) / seconds
					network.Interfaces[i].TXBPS = float64(delta(network.Interfaces[i].TX, old[1])) / seconds
					network.RXBPS += network.Interfaces[i].RXBPS
					network.TXBPS += network.Interfaces[i].TXBPS
				}
			}
		}
	}
	c.previousCPU, c.previousNet, c.previousDisk, c.previousAt = cpuTimes, netRaw, diskRaw, now
	return host, cpu, memory, load, storage, disk, network, errors.Join(cpuErr, memErr, loadErr)
}

func (c *Collector) host() Host {
	hostname, _ := os.Hostname()
	h := Host{Hostname: hostname, Architecture: runtime.GOARCH}
	if data, err := os.ReadFile(filepath.Join(c.proc, "uptime")); err == nil {
		fmt.Sscan(string(data), &h.Uptime)
	}
	if data, err := os.ReadFile(filepath.Join(c.proc, "sys", "kernel", "osrelease")); err == nil {
		h.Kernel = strings.TrimSpace(string(data))
	}
	for _, path := range []string{filepath.Join(c.root, "etc", "os-release"), "/etc/os-release"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				h.Distribution = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
				break
			}
		}
		if h.Distribution != "" {
			break
		}
	}
	if h.Distribution == "" {
		h.Distribution = "Linux"
	}
	return h
}

func (c *Collector) cpu() (CPU, CPUTimes, error) {
	f, err := os.Open(filepath.Join(c.proc, "stat"))
	if err != nil {
		return CPU{}, CPUTimes{}, err
	}
	defer f.Close()
	times, err := ParseCPUStat(f)
	if err != nil {
		return CPU{}, CPUTimes{}, err
	}
	cpu := CPU{Threads: runtime.NumCPU()}
	if data, readErr := os.ReadFile(filepath.Join(c.proc, "cpuinfo")); readErr == nil {
		physical := map[string]bool{}
		for _, block := range strings.Split(string(data), "\n\n") {
			physicalID, coreID := "", ""
			for _, line := range strings.Split(block, "\n") {
				pair := strings.SplitN(line, ":", 2)
				if len(pair) != 2 {
					continue
				}
				key, val := strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])
				switch key {
				case "model name", "Hardware":
					if cpu.Model == "" {
						cpu.Model = val
					}
				case "physical id":
					physicalID = val
				case "core id":
					coreID = val
				}
			}
			if coreID != "" {
				physical[physicalID+":"+coreID] = true
			}
		}
		cpu.Cores = len(physical)
	}
	if cpu.Model == "" {
		cpu.Model = "Unknown processor"
	}
	if cpu.Cores == 0 {
		cpu.Cores = cpu.Threads
	}
	cpu.Temperature = c.temperature()
	return cpu, times, err
}

func (c *Collector) memory() (Memory, error) {
	f, err := os.Open(filepath.Join(c.proc, "meminfo"))
	if err != nil {
		return Memory{}, err
	}
	defer f.Close()
	return ParseMeminfo(f)
}

func (c *Collector) load() (Load, error) {
	data, err := os.ReadFile(filepath.Join(c.proc, "loadavg"))
	if err != nil {
		return Load{}, err
	}
	var l Load
	_, err = fmt.Sscan(string(data), &l.One, &l.Five, &l.Fifteen)
	return l, err
}

func (c *Collector) temperature() *float64 {
	paths, _ := filepath.Glob(filepath.Join(c.sys, "class", "hwmon", "hwmon*", "temp*_input"))
	var best *float64
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		raw, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
		if err != nil {
			continue
		}
		value := raw / 1000
		if value > 0 && value < 150 {
			if best == nil || value > *best {
				v := value
				best = &v
			}
		}
	}
	return best
}

var ignoredFS = map[string]bool{"proc": true, "sysfs": true, "tmpfs": true, "devtmpfs": true, "devpts": true, "cgroup": true, "cgroup2": true, "overlay": true, "squashfs": true, "nsfs": true, "mqueue": true, "securityfs": true, "pstore": true, "debugfs": true, "tracefs": true, "configfs": true, "fusectl": true, "autofs": true, "hugetlbfs": true, "rpc_pipefs": true}

func (c *Collector) storage() ([]Filesystem, error) {
	f, err := os.Open(filepath.Join(c.proc, "mounts"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Filesystem
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || ignoredFS[fields[2]] {
			continue
		}
		mount := unescapeMount(fields[1])
		if seen[mount] || strings.HasPrefix(mount, "/var/lib/docker/") {
			continue
		}
		seen[mount] = true
		statPath := filepath.Join(c.root, filepath.FromSlash(strings.TrimPrefix(mount, "/")))
		total, free, statErr := filesystemUsage(statPath)
		if statErr != nil || total == 0 {
			continue
		}
		used := total - free
		out = append(out, Filesystem{Device: unescapeMount(fields[0]), MountPoint: mount, Type: fields[2], Used: used, Total: total, Usage: clamp(float64(used) / float64(total) * 100)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MountPoint < out[j].MountPoint })
	return out, scanner.Err()
}

func (c *Collector) diskIO() (DiskIO, [2]uint64, error) {
	f, err := os.Open(filepath.Join(c.proc, "diskstats"))
	if err != nil {
		return DiskIO{}, [2]uint64{}, err
	}
	defer f.Close()
	var sectors [2]uint64
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		if isPartitionOrVirtual(name) {
			continue
		}
		read, _ := strconv.ParseUint(fields[5], 10, 64)
		write, _ := strconv.ParseUint(fields[9], 10, 64)
		sectors[0] += read
		sectors[1] += write
	}
	return DiskIO{}, sectors, s.Err()
}

func (c *Collector) network() (Network, map[string][2]uint64, error) {
	f, err := os.Open(filepath.Join(c.proc, "net", "dev"))
	if err != nil {
		return Network{}, nil, err
	}
	defer f.Close()
	raw, err := ParseNetDev(f)
	if err != nil {
		return Network{}, nil, err
	}
	addresses := map[string]struct{ v4, v6 []string }{}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		item := addresses[iface.Name]
		for _, addr := range addrs {
			ip, _, e := net.ParseCIDR(addr.String())
			if e != nil {
				continue
			}
			if ip.To4() != nil {
				item.v4 = append(item.v4, addr.String())
			} else {
				item.v6 = append(item.v6, addr.String())
			}
		}
		addresses[iface.Name] = item
	}
	var out Network
	for name, total := range raw {
		if name == "lo" || strings.HasPrefix(name, "veth") {
			continue
		}
		state := "unknown"
		if data, e := os.ReadFile(filepath.Join(c.sys, "class", "net", name, "operstate")); e == nil {
			state = strings.TrimSpace(string(data))
		}
		a := addresses[name]
		out.Interfaces = append(out.Interfaces, Interface{Name: name, IPv4: a.v4, IPv6: a.v6, RX: total[0], TX: total[1], State: state})
	}
	sort.Slice(out.Interfaces, func(i, j int) bool { return out.Interfaces[i].Name < out.Interfaces[j].Name })
	return out, raw, nil
}

func delta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}
func unescapeMount(v string) string {
	r := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\134`, `\`)
	return r.Replace(v)
}
func isPartitionOrVirtual(name string) bool {
	if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "dm-") {
		return true
	}
	if strings.HasPrefix(name, "nvme") {
		last := name[len(name)-1]
		return last >= '0' && last <= '9' && strings.Contains(name, "p")
	}
	last := name[len(name)-1]
	return last >= '0' && last <= '9'
}
