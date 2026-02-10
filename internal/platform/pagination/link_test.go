package pagination

import (
	"net/url"
	"strings"
	"testing"
)

func TestBuildLinkHeader_NextOnly(t *testing.T) {
	q := url.Values{"limit": {"10"}}
	link := BuildLinkHeader("/items", q, "next-cursor", "")
	if !strings.Contains(link, `rel="next"`) {
		t.Fatalf("expected next link, got %q", link)
	}
	if strings.Contains(link, `rel="prev"`) {
		t.Fatal("expected no prev link")
	}
	if !strings.Contains(link, "cursor=next-cursor") {
		t.Fatalf("expected cursor param, got %q", link)
	}
}

func TestBuildLinkHeader_PrevOnly(t *testing.T) {
	q := url.Values{"limit": {"10"}}
	link := BuildLinkHeader("/items", q, "", "prev-cursor")
	if strings.Contains(link, `rel="next"`) {
		t.Fatal("expected no next link")
	}
	if !strings.Contains(link, `rel="prev"`) {
		t.Fatalf("expected prev link, got %q", link)
	}
}

func TestBuildLinkHeader_Both(t *testing.T) {
	q := url.Values{"limit": {"10"}}
	link := BuildLinkHeader("/items", q, "next-cursor", "prev-cursor")
	if !strings.Contains(link, `rel="next"`) {
		t.Fatalf("expected next link, got %q", link)
	}
	if !strings.Contains(link, `rel="prev"`) {
		t.Fatalf("expected prev link, got %q", link)
	}
}

func TestBuildLinkHeader_Empty(t *testing.T) {
	link := BuildLinkHeader("/items", nil, "", "")
	if link != "" {
		t.Fatalf("expected empty link, got %q", link)
	}
}

func TestBuildLinkHeader_PreservesQuery(t *testing.T) {
	q := url.Values{"category": {"electronics"}, "limit": {"5"}}
	link := BuildLinkHeader("/items", q, "abc", "")
	if !strings.Contains(link, "category=electronics") {
		t.Fatalf("expected preserved query param, got %q", link)
	}
	if !strings.Contains(link, "limit=5") {
		t.Fatalf("expected preserved limit param, got %q", link)
	}
}
