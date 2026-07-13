package request

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestRejectUnknownQuery(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{name: "none", target: "/items"},
		{name: "allowed", target: "/items?cursor=a&limit=5&category=tools"},
		{name: "repeated allowed", target: "/items?limit=5&limit=10"},
		{name: "unknown", target: "/items?limti=5", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.target, nil)
			c := e.NewContext(req, httptest.NewRecorder())

			err := RejectUnknownQuery(c, "cursor", "limit", "category")
			if (err != nil) != tt.wantErr {
				t.Fatalf("RejectUnknownQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
