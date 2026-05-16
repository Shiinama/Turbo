//go:build windows

package quic

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func machineID() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", err
	}
	defer key.Close()

	id, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return "", err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("MachineGuid is empty")
	}
	return id, nil
}
