package temporal

import (
	"github.com/combi/pkg/log"
)

// temporalLogger adapts our logger interface to Temporal's logger interface.
type temporalLogger struct {
	logger log.Logger
}

func newTemporalLogger(logger log.Logger) *temporalLogger {
	return &temporalLogger{
		logger: logger.With(log.String("component", "temporal")),
	}
}

func (l *temporalLogger) Debug(msg string, keyvals ...interface{}) {
	l.logger.Debug(msg, keyvalsToAttrs(keyvals)...)
}

func (l *temporalLogger) Info(msg string, keyvals ...interface{}) {
	l.logger.Info(msg, keyvalsToAttrs(keyvals)...)
}

func (l *temporalLogger) Warn(msg string, keyvals ...interface{}) {
	l.logger.Warn(msg, keyvalsToAttrs(keyvals)...)
}

func (l *temporalLogger) Error(msg string, keyvals ...interface{}) {
	l.logger.Error(msg, nil, keyvalsToAttrs(keyvals)...)
}

// keyvalsToAttrs converts Temporal's keyval format to our log attributes.
func keyvalsToAttrs(keyvals []interface{}) []log.Attr {
	if len(keyvals)%2 != 0 {
		return nil
	}

	attrs := make([]log.Attr, 0, len(keyvals)/2)
	for i := 0; i < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			continue
		}
		attrs = append(attrs, log.Any(key, keyvals[i+1]))
	}
	return attrs
}
