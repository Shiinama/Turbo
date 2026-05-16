//go:build darwin

package quic

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func machineID() (string, error) {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return "", err
	}

	const key = `"IOPlatformUUID" = "`
	for _, line := range bytes.Split(out, []byte("\n")) {
		text := strings.TrimSpace(string(line))
		if strings.HasPrefix(text, key) && strings.HasSuffix(text, `"`) {
			return strings.TrimSuffix(strings.TrimPrefix(text, key), `"`), nil
		}
	}
	return "", fmt.Errorf("IOPlatformUUID not found")
}
