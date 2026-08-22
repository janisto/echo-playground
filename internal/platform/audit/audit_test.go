package audit

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/janisto/echo-observability/v2"
	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogEventUsesRequestLogger(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	e := echo.New()
	e.Use(obs.RequestContext(obs.RequestContextConfig{Logger: logger}))
	e.POST("/audit", func(c *echo.Context) error {
		LogEvent(c.Request().Context(), "create", "profile", "success",
			map[string]any{"field": "value"})
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/audit", nil)
	req.Header.Set(echo.HeaderXRequestID, "audit-req")
	resp := httptest.NewRecorder()
	e.ServeHTTP(resp, req)

	entries := recorded.FilterMessage("Audit event").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	assertAuditLogField(t, fields, "request_id", "audit-req")
	assertAuditLogField(t, fields, "audit.action", "create")
	assertAuditLogField(t, fields, "audit.resource_type", "profile")
	assertAuditLogField(t, fields, "audit.result", "success")
	if _, present := fields["audit.user_id"]; present {
		t.Fatal("audit log must not contain a principal identifier")
	}
	if _, present := fields["audit.resource_id"]; present {
		t.Fatal("audit log must not contain a profile identifier")
	}
	if got := fields["audit.details"]; !reflect.DeepEqual(got, map[string]any{"field": "value"}) {
		t.Fatalf("expected audit details, got %#v", got)
	}
}

func assertAuditLogField(t *testing.T, fields map[string]any, key string, want any) {
	t.Helper()
	if got := fields[key]; got != want {
		t.Fatalf("expected log field %s=%v, got %v in %#v", key, want, got, fields)
	}
}
