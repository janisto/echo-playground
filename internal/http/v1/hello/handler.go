package hello

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/request"
	"github.com/janisto/echo-playground/internal/platform/respond"
)

// Register wires hello routes into the provided group.
func Register(g *echo.Group) {
	g.GET("/hello", getHandler, respond.SuccessNegotiation(false))
	g.POST("/hello", createHandler, respond.SuccessNegotiation(false))
}

// getHandler returns the fixed greeting.
//
//	@Summary		Get the default greeting
//	@ID				getHello
//	@Description	Returns the fixed greeting. The query string is closed.
//	@Tags			hello
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string	false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Success		200				{object}	Data
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Router			/v1/hello [get]
func getHandler(c *echo.Context) error {
	if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
		return err
	}
	return respond.Negotiate(c, http.StatusOK, Data{Message: "Hello, World!"})
}

// createHandler returns a personalized greeting.
//
//	@Summary		Create a personalized greeting
//	@ID				createHello
//	@Description	Creates a greeting from one strict request document. The query string is closed.
//	@Tags			hello
//	@Accept			json,application/cbor
//	@Produce		json,application/cbor
//	@Param			X-Request-ID	header		string		false	"Optional request correlation value"	minlength(1)	maxlength(128)
//	@Param			body			body		CreateInput	true	"Greeting request document"
//	@Success		200				{object}	Data
//	@Failure		400				{object}	respond.ProblemDetails
//	@Failure		406				{object}	respond.ProblemDetails
//	@Failure		413				{object}	respond.ProblemDetails
//	@Failure		415				{object}	respond.ProblemDetails
//	@Failure		422				{object}	respond.ProblemDetails
//	@Failure		500				{object}	respond.ProblemDetails
//	@Router			/v1/hello [post]
func createHandler(c *echo.Context) error {
	if err := request.RejectUnknownOrRepeatedQuery(c); err != nil {
		return err
	}
	var input CreateInput
	if err := request.Decode(c, &input); err != nil {
		return err
	}
	if err := c.Validate(&input); err != nil {
		return err
	}

	data := Data{Message: fmt.Sprintf("Hello, %s!", input.Name)}
	return respond.Negotiate(c, http.StatusOK, data)
}
