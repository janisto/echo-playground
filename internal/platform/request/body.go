// Package request contains strict HTTP request decoding helpers.
package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/httpheader"
	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/strictjson"
	"github.com/janisto/echo-playground/internal/platform/validate"
)

// BodyLimit is the exact decimal portable request-body limit.
const BodyLimit int64 = 1_000_000

type requestFormat uint8

const (
	requestFormatMissing requestFormat = iota
	requestFormatJSON
	requestFormatCBOR
)

var (
	cborSyntaxMode = mustCBORMode(cbor.DecOptions{
		DupMapKey: cbor.DupMapKeyEnforcedAPF,
		UTF8:      cbor.UTF8RejectInvalid,
	})
	cborSchemaMode = mustCBORMode(cbor.DecOptions{
		DupMapKey:         cbor.DupMapKeyEnforcedAPF,
		UTF8:              cbor.UTF8RejectInvalid,
		ExtraReturnErrors: cbor.ExtraDecErrorUnknownField,
		TagsMd:            cbor.TagsForbidden,
	})
)

func mustCBORMode(options cbor.DecOptions) cbor.DecMode {
	mode, err := options.DecMode()
	if err != nil {
		panic(err)
	}
	return mode
}

// BodyLimitMiddleware enforces the portable limit only after a supported
// body-bearing operation has been selected. Unsupported methods and body-free
// routes are never read or classified by this middleware.
func BodyLimitMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if !isBodyOperation(c.Request().Method, c.Request().URL.Path) {
				return next(c)
			}
			request := c.Request()
			if request.ContentLength > BodyLimit {
				return respond.PayloadTooLarge()
			}
			if request.ContentLength < -1 {
				return respond.InvalidRequest()
			}
			request.Body = http.MaxBytesReader(c.Response(), request.Body, BodyLimit)
			return next(c)
		}
	}
}

func isBodyOperation(method, path string) bool {
	return method == http.MethodPost && (path == "/v1/hello" || path == "/v1/profile") ||
		method == http.MethodPatch && path == "/v1/profile"
}

// Decode decodes exactly one GCP-profile JSON or CBOR request document.
// Syntax failures are 400; a well-formed document outside the operation schema
// is 422.
func Decode(c *echo.Context, target any) error {
	format, mediaErr := requestMediaType(c.Request().Header)
	if mediaErr != nil {
		return mediaErr
	}
	encodingErr := requestContentEncoding(c.Request().Header)
	if encodingErr != nil {
		return encodingErr
	}

	body, readErr := io.ReadAll(c.Request().Body)
	if readErr != nil {
		if maxBytesError := (*http.MaxBytesError)(nil); errors.As(readErr, &maxBytesError) {
			return respond.PayloadTooLarge()
		}
		if errors.Is(readErr, echo.ErrStatusRequestEntityTooLarge) {
			return respond.PayloadTooLarge()
		}
		return respond.InvalidRequest()
	}
	if len(body) == 0 {
		return respond.InvalidRequest()
	}
	if format == requestFormatMissing {
		return respond.UnsupportedMediaType()
	}

	switch format {
	case requestFormatJSON:
		return decodeJSON(body, target)
	case requestFormatCBOR:
		return decodeCBOR(body, target)
	default:
		return respond.UnsupportedMediaType()
	}
}

func requestMediaType(header http.Header) (requestFormat, error) {
	values := header.Values("Content-Type")
	if len(values) == 0 {
		return requestFormatMissing, nil
	}
	if len(values) != 1 || httpheader.HasNonHTTPWhitespace(values[0]) {
		return 0, respond.UnsupportedMediaType()
	}
	mediaType, parameters, err := mime.ParseMediaType(values[0])
	if err != nil {
		return 0, respond.UnsupportedMediaType()
	}
	parameterNames := mediaTypeParameterNames(values[0])
	switch strings.ToLower(mediaType) {
	case respond.MediaTypeJSON:
		if len(parameters) == 0 && len(parameterNames) == 0 {
			return requestFormatJSON, nil
		}
		charset, ok := parameters["charset"]
		if len(parameters) != 1 || len(parameterNames) != 1 || parameterNames[0] != "charset" || !ok ||
			!strings.EqualFold(charset, "utf-8") {
			return 0, respond.UnsupportedMediaType()
		}
		return requestFormatJSON, nil
	case respond.MediaTypeCBOR:
		if len(parameters) != 0 || len(parameterNames) != 0 {
			return 0, respond.UnsupportedMediaType()
		}
		return requestFormatCBOR, nil
	default:
		return 0, respond.UnsupportedMediaType()
	}
}

func mediaTypeParameterNames(value string) []string {
	segments := make([]string, 0, 2)
	start, quoted, escaped := 0, false, false
	for index := range len(value) {
		switch {
		case escaped:
			escaped = false
		case quoted && value[index] == '\\':
			escaped = true
		case value[index] == '"':
			quoted = !quoted
		case value[index] == ';' && !quoted:
			segments = append(segments, value[start:index])
			start = index + 1
		}
	}
	segments = append(segments, value[start:])
	parameters := make([]string, 0, len(segments))
	for _, segment := range segments[1:] {
		segment = strings.Trim(segment, " \t")
		if segment == "" {
			continue
		}
		name, _, _ := strings.Cut(segment, "=")
		parameters = append(parameters, strings.ToLower(strings.Trim(name, " \t")))
	}
	return parameters
}

func requestContentEncoding(header http.Header) error {
	values := header.Values("Content-Encoding")
	if len(values) == 0 {
		return nil
	}
	if len(values) != 1 || strings.Contains(values[0], ",") ||
		!strings.EqualFold(strings.Trim(values[0], " \t"), "identity") {
		return respond.UnsupportedMediaType()
	}
	return nil
}

func decodeJSON(body []byte, target any) error {
	if err := strictjson.Validate(body); err != nil {
		return respond.InvalidRequest()
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &validate.ValidationError{Message: "validation failed"}
	}
	return nil
}

func decodeCBOR(body []byte, target any) error {
	if err := cborSyntaxMode.Wellformed(body); err != nil {
		return respond.InvalidRequest()
	}
	var generic any
	if err := cborSyntaxMode.Unmarshal(body, &generic); err != nil {
		return respond.InvalidRequest()
	}
	if err := cborSchemaMode.Unmarshal(body, target); err != nil {
		if duplicate := (*cbor.DupMapKeyError)(nil); errors.As(err, &duplicate) {
			return respond.InvalidRequest()
		}
		return &validate.ValidationError{Message: "validation failed"}
	}
	return nil
}
