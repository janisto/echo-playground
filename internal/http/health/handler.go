package health

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/request"
	"github.com/janisto/echo-playground/internal/platform/respond"
)

// Response is the payload for the health endpoint.
type Response struct {
	Status string `json:"status" cbor:"status" example:"healthy"`
}

// Handler is the health check endpoint.
//
//	@Summary		Get application health
//	@ID				getHealth
//	@Description	Returns the dependency-free application health signal. The query string is closed.
//	@Tags			health
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Success		200				{object}	Response
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Router			/health [get]
func Handler(c *echo.Context) error {
	if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
		return err
	}
	return respond.Negotiate(c, http.StatusOK, Response{Status: "healthy"})
}
