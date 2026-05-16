package proxy

import (
	"math/rand"
	"sort"
	"sync"
)

var (
	globalClientPool ClientPool
	updateMutex sync.RWMutex
)

type ClientPool struct {
	clients           []*QuicClient
	cumulativeWeights []float64 // Pre-computed for O(log n) selection
	totalWeight       float64
}

func FindClient() *QuicClient {
	updateMutex.RLock()
	defer updateMutex.RUnlock()
	return selectFromPool(&globalClientPool)
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

func selectFromPool(pool *ClientPool) *QuicClient {
	if pool.totalWeight == 0 || len(pool.clients) == 0 {
		return nil
	}

	// Try up to 3 times to find a healthy client
	for attempts := 0; attempts < 3; attempts++ {
		randomPoint := rand.Float64() * pool.totalWeight
		idx := sort.SearchFloat64s(pool.cumulativeWeights, randomPoint)
		if idx >= len(pool.clients) {
			idx = len(pool.clients) - 1
		}

		client := pool.clients[idx]

		if client.isHealthy() {
			return client
		}
	}

	return nil // All attempts failed
}

func (c *QuicClient) isHealthy() bool {
	return c != nil && (c.conn != nil || c.wsConn != nil)
}

func updatePools() {
	updateMutex.Lock()
	defer updateMutex.Unlock()

	var globalPool ClientPool
	for _, client := range QuicClients {
		if client.isHealthy() {
			globalPool.add(client)
		}
	}
	globalClientPool = globalPool
}

func (p *ClientPool) add(client *QuicClient) {
	weight := client.Metrics.Score
	if weight < 1 {
		weight = 1
	}

	p.clients = append(p.clients, client)
	p.totalWeight += weight
	p.cumulativeWeights = append(p.cumulativeWeights, p.totalWeight)
}
