package profile

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/janisto/echo-playground/internal/testutil"
)

func newTestStore(t *testing.T) (*FirestoreStore, func()) {
	t.Helper()
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearFirestore(t)

	ctx := t.Context()
	client, err := firestore.NewClient(ctx, testutil.ProjectID)
	if err != nil {
		t.Fatalf("failed to create firestore client: %v", err)
	}

	store := NewFirestoreStore(client)
	cleanup := func() {
		testutil.ClearFirestore(t)
		if err := client.Close(); err != nil {
			t.Errorf("close Firestore client: %v", err)
		}
	}
	return store, cleanup
}

func TestFirestoreStore_CreateAndGet(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := t.Context()

	params := CreateParams{
		Firstname:   "John",
		Lastname:    "Doe",
		Email:       "  John.Doe@Example.COM  ",
		PhoneNumber: " +1234567890 ",
		Marketing:   true,
	}

	created, err := store.Create(ctx, "user-001", params)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID != "user-001" {
		t.Fatalf("expected ID user-001, got %q", created.ID)
	}
	if created.Email != "  John.Doe@Example.COM  " {
		t.Fatalf("expected preserved email, got %q", created.Email)
	}
	if created.PhoneNumber != " +1234567890 " {
		t.Fatalf("expected preserved phone, got %q", created.PhoneNumber)
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
	if got.Email != "  John.Doe@Example.COM  " {
		t.Fatalf("expected preserved email, got %q", got.Email)
	}
}

func TestFirestoreStore_TreatsUserIDAsOpaque(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	const userID = "firebase/user:with/slash"
	created, err := store.Create(t.Context(), userID, CreateParams{Firstname: "Ada"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != userID {
		t.Fatalf("created ID = %q, want %q", created.ID, userID)
	}
	got, err := store.Get(t.Context(), userID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != userID {
		t.Fatalf("stored ID = %q, want %q", got.ID, userID)
	}
}

func TestProfileDocumentID(t *testing.T) {
	const userID = "firebase/user:with/slash"
	got := profileDocumentID(userID)
	if strings.Contains(got, "/") || got == userID {
		t.Fatalf("profileDocumentID(%q) = %q, want an opaque Firestore document ID", userID, got)
	}
	if got != profileDocumentID(userID) || got == profileDocumentID(userID+"x") {
		t.Fatal("profile document ID mapping must be deterministic and distinct")
	}
}

func TestFirestoreStore_CreateDuplicate(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := t.Context()

	params := CreateParams{
		Firstname: "Jane",
		Lastname:  "Doe",
		Email:     "jane@example.com",
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
	ctx := t.Context()

	_, err := store.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreStore_Update(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := t.Context()

	params := CreateParams{
		Firstname:   "Alice",
		Lastname:    "Smith",
		Email:       "alice@example.com",
		PhoneNumber: "+1111111111",
		Marketing:   false,
	}
	if _, err := store.Create(ctx, "user-upd", params); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	docRef := store.client.Collection(profilesCollection).Doc(profileDocumentID("user-upd"))
	if _, err := docRef.Set(ctx, map[string]any{"future_field": "preserve-me"}, firestore.MergeAll); err != nil {
		t.Fatalf("seed unknown field: %v", err)
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
	if updated.Email != "  Alicia@Example.COM  " {
		t.Fatalf("expected preserved email, got %q", updated.Email)
	}
	if updated.PhoneNumber != " +2222222222 " {
		t.Fatalf("expected preserved phone, got %q", updated.PhoneNumber)
	}
	if !updated.Marketing {
		t.Fatal("expected marketing to be updated to true")
	}
	doc, err := docRef.Get(ctx)
	if err != nil {
		t.Fatalf("read updated document: %v", err)
	}
	if got := doc.Data()["future_field"]; got != "preserve-me" {
		t.Fatalf("unknown field was not preserved: %v", got)
	}
}

func TestFirestoreStore_UpdateNotFound(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := t.Context()

	newName := "Ghost"
	_, err := store.Update(ctx, "nonexistent", UpdateParams{Firstname: &newName})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreStore_UpdateLastnameOnly(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := t.Context()

	params := CreateParams{
		Firstname: "Bob",
		Lastname:  "Builder",
		Email:     "bob@example.com",
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
	ctx := t.Context()

	params := CreateParams{
		Firstname: "Charlie",
		Lastname:  "Brown",
		Email:     "charlie@example.com",
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
	ctx := t.Context()

	err := store.Delete(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFirestoreStore_ConcurrentDelete(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	if _, err := store.Create(t.Context(), "user-concurrent-delete", CreateParams{Firstname: "Ada"}); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	const attempts = 5
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Go(func() {
			errs <- store.Delete(t.Context(), "user-concurrent-delete")
		})
	}
	wg.Wait()
	close(errs)

	successes := 0
	notFound := 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrNotFound):
			notFound++
		default:
			t.Fatalf("unexpected delete error: %v", err)
		}
	}
	if successes != 1 || notFound != attempts-1 {
		t.Fatalf("successes=%d not_found=%d", successes, notFound)
	}
}

func TestClassifyDependencyError(t *testing.T) {
	if err := classifyDependencyError(nil); err != nil {
		t.Fatalf("expected nil to remain nil, got %v", err)
	}
	for _, err := range []error{
		context.DeadlineExceeded,
		status.Error(codes.Aborted, "aborted"),
		status.Error(codes.DeadlineExceeded, "deadline"),
		status.Error(codes.ResourceExhausted, "quota"),
		status.Error(codes.Unavailable, "unavailable"),
	} {
		if got := classifyDependencyError(err); !errors.Is(got, ErrUnavailable) || !errors.Is(got, err) {
			t.Fatalf("expected joined unavailable error for %v, got %v", err, got)
		}
	}
	if got := classifyDependencyError(
		context.Canceled,
	); !errors.Is(got, context.Canceled) ||
		errors.Is(got, ErrUnavailable) {
		t.Fatalf("expected cancellation to remain distinct, got %v", got)
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
		{"unavailable", ErrUnavailable, "unavailable"},
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
