package github

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/janisto/echo-playground/internal/platform/httpheader"
	"github.com/janisto/echo-playground/internal/platform/pagination"
)

type paginationKind uint8

const (
	noPagination paginationKind = iota
	numberedPagination
	activityPagination
)

type providerSpec struct {
	origin        *url.URL
	namedPath     string
	numericPrefix string
	numericSuffix string
	query         url.Values
	pagination    paginationKind
	currentValue  string
	scope         pagination.Scope
}

type navigation struct {
	nextCursor string
	prevCursor string
}

func parseNavigation(header http.Header, spec providerSpec, emptyPage bool) (navigation, error) {
	var result navigation
	seen := map[string]bool{"next": false, "prev": false}
	for _, field := range header.Values("Link") {
		values, err := splitLinkValues(field)
		if err != nil {
			return navigation{}, err
		}
		for _, rawValue := range values {
			target, relations, anchored, err := parseLinkValue(rawValue)
			if err != nil {
				return navigation{}, ErrUpstream
			}
			if anchored {
				continue
			}
			for _, relation := range relations {
				if relation != "next" && relation != "prev" {
					continue
				}
				if seen[relation] {
					return navigation{}, ErrUpstream
				}
				seen[relation] = true
				position, err := validateLinkTarget(target, relation, spec)
				if err != nil {
					return navigation{}, err
				}
				cursor := pagination.NewCursor(spec.scope, relation, position).Encode()
				if len(cursor) > pagination.MaxCursorLength {
					return navigation{}, ErrUpstream
				}
				if relation == "next" {
					result.nextCursor = cursor
				} else {
					result.prevCursor = cursor
				}
			}
		}
	}
	if emptyPage && result.nextCursor != "" {
		return navigation{}, ErrUpstream
	}
	return result, nil
}

func splitLinkValues(value string) ([]string, error) {
	if httpheader.HasNonHTTPWhitespace(value) {
		return nil, ErrUpstream
	}
	parts := make([]string, 0, 2)
	start, quoted, escaped, inTarget := 0, false, false, false
	for index := range len(value) {
		switch {
		case escaped:
			escaped = false
		case value[index] == '\\' && quoted:
			escaped = true
		case value[index] == '"':
			quoted = !quoted
		case value[index] == '<' && !quoted:
			if inTarget {
				return nil, ErrUpstream
			}
			inTarget = true
		case value[index] == '>' && !quoted:
			if !inTarget {
				return nil, ErrUpstream
			}
			inTarget = false
		case value[index] == ',' && !quoted && !inTarget:
			parts = append(parts, trimOWS(value[start:index]))
			start = index + 1
		}
	}
	if quoted || escaped || inTarget {
		return nil, ErrUpstream
	}
	return append(parts, trimOWS(value[start:])), nil
}

func parseLinkValue(raw string) (string, []string, bool, error) {
	raw = trimOWS(raw)
	if httpheader.HasNonHTTPWhitespace(raw) {
		return "", nil, false, ErrUpstream
	}
	if raw == "" || raw[0] != '<' {
		return "", nil, false, ErrUpstream
	}
	closeIndex := strings.IndexByte(raw, '>')
	if closeIndex < 2 {
		return "", nil, false, ErrUpstream
	}
	target := raw[1:closeIndex]
	remainder := trimOWS(raw[closeIndex+1:])
	if remainder == "" {
		return target, nil, false, nil
	}
	if remainder[0] != ';' {
		return "", nil, false, ErrUpstream
	}
	parameters, err := splitLinkParameters(remainder[1:])
	if err != nil {
		return "", nil, false, err
	}
	var relations []string
	anchored := false
	seenRelation := false
	for _, rawParameter := range parameters {
		name, rawValue, hasValue := strings.Cut(trimOWS(rawParameter), "=")
		name = trimOWS(name)
		if !validLinkToken(name) {
			return "", nil, false, ErrUpstream
		}
		name = strings.ToLower(name)
		if name == "rel" && seenRelation {
			continue
		}
		value := ""
		if hasValue {
			value, err = parseLinkParameterValue(trimOWS(rawValue))
			if err != nil {
				return "", nil, false, ErrUpstream
			}
		}
		switch name {
		case "anchor":
			anchored = true
		case "rel":
			seenRelation = true
			if !hasValue {
				return "", nil, false, ErrUpstream
			}
			parsedRelations, relationErr := parseRelationTypes(value)
			if relationErr != nil {
				return "", nil, false, relationErr
			}
			relations = append(relations, parsedRelations...)
		}
	}
	return target, relations, anchored, nil
}

func splitLinkParameters(value string) ([]string, error) {
	parts := make([]string, 0, 3)
	start, quoted, escaped := 0, false, false
	for index := range len(value) {
		switch {
		case escaped:
			escaped = false
		case value[index] == '\\' && quoted:
			escaped = true
		case value[index] == '"':
			quoted = !quoted
		case value[index] == ';' && !quoted:
			parts = append(parts, value[start:index])
			start = index + 1
		}
	}
	if quoted || escaped {
		return nil, ErrUpstream
	}
	return append(parts, value[start:]), nil
}

func parseLinkParameterValue(value string) (string, error) {
	if value == "" {
		return "", ErrUpstream
	}
	if value[0] != '"' {
		if !validLinkToken(value) {
			return "", ErrUpstream
		}
		return value, nil
	}
	if len(value) < 2 || value[len(value)-1] != '"' {
		return "", ErrUpstream
	}
	var decoded strings.Builder
	escaped := false
	for index := 1; index < len(value)-1; index++ {
		character := value[index]
		if escaped {
			if !validQuotedPairByte(character) {
				return "", ErrUpstream
			}
			decoded.WriteByte(character)
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == '"' || !validQuotedTextByte(character) {
			return "", ErrUpstream
		}
		decoded.WriteByte(character)
	}
	if escaped {
		return "", ErrUpstream
	}
	return decoded.String(), nil
}

func parseRelationTypes(value string) ([]string, error) {
	if value == "" || value[0] == ' ' || value[len(value)-1] == ' ' {
		return nil, ErrUpstream
	}
	relations := make([]string, 0, 2)
	for rawRelation := range strings.SplitSeq(value, " ") {
		if rawRelation == "" {
			continue
		}
		relation, ok := normalizeRelationType(rawRelation)
		if !ok {
			return nil, ErrUpstream
		}
		relations = append(relations, relation)
	}
	if len(relations) == 0 {
		return nil, ErrUpstream
	}
	return relations, nil
}

func normalizeRelationType(value string) (string, bool) {
	if validRegisteredRelation(value) {
		return strings.ToLower(value), true
	}
	if !printableASCII(value, len(value)) {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", false
	}
	if !parsed.IsAbs() || !validURICharacters(value) {
		return "", false
	}
	return value, true
}

func validURICharacters(value string) bool {
	_, remainder, hasScheme := strings.Cut(value, ":")
	if !hasScheme {
		return false
	}
	if _, err := url.PathUnescape(value); err != nil {
		return false
	}
	remainder, inAuthority := strings.CutPrefix(remainder, "//")
	fragmentSeen := false
	for index := range len(remainder) {
		character := remainder[index]
		if inAuthority && strings.ContainsRune("/?#", rune(character)) {
			inAuthority = false
		}
		switch {
		case character == '#':
			if fragmentSeen {
				return false
			}
			fragmentSeen = true
		case character == '[' || character == ']':
			if !inAuthority {
				return false
			}
		case asciiLetter(character) || character >= '0' && character <= '9' ||
			strings.ContainsRune("%-._~!$&'()*+,;=:/?@", rune(character)):
		default:
			return false
		}
	}
	return true
}

func validRegisteredRelation(value string) bool {
	if value == "" || !asciiLetter(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !asciiLetter(character) && (character < '0' || character > '9') && character != '.' && character != '-' {
			return false
		}
	}
	return true
}

func validLinkToken(value string) bool {
	if value == "" {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if !asciiLetter(character) && (character < '0' || character > '9') &&
			!strings.ContainsRune("!#$%&'*+-.^_`|~", rune(character)) {
			return false
		}
	}
	return true
}

func asciiLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func validQuotedTextByte(value byte) bool {
	return value == '\t' || value == ' ' || value == '!' || value >= '#' && value <= '[' ||
		value >= ']' && value <= '~' || value >= 0x80
}

func validQuotedPairByte(value byte) bool {
	return value == '\t' || value == ' ' || value >= '!' && value <= '~' || value >= 0x80
}

func trimOWS(value string) string {
	return strings.Trim(value, " \t")
}

func validateLinkTarget(target, relation string, spec providerSpec) (string, error) {
	parsed, err := url.Parse(target)
	if err != nil || spec.origin == nil || parsed.Scheme != spec.origin.Scheme || parsed.Host != spec.origin.Host ||
		parsed.User != nil || parsed.Fragment != "" || parsed.ForceQuery ||
		parsed.EscapedPath() != (&url.URL{Path: parsed.Path}).EscapedPath() ||
		parsed.Path != spec.namedPath && !matchesNumericPath(parsed.Path, spec.numericPrefix, spec.numericSuffix) {
		return "", ErrUpstream
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", ErrUpstream
	}
	for _, values := range query {
		if len(values) != 1 {
			return "", ErrUpstream
		}
	}
	expected := cloneQuery(spec.query)
	expected.Del("page")
	expected.Del("before")
	expected.Del("after")
	var position string
	switch spec.pagination {
	case numberedPagination:
		position = query.Get("page")
		if !canonicalPageDecimal(position) {
			return "", ErrUpstream
		}
		expected.Set("page", position)
		current := uint64(1)
		if spec.currentValue != "" {
			current, _ = strconv.ParseUint(spec.currentValue, 10, 64)
		}
		targetPage, _ := strconv.ParseUint(position, 10, 64)
		if relation == "next" && targetPage <= current || relation == "prev" && targetPage >= current {
			return "", ErrUpstream
		}
	case activityPagination:
		member := "after"
		if relation == "prev" {
			member = "before"
		}
		position = query.Get(member)
		if !printableASCII(position, 2048) || spec.currentValue == "" && relation == "prev" ||
			position == spec.currentValue {
			return "", ErrUpstream
		}
		expected.Set(member, position)
	default:
		return "", ErrUpstream
	}
	if query.Encode() != expected.Encode() {
		return "", ErrUpstream
	}
	return position, nil
}

func matchesNumericPath(path, prefix, suffix string) bool {
	if prefix == "" || !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return id != "" && !strings.Contains(id, "/") && canonicalSafeDecimal(id)
}

func canonicalSafeDecimal(value string) bool {
	if value == "" || value != "0" && value[0] == '0' {
		return false
	}
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return err == nil && parsed <= maximumSafeInteger
}

func canonicalPageDecimal(value string) bool {
	return value != "0" && canonicalSafeDecimal(value)
}

func printableASCII(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for index := range len(value) {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func cloneQuery(query url.Values) url.Values {
	cloned := make(url.Values, len(query))
	for name, values := range query {
		cloned[name] = append([]string(nil), values...)
	}
	return cloned
}
