// Package observability contains logging and metrics primitives.
package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Logger is the narrow structured logging surface used by internal packages.
type Logger interface {
	Debug(ctx context.Context, msg string, attrs ...slog.Attr)
	Info(ctx context.Context, msg string, attrs ...slog.Attr)
	Warn(ctx context.Context, msg string, attrs ...slog.Attr)
	Error(ctx context.Context, msg string, attrs ...slog.Attr)
	With(attrs ...slog.Attr) Logger
}

// SlogLogger adapts slog.Logger to the project Logger interface.
type SlogLogger struct {
	logger *slog.Logger
}

// NewLogger returns a JSON structured logger writing to stdout.
func NewLogger(level slog.Leveler) Logger {
	return NewJSONLogger(os.Stdout, level)
}

// NewJSONLogger returns a JSON structured logger writing to w.
func NewJSONLogger(w io.Writer, level slog.Leveler) Logger {
	if level == nil {
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	return &SlogLogger{logger: slog.New(handler)}
}

// NewTextLogger returns a human-readable structured logger writing to w.
func NewTextLogger(w io.Writer, level slog.Leveler) Logger {
	if level == nil {
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	return &SlogLogger{logger: slog.New(handler)}
}

// FromSlog wraps an existing slog logger.
func FromSlog(logger *slog.Logger) Logger {
	if logger == nil {
		return NewLogger(slog.LevelInfo)
	}

	return &SlogLogger{logger: logger}
}

func (l *SlogLogger) Debug(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelDebug, msg, attrs...)
}

func (l *SlogLogger) Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelInfo, msg, attrs...)
}

func (l *SlogLogger) Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelWarn, msg, attrs...)
}

func (l *SlogLogger) Error(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelError, msg, attrs...)
}

func (l *SlogLogger) With(attrs ...slog.Attr) Logger {
	args := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		args = append(args, attr)
	}

	return &SlogLogger{logger: l.logger.With(args...)}
}

func (l *SlogLogger) log(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	if ctx == nil {
		ctx = context.Background()
	}

	l.logger.LogAttrs(ctx, level, msg, attrs...)
}
