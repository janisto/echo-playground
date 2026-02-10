package profile

import (
	"context"
	"errors"
	"os"
	"testing"

	"cloud.google.com/go/firestore"

	"github.com/janisto/echo-playground/internal/testutil"
)

func newTestStore(t *testing.T) (*FirestoreStore, func()) {
	t.Helper()
	testutil.RequireEmulator(t)

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, testutil.EmulatorProjectID)
	if err != nil {
		t.Fatalf("failed to create firestore client: %v", err)
	}

	store := NewFirestoreStore(client)
	cleanup := func() {
		docs, _ := client.Collection(profilesCollection).Documents(ctx).GetAll()
		for _, doc := range docs {
			_, _ = doc.Ref.Delete(ctx)
		}
		_ = client.Close()
	}
	return store, cleanup
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestFirestoreStore_CreateAndGet(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()

	params := CreateParams{
		Firstname:   "John",
		Lastname:    "Doe",
		Email:       "  John.Doe@Example.COM  ",
		PhoneNumber: " +1234567890 ",
		Marketing:   true,
		Terms:       true,
	}

	created, err := store.Create(ctx, "user-001", params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID != "user-001" {
		t.Fatalf("expected ID user-001, got %q", created.ID)
	}
	if created.Email != "john.doe@example.com" {
		t.Fatalf("expected normalized email, got %q", created.Email)
	}
	if created.PhoneNumber != "+1234567890" {
		t.Fatalf("expected trimmed phone, got %q", created.PhoneNumber)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}

	got, err := store.Get(ctx, "user-001")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Firstname != "John" {
		t.Fatalf("expected firstname John, got %q", got.Firstname)
	}
	if got.Email != "john.doe@example.com" {
		t.Fatalf("expected email john.doe@example.com, got %q", got.Email)
	}
}

func TestFirestoreStore_CreateDuplicate(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	params := CreateParams{
		Firstname: "Jane",
		Lastname:  "Doe",
		Email:     "jane@example.com",
		Terms:     true,
	}

	if _, err := store.Create(ctx, "user-dup", params); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	_, err := store.Create(ctx, "user-dup", params)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestFirestoreStore_GetNotFound(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreStore_Update(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	params := CreateParams{
		Firstname:   "Alice",
		Lastname:    "Smith",
		Email:       "alice@example.com",
		PhoneNumber: "+1111111111",
		Marketing:   false,
		Terms:       true,
	}
	if _, err := store.Create(ctx, "user-upd", params); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	newFirst := "Alicia"
	newEmail := "  Alicia@Example.COM  "
	newPhone := " +2222222222 "
	newMarketing := true
	updated, err := store.Update(ctx, "user-upd", UpdateParams{
		Firstname:   &newFirst,
		Email:       &newEmail,
		PhoneNumber: &newPhone,
		Marketing:   &newMarketing,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if updated.Firstname != "Alicia" {
		t.Fatalf("expected firstname Alicia, got %q", updated.Firstname)
	}
	if updated.Lastname != "Smith" {
		t.Fatalf("expected lastname Smith (unchanged), got %q", updated.Lastname)
	}
	if updated.Email != "alicia@example.com" {
		t.Fatalf("expected normalized email, got %q", updated.Email)
	}
	if updated.PhoneNumber != "+2222222222" {
		t.Fatalf("expected trimmed phone, got %q", updated.PhoneNumber)
	}
	if !updated.Marketing {
		t.Fatal("expected marketing to be updated to true")
	}
	if !updated.Terms {
		t.Fatal("expected terms to remain true")
	}
}

func TestFirestoreStore_UpdateNotFound(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	newName := "Ghost"
	_, err := store.Update(ctx, "nonexistent", UpdateParams{Firstname: &newName})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreStore_UpdateLastnameOnly(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	params := CreateParams{
		Firstname: "Bob",
		Lastname:  "Builder",
		Email:     "bob@example.com",
		Terms:     true,
	}
	if _, err := store.Create(ctx, "user-ln", params); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	newLast := "Constructor"
	updated, err := store.Update(ctx, "user-ln", UpdateParams{Lastname: &newLast})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if updated.Lastname != "Constructor" {
		t.Fatalf("expected lastname Constructor, got %q", updated.Lastname)
	}
	if updated.Firstname != "Bob" {
		t.Fatalf("expected firstname Bob (unchanged), got %q", updated.Firstname)
	}
}

func TestFirestoreStore_Delete(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	params := CreateParams{
		Firstname: "Charlie",
		Lastname:  "Brown",
		Email:     "charlie@example.com",
		Terms:     true,
	}
	if _, err := store.Create(ctx, "user-del", params); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := store.Delete(ctx, "user-del"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := store.Get(ctx, "user-del")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestFirestoreStore_DeleteNotFound(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"already exists", ErrAlreadyExists, "already_exists"},
		{"not found", ErrNotFound, "not_found"},
		{"generic error", context.Canceled, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categorizeError(tt.err)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
