package logging

import (
	"context"
	"log/slog"
	"os"
)

type (
	ctxLoggerKey  struct{}
	ctxTraceIDKey struct{}
)

// LoggerFromContext returns the request-scoped logger if present,
// otherwise falls back to the global logger.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return Logger()
	}
	if l, ok := ctx.Value(ctxLoggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return Logger()
}

// TraceIDFromContext returns the correlation identifier (trace or request ID) if present.
func TraceIDFromContext(ctx context.Context) *string {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(ctxTraceIDKey{}).(*string); ok && v != nil && *v != "" {
		return v
	}
	return nil
}

// LogInfo writes an informational message using the request-aware logger.
func LogInfo(ctx context.Context, msg string, attrs ...slog.Attr) {
	LoggerFromContext(ctx).LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

// LogWarn writes a warning message using the request-aware logger.
func LogWarn(ctx context.Context, msg string, attrs ...slog.Attr) {
	LoggerFromContext(ctx).LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
}

// LogError writes an error message using the request-aware logger
// and appends the error attribute when err is non-nil.
func LogError(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
	}
	LoggerFromContext(ctx).LogAttrs(ctx, slog.LevelError, msg, attrs...)
}

// LogFatal logs with emergency severity and terminates the process.
// It attaches the error attribute when err is non-nil.
func LogFatal(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
	}
	LoggerFromContext(ctx).LogAttrs(ctx, levelEmergency, msg, attrs...)
	os.Exit(1)
}

func contextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxLoggerKey{}, logger)
}

func contextWithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	traceCopy := traceID
	return context.WithValue(ctx, ctxTraceIDKey{}, &traceCopy)
}
