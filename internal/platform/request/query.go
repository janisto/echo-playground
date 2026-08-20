package request

import (
	"net/url"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/respond"
)

// ParseQuery enforces a closed scalar query contract and returns decoded values.
func ParseQuery(c *echo.Context, allowed ...string) (url.Values, error) {
	query, err := url.ParseQuery(c.Request().URL.RawQuery)
	if err != nil {
		return nil, respond.InvalidRequest()
	}
	for name, values := range query {
		if !utf8.ValidString(name) || !slices.Contains(allowed, name) {
			return nil, respond.InvalidRequest()
		}
		if len(values) != 1 {
			return nil, respond.InvalidRequest()
		}
		if !utf8.ValidString(values[0]) {
			return nil, respond.InvalidRequest()
		}
	}
	return query, nil
}

// RejectUnknownOrRepeatedQuery enforces a closed scalar query contract.
func RejectUnknownOrRepeatedQuery(c *echo.Context, allowed ...string) error {
	_, err := ParseQuery(c, allowed...)
	return err
}

// Limit parses the portable unsigned decimal limit grammar.
func Limit(query url.Values) (int, error) {
	const defaultLimit = 20
	values, present := query["limit"]
	if !present {
		return defaultLimit, nil
	}
	value := values[0]
	if value == "" {
		return 0, limitValidationError()
	}
	for i := range len(value) {
		if value[i] < '0' || value[i] > '9' {
			return 0, limitValidationError()
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed < 1 || parsed > 100 {
		return 0, limitValidationError()
	}
	return int(parsed), nil
}

func limitValidationError() error {
	return respond.ValidationFailed(respond.ErrorDetail{
		Detail: "limit must be an integer from 1 through 100",
		Source: &respond.ErrorSource{Parameter: "limit"},
	})
}
