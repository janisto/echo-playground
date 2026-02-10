package respond

import (
	"fmt"
	"net/http"
)

// ProblemDetails represents an RFC 9457 Problem Details response.
type ProblemDetails struct {
	Type     string        `json:"type"               cbor:"type"               example:"about:blank"`
	Title    string        `json:"title"              cbor:"title"              example:"Not Found"`
	Status   int           `json:"status"             cbor:"status"             example:"404"`
	Detail   string        `json:"detail,omitempty"   cbor:"detail,omitempty"   example:"resource not found"`
	Instance string        `json:"instance,omitempty" cbor:"instance,omitempty" example:"/v1/items/42"`
	Errors   []ErrorDetail `json:"errors,omitempty"   cbor:"errors,omitempty"`
}

// ErrorDetail represents a single field-level error within a Problem Details response.
type ErrorDetail struct {
	Message  string `json:"message"            cbor:"message"            example:"firstname is required"`
	Location string `json:"location,omitempty" cbor:"location,omitempty" example:"body.firstname"`
	Value    string `json:"value,omitempty"    cbor:"value,omitempty"    example:""`
}

// Error implements the error interface.
func (p *ProblemDetails) Error() string {
	if p.Detail != "" {
		return fmt.Sprintf("%d %s: %s", p.Status, p.Title, p.Detail)
	}
	return fmt.Sprintf("%d %s", p.Status, p.Title)
}

// StatusCode implements echo.HTTPStatusCoder for Echo's status code detection.
func (p *ProblemDetails) StatusCode() int {
	return p.Status
}

// NewError creates a ProblemDetails error with the given status code and detail message.
func NewError(status int, detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	}
}

// Error400 returns a 400 Bad Request ProblemDetails error.
func Error400(detail string) *ProblemDetails {
	return NewError(http.StatusBadRequest, detail)
}

// Error401 returns a 401 Unauthorized ProblemDetails error.
func Error401(detail string) *ProblemDetails {
	return NewError(http.StatusUnauthorized, detail)
}

// Error403 returns a 403 Forbidden ProblemDetails error.
func Error403(detail string) *ProblemDetails {
	return NewError(http.StatusForbidden, detail)
}

// Error404 returns a 404 Not Found ProblemDetails error.
func Error404(detail string) *ProblemDetails {
	return NewError(http.StatusNotFound, detail)
}

// Error409 returns a 409 Conflict ProblemDetails error.
func Error409(detail string) *ProblemDetails {
	return NewError(http.StatusConflict, detail)
}

// Error422 returns a 422 Unprocessable Entity ProblemDetails error with field-level errors.
func Error422(detail string, fields ...ErrorDetail) *ProblemDetails {
	p := NewError(http.StatusUnprocessableEntity, detail)
	p.Errors = fields
	return p
}

// Error500 returns a 500 Internal Server Error ProblemDetails error.
func Error500(detail string) *ProblemDetails {
	return NewError(http.StatusInternalServerError, detail)
}

// Error503 returns a 503 Service Unavailable ProblemDetails error.
func Error503(detail string) *ProblemDetails {
	return NewError(http.StatusServiceUnavailable, detail)
}
