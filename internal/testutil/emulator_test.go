package testutil

import (
	"net/http/httptest"
	"os"
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
