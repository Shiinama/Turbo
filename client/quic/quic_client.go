package quic

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Addr string `json:"addr,omitempty"`
	Data string `json:"data,omitempty"`
}

type Connection struct {
	conn     net.Conn
	dataChan chan []byte
}

var (
	quicMutex   sync.Mutex
	nodeConn    net.Conn
	wsConn      *websocket.Conn
	clientConns = make(map[string]*Connection)
	clientMutex sync.Mutex
)

/* On disconnect:
Waits for 5 seconds 2 times
Then waits for 5 minutes forever
*/

func ConnectQuicServer() {
	connectionAttempts := 0
	retryDelay := time.Second * 4
	serverURL := getServerURL()

	for {
		transport, err := dialNodeServer(serverURL)
		if err != nil {
			if connectionAttempts == 2 {
				retryDelay = time.Minute * 5
			}

			log.Printf("Failed to connect to node server at %s. Retrying...", serverURL)
			log.Println(err)
			time.Sleep(retryDelay)
			connectionAttempts++
			continue
		}
		log.Printf("Connected to node server at %s", serverURL)

		quicMutex.Lock()
		nodeConn = transport.conn
		wsConn = transport.wsConn
		quicMutex.Unlock()
		connectionAttempts = 0

		quicReader()

		log.Println("Node connection closed, reconnecting...")

		time.Sleep(time.Second * 5)
	}
}

func quicReader() {
	for {
		var msg Message
		if err := readMessage(&msg); err != nil {
			log.Println("Node read error:", err)
			clientMutex.Lock()
			for id, cc := range clientConns {
				cc.conn.Close()
				close(cc.dataChan)
				delete(clientConns, id)
			}
			clientMutex.Unlock()

			return
		}

		log.Printf("received %+v", msg.Type)

		switch msg.Type {
		case "connect":
			log.Println("to-to ", msg.Addr)
			go handleConnect(msg)
		case "data":
			clientMutex.Lock()
			if cc, ok := clientConns[msg.ID]; ok {
				if data, err := base64.StdEncoding.DecodeString(msg.Data); err == nil {
					cc.dataChan <- data
				}
			}
			clientMutex.Unlock()
		case "close":
			clientMutex.Lock()
			if cc, ok := clientConns[msg.ID]; ok {
				cc.conn.Close()
				close(cc.dataChan)
				delete(clientConns, msg.ID)
			}
			clientMutex.Unlock()
		case "ping":
			err := SendMessage(&Message{
				Type: "pong",
				ID:   msg.ID,
			})
			if err != nil {
				log.Fatal("error sending pong:", err)
			}
		}
	}
}

func SendMessage(msg *Message) error {
	quicMutex.Lock()
	defer quicMutex.Unlock()

	if nodeConn == nil && wsConn == nil {
		log.Println("Cannot send message: no active node connection")
		return fmt.Errorf("no active node connection")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message of type %s: %v", msg.Type, err)
		return err
	}
	data = append(data, '\n')

	if wsConn != nil {
		if err := wsConn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error writing to node websocket: %v", err)
			return err
		}
		return nil
	}

	if nodeConn != nil {
		_, err = nodeConn.Write(data)
		if err != nil {
			log.Printf("Error writing to node connection: %v", err)
			return err
		}
		return nil
	}

	return fmt.Errorf("no active node connection")
}

func sendCloseMessage(id string) {
	msg := Message{Type: "close", ID: id}
	SendMessage(&msg)
	clientMutex.Lock()
	if cc, ok := clientConns[id]; ok {
		cc.conn.Close()
		close(cc.dataChan)
		delete(clientConns, id)
	}
	clientMutex.Unlock()
}

func getServerURL() string {
	if url := os.Getenv("TURBO_SERVER_URL"); url != "" {
		return url
	}
	if addr := os.Getenv("TURBO_SERVER_ADDR"); addr != "" {
		return "tcp://" + addr
	}
	return "tcp://127.0.0.1:8443"
}

type nodeTransport struct {
	conn   net.Conn
	wsConn *websocket.Conn
}

func dialNodeServer(serverURL string) (*nodeTransport, error) {
	if strings.HasPrefix(serverURL, "ws://") || strings.HasPrefix(serverURL, "wss://") {
		conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
		if err != nil {
			return nil, err
		}
		return &nodeTransport{wsConn: conn}, nil
	}

	addr := strings.TrimPrefix(serverURL, "tcp://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &nodeTransport{conn: conn}, nil
}

func readMessage(msg *Message) error {
	quicMutex.Lock()
	currentWS := wsConn
	currentConn := nodeConn
	quicMutex.Unlock()

	if currentWS != nil {
		_, data, err := currentWS.ReadMessage()
		if err != nil {
			return err
		}
		return json.Unmarshal(data, msg)
	}

	if currentConn != nil {
		return json.NewDecoder(currentConn).Decode(msg)
	}
	return fmt.Errorf("no active node connection")
}
