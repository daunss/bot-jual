package logging

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger initialises an slog.Logger with the provided level string.
func NewLogger(levelStr string) *slog.Logger {
	level := parseLevel(levelStr)
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(handler)
}

func parseLevel(levelStr string) slog.Leveler {
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
