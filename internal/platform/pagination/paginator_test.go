package pagination

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
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

func mustPaginate(
	t *testing.T,
	items []testItem,
	cursor Cursor,
	limit int,
	query url.Values,
) Result[testItem] {
	t.Helper()
	result, err := Paginate(items, cursor, limit, "item", getTestID, "/items", query)
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	return result
}

func FuzzPaginate(f *testing.F) {
	f.Add(uint8(0), uint8(0), int16(-1))
	f.Add(uint8(10), uint8(1), int16(-1))
	f.Add(uint8(10), uint8(4), int16(-1))
	f.Add(uint8(10), uint8(4), int16(2))
	f.Add(uint8(5), uint8(17), int16(4))

	f.Fuzz(func(t *testing.T, countInput, limitInput uint8, cursorPosition int16) {
		count := int(countInput % 64)
		limit := int(limitInput%18) - 1
		items := makeItems(count)
		cursor := Cursor{}
		start := 0
		if count > 0 && cursorPosition >= 0 {
			index := int(cursorPosition) % count
			cursor = Cursor{Type: "item", Value: items[index].ID}
			start = index + 1
		}

		query := url.Values{"category": {"tools"}, "cursor": {"stale"}}
		result, err := Paginate(items, cursor, limit, "item", getTestID, "/items", query)
		if limit <= 0 {
			if !errors.Is(err, ErrInvalidLimit) {
				t.Fatalf("non-positive limit %d error = %v, want %v", limit, err, ErrInvalidLimit)
			}
			return
		}
		if err != nil {
			t.Fatalf("paginate valid cursor: %v", err)
		}

		end := min(start+limit, count)
		if result.Total != count {
			t.Fatalf("total = %d, want %d", result.Total, count)
		}
		if len(result.Items) != end-start {
			t.Fatalf("page length = %d, want %d", len(result.Items), end-start)
		}
		for i, item := range result.Items {
			if want := items[start+i]; item != want {
				t.Fatalf("item %d = %#v, want %#v", i, item, want)
			}
		}

		wantNext := end < count && end > start
		if (result.NextCursor != "") != wantNext {
			t.Fatalf("next cursor presence = %t, want %t", result.NextCursor != "", wantNext)
		}
		if wantNext {
			next, decodeErr := DecodeCursor(result.NextCursor)
			if decodeErr != nil {
				t.Fatalf("decode next cursor: %v", decodeErr)
			}
			if want := (Cursor{Type: "item", Value: items[end-1].ID}); next != want {
				t.Fatalf("next cursor = %#v, want %#v", next, want)
			}
		}

		wantPrev := start > 0
		if (result.PrevCursor != "") != wantPrev {
			t.Fatalf("previous cursor presence = %t, want %t", result.PrevCursor != "", wantPrev)
		}
		if wantPrev {
			prev, decodeErr := DecodeCursor(result.PrevCursor)
			if decodeErr != nil {
				t.Fatalf("decode previous cursor: %v", decodeErr)
			}
			wantValue := ""
			if start > limit {
				wantValue = items[start-limit-1].ID
			}
			if want := (Cursor{Type: "item", Value: wantValue}); prev != want {
				t.Fatalf("previous cursor = %#v, want %#v", prev, want)
			}
		}

		wantLinks := 0
		if wantNext {
			wantLinks++
		}
		if wantPrev {
			wantLinks++
		}
		if got := strings.Count(result.LinkHeader, "rel=\""); got != wantLinks {
			t.Fatalf("link count = %d, want %d: %q", got, wantLinks, result.LinkHeader)
		}
		if wantLinks > 0 {
			if !strings.Contains(result.LinkHeader, "category=tools") ||
				!strings.Contains(result.LinkHeader, "limit="+strconv.Itoa(limit)) {
				t.Fatalf("link did not preserve filters and limit: %q", result.LinkHeader)
			}
			if strings.Contains(result.LinkHeader, "cursor=stale") {
				t.Fatalf("link retained stale cursor: %q", result.LinkHeader)
			}
		}
		if got := query.Get("cursor"); got != "stale" {
			t.Fatalf("input query cursor mutated to %q", got)
		}
	})
}

func TestPaginate_RejectsNonPositiveLimit(t *testing.T) {
	for _, limit := range []int{-1, 0} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			_, err := Paginate(makeItems(3), Cursor{}, limit, "item", getTestID, "/items", nil)
			if !errors.Is(err, ErrInvalidLimit) {
				t.Fatalf("Paginate limit %d error = %v, want %v", limit, err, ErrInvalidLimit)
			}
		})
	}
}

func TestPaginate_FirstPage(t *testing.T) {
	items := makeItems(10)
	result := mustPaginate(t, items, Cursor{}, 3, nil)
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
	first := mustPaginate(t, items, Cursor{}, 3, nil)
	cursor, err := DecodeCursor(first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	second := mustPaginate(t, items, cursor, 3, nil)
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
	first := mustPaginate(t, items, Cursor{}, 3, nil)
	cursor, err := DecodeCursor(first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	second := mustPaginate(t, items, cursor, 3, nil)
	if len(second.Items) != 2 {
		t.Fatalf("expected 2 items on last page, got %d", len(second.Items))
	}
	if second.NextCursor != "" {
		t.Fatalf("expected no next cursor on last page, got %q", second.NextCursor)
	}
}

func TestPaginate_EmptyItems(t *testing.T) {
	result := mustPaginate(t, []testItem{}, Cursor{}, 10, nil)
	if len(result.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result.Items))
	}
	if result.Total != 0 {
		t.Fatalf("expected total 0, got %d", result.Total)
	}
}

func TestPaginate_LimitExceedsItems(t *testing.T) {
	items := makeItems(3)
	result := mustPaginate(t, items, Cursor{}, 100, nil)
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
	result := mustPaginate(t, items, Cursor{}, 3, q)
	if result.LinkHeader == "" {
		t.Fatal("expected link header")
	}
	if !strings.Contains(result.LinkHeader, "category=electronics") {
		t.Fatalf("expected category in link header, got %q", result.LinkHeader)
	}
}

func TestPaginate_CursorNotFound(t *testing.T) {
	items := makeItems(5)
	cursor := Cursor{Type: "item", Value: "nonexistent"}
	_, err := Paginate(items, cursor, 3, "item", getTestID, "/items", nil)
	if !errors.Is(err, ErrCursorNotFound) {
		t.Fatalf("expected ErrCursorNotFound, got %v", err)
	}
}

func TestPaginate_PrevCursorSecondPage(t *testing.T) {
	items := makeItems(10)
	first := mustPaginate(t, items, Cursor{}, 3, nil)
	cursor, err := DecodeCursor(first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	second := mustPaginate(t, items, cursor, 3, nil)
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
	first := mustPaginate(t, items, Cursor{}, 3, nil)
	c1, err := DecodeCursor(first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	second := mustPaginate(t, items, c1, 3, nil)
	c2, err := DecodeCursor(second.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	third := mustPaginate(t, items, c2, 3, nil)
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
