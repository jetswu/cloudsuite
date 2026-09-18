package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// Config holds backend configuration loaded from environment variables.
type Config struct {
	Port               string
	AuthentikIssuer    string
	AuthentikClientID  string
	AuthentikAPIURL    string
	AuthentikAPIToken  string
	CORSAllowedOrigins string
	LogLevelRaw        string

	// Portal database (Sprint 1.3).
	PortalDBHost     string
	PortalDBPort     string
	PortalDBName     string
	PortalDBUser     string
	PortalDBPassword string

	// Stalwart mail server (Sprint 1.3).
	StalwartAPIURL string
	StalwartAPIKey string

	// Provisioning pipeline (Sprint 1.4a).
	RedisAddr     string
	RedisPassword string
	NCBaseURL     string // nextcloud OCS base, e.g. http://nextcloud
	NCDBName      string
	NCAdminUser   string
	NCAdminPass   string
	OdooBaseURL   string
	OdooDB        string
	OdooLogin     string
	OdooPassword  string

	// Widget deep links (Sprint 1.5a): public bases used to build the
	// "open in service" links of the dashboard widgets.
	DriveURL   string
	ErpURL     string
	WebmailURL string
}

// Load reads config from environment with sane defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		AuthentikIssuer:    os.Getenv("AUTHENTIK_ISSUER"),
		AuthentikClientID:  os.Getenv("AUTHENTIK_CLIENT_ID"),
		AuthentikAPIURL:    os.Getenv("AUTHENTIK_API_URL"),
		AuthentikAPIToken:  os.Getenv("AUTHENTIK_API_TOKEN"),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"),
		LogLevelRaw:        getEnv("LOG_LEVEL", "info"),

		PortalDBHost:     getEnv("PORTAL_DB_HOST", "postgres"),
		PortalDBPort:     getEnv("PORTAL_DB_PORT", "5432"),
		PortalDBName:     getEnv("PORTAL_DB_NAME", "portal"),
		PortalDBUser:     getEnv("PORTAL_DB_USER", "cloudsuite"),
		PortalDBPassword: os.Getenv("PORTAL_DB_PASSWORD"),

		StalwartAPIURL: os.Getenv("STALWART_API_URL"),
		StalwartAPIKey: os.Getenv("STALWART_API_KEY"),

		RedisAddr:     getEnv("REDIS_ADDR", ""),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		NCBaseURL:     getEnv("NEXTCLOUD_BASE_URL", "http://nextcloud"),
		NCDBName:      getEnv("NEXTCLOUD_DB_NAME", "nextcloud"),
		NCAdminUser:   os.Getenv("NEXTCLOUD_ADMIN_USER"),
		NCAdminPass:   os.Getenv("NEXTCLOUD_ADMIN_PASSWORD"),

		OdooBaseURL:  getEnv("ODOO_BASE_URL", "https://erp.idchsuite.my.id"),
		OdooDB:       getEnv("ODOO_DB", "odoo"),
		OdooLogin:    getEnv("ODOO_LOGIN", "admin"),
		OdooPassword: os.Getenv("ODOO_PASSWORD"),

		DriveURL:   getEnv("WIDGET_DRIVE_URL", "https://drive.idchsuite.my.id"),
		ErpURL:     getEnv("WIDGET_ERP_URL", "https://erp.idchsuite.my.id"),
		WebmailURL: getEnv("WIDGET_WEBMAIL_URL", "https://webmail.idchsuite.my.id"),
	}
	if cfg.AuthentikIssuer == "" {
		return nil, fmt.Errorf("AUTHENTIK_ISSUER is required")
	}
	if cfg.AuthentikClientID == "" {
		return nil, fmt.Errorf("AUTHENTIK_CLIENT_ID is required")
	}
	return cfg, nil
}

// PortalDBConfig returns the database.Config for the portal database.
func (c *Config) PortalDBConfig() (host, port, name, user, password string) {
	return c.PortalDBHost, c.PortalDBPort, c.PortalDBName, c.PortalDBUser, c.PortalDBPassword
}

// CORSAllowedOriginsSlice splits CORS_ALLOWED_ORIGINS (comma-separated).
func (c *Config) CORSAllowedOriginsSlice() []string {
	out := []string{}
	for _, o := range strings.Split(c.CORSAllowedOrigins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// LogLevel parses LOG_LEVEL to zerolog level (default info).
func (c *Config) LogLevel() zerolog.Level {
	switch strings.ToLower(c.LogLevelRaw) {
	case "debug":
		return zerolog.DebugLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
