package pagination

import (
	"errors"
	"net/url"
	"reflect"
	"testing"
)

type testItem struct{ id string }

func TestPaginateTraversesForwardAndBackwardWithoutGaps(t *testing.T) {
	items := []testItem{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}}
	scope := Scope{Operation: "list", Filter: "all", Limit: 2}
	identifier := func(item testItem) string { return item.id }
	first, err := Paginate(items, nil, scope, identifier, "/items", url.Values{"category": {"all"}})
	if err != nil || !reflect.DeepEqual(first.Items, items[:2]) || first.PrevCursor != "" || first.NextCursor == "" {
		t.Fatalf("first = %#v, err=%v", first, err)
	}
	next, _ := DecodeCursor(first.NextCursor)
	middle, err := Paginate(items, &next, scope, identifier, "/items", url.Values{"category": {"all"}})
	if err != nil || !reflect.DeepEqual(middle.Items, items[2:4]) || middle.PrevCursor == "" ||
		middle.NextCursor == "" {
		t.Fatalf("middle = %#v, err=%v", middle, err)
	}
	next, _ = DecodeCursor(middle.NextCursor)
	last, err := Paginate(items, &next, scope, identifier, "/items", url.Values{"category": {"all"}})
	if err != nil || !reflect.DeepEqual(last.Items, items[4:]) || last.NextCursor != "" || last.PrevCursor == "" {
		t.Fatalf("last = %#v, err=%v", last, err)
	}
	previous, _ := DecodeCursor(last.PrevCursor)
	back, err := Paginate(items, &previous, scope, identifier, "/items", url.Values{"category": {"all"}})
	if err != nil || !reflect.DeepEqual(back.Items, items[2:4]) {
		t.Fatalf("back = %#v, err=%v", back, err)
	}
	if first.LinkHeader == "" || middle.LinkHeader == "" || last.LinkHeader == "" {
		t.Fatal("navigation links were not emitted")
	}
}

func TestPaginateRejectsScopeStalePositionAndLimit(t *testing.T) {
	items := []testItem{{"a"}}
	identifier := func(item testItem) string { return item.id }
	scope := Scope{Operation: "list", Limit: 1}
	tests := []struct {
		scope  Scope
		cursor *Cursor
		error  error
	}{
		{scope: Scope{Operation: "list", Limit: 0}, error: ErrInvalidLimit},
		{scope: Scope{Operation: "list", Limit: 101}, error: ErrInvalidLimit},
		{
			scope:  scope,
			cursor: new(NewCursor(Scope{Operation: "other", Limit: 1}, "next", "a")),
			error:  ErrCursorScopeMismatch,
		},
		{scope: scope, cursor: new(NewCursor(scope, "next", "missing")), error: ErrCursorNotFound},
	}
	for _, test := range tests {
		if _, err := Paginate(items, test.cursor, test.scope, identifier, "/items", nil); !errors.Is(err, test.error) {
			t.Fatalf("error = %v, want %v", err, test.error)
		}
	}
}

func TestPaginateEmptyExactAndAdjacentPagesHaveExactNavigation(t *testing.T) {
	identifier := func(item testItem) string { return item.id }
	for _, test := range []struct {
		name      string
		items     []testItem
		limit     int
		wantItems int
		wantNext  bool
		wantPrev  bool
	}{
		{name: "empty", limit: 1},
		{name: "exact page", items: []testItem{{"a"}, {"b"}}, limit: 2, wantItems: 2},
		{
			name:  "one beyond page",
			items: []testItem{{"a"}, {"b"}, {"c"}},
			limit: 2, wantItems: 2, wantNext: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			scope := Scope{Operation: "list", Limit: test.limit}
			result, err := Paginate(test.items, nil, scope, identifier, "/items", nil)
			if err != nil || len(result.Items) != test.wantItems || result.Total != len(test.items) ||
				(result.NextCursor != "") != test.wantNext || (result.PrevCursor != "") != test.wantPrev {
				t.Fatalf("result = %#v, err=%v", result, err)
			}
			if (result.LinkHeader != "") != (test.wantNext || test.wantPrev) {
				t.Fatalf("Link = %q", result.LinkHeader)
			}
		})
	}
}

func TestPaginateCursorAtFirstAndLastItemHonorsEmptyPageBoundaries(t *testing.T) {
	items := []testItem{{"a"}, {"b"}, {"c"}}
	identifier := func(item testItem) string { return item.id }
	scope := Scope{Operation: "list", Limit: 1}

	first := NewCursor(scope, "next", "a")
	afterFirst, err := Paginate(items, &first, scope, identifier, "/items", nil)
	if err != nil || len(afterFirst.Items) != 1 || afterFirst.Items[0].id != "b" ||
		afterFirst.NextCursor == "" || afterFirst.PrevCursor == "" {
		t.Fatalf("after first = %#v, err=%v", afterFirst, err)
	}

	last := NewCursor(scope, "next", "c")
	afterLast, err := Paginate(items, &last, scope, identifier, "/items", nil)
	if err != nil || len(afterLast.Items) != 0 || afterLast.Total != len(items) ||
		afterLast.NextCursor != "" || afterLast.PrevCursor != "" || afterLast.LinkHeader != "" {
		t.Fatalf("after last = %#v, err=%v", afterLast, err)
	}

	hundred := make([]testItem, 101)
	for index := range hundred {
		hundred[index].id = string(rune(index + 1))
	}
	maximumScope := Scope{Operation: "list", Limit: 100}
	maximumPage, err := Paginate(hundred, nil, maximumScope, identifier, "/items", nil)
	if err != nil || len(maximumPage.Items) != 100 || maximumPage.NextCursor == "" {
		t.Fatalf("maximum page = %#v, err=%v", maximumPage, err)
	}
}

func FuzzPaginate(f *testing.F) {
	f.Add(20, "")
	f.Add(1, "a")
	f.Fuzz(func(t *testing.T, limit int, position string) {
		items := []testItem{{"a"}, {"b"}, {"c"}}
		scope := Scope{Operation: "list", Limit: limit}
		var cursor *Cursor
		if position != "" {
			value := NewCursor(scope, "next", position)
			cursor = &value
		}
		result, err := Paginate(items, cursor, scope, func(item testItem) string { return item.id }, "/items", nil)
		if err == nil && (len(result.Items) > limit || len(result.Items) > len(items)) {
			t.Fatalf("invalid result size %d for limit %d", len(result.Items), limit)
		}
	})
}
