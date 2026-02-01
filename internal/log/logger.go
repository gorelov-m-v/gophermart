// Package log provides structured logging functionality.
package log

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger defines the interface for structured logging.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Fatal(msg string, args ...any)
	With(args ...any) Logger
	Sync() error
}

type logger struct {
	zap *zap.Logger
}

// NewLogger creates a new Logger instance.
func NewLogger() Logger {
	config := zap.NewProductionEncoderConfig()
	config.TimeKey = "timestamp"
	config.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(config),
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)

	return &logger{
		zap: zap.New(core),
	}
}

func (l *logger) Debug(msg string, args ...any) {
	l.zap.Debug(msg, toZapFields(args)...)
}

func (l *logger) Info(msg string, args ...any) {
	l.zap.Info(msg, toZapFields(args)...)
}

func (l *logger) Warn(msg string, args ...any) {
	l.zap.Warn(msg, toZapFields(args)...)
}

func (l *logger) Error(msg string, args ...any) {
	l.zap.Error(msg, toZapFields(args)...)
}

func (l *logger) Fatal(msg string, args ...any) {
	l.zap.Fatal(msg, toZapFields(args)...)
}

func (l *logger) With(args ...any) Logger {
	return &logger{
		zap: l.zap.With(toZapFields(args)...),
	}
}

func (l *logger) Sync() error {
	return l.zap.Sync()
}

func toZapFields(args []any) []zap.Field {
	fields := make([]zap.Field, 0, len(args)/2)
	for i := 0; i < len(args)-1; i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, args[i+1]))
	}
	return fields
}
