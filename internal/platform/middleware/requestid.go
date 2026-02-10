package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	// HeaderXRequestID is the canonical request ID header name.
	HeaderXRequestID = "X-Request-ID"

	// maxRequestIDLength limits request ID size to prevent unbounded memory usage.
	maxRequestIDLength = 128
)

// isValidRequestID validates a request ID for safe logging.
// Only allows printable ASCII characters (0x20-0x7E) excluding control characters,
// newlines, and other problematic characters that could enable log injection.
func isValidRequestID(id string) bool {
	if len(id) == 0 || len(id) > maxRequestIDLength {
		return false
	}
	for i := range len(id) {
		c := id[i]
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return true
}

// RequestID returns Echo middleware that injects a UUIDv4 request identifier.
// If the incoming request provides a valid X-Request-ID header, that value is reused.
// Invalid request IDs (too long, empty, or containing non-printable characters)
// are rejected and a new UUID is generated instead.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			reqID := c.Request().Header.Get(HeaderXRequestID)
			if !isValidRequestID(reqID) {
				reqID = uuid.NewString()
			}

			c.Set("request_id", reqID)
			c.Response().Header().Set(HeaderXRequestID, reqID)

			return next(c)
		}
	}
}
