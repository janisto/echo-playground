package hello

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHelloHandler(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		target      string
		body        string
		contentType string
		wantStatus  int
		want        string
	}{
		{name: "default", method: http.MethodGet, target: "/", wantStatus: http.StatusOK, want: "Hello, World!"},
		{name: "query", method: http.MethodGet, target: "/?name=Ada", wantStatus: http.StatusOK, want: "Hello, Ada!"},
		{
			name:        "JSON",
			method:      http.MethodPost,
			target:      "/",
			body:        `{"name":"Grace"}`,
			contentType: "application/json; charset=utf-8",
			wantStatus:  http.StatusOK,
			want:        "Hello, Grace!",
		},
		{
			name: "malformed JSON", method: http.MethodPost, target: "/", body: `{`,
			contentType: "application/json", wantStatus: http.StatusBadRequest,
		},
		{
			name: "null", method: http.MethodPost, target: "/", body: `null`,
			contentType: "application/json", wantStatus: http.StatusBadRequest,
		},
		{
			name:        "unknown field",
			method:      http.MethodPost,
			target:      "/",
			body:        `{"unknown":true}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "multiple values",
			method:      http.MethodPost,
			target:      "/",
			body:        `{} {}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:       "name too long",
			method:     http.MethodGet,
			target:     "/?name=" + strings.Repeat("a", 101),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "body too large",
			method:      http.MethodPost,
			target:      "/",
			body:        "{\"name\":\"" + strings.Repeat("a", 1<<20) + "\"}",
			contentType: "application/json",
			wantStatus:  http.StatusRequestEntityTooLarge,
		},
		{
			name: "missing content type", method: http.MethodPost, target: "/", body: `{}`,
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name: "unsupported content type", method: http.MethodPost, target: "/", body: `{}`,
			contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType,
		},
		{name: "unsupported method", method: http.MethodDelete, target: "/", wantStatus: http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.target, strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()

			helloHandler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected application/json, got %q", got)
			}
			var response Response
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Message != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, response.Message)
			}
			if _, err := time.Parse(RFC3339Millis, response.Timestamp); err != nil {
				t.Fatalf("invalid timestamp %q: %v", response.Timestamp, err)
			}
		})
	}
}
