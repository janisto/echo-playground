package request

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/respond"
)

type decodeInput struct {
	Name string `json:"name" cbor:"name"`
}

func TestDecodeJSONAndCBOR(t *testing.T) {
	cborBody, err := cbor.Marshal(map[string]any{"name": "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		contentType []string
		encoding    []string
		body        []byte
		wantStatus  int
		wantName    string
	}{
		{name: "JSON", contentType: []string{"application/json"}, body: []byte(`{"name":"Ada"}`), wantName: "Ada"},
		{
			name:        "JSON charset",
			contentType: []string{"Application/JSON; Charset=UTF-8"},
			body:        []byte(`{"name":"Ada"}`),
			wantName:    "Ada",
		},
		{name: "CBOR", contentType: []string{"application/cbor"}, body: cborBody, wantName: "Ada"},
		{
			name:        "identity",
			contentType: []string{"application/json"},
			encoding:    []string{"identity"},
			body:        []byte(`{"name":"Ada"}`),
			wantName:    "Ada",
		},
		{name: "empty supported", contentType: []string{"application/json"}, wantStatus: 400},
		{name: "empty missing", wantStatus: 400},
		{name: "empty unsupported", contentType: []string{"text/plain"}, wantStatus: 415},
		{name: "nonempty missing", body: []byte(`{}`), wantStatus: 415},
		{
			name:        "repeated media",
			contentType: []string{"application/json", "application/json"},
			body:        []byte(`{}`),
			wantStatus:  415,
		},
		{
			name:        "comma media",
			contentType: []string{"application/json, application/cbor"},
			body:        []byte(`{}`),
			wantStatus:  415,
		},
		{
			name:        "unsupported parameter",
			contentType: []string{"application/json; profile=x"},
			body:        []byte(`{}`),
			wantStatus:  415,
		},
		{
			name:        "CBOR parameter",
			contentType: []string{"application/cbor; charset=utf-8"},
			body:        cborBody,
			wantStatus:  415,
		},
		{
			name:        "gzip",
			contentType: []string{"application/json"},
			encoding:    []string{"gzip"},
			body:        []byte(`{}`),
			wantStatus:  415,
		},
		{
			name:        "repeated coding",
			contentType: []string{"application/json"},
			encoding:    []string{"identity", "identity"},
			body:        []byte(`{}`),
			wantStatus:  415,
		},
		{name: "malformed JSON", contentType: []string{"application/json"}, body: []byte(`{`), wantStatus: 400},
		{
			name:        "duplicate JSON",
			contentType: []string{"application/json"},
			body:        []byte(`{"name":"Ada","name":"Grace"}`),
			wantStatus:  400,
		},
		{
			name:        "trailing JSON",
			contentType: []string{"application/json"},
			body:        []byte(`{"name":"Ada"} {}`),
			wantStatus:  400,
		},
		{
			name:        "JSON BOM",
			contentType: []string{"application/json"},
			body:        append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"name":"Ada"}`)...),
			wantStatus:  400,
		},
		{
			name:        "invalid UTF-8",
			contentType: []string{"application/json"},
			body:        []byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'},
			wantStatus:  400,
		},
		{
			name:        "unknown JSON member",
			contentType: []string{"application/json"},
			body:        []byte(`{"other":1}`),
			wantStatus:  422,
		},
		{
			name:        "wrong JSON type",
			contentType: []string{"application/json"},
			body:        []byte(`{"name":1}`),
			wantStatus:  422,
		},
		{name: "wrong top-level kind", contentType: []string{"application/json"}, body: []byte(`[]`), wantStatus: 422},
		{
			name:        "duplicate CBOR key",
			contentType: []string{"application/cbor"},
			body: []byte{
				0xa2,
				0x64,
				'n',
				'a',
				'm',
				'e',
				0x63,
				'A',
				'd',
				'a',
				0x64,
				'n',
				'a',
				'm',
				'e',
				0x65,
				'G',
				'r',
				'a',
				'c',
				'e',
			},
			wantStatus: 400,
		},
		{
			name:        "trailing CBOR",
			contentType: []string{"application/cbor"},
			body:        append(append([]byte(nil), cborBody...), 0xa0),
			wantStatus:  400,
		},
		{
			name:        "unknown CBOR member",
			contentType: []string{"application/cbor"},
			body:        []byte{0xa1, 0x65, 'o', 't', 'h', 'e', 'r', 0x01},
			wantStatus:  422,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(test.body))
			for _, value := range test.contentType {
				request.Header.Add("Content-Type", value)
			}
			for _, value := range test.encoding {
				request.Header.Add("Content-Encoding", value)
			}
			context := echo.New().NewContext(request, httptest.NewRecorder())
			var input decodeInput
			err := Decode(context, &input)
			if test.wantStatus == 0 {
				if err != nil || input.Name != test.wantName {
					t.Fatalf("Decode = %#v, %v", input, err)
				}
				return
			}
			if status := echo.StatusCode(err); status != test.wantStatus {
				t.Fatalf("status = %d, want %d, err=%v", status, test.wantStatus, err)
			}
		})
	}
}

func TestBodyLimitAppliesOnlyToPortableWriteOperations(t *testing.T) {
	for _, test := range []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodPost, "/v1/hello", 413},
		{http.MethodPost, "/v1/profile", 413},
		{http.MethodPatch, "/v1/profile", 413},
		{http.MethodGet, "/v1/hello", 204},
		{http.MethodPost, "/unmatched", 204},
		{http.MethodPut, "/v1/profile", 204},
	} {
		e := echo.New()
		e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
		e.Use(BodyLimitMiddleware())
		e.Any("/*", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })
		req := httptest.NewRequestWithContext(t.Context(), test.method, test.path, strings.NewReader("x"))
		req.ContentLength = BodyLimit + 1
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != test.want {
			t.Fatalf("%s %s status = %d, want %d", test.method, test.path, rec.Code, test.want)
		}
	}
}

func FuzzDecodeJSON(f *testing.F) {
	f.Add([]byte(`{"name":"Ada"}`), "application/json")
	f.Add([]byte{0xa1, 0x64, 'n', 'a', 'm', 'e', 0x63, 'A', 'd', 'a'}, "application/cbor")
	f.Add([]byte(`{`), "application/json")
	f.Fuzz(func(t *testing.T, body []byte, contentType string) {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(body))
		request.Header.Set("Content-Type", contentType)
		context := echo.New().NewContext(request, httptest.NewRecorder())
		var input decodeInput
		err := Decode(context, &input)
		if err == nil {
			return
		}
		switch echo.StatusCode(err) {
		case 400, 413, 415, 422:
		default:
			t.Fatalf("unexpected status %d for %q", echo.StatusCode(err), contentType)
		}
	})
}
