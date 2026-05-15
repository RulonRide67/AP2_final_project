package logger

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a Zap logger for the given level (debug, info, warn, error).
func New(level string) (*zap.Logger, error) {
	var cfg zap.Config
	if strings.EqualFold(level, "debug") {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	parsed, err := zapcore.ParseLevel(strings.ToLower(level))
	if err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}
	cfg.Level = zap.NewAtomicLevelAt(parsed)

	return cfg.Build()
}
