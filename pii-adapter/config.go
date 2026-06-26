package main

import "os"

// Config holds runtime configuration sourced from environment variables.
type Config struct {
	// PresidioURL is the base URL of the Presidio Analyzer REST API.
	PresidioURL string
	// ScoreThreshold is the minimum confidence score for a detection to be
	// considered. Sent to Presidio as a string per its API contract.
	ScoreThreshold string
	// ListenAddr is the address the HTTP adapter binds to.
	ListenAddr string
	// Language is the ISO language code passed to Presidio.
	Language string
}

// loadConfig reads configuration from the environment, applying defaults.
func loadConfig() Config {
	return Config{
		PresidioURL:    getenv("PRESIDIO_ANALYZER_URL", "http://localhost:3000"),
		ScoreThreshold: getenv("SCORE_THRESHOLD", "0.5"),
		ListenAddr:     getenv("LISTEN_ADDR", ":8000"),
		Language:       getenv("LANGUAGE", "en"),
	}
}

// getenv returns the value of the environment variable named by key, or def if
// it is unset or empty.
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
