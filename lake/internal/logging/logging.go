package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup configures the default slog logger with a JSON handler at the given level.
// Accepts "debug", "info", "warn", "error" (case-insensitive).
func Setup(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(h))
}
