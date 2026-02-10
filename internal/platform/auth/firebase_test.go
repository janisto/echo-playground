package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
)

func requireAuthEmulator(t *testing.T) string {
	t.Helper()
	host := os.Getenv("FIREBASE_AUTH_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIREBASE_AUTH_EMULATOR_HOST not set; skipping auth emulator test")
	}
	return host
}

func createEmulatorIDToken(t *testing.T, host string) string {
	t.Helper()
	email := fmt.Sprintf("test-verify-%d@example.com", time.Now().UnixNano())
	endpoint := fmt.Sprintf("http://%s/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-api-key", host)
	body := strings.NewReader(fmt.Sprintf(`{"email":%q,"password":"password123","returnSecureToken":true}`, email))
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		endpoint,
		body,
	) //nolint:gosec // test-only emulator URL
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create emulator user: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("emulator signUp returned %d", resp.StatusCode)
	}

	var result struct {
		IDToken string `json:"idToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode emulator response: %v", err)
	}
	if result.IDToken == "" {
		t.Fatal("emulator returned empty ID token")
	}
	return result.IDToken
}

func TestNewFirebaseVerifier(t *testing.T) {
	host := requireAuthEmulator(t)
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", host)

	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: "demo-test-project"})
	if err != nil {
		t.Fatalf("failed to create firebase app: %v", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to get auth client: %v", err)
	}

	verifier := NewFirebaseVerifier(client)
	if verifier == nil {
		t.Fatal("expected non-nil verifier")
	}
	if verifier.client != client {
		t.Fatal("expected verifier to hold the auth client")
	}
}

func TestFirebaseVerifier_Verify_ValidToken(t *testing.T) {
	host := requireAuthEmulator(t)
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", host)

	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: "demo-test-project"})
	if err != nil {
		t.Fatalf("failed to create firebase app: %v", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to get auth client: %v", err)
	}

	idToken := createEmulatorIDToken(t, host)
	verifier := NewFirebaseVerifier(client)

	user, err := verifier.Verify(ctx, idToken)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.UID == "" {
		t.Fatal("expected non-empty UID")
	}
	if user.Email == "" {
		t.Fatal("expected non-empty email")
	}
}

func TestFirebaseVerifier_Verify_InvalidToken(t *testing.T) {
	host := requireAuthEmulator(t)
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", host)

	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: "demo-test-project"})
	if err != nil {
		t.Fatalf("failed to create firebase app: %v", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to get auth client: %v", err)
	}

	verifier := NewFirebaseVerifier(client)

	_, err = verifier.Verify(ctx, "not-a-valid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

type emulatorSignUpResult struct {
	IDToken      string `json:"idToken"`
	LocalID      string `json:"localId"`
	RefreshToken string `json:"refreshToken"`
}

func signUpEmulatorUser(t *testing.T, host string) emulatorSignUpResult {
	t.Helper()
	email := fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())
	endpoint := fmt.Sprintf("http://%s/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-api-key", host)
	body := strings.NewReader(fmt.Sprintf(`{"email":%q,"password":"password123","returnSecureToken":true}`, email))
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		endpoint,
		body,
	) //nolint:gosec // test-only emulator URL
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create emulator user: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("emulator signUp returned %d", resp.StatusCode)
	}

	var result emulatorSignUpResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode emulator response: %v", err)
	}
	return result
}

func newEmulatorAuthClient(t *testing.T, host string) *fbauth.Client {
	t.Helper()
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", host)

	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: "demo-test-project"})
	if err != nil {
		t.Fatalf("failed to create firebase app: %v", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to get auth client: %v", err)
	}
	return client
}

func TestFirebaseVerifier_Verify_DisabledUser(t *testing.T) {
	host := requireAuthEmulator(t)
	client := newEmulatorAuthClient(t, host)
	ctx := context.Background()

	result := signUpEmulatorUser(t, host)

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
