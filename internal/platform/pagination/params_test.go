package pagination

import "testing"

func TestParams_DefaultLimit(t *testing.T) {
	p := Params{Limit: 0}
	if p.DefaultLimit() != DefaultLimit {
		t.Fatalf("expected %d, got %d", DefaultLimit, p.DefaultLimit())
	}
}

func TestParams_DefaultLimit_Negative(t *testing.T) {
	p := Params{Limit: -1}
	if p.DefaultLimit() != DefaultLimit {
		t.Fatalf("expected %d, got %d", DefaultLimit, p.DefaultLimit())
	}
}

func TestParams_DefaultLimit_Positive(t *testing.T) {
	p := Params{Limit: 50}
	if p.DefaultLimit() != 50 {
		t.Fatalf("expected 50, got %d", p.DefaultLimit())
	}
}

func TestConstants(t *testing.T) {
	if DefaultLimit != 20 {
		t.Fatalf("expected DefaultLimit=20, got %d", DefaultLimit)
	}
	if MaxLimit != 100 {
		t.Fatalf("expected MaxLimit=100, got %d", MaxLimit)
	}
}
