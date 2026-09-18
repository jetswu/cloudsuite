package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/jetswu/cloudsuite/portal/backend/internal/auth"
	"github.com/jetswu/cloudsuite/portal/backend/internal/config"
	"github.com/jetswu/cloudsuite/portal/backend/internal/database"
	"github.com/jetswu/cloudsuite/portal/backend/internal/handlers"
	"github.com/jetswu/cloudsuite/portal/backend/internal/provisioning"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/authentik"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/postgres"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/stalwart"
	"github.com/jetswu/cloudsuite/portal/backend/internal/service"
	"github.com/jetswu/cloudsuite/portal/backend/internal/widget"
	"github.com/jetswu/cloudsuite/portal/backend/internal/widgetcache"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}

	zerolog.SetGlobalLevel(cfg.LogLevel())
	log.Info().
		Str("port", cfg.Port).
		Str("issuer", cfg.AuthentikIssuer).
		Msg("starting portal backend")

	// OIDC verifier (Authentik)
	verifier, err := auth.NewVerifier(context.Background(), cfg.AuthentikIssuer, cfg.AuthentikClientID)
	if err != nil {
		log.Fatal().Err(err).Msg("init oidc verifier")
	}

	h := handlers.New(verifier)

	// Portal PostgreSQL pool + migrations (Sprint 1.3).
	dbCfg := database.Config{
		Host:     cfg.PortalDBHost,
		Port:     cfg.PortalDBPort,
		Name:     cfg.PortalDBName,
		User:     cfg.PortalDBUser,
		Password: cfg.PortalDBPassword,
	}
	pool, err := database.Connect(context.Background(), dbCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("connect portal db")
	}
	defer pool.Close()

	if err := database.Migrate(context.Background(), pool); err != nil {
		log.Fatal().Err(err).Msg("apply portal migrations")
	}
	log.Info().Msg("portal migrations applied")

	// Sprint 1.5a: append-only audit trail. Writes are best-effort; a nil
	// pool would make it a no-op, but the pool is always wired in prod.
	auditLog := service.NewAuditLogger(pool)

	// Admin handler backed by the Authentik API (only when configured).
	var admin *handlers.AdminHandler
	if cfg.AuthentikAPIURL != "" && cfg.AuthentikAPIToken != "" {
		adminRepo := authentik.NewClient(cfg.AuthentikAPIURL, cfg.AuthentikAPIToken)

		// Provisioning pipeline (Sprint 1.4a): wired when Redis and Stalwart
		// are configured; otherwise user creation still works without it.
		var provSvc *provisioning.Service
		if cfg.RedisAddr != "" && cfg.StalwartAPIURL != "" {
			provSvc = provisioning.NewService(
				provisioning.NewQueue(cfg.RedisAddr, cfg.RedisPassword),
				provisioning.NewStore(pool),
				stalwart.NewClient(cfg.StalwartAPIURL, cfg.StalwartAPIKey),
			)
			log.Info().Str("redis", cfg.RedisAddr).Msg("provisioning pipeline enabled")
		} else {
			log.Warn().Msg("REDIS_ADDR/STALWART_API_URL not set; provisioning disabled")
		}

		admin = handlers.NewAdminHandler(adminRepo, provSvc, auditLog)
		log.Info().Str("url", cfg.AuthentikAPIURL).Msg("admin API enabled")
	} else {
		log.Warn().Msg("AUTHENTIK_API_URL/AUTHENTIK_API_TOKEN not set; /api/admin disabled")
	}

	// Domain onboarding (Sprint 1.3).
	var domainHandler *handlers.DomainHandler
	if cfg.StalwartAPIURL != "" && cfg.StalwartAPIKey != "" {
		domainSvc := service.NewDomainService(
			postgres.NewDomainRepo(pool),
			postgres.NewDNSRecordRepo(pool),
			stalwart.NewClient(cfg.StalwartAPIURL, cfg.StalwartAPIKey),
			service.NewDNSChecker(),
		)
		domainHandler = handlers.NewDomainHandler(domainSvc, auditLog)
		log.Info().Str("url", cfg.StalwartAPIURL).Msg("domain onboarding enabled")
	} else {
		log.Warn().Msg("STALWART_API_URL/STALWART_API_KEY not set; /api/admin/domains disabled")
	}

	r := chi.NewRouter()

	allowedOrigins := cfg.CORSAllowedOriginsSlice()
	log.Info().Strs("origins", allowedOrigins).Msg("CORS")
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/api/health", h.Health)
	r.Group(func(protected chi.Router) {
		protected.Use(h.RequireAuth)
		protected.Get("/api/me", h.Me)
	})

	if admin != nil {
		admin.Routes(r, h.RequireAuth)
	}

	if domainHandler != nil {
		r.Group(func(adminRouter chi.Router) {
			adminRouter.Use(h.RequireAuth)
			if admin != nil {
				adminRouter.Use(admin.RequireSuperAdmin)
			}
			domainHandler.Routes(adminRouter)
		})
	}

	// Dashboard widgets (Sprint 1.5a): read-only summaries over Stalwart,
	// Nextcloud and Odoo with Redis-backed caching. Each widget degrades
	// independently at runtime (unavailable flag), so wiring requires only
	// the Stalwart config the portal already has.
	if cfg.StalwartAPIURL != "" && cfg.StalwartAPIKey != "" {
		var wcache *widgetcache.Cache
		if cfg.RedisAddr != "" {
			if c, err := widgetcache.New(cfg.RedisAddr, cfg.RedisPassword); err != nil {
				log.Warn().Err(err).Msg("widget redis cache unavailable (widgets fetch fresh)")
			} else {
				defer c.Close()
				wcache = c
			}
		}
		// Nextcloud DB (recent files) on the shared postgres instance; the
		// worker proves this pool works. Optional: without it the drive
		// widget still shows storage numbers.
		ncdb, err := database.Connect(context.Background(), database.Config{
			Host:     cfg.PortalDBHost,
			Port:     cfg.PortalDBPort,
			Name:     cfg.NCDBName,
			User:     cfg.PortalDBUser,
			Password: cfg.PortalDBPassword,
		})
		if err != nil {
			log.Error().Err(err).Msg("connect nextcloud db (widget recent files disabled)")
			ncdb = nil
		} else {
			defer ncdb.Close()
		}
		widgetSvc := widget.New(
			cfg.StalwartAPIURL, cfg.StalwartAPIKey,
			cfg.NCBaseURL, cfg.NCAdminUser, cfg.NCAdminPass,
			ncdb, cfg.DriveURL,
			cfg.OdooBaseURL, cfg.OdooDB, cfg.OdooLogin, cfg.OdooPassword,
			cfg.ErpURL, cfg.WebmailURL,
			wcache,
		)
		widgetHandler := handlers.NewWidgetHandler(widgetSvc)
		r.Group(func(widgetRouter chi.Router) {
			widgetRouter.Use(h.RequireAuth)
			widgetHandler.Routes(widgetRouter)
		})
		log.Info().Msg("dashboard widgets enabled")
	}

	// Sprint 1.5a: audit trail read endpoints. They require the admin handler
	// because RequireSuperAdmin lives there; without Authentik API config
	// there is no superadmin concept, so the endpoints stay closed.
	if admin != nil {
		auditHandler := handlers.NewAuditHandler(postgres.NewAuditRepo(pool))
		r.Group(func(adminRouter chi.Router) {
			adminRouter.Use(h.RequireAuth)
			adminRouter.Use(admin.RequireSuperAdmin)
			auditHandler.Routes(adminRouter)
		})
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("listen")
		}
	}()
	log.Info().Str("addr", srv.Addr).Msg("listening")

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("shutdown")
	}
}
