package profile

import "github.com/janisto/echo-playground/internal/platform/timeutil"

// Profile represents a user profile response.
type Profile struct {
	ID          string        `json:"id"           example:"user-123"`
	Firstname   string        `json:"firstname"    example:"John"`
	Lastname    string        `json:"lastname"     example:"Doe"`
	Email       string        `json:"email"        example:"john@example.com"`
	PhoneNumber string        `json:"phone_number" example:"+358401234567"`
	Marketing   bool          `json:"marketing"    example:"true"`
	Terms       bool          `json:"terms"        example:"true"`
	CreatedAt   timeutil.Time `json:"created_at"   example:"2024-01-15T10:30:00.000Z"`
	UpdatedAt   timeutil.Time `json:"updated_at"   example:"2024-01-15T10:30:00.000Z"`
}
