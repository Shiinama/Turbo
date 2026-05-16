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
	CountryCode   string    `db:"country_code"`
	ActiveConns   int32     `db:"active_conns"`
	BytesSent     uint64    `db:"bytes_sent"`
	BytesReceived uint64    `db:"bytes_received"`
	Score         float64   `db:"score"`
	Latency       float64   `db:"latency"`
	ConnectedAt   time.Time `db:"connected_at"`
	LastSeenAt    time.Time `db:"last_seen_at"`
	IsActive      bool      `db:"is_active"`
}

type ProxyUser struct {
	ID          int64      `db:"id"`
	Username    string     `db:"username"`
	Password    string     `db:"password"`
	CountryCode string     `db:"country_code"`
	MaxConns    int        `db:"max_conns"`
	BytesSent   uint64     `db:"bytes_sent"`
	BytesRecv   uint64     `db:"bytes_received"`
	IsActive    bool       `db:"is_active"`
	Notes       string     `db:"notes"`
	CreatedAt   time.Time  `db:"created_at"`
	LastUsedAt  *time.Time `db:"last_used_at"`
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
			country_code TEXT NOT NULL DEFAULT 'global',
			active_conns INTEGER NOT NULL DEFAULT 0,
			bytes_sent BIGINT NOT NULL DEFAULT 0,
			bytes_received BIGINT NOT NULL DEFAULT 0,
			score DOUBLE PRECISION NOT NULL DEFAULT 0,
			latency DOUBLE PRECISION NOT NULL DEFAULT 0,
			connected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			is_active BOOLEAN NOT NULL DEFAULT true
		)
	`)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS proxy_users (
			id BIGSERIAL PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			country_code TEXT NOT NULL DEFAULT 'global',
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
		ADD COLUMN IF NOT EXISTS bytes_received BIGINT NOT NULL DEFAULT 0
	`)
	return err
}

func UpsertNode(record NodeRecord) error {
	if DB == nil {
		return nil
	}

	_, err := DB.Exec(`
		INSERT INTO nodes (
			id, remote_addr, transport, country_code, active_conns,
			bytes_sent, bytes_received, score, latency, connected_at,
			last_seen_at, is_active
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			remote_addr = EXCLUDED.remote_addr,
			transport = EXCLUDED.transport,
			country_code = EXCLUDED.country_code,
			active_conns = EXCLUDED.active_conns,
			bytes_sent = EXCLUDED.bytes_sent,
			bytes_received = EXCLUDED.bytes_received,
			score = EXCLUDED.score,
			latency = EXCLUDED.latency,
			last_seen_at = EXCLUDED.last_seen_at,
			is_active = EXCLUDED.is_active
	`, record.ID, record.RemoteAddr, record.Transport, record.CountryCode, record.ActiveConns,
		record.BytesSent, record.BytesReceived, record.Score, record.Latency, record.ConnectedAt,
		record.LastSeenAt, record.IsActive)
	return err
}

func MarkNodeInactive(id string) error {
	if DB == nil {
		return nil
	}

	_, err := DB.Exec(`
		UPDATE nodes
		SET is_active = false, active_conns = 0, last_seen_at = now()
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
		SELECT id, remote_addr, transport, country_code, active_conns,
		       bytes_sent, bytes_received, score, latency, connected_at,
		       last_seen_at, is_active
		FROM nodes
		ORDER BY last_seen_at DESC
		LIMIT $1
	`, limit)
	return nodes, err
}

func CreateProxyUser(user ProxyUser) error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	if user.CountryCode == "" {
		user.CountryCode = "global"
	}
	if user.MaxConns <= 0 {
		user.MaxConns = 10
	}

	_, err := DB.Exec(`
		INSERT INTO proxy_users (username, password, country_code, max_conns, is_active, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, user.Username, user.Password, user.CountryCode, user.MaxConns, user.IsActive, user.Notes)
	return err
}

func ListProxyUsers() ([]ProxyUser, error) {
	if DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	var users []ProxyUser
	err := DB.Select(&users, `
		SELECT id, username, password, country_code, max_conns, is_active,
		       bytes_sent, bytes_received, notes, created_at, last_used_at
		FROM proxy_users
		ORDER BY created_at DESC
	`)
	return users, err
}

func AuthenticateProxyUser(username, password string) (*ProxyUser, error) {
	if DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	var user ProxyUser
	err := DB.Get(&user, `
		SELECT id, username, password, country_code, max_conns, is_active,
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
