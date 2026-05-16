package quic

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const nodeIDFileName = "node_id"

func announceNode() error {
	nodeID, err := getNodeID()
	if err != nil {
		return err
	}
	return SendMessage(&Message{Type: "hello", ID: nodeID})
}

func NodeID() (string, error) {
	return getNodeID()
}

func getNodeID() (string, error) {
	if id := strings.TrimSpace(os.Getenv("TURBO_NODE_ID")); id != "" {
		return normalizeNodeID(id), nil
	}

	if machineID, err := machineID(); err == nil && strings.TrimSpace(machineID) != "" {
		return nodeIDFromMachineID(machineID), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	turboDir := filepath.Join(configDir, "Turbo")
	path := filepath.Join(turboDir, nodeIDFileName)
	if data, err := os.ReadFile(path); err == nil {
		if id := normalizeNodeID(string(data)); id != "" {
			return id, nil
		}
	}

	id, err := newNodeID()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(turboDir, 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0600); err != nil {
		return "", err
	}
	return id, nil
}

func nodeIDFromMachineID(machineID string) string {
	sum := sha256.Sum256([]byte("turbo-node-id:v1:" + strings.TrimSpace(machineID)))
	return "node-" + hex.EncodeToString(sum[:16])
}

func newNodeID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "node-" + hex.EncodeToString(raw[:]), nil
}

func normalizeNodeID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, "node-") {
		return id
	}
	return fmt.Sprintf("node-%s", id)
}
