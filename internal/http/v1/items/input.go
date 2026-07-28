package items

// ListInput defines query parameters for listing items.
type ListInput struct {
	Cursor   string `query:"cursor"   validate:"omitempty,max=2048"`
	Limit    *int   `query:"limit"    validate:"omitempty,min=1,max=100"`
	Category string `query:"category" validate:"omitempty,oneof=electronics tools accessories robotics power components"`
}
