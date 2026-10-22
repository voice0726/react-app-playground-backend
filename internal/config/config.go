package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"
)

type Config struct {
	ServerAddr               string
	DatabaseURL              string
	AuthGatewayURL           string
	AuthGatewayInternalToken string
	PublicOrigin             string
	AuthGatewayTimeout       time.Duration
	ShutdownTimeout          time.Duration
	LogLevel                 slog.Level
	LogFormat                string
}

func Load() (Config, error) {
	if err := LoadDotEnv(); err != nil {
		return Config{}, err
	}
	get := func(key string) (string, error) {
		v := os.Getenv(key)
		if v == "" {
			return "", fmt.Errorf("%s is required", key)
		}
		return v, nil
	}
	serverAddr, err := get("SERVER_ADDR")
	if err != nil {
		return Config{}, err
	}
	databaseURL, err := get("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	authURL, err := get("AUTH_GATEWAY_URL")
	if err != nil {
		return Config{}, err
	}
	token, err := get("AUTH_GATEWAY_INTERNAL_TOKEN")
	if err != nil {
		return Config{}, err
	}
	publicOrigin, err := get("PUBLIC_ORIGIN")
	if err != nil {
		return Config{}, err
	}
	timeoutText, err := get("AUTH_GATEWAY_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	shutdownText, err := get("SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	levelText, err := get("LOG_LEVEL")
	if err != nil {
		return Config{}, err
	}
	format, err := get("LOG_FORMAT")
	if err != nil {
		return Config{}, err
	}
	timeout, err := time.ParseDuration(timeoutText)
	if err != nil {
		return Config{}, fmt.Errorf("AUTH_GATEWAY_TIMEOUT is invalid")
	}
	shutdown, err := time.ParseDuration(shutdownText)
	if err != nil {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT is invalid")
	}
	level := slog.LevelInfo
	switch levelText {
	case "debug":
		level = slog.LevelDebug
	case "info":
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return Config{}, fmt.Errorf("LOG_LEVEL is invalid")
	}
	if format != "text" && format != "json" {
		return Config{}, fmt.Errorf("LOG_FORMAT is invalid")
	}
	for key, value := range map[string]string{"AUTH_GATEWAY_URL": authURL, "PUBLIC_ORIGIN": publicOrigin} {
		u, parseErr := url.Parse(value)
		if parseErr != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return Config{}, fmt.Errorf("%s is invalid", key)
		}
	}
	return Config{serverAddr, databaseURL, authURL, token, publicOrigin, timeout, shutdown, level, format}, nil
}
