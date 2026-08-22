package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

// MethodNotAllowed reflects Echo's automatic HEAD support in the Allow field.
func MethodNotAllowed(c *echo.Context) error {
	setAllow(c)
	return echo.ErrMethodNotAllowed
}

// Options reflects Echo's automatic HEAD support in the Allow field.
func Options(c *echo.Context) error {
	setAllow(c)
	return c.NoContent(http.StatusNoContent)
}

func setAllow(c *echo.Context) {
	allowed, ok := c.Get(echo.ContextKeyHeaderAllow).(string)
	if !ok || allowed == "" {
		return
	}
	methods := strings.Split(allowed, ",")
	result := make([]string, 0, len(methods)+1)
	hasHead := false
	for _, method := range methods {
		method = strings.TrimSpace(method)
		if method == http.MethodHead {
			hasHead = true
		}
		result = append(result, method)
	}
	if !hasHead {
		for index, method := range result {
			if method == http.MethodGet {
				result = append(result, "")
				copy(result[index+2:], result[index+1:])
				result[index+1] = http.MethodHead
				break
			}
		}
	}
	c.Response().Header().Set(echo.HeaderAllow, strings.Join(result, ", "))
}
