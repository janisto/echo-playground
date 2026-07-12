package hello

import (
	"fmt"
	"net/http"

	"github.com/janisto/echo-observability"
	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

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
//	@Description	Returns a hello greeting
//	@Tags			hello
//	@Produce		json,application/cbor
//	@Success		200	{object}	Data
//	@Router			/hello [get]
func getHandler(c *echo.Context) error {
	obs.Logger(c.Request().Context()).Info("hello get", zap.String("path", "/hello"))
	return respond.Negotiate(c, http.StatusOK, Data{Message: "Hello, World!"})
}

// createHandler godoc
//
//	@Summary		Create greeting
//	@Description	Creates a personalized greeting
//	@Tags			hello
//	@Produce		json,application/cbor
//	@Param			body	body		CreateInput	true	"Greeting request body"
//	@Success		201		{object}	Data
//	@Failure		400		{object}	respond.ProblemDetails
//	@Failure		422		{object}	respond.ProblemDetails
//	@Router			/hello [post]
func createHandler(c *echo.Context) error {
	var input CreateInput
	if err := c.Bind(&input); err != nil {
		return err
	}
	if err := c.Validate(&input); err != nil {
		return err
	}

	obs.Logger(c.Request().Context()).Info("hello post",
		zap.String("path", "/hello"),
		zap.String("name", input.Name))

	data := Data{Message: fmt.Sprintf("Hello, %s!", input.Name)}
	return respond.Negotiate(c, http.StatusCreated, data)
}
