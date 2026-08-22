package profile

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("profile not found")
	ErrAlreadyExists      = errors.New("profile already exists")
	ErrUnavailable        = errors.New("profile store unavailable")
	ErrInvalidStoredData  = errors.New("stored profile is invalid")
	ErrTimestampExhausted = errors.New("profile timestamp cannot advance")
)

type Profile struct {
	ID             string
	FirstName      string
	LastName       string
	ContactEmail   string
	PhoneNumber    string
	MarketingOptIn bool
	TermsAccepted  bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateParams struct {
	FirstName      string
	LastName       string
	ContactEmail   string
	PhoneNumber    string
	MarketingOptIn bool
	TermsAccepted  bool
}

type UpdateParams struct {
	FirstName      *string
	LastName       *string
	ContactEmail   *string
	PhoneNumber    *string
	MarketingOptIn *bool
}

type Service interface {
	Create(context.Context, string, CreateParams) (*Profile, error)
	Get(context.Context, string) (*Profile, error)
	Update(context.Context, string, UpdateParams) (*Profile, error)
	Delete(context.Context, string) error
}
