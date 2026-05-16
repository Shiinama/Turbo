package ui

import (
	"client/platform/applog"
	"client/platform/clipboard"
	"client/platform/update"
	"client/quic"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/getlantern/systray"
)

const refreshInterval = time.Minute

func SetupTray(icon []byte) {
	systray.SetTemplateIcon(icon, icon)
	systray.SetTooltip(tooltipText(0, false))

	statusItem := systray.AddMenuItem("Status: Reconnecting", "Turbo node connection status")
	statusItem.Disable()
	nodeIDItem := systray.AddMenuItem("Node ID: Loading", "Click to copy this node identity")
	trafficItem := systray.AddMenuItem("Traffic: 0 B", "Transferred traffic for this run")
	trafficItem.Disable()
	versionItem := systray.AddMenuItem("Version: "+update.CurrentVersion(), "Current Turbo version")
	versionItem.Disable()
	checkUpdateItem := systray.AddMenuItem("Check for Updates", "Check and install the latest Turbo version")
	openLogsItem := systray.AddMenuItem("Open Logs", "Open Turbo log file")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit", "Quit the whole app")
	nodeID := updateNodeIDItem(nodeIDItem)
	updateStatusItems(statusItem, trafficItem)
	go checkForUpdates(checkUpdateItem)

	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				updateStatusItems(statusItem, trafficItem)
			case <-nodeIDItem.ClickedCh:
				go copyNodeID(nodeIDItem, nodeID)
			case <-checkUpdateItem.ClickedCh:
				go checkForUpdates(checkUpdateItem)
			case <-openLogsItem.ClickedCh:
				go openLogs(openLogsItem)
			case <-quitItem.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func updateNodeIDItem(nodeIDItem *systray.MenuItem) string {
	nodeID, err := quic.NodeID()
	if err != nil {
		log.Println("Failed to load node ID:", err)
		nodeIDItem.SetTitle("Node ID: unavailable")
		nodeIDItem.Disable()
		return ""
	}
	nodeIDItem.SetTitle("Node ID: " + nodeID)
	nodeIDItem.Enable()
	return nodeID
}

func copyNodeID(item *systray.MenuItem, nodeID string) {
	if nodeID == "" {
		showTemporaryTitle(item, "Node ID unavailable", "Node ID: unavailable")
		return
	}
	if err := clipboard.WriteText(nodeID); err != nil {
		log.Printf("Failed to copy node ID: %v", err)
		showTemporaryTitle(item, "Copy Failed: "+shortError(err), "Node ID: "+nodeID)
		return
	}
	showTemporaryTitle(item, "Copied Node ID", "Node ID: "+nodeID)
}

func updateStatusItems(statusItem *systray.MenuItem, trafficItem *systray.MenuItem) {
	total := quic.TrafficSnapshot().TotalBytes
	formatted := formatBytes(total)
	connected := quic.IsConnected()

	status := "Reconnecting"
	if connected {
		status = "Connected"
	}
	statusItem.SetTitle("Status: " + status)
	trafficItem.SetTitle("Traffic: " + formatted)
	systray.SetTooltip(tooltipText(total, connected))
}

func tooltipText(total uint64, connected bool) string {
	status := "reconnecting"
	if connected {
		status = "connected"
	}
	return "Turbo node " + status + " - " + formatBytes(total) + " transferred"
}

func checkForUpdates(item *systray.MenuItem) {
	item.SetTitle("Checking for Updates...")
	item.Disable()

	result, hasUpdate, err := update.CheckLatestVersion()
	if err != nil {
		log.Println(err)
		showTemporaryTitle(item, "Update Failed: "+shortError(err), "Check for Updates")
		return
	}
	if !hasUpdate {
		showTemporaryTitle(item, "Up to Date ("+result.CurrentVersion+")", "Check for Updates")
		return
	}

	item.SetTitle("Installing " + result.LatestVersion + "...")
	result, err = update.CheckAndUpdate()
	if err != nil {
		log.Println(err)
		showTemporaryTitle(item, "Update Failed: "+shortError(err), "Check for Updates")
		return
	}
	if result.Updated {
		item.SetTitle("Updated to " + result.LatestVersion + " - Restarting...")
		return
	}
	showTemporaryTitle(item, "Up to Date ("+result.CurrentVersion+")", "Check for Updates")
}

func openLogs(item *systray.MenuItem) {
	if err := applog.Open(); err != nil {
		log.Printf("Failed to open logs: %v", err)
		showTemporaryTitle(item, "Open Logs Failed: "+shortError(err), "Open Logs")
		return
	}
	showTemporaryTitle(item, "Opened Logs", "Open Logs")
}

func showTemporaryTitle(item *systray.MenuItem, title string, restoreTitle string) {
	item.SetTitle(title)
	time.Sleep(5 * time.Second)
	item.SetTitle(restoreTitle)
	item.Enable()
}

func shortError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) <= 64 {
		return msg
	}
	return msg[:61] + "..."
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
