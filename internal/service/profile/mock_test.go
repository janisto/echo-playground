package profile

import (
	"context"
	"errors"
	"testing"
)

func TestMockStore_UpdateAllFields(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	_, err := store.Create(ctx, "user-1", CreateParams{
		Firstname:   "John",
		Lastname:    "Doe",
		Email:       "  John@Example.com  ",
		PhoneNumber: " +358401234567 ",
		Marketing:   false,
		Terms:       true,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	newFirst := "Jane"
	newLast := "Smith"
	newEmail := "Jane@Example.com"
	newPhone := "+358409876543"
	newMarketing := true

	updated, err := store.Update(ctx, "user-1", UpdateParams{
		Firstname:   &newFirst,
		Lastname:    &newLast,
		Email:       &newEmail,
		PhoneNumber: &newPhone,
		Marketing:   &newMarketing,
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Firstname != "Jane" {
		t.Fatalf("expected firstname 'Jane', got %q", updated.Firstname)
	}
	if updated.Lastname != "Smith" {
		t.Fatalf("expected lastname 'Smith', got %q", updated.Lastname)
	}
	if updated.Email != "jane@example.com" {
		t.Fatalf("expected lowercase email 'jane@example.com', got %q", updated.Email)
	}
	if updated.PhoneNumber != "+358409876543" {
		t.Fatalf("expected phone '+358409876543', got %q", updated.PhoneNumber)
	}
	if !updated.Marketing {
		t.Fatal("expected marketing true")
	}
}

func TestMockStore_CreateNormalizesInput(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	p, err := store.Create(ctx, "user-2", CreateParams{
		Firstname:   "Alice",
		Lastname:    "Wonder",
		Email:       "  ALICE@Example.COM  ",
		PhoneNumber: "  +1234567890  ",
		Marketing:   true,
		Terms:       true,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if p.Email != "alice@example.com" {
		t.Fatalf("expected lowercase trimmed email, got %q", p.Email)
	}
	if p.PhoneNumber != "+1234567890" {
		t.Fatalf("expected trimmed phone, got %q", p.PhoneNumber)
	}
}

func TestMockStore_UpdatePartialFields(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	_, err := store.Create(ctx, "user-3", CreateParams{
		Firstname:   "Bob",
		Lastname:    "Builder",
		Email:       "bob@example.com",
		PhoneNumber: "+358401111111",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	newFirst := "Robert"
	updated, err := store.Update(ctx, "user-3", UpdateParams{
		Firstname: &newFirst,
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Firstname != "Robert" {
		t.Fatalf("expected 'Robert', got %q", updated.Firstname)
	}
	if updated.Lastname != "Builder" {
		t.Fatalf("expected 'Builder' unchanged, got %q", updated.Lastname)
	}
}

func TestMockStore_UpdateNotFound(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	newFirst := "Jane"
	_, err := store.Update(ctx, "nonexistent", UpdateParams{
		Firstname: &newFirst,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStore_DeleteNotFound(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStore_GetNotFound(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStore_DuplicateCreate(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	_, err := store.Create(ctx, "user-dup", CreateParams{
		Firstname: "A", Lastname: "B", Email: "a@b.com", PhoneNumber: "+1", Terms: true,
	})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	_, err = store.Create(ctx, "user-dup", CreateParams{
		Firstname: "C", Lastname: "D", Email: "c@d.com", PhoneNumber: "+2", Terms: true,
	})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}
