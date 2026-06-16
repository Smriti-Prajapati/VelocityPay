package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a production-grade zap logger.
// level can be "debug", "info", "warn", "error".
func New(level string, isDevelopment bool) (*zap.Logger, error) {
	var cfg zap.Config

	if isDevelopment {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		lvl = zapcore.InfoLevel
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)

	return cfg.Build()
}

// Must panics if logger creation fails. Useful in main().
func Must(log *zap.Logger, err error) *zap.Logger {
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	return log
}
