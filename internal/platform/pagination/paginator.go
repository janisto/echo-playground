package pagination

import (
	"errors"
	"net/url"
	"strconv"
)

var (
	// ErrCursorNotFound indicates that a cursor no longer references an item.
	ErrCursorNotFound = errors.New("cursor not found")
	// ErrInvalidLimit indicates that the requested page size is not positive.
	ErrInvalidLimit = errors.New("pagination limit must be positive")
	// ErrCursorScopeMismatch indicates that a cursor was issued for different filters or page size.
	ErrCursorScopeMismatch = errors.New("cursor scope mismatch")
)

// DefaultLimit is the default page size for list endpoints.
const DefaultLimit = 20

// Result holds the outcome of a pagination operation.
type Result[T any] struct {
	Items      []T
	Total      int
	LinkHeader string
	NextCursor string
	PrevCursor string
}

// Paginate applies cursor-based pagination to a slice of items.
//
// Parameters:
//   - items: The full slice of items to paginate
//   - cursor: The decoded cursor from the request
//   - limit: Positive maximum number of items per page; non-positive values return ErrInvalidLimit
//   - cursorType: Type identifier for cursor validation (e.g., "item", "user")
//   - getID: Function to extract the ID from an item
//   - baseURL: Base URL path for Link header (e.g., "/items")
//   - query: Additional query parameters to preserve in links
//
// Returns a Result containing the page of items and pagination metadata.
func Paginate[T any](
	items []T,
	cursor Cursor,
	limit int,
	cursorType string,
	getID func(T) string,
	baseURL string,
	query url.Values,
) (Result[T], error) {
	if limit <= 0 {
		return Result[T]{}, ErrInvalidLimit
	}
	expectedCursor := NewCursor(cursorType, cursor.Value, limit, query)
	if cursor != (Cursor{}) && cursor.Scope != expectedCursor.Scope {
		return Result[T]{}, ErrCursorScopeMismatch
	}

	total := len(items)

	startIdx := 0
	if cursor.Value != "" {
		found := false
		for i, item := range items {
			if getID(item) == cursor.Value {
				startIdx = i + 1
				found = true
				break
			}
		}
		if !found {
			return Result[T]{}, ErrCursorNotFound
		}
	}

	remaining := total - startIdx
	endIdx := startIdx + min(limit, remaining)

	pageItems := items[startIdx:endIdx]

	var nextCursor, prevCursor string

	if endIdx < total {
		nextCursor = NewCursor(cursorType, getID(pageItems[len(pageItems)-1]), limit, query).Encode()
	}

	if startIdx > 0 {
		if startIdx <= limit {
			prevCursor = NewCursor(cursorType, "", limit, query).Encode()
		} else {
			prevLastIdx := startIdx - 1
			prevCursor = NewCursor(cursorType, getID(items[prevLastIdx-limit]), limit, query).Encode()
		}
	}

	q := cloneValues(query)
	q.Set("limit", strconv.Itoa(limit))
	linkHeader := BuildLinkHeader(baseURL, q, nextCursor, prevCursor)

	return Result[T]{
		Items:      pageItems,
		Total:      total,
		LinkHeader: linkHeader,
		NextCursor: nextCursor,
		PrevCursor: prevCursor,
	}, nil
}
