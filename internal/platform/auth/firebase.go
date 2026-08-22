package auth

import (
	"context"
	"errors"

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
	if v == nil || v.client == nil {
		return nil, ErrAuthUnavailable
	}
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
		case fbauth.IsUserNotFound(err):
			return nil, errors.Join(ErrInvalidToken, err)
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

// ExtractBearerTokenValues enforces exactly one RFC 6750 bearer field.
func ExtractBearerTokenValues(values []string) (string, error) {
	if len(values) == 0 {
		return "", ErrNoToken
	}
	if len(values) != 1 {
		return "", ErrInvalidToken
	}
	return ExtractBearerToken(values[0])
}

// ExtractBearerToken parses the protocol-normalized Authorization field value.
func ExtractBearerToken(header string) (string, error) {
	if header == "" {
		return "", ErrNoToken
	}
	separator := 0
	for separator < len(header) && header[separator] != ' ' {
		if header[separator] == '\t' {
			return "", ErrInvalidToken
		}
		separator++
	}
	if separator != len("Bearer") || !equalFoldASCII(header[:separator], "Bearer") {
		return "", ErrInvalidToken
	}
	credentialStart := separator
	for credentialStart < len(header) && header[credentialStart] == ' ' {
		credentialStart++
	}
	if credentialStart == separator || credentialStart == len(header) {
		return "", ErrInvalidToken
	}
	credential := header[credentialStart:]
	if !validToken68(credential) {
		return "", ErrInvalidToken
	}
	return credential, nil
}

func equalFoldASCII(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range len(left) {
		l, r := left[i], right[i]
		if l >= 'A' && l <= 'Z' {
			l += 'a' - 'A'
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		if l != r {
			return false
		}
	}
	return true
}

func validToken68(value string) bool {
	baseLength := 0
	for baseLength < len(value) && value[baseLength] != '=' {
		character := value[baseLength]
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '.' &&
			character != '_' && character != '~' && character != '+' && character != '/' {
			return false
		}
		baseLength++
	}
	if baseLength == 0 {
		return false
	}
	for index := baseLength; index < len(value); index++ {
		if value[index] != '=' {
			return false
		}
	}
	return true
}

var _ Verifier = (*FirebaseVerifier)(nil)
