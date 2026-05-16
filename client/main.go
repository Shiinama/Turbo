package main

import (
	"client/platform/autostart"
	"client/platform/update"
	"client/quic"
	"client/ui"
	_ "embed"
	"log"
	"os"

	"github.com/getlantern/systray"
)

//go:embed assets/tray_icon.ico
var iconData []byte

const (
	defaultAdminURL = "https://turbo-server-production-1e29.up.railway.app"
)

func main() {
	go quic.ConnectQuicServer()

	systray.Run(onReady, nil)
}

func onReady() {
	ui.SetupTray(getAdminURL(), iconData)

	if err := autostart.EnableAutoStart(); err != nil {
		log.Println(err)
	}

	if err := update.AutoUpdate(); err != nil {
		log.Println(err)
	}
}

func getAdminURL() string {
	if url := os.Getenv("TURBO_ADMIN_URL"); url != "" {
		return url
	}
	return defaultAdminURL
}
