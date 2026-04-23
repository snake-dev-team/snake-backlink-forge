// Package util provides shared utility helpers for the API service.
package util

import (
	"fmt"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger constructs a zap.Logger tuned for the current environment.
// Production → JSON encoder on stdout (Fly log aggregation reads JSON natively).
// Development → console encoder with color for readability.
func NewLogger(cfg *config.Config) (*zap.Logger, error) {
	var zapCfg zap.Config

	if cfg.IsProduction() {
		zapCfg = zap.NewProductionConfig()
		zapCfg.Encoding = "json"
	} else {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Always write to stdout — captured by Fly, Docker, and systemd alike.
	zapCfg.OutputPaths = []string{"stdout"}
	zapCfg.ErrorOutputPaths = []string{"stdout"}

	level, err := zapcore.ParseLevel(cfg.LogLevel)
	if err != nil {
		// Non-fatal: fall back to info if the level string is unrecognised.
		level = zapcore.InfoLevel
	}
	zapCfg.Level = zap.NewAtomicLevelAt(level)

	logger, err := zapCfg.Build(zap.AddCallerSkip(0))
	if err != nil {
		return nil, fmt.Errorf("logger: build: %w", err)
	}

	return logger, nil
}
