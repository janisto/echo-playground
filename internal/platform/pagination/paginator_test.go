package pagination

import (
	"net/url"
	"testing"
)

type testItem struct {
	ID   string
	Name string
}

func makeItems(n int) []testItem {
	items := make([]testItem, n)
	for i := range n {
		items[i] = testItem{ID: string(rune('a' + i)), Name: "item-" + string(rune('a'+i))}
	}
	return items
}

func getTestID(item testItem) string { return item.ID }

func TestPaginate_FirstPage(t *testing.T) {
	items := makeItems(10)
	result := Paginate(items, Cursor{}, 3, "item", getTestID, "/items", nil)
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}
	if result.Total != 10 {
		t.Fatalf("expected total 10, got %d", result.Total)
	}
	if result.NextCursor == "" {
		t.Fatal("expected next cursor")
	}
	if result.PrevCursor != "" {
		t.Fatalf("expected no prev cursor, got %q", result.PrevCursor)
	}
}

func TestPaginate_SecondPage(t *testing.T) {
	items := makeItems(10)
	first := Paginate(items, Cursor{}, 3, "item", getTestID, "/items", nil)
	cursor, err := DecodeCursor(first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	second := Paginate(items, cursor, 3, "item", getTestID, "/items", nil)
	if len(second.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(second.Items))
	}
	if second.Items[0].ID != "d" {
		t.Fatalf("expected first item 'd', got %q", second.Items[0].ID)
	}
	if second.PrevCursor == "" {
		t.Fatal("expected prev cursor on second page")
	}
}

func TestPaginate_LastPage(t *testing.T) {
	items := makeItems(5)
	first := Paginate(items, Cursor{}, 3, "item", getTestID, "/items", nil)
	cursor, err := DecodeCursor(first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	second := Paginate(items, cursor, 3, "item", getTestID, "/items", nil)
	if len(second.Items) != 2 {
		t.Fatalf("expected 2 items on last page, got %d", len(second.Items))
	}
	if second.NextCursor != "" {
		t.Fatalf("expected no next cursor on last page, got %q", second.NextCursor)
	}
}

func TestPaginate_EmptyItems(t *testing.T) {
	result := Paginate([]testItem{}, Cursor{}, 10, "item", getTestID, "/items", nil)
	if len(result.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result.Items))
	}
	if result.Total != 0 {
		t.Fatalf("expected total 0, got %d", result.Total)
	}
}

func TestPaginate_LimitExceedsItems(t *testing.T) {
	items := makeItems(3)
	result := Paginate(items, Cursor{}, 100, "item", getTestID, "/items", nil)
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}
	if result.NextCursor != "" {
		t.Fatalf("expected no next cursor, got %q", result.NextCursor)
	}
}

func TestPaginate_PreservesQueryParams(t *testing.T) {
	items := makeItems(10)
	q := url.Values{"category": {"electronics"}}
	result := Paginate(items, Cursor{}, 3, "item", getTestID, "/items", q)
	if result.LinkHeader == "" {
		t.Fatal("expected link header")
	}
}

func TestPaginate_CursorNotFound(t *testing.T) {
	items := makeItems(5)
	cursor := Cursor{Type: "item", Value: "nonexistent"}
	result := Paginate(items, cursor, 3, "item", getTestID, "/items", nil)
	// When cursor value is not found, starts from beginning.
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}
}

func TestPaginate_PrevCursorSecondPage(t *testing.T) {
	items := makeItems(10)
	first := Paginate(items, Cursor{}, 3, "item", getTestID, "/items", nil)
	cursor, err := DecodeCursor(first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	second := Paginate(items, cursor, 3, "item", getTestID, "/items", nil)
	if second.PrevCursor == "" {
		t.Fatal("expected prev cursor on second page")
	}
	prev, err := DecodeCursor(second.PrevCursor)
	if err != nil {
		t.Fatalf("decode prev cursor: %v", err)
	}
	if prev.Value != "" {
		t.Fatalf("expected empty prev cursor value for first page, got %q", prev.Value)
	}
}

func TestPaginate_PrevCursorThirdPage(t *testing.T) {
	items := makeItems(10)
	first := Paginate(items, Cursor{}, 3, "item", getTestID, "/items", nil)
	c1, err := DecodeCursor(first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	second := Paginate(items, c1, 3, "item", getTestID, "/items", nil)
	c2, err := DecodeCursor(second.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	third := Paginate(items, c2, 3, "item", getTestID, "/items", nil)
	if third.PrevCursor == "" {
		t.Fatal("expected prev cursor on third page")
	}
	prev, err := DecodeCursor(third.PrevCursor)
	if err != nil {
		t.Fatalf("decode prev cursor: %v", err)
	}
	if prev.Value != "c" {
		t.Fatalf("expected prev cursor to point to %q, got %q", "c", prev.Value)
	}
}
