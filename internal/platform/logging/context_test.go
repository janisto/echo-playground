package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestLoggerFromContext_Nil(t *testing.T) {
	l := LoggerFromContext(context.TODO())
	if l == nil {
		t.Fatal("expected fallback to global logger")
	}
}

func TestLoggerFromContext_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	customLogger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := contextWithLogger(context.Background(), customLogger)

	l := LoggerFromContext(ctx)
	if l != customLogger {
		t.Fatal("expected custom logger from context")
	}
}

func TestLoggerFromContext_WithoutLogger(t *testing.T) {
	ctx := context.Background()
	l := LoggerFromContext(ctx)
	if l == nil {
		t.Fatal("expected fallback to global logger")
	}
}

func TestTraceIDFromContext_Nil(t *testing.T) {
	id := TraceIDFromContext(context.TODO())
	if id != nil {
		t.Fatal("expected nil for nil context")
	}
}

func TestTraceIDFromContext_WithTraceID(t *testing.T) {
	ctx := contextWithTraceID(context.Background(), "trace-abc")
	id := TraceIDFromContext(ctx)
	if id == nil {
		t.Fatal("expected non-nil trace ID")
	}
	if *id != "trace-abc" {
		t.Fatalf("expected 'trace-abc', got %q", *id)
	}
}

func TestTraceIDFromContext_EmptyTraceID(t *testing.T) {
	ctx := contextWithTraceID(context.Background(), "")
	id := TraceIDFromContext(ctx)
	if id != nil {
		t.Fatal("expected nil for empty trace ID")
	}
}

func TestLogInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := contextWithLogger(context.Background(), logger)

	LogInfo(ctx, "test info", slog.String("key", "val"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if entry["msg"] != "test info" {
		t.Fatalf("expected message 'test info', got %q", entry["msg"])
	}
	if entry["key"] != "val" {
		t.Fatalf("expected key='val', got %q", entry["key"])
	}
}

func TestLogWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx := contextWithLogger(context.Background(), logger)

	LogWarn(ctx, "test warn")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if entry["msg"] != "test warn" {
		t.Fatalf("expected message 'test warn', got %q", entry["msg"])
	}
}

func TestLogError_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := contextWithLogger(context.Background(), logger)

	LogError(ctx, "test error", errForTest("boom"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if entry["msg"] != "test error" {
		t.Fatalf("expected message 'test error', got %q", entry["msg"])
	}
	if entry["error"] != "boom" {
		t.Fatalf("expected error 'boom', got %v", entry["error"])
	}
}

func TestLogError_NilError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := contextWithLogger(context.Background(), logger)

	LogError(ctx, "no error", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, ok := entry["error"]; ok {
		t.Fatal("expected no error attribute when err is nil")
	}
}

func TestContextWithLogger_NilContext(t *testing.T) {
	logger := slog.Default()
	ctx := contextWithLogger(context.TODO(), logger)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	l := LoggerFromContext(ctx)
	if l != logger {
		t.Fatal("expected logger from context")
	}
}

func TestContextWithTraceID_NilContext(t *testing.T) {
	ctx := contextWithTraceID(context.TODO(), "test-id")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	id := TraceIDFromContext(ctx)
	if id == nil || *id != "test-id" {
		t.Fatalf("expected 'test-id', got %v", id)
	}
}

type errForTest string

func (e errForTest) Error() string { return string(e) }
