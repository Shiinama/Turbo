package proxy

import (
	"net"
	"strconv"
)

type Connection struct {
	ID            string
	Conn          net.Conn
	DataChan      chan []byte
	ProxyUserID   int64
	BytesSent     uint64
	BytesReceived uint64
}

var nextID int

func CreateConnection(conn net.Conn) *Connection {
	dataChan := make(chan []byte, 100)
	nextID++
	return &Connection{
		ID:       strconv.Itoa(nextID),
		Conn:     conn,
		DataChan: dataChan,
	}
}
