package profile

import (
	"context"
	"errors"
	"fmt"
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
	const contenders = 10
	winners := make(chan *Profile, contenders)
	createErrors := concurrentErrors(contenders, func(index int) error {
		params := canonicalCreate
		params.FirstName = fmt.Sprintf("Writer%d", index)
		created, err := store.Create(t.Context(), "principal", params)
		if err == nil {
			winners <- created
		} else if created != nil {
			t.Errorf("failed create returned profile: %#v", created)
		}
		return err
	})
	assertSingleWinner(t, createErrors, ErrAlreadyExists)
	close(winners)
	winner := <-winners
	stored, err := store.Get(t.Context(), "principal")
	if err != nil || winner == nil || stored == nil || *stored != *winner {
		t.Fatalf("stored profile = %#v, %v; want sole winner %#v", stored, err, winner)
	}
	deleteErrors := concurrentErrors(contenders, func(int) error { return store.Delete(t.Context(), "principal") })
	assertSingleWinner(t, deleteErrors, ErrNotFound)
	if _, err := store.Get(t.Context(), "principal"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after concurrent delete = %v", err)
	}
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

func concurrentErrors(count int, operation func(int) error) []error {
	start := make(chan struct{})
	results := make(chan error, count)
	var wait sync.WaitGroup
	for index := range count {
		wait.Go(func() {
			<-start
			results <- operation(index)
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

func assertSingleWinner(t *testing.T, actual []error, conflict error) {
	t.Helper()
	successes := 0
	for _, err := range actual {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, conflict):
		case errors.Is(err, ErrUnavailable) && status.Code(err) == codes.Aborted:
			// Firestore may abort a losing concurrent write before evaluating its precondition.
		default:
			t.Errorf("unexpected concurrent write error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent writes succeeded %d times, want exactly one; errors: %v", successes, actual)
	}
}
