// Package logging configures FleetAMP structured application logging.
//
// FleetAMP writes structured JSON or text logs to stdout for journald/container
// collection and can optionally mirror the same records to a local log file.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Level  string
	Format string
	File   string
}

// Setup configures FleetAMP's global structured logger and returns a function that flushes and closes file output.
func Setup(cfg Config) (func() error, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	writers := []io.Writer{os.Stdout}
	var file *os.File
	if cfg.File != "" {
		if err := os.MkdirAll(dir(cfg.File), 0o750); err != nil {
			return nil, fmt.Errorf("create log directory: %w", err)
		}
		file, err = os.OpenFile(cfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		writers = append(writers, file)
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(cfg.Format, "text") {
		handler = slog.NewTextHandler(io.MultiWriter(writers...), opts)
	} else {
		handler = slog.NewJSONHandler(io.MultiWriter(writers...), opts)
	}
	slog.SetDefault(slog.New(handler))

	return func() error {
		if file != nil {
			return file.Close()
		}
		return nil
	}, nil
}

// parseLevel translates a configured level name into the corresponding slog threshold.
func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level %q", value)
	}
}

// dir returns the parent directory that must exist before opening a configured log file.
func dir(path string) string {
	idx := strings.LastIndex(path, string(os.PathSeparator))
	if idx <= 0 {
		return "."
	}
	return path[:idx]
}
