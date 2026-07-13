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
	g.GET("/hello", getHandler)
	g.POST("/hello", createHandler)
}

// getHandler godoc
//
//	@Summary		Greeting endpoint
//	@ID				getHello
//	@Description	Returns a hello greeting
//	@Tags			hello
//	@Produce		json,application/cbor
//	@Success		200	{object}	Data
//	@Router			/hello [get]
func getHandler(c *echo.Context) error {
	return respond.Negotiate(c, http.StatusOK, Data{Message: "Hello, World!"})
}

// createHandler godoc
//
//	@Summary		Create greeting
//	@ID				createHello
//	@Description	Creates a personalized greeting
//	@Tags			hello
//	@Produce		json,application/cbor
//	@Param			body	body		CreateInput	true	"Greeting request body"
//	@Success		200		{object}	Data
//	@Failure		400		{object}	respond.ProblemDetails
//	@Failure		413		{object}	respond.ProblemDetails
//	@Failure		415		{object}	respond.ProblemDetails
//	@Failure		422		{object}	respond.ProblemDetails
//	@Router			/hello [post]
func createHandler(c *echo.Context) error {
	var input CreateInput
	if err := request.DecodeJSON(c, &input); err != nil {
		return err
	}
	if err := c.Validate(&input); err != nil {
		return err
	}

	data := Data{Message: fmt.Sprintf("Hello, %s!", input.Name)}
	return respond.Negotiate(c, http.StatusOK, data)
}
