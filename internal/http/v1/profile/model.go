package profile

import "github.com/janisto/echo-playground/internal/platform/timeutil"

// Profile is the closed current-principal profile response.
type Profile struct {
	ID             string        `json:"id"             cbor:"id"             example:"principal-123"`
	FirstName      string        `json:"firstName"      cbor:"firstName"      example:"Ada"`
	LastName       string        `json:"lastName"       cbor:"lastName"       example:"Lovelace"`
	ContactEmail   string        `json:"contactEmail"   cbor:"contactEmail"   example:"Ada@example.com"`
	PhoneNumber    string        `json:"phoneNumber"    cbor:"phoneNumber"    example:"+358401234567"`
	MarketingOptIn bool          `json:"marketingOptIn" cbor:"marketingOptIn" example:"false"`
	TermsAccepted  bool          `json:"termsAccepted"  cbor:"termsAccepted"  example:"true"`
	CreatedAt      timeutil.Time `json:"createdAt"      cbor:"createdAt"      example:"2026-07-30T12:00:00.000Z"`
	UpdatedAt      timeutil.Time `json:"updatedAt"      cbor:"updatedAt"      example:"2026-07-30T12:00:00.000Z"`
}
