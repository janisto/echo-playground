package timeutil

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMarshalJSON(t *testing.T) {
	ts := NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))
	b, err := ts.MarshalJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `"2024-01-15T10:30:00.000Z"`
	if string(b) != want {
		t.Fatalf("expected %s, got %s", want, string(b))
	}
}

func TestMarshalJSON_Milliseconds(t *testing.T) {
	ts := NewTime(time.Date(2024, 6, 1, 12, 0, 0, 123456789, time.UTC))
	b, err := ts.MarshalJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `"2024-06-01T12:00:00.123Z"`
	if string(b) != want {
		t.Fatalf("expected %s, got %s", want, string(b))
	}
}

func TestUnmarshalJSON_RFC3339Nano(t *testing.T) {
	var ts Time
	if err := ts.UnmarshalJSON([]byte(`"2024-01-15T10:30:00.123456789Z"`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.Year() != 2024 || ts.Month() != 1 || ts.Day() != 15 {
		t.Fatalf("unexpected date: %v", ts.Time)
	}
}

func TestUnmarshalJSON_RFC3339(t *testing.T) {
	var ts Time
	if err := ts.UnmarshalJSON([]byte(`"2024-01-15T10:30:00Z"`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.Hour() != 10 || ts.Minute() != 30 {
		t.Fatalf("unexpected time: %v", ts.Time)
	}
}

func TestUnmarshalJSON_Null(t *testing.T) {
	ts := NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	original := ts.Time
	if err := ts.UnmarshalJSON([]byte("null")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ts.Equal(original) {
		t.Fatalf("null should preserve existing value")
	}
}

func TestUnmarshalJSON_Invalid(t *testing.T) {
	var ts Time
	err := ts.UnmarshalJSON([]byte(`"not-a-date"`))
	if err == nil {
		t.Fatal("expected error for invalid date string")
	}
}

func TestUnmarshalJSON_BareString(t *testing.T) {
	var ts Time
	if err := ts.UnmarshalJSON([]byte(`"2024-06-15T08:00:00.000Z"`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.Hour() != 8 {
		t.Fatalf("expected hour 8, got %d", ts.Hour())
	}
}

func TestUnmarshalJSON_ShortString(t *testing.T) {
	var ts Time
	err := ts.UnmarshalJSON([]byte(`"ab"`))
	if err == nil {
		t.Fatal("expected error for short non-date string")
	}
}

func TestMarshalCBOR(t *testing.T) {
	ts := NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))
	b, err := ts.MarshalCBOR()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("expected non-empty CBOR data")
	}
	if b[0] != 0xc0 {
		t.Fatalf("expected CBOR tag 0 (0xc0), got 0x%02x", b[0])
	}
}

func TestMarshalUnmarshalCBOR_Roundtrip(t *testing.T) {
	original := NewTime(time.Date(2024, 6, 15, 14, 30, 45, 123000000, time.UTC))
	b, err := original.MarshalCBOR()
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded Time
	if err := decoded.UnmarshalCBOR(b); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	want := original.UTC().Format(RFC3339Millis)
	got := decoded.UTC().Format(RFC3339Millis)
	if got != want {
		t.Fatalf("roundtrip mismatch: want %s, got %s", want, got)
	}
}

func TestUnmarshalCBOR_EmptyData(t *testing.T) {
	var ts Time
	err := ts.UnmarshalCBOR(nil)
	if err == nil {
		t.Fatal("expected error for empty CBOR data")
	}
}

func TestUnmarshalCBOR_BareTextString(t *testing.T) {
	s := "2024-01-15T10:30:00.000Z"
	data := appendCBORTextString(nil, s)
	var ts Time
	if err := ts.UnmarshalCBOR(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.Hour() != 10 {
		t.Fatalf("expected hour 10, got %d", ts.Hour())
	}
}

func TestUnmarshalCBOR_RFC3339Fallback(t *testing.T) {
	s := "2024-01-15T10:30:00Z"
	data := make([]byte, 0, 2+len(s))
	data = append(data, 0xc0)
	data = appendCBORTextString(data, s)
	var ts Time
	if err := ts.UnmarshalCBOR(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnmarshalCBOR_InvalidTimeString(t *testing.T) {
	s := "not-a-date"
	data := make([]byte, 0, 2+len(s))
	data = append(data, 0xc0)
	data = appendCBORTextString(data, s)
	var ts Time
	if err := ts.UnmarshalCBOR(data); err == nil {
		t.Fatal("expected error for invalid time string in CBOR")
	}
}

func TestUnmarshalCBOR_InvalidMajorType(t *testing.T) {
	var ts Time
	err := ts.UnmarshalCBOR([]byte{0x01})
	if err == nil {
		t.Fatal("expected error for non-text-string CBOR")
	}
}

func TestAppendCBORTextString_ShortString(t *testing.T) {
	s := "hello"
	data := appendCBORTextString(nil, s)
	if data[0] != 0x60+byte(len(s)) {
		t.Fatalf("expected direct length encoding, got 0x%02x", data[0])
	}
	if string(data[1:]) != s {
		t.Fatalf("expected %q, got %q", s, string(data[1:]))
	}
}

func TestAppendCBORTextString_MediumString(t *testing.T) {
	s := make([]byte, 100)
	for i := range s {
		s[i] = 'a'
	}
	data := appendCBORTextString(nil, string(s))
	if data[0] != 0x78 {
		t.Fatalf("expected 1-byte length encoding (0x78), got 0x%02x", data[0])
	}
	if data[1] != 100 {
		t.Fatalf("expected length 100, got %d", data[1])
	}
}

func TestAppendCBORTextString_LargeString(t *testing.T) {
	s := make([]byte, 300)
	for i := range s {
		s[i] = 'b'
	}
	data := appendCBORTextString(nil, string(s))
	if data[0] != 0x79 {
		t.Fatalf("expected 2-byte length encoding (0x79), got 0x%02x", data[0])
	}
	length := int(data[1])<<8 | int(data[2])
	if length != 300 {
		t.Fatalf("expected length 300, got %d", length)
	}
}

func TestDecodeCBORTextString_Empty(t *testing.T) {
	_, err := decodeCBORTextString(nil)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestDecodeCBORTextString_NonTextMajorType(t *testing.T) {
	_, err := decodeCBORTextString([]byte{0x01})
	if err == nil {
		t.Fatal("expected error for non-text major type")
	}
}

func TestDecodeCBORTextString_ShortLength(t *testing.T) {
	s := "test"
	data := appendCBORTextString(nil, s)
	got, err := decodeCBORTextString(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != s {
		t.Fatalf("expected %q, got %q", s, got)
	}
}

func TestDecodeCBORTextString_OneByteLengthTruncated(t *testing.T) {
	_, err := decodeCBORTextString([]byte{0x78})
	if err == nil {
		t.Fatal("expected error for truncated 1-byte length")
	}
}

func TestDecodeCBORTextString_TwoByteLengthTruncated(t *testing.T) {
	_, err := decodeCBORTextString([]byte{0x79, 0x00})
	if err == nil {
		t.Fatal("expected error for truncated 2-byte length")
	}
}

func TestDecodeCBORTextString_UnsupportedLengthEncoding(t *testing.T) {
	_, err := decodeCBORTextString([]byte{0x7b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05})
	if err == nil {
		t.Fatal("expected error for unsupported length encoding")
	}
}

func TestDecodeCBORTextString_TruncatedPayload(t *testing.T) {
	_, err := decodeCBORTextString([]byte{0x65, 'h', 'e'})
	if err == nil {
		t.Fatal("expected error for truncated payload")
	}
}

func TestDecodeCBORTextString_OneByteLengthValid(t *testing.T) {
	s := make([]byte, 50)
	for i := range s {
		s[i] = 'x'
	}
	data := appendCBORTextString(nil, string(s))
	got, err := decodeCBORTextString(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != string(s) {
		t.Fatalf("decoded string mismatch")
	}
}

func TestDecodeCBORTextString_TwoByteLengthValid(t *testing.T) {
	s := make([]byte, 300)
	for i := range s {
		s[i] = 'y'
	}
	data := appendCBORTextString(nil, string(s))
	got, err := decodeCBORTextString(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != string(s) {
		t.Fatalf("decoded string mismatch")
	}
}

func TestNewTime(t *testing.T) {
	now := time.Now()
	ts := NewTime(now)
	if !ts.Equal(now) {
		t.Fatal("NewTime should wrap the given time")
	}
}

func TestNow(t *testing.T) {
	before := time.Now()
	ts := Now()
	after := time.Now()
	if ts.Before(before) || ts.After(after) {
		t.Fatal("Now() should return current time")
	}
}

func TestTimeJSONRoundtrip(t *testing.T) {
	original := NewTime(time.Date(2024, 3, 20, 15, 45, 30, 500000000, time.UTC))
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded Time
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	want := "2024-03-20T15:45:30.500Z"
	got := decoded.UTC().Format(RFC3339Millis)
	if got != want {
		t.Fatalf("roundtrip mismatch: want %s, got %s", want, got)
	}
}

func TestUnmarshalJSON_RFC3339Millis(t *testing.T) {
	var ts Time
	if err := ts.UnmarshalJSON([]byte(`"2024-01-15T10:30:00.123Z"`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.Nanosecond() != 123000000 {
		t.Fatalf("expected 123ms, got %dns", ts.Nanosecond())
	}
}

func TestDecodeCBORTextString_TwoByteLengthTruncatedPayload(t *testing.T) {
	_, err := decodeCBORTextString([]byte{0x79, 0x01, 0x00, 'a', 'b'})
	if err == nil {
		t.Fatal("expected error for truncated 2-byte length payload")
	}
}
