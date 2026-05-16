package ui

import (
	"log"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
)

func SetupTray(adminURL string, icon []byte) {
	systray.SetTemplateIcon(icon, icon)
	systray.SetTooltip("Turbo running")

	dashboard := systray.AddMenuItem("Nodes", "Open server nodes")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit", "Quit the whole app")

	go func() {
		for {
			select {
			case <-dashboard.ClickedCh:
				err := open(adminURL + "/admin/nodes")
				if err != nil {
					log.Println("Failed to open browser:", err)
				}
			case <-quitItem.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
