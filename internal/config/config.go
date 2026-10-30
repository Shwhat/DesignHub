// Package config reads DesignHub settings from environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the settings the server needs at startup. Settings that an
// Admin can change at runtime live in the database instead.
type Config struct {
	Addr         string // listen address inside the container
	DatabaseURL  string // full PostgreSQL URL; built from the parts below when empty
	DBHost       string
	DBPort       string
	DBName       string
	DBUser       string
	DBPassword   string
	SecretsDir   string // generated database password and TLS certificate
	RepoDir      string // engineering git repository
	FilesDir     string // attachment file store
	BackupsDir   string // backup output
	SecureCookie string // auto, always or never
	Version      string
}

// Load reads the environment. Missing values fall back to the defaults used
// by the published docker-compose.yml.
func Load(version string) (*Config, error) {
	c := &Config{
		Addr:         env("DESIGNHUB_ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DESIGNHUB_DATABASE_URL"),
		DBHost:       env("DESIGNHUB_DB_HOST", "postgres"),
		DBPort:       env("DESIGNHUB_DB_PORT", "5432"),
		DBName:       env("DESIGNHUB_DB_NAME", "designhub"),
		DBUser:       env("DESIGNHUB_DB_USER", "designhub"),
		DBPassword:   os.Getenv("DESIGNHUB_DB_PASSWORD"),
		SecretsDir:   env("DESIGNHUB_SECRETS", "/secrets"),
		RepoDir:      env("DESIGNHUB_REPO", "/data/repo"),
		FilesDir:     env("DESIGNHUB_FILES", "/data/files"),
		BackupsDir:   env("DESIGNHUB_BACKUPS", "/backups"),
		SecureCookie: env("DESIGNHUB_SECURE_COOKIE", "auto"),
		Version:      version,
	}
	switch c.SecureCookie {
	case "auto", "always", "never":
	default:
		return nil, fmt.Errorf("DESIGNHUB_SECURE_COOKIE must be auto, always or never")
	}
	if c.DatabaseURL == "" && c.DBPassword == "" {
		b, err := os.ReadFile(filepath.Join(c.SecretsDir, "db_password"))
		if err == nil {
			c.DBPassword = strings.TrimSpace(string(b))
		}
	}
	return c, nil
}

// DSN returns the PostgreSQL connection string.
func (c *Config) DSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		quote(c.DBHost), quote(c.DBPort), quote(c.DBName), quote(c.DBUser), quote(c.DBPassword))
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `'`, `\'`) + "'"
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
