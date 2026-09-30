// internal/platform/log/zap/adapter.go
package zap

import (
	"fmt"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapAdapter wraps a Zap logger to satisfy our domain's log.Logger interface.
type ZapAdapter struct {
	logger *zap.Logger
}

// Statically assert that ZapAdapter implements the log.Logger interface.
var _ log.Logger = (*ZapAdapter)(nil)

// New creates a new ZapAdapter.
func New(logger *zap.Logger) log.Logger {
	return &ZapAdapter{logger: logger}
}

// NewZapLogger creates a new configured Zap logger instance based on application config.
func NewZapLogger(logCfg config.LogConfig, serverEnv string) (*zap.Logger, error) {
	var zapCfg zap.Config
	if serverEnv == "development" {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		zapCfg = zap.NewProductionConfig()
	}

	level, err := zapcore.ParseLevel(logCfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	zapCfg.Level = zap.NewAtomicLevelAt(level)

	// AddCallerSkip(1) tells zap to skip one level in the call stack
	// so the log line numbers point to your application code, not this adapter.
	logger, err := zapCfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, fmt.Errorf("failed to build zap logger: %w", err)
	}

	return logger, nil
}

// toZapFields converts our domain's log.Attr slice into Zap's native zap.Field slice.
func toZapFields(attrs []log.Attr) []zap.Field {
	fields := make([]zap.Field, len(attrs))
	for i, attr := range attrs {
		fields[i] = zap.Any(attr.Key, attr.Value)
	}
	return fields
}

func (z *ZapAdapter) Debug(msg string, attrs ...log.Attr) {
	z.logger.Debug(msg, toZapFields(attrs)...)
}

func (z *ZapAdapter) Info(msg string, attrs ...log.Attr) {
	z.logger.Info(msg, toZapFields(attrs)...)
}

func (z *ZapAdapter) Warn(msg string, attrs ...log.Attr) {
	z.logger.Warn(msg, toZapFields(attrs)...)
}

func (z *ZapAdapter) Error(msg string, err error, attrs ...log.Attr) {
	allAttrs := make([]log.Attr, 0, len(attrs)+1)
	if err != nil {
		allAttrs = append(allAttrs, log.String("error", err.Error()))
	}
	allAttrs = append(allAttrs, attrs...)
	z.logger.Error(msg, toZapFields(allAttrs)...)
}

// With returns a new logger instance with the provided attributes.
func (z *ZapAdapter) With(attrs ...log.Attr) log.Logger {
	fields := toZapFields(attrs)
	return New(z.logger.With(fields...))
}
