package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"server/admin"
	"server/database"
	"server/proxy"
)

func main() {
	if err := database.InitPostgres(); err != nil {
		log.Fatal("Failed to initialize Postgres:", err)
	}

	http.HandleFunc("/admin/nodes", admin.AdminNodesHandler)
	http.HandleFunc("/node", proxy.HandleNodeWebSocket)
	go func() {
		httpPort := getEnv("HTTP_PORT", "8080")
		log.Println("Starting admin/node HTTP endpoint on :" + httpPort)
		if err := http.ListenAndServe(":"+httpPort, nil); err != nil {
			log.Fatal("Failed to start admin/node HTTP endpoint:", err)
		}
	}()

	nodePort := getEnv("NODE_PORT", "8443")
	log.Println("Starting node TCP server on :" + nodePort)
	err := proxy.StartNodeServer(":" + nodePort)
	if err != nil {
		log.Fatal("Failed to start node TCP server:", err)
	}

	socksPort := getEnv("SOCKS_PORT", "1080")
	log.Println("Starting SOCKS5 receiver on :" + socksPort)
	listener, err := net.Listen("tcp", ":"+socksPort)
	if err != nil {
		log.Fatal("Failed to start SOCKS5 receiver:", err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Couldn't accept SOCKS5 connection: %v", err)
				continue
			}
			go proxy.HandleSocksConn(conn)
		}
	}()

	select {}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
