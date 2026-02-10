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

func TestBuildLinkHeader_URLEncoding(t *testing.T) {
	q := url.Values{"filter": {"hello world"}}
	link := BuildLinkHeader("/items", q, "next", "")

	if !strings.Contains(link, "filter=hello+world") && !strings.Contains(link, "filter=hello%20world") {
		t.Errorf("filter param should be URL encoded, got %q", link)
	}
}

func TestBuildLinkHeader_MultipleQueryValues(t *testing.T) {
	q := url.Values{"tag": {"a", "b", "c"}}
	link := BuildLinkHeader("/items", q, "next", "")

	if !strings.Contains(link, "tag=a") || !strings.Contains(link, "tag=b") || !strings.Contains(link, "tag=c") {
		t.Errorf("all tag values should be present, got %q", link)
	}
}

func TestBuildLinkHeader_CursorWithSpecialChars(t *testing.T) {
	cursor := Cursor{Type: "item", Value: "abc/def+ghi=jkl"}.Encode()
	link := BuildLinkHeader("/items", nil, cursor, "")

	if !strings.Contains(link, "cursor=") {
		t.Error("cursor param should be present")
	}
}

func TestBuildLinkHeader_ReplacesExistingCursor(t *testing.T) {
	q := url.Values{"cursor": {"old-cursor"}, "limit": {"10"}}
	link := BuildLinkHeader("/items", q, "new-cursor", "")

	if strings.Contains(link, "old-cursor") {
		t.Error("old cursor should be replaced")
	}
	if !strings.Contains(link, "cursor=new-cursor") {
		t.Error("new cursor should be present")
	}
	if !strings.Contains(link, "limit=10") {
		t.Error("other params should be preserved")
	}
}

func TestBuildLinkHeader_EmptyBaseURL(t *testing.T) {
	link := BuildLinkHeader("", nil, "next", "")

	if !strings.Contains(link, "<?cursor=next>") {
		t.Errorf("should handle empty base URL, got %q", link)
	}
}

func TestBuildLinkHeader_RelativePath(t *testing.T) {
	link := BuildLinkHeader("/items", nil, "next", "")

	if !strings.Contains(link, "</items?cursor=next>") {
		t.Errorf("should handle relative path, got %q", link)
	}
}
