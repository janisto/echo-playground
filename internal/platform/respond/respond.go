package respond

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/fxamacker/cbor/v2"
	obs "github.com/janisto/echo-observability/v2"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"

	"github.com/janisto/echo-playground/internal/platform/httpheader"
	"github.com/janisto/echo-playground/internal/platform/validate"
)

const (
	MediaTypeCBOR        = "application/cbor"
	MediaTypeJSON        = "application/json"
	MediaTypeProblemJSON = "application/problem+json"
	selectedFormatKey    = "portable-response-format"
)

type responseFormat uint8

const (
	formatNotAcceptable responseFormat = iota
	formatJSON
	formatCBOR
)

type representation struct {
	format      responseFormat
	contentType string
}

type mediaRange struct {
	typ         string
	subtype     string
	params      map[string]string
	quality     int
	specificity int
}

type candidate struct {
	format      responseFormat
	typ         string
	subtype     string
	params      map[string]string
	contentType string
}

var (
	jsonCandidates = []candidate{
		{formatJSON, "application", "json", nil, MediaTypeJSON},
		{formatJSON, "application", "json", map[string]string{"charset": "utf-8"}, MediaTypeJSON + "; charset=utf-8"},
	}
	problemCandidates = []candidate{
		{formatJSON, "application", "problem+json", nil, MediaTypeProblemJSON},
		{
			formatJSON,
			"application",
			"problem+json",
			map[string]string{"charset": "utf-8"},
			MediaTypeProblemJSON + "; charset=utf-8",
		},
		{formatCBOR, "application", "cbor", nil, MediaTypeCBOR},
	}
	successCandidates = append(append([]candidate(nil), jsonCandidates...), candidate{
		formatCBOR, "application", "cbor", nil, MediaTypeCBOR,
	})
)

func splitQuoted(value string, separator byte) []string {
	parts := make([]string, 0, 1)
	start, quoted, escaped := 0, false, false
	for i := range len(value) {
		switch {
		case escaped:
			escaped = false
		case value[i] == '\\' && quoted:
			escaped = true
		case value[i] == '"':
			quoted = !quoted
		case value[i] == separator && !quoted:
			parts = append(parts, value[start:i])
			start = i + 1
		}
	}
	return append(parts, value[start:])
}

func parseQuality(value string) (int, bool) {
	whole, fraction, hasFraction := strings.Cut(value, ".")
	if whole != "0" && whole != "1" || hasFraction && len(fraction) > 3 {
		return 0, false
	}
	for _, digit := range fraction {
		if digit < '0' || digit > '9' || whole == "1" && digit != '0' {
			return 0, false
		}
	}
	for len(fraction) < 3 {
		fraction += "0"
	}
	if fraction == "" {
		fraction = "000"
	}
	quality, err := strconv.Atoi(fraction)
	if err != nil {
		return 0, false
	}
	if whole == "1" {
		quality += 1000
	}
	return quality, true
}

func parseAccept(header string) []mediaRange {
	if header == "" {
		return nil
	}
	ranges := make([]mediaRange, 0, 4)
	for _, rawRange := range splitQuoted(header, ',') {
		segments := splitQuoted(strings.TrimSpace(rawRange), ';')
		if len(segments) == 0 {
			continue
		}
		mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(segments[0]))
		typ, subtype, ok := strings.Cut(strings.ToLower(mediaType), "/")
		if err != nil || !ok || typ == "" || subtype == "" || typ == "*" && subtype != "*" {
			continue
		}
		parsed := mediaRange{typ: typ, subtype: subtype, params: make(map[string]string), quality: 1000}
		if typ == "*" {
			parsed.specificity = 0
		} else if subtype == "*" {
			parsed.specificity = 100
		} else {
			parsed.specificity = 200
		}
		valid, sawQ := true, false
		for _, rawParameter := range segments[1:] {
			parameter := strings.TrimSpace(rawParameter)
			name, rawValue, found := strings.Cut(parameter, "=")
			name = strings.ToLower(strings.TrimSpace(name))
			if !found || name == "" {
				valid = false
				break
			}
			if name == "q" {
				if sawQ || strings.HasPrefix(strings.TrimSpace(rawValue), "\"") {
					valid = false
					break
				}
				quality, qualityOK := parseQuality(strings.TrimSpace(rawValue))
				if !qualityOK {
					valid = false
					break
				}
				parsed.quality, sawQ = quality, true
				continue
			}
			if sawQ {
				continue
			}
			_, parameterMap, parseErr := mime.ParseMediaType("application/x;" + parameter)
			if parseErr != nil || len(parameterMap) != 1 {
				valid = false
				break
			}
			value, exists := parameterMap[name]
			if !exists {
				valid = false
				break
			}
			if _, duplicate := parsed.params[name]; duplicate {
				valid = false
				break
			}
			parsed.params[name] = strings.ToLower(value)
			parsed.specificity++
		}
		if valid {
			ranges = append(ranges, parsed)
		}
	}
	return ranges
}

func acceptHeader(header http.Header) string { return strings.Join(header.Values("Accept"), ",") }

func selectRepresentation(header string, candidates []candidate) representation {
	if header == "" {
		return representation{format: candidates[0].format, contentType: candidates[0].contentType}
	}
	ranges := parseAccept(header)
	if len(ranges) == 0 {
		return representation{format: formatNotAcceptable}
	}
	best := representation{format: formatNotAcceptable}
	bestQuality, bestSpecificity, bestPreference := -1, -1, -1
	for candidateIndex, candidate := range candidates {
		effectiveQuality, effectiveSpecificity := -1, -1
		for _, mediaRange := range ranges {
			if !rangeMatches(mediaRange, candidate) {
				continue
			}
			if mediaRange.specificity > effectiveSpecificity ||
				mediaRange.specificity == effectiveSpecificity && mediaRange.quality > effectiveQuality {
				effectiveQuality = mediaRange.quality
				effectiveSpecificity = mediaRange.specificity
			}
		}
		if effectiveQuality <= 0 {
			continue
		}
		preference := len(candidates) - candidateIndex
		if effectiveQuality > bestQuality ||
			effectiveQuality == bestQuality && effectiveSpecificity > bestSpecificity ||
			effectiveQuality == bestQuality && effectiveSpecificity == bestSpecificity && preference > bestPreference {
			best = representation{format: candidate.format, contentType: candidate.contentType}
			bestQuality, bestSpecificity, bestPreference = effectiveQuality, effectiveSpecificity, preference
		}
	}
	return best
}

func rangeMatches(mediaRange mediaRange, candidate candidate) bool {
	if mediaRange.typ != "*" && mediaRange.typ != candidate.typ ||
		mediaRange.subtype != "*" && mediaRange.subtype != candidate.subtype {
		return false
	}
	for name, value := range mediaRange.params {
		candidateValue, ok := candidate.params[name]
		if !ok || !strings.EqualFold(candidateValue, value) {
			return false
		}
	}
	return true
}

// SuccessNegotiation rejects an unacceptable success representation before
// authentication, persistence, or an external-service call. Bodyless DELETE
// is intentionally exempt.
func SuccessNegotiation(jsonOnly bool) echo.MiddlewareFunc {
	candidates := successCandidates
	if jsonOnly {
		candidates = jsonCandidates
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if c.Request().Method == http.MethodDelete {
				return next(c)
			}
			selected := selectRepresentation(acceptHeader(c.Request().Header), candidates)
			if selected.format == formatNotAcceptable {
				return NotAcceptable()
			}
			c.Set(selectedFormatKey, selected)
			return next(c)
		}
	}
}

func selectedSuccess(c *echo.Context) representation {
	if selected, ok := c.Get(selectedFormatKey).(representation); ok {
		return selected
	}
	return selectRepresentation(acceptHeader(c.Request().Header), successCandidates)
}

func marshalJSON(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Negotiate writes a GCP-profile JSON or CBOR success.
func Negotiate(c *echo.Context, status int, data any) error {
	httpheader.AddVary(c.Response().Header(), "Accept")
	selected := selectedSuccess(c)
	if selected.format == formatNotAcceptable {
		return NotAcceptable()
	}
	var (
		body []byte
		err  error
	)
	if selected.format == formatCBOR {
		body, err = cbor.Marshal(data)
	} else {
		body, err = marshalJSON(data)
	}
	if err != nil {
		return InternalError()
	}
	return c.Blob(status, selected.contentType, body)
}

// JSONDocument writes already-generated JSON while honoring the selected
// parameterless or UTF-8-charset JSON candidate.
func JSONDocument(c *echo.Context, status int, document []byte) error {
	httpheader.AddVary(c.Response().Header(), "Accept")
	selected := selectedSuccess(c)
	if selected.format != formatJSON {
		return NotAcceptable()
	}
	return c.Blob(status, selected.contentType, document)
}

func writeProblem(w http.ResponseWriter, r *http.Request, problem ProblemDetails) {
	httpheader.AddVary(w.Header(), "Accept")
	selected := selectRepresentation(acceptHeader(r.Header), problemCandidates)
	if selected.format == formatNotAcceptable {
		selected = representation{format: formatJSON, contentType: MediaTypeProblemJSON}
	}
	var (
		body []byte
		err  error
	)
	if selected.format == formatCBOR {
		body, err = cbor.Marshal(problem)
	} else {
		body, err = marshalJSON(problem)
	}
	if err != nil {
		fallback := *InternalError()
		body, _ = marshalJSON(fallback)
		selected = representation{format: formatJSON, contentType: MediaTypeProblemJSON}
		problem.Status = fallback.Status
	}
	w.Header().Set("Content-Type", selected.contentType)
	w.WriteHeader(problem.Status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func Recoverer(loggers ...*zap.Logger) echo.MiddlewareFunc {
	fallback := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		fallback = loggers[0]
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			defer func() {
				if recovered := recover(); recovered != nil {
					if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
						panic(recovered)
					}
					recoveryLogger(c.Request().Context(), fallback).Error(
						"panic recovered",
						zap.String("reason", "panic"),
						zap.ByteString("stack", debug.Stack()),
					)
					response, unwrapErr := echo.UnwrapResponse(c.Response())
					if unwrapErr == nil && response.Committed {
						panic(http.ErrAbortHandler)
					}
					writeProblem(c.Response(), c.Request(), *InternalError())
				}
			}()
			return next(c)
		}
	}
}

func recoveryLogger(ctx context.Context, fallback *zap.Logger) *zap.Logger {
	if obs.RequestID(ctx) != "" {
		return obs.Logger(ctx)
	}
	if fallback == nil {
		return zap.NewNop()
	}
	return fallback
}

func NewHTTPErrorHandler() echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		response, unwrapErr := echo.UnwrapResponse(c.Response())
		if unwrapErr == nil && response.Committed {
			return
		}
		if errors.Is(err, context.Canceled) {
			c.Response().WriteHeader(middleware.StatusCodeContextCanceled)
			return
		}
		writeProblem(c.Response(), c.Request(), problemFromError(err))
	}
}

func problemFromError(err error) ProblemDetails {
	if problem, ok := errors.AsType[*ProblemDetails](err); ok {
		definition, valid := problemDefinitions[problem.Code]
		if !valid || definition.Status != problem.Status || definition.Title != problem.Title ||
			definition.Detail != problem.Detail {
			return *InternalError()
		}
		return *problem
	}
	if validationError, ok := errors.AsType[*validate.ValidationError](err); ok {
		return *ValidationFailed(validationErrorDetails(validationError)...)
	}
	if errors.Is(err, echo.ErrNotFound) {
		return *NotFound()
	}
	if errors.Is(err, echo.ErrMethodNotAllowed) {
		return *MethodNotAllowed()
	}
	if maxBytesError := (*http.MaxBytesError)(nil); errors.As(err, &maxBytesError) {
		return *PayloadTooLarge()
	}
	if httpError, ok := errors.AsType[*echo.HTTPError](err); ok {
		switch httpError.Code {
		case http.StatusBadRequest:
			return *InvalidRequest()
		case http.StatusRequestEntityTooLarge:
			return *PayloadTooLarge()
		case http.StatusUnsupportedMediaType:
			return *UnsupportedMediaType()
		case http.StatusUnprocessableEntity:
			return *ValidationFailed()
		case http.StatusNotAcceptable:
			return *NotAcceptable()
		case http.StatusNotFound:
			return *NotFound()
		case http.StatusMethodNotAllowed:
			return *MethodNotAllowed()
		}
	}
	return *InternalError()
}

func validationErrorDetails(validationError *validate.ValidationError) []ErrorDetail {
	details := make([]ErrorDetail, 0, len(validationError.Fields))
	for _, field := range validationError.Fields {
		issue := ErrorDetail{Detail: field.Message}
		switch field.Location {
		case validate.LocationBody:
			issue.Source = &ErrorSource{Pointer: "/" + escapeJSONPointer(field.Field)}
		case validate.LocationQuery:
			issue.Source = &ErrorSource{Parameter: field.Field}
		case validate.LocationHeader:
			issue.Source = &ErrorSource{Header: field.Field}
		}
		details = append(details, issue)
	}
	return details
}

func escapeJSONPointer(value string) string {
	value = strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(value, "/", "~1")
}
