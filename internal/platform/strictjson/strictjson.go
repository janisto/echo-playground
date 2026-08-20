// Package strictjson validates the syntax rules that encoding/json accepts
// permissively but the portable contract rejects.
package strictjson

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

// Validate accepts exactly one JSON value with unique object names, valid
// UTF-8, and valid Unicode scalar escapes.
func Validate(document []byte) error {
	if len(document) >= 3 && bytes.Equal(document[:3], []byte{0xef, 0xbb, 0xbf}) {
		return errors.New("JSON byte-order mark is forbidden")
	}
	if !utf8.Valid(document) {
		return errors.New("JSON is not UTF-8")
	}
	if hasInvalidSurrogate(document) {
		return errors.New("JSON contains a lone surrogate")
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	if err := validateValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains trailing data")
	}
	return nil
}

func validateValue(decoder *json.Decoder) error {
	token, tokenErr := decoder.Token()
	if tokenErr != nil {
		return tokenErr
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			nameToken, nameErr := decoder.Token()
			if nameErr != nil {
				return nameErr
			}
			name, ok := nameToken.(string)
			if !ok {
				return errors.New("JSON object name is not text")
			}
			if _, duplicate := seen[name]; duplicate {
				return errors.New("JSON object name is duplicated")
			}
			seen[name] = struct{}{}
			valueErr := validateValue(decoder)
			if valueErr != nil {
				return valueErr
			}
		}
	case '[':
		for decoder.More() {
			valueErr := validateValue(decoder)
			if valueErr != nil {
				return valueErr
			}
		}
	default:
		return errors.New("JSON delimiter is invalid")
	}
	_, closeErr := decoder.Token()
	return closeErr
}

func hasInvalidSurrogate(document []byte) bool {
	inString, escaped := false, false
	for index := 0; index < len(document); index++ {
		if !inString {
			if document[index] == '"' {
				inString = true
			}
			continue
		}
		if escaped {
			escaped = false
			if document[index] != 'u' {
				continue
			}
			value, ok := parseHex16(document, index+1)
			if !ok {
				return false
			}
			index += 4
			switch {
			case value >= 0xd800 && value <= 0xdbff:
				if index+6 >= len(document) || document[index+1] != '\\' || document[index+2] != 'u' {
					return true
				}
				low, lowOK := parseHex16(document, index+3)
				if !lowOK || low < 0xdc00 || low > 0xdfff {
					return true
				}
				index += 6
			case value >= 0xdc00 && value <= 0xdfff:
				return true
			}
			continue
		}
		switch document[index] {
		case '\\':
			escaped = true
		case '"':
			inString = false
		}
	}
	return false
}

func parseHex16(document []byte, offset int) (uint16, bool) {
	if offset+4 > len(document) {
		return 0, false
	}
	var decoded [2]byte
	if _, err := hex.Decode(decoded[:], document[offset:offset+4]); err != nil {
		return 0, false
	}
	return uint16(decoded[0])<<8 | uint16(decoded[1]), true
}
