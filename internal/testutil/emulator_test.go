package testutil

import (
	"net/http/httptest"
	"testing"
)

func TestRequireEmulator_NoHost(t *testing.T) {
	t.Setenv("FIRESTORE_EMULATOR_HOST", "")

	t.Run("sub", func(t *testing.T) {
		RequireEmulator(t)
	})
}

func TestRequireEmulator_Unreachable(t *testing.T) {
	t.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:1")

	t.Run("sub", func(t *testing.T) {
		RequireEmulator(t)
	})
}

func TestRequireEmulator_Reachable(t *testing.T) {
	ts := httptest.NewServer(nil)
	defer ts.Close()

	t.Setenv("FIRESTORE_EMULATOR_HOST", ts.Listener.Addr().String())

	t.Run("sub", func(t *testing.T) {
		RequireEmulator(t)
	})
}
