package proxy

import (
	"math/rand"
	"sort"
	"sync"
)

var (
	// Lock-free reads with sync.Map
	countryClients sync.Map // 2-digit country code -> *CountryPool
	globalClients  sync.Map // "global" -> *CountryPool

	updateMutex sync.RWMutex
)

type CountryPool struct {
	clients           []*QuicClient
	cumulativeWeights []float64 // Pre-computed for O(log n) selection
	totalWeight       float64
}

func FindClientByCountry(countryCode string) *QuicClient {
	var pool interface{}
	var ok bool

	if countryCode == "global" {
		pool, ok = globalClients.Load(countryCode)
	} else {
		pool, ok = countryClients.Load(countryCode)
	}

	if ok {
		countryPool := pool.(*CountryPool)
		if client := selectFromPool(countryPool); client != nil {
			return client
		}
	}

	return nil
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

func selectFromPool(pool *CountryPool) *QuicClient {
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

	var globalPool CountryPool
	for _, client := range QuicClients {
		if client.isHealthy() {
			globalPool.add(client)
		}
	}
	globalClients.Store("global", &globalPool)

	countryMap := make(map[string]*CountryPool)
	for _, client := range QuicClients {
		if client.isHealthy() {
			country := client.Stats.CountryCode
			if country == "global" {
				continue
			}

			if _, exists := countryMap[country]; !exists {
				countryMap[country] = &CountryPool{}
			}
			countryMap[country].add(client)
		}
	}
	for country, pool := range countryMap {
		countryClients.Store(country, pool)
	}
}

func (p *CountryPool) add(client *QuicClient) {
	weight := client.Metrics.Score
	if weight < 1 {
		weight = 1
	}

	p.clients = append(p.clients, client)
	p.totalWeight += weight
	p.cumulativeWeights = append(p.cumulativeWeights, p.totalWeight)
}
