package auth

import (
	"errors"
	"testing"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"

	"github.com/janisto/echo-playground/internal/testutil"
)

func newEmulatorAuthClient(t *testing.T) *fbauth.Client {
	t.Helper()
	ctx := t.Context()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: testutil.ProjectID})
	if err != nil {
		t.Fatalf("failed to create firebase app: %v", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to get auth client: %v", err)
	}
	return client
}

func TestNewFirebaseVerifier(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)

	client := newEmulatorAuthClient(t)
	verifier := NewFirebaseVerifier(client)
	if verifier == nil {
		t.Fatal("expected non-nil verifier")
	}
	if verifier.client != client {
		t.Fatal("expected verifier to hold the auth client")
	}
}

func TestFirebaseVerifier_Verify_ValidToken(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearAccounts(t)

	client := newEmulatorAuthClient(t)
	result := testutil.CreateTestUser(t, "verify@example.com", "password123")

	verifier := NewFirebaseVerifier(client)
	user, err := verifier.Verify(t.Context(), result.IDToken)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.UID == "" {
		t.Fatal("expected non-empty UID")
	}
}

func TestFirebaseVerifier_Verify_InvalidToken(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)

	client := newEmulatorAuthClient(t)
	verifier := NewFirebaseVerifier(client)

	_, err := verifier.Verify(t.Context(), "not-a-valid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestFirebaseVerifier_Verify_RevokedToken(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearEmulators(t)

	client := newEmulatorAuthClient(t)
	ctx := t.Context()

	result := testutil.CreateTestUser(t, "revoke@example.com", "password123")

	if err := client.RevokeRefreshTokens(ctx, result.LocalID); err != nil {
		t.Fatalf("failed to revoke tokens: %v", err)
	}

	verifier := NewFirebaseVerifier(client)
	_, verifyErr := verifier.Verify(ctx, result.IDToken)
	if verifyErr == nil {
		t.Skip("emulator does not enforce token revocation checks")
	}
	if !errors.Is(verifyErr, ErrTokenRevoked) && !errors.Is(verifyErr, ErrInvalidToken) {
		t.Fatalf("expected ErrTokenRevoked or ErrInvalidToken, got %v", verifyErr)
	}
}

func TestFirebaseVerifier_Verify_DisabledUser(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearAccounts(t)

	client := newEmulatorAuthClient(t)
	ctx := t.Context()

	result := testutil.CreateTestUser(t, "disabled@example.com", "password123")

	params := (&fbauth.UserToUpdate{}).Disabled(true)
	if _, err := client.UpdateUser(ctx, result.LocalID, params); err != nil {
		t.Fatalf("failed to disable user: %v", err)
	}

	verifier := NewFirebaseVerifier(client)
	_, err := verifier.Verify(ctx, result.IDToken)
	if err == nil {
		t.Fatal("expected error for disabled user")
	}
	if !errors.Is(err, ErrUserDisabled) && !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrUserDisabled or ErrInvalidToken, got %v", err)
	}
}
