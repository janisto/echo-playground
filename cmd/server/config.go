package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

const environmentDevelopment = "development"

type config struct {
	Address           string
	Environment       string
	FirebaseProject   string
	FirebaseEmulators bool
	IPExtractor       string
	LogLevel          zapcore.Level
	RequestTimeout    time.Duration
	ShutdownTimeout   time.Duration
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func loadConfig(getenv func(string) string) (config, error) {
	host := valueOrDefault(getenv("HOST"), "0.0.0.0")
	port := valueOrDefault(getenv("PORT"), "8080")
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return config{}, errors.New("PORT must be an integer from 1 to 65535")
	}

	environment := valueOrDefault(getenv("APP_ENVIRONMENT"), environmentDevelopment)
	projectID := strings.TrimSpace(getenv("FIREBASE_PROJECT_ID"))
	if projectID == "" {
		if environment != environmentDevelopment {
			return config{}, errors.New("FIREBASE_PROJECT_ID is required outside development")
		}
		projectID = "demo-test-project"
	}
	if environment != environmentDevelopment && strings.HasPrefix(projectID, "demo-") {
		return config{}, errors.New("demo Firebase projects are allowed only in development")
	}
	authEmulator := getenv("FIREBASE_AUTH_EMULATOR_HOST") != ""
	firestoreEmulator := getenv("FIRESTORE_EMULATOR_HOST") != ""
	if authEmulator != firestoreEmulator {
		return config{}, errors.New(
			"FIREBASE_AUTH_EMULATOR_HOST and FIRESTORE_EMULATOR_HOST must be configured together",
		)
	}
	if environment != environmentDevelopment && authEmulator {
		return config{}, errors.New("firebase emulators are allowed only in development")
	}

	var level zapcore.Level
	if err := level.UnmarshalText([]byte(valueOrDefault(getenv("LOG_LEVEL"), "info"))); err != nil {
		return config{}, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error: %w", err)
	}

	ipExtractor := valueOrDefault(getenv("IP_EXTRACTOR"), "direct")
	if ipExtractor != "direct" && ipExtractor != "xff" {
		return config{}, errors.New("IP_EXTRACTOR must be direct or xff")
	}

	return config{
		Address:           net.JoinHostPort(host, port),
		Environment:       environment,
		FirebaseProject:   projectID,
		FirebaseEmulators: authEmulator && firestoreEmulator,
		IPExtractor:       ipExtractor,
		LogLevel:          level,
		RequestTimeout:    8 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
