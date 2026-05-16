package proxy

import (
	"math/rand"
	"time"
)

func ReportPing() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		QuicMutex.RLock()
		clientSnapshot := make([]*QuicClient, 0, len(QuicClients))
		for _, client := range QuicClients {
			clientSnapshot = append(clientSnapshot, client)
		}
		QuicMutex.RUnlock()

		for _, client := range clientSnapshot {
			if client.kicked.Load() {
				continue
			}

			client.lastPing = time.Now()
			if client.lastPingID != "" {
				client.Kick("ping timeout")
				continue
			}

			pingID := string(rune(rand.Int()))

			err := client.SendMessage(Message{
				Type: "ping",
				ID:   pingID,
			})
			if err != nil {
				client.Kick("ping send error")
				continue
			}
			client.lastPingID = pingID
		}
	}
}
func (c *QuicClient) Pong() {
	c.lastPingID = ""
	c.Save()
}
