package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestLogAuditEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := contextWithLogger(context.Background(), logger)

	LogAuditEvent(ctx, "create", "user-123", "profile", "profile-123", "success", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if entry["msg"] != "Audit event" {
		t.Fatalf("expected message 'Audit event', got %q", entry["msg"])
	}
	if entry["audit.action"] != "create" {
		t.Fatalf("expected audit.action 'create', got %q", entry["audit.action"])
	}
	if entry["audit.user_id"] != "user-123" {
		t.Fatalf("expected audit.user_id 'user-123', got %q", entry["audit.user_id"])
	}
	if entry["audit.resource_type"] != "profile" {
		t.Fatalf("expected audit.resource_type 'profile', got %q", entry["audit.resource_type"])
	}
	if entry["audit.resource_id"] != "profile-123" {
		t.Fatalf("expected audit.resource_id 'profile-123', got %q", entry["audit.resource_id"])
	}
	if entry["audit.result"] != "success" {
		t.Fatalf("expected audit.result 'success', got %q", entry["audit.result"])
	}
}

func TestLogAuditEvent_WithDetails(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := contextWithLogger(context.Background(), logger)

	details := map[string]any{"error": "not_found"}
	LogAuditEvent(ctx, "delete", "user-456", "profile", "profile-456", "failure", details)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if entry["audit.result"] != "failure" {
		t.Fatalf("expected audit.result 'failure', got %q", entry["audit.result"])
	}
	auditDetails, ok := entry["audit.details"].(map[string]any)
	if !ok {
		t.Fatal("expected audit.details to be a map")
	}
	if auditDetails["error"] != "not_found" {
		t.Fatalf("expected error 'not_found', got %v", auditDetails["error"])
	}
}
