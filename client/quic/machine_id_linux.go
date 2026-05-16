//go:build linux

package quic

import (
	"fmt"
	"os"
	"strings"
)

func machineID() (string, error) {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		data, err := os.ReadFile(path)
		if err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("machine-id not found")
}
