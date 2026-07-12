package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/janisto/echo-observability"
	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"

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

//	@title						Echo Playground API
//	@version					1.0
//	@description				Example API built with Echo v5
//	@servers.url				http://localhost:8080/v1
//	@servers.description		Local development server
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization

// Version can be overridden at build time: -ldflags "-X main.Version=1.2.3"
var Version = "dev"

func main() {
	logger, err := obs.NewLogger(obs.LoggerConfig{
		Preset:    obs.PresetGCP,
		AddCaller: true,
	})
	if err != nil {
		panic(err)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil && !errors.Is(syncErr, syscall.ENOTTY) {
			logger.Error("logger sync error", zap.Error(syncErr))
		}
	}()

	ctx := context.Background()

	firebaseProjectID := os.Getenv("FIREBASE_PROJECT_ID")
	if firebaseProjectID == "" {
		if os.Getenv("APP_ENVIRONMENT") == "development" {
			firebaseProjectID = "demo-test-project"
			logger.Warn("using demo-test-project for local development")
		} else {
			logger.Fatal("FIREBASE_PROJECT_ID environment variable is required")
		}
	}

	firebaseClients, err := firebase.InitializeClients(ctx, firebase.Config{
		ProjectID: firebaseProjectID,
	})
	if err != nil {
		logger.Fatal("firebase init failed", zap.Error(err))
	}
	defer func() {
		if closeErr := firebaseClients.Close(); closeErr != nil {
			logger.Error("firebase close error", zap.Error(closeErr))
		}
	}()

	verifier := auth.NewFirebaseVerifier(firebaseClients.Auth)
	profileService := profilesvc.NewFirestoreStore(firebaseClients.Firestore)

	e := echo.New()
	e.Validator = validate.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.IPExtractor = echo.ExtractIPFromRealIPHeader()

	e.Use(
		obs.RequestContext(obs.RequestContextConfig{
			Logger: logger,
			Preset: obs.PresetGCP,
		}),
		obs.AccessLogger(obs.AccessLoggerConfig{
			Logger: logger,
			Preset: obs.PresetGCP,
		}),
		appmiddleware.Security("/api-docs"),
		appmiddleware.Vary(),
		appmiddleware.CORS(),
		middleware.BodyLimit(1<<20),
		respond.Recoverer(logger),
	)

	e.GET("/health", health.Handler)
	docs.Register(e, "api-docs/swagger.json")

	v1 := e.Group("/v1")
	routes.Register(v1, verifier, profileService)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("server starting",
		zap.String("addr", ":"+port),
		zap.String("version", Version))

	sc := echo.StartConfig{
		Address:         ":" + port,
		GracefulTimeout: 10 * time.Second,
		BeforeServeFunc: func(s *http.Server) error {
			s.ReadTimeout = 5 * time.Second
			s.ReadHeaderTimeout = 2 * time.Second
			s.WriteTimeout = 10 * time.Second
			s.IdleTimeout = 60 * time.Second
			s.MaxHeaderBytes = 64 << 10
			return nil
		},
	}

	sigCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := sc.Start(sigCtx, e); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}

	if cause := context.Cause(sigCtx); cause != nil {
		logger.Info("server exited", zap.Error(cause))
	} else {
		logger.Info("server exited")
	}
}
