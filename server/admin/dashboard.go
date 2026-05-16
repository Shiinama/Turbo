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
	Nodes      []database.NodeRecord
	Users      []database.ProxyUser
	ProxyHost  string
	ProxyPort  string
	NodeURL    string
	CreateErr  string
	CreateDone string
}

func AdminNodesHandler(w http.ResponseWriter, r *http.Request) {
	if !checkBasicAuth(w, r) {
		return
	}

	createErr := ""
	createDone := ""
	if r.Method == http.MethodPost {
		if err := createProxyUser(r); err != nil {
			createErr = err.Error()
		} else {
			createDone = "proxy user created"
		}
	}

	nodes, err := database.ListNodes(100)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load nodes: %v", err), http.StatusInternalServerError)
		return
	}

	users, err := database.ListProxyUsers()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load proxy users: %v", err), http.StatusInternalServerError)
		return
	}

	err = templates.ExecuteTemplate(w, "admin_nodes.html", AdminNodesData{
		Nodes:      nodes,
		Users:      users,
		ProxyHost:  proxyHost(r),
		ProxyPort:  firstEnv("PROXY_PUBLIC_PORT", "SOCKS_PUBLIC_PORT", "SOCKS_PORT"),
		NodeURL:    os.Getenv("TURBO_NODE_URL"),
		CreateErr:  createErr,
		CreateDone: createDone,
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

func createProxyUser(r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	countryCode := r.FormValue("country_code")
	if countryCode == "" {
		countryCode = "global"
	}

	maxConns := 10
	if value := r.FormValue("max_conns"); value != "" {
		if _, err := fmt.Sscanf(value, "%d", &maxConns); err != nil {
			return fmt.Errorf("invalid max connections")
		}
	}

	return database.CreateProxyUser(database.ProxyUser{
		Username:    username,
		Password:    password,
		CountryCode: countryCode,
		MaxConns:    maxConns,
		IsActive:    r.FormValue("is_active") != "",
		Notes:       r.FormValue("notes"),
	})
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

func formatBytes(bytes uint64) string {
	const unit = 1000
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
