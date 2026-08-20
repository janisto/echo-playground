package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

func TestRunRejectsUnsafeInvocationBeforeFirestoreInitialization(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing project", want: "--project is required"},
		{name: "positional argument", args: []string{"--project", "demo-project", "extra"}, want: "positional arguments"},
		{name: "missing apply confirmation", args: []string{"--project", "production", "--apply"}, want: "--confirm-project"},
		{name: "mismatched apply confirmation", args: []string{"--project", "production", "--apply", "--confirm-project", "other"}, want: "--confirm-project"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := run(t.Context(), test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("run error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestReadManifestIsStrictClosedAndEvidenceBound(t *testing.T) {
	directory := t.TempDir()
	write := func(t *testing.T, name, body string) string {
		t.Helper()
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	validPath := write(
		t,
		"valid.json",
		`{"version":1,"entries":{"cHJvZmlsZQ":{"termsAccepted":true,"evidence":"consent-ledger/record-1"}}}`,
	)
	manifest, err := readManifest(validPath)
	if err != nil || manifest.Version != profilesvc.ProfileMigrationManifestVersion || len(manifest.Entries) != 1 {
		t.Fatalf("valid manifest = %#v, %v", manifest, err)
	}
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "duplicate member", body: `{"version":1,"version":1,"entries":{}}`},
		{name: "unknown member", body: `{"version":1,"entries":{},"project":"production"}`},
		{name: "wrong version", body: `{"version":2,"entries":{}}`},
		{name: "missing terms evidence", body: `{"version":1,"entries":{"record":{"termsAccepted":false,"evidence":""}}}`},
		{name: "path-shaped document", body: `{"version":1,"entries":{"profiles/record":{"termsAccepted":true,"evidence":"ledger"}}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := readManifest(write(t, test.name+".json", test.body)); err == nil {
				t.Fatal("invalid manifest was accepted")
			}
		})
	}
}

func TestReadManifestDefaultsToEmptyAuditManifest(t *testing.T) {
	manifest, err := readManifest("")
	if err != nil || manifest.Version != profilesvc.ProfileMigrationManifestVersion || manifest.Entries == nil ||
		len(manifest.Entries) != 0 {
		t.Fatalf("default manifest = %#v, %v", manifest, err)
	}
}
