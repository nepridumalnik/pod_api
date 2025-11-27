package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Setup configures zerolog with sane defaults.
// Uses console writer for human-readable logs by default.
func Setup() {
	// Timestamp format
	zerolog.TimeFieldFormat = time.RFC3339

	// Human-friendly console writer by default
	cw := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stdout
		w.TimeFormat = time.RFC3339
	})

	log.Logger = zerolog.New(cw).With().Timestamp().Logger()
}
