package pagination

import (
	"errors"
	"net/url"
	"strconv"
)

var (
	ErrCursorScopeMismatch = errors.New("cursor scope mismatch")
	ErrCursorNotFound      = errors.New("cursor position not found")
	ErrInvalidLimit        = errors.New("invalid limit")
)

const DefaultLimit = 20

type Result[T any] struct {
	Items      []T
	Total      int
	LinkHeader string
	NextCursor string
	PrevCursor string
}

// Paginate applies stable forward or backward cursor traversal to a local slice.
func Paginate[T any](
	items []T,
	cursor *Cursor,
	scope Scope,
	getID func(T) string,
	baseURL string,
	query url.Values,
) (Result[T], error) {
	if scope.Limit < 1 || scope.Limit > 100 {
		return Result[T]{}, ErrInvalidLimit
	}
	start := 0
	if cursor != nil {
		if !cursor.Matches(scope) {
			return Result[T]{}, ErrCursorScopeMismatch
		}
		position := -1
		for index, item := range items {
			if getID(item) == cursor.Position {
				position = index
				break
			}
		}
		if position < 0 {
			return Result[T]{}, ErrCursorNotFound
		}
		if cursor.Direction == "next" {
			start = position + 1
		} else {
			start = max(0, position-scope.Limit)
		}
	}
	end := min(len(items), start+scope.Limit)
	page := items[start:end]

	var nextCursor, prevCursor string
	if len(page) > 0 && end < len(items) {
		nextCursor = NewCursor(scope, "next", getID(page[len(page)-1])).Encode()
	}
	if len(page) > 0 && start > 0 {
		prevCursor = NewCursor(scope, "prev", getID(page[0])).Encode()
	}
	linkQuery := cloneValues(query)
	linkQuery.Set("limit", strconv.Itoa(scope.Limit))
	return Result[T]{
		Items:      page,
		Total:      len(items),
		LinkHeader: BuildLinkHeader(baseURL, linkQuery, nextCursor, prevCursor),
		NextCursor: nextCursor,
		PrevCursor: prevCursor,
	}, nil
}
