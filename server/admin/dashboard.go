package admin

import (
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"server/database"
)

var templates, _ = template.ParseFiles(
	"./admin/templates/admin_nodes.html",
)

type AdminNodesData struct {
	Nodes     []database.NodeRecord
	ProxyHost string
	ProxyPort string
	NodeURL   string
}

func AdminNodesHandler(w http.ResponseWriter, r *http.Request) {
	if !checkBasicAuth(w, r) {
		return
	}

	nodes, err := database.ListNodesWithProxyUsers(100)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load nodes: %v", err), http.StatusInternalServerError)
		return
	}

	proxyHost := proxyHost(r)
	proxyPort := firstEnv("PROXY_PUBLIC_PORT", "SOCKS_PUBLIC_PORT", "SOCKS_PORT")
	for i := range nodes {
		if nodes[i].ProxyUser != nil {
			nodes[i].ProxyLink = proxyLink(proxyHost, proxyPort, *nodes[i].ProxyUser)
		}
	}

	err = templates.ExecuteTemplate(w, "admin_nodes.html", AdminNodesData{
		Nodes:     nodes,
		ProxyHost: proxyHost,
		ProxyPort: proxyPort,
		NodeURL:   os.Getenv("TURBO_NODE_URL"),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal Server Error: %v", err), http.StatusInternalServerError)
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func proxyHost(r *http.Request) string {
	if host := firstEnv("PROXY_PUBLIC_HOST", "SOCKS_PUBLIC_HOST"); host != "" {
		return host
	}

	host, _, err := net.SplitHostPort(r.Host)
	if err == nil {
		return host
	}
	return r.Host
}

func proxyLink(host string, port string, user database.ProxyUser) string {
	if host == "" || port == "" {
		return ""
	}
	return fmt.Sprintf("socks5://%s:%s@%s:%s", user.Username, user.Password, host, port)
}

func checkBasicAuth(w http.ResponseWriter, r *http.Request) bool {
	username, password, ok := r.BasicAuth()
	if ok && username == os.Getenv("ADMIN_USERNAME") && password == os.Getenv("ADMIN_PASSWORD") {
		return true
	}

	w.Header().Set("WWW-Authenticate", `Basic realm="Turbo Admin"`)
	http.Error(w, "admin authentication required", http.StatusUnauthorized)
	return false
}
