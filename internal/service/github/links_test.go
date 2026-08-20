package github

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSplitLinkValuesTracksTargetsAndQuotedStrings(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  []string
	}{
		{
			name:  "comma in target",
			value: `<https://example.test/a,b>; rel=next, <https://example.test/c>; rel=prev`,
			want: []string{
				`<https://example.test/a,b>; rel=next`,
				`<https://example.test/c>; rel=prev`,
			},
		},
		{
			name:  "comma in quoted string",
			value: `<https://example.test/a>; title="a,b"; rel=next`,
			want:  []string{`<https://example.test/a>; title="a,b"; rel=next`},
		},
		{
			name:  "escaped quote before comma",
			value: `<https://example.test/a>; title="a\",b"; rel=next, <https://example.test/c>; rel=prev`,
			want: []string{
				`<https://example.test/a>; title="a\",b"; rel=next`,
				`<https://example.test/c>; rel=prev`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := splitLinkValues(test.value)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("splitLinkValues(%q) = %#v, %v; want %#v, nil", test.value, got, err, test.want)
			}
		})
	}
}

func TestSplitLinkValuesRejectsMalformedState(t *testing.T) {
	for _, value := range []string{
		`https://example.test/a>; rel=next`,
		`<<https://example.test/a>>; rel=next`,
		`<https://example.test/a; rel=next`,
		`<https://example.test/a>; title="unterminated`,
		`<https://example.test/a>; title="dangling\`,
		"\u00a0<https://example.test/a>; rel=next",
	} {
		if got, err := splitLinkValues(value); !errors.Is(err, ErrUpstream) || got != nil {
			t.Fatalf("splitLinkValues(%q) = %#v, %v; want nil, ErrUpstream", value, got, err)
		}
	}
}

func TestSplitLinkParametersTracksQuotedStrings(t *testing.T) {
	for _, test := range []struct {
		value string
		want  []string
	}{
		{
			value: `rel=next; title="a;b"; preload`,
			want:  []string{"rel=next", ` title="a;b"`, " preload"},
		},
		{
			value: `title="a\";b"; rel=next`,
			want:  []string{`title="a\";b"`, " rel=next"},
		},
	} {
		got, err := splitLinkParameters(test.value)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("splitLinkParameters(%q) = %#v, %v; want %#v, nil", test.value, got, err, test.want)
		}
	}
	for _, value := range []string{`title="unterminated`, `title="dangling\`} {
		if got, err := splitLinkParameters(value); !errors.Is(err, ErrUpstream) || got != nil {
			t.Fatalf("splitLinkParameters(%q) = %#v, %v; want nil, ErrUpstream", value, got, err)
		}
	}
}

func TestParseLinkParameterValueExactGrammar(t *testing.T) {
	for _, test := range []struct {
		value string
		want  string
	}{
		{value: "AZaz09!#$%&'*+-.^_`|~", want: "AZaz09!#$%&'*+-.^_`|~"},
		{value: "\"\t !#[~\x80\"", want: "\t !#[~\x80"},
		{value: `"escaped\"quote\\slash"`, want: `escaped"quote\slash`},
	} {
		got, err := parseLinkParameterValue(test.value)
		if err != nil || got != test.want {
			t.Fatalf("parseLinkParameterValue(%q) = %q, %v; want %q, nil", test.value, got, err, test.want)
		}
	}

	for _, value := range []string{
		"", "(", ")", "/", ":", ";", "=", "?", "@", "[", "]", "{", "}", " ", "\t", "\x00", "é",
		`"raw"quote"`, "\"\x08\"", "\"\x7f\"", `"dangling\"`,
	} {
		if got, err := parseLinkParameterValue(value); !errors.Is(err, ErrUpstream) || got != "" {
			t.Fatalf("parseLinkParameterValue(%q) = %q, %v; want empty, ErrUpstream", value, got, err)
		}
	}
}

func TestParseRelationTypesExactGrammar(t *testing.T) {
	for _, test := range []struct {
		value string
		want  []string
	}{
		{value: "NEXT", want: []string{"next"}},
		{value: "next  prev", want: []string{"next", "prev"}},
		{value: "a0.-Z", want: []string{"a0.-z"}},
		{value: "a:x", want: []string{"a:x"}},
		{value: "urn:example:test", want: []string{"urn:example:test"}},
		{value: "https://example.test/relation#part", want: []string{"https://example.test/relation#part"}},
	} {
		got, err := parseRelationTypes(test.value)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("parseRelationTypes(%q) = %#v, %v; want %#v, nil", test.value, got, err, test.want)
		}
	}

	for _, value := range []string{
		"", " next", "next ", "next\tprev", "next\u00a0prev", "_next", "1next", "relative/path", "relative/path:x", "urn:%",
		"urn:[", "urn:hello|world",
	} {
		if got, err := parseRelationTypes(value); !errors.Is(err, ErrUpstream) || got != nil {
			t.Fatalf("parseRelationTypes(%q) = %#v, %v; want nil, ErrUpstream", value, got, err)
		}
	}
}

func TestValidURICharactersExactBoundaries(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: "urn:example:test", want: true},
		{value: "https://[2001:db8::1]/a%20b?q=x/y?z#fragment", want: true},
		{value: "scheme:", want: true},
		{value: "relative/path"},
		{value: "urn:%"},
		{value: "urn:%0"},
		{value: "urn:%0g"},
		{value: "urn:["},
		{value: "urn:]"},
		{value: "urn:hello|world"},
		{value: "urn:hello#one#two"},
		{value: "urn:héllo"},
	} {
		if got := validURICharacters(test.value); got != test.want {
			t.Fatalf("validURICharacters(%q) = %t, want %t", test.value, got, test.want)
		}
	}
}

func TestValidRegisteredRelationExactCharacterSet(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{value: ""},
		{value: "A", want: true},
		{value: "a0", want: true},
		{value: "a9", want: true},
		{value: "a.-Z", want: true},
		{value: "0a"},
		{value: "a/"},
		{value: "a:"},
		{value: "a_"},
	} {
		if got := validRegisteredRelation(test.value); got != test.want {
			t.Fatalf("validRegisteredRelation(%q) = %t, want %t", test.value, got, test.want)
		}
	}
}

func TestLinkGrammarByteBoundaries(t *testing.T) {
	for _, test := range []struct {
		value byte
		want  bool
	}{
		{value: '@'},
		{value: 'A', want: true},
		{value: 'Z', want: true},
		{value: '['},
		{value: '`'},
		{value: 'a', want: true},
		{value: 'z', want: true},
		{value: '{'},
	} {
		if got := asciiLetter(test.value); got != test.want {
			t.Fatalf("asciiLetter(%#x) = %t, want %t", test.value, got, test.want)
		}
	}

	for _, test := range []struct {
		value byte
		text  bool
		pair  bool
	}{
		{value: 0x08},
		{value: 0x09, text: true, pair: true},
		{value: 0x1f},
		{value: 0x20, text: true, pair: true},
		{value: 0x21, text: true, pair: true},
		{value: 0x22, pair: true},
		{value: 0x23, text: true, pair: true},
		{value: 0x5b, text: true, pair: true},
		{value: 0x5c, pair: true},
		{value: 0x5d, text: true, pair: true},
		{value: 0x7e, text: true, pair: true},
		{value: 0x7f},
		{value: 0x80, text: true, pair: true},
	} {
		if got := validQuotedTextByte(test.value); got != test.text {
			t.Errorf("validQuotedTextByte(%#x) = %t, want %t", test.value, got, test.text)
		}
		if got := validQuotedPairByte(test.value); got != test.pair {
			t.Errorf("validQuotedPairByte(%#x) = %t, want %t", test.value, got, test.pair)
		}
	}
}

func TestValidLinkTokenExactCharacterSet(t *testing.T) {
	if value := "AZaz09!#$%&'*+-.^_`|~"; !validLinkToken(value) {
		t.Fatalf("validLinkToken(%q) = false, want true", value)
	}
	for _, value := range []string{
		"", "(", ")", "/", ":", ";", "=", "?", "@", "[", "]", "{", "}", " ", "\t", "é",
	} {
		if validLinkToken(value) {
			t.Fatalf("validLinkToken(%q) = true, want false", value)
		}
	}
}

func FuzzProviderLinkParsing(f *testing.F) {
	for _, seed := range []string{
		`<https://api.example.test/items?page=2>; rel="next"`,
		`<https://api.example.test/a,b>; title="a,b"; rel="last NEXT"`,
		`<https://api.example.test/items>; preload`,
		`<unterminated`,
		"<https://api.example.test/items>; rel=\"next\u00a0prev\"",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		parts, err := splitLinkValues(value)
		if err != nil {
			return
		}
		if len(parts) == 0 {
			t.Fatal("successful Link split returned no values")
		}
		for _, part := range parts {
			target, relations, _, parseErr := parseLinkValue(part)
			if parseErr != nil {
				continue
			}
			if target == "" {
				t.Fatal("successful link-value parse returned an empty target")
			}
			for _, relation := range relations {
				if relation == "" {
					t.Fatal("successful link-value parse returned an empty relation")
				}
				if validRegisteredRelation(relation) {
					if relation != strings.ToLower(relation) {
						t.Fatalf("registered relation %q was not normalized", relation)
					}
					continue
				}
				if normalized, ok := normalizeRelationType(relation); !ok || normalized != relation {
					t.Fatalf("extension relation %q no longer validates", relation)
				}
			}
		}
	})
}
