package main

import (
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Address != "0.0.0.0:8080" {
		t.Fatalf("unexpected address %q", cfg.Address)
	}
	if cfg.FirebaseProject != "demo-test-project" {
		t.Fatalf("unexpected project %q", cfg.FirebaseProject)
	}
	if cfg.FirebaseMode != firebaseModeOffline || cfg.Environment != environmentDevelopment {
		t.Fatalf("unexpected Firebase defaults: %#v", cfg)
	}
	if cfg.LogLevel != zapcore.InfoLevel {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
	if cfg.RequestTimeout >= cfg.WriteTimeout {
		t.Fatalf("request timeout %s must be less than write timeout %s", cfg.RequestTimeout, cfg.WriteTimeout)
	}
}

func TestLoadConfigValues(t *testing.T) {
	values := map[string]string{
		"HOST":                "127.0.0.1",
		"PORT":                "9090",
		"APP_ENVIRONMENT":     "staging",
		"FIREBASE_MODE":       "live",
		"FIREBASE_PROJECT_ID": "project-1",
		"LOG_LEVEL":           "debug",
	}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Address != "127.0.0.1:9090" || cfg.FirebaseProject != "project-1" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.LogLevel != zapcore.DebugLevel {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestParseCORSOriginsAcceptsCanonicalOrigins(t *testing.T) {
	origins, err := parseCORSOrigins(
		" https://app.example, http://localhost:3000,https://[2001:db8::1],https://0x7g,https://app.example ",
	)
	if err != nil {
		t.Fatalf("parse CORS origins: %v", err)
	}
	want := []string{"https://app.example", "http://localhost:3000", "https://[2001:db8::1]", "https://0x7g"}
	if strings.Join(origins, ",") != strings.Join(want, ",") {
		t.Fatalf("origins = %#v, want %#v", origins, want)
	}
}

func TestParseCORSOriginsRejectsNoncanonicalOrigins(t *testing.T) {
	for _, origin := range []string{
		"HTTPS://app.example",
		"https://APP.example",
		"https://app.example:443",
		"http://app.example:80",
		"https://app.example:08443",
		"https://faß.example",
		"https://127.1",
		"https://0x7f.1",
		"https://0x7f",
		"https://[2001:0db8:0:0:0:0:0:1]",
	} {
		t.Run(origin, func(t *testing.T) {
			if _, err := parseCORSOrigins(origin); err == nil {
				t.Fatalf("parseCORSOrigins(%q) unexpectedly succeeded", origin)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		match  string
	}{
		{name: "port", values: map[string]string{"PORT": "0"}, match: "PORT"},
		{name: "environment", values: map[string]string{"APP_ENVIRONMENT": "prod"}, match: "APP_ENVIRONMENT"},
		{name: "log level", values: map[string]string{"LOG_LEVEL": "verbose"}, match: "LOG_LEVEL"},
		{name: "terminating log level", values: map[string]string{"LOG_LEVEL": "fatal"}, match: "LOG_LEVEL"},
		{
			name:   "project outside development",
			values: map[string]string{"APP_ENVIRONMENT": "production"},
			match:  "offline is allowed only in development",
		},
		{
			name: "demo project outside development",
			values: map[string]string{
				"APP_ENVIRONMENT": "production", "FIREBASE_MODE": "live", "FIREBASE_PROJECT_ID": "demo-test-project",
			},
			match: "rejects demo-*",
		},
		{
			name: "partial emulator configuration",
			values: map[string]string{
				"FIREBASE_MODE": "emulator", "FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:7110",
			},
			match: "configured together",
		},
		{
			name: "emulators outside development",
			values: map[string]string{
				"APP_ENVIRONMENT":             "production",
				"FIREBASE_MODE":               "emulator",
				"FIREBASE_PROJECT_ID":         "demo-project",
				"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:7110",
				"FIRESTORE_EMULATOR_HOST":     "127.0.0.1:7130",
			},
			match: "emulator is allowed only in development",
		},
		{
			name: "emulator URL",
			values: map[string]string{
				"FIREBASE_MODE":               "emulator",
				"FIREBASE_AUTH_EMULATOR_HOST": "http://127.0.0.1:7110",
				"FIRESTORE_EMULATOR_HOST":     "127.0.0.1:7130",
			},
			match: "host:port",
		},
		{
			name: "emulator surrounding whitespace",
			values: map[string]string{
				"FIREBASE_MODE":               "emulator",
				"FIREBASE_AUTH_EMULATOR_HOST": " 127.0.0.1:7110",
				"FIRESTORE_EMULATOR_HOST":     "127.0.0.1:7130",
			},
			match: "whitespace",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(func(key string) string { return tt.values[key] })
			if err == nil || !strings.Contains(err.Error(), tt.match) {
				t.Fatalf("expected error containing %q, got %v", tt.match, err)
			}
		})
	}
}

func TestLoadConfigEmulator(t *testing.T) {
	values := map[string]string{
		"FIREBASE_MODE":               "emulator",
		"FIREBASE_PROJECT_ID":         "demo-local",
		"FIREBASE_AUTH_EMULATOR_HOST": "[::1]:7110",
		"FIRESTORE_EMULATOR_HOST":     "firestore:7130",
	}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load emulator config: %v", err)
	}
	if cfg.FirebaseMode != firebaseModeEmulator || cfg.FirebaseProject != "demo-local" {
		t.Fatalf("unexpected emulator config: %#v", cfg)
	}
}

func TestNewServer(t *testing.T) {
	cfg, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	core, logs := observer.New(zapcore.ErrorLevel)
	server, err := newServer(cfg, echo.New(), zap.New(core))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if server.Addr != cfg.Address || server.ReadTimeout != 5*time.Second ||
		server.WriteTimeout != 20*time.Second || server.MaxHeaderBytes != 64<<10 {
		t.Fatalf("unexpected server: %#v", server)
	}
	server.ErrorLog.Print("accept failed")
	entries := logs.All()
	if len(entries) != 1 || entries[0].LoggerName != "http.server" || entries[0].Message != "accept failed" {
		t.Fatalf("expected net/http error through process logger, got %#v", entries)
	}
}
