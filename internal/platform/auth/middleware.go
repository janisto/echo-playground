package auth

import (
	"context"
	"errors"
	"log/slog"

	"github.com/labstack/echo/v5"

	applog "github.com/janisto/echo-playground/internal/platform/logging"
	"github.com/janisto/echo-playground/internal/platform/respond"
)

// userContextKey is the context key for the authenticated user.
type userContextKey struct{}

// Middleware returns Echo middleware for Firebase authentication.
// Applied at the group level to protect routes requiring authentication.
func Middleware(verifier Verifier) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			token, err := ExtractBearerToken(c.Request().Header.Get("Authorization"))
			if err != nil {
				applog.LogWarn(c.Request().Context(), "auth failed: missing or invalid header",
					slog.String("reason", "no_token"))
				c.Response().Header().Set("WWW-Authenticate", "Bearer")
				return respond.Error401("missing or invalid authorization header")
			}

			user, err := verifier.Verify(c.Request().Context(), token)
			if err != nil {
				reason := categorizeAuthError(err)
				applog.LogWarn(c.Request().Context(), "auth failed: token verification failed",
					slog.String("reason", reason))

				if errors.Is(err, ErrCertificateFetch) {
					c.Response().Header().Set("Retry-After", "30")
					return respond.Error503("authentication service temporarily unavailable")
				}
				c.Response().Header().Set("WWW-Authenticate", "Bearer")
				return respond.Error401("invalid or expired token")
			}

			c.Set("user", user)
			ctx := context.WithValue(c.Request().Context(), userContextKey{}, user)
			c.SetRequest(c.Request().WithContext(ctx))

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
	case errors.Is(err, ErrInvalidToken):
		return "invalid_token"
	default:
		return "unknown"
	}
}

// UserFromEchoContext retrieves the authenticated user from Echo context.
func UserFromEchoContext(c *echo.Context) (*FirebaseUser, error) {
	return echo.ContextGet[*FirebaseUser](c, "user")
}

// UserFromContext retrieves the authenticated user from standard context.
// Returns nil if no user is authenticated.
func UserFromContext(ctx context.Context) *FirebaseUser {
	user, _ := ctx.Value(userContextKey{}).(*FirebaseUser)
	return user
}
