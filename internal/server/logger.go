package server

import (
	"context"
	"log/slog"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/config"
)

var logger *slog.Logger

func Init(cfg *config.Config) {
	logger = slog.New(cfg.Handler())
}

func Logger() *slog.Logger {
	return logger
}

func WithContext(ctx context.Context) *slog.Logger {
	reqID := ctx.Value("request_id")
	if reqID != nil {
		return logger.With("request_id", reqID)
	}
	return logger
}
