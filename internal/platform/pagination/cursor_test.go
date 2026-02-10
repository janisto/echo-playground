package pagination

import (
	"errors"
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
