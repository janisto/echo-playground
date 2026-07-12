package main

import (
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
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
	if cfg.LogLevel != zapcore.InfoLevel || cfg.IPExtractor != "direct" {
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
		"FIREBASE_PROJECT_ID": "project-1",
		"LOG_LEVEL":           "debug",
		"IP_EXTRACTOR":        "xff",
	}
	cfg, err := loadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Address != "127.0.0.1:9090" || cfg.FirebaseProject != "project-1" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.LogLevel != zapcore.DebugLevel || cfg.IPExtractor != "xff" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		match  string
	}{
		{name: "port", values: map[string]string{"PORT": "0"}, match: "PORT"},
		{name: "log level", values: map[string]string{"LOG_LEVEL": "verbose"}, match: "LOG_LEVEL"},
		{name: "IP extractor", values: map[string]string{"IP_EXTRACTOR": "forwarded"}, match: "IP_EXTRACTOR"},
		{
			name:   "project outside development",
			values: map[string]string{"APP_ENVIRONMENT": "production"},
			match:  "FIREBASE_PROJECT_ID",
		},
		{
			name: "demo project outside development",
			values: map[string]string{
				"APP_ENVIRONMENT": "production", "FIREBASE_PROJECT_ID": "demo-test-project",
			},
			match: "demo Firebase projects",
		},
		{
			name:   "partial emulator configuration",
			values: map[string]string{"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:7110"},
			match:  "configured together",
		},
		{
			name: "emulators outside development",
			values: map[string]string{
				"APP_ENVIRONMENT":             "production",
				"FIREBASE_PROJECT_ID":         "prod-project",
				"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:7110",
				"FIRESTORE_EMULATOR_HOST":     "127.0.0.1:7130",
			},
			match: "emulators are allowed only in development",
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

func TestNewStartConfig(t *testing.T) {
	cfg, err := loadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	sc := newStartConfig(cfg)
	if !sc.HideBanner || !sc.HidePort {
		t.Fatal("expected duplicate Echo startup output to be hidden")
	}
	if sc.GracefulTimeout != 10*time.Second {
		t.Fatalf("unexpected graceful timeout %s", sc.GracefulTimeout)
	}
}
