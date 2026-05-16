package proxy

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"server/database"
	"server/proxy/socks"
	"sync/atomic"
	"time"
)

var (
	connectTimeout = 5 * time.Second
)

type ClientStats struct {
	ConnectTime   time.Time
	OutboundBytes uint64
}

func HandleSocksConn(conn net.Conn) {
	defer conn.Close()

	host, port, auth, err := socks.HandleSocksHandshake(conn)
	nodeID := ""
	if auth != nil {
		nodeID = auth.NodeID
	}

	if err != nil {
		log.Printf("SOCKS handshake failed for %s, %v", conn.RemoteAddr(), err)
		return
	}

	var client *QuicClient

	pc := CreateConnection(conn)
	if auth != nil {
		pc.ProxyUserID = auth.UserID
	}

	_, err = conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}) // success
	if err != nil {
		log.Printf("Failed to send SOCKS success response to %s: %v", conn.RemoteAddr(), err)
		return
	}

	// Premake connect message
	buffer := make([]byte, 32*1024)
	var connData string
	n, err := pc.Conn.Read(buffer)
	if err != nil {
		return
	}
	if n > 0 {
		connData = base64.StdEncoding.EncodeToString(buffer[:n])
		pc.BytesSent += uint64(n)
	}
	msg := Message{Type: "connect", ID: pc.ID, Addr: fmt.Sprintf("%s:%d", host, port), Data: connData}

	success := false
	attempts := 0

	for !success && attempts < 3 {
		attempts++
		client = findClientForProxyUser(nodeID)
		if client == nil {
			log.Println("No available clients found for this request")
			return
		}

		client.userMutex.Lock()
		client.userConns[pc.ID] = pc
		client.userMutex.Unlock()

		err = client.SendMessage(msg)
		if err != nil {
			log.Println("WriteJSON error:", err)
			client.userMutex.Lock()
			delete(client.userConns, pc.ID)
			client.userMutex.Unlock()
			continue
		}

		select {
		case <-pc.DataChan:
			success = true
		case <-time.After(connectTimeout):
			log.Printf("Connection timeout for client %s, retrying with another client", client.ID)
			client.userMutex.Lock()
			delete(client.userConns, pc.ID)
			client.userMutex.Unlock()
			continue
		}

		if success {
			go relayFromSocksToQuic(client, pc)
			if initialData := <-pc.DataChan; len(initialData) > 0 {
				_, err := pc.Conn.Write(initialData)
				if err != nil {
					return
				}
			}
			relayFromChanToSocks(client, pc)
			return
		}
	}

	conn.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
}

func findClientForProxyUser(nodeID string) *QuicClient {
	if nodeID != "" {
		return FindClientByID(nodeID)
	}
	return FindClient()
}

func relayFromSocksToQuic(client *QuicClient, pc *Connection) {
	buf := make([]byte, 4096)
	for {
		n, err := pc.Conn.Read(buf)
		if err != nil {
			client.SendCloseMessage(pc.ID)
			return
		}

		dataSize := uint64(n)
		pc.BytesSent += dataSize

		data := base64.StdEncoding.EncodeToString(buf[:n])
		msg := Message{Type: "data", ID: pc.ID, Data: data}
		if err := client.SendMessage(msg); err != nil {
			return
		}
	}
}

func relayFromChanToSocks(client *QuicClient, pc *Connection) {
	for data := range pc.DataChan {
		n, err := pc.Conn.Write(data)
		outboundBytes := uint64(n)
		atomic.AddUint64(&client.Stats.OutboundBytes, outboundBytes)
		pc.BytesReceived += outboundBytes
		client.Save()
		if err != nil {
			//client.SendCloseMessage(pc.ID)
			return
		}
	}
}

func (c *QuicClient) SendCloseMessage(id string) {
	msg := Message{Type: "close", ID: id}
	c.SendMessage(msg)

	c.userMutex.Lock()
	sc := c.userConns[id]
	delete(c.userConns, id)
	c.userMutex.Unlock()

	if sc != nil {
		if err := database.AddProxyUserUsage(sc.ProxyUserID, sc.BytesSent, sc.BytesReceived); err != nil {
			log.Printf("failed to update proxy user usage: %v", err)
		}
		c.Save()
		sc.Conn.Close()
	} else {
		println("é- double closing")
	}

}
