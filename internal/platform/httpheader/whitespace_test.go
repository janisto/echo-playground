package httpheader

import "testing"

func TestHasNonHTTPWhitespaceDistinguishesSyntaxFromQuotedContent(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "ordinary field", value: "application/json; charset=utf-8"},
		{name: "SP and HTAB", value: " application/json\t; charset=utf-8 "},
		{name: "quoted Unicode content", value: "application/json; note=\"a\u00a0b\""},
		{name: "escaped quote", value: "application/json; note=\"a\\\"\u00a0b\""},
		{name: "leading no-break space", value: "\u00a0application/json", want: true},
		{name: "Unicode separator", value: "application/json\u2003;charset=utf-8", want: true},
		{name: "line feed", value: "application/json\n", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := HasNonHTTPWhitespace(test.value); got != test.want {
				t.Fatalf("HasNonHTTPWhitespace(%q) = %t, want %t", test.value, got, test.want)
			}
		})
	}
}
