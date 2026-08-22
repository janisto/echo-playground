package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"maps"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"

	"github.com/janisto/echo-playground/internal/testutil"
)

const firebaseContractProject = "contract-test-project"

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type firebaseSDKHarness struct {
	t              *testing.T
	signingKey     *rsa.PrivateKey
	certificatePEM string
	certificateOK  bool
	server         *httptest.Server
	mu             sync.Mutex
	userResponse   string
	userCalls      int
}

func newFirebaseSDKHarness(t *testing.T, certificateOK bool) (*FirebaseVerifier, *firebaseSDKHarness) {
	t.Helper()
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "")
	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate Firebase signing key: %v", err)
	}
	certificateTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		certificateTemplate,
		certificateTemplate,
		&signingKey.PublicKey,
		signingKey,
	)
	if err != nil {
		t.Fatalf("create Firebase signing certificate: %v", err)
	}
	harness := &firebaseSDKHarness{
		t:              t,
		signingKey:     signingKey,
		certificatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})),
		certificateOK:  certificateOK,
		userResponse:   firebaseUserResponse(false, 0),
	}
	harness.server = httptest.NewTLSServer(http.HandlerFunc(harness.serveHTTP))
	t.Cleanup(harness.server.Close)

	serverURL, err := url.Parse(harness.server.URL)
	if err != nil {
		t.Fatalf("parse local Firebase server URL: %v", err)
	}
	serverTransport := harness.server.Client().Transport
	identityClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		clone := request.Clone(request.Context())
		clone.URL.Scheme = serverURL.Scheme
		clone.URL.Host = serverURL.Host
		return serverTransport.RoundTrip(clone)
	})}

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(signingKey)
	if err != nil {
		t.Fatalf("marshal service-account key: %v", err)
	}
	//nolint:gosec // All service-account material is generated in-memory for the local SDK boundary test.
	credentials, err := json.Marshal(map[string]any{
		"type":           "service_account",
		"project_id":     firebaseContractProject,
		"private_key_id": "test-service-account-key",
		"private_key": string(
			pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}),
		),
		"client_email":                "firebase-contract@example.test",
		"client_id":                   "1234567890",
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   "https://oauth2.googleapis.com/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
	})
	if err != nil {
		t.Fatalf("encode service-account credentials: %v", err)
	}

	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected default HTTP transport %T", http.DefaultTransport)
	}
	certificateTransport := defaultTransport.Clone()
	certificateTransport.Proxy = nil
	certificateTransport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, harness.server.Listener.Addr().String())
	}
	roots := x509.NewCertPool()
	roots.AddCert(harness.server.Certificate())
	serverCertificate := harness.server.Certificate()
	if len(serverCertificate.IPAddresses) == 0 {
		t.Fatal("local Firebase server certificate lacks an IP address")
	}
	certificateTransport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
		ServerName: serverCertificate.IPAddresses[0].String(),
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = certificateTransport
	defer func() { http.DefaultTransport = originalTransport }()

	app, err := firebase.NewApp(
		t.Context(),
		&firebase.Config{ProjectID: firebaseContractProject},
		option.WithAuthCredentialsJSON(option.ServiceAccount, credentials),
		option.WithHTTPClient(identityClient),
	)
	if err != nil {
		t.Fatalf("create local Firebase app: %v", err)
	}
	client, err := app.Auth(t.Context())
	if err != nil {
		t.Fatalf("create local Firebase Auth client: %v", err)
	}
	return NewFirebaseVerifier(client), harness
}

func (harness *firebaseSDKHarness) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	switch {
	case request.URL.Path == "/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com":
		if !harness.certificateOK {
			http.Error(writer, "certificate service unavailable", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Cache-Control", "public, max-age=3600")
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(map[string]string{"known-key": harness.certificatePEM}); err != nil {
			harness.t.Errorf("encode certificate response: %v", err)
		}
	case strings.HasSuffix(request.URL.Path, "/accounts:lookup"):
		body, err := io.ReadAll(request.Body)
		if err != nil {
			harness.t.Errorf("read user lookup body: %v", err)
		}
		if string(body) != `{"localId":["principal"]}` {
			harness.t.Errorf("user lookup body = %s", body)
		}
		harness.mu.Lock()
		harness.userCalls++
		response := harness.userResponse
		harness.mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, response)
	default:
		harness.t.Errorf("unexpected Firebase SDK request %s %s", request.Method, request.URL.Path)
		http.NotFound(writer, request)
	}
}

func (harness *firebaseSDKHarness) setUserResponse(response string) {
	harness.mu.Lock()
	defer harness.mu.Unlock()
	harness.userResponse = response
}

func (harness *firebaseSDKHarness) userCallCount() int {
	harness.mu.Lock()
	defer harness.mu.Unlock()
	return harness.userCalls
}

func firebaseUserResponse(disabled bool, validSince int64) string {
	return `{"users":[{"localId":"principal","validSince":"` + strconv.FormatInt(validSince, 10) +
		`","disabled":` + strconv.FormatBool(disabled) + `}]}`
}

func firebaseTokenClaims(now int64) map[string]any {
	return map[string]any{
		"iss":       "https://securetoken.google.com/" + firebaseContractProject,
		"aud":       firebaseContractProject,
		"sub":       "principal",
		"iat":       now - 10,
		"exp":       now + 3600,
		"auth_time": now - 10,
		"firebase":  map[string]any{"sign_in_provider": "custom"},
	}
}

func cloneClaims(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	maps.Copy(clone, source)
	return clone
}

func firebaseTestToken(
	t *testing.T,
	algorithm, keyID string,
	claims map[string]any,
	signingKey *rsa.PrivateKey,
) string {
	t.Helper()
	header := map[string]any{"alg": algorithm, "typ": "JWT"}
	if keyID != "" {
		header["kid"] = keyID
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("encode JWT header: %v", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("encode JWT payload: %v", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)
	signature := []byte("not-an-rs256-signature")
	if algorithm == "RS256" {
		digest := sha256.Sum256([]byte(signingInput))
		signature, err = rsa.SignPKCS1v15(rand.Reader, signingKey, crypto.SHA256, digest[:])
		if err != nil {
			t.Fatalf("sign JWT: %v", err)
		}
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

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

func TestFirebaseVerifierWithoutClientFailsClosed(t *testing.T) {
	for _, verifier := range []*FirebaseVerifier{nil, NewFirebaseVerifier(nil)} {
		if _, err := verifier.Verify(t.Context(), "token"); !errors.Is(err, ErrAuthUnavailable) {
			t.Fatalf("Verify error = %v, want %v", err, ErrAuthUnavailable)
		}
	}
}

func TestFirebaseVerifierUsesAdminSDKCryptographicAndClaimValidation(t *testing.T) {
	verifier, harness := newFirebaseSDKHarness(t, true)
	now := time.Now().Unix()
	validClaims := firebaseTokenClaims(now)
	validToken := firebaseTestToken(t, "RS256", "known-key", validClaims, harness.signingKey)
	user, err := verifier.Verify(t.Context(), validToken)
	if err != nil || user == nil || user.UID != "principal" {
		t.Fatalf("valid SDK verification = %#v, %v", user, err)
	}
	if harness.userCallCount() != 1 {
		t.Fatalf("valid token revocation checks = %d, want 1", harness.userCallCount())
	}

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate corrupt-signature key: %v", err)
	}
	expired := cloneClaims(validClaims)
	expired["iat"] = now - 2000
	expired["exp"] = now - 1000
	wrongAudience := cloneClaims(validClaims)
	wrongAudience["aud"] = "other-project"
	wrongIssuer := cloneClaims(validClaims)
	wrongIssuer["iss"] = "https://securetoken.google.com/other-project"
	futureIssued := cloneClaims(validClaims)
	futureIssued["iat"] = now + 1000
	emptySubject := cloneClaims(validClaims)
	emptySubject["sub"] = ""
	longSubject := cloneClaims(validClaims)
	longSubject["sub"] = strings.Repeat("a", 129)

	tests := []struct {
		name  string
		token string
		want  error
	}{
		{
			name:  "expired",
			token: firebaseTestToken(t, "RS256", "known-key", expired, harness.signingKey),
			want:  ErrTokenExpired,
		},
		{
			name:  "wrong audience",
			token: firebaseTestToken(t, "RS256", "known-key", wrongAudience, harness.signingKey),
			want:  ErrInvalidToken,
		},
		{
			name:  "wrong issuer",
			token: firebaseTestToken(t, "RS256", "known-key", wrongIssuer, harness.signingKey),
			want:  ErrInvalidToken,
		},
		{
			name:  "future issued at",
			token: firebaseTestToken(t, "RS256", "known-key", futureIssued, harness.signingKey),
			want:  ErrInvalidToken,
		},
		{
			name:  "empty subject",
			token: firebaseTestToken(t, "RS256", "known-key", emptySubject, harness.signingKey),
			want:  ErrInvalidToken,
		},
		{
			name:  "oversized subject",
			token: firebaseTestToken(t, "RS256", "known-key", longSubject, harness.signingKey),
			want:  ErrInvalidToken,
		},
		{
			name:  "missing key ID",
			token: firebaseTestToken(t, "RS256", "", validClaims, harness.signingKey),
			want:  ErrInvalidToken,
		},
		{
			name:  "none algorithm",
			token: firebaseTestToken(t, "none", "known-key", validClaims, nil),
			want:  ErrInvalidToken,
		},
		{
			name:  "symmetric algorithm",
			token: firebaseTestToken(t, "HS256", "known-key", validClaims, nil),
			want:  ErrInvalidToken,
		},
		{
			name:  "unknown key",
			token: firebaseTestToken(t, "RS256", "unknown-key", validClaims, harness.signingKey),
			want:  ErrInvalidToken,
		},
		{
			name:  "corrupted known-key signature",
			token: firebaseTestToken(t, "RS256", "known-key", validClaims, otherKey),
			want:  ErrInvalidToken,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := harness.userCallCount()
			user, err := verifier.Verify(t.Context(), test.token)
			if user != nil || !errors.Is(err, test.want) {
				t.Fatalf("Verify = %#v, %v, want nil/%v", user, err, test.want)
			}
			if harness.userCallCount() != before {
				t.Fatal("invalid token reached the revocation dependency")
			}
		})
	}
}

func TestFirebaseVerifierUsesAdminSDKRevokedDisabledAndDeletedChecks(t *testing.T) {
	verifier, harness := newFirebaseSDKHarness(t, true)
	now := time.Now().Unix()
	token := firebaseTestToken(t, "RS256", "known-key", firebaseTokenClaims(now), harness.signingKey)
	tests := []struct {
		name     string
		response string
		want     error
	}{
		{name: "revoked", response: firebaseUserResponse(false, now), want: ErrTokenRevoked},
		{name: "disabled", response: firebaseUserResponse(true, 0), want: ErrUserDisabled},
		{name: "deleted", response: `{"users":[]}`, want: ErrInvalidToken},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness.setUserResponse(test.response)
			before := harness.userCallCount()
			user, err := verifier.Verify(t.Context(), token)
			if user != nil || !errors.Is(err, test.want) {
				t.Fatalf("Verify = %#v, %v, want nil/%v", user, err, test.want)
			}
			if harness.userCallCount() != before+1 {
				t.Fatalf("revocation dependency calls = %d, want %d", harness.userCallCount(), before+1)
			}
		})
	}
}

func TestFirebaseVerifierCertificateDependencyFailureFailsClosed(t *testing.T) {
	verifier, harness := newFirebaseSDKHarness(t, false)
	token := firebaseTestToken(
		t,
		"RS256",
		"known-key",
		firebaseTokenClaims(time.Now().Unix()),
		harness.signingKey,
	)
	user, err := verifier.Verify(t.Context(), token)
	if user != nil || !errors.Is(err, ErrCertificateFetch) || !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("Verify = %#v, %v, want certificate dependency failure", user, err)
	}
	if harness.userCallCount() != 0 {
		t.Fatal("certificate failure reached the revocation dependency")
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
	testutil.ClearAccounts(t)

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

func TestFirebaseVerifier_Verify_DeletedUser(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearAccounts(t)

	client := newEmulatorAuthClient(t)
	result := testutil.CreateTestUser(t, "deleted@example.com", "password123")
	if err := client.DeleteUser(t.Context(), result.LocalID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	_, err := NewFirebaseVerifier(client).Verify(t.Context(), result.IDToken)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() error = %v, want %v", err, ErrInvalidToken)
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
