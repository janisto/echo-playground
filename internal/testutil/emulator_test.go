package testutil

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestEmulatorAvailable_Unreachable(t *testing.T) {
	if emulatorAvailable("127.0.0.1:1") {
		t.Fatal("expected false for unreachable host")
	}
}

func TestEmulatorAvailable_Reachable(t *testing.T) {
	ts := httptest.NewServer(nil)
	defer ts.Close()

	if !emulatorAvailable(ts.Listener.Addr().String()) {
		t.Fatal("expected true for reachable host")
	}
}

func TestSkipIfEmulatorUnavailable(t *testing.T) {
	if EmulatorAvailable() {
		t.Skip("emulators are running; cannot test skip behavior")
	}
	t.Run("sub", func(t *testing.T) {
		SkipIfEmulatorUnavailable(t)
		t.Fatal("expected test to be skipped")
	})
}

func TestSetupEmulator(t *testing.T) {
	SetupEmulator(t)

	if got := os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"); got != AuthEmulatorHost {
		t.Fatalf("expected %s, got %s", AuthEmulatorHost, got)
	}
	if got := os.Getenv("FIRESTORE_EMULATOR_HOST"); got != FirestoreEmulatorHost {
		t.Fatalf("expected %s, got %s", FirestoreEmulatorHost, got)
	}
}

func TestUnexpectedStatusError(t *testing.T) {
	if err := unexpectedStatusError(
		&http.Response{StatusCode: http.StatusNoContent},
		http.StatusNoContent,
	); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	err := unexpectedStatusError(&http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 9<<10))),
	}, http.StatusOK)
	if err == nil || !strings.HasPrefix(err.Error(), "status 500: ") ||
		len(err.Error()) != len("status 500: ")+(8<<10) {
		t.Fatalf("unexpected bounded error: %v", err)
	}

	err = unexpectedStatusError(&http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(errorReader{}),
	}, http.StatusOK)
	if err == nil || err.Error() != "status 502; read response: read failed" {
		t.Fatalf("unexpected read error: %v", err)
	}
}

func TestDecodeSignUpResponse(t *testing.T) {
	result, err := decodeSignUpResponse(strings.NewReader(`{"idToken":"token","localId":"user"}`))
	if err != nil || result.IDToken != "token" || result.LocalID != "user" {
		t.Fatalf("unexpected result=%#v error=%v", result, err)
	}
	for _, body := range []string{`{}`, `{"idToken":"token"}`, `{"localId":"user"}`} {
		if _, err := decodeSignUpResponse(strings.NewReader(body)); err == nil {
			t.Fatalf("expected incomplete identity error for %s", body)
		}
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}
