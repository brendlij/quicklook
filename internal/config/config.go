package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port           string
	HostProc       string
	HostSys        string
	HostRoot       string
	HistorySeconds int
	Interval       time.Duration
	DockerEnabled  bool
	DockerSocket   string
}

func Load() Config {
	interval := durationEnv("QUICKLOOK_INTERVAL", 2*time.Second)
	return Config{
		Port:           env("QUICKLOOK_PORT", "7373"),
		HostProc:       env("QUICKLOOK_HOST_PROC", "/proc"),
		HostSys:        env("QUICKLOOK_HOST_SYS", "/sys"),
		HostRoot:       env("QUICKLOOK_HOST_ROOT", "/"),
		HistorySeconds: intEnv("QUICKLOOK_HISTORY_SECONDS", 300),
		Interval:       interval,
		DockerEnabled:  boolEnv("QUICKLOOK_DOCKER", true),
		DockerSocket:   env("QUICKLOOK_DOCKER_SOCKET", "/var/run/docker.sock"),
	}
}

func (c Config) HistoryCapacity() int {
	n := int(time.Duration(c.HistorySeconds)*time.Second/c.Interval) + 1
	if n < 2 {
		return 2
	}
	if n > 3600 {
		return 3600
	}
	return n
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil || value < 500*time.Millisecond {
		return fallback
	}
	return value
}

func boolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
