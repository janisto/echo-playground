package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/janisto/echo-observability/v2"
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
	"github.com/janisto/echo-playground/internal/platform/request"
	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/validate"
	githubsvc "github.com/janisto/echo-playground/internal/service/github"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

type applicationClients struct {
	*firebase.Clients
	verifier auth.Verifier
	profiles profilesvc.Service
	github   githubsvc.Service
}

const observabilityTraceContextLevel = obs.TraceContextLevel1

func newFirebaseClients(ctx context.Context, cfg config, logger *zap.Logger) (*applicationClients, error) {
	if cfg.FirebaseMode == firebaseModeOffline {
		logger.Warn("Firebase is offline; protected routes return service unavailable")
		return &applicationClients{
			verifier: unavailableVerifier{},
			profiles: unavailableProfileStore{},
			github:   githubsvc.NewClient(),
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
		github:   githubsvc.NewClient(),
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
	e := echo.NewWithConfig(echo.Config{
		Router: echo.NewRouter(echo.RouterConfig{
			AllowOverwritingRoute:   false,
			AutoHandleHEAD:          true,
			MethodNotAllowedHandler: appmiddleware.MethodNotAllowed,
			OptionsMethodHandler:    appmiddleware.Options,
		}),
		HTTPErrorHandler:             respond.NewHTTPErrorHandler(),
		IPExtractor:                  echo.ExtractIPDirect(),
		Validator:                    validate.New(),
		NoGroupAutoRegister404Routes: true,
	})

	e.Use(
		obs.RequestContext(obs.RequestContextConfig{
			Logger:            logger,
			Preset:            obs.PresetGCP,
			TraceContextLevel: observabilityTraceContextLevel,
			ValidateRequestID: validatePortableRequestID,
		}),
		respond.Recoverer(logger),
		obs.AccessLogger(obs.AccessLoggerConfig{
			Logger:            logger,
			Preset:            obs.PresetGCP,
			TraceContextLevel: observabilityTraceContextLevel,
		}),
		middleware.ContextTimeoutWithConfig(middleware.ContextTimeoutConfig{
			Timeout: cfg.RequestTimeout,
			ErrorHandler: func(_ *echo.Context, err error) error {
				if errors.Is(err, context.DeadlineExceeded) {
					return respond.DependencyUnavailable()
				}
				return err
			},
		}),
		appmiddleware.Security(),
		appmiddleware.Vary(),
		appmiddleware.CORS(cfg.CORSOrigins),
		request.BodyLimitMiddleware(),
	)

	e.GET("/health", health.Handler, respond.SuccessNegotiation(false))
	docs.Register(e, apidocs.OpenAPIJSON)
	githubService := clients.github
	if githubService == nil {
		githubService = githubsvc.NewClient()
	}
	routes.Register(e.Group("/v1"), clients.verifier, clients.profiles, githubService)
	return e
}

func validatePortableRequestID(value string) bool {
	if len(value) < 1 || len(value) > 128 || !isASCIIAlphaNumeric(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !isASCIIAlphaNumeric(character) && character != '.' && character != '_' &&
			character != ':' && character != '-' {
			return false
		}
	}
	return true
}

func isASCIIAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func newServer(cfg config, handler *echo.Echo, logger *zap.Logger) (*http.Server, error) {
	errorLog, err := zap.NewStdLogAt(logger.Named("http.server"), zap.ErrorLevel)
	if err != nil {
		return nil, fmt.Errorf("create HTTP server logger: %w", err)
	}
	return &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ErrorLog:          errorLog,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    64 << 10,
	}, nil
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
