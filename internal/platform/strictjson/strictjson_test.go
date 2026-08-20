package strictjson

import "testing"

func TestValidateAcceptsOneStrictJSONValue(t *testing.T) {
	for _, document := range [][]byte{
		[]byte(`null`),
		[]byte(`{"name":"Ada","nested":{"items":[true,1,null]}}`),
		[]byte(`{"futureNumber":1e1000}`),
		[]byte(`{"value":"\uD800\uDC00"}`),
		[]byte(`{"value":"\uDBFF\uDFFF"}`),
		[]byte(`{"value":"\uD7FF\uE000"}`),
		[]byte(`{"value":"\\uD800"}`),
	} {
		if err := Validate(document); err != nil {
			t.Fatalf("Validate(%q) = %v, want nil", document, err)
		}
	}
}

func TestValidateRejectsAmbiguousOrNonUnicodeJSON(t *testing.T) {
	tests := []struct {
		name     string
		document []byte
	}{
		{name: "byte-order mark", document: append([]byte{0xef, 0xbb, 0xbf}, []byte(`{}`)...)},
		{name: "invalid UTF-8", document: []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}},
		{name: "root duplicate", document: []byte(`{"name":"Ada","name":"Grace"}`)},
		{name: "escaped duplicate", document: []byte(`{"a":1,"\u0061":2}`)},
		{name: "nested duplicate", document: []byte(`{"outer":{"x":1,"x":2}}`)},
		{name: "lone high surrogate", document: []byte(`{"value":"\uD800"}`)},
		{name: "lone low surrogate", document: []byte(`{"value":"\uDC00"}`)},
		{name: "high surrogate followed by text", document: []byte(`{"value":"\uD800xxxxxx"}`)},
		{name: "high surrogate followed by another escape", document: []byte(`{"value":"\uD800\nxxxx"}`)},
		{name: "low surrogate below range", document: []byte(`{"value":"\uD800\uDBFF"}`)},
		{name: "low surrogate above range", document: []byte(`{"value":"\uD800\uE000"}`)},
		{name: "maximum lone low surrogate", document: []byte(`{"value":"\uDFFF"}`)},
		{name: "malformed", document: []byte(`{"name":`)},
		{name: "trailing object", document: []byte(`{} {}`)},
		{name: "trailing scalar", document: []byte(`null true`)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Validate(test.document); err == nil {
				t.Fatalf("Validate(%q) unexpectedly succeeded", test.document)
			}
		})
	}
}
