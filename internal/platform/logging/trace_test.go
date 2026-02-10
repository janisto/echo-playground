package logging

import (
	"testing"
)

func TestTraceAttrs_ValidHeader(t *testing.T) {
	header := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	attrs := traceAttrs(header, "my-project")

	if len(attrs) != 3 {
		t.Fatalf("expected 3 attrs, got %d", len(attrs))
	}

	traceVal := attrs[0].Value.String()
	expected := "projects/my-project/traces/0af7651916cd43dd8448eb211c80319c"
	if traceVal != expected {
		t.Fatalf("expected trace %q, got %q", expected, traceVal)
	}

	spanVal := attrs[1].Value.String()
	if spanVal != "b7ad6b7169203331" {
		t.Fatalf("expected spanId 'b7ad6b7169203331', got %q", spanVal)
	}

	sampled := attrs[2].Value.Bool()
	if !sampled {
		t.Fatal("expected trace_sampled to be true")
	}
}

func TestTraceAttrs_NotSampled(t *testing.T) {
	header := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-00"
	attrs := traceAttrs(header, "my-project")

	if len(attrs) != 3 {
		t.Fatalf("expected 3 attrs, got %d", len(attrs))
	}

	sampled := attrs[2].Value.Bool()
	if sampled {
		t.Fatal("expected trace_sampled to be false")
	}
}

func TestTraceAttrs_EmptyProjectID(t *testing.T) {
	header := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	attrs := traceAttrs(header, "")

	if attrs != nil {
		t.Fatalf("expected nil attrs for empty project, got %d", len(attrs))
	}
}

func TestTraceAttrs_InvalidHeader(t *testing.T) {
	attrs := traceAttrs("invalid-header", "my-project")
	if attrs != nil {
		t.Fatalf("expected nil attrs for invalid header, got %d", len(attrs))
	}
}

func TestTraceAttrs_EmptyHeader(t *testing.T) {
	attrs := traceAttrs("", "my-project")
	if attrs != nil {
		t.Fatalf("expected nil attrs for empty header, got %d", len(attrs))
	}
}

func TestTraceResource_Valid(t *testing.T) {
	header := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	resource := traceResource(header, "my-project")

	expected := "projects/my-project/traces/0af7651916cd43dd8448eb211c80319c"
	if resource != expected {
		t.Fatalf("expected %q, got %q", expected, resource)
	}
}

func TestTraceResource_EmptyProject(t *testing.T) {
	resource := traceResource("00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", "")
	if resource != "" {
		t.Fatalf("expected empty resource for empty project, got %q", resource)
	}
}

func TestTraceResource_InvalidHeader(t *testing.T) {
	resource := traceResource("invalid", "my-project")
	if resource != "" {
		t.Fatalf("expected empty resource for invalid header, got %q", resource)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"first non-empty", []string{"", "b", "c"}, "b"},
		{"all empty", []string{"", "", ""}, ""},
		{"first is value", []string{"a", "b"}, "a"},
		{"no values", []string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonEmpty(tt.values...)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestLoggerWithTrace_NilBase(t *testing.T) {
	header := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	l := loggerWithTrace(nil, header, "my-project", "req-123")
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerWithTrace_NoAttrs(t *testing.T) {
	l := loggerWithTrace(Logger(), "", "", "")
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLoggerWithTrace_WithRequestID(t *testing.T) {
	l := loggerWithTrace(Logger(), "", "", "req-456")
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}
