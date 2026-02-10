package logging

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/janisto/echo-playground/internal/platform/timeutil"
)

var (
	loggerOnce sync.Once
	baseLogger *slog.Logger
)

// gcpHandler wraps slog.JSONHandler to remap level names to GCP Cloud Logging
// severity strings and format timestamps with microsecond precision.
type gcpHandler struct {
	slog.Handler
}

func (h *gcpHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Time = r.Time.UTC()
	return h.Handler.Handle(ctx, r)
}

func (h *gcpHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &gcpHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *gcpHandler) WithGroup(name string) slog.Handler {
	return &gcpHandler{Handler: h.Handler.WithGroup(name)}
}

// gcpLevelNames maps slog levels to GCP Cloud Logging severity strings.
var gcpLevelNames = map[slog.Level]string{
	slog.LevelDebug: "DEBUG",
	slog.LevelInfo:  "INFO",
	slog.LevelWarn:  "WARNING",
	slog.LevelError: "ERROR",
	levelCritical:   "CRITICAL",
	levelAlert:      "ALERT",
	levelEmergency:  "EMERGENCY",
}

const (
	levelCritical  = slog.LevelError + 4
	levelAlert     = slog.LevelError + 8
	levelEmergency = slog.LevelError + 12
)

func initLogger() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().UTC().Format(timeutil.RFC3339Micros))
				a.Key = "timestamp"
			}
			if a.Key == slog.LevelKey {
				if level, ok := a.Value.Any().(slog.Level); ok {
					if name, found := gcpLevelNames[level]; found {
						a.Value = slog.StringValue(name)
					}
				}
				a.Key = "severity"
			}
			if a.Key == slog.MessageKey {
				a.Key = "message"
			}
			return a
		},
	})
	baseLogger = slog.New(&gcpHandler{Handler: h})
}

// Logger returns the process-wide slog.Logger instance.
func Logger() *slog.Logger {
	loggerOnce.Do(initLogger)
	return baseLogger
}
