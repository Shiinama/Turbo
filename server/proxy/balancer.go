package proxy

import (
	"math/rand"
)

func FindClient() *QuicClient {
	QuicMutex.RLock()
	defer QuicMutex.RUnlock()

	healthy := make([]*QuicClient, 0, len(QuicClients))
	for _, client := range QuicClients {
		if client.isHealthy() {
			healthy = append(healthy, client)
		}
	}
	if len(healthy) == 0 {
		return nil
	}
	return healthy[rand.Intn(len(healthy))]
}

func FindClientByID(id string) *QuicClient {
	if id == "" {
		return nil
	}

	QuicMutex.RLock()
	client := QuicClients[id]
	QuicMutex.RUnlock()

	if client != nil && client.isHealthy() {
		return client
	}
	return nil
}

func (c *QuicClient) isHealthy() bool {
	return c != nil && (c.conn != nil || c.wsConn != nil)
}
