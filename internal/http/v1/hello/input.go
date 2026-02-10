package hello

// CreateInput is the request body for creating a greeting.
type CreateInput struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}
