package profile

// CreateInput for POST /profile.
type CreateInput struct {
	Firstname   string `json:"firstname"   validate:"required,min=1,max=100" example:"John"`
	Lastname    string `json:"lastname"    validate:"required,min=1,max=100" example:"Doe"`
	Email       string `json:"email"       validate:"required,email"         example:"john@example.com"`
	PhoneNumber string `json:"phoneNumber" validate:"required,e164"          example:"+358401234567"`
	Marketing   bool   `json:"marketing"                                     example:"true"`
	Terms       bool   `json:"terms"                                         example:"true"`
}

// UpdateInput for PATCH /profile.
type UpdateInput struct {
	Firstname   *string `json:"firstname,omitempty"   validate:"omitempty,min=1,max=100" example:"John"`
	Lastname    *string `json:"lastname,omitempty"    validate:"omitempty,min=1,max=100" example:"Doe"`
	Email       *string `json:"email,omitempty"       validate:"omitempty,email"         example:"john@example.com"`
	PhoneNumber *string `json:"phoneNumber,omitempty" validate:"omitempty,e164"          example:"+358401234567"`
	Marketing   *bool   `json:"marketing,omitempty"                                      example:"true"`
}
