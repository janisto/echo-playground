package auth

import (
	"context"
	"errors"

	"github.com/janisto/echo-observability"
	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"github.com/janisto/echo-playground/internal/platform/respond"
)

const echoUserKey = "user"

// Middleware returns Echo middleware for Firebase authentication.
// Applied at the group level to protect routes requiring authentication.
func Middleware(verifier Verifier) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			token, err := ExtractBearerToken(c.Request().Header.Get("Authorization"))
			if err != nil {
				c.Response().Header().Set("WWW-Authenticate", "Bearer")
				return respond.Error401("missing or invalid authorization header")
			}

			user, err := verifier.Verify(c.Request().Context(), token)
			if err != nil {
				reason := categorizeAuthError(err)
				if errors.Is(err, ErrAuthUnavailable) || errors.Is(err, ErrCertificateFetch) {
					obs.Logger(c.Request().Context()).Error(
						"authentication dependency failed",
						zap.String("reason", reason),
					)
					c.Response().Header().Set("Retry-After", "30")
					return respond.Error503("authentication service temporarily unavailable")
				}
				if errors.Is(err, context.Canceled) {
					return err
				}
				c.Response().Header().Set("WWW-Authenticate", "Bearer")
				return respond.Error401("invalid or expired token")
			}

			c.Set(echoUserKey, user)

			return next(c)
		}
	}
}

// categorizeAuthError returns a safe category string for logging.
func categorizeAuthError(err error) string {
	switch {
	case errors.Is(err, ErrTokenExpired):
		return "token_expired"
	case errors.Is(err, ErrTokenRevoked):
		return "token_revoked"
	case errors.Is(err, ErrUserDisabled):
		return "user_disabled"
	case errors.Is(err, ErrCertificateFetch):
		return "certificate_fetch_failed"
	case errors.Is(err, ErrAuthUnavailable):
		return "dependency_unavailable"
	case errors.Is(err, ErrInvalidToken):
		return "invalid_token"
	default:
		return "unknown"
	}
}

// UserFromEchoContext retrieves the authenticated user from Echo context.
func UserFromEchoContext(c *echo.Context) (*FirebaseUser, error) {
	return echo.ContextGet[*FirebaseUser](c, echoUserKey)
}
