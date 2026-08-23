// Package config loads and validates the application configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	ListenAddr       string
	DatabasePath     string
	JWTSecret        []byte
	AdminPassword    string
	TelegramToken    string
	TelegramChatID   string
	TelegramThreadID string
	LogLevel         string
	MaxBodyBytes     int64
	SessionTTL       int
	AdminHosts       []string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		ListenAddr:       getEnv("OOB_LISTEN_ADDR", ":8080"),
		DatabasePath:     getEnv("DATABASE_PATH", "data/ooblivion.db"),
		JWTSecret:        []byte(os.Getenv("JWT_SECRET")),
		AdminPassword:    os.Getenv("ADMIN_PASSWORD"),
		TelegramToken:    os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		TelegramThreadID: os.Getenv("TELEGRAM_THREAD_ID"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		MaxBodyBytes:     1048576,
		SessionTTL:       12,
	}

	if v := os.Getenv("MAX_BODY_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("MAX_BODY_BYTES: %w", err)
		}
		cfg.MaxBodyBytes = n
	}

	cfg.AdminHosts = parseHosts(os.Getenv("ADMIN_HOST"))

	if len(cfg.JWTSecret) < 64 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 64 bytes")
	}
	if cfg.AdminPassword != "" && len(cfg.AdminPassword) < 16 {
		return nil, fmt.Errorf("ADMIN_PASSWORD must be at least 16 chars")
	}
	return cfg, nil
}

func parseHosts(raw string) []string {
	hosts := []string{}
	for _, part := range strings.Split(raw, ",") {
		host := strings.ToLower(strings.TrimSpace(part))
		if host == "" {
			continue
		}
		if i := strings.LastIndex(host, ":"); i != -1 && !strings.Contains(host[i+1:], "]") {
			host = host[:i]
		}
		if host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
