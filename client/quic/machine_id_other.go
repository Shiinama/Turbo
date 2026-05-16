//go:build !darwin && !linux && !windows

package quic

import "fmt"

func machineID() (string, error) {
	return "", fmt.Errorf("machine id is not supported on this platform")
}
