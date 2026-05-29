package log

import (
	"io"
	"log/slog"
	"os"
)

// Config controls structured logging.
type Config struct {
	Level  string
	Format string
}

// New creates a slog logger.
func New(cfg Config, out io.Writer) *slog.Logger {
	if out == nil {
		out = os.Stdout
	}
	level := new(slog.LevelVar)
	level.Set(parseLevel(cfg.Level))

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}
	return slog.New(handler)
}

func parseLevel(value string) slog.Level {
	switch value {
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
