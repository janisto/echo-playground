package request

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"unicode/utf8"

	"github.com/labstack/echo/v5"
)

func TestParseQueryClosedScalarBoundary(t *testing.T) {
	tests := []struct {
		rawQuery string
		valid    bool
	}{
		{"", true},
		{"cursor=a&limit=5&category=tools", true},
		{"limit=5&limit=10", false},
		{"unknown=1", false},
		{"limit=%zz", false},
		{"limit=5;category=tools", false},
		{"category=%FF", false},
		{"%FF=value", false},
	}
	for _, test := range tests {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
		req.URL.RawQuery = test.rawQuery
		context := echo.New().NewContext(req, httptest.NewRecorder())
		_, err := ParseQuery(context, "cursor", "limit", "category")
		if test.valid && err != nil {
			t.Fatalf("valid query %q: %v", test.rawQuery, err)
		}
		if !test.valid && echo.StatusCode(err) != 400 {
			t.Fatalf("invalid query %q status = %d", test.rawQuery, echo.StatusCode(err))
		}
	}
}

func TestLimitSyntaxAndBounds(t *testing.T) {
	tests := []struct {
		value string
		want  int
		valid bool
	}{
		{"", 0, false},
		{"1", 1, true},
		{"01", 1, true},
		{"20", 20, true},
		{"100", 100, true},
		{"0", 0, false},
		{"101", 0, false},
		{"+1", 0, false},
		{"1.0", 0, false},
		{" 1", 0, false},
		{"18446744073709551616", 0, false},
	}
	for _, test := range tests {
		value, err := Limit(url.Values{"limit": {test.value}})
		if test.valid && (err != nil || value != test.want) {
			t.Fatalf("Limit(%q) = %d, %v", test.value, value, err)
		}
		if !test.valid && echo.StatusCode(err) != 422 {
			t.Fatalf("Limit(%q) status = %d", test.value, echo.StatusCode(err))
		}
	}
	if value, err := Limit(url.Values{}); err != nil || value != 20 {
		t.Fatalf("default Limit = %d, %v", value, err)
	}
}

func FuzzRejectUnknownOrRepeatedQuery(f *testing.F) {
	f.Add("cursor=a&limit=5&category=tools")
	f.Add("limit=5&limit=10")
	f.Add("category=%FF")
	f.Fuzz(func(t *testing.T, rawQuery string) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
		req.URL.RawQuery = rawQuery
		context := echo.New().NewContext(req, httptest.NewRecorder())
		err := RejectUnknownOrRepeatedQuery(context, "cursor", "limit", "category")
		values, parseErr := url.ParseQuery(rawQuery)
		valid := parseErr == nil
		for name, entries := range values {
			if name != "cursor" && name != "limit" && name != "category" || len(entries) != 1 ||
				!utf8.ValidString(name) ||
				!utf8.ValidString(entries[0]) {
				valid = false
			}
		}
		if valid && err != nil || !valid && echo.StatusCode(err) != 400 {
			t.Fatalf("query %q valid=%t err=%v", rawQuery, valid, err)
		}
	})
}
