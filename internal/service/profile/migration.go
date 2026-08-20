package profile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/janisto/echo-playground/internal/platform/validate"
)

const ProfileMigrationManifestVersion = 1

type MigrationAuthorization struct {
	TermsAccepted bool   `json:"termsAccepted"`
	Evidence      string `json:"evidence"`
}

type MigrationManifest struct {
	Version int                               `json:"version"`
	Entries map[string]MigrationAuthorization `json:"entries"`
}

type MigrationState string

const (
	MigrationVerified MigrationState = "verified"
	MigrationRequired MigrationState = "migration_required"
	MigrationBlocked  MigrationState = "blocked"
	MigrationApplied  MigrationState = "applied"
)

type MigrationResult struct {
	DocumentFingerprint string
	State               MigrationState
	Reason              string
}

func (manifest MigrationManifest) Validate() error {
	if manifest.Version != ProfileMigrationManifestVersion || manifest.Entries == nil {
		return errors.New("profile migration manifest must have version 1 and an entries object")
	}
	for documentID, authorization := range manifest.Entries {
		if documentID == "" || strings.Contains(documentID, "/") {
			return errors.New("profile migration manifest contains an invalid document ID")
		}
		if !authorization.TermsAccepted || !validEvidence(authorization.Evidence) {
			return errors.New(
				"every profile migration authorization must affirm terms acceptance and cite safe evidence",
			)
		}
	}
	return nil
}

func AuditProfileMigration(
	ctx context.Context,
	client *firestore.Client,
	manifest MigrationManifest,
) ([]MigrationResult, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	documents := client.Collection(profilesCollection).Documents(ctx)
	defer documents.Stop()
	results := make([]MigrationResult, 0)
	for {
		snapshot, err := documents.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate profile documents: %w", err)
		}
		state, reason, _ := classifyMigration(snapshot.Data(), manifest.Entries[snapshot.Ref.ID])
		results = append(results, MigrationResult{
			DocumentFingerprint: fingerprintDocumentID(snapshot.Ref.ID),
			State:               state,
			Reason:              reason,
		})
	}
	return results, nil
}

func ApplyProfileMigration(
	ctx context.Context,
	client *firestore.Client,
	manifest MigrationManifest,
) ([]MigrationResult, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	audit, err := AuditProfileMigration(ctx, client, manifest)
	if err != nil {
		return nil, err
	}
	for _, result := range audit {
		if result.State == MigrationBlocked {
			return audit, errors.New("profile migration is blocked; no records were changed")
		}
	}

	results := make([]MigrationResult, 0, len(audit))
	documents := client.Collection(profilesCollection).Documents(ctx)
	defer documents.Stop()
	for {
		snapshot, nextErr := documents.Next()
		if errors.Is(nextErr, iterator.Done) {
			break
		}
		if nextErr != nil {
			return results, fmt.Errorf("iterate profile documents for apply: %w", nextErr)
		}
		result := MigrationResult{DocumentFingerprint: fingerprintDocumentID(snapshot.Ref.ID)}
		transactionErr := client.RunTransaction(
			ctx,
			func(ctx context.Context, transaction *firestore.Transaction) error {
				current, getErr := transaction.Get(snapshot.Ref)
				if getErr != nil {
					return getErr
				}
				state, reason, replacement := classifyMigration(current.Data(), manifest.Entries[snapshot.Ref.ID])
				result.State, result.Reason = state, reason
				switch state {
				case MigrationVerified:
					return nil
				case MigrationRequired:
					if setErr := transaction.Set(snapshot.Ref, replacement); setErr != nil {
						return setErr
					}
					result.State = MigrationApplied
					return nil
				default:
					return errors.New("profile record became blocked during migration")
				}
			},
		)
		if transactionErr != nil {
			return results, fmt.Errorf("apply profile migration to %s: %w", result.DocumentFingerprint, transactionErr)
		}
		results = append(results, result)
	}
	return results, nil
}

func classifyMigration(
	data map[string]any,
	authorization MigrationAuthorization,
) (MigrationState, string, map[string]any) {
	if stored, ok := canonicalProfileData(data); ok {
		return MigrationVerified, "record already satisfies the accepted schema", canonicalProfileMap(stored)
	}
	if !exactLegacyProfileFields(data) {
		return MigrationBlocked, "record is neither an exact canonical record nor the known pre-adoption shape", nil
	}
	if !authorization.TermsAccepted || !validEvidence(authorization.Evidence) {
		return MigrationBlocked, "legacy record lacks an explicit terms-acceptance evidence entry", nil
	}
	legacy, ok := legacyProfileData(data)
	if !ok {
		return MigrationBlocked, "legacy record contains an invalid field type or lifecycle", nil
	}
	canonical := firestoreProfile{
		FirstName:      legacy.FirstName,
		LastName:       legacy.LastName,
		ContactEmail:   validate.NormalizeContactEmail(legacy.ContactEmail),
		PhoneNumber:    validate.StripASCIIWhitespace(legacy.PhoneNumber),
		MarketingOptIn: legacy.MarketingOptIn,
		TermsAccepted:  true,
		CreatedAt:      legacy.CreatedAt.UTC().Truncate(time.Millisecond),
		UpdatedAt:      legacy.UpdatedAt.UTC().Truncate(time.Millisecond),
	}
	if !canonical.valid() {
		return MigrationBlocked, "legacy record cannot be converted without inventing or repairing domain data", nil
	}
	return MigrationRequired, "known pre-adoption record has an authorized canonical replacement", canonicalProfileMap(
		canonical,
	)
}

type legacyProfile struct {
	FirstName      string
	LastName       string
	ContactEmail   string
	PhoneNumber    string
	MarketingOptIn bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func legacyProfileData(data map[string]any) (legacyProfile, bool) {
	firstName, firstNameOK := data["firstname"].(string)
	lastName, lastNameOK := data["lastname"].(string)
	contactEmail, contactEmailOK := data["email"].(string)
	phoneNumber, phoneNumberOK := data["phone_number"].(string)
	marketing, marketingOK := data["marketing"].(bool)
	createdAt, createdAtOK := data["created_at"].(time.Time)
	updatedAt, updatedAtOK := data["updated_at"].(time.Time)
	if !firstNameOK || !lastNameOK || !contactEmailOK || !phoneNumberOK || !marketingOK || !createdAtOK ||
		!updatedAtOK ||
		updatedAt.Before(createdAt) {
		return legacyProfile{}, false
	}
	return legacyProfile{
		FirstName: firstName, LastName: lastName, ContactEmail: contactEmail,
		PhoneNumber: phoneNumber, MarketingOptIn: marketing, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, true
}

func canonicalProfileData(data map[string]any) (firestoreProfile, bool) {
	if !exactCanonicalProfileFields(data) {
		return firestoreProfile{}, false
	}
	firstName, firstNameOK := data["first_name"].(string)
	lastName, lastNameOK := data["last_name"].(string)
	contactEmail, contactEmailOK := data["contact_email"].(string)
	phoneNumber, phoneNumberOK := data["phone_number"].(string)
	marketing, marketingOK := data["marketing_opt_in"].(bool)
	terms, termsOK := data["terms_accepted"].(bool)
	createdAt, createdAtOK := data["created_at"].(time.Time)
	updatedAt, updatedAtOK := data["updated_at"].(time.Time)
	stored := firestoreProfile{
		FirstName: firstName, LastName: lastName, ContactEmail: contactEmail, PhoneNumber: phoneNumber,
		MarketingOptIn: marketing, TermsAccepted: terms, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	return stored, firstNameOK && lastNameOK && contactEmailOK && phoneNumberOK && marketingOK && termsOK &&
		createdAtOK && updatedAtOK && stored.valid()
}

func canonicalProfileMap(stored firestoreProfile) map[string]any {
	return map[string]any{
		"first_name": stored.FirstName, "last_name": stored.LastName,
		"contact_email": stored.ContactEmail, "phone_number": stored.PhoneNumber,
		"marketing_opt_in": stored.MarketingOptIn, "terms_accepted": stored.TermsAccepted,
		"created_at": stored.CreatedAt, "updated_at": stored.UpdatedAt,
	}
}

func exactCanonicalProfileFields(data map[string]any) bool {
	return exactFields(
		data,
		"first_name",
		"last_name",
		"contact_email",
		"phone_number",
		"marketing_opt_in",
		"terms_accepted",
		"created_at",
		"updated_at",
	)
}

func exactLegacyProfileFields(data map[string]any) bool {
	return exactFields(data, "firstname", "lastname", "email", "phone_number", "marketing", "created_at", "updated_at")
}

func exactFields(data map[string]any, fields ...string) bool {
	if len(data) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, ok := data[field]; !ok {
			return false
		}
	}
	return true
}

func validEvidence(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value ||
		utf8.RuneCountInString(value) > 500 {
		return false
	}
	for _, character := range value {
		if character <= 0x1f || character >= 0x7f && character <= 0x9f {
			return false
		}
	}
	return true
}

func fingerprintDocumentID(documentID string) string {
	digest := sha256.Sum256([]byte(documentID))
	return hex.EncodeToString(digest[:8])
}
