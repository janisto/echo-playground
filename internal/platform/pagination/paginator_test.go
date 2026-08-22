package pagination

import (
	"errors"
	"net/url"
	"reflect"
	"strconv"
	"strings"
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

	wideScope := Scope{Operation: "list", Limit: 2}
	beforeSecond := NewCursor(wideScope, "prev", "b")
	nearStart, err := Paginate(items, &beforeSecond, wideScope, identifier, "/items", nil)
	if err != nil || !reflect.DeepEqual(nearStart.Items, items[:1]) || nearStart.PrevCursor != "" ||
		nearStart.NextCursor == "" {
		t.Fatalf("near-start previous page = %#v, err=%v", nearStart, err)
	}
}

func FuzzPaginate(f *testing.F) {
	f.Add(2, 0, false, false, false)
	f.Add(2, 1, true, false, false)
	f.Add(2, 1, true, true, false)
	f.Add(2, 4, true, true, false)
	f.Add(0, 0, false, false, false)
	f.Add(101, 0, false, false, false)
	f.Add(2, -1, true, false, false)
	f.Add(2, 1, true, false, true)
	f.Fuzz(func(t *testing.T, rawLimit, rawPosition int, useCursor, backwards, scopeMismatch bool) {
		items := []testItem{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}, {"g"}}
		limit, invalidLimit := fuzzLimit(rawLimit)
		positionIndex, stalePosition := fuzzPosition(rawPosition, len(items))
		scope := Scope{
			Operation: "listItems", Owner: "owner", Repository: "repository", Filter: "all", Limit: limit,
		}
		query := url.Values{
			"category": {"all"}, "tag": {"a", "b"}, "limit": {"999"}, "cursor": {"caller-owned"},
		}
		originalQuery := url.Values{
			"category": {"all"}, "tag": {"a", "b"}, "limit": {"999"}, "cursor": {"caller-owned"},
		}

		var cursor *Cursor
		if useCursor {
			cursorScope := scope
			if scopeMismatch {
				cursorScope.Operation = "otherOperation"
			}
			direction := "next"
			if backwards {
				direction = "prev"
			}
			position := "missing"
			if !stalePosition {
				position = items[positionIndex].id
			}
			value := NewCursor(cursorScope, direction, position)
			cursor = &value
		}
		result, err := Paginate(
			items,
			cursor,
			scope,
			func(item testItem) string { return item.id },
			"/items",
			query,
		)
		if !reflect.DeepEqual(query, originalQuery) {
			t.Fatalf("caller query mutated: %#v, want %#v", query, originalQuery)
		}

		var wantErr error
		switch {
		case invalidLimit:
			wantErr = ErrInvalidLimit
		case cursor != nil && scopeMismatch:
			wantErr = ErrCursorScopeMismatch
		case cursor != nil && stalePosition:
			wantErr = ErrCursorNotFound
		}
		if wantErr != nil {
			if !errors.Is(err, wantErr) {
				t.Fatalf("Paginate() error = %v, want %v", err, wantErr)
			}
			return
		}
		if err != nil {
			t.Fatalf("Paginate() unexpected error = %v", err)
		}

		start := 0
		end := min(len(items), limit)
		if cursor != nil {
			if cursor.Direction == "next" {
				start = positionIndex + 1
				end = min(len(items), start+limit)
			} else {
				end = positionIndex
				start = max(0, end-limit)
			}
		}
		if !reflect.DeepEqual(result.Items, items[start:end]) || result.Total != len(items) {
			t.Fatalf("page = %#v total=%d, want %#v total=%d", result.Items, result.Total, items[start:end], len(items))
		}

		wantNext := ""
		if len(result.Items) > 0 && end < len(items) {
			wantNext = NewCursor(scope, "next", items[end-1].id).Encode()
		}
		wantPrev := ""
		if len(result.Items) > 0 && start > 0 {
			wantPrev = NewCursor(scope, "prev", items[start].id).Encode()
		}
		if result.NextCursor != wantNext || result.PrevCursor != wantPrev {
			t.Fatalf("cursors = %q/%q, want %q/%q", result.NextCursor, result.PrevCursor, wantNext, wantPrev)
		}
		if wantNext != "" {
			assertFuzzCursor(t, wantNext, NewCursor(scope, "next", items[end-1].id))
		}
		if wantPrev != "" {
			assertFuzzCursor(t, wantPrev, NewCursor(scope, "prev", items[start].id))
		}
		assertFuzzLinks(t, result.LinkHeader, scope, wantNext, wantPrev)
	})
}

func fuzzLimit(value int) (int, bool) {
	if value >= -10 && value <= 0 || value >= 101 && value <= 110 {
		return value, true
	}
	value %= 100
	if value <= 0 {
		value += 100
	}
	return value, false
}

func fuzzPosition(value, itemCount int) (int, bool) {
	if value >= -3 && value < 0 || value >= itemCount && value <= itemCount+3 {
		return value, true
	}
	value %= itemCount
	if value < 0 {
		value += itemCount
	}
	return value, false
}

func assertFuzzCursor(t *testing.T, encoded string, want Cursor) {
	t.Helper()
	decoded, err := DecodeCursor(encoded)
	if err != nil || !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded cursor = %#v, %v, want %#v", decoded, err, want)
	}
}

func assertFuzzLinks(t *testing.T, header string, scope Scope, nextCursor, prevCursor string) {
	t.Helper()
	expected := map[string]string{}
	if nextCursor != "" {
		expected["next"] = nextCursor
	}
	if prevCursor != "" {
		expected["prev"] = prevCursor
	}
	if len(expected) == 0 {
		if header != "" {
			t.Fatalf("Link = %q, want empty", header)
		}
		return
	}
	parts := strings.Split(header, ", ")
	if len(parts) != len(expected) {
		t.Fatalf("Link parts = %#v, want %d", parts, len(expected))
	}
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		closeIndex := strings.Index(part, ">; rel=\"")
		if len(part) < 3 || part[0] != '<' || closeIndex < 2 || part[len(part)-1] != '"' {
			t.Fatalf("malformed Link value %q", part)
		}
		relation := part[closeIndex+8 : len(part)-1]
		cursor, ok := expected[relation]
		if !ok || seen[relation] {
			t.Fatalf("unexpected or repeated Link relation %q in %q", relation, part)
		}
		seen[relation] = true
		target, err := url.Parse(part[1:closeIndex])
		if err != nil || target.Scheme != "" || target.Host != "" || target.User != nil ||
			target.Path != "/items" || target.Fragment != "" {
			t.Fatalf("Link target = %#v, %v", target, err)
		}
		wantQuery := url.Values{
			"category": {scope.Filter},
			"tag":      {"a", "b"},
			"limit":    {strconv.Itoa(scope.Limit)},
			"cursor":   {cursor},
		}
		if query := target.Query(); !reflect.DeepEqual(query, wantQuery) {
			t.Fatalf("Link query = %#v, want %#v", query, wantQuery)
		}
	}
}
