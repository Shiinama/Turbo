package applog

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const logFileName = "turbo.log"

var logPath string

func Init() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("getting user config dir: %w", err)
	}

	turboDir := filepath.Join(configDir, "Turbo")
	if err := os.MkdirAll(turboDir, 0700); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	path := filepath.Join(turboDir, logFileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}

	logPath = path
	log.SetOutput(io.MultiWriter(os.Stderr, file))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Logging to %s", path)
	return nil
}

func Path() string {
	return logPath
}

func Open() error {
	if logPath == "" {
		return fmt.Errorf("log file is not initialized")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", logPath)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", logPath)
	default:
		cmd = exec.Command("xdg-open", logPath)
	}
	return cmd.Start()
}
