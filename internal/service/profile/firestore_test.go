package profile

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/janisto/echo-playground/internal/testutil"
)

var canonicalCreate = CreateParams{
	FirstName: "Ada", LastName: "Lovelace", ContactEmail: "Ada@example.com",
	PhoneNumber: "+358401234567", MarketingOptIn: false, TermsAccepted: true,
}

func newTestStore(t *testing.T) (*FirestoreStore, func()) {
	t.Helper()
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearFirestore(t)
	client, err := firestore.NewClient(t.Context(), testutil.ProjectID)
	if err != nil {
		t.Fatalf("create Firestore client: %v", err)
	}
	cleanup := func() {
		testutil.ClearFirestore(t)
		if err := client.Close(); err != nil {
			t.Errorf("close Firestore client: %v", err)
		}
	}
	return NewFirestoreStore(client), cleanup
}

func TestFirestoreLifecycleAndNoOpWrite(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	createdClock := time.Date(2026, 7, 30, 12, 0, 0, 987_654_321, time.UTC)
	store.clock = func() time.Time { return createdClock }
	created, err := store.Create(t.Context(), "principal", canonicalCreate)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	wantCreated := createdClock.Truncate(time.Millisecond)
	if created.ID != "principal" || !created.CreatedAt.Equal(wantCreated) || !created.UpdatedAt.Equal(wantCreated) {
		t.Fatalf("created = %#v", created)
	}
	document := store.client.Collection(profilesCollection).Doc(profileDocumentID("principal"))
	before, err := document.Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	name := "Ada"
	store.clock = func() time.Time { return createdClock.Add(time.Hour) }
	unchanged, err := store.Update(t.Context(), "principal", UpdateParams{FirstName: &name})
	if err != nil || !unchanged.UpdatedAt.Equal(wantCreated) {
		t.Fatalf("no-op update = %#v, %v", unchanged, err)
	}
	afterNoOp, _ := document.Get(t.Context())
	if !afterNoOp.UpdateTime.Equal(before.UpdateTime) {
		t.Fatalf("no-op committed a write: %s -> %s", before.UpdateTime, afterNoOp.UpdateTime)
	}

	newName := "Grace"
	store.clock = func() time.Time { return createdClock.Add(-time.Hour) }
	changed, err := store.Update(t.Context(), "principal", UpdateParams{FirstName: &newName})
	if err != nil || changed.FirstName != "Grace" || !changed.CreatedAt.Equal(wantCreated) ||
		!changed.UpdatedAt.Equal(wantCreated.Add(time.Millisecond)) {
		t.Fatalf("changed update = %#v, %v", changed, err)
	}
	got, err := store.Get(t.Context(), "principal")
	if err != nil || got.FirstName != "Grace" || got.ContactEmail != "Ada@example.com" || !got.TermsAccepted {
		t.Fatalf("Get = %#v, %v", got, err)
	}
	if err := store.Delete(t.Context(), "principal"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(t.Context(), "principal"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete = %v", err)
	}
}

func TestFirestoreAtomicCreateAndDeleteRaces(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	store.clock = func() time.Time { return time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC) }
	createErrors := concurrentErrors(2, func() error {
		_, err := store.Create(t.Context(), "principal", canonicalCreate)
		return err
	})
	assertRaceOutcomes(t, createErrors, nil, ErrAlreadyExists)
	stored, err := store.Get(t.Context(), "principal")
	if err != nil || stored.FirstName != canonicalCreate.FirstName || stored.LastName != canonicalCreate.LastName ||
		!stored.TermsAccepted {
		t.Fatalf("race winner = %#v, %v", stored, err)
	}
	deleteErrors := concurrentErrors(2, func() error { return store.Delete(t.Context(), "principal") })
	assertRaceOutcomes(t, deleteErrors, nil, ErrNotFound)
}

func TestStoredProfileFailsClosedOnLegacyOrNoncanonicalData(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	document := store.client.Collection(profilesCollection).Doc(profileDocumentID("principal"))
	_, err := document.Set(t.Context(), map[string]any{
		"firstname": "Ada", "lastname": "Lovelace", "email": "Ada@example.com",
		"phone_number": "+358401234567", "marketing": false,
		"created_at": time.Now(), "updated_at": time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), "principal"); !errors.Is(err, ErrInvalidStoredData) {
		t.Fatalf("legacy Get error = %v", err)
	}
	if _, err := store.Update(t.Context(), "principal", UpdateParams{}); !errors.Is(err, ErrInvalidStoredData) {
		t.Fatalf("legacy Update error = %v", err)
	}
}

func TestMigrationClassificationRequiresTermsEvidenceAndCanonicalizes(t *testing.T) {
	created := time.Date(2026, 7, 30, 12, 0, 0, 987_654_000, time.FixedZone("offset", 2*60*60))
	legacy := map[string]any{
		"firstname": "Ada", "lastname": "Lovelace", "email": " Ada@EXAMPLE.com ",
		"phone_number": " +358401234567 ", "marketing": true,
		"created_at": created, "updated_at": created.Add(time.Second),
	}
	state, _, replacement := classifyMigration(legacy, MigrationAuthorization{})
	if state != MigrationBlocked || replacement != nil {
		t.Fatalf("migration without evidence = %s %#v", state, replacement)
	}
	state, _, replacement = classifyMigration(
		legacy,
		MigrationAuthorization{TermsAccepted: true, Evidence: "consent-ledger/record-1"},
	)
	replacementCreatedAt, hasCreatedAt := replacement["created_at"].(time.Time)
	if state != MigrationRequired || replacement["contact_email"] != "Ada@example.com" ||
		replacement["phone_number"] != "+358401234567" ||
		replacement["terms_accepted"] != true ||
		!hasCreatedAt || !replacementCreatedAt.Equal(created.UTC().Truncate(time.Millisecond)) {
		t.Fatalf("canonical replacement = %s %#v", state, replacement)
	}
	state, _, _ = classifyMigration(replacement, MigrationAuthorization{})
	if state != MigrationVerified {
		t.Fatalf("canonical replacement classified %s", state)
	}
	legacy["unknown"] = "value"
	state, _, _ = classifyMigration(
		legacy,
		MigrationAuthorization{TermsAccepted: true, Evidence: "consent-ledger/record-1"},
	)
	if state != MigrationBlocked {
		t.Fatalf("unknown legacy field classified %s", state)
	}
	delete(legacy, "unknown")
	legacy["email"] = "Ada@example.\u212AOM"
	state, _, replacement = classifyMigration(
		legacy,
		MigrationAuthorization{TermsAccepted: true, Evidence: "consent-ledger/record-1"},
	)
	if state != MigrationBlocked || replacement != nil {
		t.Fatalf("internationalized legacy email classified %s: %#v", state, replacement)
	}
}

func TestProfileMigrationAgainstEmulator(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	documentID := profileDocumentID("principal")
	_, err := store.client.Collection(profilesCollection).Doc(documentID).Set(t.Context(), map[string]any{
		"firstname": "Ada", "lastname": "Lovelace", "email": "Ada@EXAMPLE.com",
		"phone_number": "+358401234567", "marketing": false,
		"created_at": time.Date(2026, 7, 30, 12, 0, 0, 123_000, time.UTC),
		"updated_at": time.Date(2026, 7, 30, 12, 1, 0, 456_000, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := MigrationManifest{Version: 1, Entries: map[string]MigrationAuthorization{
		documentID: {TermsAccepted: true, Evidence: "consent-ledger/record-1"},
	}}
	audit, err := AuditProfileMigration(t.Context(), store.client, manifest)
	if err != nil || len(audit) != 1 || audit[0].State != MigrationRequired {
		t.Fatalf("audit = %#v, %v", audit, err)
	}
	results, err := ApplyProfileMigration(t.Context(), store.client, manifest)
	if err != nil || len(results) != 1 || results[0].State != MigrationApplied {
		t.Fatalf("apply = %#v, %v", results, err)
	}
	got, err := store.Get(t.Context(), "principal")
	if err != nil || got.ContactEmail != "Ada@example.com" || !got.TermsAccepted ||
		got.CreatedAt.Nanosecond()%1_000_000 != 0 {
		t.Fatalf("migrated profile = %#v, %v", got, err)
	}
}

func TestProfileTimeAdvanceAndExhaustion(t *testing.T) {
	previous := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	for _, now := range []time.Time{previous, previous.Add(-time.Hour)} {
		got, err := nextUpdatedAt(previous, now)
		if err != nil || !got.Equal(previous.Add(time.Millisecond)) {
			t.Fatalf("nextUpdatedAt(%s) = %s, %v", now, got, err)
		}
	}
	later := previous.Add(time.Hour + 987*time.Microsecond)
	if got, err := nextUpdatedAt(previous, later); err != nil || !got.Equal(later.Truncate(time.Millisecond)) {
		t.Fatalf("later nextUpdatedAt = %s, %v", got, err)
	}
	if _, err := nextUpdatedAt(maximumTimestamp, maximumTimestamp); !errors.Is(err, ErrTimestampExhausted) {
		t.Fatalf("maximum timestamp error = %v", err)
	}
	for _, invalid := range []time.Time{
		maximumTimestamp.Add(time.Millisecond),
		time.Date(-1, 12, 31, 23, 59, 59, 999_000_000, time.UTC),
	} {
		if _, err := nextUpdatedAt(previous, invalid); !errors.Is(err, ErrTimestampExhausted) {
			t.Fatalf("out-of-domain clock %s error = %v", invalid, err)
		}
	}
}

func TestCreateRejectsOutOfDomainClockBeforeFirestoreAccess(t *testing.T) {
	store := &FirestoreStore{
		clock: func() time.Time { return maximumTimestamp.Add(time.Millisecond) },
	}
	if _, err := store.Create(t.Context(), "principal", canonicalCreate); !errors.Is(err, ErrTimestampExhausted) {
		t.Fatalf("Create error = %v, want %v", err, ErrTimestampExhausted)
	}
}

func TestMigrationManifestRequiresMeaningfulEvidence(t *testing.T) {
	for _, evidence := range []string{"", "   ", " leading", "trailing\u00a0", string([]byte{0xff})} {
		manifest := MigrationManifest{
			Version: ProfileMigrationManifestVersion,
			Entries: map[string]MigrationAuthorization{
				"record": {TermsAccepted: true, Evidence: evidence},
			},
		}
		if err := manifest.Validate(); err == nil {
			t.Fatalf("evidence %q was accepted", evidence)
		}
	}
	manifest := MigrationManifest{Version: ProfileMigrationManifestVersion, Entries: map[string]MigrationAuthorization{
		"record": {TermsAccepted: true, Evidence: "consent-ledger/record-1"},
	}}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("valid evidence rejected: %v", err)
	}
}

func TestProfileDocumentIDAndDependencyClassification(t *testing.T) {
	const userID = "firebase/user:with/slash"
	got := profileDocumentID(userID)
	if strings.Contains(got, "/") || got == userID || got != profileDocumentID(userID) ||
		got == profileDocumentID(userID+"x") {
		t.Fatalf("profileDocumentID(%q) = %q", userID, got)
	}
	for _, err := range []error{
		context.DeadlineExceeded, status.Error(codes.Aborted, "aborted"), status.Error(codes.ResourceExhausted, "quota"), status.Error(codes.Unavailable, "unavailable"),
	} {
		if classified := classifyDependencyError(
			err,
		); !errors.Is(classified, ErrUnavailable) ||
			!errors.Is(classified, err) {
			t.Fatalf("classifyDependencyError(%v) = %v", err, classified)
		}
	}
	if classified := classifyDependencyError(
		context.Canceled,
	); !errors.Is(classified, context.Canceled) ||
		errors.Is(classified, ErrUnavailable) {
		t.Fatalf("cancellation classified %v", classified)
	}
}

func concurrentErrors(count int, operation func() error) []error {
	start := make(chan struct{})
	results := make(chan error, count)
	var wait sync.WaitGroup
	for range count {
		wait.Go(func() {
			<-start
			results <- operation()
		})
	}
	close(start)
	wait.Wait()
	close(results)
	errors := make([]error, 0, count)
	for err := range results {
		errors = append(errors, err)
	}
	return errors
}

func assertRaceOutcomes(t *testing.T, actual []error, first, second error) {
	t.Helper()
	matches := func(value, target error) bool {
		if target == nil {
			return value == nil
		}
		return errors.Is(value, target)
	}
	forward := len(actual) == 2 && matches(actual[0], first) && matches(actual[1], second)
	reverse := len(actual) == 2 && matches(actual[0], second) && matches(actual[1], first)
	if !forward && !reverse {
		t.Fatalf("race outcomes = %v, want %v and %v", actual, first, second)
	}
}
