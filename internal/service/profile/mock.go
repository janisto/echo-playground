package profile

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MockStore implements Service with in-memory storage for testing.
type MockStore struct {
	mu       sync.RWMutex
	profiles map[string]*Profile
}

// NewMockStore creates a new in-memory profile store.
func NewMockStore() *MockStore {
	return &MockStore{profiles: make(map[string]*Profile)}
}

func (m *MockStore) Create(_ context.Context, userID string, params CreateParams) (*Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.profiles[userID]; exists {
		return nil, ErrAlreadyExists
	}

	now := time.Now().UTC()
	p := &Profile{
		ID:          userID,
		Firstname:   params.Firstname,
		Lastname:    params.Lastname,
		Email:       strings.ToLower(strings.TrimSpace(params.Email)),
		PhoneNumber: strings.TrimSpace(params.PhoneNumber),
		Marketing:   params.Marketing,
		Terms:       params.Terms,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.profiles[userID] = p

	return p, nil
}

func (m *MockStore) Get(_ context.Context, userID string) (*Profile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.profiles[userID]
	if !ok {
		return nil, ErrNotFound
	}

	return p, nil
}

func (m *MockStore) Update(_ context.Context, userID string, params UpdateParams) (*Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.profiles[userID]
	if !ok {
		return nil, ErrNotFound
	}

	if params.Firstname != nil {
		p.Firstname = *params.Firstname
	}
	if params.Lastname != nil {
		p.Lastname = *params.Lastname
	}
	if params.Email != nil {
		p.Email = strings.ToLower(strings.TrimSpace(*params.Email))
	}
	if params.PhoneNumber != nil {
		p.PhoneNumber = strings.TrimSpace(*params.PhoneNumber)
	}
	if params.Marketing != nil {
		p.Marketing = *params.Marketing
	}
	p.UpdatedAt = time.Now().UTC()

	return p, nil
}

func (m *MockStore) Delete(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.profiles[userID]; !ok {
		return ErrNotFound
	}

	delete(m.profiles, userID)

	return nil
}

var _ Service = (*MockStore)(nil)
