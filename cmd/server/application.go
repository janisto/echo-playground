package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/janisto/echo-observability"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"

	"github.com/janisto/echo-playground/api-docs"
	"github.com/janisto/echo-playground/internal/http/docs"
	"github.com/janisto/echo-playground/internal/http/health"
	"github.com/janisto/echo-playground/internal/http/v1/routes"
	"github.com/janisto/echo-playground/internal/platform/auth"
	"github.com/janisto/echo-playground/internal/platform/firebase"
	appmiddleware "github.com/janisto/echo-playground/internal/platform/middleware"
	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/validate"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

type applicationClients struct {
	*firebase.Clients
	verifier auth.Verifier
	profiles profilesvc.Service
}

func newFirebaseClients(ctx context.Context, cfg config, logger *zap.Logger) (*applicationClients, error) {
	if cfg.Environment == environmentDevelopment &&
		cfg.FirebaseProject == "demo-test-project" &&
		!cfg.FirebaseEmulators {
		logger.Warn("Firebase emulators are not configured; protected routes are unavailable")
		return &applicationClients{
			verifier: unavailableVerifier{},
			profiles: profilesvc.NewMockStore(),
		}, nil
	}
	if cfg.FirebaseEmulators {
		logger.Info("using Firebase emulators", zap.String("project_id", cfg.FirebaseProject))
	}
	clients, err := firebase.InitializeClients(ctx, firebase.Config{ProjectID: cfg.FirebaseProject})
	if err != nil {
		return nil, fmt.Errorf("initialize Firebase clients: %w", err)
	}
	return &applicationClients{
		Clients:  clients,
		verifier: auth.NewFirebaseVerifier(clients.Auth),
		profiles: profilesvc.NewFirestoreStore(clients.Firestore),
	}, nil
}

func (c *applicationClients) Close() error {
	if c.Clients == nil {
		return nil
	}
	return c.Clients.Close()
}

type unavailableVerifier struct{}

func (unavailableVerifier) Verify(context.Context, string) (*auth.FirebaseUser, error) {
	return nil, auth.ErrAuthUnavailable
}

func newEcho(cfg config, logger *zap.Logger, clients *applicationClients) *echo.Echo {
	e := echo.New()
	e.Validator = validate.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.IPExtractor = echo.ExtractIPDirect()
	if cfg.IPExtractor == "xff" {
		e.IPExtractor = echo.ExtractIPFromXFFHeader()
	}

	e.Use(
		obs.RequestContext(obs.RequestContextConfig{Logger: logger, Preset: obs.PresetGCP}),
		obs.AccessLogger(obs.AccessLoggerConfig{Logger: logger, Preset: obs.PresetGCP}),
		respond.Recoverer(logger),
		middleware.ContextTimeoutWithConfig(middleware.ContextTimeoutConfig{
			Timeout: cfg.RequestTimeout,
			ErrorHandler: func(_ *echo.Context, err error) error {
				if errors.Is(err, context.DeadlineExceeded) {
					return respond.Error503("request deadline exceeded")
				}
				return err
			},
		}),
		appmiddleware.Security(),
		appmiddleware.Vary(),
		appmiddleware.CORS(),
		middleware.BodyLimit(1<<20),
	)

	e.GET("/health", health.Handler)
	docs.Register(e, apidocs.OpenAPIJSON)
	routes.Register(e.Group("/v1"), clients.verifier, clients.profiles)
	return e
}

func newStartConfig(cfg config) echo.StartConfig {
	return echo.StartConfig{
		Address:         cfg.Address,
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: cfg.ShutdownTimeout,
		BeforeServeFunc: func(s *http.Server) error {
			s.ReadTimeout = cfg.ReadTimeout
			s.ReadHeaderTimeout = cfg.ReadHeaderTimeout
			s.WriteTimeout = cfg.WriteTimeout
			s.IdleTimeout = cfg.IdleTimeout
			s.MaxHeaderBytes = 64 << 10
			return nil
		},
	}
}
