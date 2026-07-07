package httpheader

import (
	"net/http"
	"strings"
	"testing"
)

func TestAddVaryAddsValues(t *testing.T) {
	h := make(http.Header)
	AddVary(h, "Origin", "Accept")

	set := headerSet(h.Values("Vary"))
	if _, ok := set["Origin"]; !ok {
		t.Fatal("expected Vary to contain Origin")
	}
	if _, ok := set["Accept"]; !ok {
		t.Fatal("expected Vary to contain Accept")
	}
}

func TestAddVaryNoDuplicates(t *testing.T) {
	h := make(http.Header)
	h.Add("Vary", "accept")
	AddVary(h, "Accept", "Origin")

	if count := countInHeader(h.Values("Vary"), "Accept"); count != 1 {
		t.Fatalf("expected Accept once, got %d", count)
	}
}

func TestAddVaryMergesCommaSeparated(t *testing.T) {
	h := make(http.Header)
	h.Set("Vary", "Accept-Encoding, Accept-Language")
	AddVary(h, "Origin", "Accept")

	set := headerSet(h.Values("Vary"))
	for _, v := range []string{"Accept-Encoding", "Accept-Language", "Origin", "Accept"} {
		if _, ok := set[v]; !ok {
			t.Fatalf("expected Vary to contain %q", v)
		}
	}
}

func TestAddVaryEmptyInput(t *testing.T) {
	h := make(http.Header)
	AddVary(h)

	if len(h.Values("Vary")) != 0 {
		t.Fatalf("expected no Vary header, got %v", h.Values("Vary"))
	}
}

func TestAddVaryDuplicateInSingleCall(t *testing.T) {
	h := make(http.Header)
	AddVary(h, "Accept", "Accept", "Origin")

	if count := countInHeader(h.Values("Vary"), "Accept"); count != 1 {
		t.Fatalf("expected Accept once, got %d", count)
	}
}

func TestAddVaryWildcardPreserved(t *testing.T) {
	h := make(http.Header)
	h.Set("Vary", "*")
	AddVary(h, "Accept")

	if got := h.Get("Vary"); got != "*" {
		t.Fatalf("expected Vary wildcard to be preserved, got %q", got)
	}
}

func headerSet(values []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			value := strings.TrimSpace(part)
			if value != "" {
				set[value] = struct{}{}
			}
		}
	}
	return set
}

func countInHeader(values []string, target string) int {
	count := 0
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), target) {
				count++
			}
		}
	}
	return count
}
