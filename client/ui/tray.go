package ui

import (
	"client/platform/update"
	"client/quic"
	"fmt"
	"log"
	"time"

	"github.com/getlantern/systray"
)

const refreshInterval = time.Minute

func SetupTray(icon []byte) {
	systray.SetTemplateIcon(icon, icon)
	systray.SetTooltip(tooltipText(0))

	statusItem := systray.AddMenuItem("Status: Running", "Turbo node is running")
	statusItem.Disable()
	trafficItem := systray.AddMenuItem("Traffic: 0 B", "Transferred traffic for this run")
	trafficItem.Disable()
	versionItem := systray.AddMenuItem("Version: "+update.CurrentVersion(), "Current Turbo version")
	versionItem.Disable()
	checkUpdateItem := systray.AddMenuItem("Check for Updates", "Check and install the latest Turbo version")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit", "Quit the whole app")
	updateTrafficItem(trafficItem)

	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				updateTrafficItem(trafficItem)
			case <-checkUpdateItem.ClickedCh:
				go checkForUpdates(checkUpdateItem)
			case <-quitItem.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func updateTrafficItem(item *systray.MenuItem) {
	total := quic.TrafficSnapshot().TotalBytes
	formatted := formatBytes(total)
	item.SetTitle("Traffic: " + formatted)
	systray.SetTooltip(tooltipText(total))
}

func tooltipText(total uint64) string {
	return "Turbo node running - " + formatBytes(total) + " transferred"
}

func checkForUpdates(item *systray.MenuItem) {
	item.SetTitle("Checking for Updates...")
	item.Disable()

	result, hasUpdate, err := update.CheckLatestVersion()
	if err != nil {
		log.Println(err)
		showTemporaryTitle(item, "Update Check Failed")
		return
	}
	if !hasUpdate {
		showTemporaryTitle(item, "Up to Date")
		return
	}

	item.SetTitle("Installing " + result.LatestVersion + "...")
	result, err = update.CheckAndUpdate()
	if err != nil {
		log.Println(err)
		showTemporaryTitle(item, "Update Failed")
		return
	}
	if result.Updated {
		showTemporaryTitle(item, "Update Installed")
		return
	}
	showTemporaryTitle(item, "Up to Date")
}

func showTemporaryTitle(item *systray.MenuItem, title string) {
	item.SetTitle(title)
	time.Sleep(5 * time.Second)
	item.SetTitle("Check for Updates")
	item.Enable()
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	value := float64(bytes)
	for _, suffix := range []string{"KB", "MB", "GB"} {
		value /= unit
		if value < unit || suffix == "GB" {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f GB", value)
}
