package testutil

import (
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

func TestSuccessfulResponseError(t *testing.T) {
	if err := successfulResponseError(&http.Response{StatusCode: http.StatusNoContent}, "cleanup"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	err := successfulResponseError(&http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Internal Server Error",
		Body:       io.NopCloser(strings.NewReader("emulator failed")),
	}, "cleanup")
	if err == nil || !strings.Contains(err.Error(), "cleanup: unexpected status 500") ||
		!strings.Contains(err.Error(), "emulator failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
