package ui

import (
	"client/quic"
	"fmt"
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
