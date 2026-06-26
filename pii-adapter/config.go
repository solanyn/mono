package main

import (
	"log"
	"os"
	"strconv"
)

// defaultScoreThreshold is used when SCORE_THRESHOLD is unset or unparseable.
const defaultScoreThreshold = 0.5

// Config holds runtime configuration sourced from environment variables.
type Config struct {
	// PresidioURL is the base URL of the Presidio Analyzer REST API.
	PresidioURL string
	// ScoreThreshold is the minimum confidence score for a detection to be
	// considered. Presidio's /analyze API requires this as a JSON number.
	ScoreThreshold float64
	// ListenAddr is the address the HTTP adapter binds to.
	ListenAddr string
	// Language is the ISO language code passed to Presidio.
	Language string
}

// loadConfig reads configuration from the environment, applying defaults.
func loadConfig() Config {
	return Config{
		PresidioURL:    getenv("PRESIDIO_ANALYZER_URL", "http://localhost:3000"),
		ScoreThreshold: parseFloat(getenv("SCORE_THRESHOLD", ""), defaultScoreThreshold),
		ListenAddr:     getenv("LISTEN_ADDR", ":8000"),
		Language:       getenv("LANGUAGE", "en"),
	}
}

// parseFloat parses s as a float64, returning def (and logging a warning) when
// s is empty or invalid.
func parseFloat(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("warning: invalid SCORE_THRESHOLD %q, using default %g: %v", s, def, err)
		return def
	}
	return v
}

// getenv returns the value of the environment variable named by key, or def if
// it is unset or empty.
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
