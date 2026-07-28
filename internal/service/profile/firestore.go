package profile

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/janisto/echo-playground/internal/platform/audit"
)

const profilesCollection = "profiles"

func profileDocumentID(userID string) string {
	return "uid_" + base64.RawURLEncoding.EncodeToString([]byte(userID))
}

func categorizeError(err error) string {
	switch {
	case errors.Is(err, ErrAlreadyExists):
		return "already_exists"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrUnavailable):
		return "unavailable"
	default:
		return "internal_error"
	}
}

func classifyDependencyError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(ErrUnavailable, err)
	}
	switch status.Code(err) {
	case codes.Aborted, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Unavailable:
		return errors.Join(ErrUnavailable, err)
	default:
		return err
	}
}

// firestoreProfile maps to Firestore document structure.
type firestoreProfile struct {
	Firstname   string    `firestore:"firstname"`
	Lastname    string    `firestore:"lastname"`
	Email       string    `firestore:"email"`
	PhoneNumber string    `firestore:"phone_number"`
	Marketing   bool      `firestore:"marketing"`
	CreatedAt   time.Time `firestore:"created_at"`
	UpdatedAt   time.Time `firestore:"updated_at"`
}

func newFirestoreProfile(params CreateParams, now time.Time) firestoreProfile {
	return firestoreProfile{
		Firstname:   params.Firstname,
		Lastname:    params.Lastname,
		Email:       params.Email,
		PhoneNumber: params.PhoneNumber,
		Marketing:   params.Marketing,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (p firestoreProfile) toProfile(userID string) *Profile {
	return &Profile{
		ID:          userID,
		Firstname:   p.Firstname,
		Lastname:    p.Lastname,
		Email:       p.Email,
		PhoneNumber: p.PhoneNumber,
		Marketing:   p.Marketing,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// FirestoreStore implements Service using Firestore with transactions.
type FirestoreStore struct {
	client *firestore.Client
}

// NewFirestoreStore creates a new Firestore-backed store.
func NewFirestoreStore(client *firestore.Client) *FirestoreStore {
	return &FirestoreStore{client: client}
}

// Create atomically creates a profile if it does not already exist.
func (s *FirestoreStore) Create(ctx context.Context, userID string, params CreateParams) (*Profile, error) {
	docRef := s.client.Collection(profilesCollection).Doc(profileDocumentID(userID))
	now := time.Now().UTC()
	fp := newFirestoreProfile(params, now)
	_, err := docRef.Create(ctx, fp)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			err = ErrAlreadyExists
		} else {
			err = classifyDependencyError(err)
		}
		audit.LogEvent(ctx, "create", userID, "profile", userID, "failure",
			map[string]any{"error": categorizeError(err)})
		if errors.Is(err, ErrAlreadyExists) {
			return nil, err
		}
		return nil, fmt.Errorf("create profile: %w", err)
	}

	audit.LogEvent(ctx, "create", userID, "profile", userID, "success", nil)

	return fp.toProfile(userID), nil
}

// Get retrieves a profile by user ID.
func (s *FirestoreStore) Get(ctx context.Context, userID string) (*Profile, error) {
	docRef := s.client.Collection(profilesCollection).Doc(profileDocumentID(userID))
	doc, err := docRef.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get profile: %w", classifyDependencyError(err))
	}

	var fp firestoreProfile
	if err := doc.DataTo(&fp); err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}

	return fp.toProfile(userID), nil
}

// Update updates a profile using a transaction for atomicity.
func (s *FirestoreStore) Update(ctx context.Context, userID string, params UpdateParams) (*Profile, error) {
	docRef := s.client.Collection(profilesCollection).Doc(profileDocumentID(userID))

	var result *Profile

	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ErrNotFound
			}
			return err
		}

		var fp firestoreProfile
		if err := doc.DataTo(&fp); err != nil {
			return err
		}

		if params.Firstname != nil {
			fp.Firstname = *params.Firstname
		}
		if params.Lastname != nil {
			fp.Lastname = *params.Lastname
		}
		if params.Email != nil {
			fp.Email = *params.Email
		}
		if params.PhoneNumber != nil {
			fp.PhoneNumber = *params.PhoneNumber
		}
		if params.Marketing != nil {
			fp.Marketing = *params.Marketing
		}
		fp.UpdatedAt = time.Now().UTC()

		if err := tx.Update(docRef, profileUpdates(params, fp.UpdatedAt)); err != nil {
			return err
		}

		result = fp.toProfile(userID)
		return nil
	})
	if err != nil {
		err = classifyDependencyError(err)
		audit.LogEvent(ctx, "update", userID, "profile", userID, "failure",
			map[string]any{"error": categorizeError(err)})
		if errors.Is(err, ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("update profile: %w", err)
	}

	audit.LogEvent(ctx, "update", userID, "profile", userID, "success", nil)

	return result, nil
}

func profileUpdates(params UpdateParams, updatedAt time.Time) []firestore.Update {
	updates := make([]firestore.Update, 0, 6)
	if params.Firstname != nil {
		updates = append(updates, firestore.Update{Path: "firstname", Value: *params.Firstname})
	}
	if params.Lastname != nil {
		updates = append(updates, firestore.Update{Path: "lastname", Value: *params.Lastname})
	}
	if params.Email != nil {
		updates = append(updates, firestore.Update{Path: "email", Value: *params.Email})
	}
	if params.PhoneNumber != nil {
		updates = append(updates, firestore.Update{Path: "phone_number", Value: *params.PhoneNumber})
	}
	if params.Marketing != nil {
		updates = append(updates, firestore.Update{Path: "marketing", Value: *params.Marketing})
	}
	return append(updates, firestore.Update{Path: "updated_at", Value: updatedAt})
}

// Delete atomically removes an existing profile.
func (s *FirestoreStore) Delete(ctx context.Context, userID string) error {
	docRef := s.client.Collection(profilesCollection).Doc(profileDocumentID(userID))
	_, err := docRef.Delete(ctx, firestore.Exists)
	if err != nil {
		switch status.Code(err) {
		case codes.FailedPrecondition, codes.NotFound:
			err = ErrNotFound
		default:
			err = classifyDependencyError(err)
		}
		audit.LogEvent(ctx, "delete", userID, "profile", userID, "failure",
			map[string]any{"error": categorizeError(err)})
		if errors.Is(err, ErrNotFound) {
			return err
		}
		return fmt.Errorf("delete profile: %w", err)
	}

	audit.LogEvent(ctx, "delete", userID, "profile", userID, "success", nil)

	return nil
}

var _ Service = (*FirestoreStore)(nil)
