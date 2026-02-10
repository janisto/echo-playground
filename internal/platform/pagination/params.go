package pagination

// DefaultLimit is the default number of items per page.
const DefaultLimit = 20

// MaxLimit is the maximum number of items per page.
const MaxLimit = 100

// Params provides a helper for pagination defaults.
type Params struct {
	Cursor string
	Limit  int
}

// DefaultLimit returns the limit, defaulting to 20 if zero or negative.
func (p Params) DefaultLimit() int {
	if p.Limit <= 0 {
		return DefaultLimit
	}
	return p.Limit
}
