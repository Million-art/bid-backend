package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey struct{}
type ridCtxKey struct{}

// WithContext returns ctx with l attached so callers downstream can pull a
// request-scoped logger without having to thread it through every signature.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the request-scoped logger embedded in ctx, falling
// back to slog.Default() (which is the process-wide logger configured at
// startup) when no per-request logger is attached  for cron jobs etc.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return slog.Default()
}

// WithRequestID returns ctx tagged with the bare request_id string. The
// RequestID HTTP middleware stores both this and the bound logger so
// service-layer code that needs the raw id (e.g. propagating X-Request-ID
// onto an outbound HTTP call) can pull it without parsing slog attrs.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ridCtxKey{}, id)
}

// RequestIDFromContext returns the request_id attached by WithRequestID, or
// the empty string if none was set (cron, background tasks, etc.).
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ridCtxKey{}).(string); ok {
		return v
	}
	return ""
}

const (
	FormatJSON = "json"
	FormatText = "text"
)

func New(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatText:
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

// parseLevel maps a textual level to slog.Level. Unknown values yield Info.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
