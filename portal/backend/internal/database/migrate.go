// Package database also embeds the goose migrations so they ship inside the
// single backend binary; no external migration files are needed at runtime.
package database

import (
	"context"
	"embed"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // register "pgx" database/sql driver for goose
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies all pending goose migrations against the pool using the
// embedded SQL files. It is idempotent: on a fresh database it creates the
// goose schema history table and applies 00001..000NN; on an up-to-date
// database it is a no-op.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	db, err := goose.OpenDBWithDriver("pgx", pool.Config().ConnString())
	if err != nil {
		return fmt.Errorf("open goose db: %w", err)
	}
	defer db.Close()

	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// MigrateHandler exposes a tiny HTTP endpoint that runs Migrate on demand. It
// is guarded behind admin auth by the caller and is only used for one-off
// manual runs; the normal path is Migrate() at startup.
func MigrateHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := Migrate(r.Context(), pool); err != nil {
			http.Error(w, fmt.Sprintf("migration failed: %v", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("migrations applied\n"))
	}
}
