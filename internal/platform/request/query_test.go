package request

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/labstack/echo/v5"
)

func TestRejectUnknownOrRepeatedQuery(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		rawQuery    string
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
		{
			name:        "invalid percent encoding",
			target:      "/items",
			rawQuery:    "limit=%zz",
			wantMessage: "malformed query string",
		},
		{
			name:        "unescaped semicolon",
			target:      "/items",
			rawQuery:    "limit=5;category=tools",
			wantMessage: "malformed query string",
		},
		{
			name:        "invalid UTF-8 value",
			target:      "/items",
			rawQuery:    "category=%FF",
			wantMessage: `query parameter "category" must contain valid UTF-8`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.target, nil)
			if tt.rawQuery != "" {
				req.URL.RawQuery = tt.rawQuery
			}
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

func FuzzRejectUnknownOrRepeatedQuery(f *testing.F) {
	f.Add("cursor=a&limit=5&category=tools")
	f.Add("limit=5&limit=10")
	f.Add("limit=%zz")
	f.Add("limti=5")
	f.Add("category=%FF")

	f.Fuzz(func(t *testing.T, rawQuery string) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
		req.URL.RawQuery = rawQuery
		c := echo.New().NewContext(req, httptest.NewRecorder())

		err := RejectUnknownOrRepeatedQuery(c, "cursor", "limit", "category")
		values, parseErr := url.ParseQuery(rawQuery)
		valid := parseErr == nil
		if valid {
			for name, entries := range values {
				if name != "cursor" && name != "limit" && name != "category" || len(entries) != 1 {
					valid = false
					break
				}
				if !utf8.ValidString(entries[0]) {
					valid = false
					break
				}
			}
		}
		if valid && err != nil {
			t.Fatalf("valid query %q rejected: %v", rawQuery, err)
		}
		if !valid && echo.StatusCode(err) != http.StatusBadRequest {
			t.Fatalf("invalid query %q status = %d, want 400", rawQuery, echo.StatusCode(err))
		}
	})
}
