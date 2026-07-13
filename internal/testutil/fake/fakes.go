// Package fake provides reusable test doubles for application boundaries.
package fake

import (
	"context"
	"sync"
	"time"

	"github.com/janisto/echo-playground/internal/platform/auth"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

// MockVerifier returns a configured authentication result.
type MockVerifier struct {
	User  *auth.FirebaseUser
	Error error
}

func (m *MockVerifier) Verify(context.Context, string) (*auth.FirebaseUser, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.User, nil
}

// TestUser returns a stable authenticated identity for tests.
func TestUser() *auth.FirebaseUser {
	return &auth.FirebaseUser{UID: "test-user-123"}
}

// ProfileStore is an in-memory profile service for tests.
type ProfileStore struct {
	mu       sync.RWMutex
	profiles map[string]*profilesvc.Profile
}

// NewProfileStore creates an empty test profile store.
func NewProfileStore() *ProfileStore {
	return &ProfileStore{profiles: make(map[string]*profilesvc.Profile)}
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
	now := time.Now().UTC()
	p := &profilesvc.Profile{
		ID: userID, Firstname: params.Firstname, Lastname: params.Lastname, Email: params.Email,
		PhoneNumber: params.PhoneNumber, Marketing: params.Marketing, CreatedAt: now, UpdatedAt: now,
	}
	m.profiles[userID] = p
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
	if params.Firstname != nil {
		p.Firstname = *params.Firstname
	}
	if params.Lastname != nil {
		p.Lastname = *params.Lastname
	}
	if params.Email != nil {
		p.Email = *params.Email
	}
	if params.PhoneNumber != nil {
		p.PhoneNumber = *params.PhoneNumber
	}
	if params.Marketing != nil {
		p.Marketing = *params.Marketing
	}
	p.UpdatedAt = time.Now().UTC()
	return cloneProfile(p), nil
}

func (m *ProfileStore) Delete(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.profiles[userID]; !ok {
		return profilesvc.ErrNotFound
	}
	delete(m.profiles, userID)
	return nil
}

func cloneProfile(p *profilesvc.Profile) *profilesvc.Profile {
	result := *p
	return &result
}

var (
	_ auth.Verifier      = (*MockVerifier)(nil)
	_ profilesvc.Service = (*ProfileStore)(nil)
)
