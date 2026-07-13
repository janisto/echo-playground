package request

import (
	"slices"

	"github.com/labstack/echo/v5"
)

// RejectUnknownQuery rejects query parameter names outside the endpoint contract.
func RejectUnknownQuery(c *echo.Context, allowed ...string) error {
	for name := range c.QueryParams() {
		if !slices.Contains(allowed, name) {
			return echo.ErrBadRequest
		}
	}
	return nil
}
