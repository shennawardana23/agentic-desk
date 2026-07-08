package config

import (
	"net/url"
	"os/user"
	"strconv"
	"time"
)

// DatabaseConfig holds discrete Postgres connection settings — split
// fields instead of one DATABASE_URL string, matching the shape (field
// names, DB_* env var convention, pool-tuning defaults) of a reference
// implementation the user pointed at (archpublicwebsite-mcp's
// cmd/config.DatabaseConfig). The driver stays pgx (not that reference's
// lib/pq) — this repo's pgvector support requires pgx's own connection
// codec, and DSN() below encodes pool tuning as pgxpool.ParseConfig's
// documented pool_* query parameters rather than switching drivers.
type DatabaseConfig struct {
	Host            string
	Port            string
	Username        string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DSN builds a pgx-compatible connection string from d, encoding pool
// tuning as pgxpool.ParseConfig's pool_max_conns/pool_min_conns/
// pool_max_conn_lifetime/pool_max_conn_idle_time query parameters.
func (d DatabaseConfig) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		Host:   d.Host + ":" + d.Port,
		Path:   "/" + d.Name,
	}
	if d.Username != "" {
		if d.Password != "" {
			u.User = url.UserPassword(d.Username, d.Password)
		} else {
			u.User = url.User(d.Username)
		}
	}

	q := u.Query()
	q.Set("sslmode", d.SSLMode)
	q.Set("pool_max_conns", strconv.Itoa(d.MaxOpenConns))
	q.Set("pool_min_conns", strconv.Itoa(d.MaxIdleConns))
	q.Set("pool_max_conn_lifetime", d.ConnMaxLifetime.String())
	q.Set("pool_max_conn_idle_time", d.ConnMaxIdleTime.String())
	u.RawQuery = q.Encode()

	return u.String()
}

// defaultDBUsername mirrors the Makefile's own default
// (postgres://$(whoami)@localhost:5432/agentic_desk) for local
// trust-auth Postgres, where the role name matches the OS user.
func defaultDBUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return ""
}
