// Package hello provides an HTTP Cloud Run function example.
package hello

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"
	"unicode/utf8"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

// RFC3339Millis matches the main project's timestamp format.
const RFC3339Millis = "2006-01-02T15:04:05.000Z"

func init() {
	functions.HTTP("Hello", helloHandler)
}

// Request represents the optional request body.
type Request struct {
	Name string `json:"name"`
}

// Response represents the function response.
type Response struct {
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "invalid query string", http.StatusBadRequest)
		return
	}
	for name, values := range query {
		if name != "name" || len(values) != 1 || !utf8.ValidString(values[0]) {
			http.Error(w, "invalid query string", http.StatusBadRequest)
			return
		}
	}

	var req *Request
	if r.Method == http.MethodPost {
		contentTypes := r.Header.Values("Content-Type")
		if len(contentTypes) != 1 {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		mediaType, _, err := mime.ParseMediaType(contentTypes[0])
		if err != nil || mediaType != "application/json" {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		dec := json.NewDecoder(r.Body)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			writeDecodeError(w, err)
			return
		}
		if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeDecodeError(w, err)
			return
		}
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || raw[0] != '{' || !utf8.Valid(raw) {
			http.Error(w, "request body must contain one JSON object", http.StatusBadRequest)
			return
		}
		if err := validateRequestJSON(raw); err != nil {
			http.Error(w, "invalid JSON request body", http.StatusBadRequest)
			return
		}
		req = &Request{}
		if err := json.Unmarshal(raw, req); err != nil {
			http.Error(w, "invalid JSON request body", http.StatusBadRequest)
			return
		}
	}

	var name string
	if req != nil {
		name = req.Name
	}
	if name == "" {
		name = query.Get("name")
	}
	if name == "" {
		name = "World"
	}
	if utf8.RuneCountInString(name) > 100 {
		http.Error(w, "name must be at most 100 characters", http.StatusBadRequest)
		return
	}

	resp := Response{
		Message:   "Hello, " + name + "!",
		Timestamp: time.Now().UTC().Format(RFC3339Millis),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return
	}
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "invalid JSON request body", http.StatusBadRequest)
}

func validateRequestJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return errors.New("request body is not a JSON object")
	}

	seen := make(map[string]struct{})
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return fmt.Errorf("read JSON object name: %w", err)
		}
		name, ok := token.(string)
		if !ok || name != "name" {
			return errors.New("unknown JSON field")
		}
		if _, exists := seen[name]; exists {
			return errors.New("duplicate JSON field")
		}
		seen[name] = struct{}{}

		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return fmt.Errorf("read JSON field value: %w", err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("close JSON object: %w", err)
	}
	return nil
}
