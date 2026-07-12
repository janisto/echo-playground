package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/janisto/echo-observability"
	"go.uber.org/zap"
)

//	@title				Echo Playground API
//	@version			1.0
//	@description		Example API built with Echo v5. CBOR is supported for responses through content negotiation.
//	@servers.url		/v1
//	@servers.description	Version 1 API

// Version can be overridden at build time: -ldflags "-X main.Version=1.2.3"
var Version = "dev"

func main() {
	if err := run(context.Background()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "echo-playground: %v\n", err)
		os.Exit(1)
	}
}

func run(parent context.Context) (runErr error) {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}

	logger, err := obs.NewLogger(obs.LoggerConfig{
		Preset:      obs.PresetGCP,
		Level:       cfg.LogLevel,
		AddCaller:   true,
		Development: cfg.Environment == environmentDevelopment,
	})
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil &&
			!errors.Is(syncErr, syscall.ENOTTY) &&
			!errors.Is(syncErr, syscall.EINVAL) {
			runErr = errors.Join(runErr, fmt.Errorf("sync logger: %w", syncErr))
		}
	}()

	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	firebaseClients, err := newFirebaseClients(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := firebaseClients.Close(); closeErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close Firebase clients: %w", closeErr))
		}
	}()

	e := newEcho(cfg, logger, firebaseClients)
	sc := newStartConfig(cfg)

	logger.Info("server starting", zap.String("addr", cfg.Address), zap.String("version", Version))
	if err := sc.Start(ctx, e); err != nil {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	if cause := context.Cause(ctx); cause != nil {
		logger.Info("server exited", zap.Error(cause))
	} else {
		logger.Info("server exited")
	}
	return nil
}
