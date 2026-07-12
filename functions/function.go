// Package hello provides an HTTP Cloud Run function example.
package hello

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
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

	var req *Request
	if r.Method == http.MethodPost {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid JSON request body", http.StatusBadRequest)
			return
		}
		if req == nil {
			http.Error(w, "request body must contain one JSON object", http.StatusBadRequest)
			return
		}
		if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			http.Error(w, "request body must contain one JSON object", http.StatusBadRequest)
			return
		}
	}

	var name string
	if req != nil {
		name = req.Name
	}
	if name == "" {
		name = r.URL.Query().Get("name")
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
