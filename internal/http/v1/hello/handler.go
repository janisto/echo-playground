package hello

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"

	applog "github.com/janisto/echo-playground/internal/platform/logging"
	"github.com/janisto/echo-playground/internal/platform/respond"
)

// Register wires hello routes into the provided group.
func Register(g *echo.Group) {
	g.GET("/hello", getHandler)
	g.POST("/hello", createHandler)
}

func getHandler(c *echo.Context) error {
	applog.LogInfo(c.Request().Context(), "hello get", slog.String("path", "/hello"))
	return respond.Negotiate(c, http.StatusOK, Data{Message: "Hello, World!"})
}

func createHandler(c *echo.Context) error {
	var input CreateInput
	if err := c.Bind(&input); err != nil {
		return err
	}
	if err := c.Validate(&input); err != nil {
		return err
	}

	applog.LogInfo(c.Request().Context(), "hello post",
		slog.String("path", "/hello"),
		slog.String("name", input.Name))

	data := Data{Message: fmt.Sprintf("Hello, %s!", input.Name)}
	return respond.Negotiate(c, http.StatusCreated, data)
}
