package hello

// Data models the response payload for hello endpoints.
type Data struct {
	Message string `json:"message" example:"Hello, World!"`
}
