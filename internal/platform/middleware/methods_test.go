package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/janisto/echo-playground/internal/platform/respond"
)

func TestAutomaticHeadIsReflectedInAllow(t *testing.T) {
	e := echo.NewWithConfig(echo.Config{
		Router: echo.NewRouter(echo.RouterConfig{
			AutoHandleHEAD:          true,
			MethodNotAllowedHandler: MethodNotAllowed,
			OptionsMethodHandler:    Options,
		}),
		HTTPErrorHandler: respond.NewHTTPErrorHandler(),
	})
	e.GET("/resource", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	for _, method := range []string{http.MethodPost, http.MethodOptions} {
		recorder := httptest.NewRecorder()
		e.ServeHTTP(
			recorder,
			httptest.NewRequestWithContext(t.Context(), method, "/resource", strings.NewReader("unread")),
		)
		if recorder.Header().Get("Allow") != "OPTIONS, GET, HEAD" {
			t.Fatalf("%s Allow = %q", method, recorder.Header().Get("Allow"))
		}
	}
}
