package hello

// CreateInput is the request body for creating a greeting.
type CreateInput struct {
	Name string `json:"name" cbor:"name" validate:"required,bounded_name" example:"World"`
}
