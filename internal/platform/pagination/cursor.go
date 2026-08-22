package pagination

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
)

const MaxCursorLength = 2048

var ErrInvalidCursor = errors.New("invalid cursor")

// Scope is every result-shaping value to which a portable cursor is bound.
type Scope struct {
	Operation  string
	Owner      string
	Repository string
	Filter     string
	Limit      int
}

// Cursor contains only validated local transport state. It never contains a
// complete provider URL, credential, or profile data.
type Cursor struct {
	Operation  string
	Owner      string
	Repository string
	Filter     string
	Direction  string
	Position   string
	Limit      int
}

func NewCursor(scope Scope, direction, position string) Cursor {
	return Cursor{
		Operation:  scope.Operation,
		Owner:      scope.Owner,
		Repository: scope.Repository,
		Filter:     scope.Filter,
		Direction:  direction,
		Position:   position,
		Limit:      scope.Limit,
	}
}

func (c Cursor) Matches(scope Scope) bool {
	return c.Operation == scope.Operation && c.Owner == scope.Owner &&
		c.Repository == scope.Repository && c.Filter == scope.Filter && c.Limit == scope.Limit
}

func (c Cursor) Encode() string {
	if c.Limit < 1 || c.Limit > 100 {
		return ""
	}
	payload := []byte{1}
	payload = binary.AppendUvarint(payload, uint64(c.Limit))
	for _, value := range [...]string{c.Operation, c.Owner, c.Repository, c.Filter, c.Direction, c.Position} {
		payload = binary.AppendUvarint(payload, uint64(len(value)))
		payload = append(payload, value...)
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func DecodeCursor(value string) (Cursor, error) {
	if value == "" || len(value) > MaxCursorLength {
		return Cursor{}, ErrInvalidCursor
	}
	for i := range len(value) {
		if value[i] < 0x21 || value[i] > 0x7e {
			return Cursor{}, ErrInvalidCursor
		}
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(payload) < 2 || payload[0] != 1 {
		return Cursor{}, ErrInvalidCursor
	}
	offset := 1
	limit, read := binary.Uvarint(payload[offset:])
	if read <= 0 || limit < 1 || limit > 100 {
		return Cursor{}, ErrInvalidCursor
	}
	offset += read
	parts := [6]string{}
	for index := range parts {
		part, nextOffset, ok := readPart(payload, offset)
		if !ok {
			return Cursor{}, ErrInvalidCursor
		}
		parts[index], offset = part, nextOffset
	}
	if offset != len(payload) || parts[0] == "" || parts[4] != "next" && parts[4] != "prev" || parts[5] == "" {
		return Cursor{}, ErrInvalidCursor
	}
	cursor := Cursor{
		Operation:  parts[0],
		Owner:      parts[1],
		Repository: parts[2],
		Filter:     parts[3],
		Direction:  parts[4],
		Position:   parts[5],
		Limit:      int(limit),
	}
	if !bytes.Equal([]byte(cursor.Encode()), []byte(value)) {
		return Cursor{}, ErrInvalidCursor
	}
	return cursor, nil
}

func readPart(payload []byte, offset int) (string, int, bool) {
	if offset >= len(payload) {
		return "", offset, false
	}
	length, read := binary.Uvarint(payload[offset:])
	if read <= 0 {
		return "", offset, false
	}
	offset += read
	if length > uint64(len(payload)-offset) { //nolint:gosec // remaining byte length is nonnegative and int-sized
		return "", offset, false
	}
	end := offset + int(length) //nolint:gosec // length is bounded by the remaining int-sized payload.
	return string(payload[offset:end]), end, true
}
