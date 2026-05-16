package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"server/database"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	// Addr also contains port of the target website
	Addr string `json:"addr,omitempty"`
	Data string `json:"data,omitempty"`
}

var (
	QuicClients  = make(map[string]*QuicClient)
	QuicMutex    sync.RWMutex
	nodeListener net.Listener
	upgrader     = websocket.Upgrader{}
)

// QuicClient represents a connected QUIC client
type QuicClient struct {
	ID         string
	conn       net.Conn
	wsConn     *websocket.Conn
	transport  string
	mutex      sync.Mutex
	userConns  map[string]*Connection
	userMutex  sync.Mutex
	lastPing   time.Time
	lastPingID string
	Metrics    *Metrics
	Stats      *ClientStats
	kicked     atomic.Bool
}

// StartNodeServer initializes the TCP server used by node clients.
func StartNodeServer(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start node server: %w", err)
	}

	nodeListener = listener
	log.Printf("Node TCP server listening on %s", addr)

	go acceptNodeConnections(nodeListener)

	go ReportPing()

	return nil
}

func acceptNodeConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Node accept error: %v", err)
			continue
		}

		go handleNodeConnection(conn)
	}
}

func handleNodeConnection(conn net.Conn) {
	clientID := conn.RemoteAddr().String()
	log.Printf("New node client connected: %s", clientID)

	client := &QuicClient{
		ID:        clientID,
		conn:      conn,
		transport: "tcp",
		userConns: make(map[string]*Connection),
		lastPing:  time.Now(),
		Metrics: &Metrics{
			Reliability: 0.7,
			Score:       50,
		},
		Stats: &ClientStats{
			ConnectTime: time.Now(),
		},
	}

	QuicMutex.Lock()
	QuicClients[clientID] = client
	QuicMutex.Unlock()

	go quicReader(client)

	updatePools()
	client.Save()
}

func HandleNodeWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Node websocket upgrade failed: %v", err)
		return
	}

	clientID := forwardedRemoteAddr(r)
	log.Printf("New websocket node client connected: %s", clientID)

	client := &QuicClient{
		ID:        clientID,
		wsConn:    conn,
		transport: "websocket",
		userConns: make(map[string]*Connection),
		lastPing:  time.Now(),
		Metrics: &Metrics{
			Reliability: 0.7,
			Score:       50,
		},
		Stats: &ClientStats{
			ConnectTime: time.Now(),
		},
	}

	QuicMutex.Lock()
	QuicClients[clientID] = client
	QuicMutex.Unlock()

	go quicReader(client)

	updatePools()
	client.Save()
}

func forwardedRemoteAddr(r *http.Request) string {
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor == "" {
		return r.RemoteAddr
	}

	ip := strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
	if ip == "" {
		return r.RemoteAddr
	}

	_, port, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || port == "" {
		return ip
	}
	return net.JoinHostPort(ip, port)
}

func quicReader(client *QuicClient) {
	defer func() {
		QuicMutex.Lock()
		delete(QuicClients, client.ID)
		log.Printf("Node client disconnected: %s. Remaining clients: %d", client.ID, len(QuicClients))
		QuicMutex.Unlock()

		if err := database.MarkNodeInactive(client.ID); err != nil {
			log.Printf("Failed to mark node inactive %s: %v", client.ID, err)
		}
		client.closeTransport()
	}()

	for {
		var msg Message
		if err := client.readMessage(&msg); err != nil {
			if client.kicked.Load() {
				return
			}
			log.Printf("Node read error for client %s: %v", client.ID, err)
			return
		}

		switch msg.Type {
		case "connected":
			client.userMutex.Lock()
			if sc, ok := client.userConns[msg.ID]; ok {
				select {
				case sc.DataChan <- []byte{}:
				default:
				}
			}
			client.userMutex.Unlock()
		case "data":
			client.userMutex.Lock()
			if sc, ok := client.userConns[msg.ID]; ok {
				if data, err := base64.StdEncoding.DecodeString(msg.Data); err == nil {
					sc.DataChan <- data
				} else {
					log.Println("WARN: Suspicious data received from client", client.ID)
				}
			}
			client.userMutex.Unlock()
		case "close":
			client.userMutex.Lock()
			if sc, ok := client.userConns[msg.ID]; ok {
				sc.Conn.Close()
				delete(client.userConns, msg.ID)
			}
			client.userMutex.Unlock()
		case "pong":
			client.Pong()
		}
	}
}

func (c *QuicClient) SendMessage(msg Message) error {
	if c == nil {
		return fmt.Errorf("client is nil")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n') // Add newline for JSON decoder

	if c.wsConn != nil {
		return c.wsConn.WriteMessage(websocket.TextMessage, data)
	}
	if c.conn != nil {
		_, err = c.conn.Write(data)
		return err
	}
	return fmt.Errorf("client has no active transport")
}

func (c *QuicClient) Kick(reason string) {
	if !c.kicked.CompareAndSwap(false, true) {
		return // Already kicked
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.closeTransport()

	for id, sc := range c.userConns {
		sc.Conn.Close()
		delete(c.userConns, id)
	}

	QuicMutex.Lock()
	delete(QuicClients, c.ID)
	QuicMutex.Unlock()

	updatePools() // TODO: Inefficient, optimize client erasure

	log.Printf("Kicked node client %s for \"%s\"", c.ID, reason)
}

func (c *QuicClient) readMessage(msg *Message) error {
	if c.wsConn != nil {
		_, data, err := c.wsConn.ReadMessage()
		if err != nil {
			return err
		}
		return json.Unmarshal(data, msg)
	}
	if c.conn != nil {
		return json.NewDecoder(c.conn).Decode(msg)
	}
	return fmt.Errorf("client has no active transport")
}

func (c *QuicClient) closeTransport() {
	if c.wsConn != nil {
		c.wsConn.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *QuicClient) Save() {
	if c == nil || c.Stats == nil || c.Metrics == nil {
		return
	}

	record := database.NodeRecord{
		ID:            c.ID,
		RemoteAddr:    c.ID,
		Transport:     c.transport,
		ActiveConns:   atomic.LoadInt32(&c.Stats.ActiveConns),
		BytesSent:     atomic.LoadUint64(&c.Stats.BytesSent),
		BytesReceived: atomic.LoadUint64(&c.Stats.BytesReceived),
		Score:         c.Metrics.Score,
		Latency:       c.Metrics.Latency,
		ConnectedAt:   c.Stats.ConnectTime,
		LastSeenAt:    time.Now(),
		IsActive:      true,
	}

	err := database.UpsertNode(record)
	if err != nil {
		log.Printf("Failed to save node %s: %v", c.ID, err)
	}

	if err = database.EnsureProxyUserForNode(record); err != nil {
		log.Printf("Failed to ensure proxy user for node %s: %v", c.ID, err)
	}
}
