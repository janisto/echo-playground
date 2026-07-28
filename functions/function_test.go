package hello

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func FuzzHelloHandler(f *testing.F) {
	f.Add("", "", false)
	f.Add("Grace", "Ada", true)
	f.Add("", "Ada", true)
	f.Add(strings.Repeat("a", 100), "", false)
	f.Add(strings.Repeat("a", 101), "", false)
	f.Add(strings.Repeat("🙂", 100), "", true)
	f.Add(strings.Repeat("🙂", 101), "", true)

	f.Fuzz(func(t *testing.T, bodyName, queryName string, post bool) {
		if !utf8.ValidString(bodyName) || !utf8.ValidString(queryName) {
			return
		}

		method := http.MethodGet
		target := "/?" + url.Values{"name": {queryName}}.Encode()
		var body []byte
		if post {
			method = http.MethodPost
			var err error
			body, err = json.Marshal(Request{Name: bodyName})
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
		}

		req := httptest.NewRequestWithContext(t.Context(), method, target, bytes.NewReader(body))
		if post {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		helloHandler(rec, req)

		if post && len(body) > 1<<20 {
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("oversized body status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
			}
			return
		}

		name := queryName
		if post && bodyName != "" {
			name = bodyName
		}
		if name == "" {
			name = "World"
		}
		if utf8.RuneCountInString(name) > 100 {
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("overlong name status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			return
		}

		if rec.Code != http.StatusOK {
			t.Fatalf("valid name status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type = %q, want application/json", got)
		}
		var response Response
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if want := "Hello, " + name + "!"; response.Message != want {
			t.Fatalf("message = %q, want %q", response.Message, want)
		}
		if _, err := time.Parse(RFC3339Millis, response.Timestamp); err != nil {
			t.Fatalf("invalid timestamp %q: %v", response.Timestamp, err)
		}
	})
}

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
			name: "repeated query", method: http.MethodGet, target: "/?name=Ada&name=Grace",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "unknown query", method: http.MethodGet, target: "/?other=Ada",
			wantStatus: http.StatusBadRequest,
		},
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
			name: "array", method: http.MethodPost, target: "/", body: `[]`,
			contentType: "application/json", wantStatus: http.StatusBadRequest,
		},
		{
			name: "empty body", method: http.MethodPost, target: "/",
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
			name:        "case variant field",
			method:      http.MethodPost,
			target:      "/",
			body:        `{"Name":"Grace"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "duplicate field",
			method:      http.MethodPost,
			target:      "/",
			body:        `{"name":"Ada","name":"Grace"}`,
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
			name:       "name at limit",
			method:     http.MethodGet,
			target:     "/?name=" + strings.Repeat("a", 100),
			wantStatus: http.StatusOK,
			want:       "Hello, " + strings.Repeat("a", 100) + "!",
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
			name:        "oversized trailing data",
			method:      http.MethodPost,
			target:      "/",
			body:        `{}` + strings.Repeat(" ", 1<<20),
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

func TestHelloHandlerRejectsMalformedQuery(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.URL.RawQuery = "name=%zz"
	rec := httptest.NewRecorder()

	helloHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHelloHandlerRejectsInvalidUTF8AndRepeatedContentType(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		addHeader  bool
		wantStatus int
	}{
		{
			name:       "invalid UTF-8",
			body:       []byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "repeated content type",
			body:       []byte(`{"name":"Ada"}`),
			addHeader:  true,
			wantStatus: http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.addHeader {
				req.Header.Add("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()

			helloHandler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}
