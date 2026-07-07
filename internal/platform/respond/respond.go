package respond

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/httpheader"
	"github.com/janisto/echo-playground/internal/platform/validate"
)

const (
	mediaTypeApplicationCBOR = "application/cbor"
	mediaTypeProblemCBOR     = "application/problem+cbor"
	mediaTypeProblemJSON     = "application/problem+json"
)

// mediaRange represents a parsed Accept header media range with quality value.
type mediaRange struct {
	typ     string
	subtype string
	q       float64
}

// parseAccept parses an Accept header value into media ranges per RFC 9110.
func parseAccept(header string) []mediaRange {
	if header == "" {
		return nil
	}

	var ranges []mediaRange
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		mr := mediaRange{q: 1.0}
		mediaType := part
		if before, after, ok := strings.Cut(part, ";"); ok {
			mediaType = strings.TrimSpace(before)
			for param := range strings.SplitSeq(after, ";") {
				param = strings.TrimSpace(param)
				if strings.HasPrefix(strings.ToLower(param), "q=") {
					if qval, err := strconv.ParseFloat(param[2:], 64); err == nil && qval >= 0 && qval <= 1 {
						mr.q = qval
					}
				}
			}
		}

		if before, after, ok := strings.Cut(mediaType, "/"); ok {
			mr.typ = strings.ToLower(strings.TrimSpace(before))
			mr.subtype = strings.ToLower(strings.TrimSpace(after))
		} else {
			mr.typ = strings.ToLower(strings.TrimSpace(mediaType))
			mr.subtype = "*"
		}
		ranges = append(ranges, mr)
	}
	return ranges
}

// selectFormat determines the preferred response format based on Accept header.
// Returns true for CBOR, false for JSON (default).
// Per RFC 9110: q-value is the primary ranking factor, specificity is tie-breaker.
func selectFormat(header string) bool {
	ranges := parseAccept(header)
	if len(ranges) == 0 {
		return false
	}

	var cborQ, jsonQ float64 = -1, -1
	cborSpecificity, jsonSpecificity := 0, 0

	for _, mr := range ranges {
		if mr.q == 0 {
			continue
		}

		specificity := 0
		matchesCBOR, matchesJSON := false, false

		switch {
		case mr.typ == "application" && mr.subtype == "problem+cbor":
			matchesCBOR = true
			specificity = 4
		case mr.typ == "application" && mr.subtype == "problem+json":
			matchesJSON = true
			specificity = 4
		case mr.typ == "application" && mr.subtype == "cbor":
			matchesCBOR = true
			specificity = 3
		case mr.typ == "application" && mr.subtype == "json":
			matchesJSON = true
			specificity = 3
		case mr.typ == "application" && strings.HasSuffix(mr.subtype, "+cbor"):
			matchesCBOR = true
			specificity = 3
		case mr.typ == "application" && strings.HasSuffix(mr.subtype, "+json"):
			matchesJSON = true
			specificity = 3
		case mr.typ == "application" && mr.subtype == "*":
			matchesCBOR = true
			matchesJSON = true
			specificity = 2
		case mr.typ == "*" && mr.subtype == "*":
			matchesCBOR = true
			matchesJSON = true
			specificity = 1
		}

		if matchesCBOR && (specificity > cborSpecificity || (specificity == cborSpecificity && mr.q > cborQ)) {
			cborQ = mr.q
			cborSpecificity = specificity
		}
		if matchesJSON && (specificity > jsonSpecificity || (specificity == jsonSpecificity && mr.q > jsonQ)) {
			jsonQ = mr.q
			jsonSpecificity = specificity
		}
	}

	if cborQ <= 0 && jsonQ <= 0 {
		return false
	}

	if cborQ > jsonQ {
		return true
	}
	if jsonQ > cborQ {
		return false
	}
	if cborSpecificity > jsonSpecificity {
		return true
	}
	return false
}

// writeProblem writes a Problem Details response honoring content negotiation.
// Uses application/problem+json (RFC 9457) by default.
// Uses application/problem+cbor when CBOR is preferred via Accept header.
func writeProblem(w http.ResponseWriter, r *http.Request, problem ProblemDetails) {
	if problem.Instance == "" {
		problem.Instance = r.URL.Path
	}

	httpheader.AddVary(w.Header(), "Origin", "Accept")

	if selectFormat(r.Header.Get("Accept")) {
		w.Header().Set("Content-Type", mediaTypeProblemCBOR)
		w.WriteHeader(problem.Status)
		if err := cbor.NewEncoder(w).Encode(problem); err != nil {
			slog.ErrorContext(r.Context(), "failed to encode problem+cbor", slog.Any("error", err))
		}
	} else {
		w.Header().Set("Content-Type", mediaTypeProblemJSON)
		w.WriteHeader(problem.Status)
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(problem); err != nil {
			slog.ErrorContext(r.Context(), "failed to encode problem+json", slog.Any("error", err))
		}
	}
}

// Negotiate writes a response using content negotiation (JSON or CBOR).
func Negotiate(c *echo.Context, status int, data any) error {
	httpheader.AddVary(c.Response().Header(), "Accept")
	if selectFormat(c.Request().Header.Get("Accept")) {
		b, err := cbor.Marshal(data)
		if err != nil {
			return err
		}
		return c.Blob(status, mediaTypeApplicationCBOR, b)
	}
	return c.JSON(status, data)
}

// Recoverer returns Echo middleware that recovers from panics with Problem Details.
// Re-panics on http.ErrAbortHandler to preserve net/http abort semantics.
func Recoverer() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			defer func() {
				if rec := recover(); rec != nil {
					if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
						panic(rec)
					}

					stack := debug.Stack()
					slog.ErrorContext(c.Request().Context(), "panic recovered",
						slog.Any("error", rec),
						slog.String("stack", string(stack)),
					)

					resp, unwrapErr := echo.UnwrapResponse(c.Response())
					if unwrapErr == nil && resp.Committed {
						return
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

// NewHTTPErrorHandler returns an Echo HTTPErrorHandler that produces RFC 9457 Problem Details.
func NewHTTPErrorHandler() echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		resp, unwrapErr := echo.UnwrapResponse(c.Response())
		if unwrapErr == nil && resp.Committed {
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
			Value:    f.Value,
		}
	}
	return details
}
