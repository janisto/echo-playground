package profile

// CreateInput for POST /profile.
type CreateInput struct {
	Firstname   string `json:"firstname"    validate:"required,min=1,max=100"`
	Lastname    string `json:"lastname"     validate:"required,min=1,max=100"`
	Email       string `json:"email"        validate:"required,email"`
	PhoneNumber string `json:"phone_number" validate:"required,e164"`
	Marketing   bool   `json:"marketing"`
	Terms       bool   `json:"terms"        validate:"required"`
}

// UpdateInput for PATCH /profile.
type UpdateInput struct {
	Firstname   *string `json:"firstname,omitempty"    validate:"omitempty,min=1,max=100"`
	Lastname    *string `json:"lastname,omitempty"     validate:"omitempty,min=1,max=100"`
	Email       *string `json:"email,omitempty"        validate:"omitempty,email"`
	PhoneNumber *string `json:"phone_number,omitempty" validate:"omitempty,e164"`
	Marketing   *bool   `json:"marketing,omitempty"`
}
