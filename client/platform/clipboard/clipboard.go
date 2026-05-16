package clipboard

import (
	"fmt"
	"os/exec"
	"runtime"
)

func WriteText(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			return fmt.Errorf("no clipboard command found")
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("opening clipboard stdin: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting clipboard command: %w", err)
	}
	if _, err := stdin.Write([]byte(text)); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("writing clipboard text: %w", err)
	}
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("closing clipboard stdin: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("copy command failed: %w", err)
	}
	return nil
}
