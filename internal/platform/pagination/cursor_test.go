package pagination

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
)

func TestCursorRoundTripBindsEveryScopeMember(t *testing.T) {
	scope := Scope{Operation: "operation", Owner: "owner", Repository: "repo", Filter: "category", Limit: 100}
	want := NewCursor(scope, "prev", "position")
	encoded := want.Encode()
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if decoded != want || !decoded.Matches(scope) || decoded.Encode() != encoded {
		t.Fatalf("cursor round trip = %#v, want %#v", decoded, want)
	}
	for _, changed := range []Scope{
		{Operation: "other", Owner: "owner", Repository: "repo", Filter: "category", Limit: 100},
		{Operation: "operation", Owner: "other", Repository: "repo", Filter: "category", Limit: 100},
		{Operation: "operation", Owner: "owner", Repository: "other", Filter: "category", Limit: 100},
		{Operation: "operation", Owner: "owner", Repository: "repo", Filter: "other", Limit: 100},
		{Operation: "operation", Owner: "owner", Repository: "repo", Filter: "category", Limit: 99},
	} {
		if decoded.Matches(changed) {
			t.Fatalf("cursor matched changed scope %#v", changed)
		}
	}
}

func TestCursorRoundTripAcceptsAdjacentValidLimitsAndMaximumWireLength(t *testing.T) {
	for _, limit := range []int{1, 100} {
		cursor := NewCursor(Scope{Operation: "items", Limit: limit}, "next", "item-001")
		decoded, err := DecodeCursor(cursor.Encode())
		if err != nil || decoded != cursor {
			t.Fatalf("limit %d round trip = %#v, err=%v", limit, decoded, err)
		}
	}

	maximum := NewCursor(Scope{Operation: "items", Limit: 1}, "next", strings.Repeat("p", 1518)).Encode()
	if len(maximum) != MaxCursorLength {
		t.Fatalf("constructed cursor length = %d, want %d", len(maximum), MaxCursorLength)
	}
	if _, err := DecodeCursor(maximum); err != nil {
		t.Fatalf("maximum-length cursor rejected: %v", err)
	}
	overlong := NewCursor(Scope{Operation: "items", Limit: 1}, "next", strings.Repeat("p", 1519)).Encode()
	if len(overlong) <= MaxCursorLength {
		t.Fatalf("constructed overlong cursor length = %d", len(overlong))
	}
	if _, err := DecodeCursor(overlong); err == nil {
		t.Fatal("overlong otherwise-canonical cursor accepted")
	}
}

func TestDecodeCursorRejectsMalformedNoncanonicalAndInvalidState(t *testing.T) {
	valid := NewCursor(Scope{Operation: "items", Limit: 20}, "next", "item-020").Encode()
	padded := valid + "="
	payload, _ := base64.RawURLEncoding.DecodeString(valid)
	noncanonical := base64.URLEncoding.EncodeToString(payload)
	tests := []string{
		"", strings.Repeat("a", MaxCursorLength+1), "contains space", "é", "%%%", padded, noncanonical,
		base64.RawURLEncoding.EncodeToString([]byte{2, 1}),
		NewCursor(Scope{Operation: "items", Limit: 0}, "next", "x").Encode(),
		NewCursor(Scope{Operation: "items", Limit: 20}, "sideways", "x").Encode(),
		NewCursor(Scope{Operation: "", Limit: 20}, "next", "x").Encode(),
		NewCursor(Scope{Operation: "items", Limit: 20}, "next", "").Encode(),
	}
	for _, value := range tests {
		if _, err := DecodeCursor(value); err == nil {
			t.Fatalf("DecodeCursor(%q) unexpectedly succeeded", value)
		}
	}
}

func TestCursorEncodeRejectsAdjacentInvalidLimits(t *testing.T) {
	for _, limit := range []int{0, 101} {
		cursor := NewCursor(Scope{Operation: "items", Limit: limit}, "next", "item-001")
		if encoded := cursor.Encode(); encoded != "" {
			t.Fatalf("Encode limit %d = %q, want empty", limit, encoded)
		}
	}
}

func TestDecodeCursorRejectsMalformedEncodedStateAtEachBoundary(t *testing.T) {
	validParts := []string{"items", "", "", "", "next", "item-020"}
	tests := [][]byte{
		{1},
		{1, 0x80},
		cursorPayload(0, validParts...),
		cursorPayload(101, validParts...),
		cursorPayload(20, "", "", "", "", "next", "item-020"),
		cursorPayload(20, "items", "", "", "", "sideways", "item-020"),
		cursorPayload(20, "items", "", "", "", "next", ""),
		append(cursorPayload(20, validParts...), 0),
		{1, 20, 5, 'i'},
	}
	wrongVersion := cursorPayload(20, validParts...)
	wrongVersion[0] = 0
	tests = append(tests, wrongVersion)
	for _, payload := range tests {
		encoded := base64.RawURLEncoding.EncodeToString(payload)
		if _, err := DecodeCursor(encoded); err == nil {
			t.Fatalf("DecodeCursor accepted payload %x", payload)
		}
	}

	noncanonicalLimit := append([]byte{1, 0x94, 0x00}, cursorPayloadParts(validParts...)...)
	if _, err := DecodeCursor(base64.RawURLEncoding.EncodeToString(noncanonicalLimit)); err == nil {
		t.Fatalf("DecodeCursor accepted noncanonical uvarint %x", noncanonicalLimit)
	}
	valid := base64.RawURLEncoding.EncodeToString(cursorPayload(20, validParts...))
	if _, err := DecodeCursor(valid[:len(valid)-1] + "%"); err == nil {
		t.Fatal("DecodeCursor accepted a partially decodable invalid base64 value")
	}
}

func cursorPayload(limit uint64, parts ...string) []byte {
	payload := binary.AppendUvarint([]byte{1}, limit)
	return append(payload, cursorPayloadParts(parts...)...)
}

func cursorPayloadParts(parts ...string) []byte {
	var payload []byte
	for _, part := range parts {
		payload = binary.AppendUvarint(payload, uint64(len(part)))
		payload = append(payload, part...)
	}
	return payload
}

func FuzzDecodeCursor(f *testing.F) {
	f.Add(NewCursor(Scope{Operation: "items", Limit: 20}, "next", "item-020").Encode())
	f.Add("malformed")
	f.Fuzz(func(t *testing.T, value string) {
		cursor, err := DecodeCursor(value)
		if err == nil {
			if cursor.Encode() != value {
				t.Fatalf("accepted noncanonical cursor %q", value)
			}
			if cursor.Direction != "next" && cursor.Direction != "prev" {
				t.Fatalf("accepted direction %q", cursor.Direction)
			}
		}
	})
}
