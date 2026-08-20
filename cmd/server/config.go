package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

const (
	environmentDevelopment = "development"
	environmentStaging     = "staging"
	environmentProduction  = "production"

	firebaseModeOffline  = "offline"
	firebaseModeEmulator = "emulator"
	firebaseModeLive     = "live"
)

type config struct {
	Address           string
	CORSOrigins       []string
	Environment       string
	FirebaseProject   string
	FirebaseMode      string
	LogLevel          zapcore.Level
	RequestTimeout    time.Duration
	ShutdownTimeout   time.Duration
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func loadConfig(getenv func(string) string) (config, error) {
	host := valueOrDefault(strings.TrimSpace(getenv("HOST")), "0.0.0.0")
	port := valueOrDefault(strings.TrimSpace(getenv("PORT")), "8080")
	portNumber, portErr := strconv.Atoi(port)
	if portErr != nil || portNumber < 1 || portNumber > 65535 {
		return config{}, errors.New("PORT must be an integer from 1 to 65535")
	}

	environment := valueOrDefault(strings.TrimSpace(getenv("APP_ENVIRONMENT")), environmentDevelopment)
	switch environment {
	case environmentDevelopment, environmentStaging, environmentProduction:
	default:
		return config{}, errors.New("APP_ENVIRONMENT must be development, staging, or production")
	}

	mode := valueOrDefault(strings.TrimSpace(getenv("FIREBASE_MODE")), firebaseModeOffline)
	projectID := strings.TrimSpace(getenv("FIREBASE_PROJECT_ID"))
	authEmulatorRaw := getenv("FIREBASE_AUTH_EMULATOR_HOST")
	firestoreEmulatorRaw := getenv("FIRESTORE_EMULATOR_HOST")
	authEmulator := strings.TrimSpace(authEmulatorRaw)
	firestoreEmulator := strings.TrimSpace(firestoreEmulatorRaw)
	if authEmulator != authEmulatorRaw || firestoreEmulator != firestoreEmulatorRaw {
		return config{}, errors.New("firebase emulator hosts must not contain leading or trailing whitespace")
	}
	if err := validateFirebaseConfig(environment, mode, projectID, authEmulator, firestoreEmulator); err != nil {
		return config{}, err
	}
	if projectID == "" && mode != firebaseModeLive {
		projectID = "demo-test-project"
	}

	levelName := valueOrDefault(strings.TrimSpace(getenv("LOG_LEVEL")), "info")
	switch levelName {
	case "debug", "info", "warn", "error":
	default:
		return config{}, errors.New("LOG_LEVEL must be debug, info, warn, or error")
	}
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(levelName)); err != nil {
		return config{}, fmt.Errorf("parse LOG_LEVEL: %w", err)
	}

	corsOrigins, err := parseCORSOrigins(getenv("CORS_ALLOWED_ORIGINS"))
	if err != nil {
		return config{}, err
	}

	return config{
		Address:           net.JoinHostPort(host, port),
		CORSOrigins:       corsOrigins,
		Environment:       environment,
		FirebaseProject:   projectID,
		FirebaseMode:      mode,
		LogLevel:          level,
		RequestTimeout:    15 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}

func parseCORSOrigins(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme != "https" && parsed.Scheme != "http" || parsed.Host == "" ||
			parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
			return nil, errors.New("CORS_ALLOWED_ORIGINS must contain comma-separated absolute HTTP(S) origins")
		}
		if origin == "*" {
			return nil, errors.New("CORS_ALLOWED_ORIGINS must not contain a wildcard")
		}
		if _, duplicate := seen[origin]; duplicate {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins, nil
}

func validateFirebaseConfig(environment, mode, projectID, authEmulator, firestoreEmulator string) error {
	if (authEmulator == "") != (firestoreEmulator == "") {
		return errors.New("FIREBASE_AUTH_EMULATOR_HOST and FIRESTORE_EMULATOR_HOST must be configured together")
	}
	switch mode {
	case firebaseModeOffline:
		if environment != environmentDevelopment {
			return errors.New("FIREBASE_MODE=offline is allowed only in development")
		}
		if authEmulator != "" {
			return errors.New("firebase emulator hosts require FIREBASE_MODE=emulator")
		}
	case firebaseModeEmulator:
		if environment != environmentDevelopment {
			return errors.New("FIREBASE_MODE=emulator is allowed only in development")
		}
		if authEmulator == "" {
			return errors.New("FIREBASE_MODE=emulator requires Auth and Firestore emulator hosts")
		}
		if err := validateHostPort("FIREBASE_AUTH_EMULATOR_HOST", authEmulator); err != nil {
			return err
		}
		if err := validateHostPort("FIRESTORE_EMULATOR_HOST", firestoreEmulator); err != nil {
			return err
		}
		if projectID != "" && !strings.HasPrefix(projectID, "demo-") {
			return errors.New("FIREBASE_MODE=emulator requires a demo-* project ID")
		}
	case firebaseModeLive:
		if projectID == "" {
			return errors.New("FIREBASE_MODE=live requires FIREBASE_PROJECT_ID")
		}
		if strings.HasPrefix(projectID, "demo-") {
			return errors.New("FIREBASE_MODE=live rejects demo-* project IDs")
		}
		if authEmulator != "" {
			return errors.New("FIREBASE_MODE=live rejects emulator hosts")
		}
	default:
		return errors.New("FIREBASE_MODE must be offline, emulator, or live")
	}
	return nil
}

func validateHostPort(name, value string) error {
	host, port, err := net.SplitHostPort(value)
	if err != nil || strings.TrimSpace(host) == "" {
		return fmt.Errorf("%s must use host:port without a URL scheme", name)
	}
	endpoint, err := url.Parse("http://" + value)
	if err != nil || endpoint.Host != value || endpoint.Hostname() != host || endpoint.Port() != port {
		return fmt.Errorf("%s must contain a valid host and port", name)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("%s port must be an integer from 1 to 65535", name)
	}
	return nil
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
