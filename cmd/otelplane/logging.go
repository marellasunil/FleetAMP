// OTelPlane logging bootstrap and fatal-error helpers.
package main

import (
	"fmt"
	"log/slog"
	"os"

	fleetlogging "github.com/marellasunil/OTelPlane/internal/logging"
)

// configureLogging initializes structured logging from OTelPlane environment variables and returns a cleanup function.
func configureLogging() func() {
	closeLog, err := fleetlogging.Setup(fleetlogging.Config{
		Level:  envOrDefault("OTELPLANE_LOG_LEVEL", "info"),
		Format: envOrDefault("OTELPLANE_LOG_FORMAT", "json"),
		File:   os.Getenv("OTELPLANE_LOG_FILE"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure logging: %v\n", err)
		os.Exit(1)
	}
	slog.Info("OTelPlane logging initialized", "component", "logging", "event", "logging_initialized")
	return func() { _ = closeLog() }
}

// fatalLog records an unrecoverable startup error and terminates the process.
func fatalLog(message string, err error) {
	slog.Error(message, "component", "startup", "event", "startup_failed", "error", err)
	os.Exit(1)
}
