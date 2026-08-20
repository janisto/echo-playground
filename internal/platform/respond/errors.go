package respond

import (
	"fmt"
	"net/http"
)

const problemTypeAboutBlank = "about:blank"

// Stable portable error codes.
const (
	CodeInvalidRequest        = "invalid_request"
	CodeUnauthorized          = "unauthorized"
	CodeForbidden             = "forbidden"
	CodeNotFound              = "not_found"
	CodeProfileNotFound       = "profile_not_found"
	CodeGitHubNotFound        = "github_not_found"
	CodeMethodNotAllowed      = "method_not_allowed"
	CodeNotAcceptable         = "not_acceptable"
	CodeProfileExists         = "profile_exists"
	CodePayloadTooLarge       = "payload_too_large"
	CodeUnsupportedMediaType  = "unsupported_media_type"
	CodeValidationFailed      = "validation_failed"
	CodeGitHubRateLimit       = "github_rate_limit"
	CodeInternalError         = "internal_error"
	CodeGitHubUpstream        = "github_upstream"
	CodeDependencyUnavailable = "dependency_unavailable"
	CodeGitHubTimeout         = "github_timeout"
)

type problemDefinition struct {
	Status int
	Title  string
	Detail string
}

var problemDefinitions = map[string]problemDefinition{
	CodeInvalidRequest:   {http.StatusBadRequest, "Bad Request", "Request is malformed"},
	CodeUnauthorized:     {http.StatusUnauthorized, "Unauthorized", "Authentication is required or invalid"},
	CodeForbidden:        {http.StatusForbidden, "Forbidden", "Access is forbidden"},
	CodeNotFound:         {http.StatusNotFound, "Not Found", "Resource not found"},
	CodeProfileNotFound:  {http.StatusNotFound, "Not Found", "Profile not found"},
	CodeGitHubNotFound:   {http.StatusNotFound, "Not Found", "GitHub resource not found"},
	CodeMethodNotAllowed: {http.StatusMethodNotAllowed, "Method Not Allowed", "Method not allowed"},
	CodeNotAcceptable: {
		http.StatusNotAcceptable,
		"Not Acceptable",
		"No acceptable response representation is available",
	},
	CodeProfileExists:   {http.StatusConflict, "Conflict", "Profile already exists"},
	CodePayloadTooLarge: {http.StatusRequestEntityTooLarge, "Content Too Large", "Request body is too large"},
	CodeUnsupportedMediaType: {
		http.StatusUnsupportedMediaType,
		"Unsupported Media Type",
		"Request representation is not supported",
	},
	CodeValidationFailed: {http.StatusUnprocessableEntity, "Unprocessable Content", "Request validation failed"},
	CodeGitHubRateLimit:  {http.StatusTooManyRequests, "Too Many Requests", "GitHub rate limit exceeded"},
	CodeInternalError:    {http.StatusInternalServerError, "Internal Server Error", "Internal server error"},
	CodeGitHubUpstream: {
		http.StatusBadGateway,
		"Bad Gateway",
		"GitHub upstream response is invalid or unavailable",
	},
	CodeDependencyUnavailable: {
		http.StatusServiceUnavailable,
		"Service Unavailable",
		"A required dependency is unavailable",
	},
	CodeGitHubTimeout: {http.StatusGatewayTimeout, "Gateway Timeout", "GitHub request timed out"},
}

// ProblemDetails is the closed RFC 9457 error document used by the GCP profile.
type ProblemDetails struct {
	Type   string        `json:"type,omitempty"   cbor:"type,omitempty"   example:"about:blank"`
	Title  string        `json:"title"            cbor:"title"            example:"Not Found"`
	Status int           `json:"status"           cbor:"status"           example:"404"`
	Detail string        `json:"detail"           cbor:"detail"           example:"Resource not found"`
	Code   string        `json:"code"             cbor:"code"             example:"not_found"`
	Errors []ErrorDetail `json:"errors,omitempty" cbor:"errors,omitempty"`
}

// ErrorDetail is a safe normalized validation issue.
type ErrorDetail struct {
	Detail string       `json:"detail"           cbor:"detail"`
	Source *ErrorSource `json:"source,omitempty" cbor:"source,omitempty"`
}

// ErrorSource identifies only application-owned input structure.
type ErrorSource struct {
	Pointer   string `json:"pointer,omitempty"   cbor:"pointer,omitempty"`
	Parameter string `json:"parameter,omitempty" cbor:"parameter,omitempty"`
	Header    string `json:"header,omitempty"    cbor:"header,omitempty"`
}

func (p *ProblemDetails) Error() string {
	return fmt.Sprintf("%d %s: %s", p.Status, p.Title, p.Detail)
}

func (p *ProblemDetails) StatusCode() int { return p.Status }

// Problem returns the exact portable problem definition for code. Unknown
// local codes fail closed as internal_error.
func Problem(code string, issues ...ErrorDetail) *ProblemDetails {
	definition, ok := problemDefinitions[code]
	if !ok {
		code = CodeInternalError
		definition = problemDefinitions[code]
	}
	if len(issues) > 32 {
		issues = append(issues[:31], ErrorDetail{Detail: "Additional validation errors omitted"})
	}
	return &ProblemDetails{
		Type:   problemTypeAboutBlank,
		Title:  definition.Title,
		Status: definition.Status,
		Detail: definition.Detail,
		Code:   code,
		Errors: issues,
	}
}

func InvalidRequest() *ProblemDetails       { return Problem(CodeInvalidRequest) }
func Unauthorized() *ProblemDetails         { return Problem(CodeUnauthorized) }
func Forbidden() *ProblemDetails            { return Problem(CodeForbidden) }
func NotFound() *ProblemDetails             { return Problem(CodeNotFound) }
func ProfileNotFound() *ProblemDetails      { return Problem(CodeProfileNotFound) }
func GitHubNotFound() *ProblemDetails       { return Problem(CodeGitHubNotFound) }
func MethodNotAllowed() *ProblemDetails     { return Problem(CodeMethodNotAllowed) }
func NotAcceptable() *ProblemDetails        { return Problem(CodeNotAcceptable) }
func ProfileExists() *ProblemDetails        { return Problem(CodeProfileExists) }
func PayloadTooLarge() *ProblemDetails      { return Problem(CodePayloadTooLarge) }
func UnsupportedMediaType() *ProblemDetails { return Problem(CodeUnsupportedMediaType) }
func ValidationFailed(issues ...ErrorDetail) *ProblemDetails {
	return Problem(CodeValidationFailed, issues...)
}
func GitHubRateLimit() *ProblemDetails       { return Problem(CodeGitHubRateLimit) }
func InternalError() *ProblemDetails         { return Problem(CodeInternalError) }
func GitHubUpstream() *ProblemDetails        { return Problem(CodeGitHubUpstream) }
func DependencyUnavailable() *ProblemDetails { return Problem(CodeDependencyUnavailable) }
func GitHubTimeout() *ProblemDetails         { return Problem(CodeGitHubTimeout) }
