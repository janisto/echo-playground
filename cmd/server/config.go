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
	Environment       string
	FirebaseProject   string
	FirebaseMode      string
	IPExtractor       string
	TrustedProxyCIDRs []*net.IPNet
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

	ipExtractor := valueOrDefault(getenv("IP_EXTRACTOR"), "direct")
	if ipExtractor != "direct" && ipExtractor != "xff" {
		return config{}, errors.New("IP_EXTRACTOR must be direct or xff")
	}
	trustedProxyCIDRs, err := parseTrustedProxyCIDRs(ipExtractor, getenv("TRUSTED_PROXY_CIDRS"))
	if err != nil {
		return config{}, err
	}

	return config{
		Address:           net.JoinHostPort(host, port),
		Environment:       environment,
		FirebaseProject:   projectID,
		FirebaseMode:      mode,
		IPExtractor:       ipExtractor,
		TrustedProxyCIDRs: trustedProxyCIDRs,
		LogLevel:          level,
		RequestTimeout:    8 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}

func parseTrustedProxyCIDRs(ipExtractor, value string) ([]*net.IPNet, error) {
	value = strings.TrimSpace(value)
	if ipExtractor == "direct" {
		if value != "" {
			return nil, errors.New("TRUSTED_PROXY_CIDRS requires IP_EXTRACTOR=xff")
		}
		return nil, nil
	}
	if value == "" {
		return nil, errors.New("IP_EXTRACTOR=xff requires TRUSTED_PROXY_CIDRS")
	}

	parts := strings.Split(value, ",")
	ranges := make([]*net.IPNet, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("TRUSTED_PROXY_CIDRS must be a comma-separated list without empty entries")
		}
		_, network, err := net.ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXY_CIDRS contains invalid CIDR %q", part)
		}
		ranges = append(ranges, network)
	}
	return ranges, nil
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
