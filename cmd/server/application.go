package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

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
	if cfg.FirebaseMode == firebaseModeOffline {
		logger.Warn("Firebase is offline; protected routes return service unavailable")
		return &applicationClients{
			verifier: unavailableVerifier{},
			profiles: unavailableProfileStore{},
		}, nil
	}
	if cfg.FirebaseMode == firebaseModeEmulator {
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

type unavailableProfileStore struct{}

func (unavailableProfileStore) Create(context.Context, string, profilesvc.CreateParams) (*profilesvc.Profile, error) {
	return nil, profilesvc.ErrUnavailable
}

func (unavailableProfileStore) Get(context.Context, string) (*profilesvc.Profile, error) {
	return nil, profilesvc.ErrUnavailable
}

func (unavailableProfileStore) Update(context.Context, string, profilesvc.UpdateParams) (*profilesvc.Profile, error) {
	return nil, profilesvc.ErrUnavailable
}

func (unavailableProfileStore) Delete(context.Context, string) error {
	return profilesvc.ErrUnavailable
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

func newServer(cfg config, handler *echo.Echo) *http.Server {
	return &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ErrorLog:          slog.NewLogLogger(handler.Logger.Handler(), slog.LevelError),
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    64 << 10,
	}
}

func serve(ctx context.Context, server *http.Server, shutdownTimeout time.Duration, logger *zap.Logger) error {
	if ctx.Err() != nil {
		return nil
	}
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}
	logger.Info("server listening", zap.String("addr", listener.Addr().String()), zap.String("version", Version))

	listenErr := make(chan error, 1)
	go func() {
		listenErr <- server.Serve(listener)
	}()

	select {
	case err := <-listenErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve on %s: %w", server.Addr, err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return errors.Join(fmt.Errorf("graceful shutdown: %w", err), server.Close())
	}
	if err := <-listenErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server stopped: %w", err)
	}
	return nil
}
