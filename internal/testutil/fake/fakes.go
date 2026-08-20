// Package fake provides reusable test doubles for application boundaries.
package fake

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/janisto/echo-playground/internal/platform/auth"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

// MockVerifier returns a configured authentication result.
type MockVerifier struct {
	User  *auth.FirebaseUser
	Error error
	calls atomic.Int32
}

func (m *MockVerifier) Verify(context.Context, string) (*auth.FirebaseUser, error) {
	m.calls.Add(1)
	if m.Error != nil {
		return nil, m.Error
	}
	return m.User, nil
}

// CallCount returns the number of verification attempts.
func (m *MockVerifier) CallCount() int32 { return m.calls.Load() }

// TestUser returns a stable authenticated identity for tests.
func TestUser() *auth.FirebaseUser {
	return &auth.FirebaseUser{UID: "test-user-123"}
}

// ProfileStore is an in-memory profile service for tests.
type ProfileStore struct {
	mu       sync.RWMutex
	profiles map[string]*profilesvc.Profile
	clock    func() time.Time
	writes   int
}

// NewProfileStore creates an empty test profile store.
func NewProfileStore() *ProfileStore {
	return &ProfileStore{
		profiles: make(map[string]*profilesvc.Profile),
		clock: func() time.Time {
			return time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
		},
	}
}

func (m *ProfileStore) Create(
	_ context.Context,
	userID string,
	params profilesvc.CreateParams,
) (*profilesvc.Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.profiles[userID]; exists {
		return nil, profilesvc.ErrAlreadyExists
	}
	now := m.clock().UTC().Truncate(time.Millisecond)
	p := &profilesvc.Profile{
		ID: userID, FirstName: params.FirstName, LastName: params.LastName, ContactEmail: params.ContactEmail,
		PhoneNumber: params.PhoneNumber, MarketingOptIn: params.MarketingOptIn, TermsAccepted: params.TermsAccepted,
		CreatedAt: now, UpdatedAt: now,
	}
	m.profiles[userID] = p
	m.writes++
	return cloneProfile(p), nil
}

func (m *ProfileStore) Get(_ context.Context, userID string) (*profilesvc.Profile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.profiles[userID]
	if !ok {
		return nil, profilesvc.ErrNotFound
	}
	return cloneProfile(p), nil
}

func (m *ProfileStore) Update(
	_ context.Context,
	userID string,
	params profilesvc.UpdateParams,
) (*profilesvc.Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.profiles[userID]
	if !ok {
		return nil, profilesvc.ErrNotFound
	}
	changed := false
	if params.FirstName != nil && p.FirstName != *params.FirstName {
		p.FirstName, changed = *params.FirstName, true
	}
	if params.LastName != nil && p.LastName != *params.LastName {
		p.LastName, changed = *params.LastName, true
	}
	if params.ContactEmail != nil && p.ContactEmail != *params.ContactEmail {
		p.ContactEmail, changed = *params.ContactEmail, true
	}
	if params.PhoneNumber != nil && p.PhoneNumber != *params.PhoneNumber {
		p.PhoneNumber, changed = *params.PhoneNumber, true
	}
	if params.MarketingOptIn != nil && p.MarketingOptIn != *params.MarketingOptIn {
		p.MarketingOptIn, changed = *params.MarketingOptIn, true
	}
	if changed {
		now := m.clock().UTC().Truncate(time.Millisecond)
		if !now.After(p.UpdatedAt) {
			now = p.UpdatedAt.Add(time.Millisecond)
		}
		p.UpdatedAt = now
		m.writes++
	}
	return cloneProfile(p), nil
}

func (m *ProfileStore) Delete(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.profiles[userID]; !ok {
		return profilesvc.ErrNotFound
	}
	delete(m.profiles, userID)
	m.writes++
	return nil
}

// WriteCount returns committed mutations for forbidden-side-effect assertions.
func (m *ProfileStore) WriteCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.writes
}

func cloneProfile(p *profilesvc.Profile) *profilesvc.Profile {
	result := *p
	return &result
}

var (
	_ auth.Verifier      = (*MockVerifier)(nil)
	_ profilesvc.Service = (*ProfileStore)(nil)
)
