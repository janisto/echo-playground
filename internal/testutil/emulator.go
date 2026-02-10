package testutil

import (
	"context"
	"net"
	"os"
	"testing"
)

// RequireEmulator skips the test if the Firebase Emulator is not running.
// It checks the FIRESTORE_EMULATOR_HOST environment variable and verifies
// connectivity to the emulator.
func RequireEmulator(t *testing.T) {
	t.Helper()

	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST not set; skipping emulator test")
	}

	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "tcp", host)
	if err != nil {
		t.Skipf("Firestore emulator not reachable at %s: %v", host, err)
	}
	_ = conn.Close()
}

// EmulatorProjectID returns the project ID used for emulator tests.
const EmulatorProjectID = "demo-test-project"
