package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	// defaultAPIURL is the verified OpenRouter credits endpoint. It returns
	// data.total_credits and data.total_usage for the account owning the
	// management key.
	defaultAPIURL = "https://openrouter.ai/api/v1/credits"

	defaultAddr     = ":9101"
	defaultInterval = 5 * time.Minute
	defaultTimeout  = 10 * time.Second
	defaultStaleMul = 3

	// minInterval keeps the poll rate conservative. The credits endpoint is a
	// management API and should not be hammered.
	minInterval = time.Minute
	maxTimeout  = 60 * time.Second
)

// ErrMissingManagementKey is returned when no management key is configured.
// It never contains the key value.
var ErrMissingManagementKey = errors.New("no OpenRouter management key configured; set OPENROUTER_MANAGEMENT_KEY or OPENROUTER_MANAGEMENT_KEY_FILE")

// Config is the exporter's runtime configuration, resolved from the
// environment. It is deliberately free of the key's value so it is safe to log.
type Config struct {
	Addr          string
	APIURL        string
	ManagementKey string
	KeySource     string
	Interval      time.Duration
	Timeout       time.Duration
	StaleAfter    time.Duration
}

// LoadConfig reads configuration from getenv (normally os.Getenv) and
// validates it. File-backed secrets are read from disk.
func LoadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		Addr:   envOr(getenv, "OPENROUTER_EXPORTER_ADDR", defaultAddr),
		APIURL: envOr(getenv, "OPENROUTER_API_URL", defaultAPIURL),
	}

	key, source, err := loadSecret(getenv)
	if err != nil {
		return Config{}, err
	}
	cfg.ManagementKey = key
	cfg.KeySource = source

	if cfg.Interval, err = envDuration(getenv, "OPENROUTER_EXPORTER_INTERVAL", defaultInterval); err != nil {
		return Config{}, err
	}
	if cfg.Interval < minInterval {
		return Config{}, fmt.Errorf("OPENROUTER_EXPORTER_INTERVAL %s is below the %s minimum", cfg.Interval, minInterval)
	}
	if cfg.Timeout, err = envDuration(getenv, "OPENROUTER_EXPORTER_TIMEOUT", defaultTimeout); err != nil {
		return Config{}, err
	}
	if cfg.Timeout <= 0 || cfg.Timeout > maxTimeout {
		return Config{}, fmt.Errorf("OPENROUTER_EXPORTER_TIMEOUT must be > 0 and <= %s, got %s", maxTimeout, cfg.Timeout)
	}
	if cfg.StaleAfter, err = envDuration(getenv, "OPENROUTER_EXPORTER_STALE_AFTER", cfg.Interval*defaultStaleMul); err != nil {
		return Config{}, err
	}
	if cfg.StaleAfter < cfg.Timeout {
		return Config{}, fmt.Errorf("OPENROUTER_EXPORTER_STALE_AFTER %s must be >= OPENROUTER_EXPORTER_TIMEOUT %s", cfg.StaleAfter, cfg.Timeout)
	}
	return cfg, nil
}

// loadSecret resolves the management key. A secret file takes precedence over a
// raw environment value so file-backed deployments (tmpfs secret volumes) are
// unambiguous. The returned value is never logged.
func loadSecret(getenv func(string) string) (key, source string, err error) {
	if path := strings.TrimSpace(getenv("OPENROUTER_MANAGEMENT_KEY_FILE")); path != "" {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", "", fmt.Errorf("reading OPENROUTER_MANAGEMENT_KEY_FILE: %w", readErr)
		}
		value := strings.TrimSpace(string(raw))
		if value == "" {
			return "", "", fmt.Errorf("OPENROUTER_MANAGEMENT_KEY_FILE %q is empty", path)
		}
		return value, "file", nil
	}
	if value := strings.TrimSpace(getenv("OPENROUTER_MANAGEMENT_KEY")); value != "" {
		return value, "env", nil
	}
	return "", "", ErrMissingManagementKey
}

func envOr(getenv func(string) string, key, fallback string) string {
	if v := strings.TrimSpace(getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envDuration(getenv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a valid duration", key, raw)
	}
	return d, nil
}
