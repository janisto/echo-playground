package request

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestRejectUnknownOrRepeatedQuery(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		wantMessage string
	}{
		{name: "none", target: "/items"},
		{name: "allowed", target: "/items?cursor=a&limit=5&category=tools"},
		{
			name:        "repeated allowed name",
			target:      "/items?limit=5&limit=10",
			wantMessage: `query parameter "limit" must appear exactly once`,
		},
		{name: "unknown", target: "/items?limti=5", wantMessage: `unknown query parameter "limti"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.target, nil)
			c := e.NewContext(req, httptest.NewRecorder())

			err := RejectUnknownOrRepeatedQuery(c, "cursor", "limit", "category")
			if tt.wantMessage == "" {
				if err != nil {
					t.Fatalf("RejectUnknownOrRepeatedQuery() unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("RejectUnknownOrRepeatedQuery() error = %v, want message %q", err, tt.wantMessage)
			}
		})
	}
}
