// Package request contains strict HTTP request decoding helpers.
package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/labstack/echo/v5"
)

// DecodeJSON decodes exactly one application/json object and rejects unknown fields.
func DecodeJSON(c *echo.Context, target any) error {
	req := c.Request()
	if req.ContentLength == 0 {
		return echo.ErrBadRequest
	}

	contentTypes := req.Header.Values(echo.HeaderContentType)
	if len(contentTypes) != 1 {
		return echo.ErrUnsupportedMediaType
	}
	mediaType, _, err := mime.ParseMediaType(contentTypes[0])
	if err != nil || mediaType != echo.MIMEApplicationJSON {
		return echo.ErrUnsupportedMediaType
	}

	decoder := json.NewDecoder(req.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return decodeError(err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return decodeError(err)
	}

	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return echo.ErrBadRequest
	}
	if err := validateStrictJSON(raw, reflect.TypeOf(target)); err != nil {
		return echo.ErrBadRequest.Wrap(err)
	}

	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return echo.ErrBadRequest.Wrap(err)
	}
	return nil
}

func decodeError(err error) error {
	if echo.StatusCode(err) == http.StatusRequestEntityTooLarge {
		return echo.ErrStatusRequestEntityTooLarge
	}
	return echo.ErrBadRequest.Wrap(err)
}

func validateStrictJSON(raw []byte, targetType reflect.Type) error {
	if !utf8.Valid(raw) {
		return errors.New("JSON must contain valid UTF-8")
	}
	targetType = dereferenceType(targetType)
	if targetType == nil {
		return errors.New("JSON target type is required")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := validateJSONValue(decoder, targetType); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("read trailing JSON token: %w", err)
	}
	return nil
}

func validateJSONValue(decoder *json.Decoder, targetType reflect.Type) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("read JSON token: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		return validateJSONObject(decoder, targetType)
	case '[':
		elementType := dereferenceType(targetType)
		if elementType != nil && (elementType.Kind() == reflect.Array || elementType.Kind() == reflect.Slice) {
			elementType = elementType.Elem()
		}
		for decoder.More() {
			if err := validateJSONValue(decoder, elementType); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return fmt.Errorf("close JSON array: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
}

func validateJSONObject(decoder *json.Decoder, targetType reflect.Type) error {
	fields := jsonFieldTypes(dereferenceType(targetType))
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("read JSON object name: %w", err)
		}
		name, ok := token.(string)
		if !ok {
			return errors.New("JSON object name is not a string")
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate JSON object name %q", name)
		}
		seen[name] = struct{}{}

		fieldType := reflect.Type(nil)
		if fields != nil {
			var exists bool
			fieldType, exists = fields[name]
			if !exists {
				return fmt.Errorf("unknown JSON field %q", name)
			}
		}
		if err := validateJSONValue(decoder, fieldType); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("close JSON object: %w", err)
	}
	return nil
}

func jsonFieldTypes(targetType reflect.Type) map[string]reflect.Type {
	targetType = dereferenceType(targetType)
	if targetType == nil || targetType.Kind() != reflect.Struct {
		return nil
	}

	fields := make(map[string]reflect.Type)
	for field := range targetType.Fields() {
		if !field.IsExported() {
			continue
		}
		tag, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if tag == "-" {
			continue
		}
		if field.Anonymous && tag == "" {
			maps.Copy(fields, jsonFieldTypes(field.Type))
			continue
		}
		if tag == "" {
			tag = field.Name
		}
		fields[tag] = field.Type
	}
	return fields
}

func dereferenceType(targetType reflect.Type) reflect.Type {
	for targetType != nil && targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	return targetType
}
