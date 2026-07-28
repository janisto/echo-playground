package request

import (
	"bytes"
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/labstack/echo/v5"
)

type jsonInput struct {
	Name string `json:"name"`
}

type nestedJSONInput struct {
	Children []jsonInput `json:"children"`
}

func FuzzDecodeJSON(f *testing.F) {
	f.Add([]byte(`{"name":"Ada"}`), "application/json")
	f.Add([]byte(`{"name":null}`), "application/json; charset=utf-8")
	f.Add([]byte(`{"other":true}`), "application/json")
	f.Add([]byte(`{} {}`), "application/json")
	f.Add([]byte(`[]`), "application/json")
	f.Add([]byte(`{}`), "application/cbor")
	f.Add([]byte{}, "application/json")

	f.Fuzz(func(t *testing.T, body []byte, contentType string) {
		e := echo.New()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, contentType)
		c := e.NewContext(req, httptest.NewRecorder())

		var got jsonInput
		err := DecodeJSON(c, &got)

		if len(body) == 0 {
			if status := echo.StatusCode(err); status != http.StatusBadRequest {
				t.Fatalf("empty body status = %d, want %d", status, http.StatusBadRequest)
			}
			return
		}

		mediaType, _, mediaErr := mime.ParseMediaType(contentType)
		if mediaErr != nil || mediaType != echo.MIMEApplicationJSON {
			if status := echo.StatusCode(err); status != http.StatusUnsupportedMediaType {
				t.Fatalf(
					"unsupported content type %q status = %d, want %d",
					contentType,
					status,
					http.StatusUnsupportedMediaType,
				)
			}
			return
		}

		want, valid := validJSONInput(body)
		if !valid {
			if status := echo.StatusCode(err); status != http.StatusBadRequest {
				t.Fatalf("invalid JSON object %q status = %d, want %d", body, status, http.StatusBadRequest)
			}
			return
		}
		if err != nil {
			t.Fatalf("valid JSON object %q was rejected: %v", body, err)
		}
		if got != want {
			t.Fatalf("decoded input = %#v, want %#v", got, want)
		}
	})
}

func validJSONInput(data []byte) (jsonInput, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' || !utf8.Valid(trimmed) || !json.Valid(trimmed) {
		return jsonInput{}, false
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return jsonInput{}, false
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return jsonInput{}, false
		}
		field, ok := token.(string)
		if !ok || field != "name" {
			return jsonInput{}, false
		}
		if _, exists := seen[field]; exists {
			return jsonInput{}, false
		}
		seen[field] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return jsonInput{}, false
		}
	}

	var input jsonInput
	if err := json.Unmarshal(trimmed, &input); err != nil {
		return jsonInput{}, false
	}
	return input, true
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{name: "valid", contentType: "application/json; charset=utf-8", body: `{"name":"Ada"}`},
		{name: "empty", contentType: "application/json", wantStatus: http.StatusBadRequest},
		{name: "malformed", contentType: "application/json", body: `{`, wantStatus: http.StatusBadRequest},
		{name: "null", contentType: "application/json", body: `null`, wantStatus: http.StatusBadRequest},
		{name: "array", contentType: "application/json", body: `[]`, wantStatus: http.StatusBadRequest},
		{name: "string", contentType: "application/json", body: `"Ada"`, wantStatus: http.StatusBadRequest},
		{name: "number", contentType: "application/json", body: `42`, wantStatus: http.StatusBadRequest},
		{name: "boolean", contentType: "application/json", body: `true`, wantStatus: http.StatusBadRequest},
		{
			name:        "unknown field",
			contentType: "application/json",
			body:        `{"other":true}`,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "case variant field",
			contentType: "application/json",
			body:        `{"Name":"Ada"}`,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "duplicate field",
			contentType: "application/json",
			body:        `{"name":"Ada","name":"Grace"}`,
			wantStatus:  http.StatusBadRequest,
		},
		{name: "trailing object", contentType: "application/json", body: `{} {}`, wantStatus: http.StatusBadRequest},
		{
			name:        "wrong content type",
			contentType: "application/cbor",
			body:        `{}`,
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{name: "missing content type", body: `{}`, wantStatus: http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/",
				strings.NewReader(tt.body),
			)
			req.Header.Set(echo.HeaderContentType, tt.contentType)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			var input jsonInput
			err := DecodeJSON(c, &input)
			if tt.wantStatus == 0 {
				if err != nil {
					t.Fatalf("decode: %v", err)
				}
				if input.Name != "Ada" {
					t.Fatalf("expected Ada, got %q", input.Name)
				}
				return
			}
			if got := echo.StatusCode(err); got != tt.wantStatus {
				t.Fatalf("expected HTTP %d, got %v", tt.wantStatus, err)
			}
		})
	}
}

func TestDecodeJSONRejectsInvalidUTF8AndRepeatedContentType(t *testing.T) {
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
			e := echo.New()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(tt.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			if tt.addHeader {
				req.Header.Add(echo.HeaderContentType, echo.MIMEApplicationJSON)
			}
			c := e.NewContext(req, httptest.NewRecorder())

			var input jsonInput
			if got := echo.StatusCode(DecodeJSON(c, &input)); got != tt.wantStatus {
				t.Fatalf("status = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}

func TestDecodeJSONRejectsNestedAliasesAndDuplicates(t *testing.T) {
	for _, body := range []string{
		`{"children":[{"Name":"Ada"}]}`,
		`{"children":[{"name":"Ada","name":"Grace"}]}`,
		`{"children":[{"unknown":"Ada"}]}`,
	} {
		e := echo.New()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		c := e.NewContext(req, httptest.NewRecorder())

		var input nestedJSONInput
		if got := echo.StatusCode(DecodeJSON(c, &input)); got != http.StatusBadRequest {
			t.Fatalf("body %q status = %d, want %d", body, got, http.StatusBadRequest)
		}
	}

	e := echo.New()
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/",
		strings.NewReader(`{"children":[{"name":"Ada"}]}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req, httptest.NewRecorder())
	var input nestedJSONInput
	if err := DecodeJSON(c, &input); err != nil {
		t.Fatalf("valid nested request rejected: %v", err)
	}
	if len(input.Children) != 1 || input.Children[0].Name != "Ada" {
		t.Fatalf("decoded nested input = %#v", input)
	}
}
