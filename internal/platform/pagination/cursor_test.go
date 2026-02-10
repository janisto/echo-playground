package pagination

import (
	"errors"
	"strings"
	"testing"
)

func TestCursor_EncodeDecode_Roundtrip(t *testing.T) {
	original := Cursor{Type: "item", Value: "42"}
	encoded := original.Encode()
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decoded.Type != original.Type || decoded.Value != original.Value {
		t.Fatalf("expected %+v, got %+v", original, decoded)
	}
}

func TestDecodeCursor_Empty(t *testing.T) {
	decoded, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decoded.Type != "" || decoded.Value != "" {
		t.Fatalf("expected zero Cursor, got %+v", decoded)
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := DecodeCursor("!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestDecodeCursor_MissingColon(t *testing.T) {
	// Encode "nocolon" without a colon separator.
	_, err := DecodeCursor("bm9jb2xvbg")
	if err == nil {
		t.Fatal("expected error for missing colon")
	}
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestCursor_Encode_SpecialChars(t *testing.T) {
	original := Cursor{Type: "item", Value: "key with spaces & symbols!"}
	encoded := original.Encode()
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decoded.Value != original.Value {
		t.Fatalf("expected %q, got %q", original.Value, decoded.Value)
	}
}

func TestCursor_Encode_EmptyValue(t *testing.T) {
	c := Cursor{Type: "item", Value: ""}
	encoded := c.Encode()
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decoded.Type != "item" {
		t.Fatalf("expected type 'item', got %q", decoded.Type)
	}
	if decoded.Value != "" {
		t.Fatalf("expected empty value, got %q", decoded.Value)
	}
}

func TestCursor_Encode_URLSafe(t *testing.T) {
	c := Cursor{Type: "test", Value: "value+with/special=chars"}
	encoded := c.Encode()

	for _, ch := range encoded {
		if ch == '+' || ch == '/' {
			t.Errorf("encoded cursor contains non-URL-safe character: %c", ch)
		}
	}
}

func TestCursor_EmptyType_NonEmptyValue(t *testing.T) {
	c := Cursor{Type: "", Value: "some-value"}
	encoded := c.Encode()

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != "" {
		t.Errorf("expected empty type, got %q", decoded.Type)
	}
	if decoded.Value != "some-value" {
		t.Errorf("expected 'some-value', got %q", decoded.Value)
	}
}

func TestCursor_ColonInValue(t *testing.T) {
	c := Cursor{Type: "item", Value: "2024-01-15T10:30:00.000Z"}
	encoded := c.Encode()

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != "item" {
		t.Errorf("type mismatch: got %q", decoded.Type)
	}
	if decoded.Value != "2024-01-15T10:30:00.000Z" {
		t.Errorf("value mismatch: got %q", decoded.Value)
	}
}

func TestCursor_MultipleColonsInValue(t *testing.T) {
	c := Cursor{Type: "composite", Value: "a:b:c:d"}
	encoded := c.Encode()

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Value != "a:b:c:d" {
		t.Errorf("value should preserve all colons, got %q", decoded.Value)
	}
}

func TestCursor_LongValue(t *testing.T) {
	longValue := strings.Repeat("x", 1000)
	c := Cursor{Type: "item", Value: longValue}
	encoded := c.Encode()

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Value != longValue {
		t.Error("long value not preserved correctly")
	}
}

func TestCursor_UnicodeValue(t *testing.T) {
	c := Cursor{Type: "item", Value: "日本語テスト"}
	encoded := c.Encode()

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Value != "日本語テスト" {
		t.Errorf("unicode value mismatch: got %q", decoded.Value)
	}
}

func TestDecodeCursor_PaddingVariations(t *testing.T) {
	tests := []struct {
		name   string
		cursor Cursor
	}{
		{"no-padding-needed", Cursor{Type: "abc", Value: "def"}},
		{"one-pad", Cursor{Type: "ab", Value: "cd"}},
		{"two-pad", Cursor{Type: "a", Value: "b"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded := tc.cursor.Encode()
			decoded, err := DecodeCursor(encoded)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if decoded.Type != tc.cursor.Type || decoded.Value != tc.cursor.Value {
				t.Errorf("mismatch: got %+v, want %+v", decoded, tc.cursor)
			}
		})
	}
}
