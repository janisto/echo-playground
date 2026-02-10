package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/janisto/echo-playground/internal/http/health"
	"github.com/janisto/echo-playground/internal/http/v1/routes"
	"github.com/janisto/echo-playground/internal/platform/auth"
	"github.com/janisto/echo-playground/internal/platform/firebase"
	applog "github.com/janisto/echo-playground/internal/platform/logging"
	appmiddleware "github.com/janisto/echo-playground/internal/platform/middleware"
	"github.com/janisto/echo-playground/internal/platform/respond"
	"github.com/janisto/echo-playground/internal/platform/validate"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

// Version can be overridden at build time: -ldflags "-X main.Version=1.2.3"
var Version = "dev"

func main() {
	ctx := context.Background()

	firebaseProjectID := os.Getenv("FIREBASE_PROJECT_ID")
	if firebaseProjectID == "" {
		if os.Getenv("APP_ENVIRONMENT") == "development" {
			firebaseProjectID = "demo-test-project"
			applog.LogWarn(ctx, "using demo-test-project for local development")
		} else {
			applog.LogFatal(ctx, "FIREBASE_PROJECT_ID environment variable is required", nil)
		}
	}

	firebaseClients, err := firebase.InitializeClients(ctx, firebase.Config{
		ProjectID: firebaseProjectID,
	})
	if err != nil {
		applog.LogFatal(ctx, "firebase init failed", err)
	}
	defer func() {
		if closeErr := firebaseClients.Close(); closeErr != nil {
			applog.LogError(ctx, "firebase close error", closeErr)
		}
	}()

	verifier := auth.NewFirebaseVerifier(firebaseClients.Auth)
	profileService := profilesvc.NewFirestoreStore(firebaseClients.Firestore)

	e := echo.New()
	e.Validator = validate.New()
	e.HTTPErrorHandler = respond.NewHTTPErrorHandler()
	e.IPExtractor = echo.ExtractIPFromRealIPHeader()
	e.Logger = applog.Logger()

	e.Use(
		appmiddleware.Security("/v1/api-docs"),
		appmiddleware.Vary(),
		appmiddleware.CORS(),
		appmiddleware.RequestID(),
		middleware.BodyLimit(1<<20),
		applog.RequestLogger(),
		applog.AccessLogger(),
		respond.Recoverer(),
	)

	e.GET("/health", health.Handler)

	v1 := e.Group("/v1")
	routes.Register(v1, verifier, profileService)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	applog.LogInfo(ctx, "server starting",
		slog.String("addr", ":"+port),
		slog.String("version", Version))

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
		log.Fatal(err)
	}

	applog.LogInfo(ctx, "server exited")
}
