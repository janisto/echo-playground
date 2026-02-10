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

	"github.com/janisto/echo-playground/internal/platform/validate"
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

// ensureVary adds values to the Vary header without duplicating existing entries.
func ensureVary(h http.Header, values ...string) {
	existing := make(map[string]struct{})
	for _, v := range h.Values("Vary") {
		for part := range strings.SplitSeq(v, ",") {
			existing[strings.TrimSpace(part)] = struct{}{}
		}
	}
	for _, v := range values {
		if _, ok := existing[v]; !ok {
			h.Add("Vary", v)
			existing[v] = struct{}{}
		}
	}
}

// writeProblem writes a Problem Details response honoring content negotiation.
// Uses application/problem+json (RFC 9457) by default.
// Uses application/problem+cbor when CBOR is preferred via Accept header.
func writeProblem(w http.ResponseWriter, r *http.Request, problem ProblemDetails) {
	ensureVary(w.Header(), "Origin", "Accept")

	if selectFormat(r.Header.Get("Accept")) {
		w.Header().Set("Content-Type", "application/problem+cbor")
		w.WriteHeader(problem.Status)
		_ = cbor.NewEncoder(w).Encode(problem)
	} else {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(problem.Status)
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(problem)
	}
}

// Negotiate writes a response using content negotiation (JSON or CBOR).
func Negotiate(c *echo.Context, status int, data any) error {
	if selectFormat(c.Request().Header.Get("Accept")) {
		b, err := cbor.Marshal(data)
		if err != nil {
			return err
		}
		return c.Blob(status, "application/cbor", b)
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

					problem := ProblemDetails{
						Type:   "about:blank",
						Title:  http.StatusText(http.StatusInternalServerError),
						Status: http.StatusInternalServerError,
						Detail: "internal server error",
					}
					writeProblem(c.Response(), c.Request(), problem)
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

		var problem ProblemDetails

		var pd *ProblemDetails
		var he *echo.HTTPError
		var ve *validate.ValidationError

		switch {
		case errors.As(err, &pd):
			problem = *pd

		case errors.As(err, &ve):
			problem = ProblemDetails{
				Type:   "about:blank",
				Title:  http.StatusText(http.StatusUnprocessableEntity),
				Status: http.StatusUnprocessableEntity,
				Detail: ve.Message,
			}
			if len(ve.Fields) > 0 {
				problem.Errors = make([]ErrorDetail, len(ve.Fields))
				for i, f := range ve.Fields {
					problem.Errors[i] = ErrorDetail{
						Message:  f.Message,
						Location: f.Field,
						Value:    f.Value,
					}
				}
			}

		case errors.Is(err, echo.ErrNotFound):
			problem = ProblemDetails{
				Type:   "about:blank",
				Title:  http.StatusText(http.StatusNotFound),
				Status: http.StatusNotFound,
				Detail: "resource not found",
			}

		case errors.Is(err, echo.ErrMethodNotAllowed):
			problem = ProblemDetails{
				Type:   "about:blank",
				Title:  http.StatusText(http.StatusMethodNotAllowed),
				Status: http.StatusMethodNotAllowed,
				Detail: fmt.Sprintf("method %s not allowed", c.Request().Method),
			}

		case errors.As(err, &he):
			problem = ProblemDetails{
				Type:   "about:blank",
				Title:  http.StatusText(he.Code),
				Status: he.Code,
				Detail: he.Message,
			}

		default:
			problem = ProblemDetails{
				Type:   "about:blank",
				Title:  http.StatusText(http.StatusInternalServerError),
				Status: http.StatusInternalServerError,
				Detail: "internal server error",
			}
		}

		writeProblem(c.Response(), c.Request(), problem)
	}
}
