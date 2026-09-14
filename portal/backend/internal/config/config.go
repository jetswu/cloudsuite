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
	CORSAllowedOrigins string
	LogLevelRaw        string
}

// Load reads config from environment with sane defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		AuthentikIssuer:    os.Getenv("AUTHENTIK_ISSUER"),
		AuthentikClientID:  os.Getenv("AUTHENTIK_CLIENT_ID"),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"),
		LogLevelRaw:        getEnv("LOG_LEVEL", "info"),
	}
	if cfg.AuthentikIssuer == "" {
		return nil, fmt.Errorf("AUTHENTIK_ISSUER is required")
	}
	if cfg.AuthentikClientID == "" {
		return nil, fmt.Errorf("AUTHENTIK_CLIENT_ID is required")
	}
	return cfg, nil
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
