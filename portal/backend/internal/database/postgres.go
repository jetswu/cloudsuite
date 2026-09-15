// Package database wires the portal PostgreSQL connection pool and the
// goose migration runner. The backend was previously stateless; Sprint 1.3
// introduces a `portal` database to persist domains and their DNS records.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds the PostgreSQL connection settings sourced from the
// PORTAL_DB_* environment variables.
type Config struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

// DSN builds a pgx connection string from the config. It never logs the
// password; callers must keep the returned string out of log output.
func (c Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.User, c.Password, c.Host, c.Port, c.Name,
	)
}

// Connect opens a pgx connection pool, verifies connectivity with a short
// ping, and returns a ready-to-use pool. The caller owns closing it.
func Connect(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse portal db dsn: %w", err)
	}
	poolCfg.MaxConns = 8
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create portal db pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping portal db: %w", err)
	}
	return pool, nil
}
