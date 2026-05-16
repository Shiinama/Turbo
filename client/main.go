package main

import (
	"client/platform/applog"
	"client/platform/autostart"
	"client/quic"
	"client/ui"
	_ "embed"
	"log"

	"github.com/getlantern/systray"
)

//go:embed assets/tray_icon.ico
var iconData []byte

func main() {
	if err := applog.Init(); err != nil {
		log.Println(err)
	}

	go quic.ConnectQuicServer()

	systray.Run(onReady, nil)
}

func onReady() {
	ui.SetupTray(iconData)

	if err := autostart.EnableAutoStart(); err != nil {
		log.Println(err)
	}
}
