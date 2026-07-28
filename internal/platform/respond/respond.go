package respond

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/janisto/echo-observability/v2"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"

	"github.com/janisto/echo-playground/internal/platform/httpheader"
	"github.com/janisto/echo-playground/internal/platform/validate"
)

const (
	mediaTypeApplicationCBOR = "application/cbor"
	mediaTypeProblemCBOR     = "application/problem+cbor"
	mediaTypeProblemJSON     = "application/problem+json"
)

type responseFormat uint8

const (
	formatNotAcceptable responseFormat = iota
	formatJSON
	formatCBOR
)

// mediaRange represents a parsed Accept header media range with quality value.
type mediaRange struct {
	typ            string
	subtype        string
	q              float64
	hasMediaParams bool
}

// parseAccept parses an Accept header value into media ranges per RFC 9110.
func parseAccept(header string) []mediaRange {
	if header == "" {
		return nil
	}

	var ranges []mediaRange
	for _, part := range splitQuoted(header, ',') {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		mr := mediaRange{q: 1.0}
		segments := splitQuoted(part, ';')
		mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(segments[0]))
		typ, subtype, hasSlash := strings.Cut(strings.ToLower(mediaType), "/")
		if err != nil || !hasSlash || typ == "" || subtype == "" ||
			(typ == "*" && subtype != "*") {
			continue
		}
		mr.typ, mr.subtype = typ, subtype

		hasQ := false
		valid := true
		for _, segment := range segments[1:] {
			segment = strings.TrimSpace(segment)
			name, rawValue, _ := strings.Cut(segment, "=")
			if strings.EqualFold(strings.TrimSpace(name), "q") &&
				strings.HasPrefix(strings.TrimSpace(rawValue), `"`) {
				valid = false
				break
			}
			_, params, err := mime.ParseMediaType("application/x-accept;" + segment)
			if err != nil || len(params) != 1 {
				valid = false
				break
			}
			for name, value := range params {
				if strings.EqualFold(name, "q") {
					if hasQ {
						valid = false
						break
					}
					q, ok := parseQuality(value)
					if !ok {
						valid = false
						break
					}
					mr.q = q
					hasQ = true
				} else if !hasQ {
					mr.hasMediaParams = true
				}
			}
		}
		if valid {
			ranges = append(ranges, mr)
		}
	}
	return ranges
}

func splitQuoted(value string, separator byte) []string {
	parts := make([]string, 0, 1)
	start := 0
	quoted := false
	escaped := false
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

func parseQuality(value string) (float64, bool) {
	whole, fraction, hasFraction := strings.Cut(value, ".")
	if whole != "0" && whole != "1" {
		return 0, false
	}
	if hasFraction {
		if len(fraction) > 3 {
			return 0, false
		}
		for _, digit := range fraction {
			if digit < '0' || digit > '9' || whole == "1" && digit != '0' {
				return 0, false
			}
		}
	}
	q, err := strconv.ParseFloat(value, 64)
	return q, err == nil
}

func acceptHeader(header http.Header) string {
	return strings.Join(header.Values("Accept"), ",")
}

func selectSuccessFormat(header string) responseFormat {
	return selectFormat(header, []string{"json"}, []string{"cbor"})
}

func selectProblemFormat(header string) responseFormat {
	return selectFormat(header, []string{"problem+json", "json"}, []string{"problem+cbor", "cbor"})
}

// selectFormat chooses between supported JSON and CBOR subtypes. The first
// subtype in each list is the canonical representation; later entries are
// explicit aliases with lower specificity.
func selectFormat(header string, jsonSubtypes, cborSubtypes []string) responseFormat {
	ranges := parseAccept(header)
	if len(ranges) == 0 {
		if strings.TrimSpace(header) != "" {
			return formatNotAcceptable
		}
		return formatJSON
	}

	var cborQ, jsonQ float64 = -1, -1
	cborSpecificity, jsonSpecificity := 0, 0

	for _, mr := range ranges {
		cborMatch := matchSpecificity(mr, cborSubtypes)
		jsonMatch := matchSpecificity(mr, jsonSubtypes)
		if cborMatch > cborSpecificity || (cborMatch != 0 && cborMatch == cborSpecificity && mr.q > cborQ) {
			cborQ = mr.q
			cborSpecificity = cborMatch
		}
		if jsonMatch > jsonSpecificity || (jsonMatch != 0 && jsonMatch == jsonSpecificity && mr.q > jsonQ) {
			jsonQ = mr.q
			jsonSpecificity = jsonMatch
		}
	}

	if cborQ <= 0 && jsonQ <= 0 {
		return formatNotAcceptable
	}

	if cborQ > jsonQ {
		return formatCBOR
	}
	if jsonQ > cborQ {
		return formatJSON
	}
	if cborSpecificity > jsonSpecificity {
		return formatCBOR
	}
	return formatJSON
}

func matchSpecificity(mr mediaRange, subtypes []string) int {
	if mr.hasMediaParams {
		return 0
	}
	if mr.typ == "*" && mr.subtype == "*" {
		return 1
	}
	if mr.typ != "application" {
		return 0
	}
	if mr.subtype == "*" {
		return 2
	}
	for i, subtype := range subtypes {
		if mr.subtype == subtype {
			return 4 - i
		}
	}
	return 0
}

// writeProblem writes a Problem Details response honoring content negotiation.
// Uses application/problem+json (RFC 9457) by default.
// Uses application/problem+cbor when CBOR is preferred via Accept header.
func writeProblem(w http.ResponseWriter, r *http.Request, problem ProblemDetails) {
	if problem.Instance == "" {
		problem.Instance = r.URL.Path
	}

	httpheader.AddVary(w.Header(), "Origin", "Accept")

	format := selectProblemFormat(acceptHeader(r.Header))
	if format == formatNotAcceptable {
		problem = newProblem(http.StatusNotAcceptable, problemDetailRepresentationUnavailable)
		format = formatJSON
	}

	if format == formatCBOR {
		w.Header().Set("Content-Type", mediaTypeProblemCBOR)
		w.WriteHeader(problem.Status)
		if r.Method == http.MethodHead {
			return
		}
		if err := cbor.NewEncoder(w).Encode(problem); err != nil {
			obs.Logger(r.Context()).Error("failed to encode problem+cbor", zap.Error(err))
		}
	} else {
		w.Header().Set("Content-Type", mediaTypeProblemJSON)
		w.WriteHeader(problem.Status)
		if r.Method == http.MethodHead {
			return
		}
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(problem); err != nil {
			obs.Logger(r.Context()).Error("failed to encode problem+json", zap.Error(err))
		}
	}
}

// Negotiate writes a response using content negotiation (JSON or CBOR).
func Negotiate(c *echo.Context, status int, data any) error {
	httpheader.AddVary(c.Response().Header(), "Accept")
	switch selectSuccessFormat(acceptHeader(c.Request().Header)) {
	case formatCBOR:
		b, err := cbor.Marshal(data)
		if err != nil {
			return err
		}
		return c.Blob(status, mediaTypeApplicationCBOR, b)
	case formatNotAcceptable:
		return echo.NewHTTPError(http.StatusNotAcceptable, problemDetailRepresentationUnavailable)
	default:
		return c.JSON(status, data)
	}
}

// Recoverer returns Echo middleware that recovers from panics with Problem Details.
// Re-panics on http.ErrAbortHandler to preserve net/http abort semantics.
func Recoverer(loggers ...*zap.Logger) echo.MiddlewareFunc {
	fallback := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		fallback = loggers[0]
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			defer func() {
				if rec := recover(); rec != nil {
					if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
						panic(rec)
					}

					stack := debug.Stack()
					recoveryLogger(c.Request().Context(), fallback).Error("panic recovered",
						zap.Any("error", rec),
						zap.ByteString("stack", stack),
					)

					resp, unwrapErr := echo.UnwrapResponse(c.Response())
					if unwrapErr == nil && resp.Committed {
						panic(http.ErrAbortHandler)
					}

					writeProblem(
						c.Response(),
						c.Request(),
						newProblem(http.StatusInternalServerError, problemDetailInternalError),
					)
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

// NewHTTPErrorHandler returns an Echo HTTPErrorHandler that produces RFC 9457 Problem Details.
func NewHTTPErrorHandler() echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		resp, unwrapErr := echo.UnwrapResponse(c.Response())
		if unwrapErr == nil && resp.Committed {
			return
		}
		if errors.Is(err, context.Canceled) {
			c.Response().WriteHeader(middleware.StatusCodeContextCanceled)
			return
		}

		writeProblem(c.Response(), c.Request(), problemFromError(c, err))
	}
}

func problemFromError(c *echo.Context, err error) ProblemDetails {
	if pd, ok := errors.AsType[*ProblemDetails](err); ok {
		return *pd
	}

	if ve, ok := errors.AsType[*validate.ValidationError](err); ok {
		problem := Error422(ve.Message, validationErrorDetails(ve)...)
		return *problem
	}

	switch {
	case errors.Is(err, echo.ErrNotFound):
		return newProblem(http.StatusNotFound, problemDetailResourceMissing)

	case errors.Is(err, echo.ErrMethodNotAllowed):
		return newProblem(http.StatusMethodNotAllowed, fmt.Sprintf("method %s not allowed", c.Request().Method))
	}

	if he, ok := errors.AsType[*echo.HTTPError](err); ok {
		return newProblem(he.Code, he.Message)
	}
	if status := echo.StatusCode(err); status != 0 {
		return newProblem(status, http.StatusText(status))
	}

	return newProblem(http.StatusInternalServerError, problemDetailInternalError)
}

func validationErrorDetails(ve *validate.ValidationError) []ErrorDetail {
	if len(ve.Fields) == 0 {
		return nil
	}

	details := make([]ErrorDetail, len(ve.Fields))
	for i, f := range ve.Fields {
		details[i] = ErrorDetail{
			Message:  f.Message,
			Location: f.Field,
		}
	}
	return details
}
