package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestLogger_ReturnsLogger(t *testing.T) {
	l := Logger()
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	l2 := Logger()
	if l != l2 {
		t.Fatal("expected same logger instance from singleton")
	}
}

func TestGCPHandler_LevelMapping(t *testing.T) {
	tests := []struct {
		level    slog.Level
		severity string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARNING"},
		{slog.LevelError, "ERROR"},
		{levelCritical, "CRITICAL"},
		{levelAlert, "ALERT"},
		{levelEmergency, "EMERGENCY"},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			var buf bytes.Buffer
			h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
				ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
					if a.Key == slog.LevelKey {
						if level, ok := a.Value.Any().(slog.Level); ok {
							if name, found := gcpLevelNames[level]; found {
								a.Value = slog.StringValue(name)
							}
						}
						a.Key = "severity"
					}
					return a
				},
			})
			logger := slog.New(&gcpHandler{Handler: h})
			logger.Log(context.TODO(), tt.level, "test message")

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("failed to unmarshal log: %v", err)
			}
			if entry["severity"] != tt.severity {
				t.Fatalf("expected severity %q, got %q", tt.severity, entry["severity"])
			}
		})
	}
}

func TestGCPHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := &gcpHandler{Handler: h}

	wrapped := handler.WithAttrs([]slog.Attr{slog.String("key", "value")})
	if _, ok := wrapped.(*gcpHandler); !ok {
		t.Fatal("expected *gcpHandler from WithAttrs")
	}
}

func TestGCPHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := &gcpHandler{Handler: h}

	wrapped := handler.WithGroup("group")
	if _, ok := wrapped.(*gcpHandler); !ok {
		t.Fatal("expected *gcpHandler from WithGroup")
	}
}
