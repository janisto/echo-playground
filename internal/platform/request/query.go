package request

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/labstack/echo/v5"
)

// RejectUnknownOrRepeatedQuery enforces a closed scalar query contract.
func RejectUnknownOrRepeatedQuery(c *echo.Context, allowed ...string) error {
	for name, values := range c.QueryParams() {
		if !slices.Contains(allowed, name) {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("unknown query parameter %q", name))
		}
		if len(values) != 1 {
			return echo.NewHTTPError(
				http.StatusBadRequest,
				fmt.Sprintf("query parameter %q must appear exactly once", name),
			)
		}
	}
	return nil
}
