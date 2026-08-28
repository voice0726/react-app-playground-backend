package logging

import (
	"io"
	"log/slog"
	"os"

	"github.com/voice0726/react-app-playground-backend/internal/config"
)

func New(cfg config.Config) *slog.Logger {
	var h slog.Handler
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(io.Writer(os.Stdout), opts)
	}
	return slog.New(h)
}
