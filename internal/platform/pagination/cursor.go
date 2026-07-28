package pagination

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/url"
	"strconv"
)

// ErrInvalidCursor indicates the cursor could not be decoded.
var ErrInvalidCursor = errors.New("invalid cursor format")

// Cursor represents a pagination position.
type Cursor struct {
	Type  string // resource type identifier
	Value string // last seen value (ID, timestamp, etc.)
	Scope string // canonical filters and page size
}

// Encode returns a URL-safe opaque Base64 representation.
func (c Cursor) Encode() string {
	payload := []byte{1}
	for _, value := range [...]string{c.Type, c.Value, c.Scope} {
		payload = binary.AppendUvarint(payload, uint64(len(value)))
		payload = append(payload, value...)
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

// DecodeCursor parses a URL-safe Base64 cursor string.
func DecodeCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	if len(b) == 0 || b[0] != 1 {
		return Cursor{}, ErrInvalidCursor
	}
	parts := [3]string{}
	offset := 1
	for i := range parts {
		length, read := binary.Uvarint(b[offset:])
		if read <= 0 {
			return Cursor{}, ErrInvalidCursor
		}
		offset += read
		remaining := len(b) - offset
		if length > uint64(remaining) { //nolint:gosec // remaining is non-negative and bounded by len(b).
			return Cursor{}, ErrInvalidCursor
		}
		lengthInt := int(length) //nolint:gosec // length was bounded by remaining, which is an int.
		parts[i] = string(b[offset : offset+lengthInt])
		offset += lengthInt
	}
	if offset != len(b) {
		return Cursor{}, ErrInvalidCursor
	}
	return Cursor{Type: parts[0], Value: parts[1], Scope: parts[2]}, nil
}

// NewCursor binds a pagination position to the effective filters and page size.
func NewCursor(cursorType, value string, limit int, query url.Values) Cursor {
	scope := cloneValues(query)
	scope.Del("cursor")
	scope.Set("limit", strconv.Itoa(limit))
	return Cursor{Type: cursorType, Value: value, Scope: scope.Encode()}
}
