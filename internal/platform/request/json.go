// Package request contains strict HTTP request decoding helpers.
package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"

	"github.com/labstack/echo/v5"
)

// DecodeJSON decodes exactly one application/json object and rejects unknown fields.
func DecodeJSON(c *echo.Context, target any) error {
	req := c.Request()
	if req.ContentLength == 0 {
		return echo.ErrBadRequest
	}

	mediaType, _, err := mime.ParseMediaType(req.Header.Get(echo.HeaderContentType))
	if err != nil || mediaType != echo.MIMEApplicationJSON {
		return echo.ErrUnsupportedMediaType
	}

	decoder := json.NewDecoder(req.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return echo.ErrBadRequest.Wrap(err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return echo.ErrBadRequest.Wrap(err)
	}

	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return echo.ErrBadRequest
	}

	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return echo.ErrBadRequest.Wrap(err)
	}
	return nil
}
