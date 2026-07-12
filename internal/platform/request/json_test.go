package request

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

type jsonInput struct {
	Name string `json:"name"`
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
		{
			name:        "unknown field",
			contentType: "application/json",
			body:        `{"other":true}`,
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
