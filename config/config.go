package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port     int
	BaseURL  string
	DataPath string
	AdminKey string

	// SMTP settings
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func LoadConfig() *Config {
	cfg := &Config{}

	flag.IntVar(&cfg.Port, "port", getEnvInt("PORT", 8080), "HTTP server port")
	flag.StringVar(&cfg.BaseURL, "base-url", getEnv("BASE_URL", "http://localhost:8080"), "Base URL for generating invitation links")
	flag.StringVar(&cfg.DataPath, "data", getEnv("DATA_PATH", "data/secretsanta.json"), "Path to database JSON file")
	flag.StringVar(&cfg.AdminKey, "admin-key", getEnv("ADMIN_KEY", ""), "Admin secret passkey (auto-generated if empty)")

	flag.StringVar(&cfg.SMTPHost, "smtp-host", getEnv("SMTP_HOST", ""), "SMTP server hostname")
	flag.IntVar(&cfg.SMTPPort, "smtp-port", getEnvInt("SMTP_PORT", 587), "SMTP port")
	flag.StringVar(&cfg.SMTPUser, "smtp-user", getEnv("SMTP_USER", ""), "SMTP username")
	flag.StringVar(&cfg.SMTPPass, "smtp-pass", getEnv("SMTP_PASS", ""), "SMTP password")
	flag.StringVar(&cfg.SMTPFrom, "smtp-from", getEnv("SMTP_FROM", ""), "From address for emails")

	flag.Parse()

	// Clean BaseURL (remove trailing slash)
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if !strings.HasPrefix(cfg.BaseURL, "http://") && !strings.HasPrefix(cfg.BaseURL, "https://") {
		cfg.BaseURL = "http://" + cfg.BaseURL
	}

	return cfg
}

func (c *Config) HasSMTP() bool {
	return c.SMTPHost != "" && c.SMTPPort > 0 && c.SMTPFrom != ""
}

func (c *Config) FormatInviteLink(code string) string {
	return fmt.Sprintf("%s/join/%s", c.BaseURL, code)
}

func (c *Config) FormatRevealLink(code string) string {
	return fmt.Sprintf("%s/reveal/%s", c.BaseURL, code)
}
