package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/jetswu/cloudsuite/portal/backend/internal/config"
	"github.com/jetswu/cloudsuite/portal/backend/internal/database"
	"github.com/jetswu/cloudsuite/portal/backend/internal/provisioning"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/authentik"
)

// worker is the provisioning consumer (Sprint 1.4a). It owns no HTTP surface:
// it drains the Redis queue, runs the per-service connectors and writes job /
// user_provisioning state to the portal database. Shutdown is graceful
// (SIGINT/SIGTERM): the in-flight job finishes, the BRPOP is unblocked.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}
	if cfg.AuthentikAPIURL == "" || cfg.AuthentikAPIToken == "" {
		log.Fatal().Msg("AUTHENTIK_API_URL and AUTHENTIK_API_TOKEN are required for the provisioning worker")
	}
	if cfg.RedisAddr == "" {
		log.Fatal().Msg("REDIS_ADDR is required for the provisioning worker")
	}

	zerolog.SetGlobalLevel(cfg.LogLevel())
	log.Info().
		Str("redis", cfg.RedisAddr).
		Str("authentik", cfg.AuthentikAPIURL).
		Msg("starting provisioning worker")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Portal DB (jobs + user_provisioning state).
	pool, err := database.Connect(ctx, database.Config{
		Host:     cfg.PortalDBHost,
		Port:     cfg.PortalDBPort,
		Name:     cfg.PortalDBName,
		User:     cfg.PortalDBUser,
		Password: cfg.PortalDBPassword,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("connect portal db")
	}
	defer pool.Close()

	// NOTE: the worker does NOT run migrations — the portal backend owns them
	// and concurrent goose runs would race on startup.

	// Nextcloud DB (user_oidc mapping seed) on the same postgres instance.
	ncdb, err := database.Connect(ctx, database.Config{
		Host:     cfg.PortalDBHost,
		Port:     cfg.PortalDBPort,
		Name:     cfg.NCDBName,
		User:     cfg.PortalDBUser,
		Password: cfg.PortalDBPassword,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("connect nextcloud db")
	}
	defer ncdb.Close()

	authClient := authentik.NewClient(cfg.AuthentikAPIURL, cfg.AuthentikAPIToken)

	connectors := []provisioning.Connector{
		provisioning.NewStalwartConnector(cfg.StalwartAPIURL, cfg.StalwartAPIKey),
		provisioning.NewNextcloudConnector(cfg.NCBaseURL, cfg.NCAdminUser, cfg.NCAdminPass, ncdb),
		provisioning.NewOdooConnector(cfg.OdooBaseURL, cfg.OdooDB, cfg.OdooLogin, cfg.OdooPassword),
	}

	worker := provisioning.NewWorker(
		provisioning.NewQueue(cfg.RedisAddr, cfg.RedisPassword),
		provisioning.NewStore(pool),
		authClient,
		connectors,
	)
	worker.Run(ctx) // blocks until the signal context is cancelled
}
