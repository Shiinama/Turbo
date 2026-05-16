package database

import (
	"fmt"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var DB *sqlx.DB

type NodeRecord struct {
	ID            string    `db:"id"`
	RemoteAddr    string    `db:"remote_addr"`
	Transport     string    `db:"transport"`
	OutboundBytes uint64    `db:"outbound_bytes"`
	ConnectedAt   time.Time `db:"connected_at"`
	LastSeenAt    time.Time `db:"last_seen_at"`
	IsActive      bool      `db:"is_active"`
	ProxyUser     *ProxyUser
	ProxyLink     string
}

type ProxyUser struct {
	ID         int64      `db:"id"`
	Username   string     `db:"username"`
	Password   string     `db:"password"`
	NodeID     string     `db:"node_id"`
	MaxConns   int        `db:"max_conns"`
	BytesSent  uint64     `db:"bytes_sent"`
	BytesRecv  uint64     `db:"bytes_received"`
	IsActive   bool       `db:"is_active"`
	Notes      string     `db:"notes"`
	CreatedAt  time.Time  `db:"created_at"`
	LastUsedAt *time.Time `db:"last_used_at"`
}

func InitPostgres() error {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	db, err := sqlx.Connect("postgres", connString)
	if err != nil {
		return err
	}

	DB = db
	return migrate()
}

func migrate() error {
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			remote_addr TEXT NOT NULL,
			transport TEXT NOT NULL,
			outbound_bytes BIGINT NOT NULL DEFAULT 0,
			connected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			is_active BOOLEAN NOT NULL DEFAULT true
		)
	`)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		ALTER TABLE nodes
		ADD COLUMN IF NOT EXISTS outbound_bytes BIGINT NOT NULL DEFAULT 0;

		ALTER TABLE nodes
		DROP COLUMN IF EXISTS active_conns,
		DROP COLUMN IF EXISTS bytes_sent,
		DROP COLUMN IF EXISTS bytes_received,
		DROP COLUMN IF EXISTS score,
		DROP COLUMN IF EXISTS latency,
		DROP COLUMN IF EXISTS country_code;
	`)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS proxy_users (
			id BIGSERIAL PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			max_conns INTEGER NOT NULL DEFAULT 10,
			bytes_sent BIGINT NOT NULL DEFAULT 0,
			bytes_received BIGINT NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT true,
			notes TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_used_at TIMESTAMPTZ
		)
	`)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		ALTER TABLE proxy_users
		ADD COLUMN IF NOT EXISTS bytes_sent BIGINT NOT NULL DEFAULT 0,
		ADD COLUMN IF NOT EXISTS bytes_received BIGINT NOT NULL DEFAULT 0,
		ADD COLUMN IF NOT EXISTS node_id TEXT NOT NULL DEFAULT '';

		ALTER TABLE proxy_users DROP COLUMN IF EXISTS country_code;
	`)
	return err
}

func UpsertNode(record NodeRecord) error {
	if DB == nil {
		return nil
	}

	_, err := DB.Exec(`
		INSERT INTO nodes (
			id, remote_addr, transport, outbound_bytes, connected_at,
			last_seen_at, is_active
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			remote_addr = EXCLUDED.remote_addr,
			transport = EXCLUDED.transport,
			outbound_bytes = EXCLUDED.outbound_bytes,
			last_seen_at = EXCLUDED.last_seen_at,
			is_active = EXCLUDED.is_active
	`, record.ID, record.RemoteAddr, record.Transport, record.OutboundBytes, record.ConnectedAt,
		record.LastSeenAt, record.IsActive)
	return err
}

func MarkNodeInactive(id string) error {
	if DB == nil {
		return nil
	}

	_, err := DB.Exec(`
		UPDATE nodes
		SET is_active = false, last_seen_at = now()
		WHERE id = $1
	`, id)
	return err
}

func ListNodes(limit int) ([]NodeRecord, error) {
	if DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var nodes []NodeRecord
	err := DB.Select(&nodes, `
		SELECT id, remote_addr, transport, outbound_bytes,
		       connected_at, last_seen_at, is_active
		FROM nodes
		ORDER BY last_seen_at DESC
		LIMIT $1
	`, limit)
	return nodes, err
}

func EnsureProxyUserForNode(node NodeRecord) error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	if node.ID == "" {
		return fmt.Errorf("node id is required")
	}

	nodeKey := stableNodeKey(node)
	username := "node-" + stableToken(nodeKey, 10)
	password := stableToken(nodeKey+":proxy-password", 18)
	notes := "auto-created for client node " + node.RemoteAddr

	_, err := DB.Exec(`
		INSERT INTO proxy_users (username, password, node_id, max_conns, is_active, notes)
		VALUES ($1, $2, $3, $4, true, $5)
		ON CONFLICT (username) DO UPDATE SET
			node_id = EXCLUDED.node_id,
			is_active = true,
			notes = EXCLUDED.notes
	`, username, password, node.ID, 10, notes)
	return err
}

func ListNodesWithProxyUsers(limit int) ([]NodeRecord, error) {
	nodes, err := ListNodes(limit)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nodes, nil
	}

	var users []ProxyUser
	err = DB.Select(&users, `
		SELECT id, username, password, node_id, max_conns, is_active,
		       bytes_sent, bytes_received, notes, created_at, last_used_at
		FROM proxy_users
		WHERE node_id != ''
	`)
	if err != nil {
		return nil, err
	}

	usersByNodeID := make(map[string]*ProxyUser, len(users))
	for i := range users {
		usersByNodeID[users[i].NodeID] = &users[i]
	}
	for i := range nodes {
		nodes[i].ProxyUser = usersByNodeID[nodes[i].ID]
	}
	return nodes, nil
}

func stableNodeKey(node NodeRecord) string {
	return node.ID
}

func stableToken(value string, length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz234567"
	if length <= 0 {
		length = 12
	}

	hash := uint64(1469598103934665603)
	for i := 0; i < len(value); i++ {
		hash ^= uint64(value[i])
		hash *= 1099511628211
	}

	out := make([]byte, length)
	for i := 0; i < length; i++ {
		hash ^= hash >> 12
		hash ^= hash << 25
		hash ^= hash >> 27
		out[i] = alphabet[(hash*2685821657736338717)%uint64(len(alphabet))]
	}
	return string(out)
}

func AuthenticateProxyUser(username, password string) (*ProxyUser, error) {
	if DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	var user ProxyUser
	err := DB.Get(&user, `
		SELECT id, username, password, node_id, max_conns, is_active,
		       bytes_sent, bytes_received, notes, created_at, last_used_at
		FROM proxy_users
		WHERE username = $1 AND password = $2 AND is_active = true
	`, username, password)
	if err != nil {
		return nil, err
	}

	_, _ = DB.Exec("UPDATE proxy_users SET last_used_at = now() WHERE id = $1", user.ID)
	return &user, nil
}

func AddProxyUserUsage(id int64, bytesSent uint64, bytesReceived uint64) error {
	if DB == nil || id == 0 {
		return nil
	}

	_, err := DB.Exec(`
		UPDATE proxy_users
		SET bytes_sent = bytes_sent + $2,
		    bytes_received = bytes_received + $3,
		    last_used_at = now()
		WHERE id = $1
	`, id, bytesSent, bytesReceived)
	return err
}
