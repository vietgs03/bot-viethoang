package logging

import (
	"context"
	"log/slog"

	"bot-viethoang/internal/domain/ports"
)

// SLogger is an adapter around slog.Logger implementing ports.Logger.
type SLogger struct {
	logger *slog.Logger
}

var _ ports.Logger = (*SLogger)(nil)

// New creates a new SLogger.
func New(logger *slog.Logger) *SLogger {
	return &SLogger{logger: logger}
}

// Info logs an informational message.
func (l *SLogger) Info(ctx context.Context, msg string, args ...any) {
	if l.logger == nil {
		return
	}
	l.logger.Log(ctx, slog.LevelInfo, msg, args...)
}

// Error logs an error message.
func (l *SLogger) Error(ctx context.Context, msg string, args ...any) {
	if l.logger == nil {
		return
	}
	l.logger.Log(ctx, slog.LevelError, msg, args...)
}
