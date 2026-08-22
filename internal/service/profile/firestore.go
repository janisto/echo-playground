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
	"github.com/janisto/echo-playground/internal/platform/validate"
)

const profilesCollection = "profiles"

var maximumTimestamp = time.Date(9999, 12, 31, 23, 59, 59, 999_000_000, time.UTC)

func profileDocumentID(userID string) string {
	return "uid_" + base64.RawURLEncoding.EncodeToString([]byte(userID))
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

type firestoreProfile struct {
	FirstName      string    `firestore:"first_name"`
	LastName       string    `firestore:"last_name"`
	ContactEmail   string    `firestore:"contact_email"`
	PhoneNumber    string    `firestore:"phone_number"`
	MarketingOptIn bool      `firestore:"marketing_opt_in"`
	TermsAccepted  bool      `firestore:"terms_accepted"`
	CreatedAt      time.Time `firestore:"created_at"`
	UpdatedAt      time.Time `firestore:"updated_at"`
}

func newFirestoreProfile(params CreateParams, now time.Time) firestoreProfile {
	return firestoreProfile{
		FirstName:      params.FirstName,
		LastName:       params.LastName,
		ContactEmail:   params.ContactEmail,
		PhoneNumber:    params.PhoneNumber,
		MarketingOptIn: params.MarketingOptIn,
		TermsAccepted:  params.TermsAccepted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (profile firestoreProfile) toProfile(userID string) *Profile {
	return &Profile{
		ID:             userID,
		FirstName:      profile.FirstName,
		LastName:       profile.LastName,
		ContactEmail:   profile.ContactEmail,
		PhoneNumber:    profile.PhoneNumber,
		MarketingOptIn: profile.MarketingOptIn,
		TermsAccepted:  profile.TermsAccepted,
		CreatedAt:      profile.CreatedAt,
		UpdatedAt:      profile.UpdatedAt,
	}
}

func (profile firestoreProfile) valid() bool {
	return validate.BoundedName(profile.FirstName) && validate.BoundedName(profile.LastName) &&
		validate.ContactEmail(
			profile.ContactEmail,
		) && validate.NormalizeContactEmail(profile.ContactEmail) == profile.ContactEmail &&
		validate.PhoneNumber(
			profile.PhoneNumber,
		) && validate.StripASCIIWhitespace(profile.PhoneNumber) == profile.PhoneNumber &&
		profile.TermsAccepted && !profile.CreatedAt.IsZero() && profile.CreatedAt.Year() >= 0 && profile.CreatedAt.Year() <= 9999 &&
		profile.CreatedAt.Equal(profile.CreatedAt.UTC().Truncate(time.Millisecond)) &&
		profile.UpdatedAt.Equal(profile.UpdatedAt.UTC().Truncate(time.Millisecond)) &&
		!profile.UpdatedAt.Before(profile.CreatedAt) && !profile.UpdatedAt.After(maximumTimestamp)
}

type FirestoreStore struct {
	client *firestore.Client
	clock  func() time.Time
}

func NewFirestoreStore(client *firestore.Client) *FirestoreStore {
	return &FirestoreStore{client: client, clock: time.Now}
}

func (store *FirestoreStore) Create(ctx context.Context, userID string, params CreateParams) (*Profile, error) {
	now, err := normalizeProfileClock(store.clock())
	if err != nil {
		audit.LogEvent(ctx, "create", "profile", "failure", map[string]any{"error": safeErrorCategory(err)})
		return nil, err
	}
	document := store.client.Collection(profilesCollection).Doc(profileDocumentID(userID))
	stored := newFirestoreProfile(params, now)
	_, err = document.Create(ctx, stored)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			audit.LogEvent(ctx, "create", "profile", "failure", map[string]any{"error": "already_exists"})
			return nil, ErrAlreadyExists
		}
		err = classifyDependencyError(err)
		audit.LogEvent(ctx, "create", "profile", "failure", map[string]any{"error": safeErrorCategory(err)})
		return nil, fmt.Errorf("create profile: %w", err)
	}
	audit.LogEvent(ctx, "create", "profile", "success", nil)
	return stored.toProfile(userID), nil
}

func (store *FirestoreStore) Get(ctx context.Context, userID string) (*Profile, error) {
	document, err := store.client.Collection(profilesCollection).Doc(profileDocumentID(userID)).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get profile: %w", classifyDependencyError(err))
	}
	stored, err := decodeStoredProfile(document)
	if err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}
	return stored.toProfile(userID), nil
}

func (store *FirestoreStore) Update(ctx context.Context, userID string, params UpdateParams) (*Profile, error) {
	document := store.client.Collection(profilesCollection).Doc(profileDocumentID(userID))
	now := store.clock()
	var result *Profile
	err := store.client.RunTransaction(ctx, func(ctx context.Context, transaction *firestore.Transaction) error {
		snapshot, err := transaction.Get(document)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ErrNotFound
			}
			return err
		}
		stored, err := decodeStoredProfile(snapshot)
		if err != nil {
			return err
		}
		changed := applyUpdate(&stored, params)
		if !changed {
			result = stored.toProfile(userID)
			return nil
		}
		updatedAt, err := nextUpdatedAt(stored.UpdatedAt, now)
		if err != nil {
			return err
		}
		stored.UpdatedAt = updatedAt
		if err := transaction.Set(document, stored); err != nil {
			return err
		}
		result = stored.toProfile(userID)
		return nil
	})
	if err != nil {
		err = classifyDependencyError(err)
		audit.LogEvent(ctx, "update", "profile", "failure", map[string]any{"error": safeErrorCategory(err)})
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidStoredData) ||
			errors.Is(err, ErrTimestampExhausted) {
			return nil, err
		}
		return nil, fmt.Errorf("update profile: %w", err)
	}
	audit.LogEvent(ctx, "update", "profile", "success", nil)
	return result, nil
}

func (store *FirestoreStore) Delete(ctx context.Context, userID string) error {
	document := store.client.Collection(profilesCollection).Doc(profileDocumentID(userID))
	_, err := document.Delete(ctx, firestore.Exists)
	if err != nil {
		switch status.Code(err) {
		case codes.FailedPrecondition, codes.NotFound:
			err = ErrNotFound
		default:
			err = classifyDependencyError(err)
		}
		audit.LogEvent(ctx, "delete", "profile", "failure", map[string]any{"error": safeErrorCategory(err)})
		if errors.Is(err, ErrNotFound) {
			return err
		}
		return fmt.Errorf("delete profile: %w", err)
	}
	audit.LogEvent(ctx, "delete", "profile", "success", nil)
	return nil
}

func decodeStoredProfile(snapshot *firestore.DocumentSnapshot) (firestoreProfile, error) {
	data := snapshot.Data()
	expected := map[string]struct{}{
		"first_name": {}, "last_name": {}, "contact_email": {}, "phone_number": {},
		"marketing_opt_in": {}, "terms_accepted": {}, "created_at": {}, "updated_at": {},
	}
	if len(data) != len(expected) {
		return firestoreProfile{}, ErrInvalidStoredData
	}
	for field := range data {
		if _, ok := expected[field]; !ok {
			return firestoreProfile{}, ErrInvalidStoredData
		}
	}
	var stored firestoreProfile
	if err := snapshot.DataTo(&stored); err != nil || !stored.valid() {
		return firestoreProfile{}, ErrInvalidStoredData
	}
	return stored, nil
}

func applyUpdate(stored *firestoreProfile, params UpdateParams) bool {
	changed := false
	if params.FirstName != nil && stored.FirstName != *params.FirstName {
		stored.FirstName, changed = *params.FirstName, true
	}
	if params.LastName != nil && stored.LastName != *params.LastName {
		stored.LastName, changed = *params.LastName, true
	}
	if params.ContactEmail != nil && stored.ContactEmail != *params.ContactEmail {
		stored.ContactEmail, changed = *params.ContactEmail, true
	}
	if params.PhoneNumber != nil && stored.PhoneNumber != *params.PhoneNumber {
		stored.PhoneNumber, changed = *params.PhoneNumber, true
	}
	if params.MarketingOptIn != nil && stored.MarketingOptIn != *params.MarketingOptIn {
		stored.MarketingOptIn, changed = *params.MarketingOptIn, true
	}
	return changed
}

func nextUpdatedAt(previous, now time.Time) (time.Time, error) {
	now, err := normalizeProfileClock(now)
	if err != nil {
		return time.Time{}, err
	}
	if now.After(previous) {
		return now, nil
	}
	if previous.Equal(maximumTimestamp) {
		return time.Time{}, ErrTimestampExhausted
	}
	return previous.Add(time.Millisecond), nil
}

func normalizeProfileClock(now time.Time) (time.Time, error) {
	normalized := now.UTC().Truncate(time.Millisecond)
	if normalized.Year() < 0 || normalized.After(maximumTimestamp) {
		return time.Time{}, ErrTimestampExhausted
	}
	return normalized, nil
}

func safeErrorCategory(err error) string {
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

var _ Service = (*FirestoreStore)(nil)
