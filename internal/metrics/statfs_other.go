//go:build !linux

package metrics

import "fmt"

func filesystemUsage(path string) (uint64, uint64, error) {
	return 0, 0, fmt.Errorf("filesystem usage unsupported on this platform")
}
