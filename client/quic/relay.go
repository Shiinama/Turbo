package quic

import (
	"encoding/base64"
	"sync/atomic"
)

var (
	uploadedBytes   atomic.Uint64
	downloadedBytes atomic.Uint64
)

type TrafficStats struct {
	UploadedBytes   uint64
	DownloadedBytes uint64
	TotalBytes      uint64
}

func TrafficSnapshot() TrafficStats {
	uploaded := uploadedBytes.Load()
	downloaded := downloadedBytes.Load()
	return TrafficStats{
		UploadedBytes:   uploaded,
		DownloadedBytes: downloaded,
		TotalBytes:      uploaded + downloaded,
	}
}

func relayFromConnToQuic(cc *Connection, id string) {
	buf := make([]byte, 4096)
	for {
		n, err := cc.conn.Read(buf)
		if err != nil {
			sendCloseMessage(id)
			return
		}
		uploadedBytes.Add(uint64(n))
		data := base64.StdEncoding.EncodeToString(buf[:n])
		msg := Message{Type: "data", ID: id, Data: data}
		SendMessage(&msg)
	}
}

func relayFromChanToConn(cc *Connection, id string) {
	for data := range cc.dataChan {
		n, err := cc.conn.Write(data)
		downloadedBytes.Add(uint64(n))
		if err != nil {
			sendCloseMessage(id)
			return
		}
	}
}
