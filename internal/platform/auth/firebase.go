package auth

import (
	"context"
	"errors"
	"strings"

	fbauth "firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/errorutils"
)

// FirebaseUser represents an authenticated user.
type FirebaseUser struct {
	UID string
}

// Error types for authentication failures.
var (
	ErrNoToken          = errors.New("missing authorization header")
	ErrInvalidToken     = errors.New("invalid token")
	ErrTokenExpired     = errors.New("token expired")
	ErrTokenRevoked     = errors.New("token revoked")
	ErrUserDisabled     = errors.New("user disabled")
	ErrCertificateFetch = errors.New("failed to fetch certificates")
	ErrAuthUnavailable  = errors.New("authentication service unavailable")
)

// Verifier validates tokens and returns user information.
type Verifier interface {
	Verify(ctx context.Context, token string) (*FirebaseUser, error)
}

// FirebaseVerifier implements Verifier using Firebase Admin SDK.
type FirebaseVerifier struct {
	client *fbauth.Client
}

// NewFirebaseVerifier creates a new verifier with the given auth client.
func NewFirebaseVerifier(client *fbauth.Client) *FirebaseVerifier {
	return &FirebaseVerifier{client: client}
}

// Verify validates a Firebase ID token and checks for revocation.
func (v *FirebaseVerifier) Verify(ctx context.Context, idToken string) (*FirebaseUser, error) {
	token, err := v.client.VerifyIDTokenAndCheckRevoked(ctx, idToken)
	if err != nil {
		switch {
		case fbauth.IsCertificateFetchFailed(err):
			return nil, errors.Join(ErrCertificateFetch, ErrAuthUnavailable, err)
		case fbauth.IsIDTokenExpired(err):
			return nil, errors.Join(ErrTokenExpired, err)
		case fbauth.IsIDTokenRevoked(err):
			return nil, errors.Join(ErrTokenRevoked, err)
		case fbauth.IsUserDisabled(err):
			return nil, errors.Join(ErrUserDisabled, err)
		case fbauth.IsIDTokenInvalid(err):
			return nil, errors.Join(ErrInvalidToken, err)
		case errors.Is(err, context.Canceled), errorutils.IsCancelled(err):
			return nil, errors.Join(context.Canceled, err)
		case errors.Is(err, context.DeadlineExceeded), errorutils.IsDeadlineExceeded(err):
			return nil, errors.Join(ErrAuthUnavailable, context.DeadlineExceeded, err)
		case errorutils.IsUnavailable(err), errorutils.IsInternal(err), errorutils.IsUnknown(err):
			return nil, errors.Join(ErrAuthUnavailable, err)
		default:
			return nil, errors.Join(ErrAuthUnavailable, err)
		}
	}

	return &FirebaseUser{
		UID: token.UID,
	}, nil
}

// ExtractBearerToken extracts the token from Authorization header.
func ExtractBearerToken(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
		if header == "" {
			return "", ErrNoToken
		}
		return "", ErrInvalidToken
	}
	return parts[1], nil
}

var _ Verifier = (*FirebaseVerifier)(nil)
