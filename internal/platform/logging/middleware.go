package logging

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v5"
)

// RequestLogger returns Echo middleware that enriches the request context
// with an slog logger containing Cloud Trace metadata and request attributes.
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			header := c.Request().Header.Get(traceparentHeader)
			projectID := resolveProjectID()

			reqID, _ := c.Get("request_id").(string)

			traceID := traceResource(header, projectID)
			if traceID == "" && reqID != "" {
				traceID = reqID
			}

			logger := loggerWithTrace(Logger(), header, projectID, reqID)

			ctx := c.Request().Context()
			ctx = contextWithTraceID(ctx, traceID)
			ctx = contextWithLogger(ctx, logger)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

// AccessLogger returns Echo middleware that logs structured request summaries
// after each request completes.
func AccessLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()

			err := next(c)

			resp, unwrapErr := echo.UnwrapResponse(c.Response())
			status := 0
			size := 0
			if unwrapErr == nil {
				status = resp.Status
				size = int(resp.Size)
			}

			logger := LoggerFromContext(c.Request().Context())
			logger.LogAttrs(c.Request().Context(), slog.LevelInfo, "request completed",
				slog.String("method", c.Request().Method),
				slog.String("path", c.Request().URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", size),
				slog.Duration("duration", time.Since(start)),
			)

			return err
		}
	}
}
