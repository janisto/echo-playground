package firebase

import (
	"context"
	"testing"

	"github.com/janisto/echo-playground/internal/testutil"
)

func TestInitializeClients(t *testing.T) {
	testutil.RequireEmulator(t)

	ctx := context.Background()
	clients, err := InitializeClients(ctx, Config{
		ProjectID: testutil.EmulatorProjectID,
	})
	if err != nil {
		t.Fatalf("InitializeClients failed: %v", err)
	}
	if clients.Auth == nil {
		t.Fatal("expected Auth client to be non-nil")
	}
	if clients.Firestore == nil {
		t.Fatal("expected Firestore client to be non-nil")
	}
	if err := clients.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestClose_NilFirestore(t *testing.T) {
	c := &Clients{Firestore: nil}
	if err := c.Close(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
