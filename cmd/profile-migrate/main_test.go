package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

type reportTestWriter struct {
	writes int
	failAt int
	short  bool
}

func (writer *reportTestWriter) Write(data []byte) (int, error) {
	writer.writes++
	if writer.writes != writer.failAt {
		return len(data), nil
	}
	if writer.short {
		return len(data) - 1, nil
	}
	return 0, errors.New("report destination failed")
}

func TestRunRejectsUnsafeInvocationBeforeFirestoreInitialization(t *testing.T) {
	t.Setenv("FIRESTORE_EMULATOR_HOST", "")
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing project", want: "--project is required"},
		{name: "missing mode", args: []string{"--project", "production"}, want: "--mode"},
		{name: "invalid mode", args: []string{"--project", "production", "--mode", "offline"}, want: "--mode"},
		{
			name: "positional argument",
			args: []string{"--project", "demo-project", "--mode", "emulator", "extra"},
			want: "positional arguments",
		},
		{
			name: "missing apply confirmation",
			args: []string{"--project", "production", "--mode", "live", "--apply"},
			want: "--confirm-project",
		},
		{
			name: "mismatched apply confirmation",
			args: []string{
				"--project", "production", "--mode", "live", "--apply", "--confirm-project", "other",
			},
			want: "--confirm-project",
		},
		{
			name: "apply confirmation without apply",
			args: []string{
				"--project", "production", "--mode", "live", "--confirm-project", "production",
			},
			want: "require --apply",
		},
		{
			name: "missing rollback confirmation",
			args: []string{
				"--project", "production", "--mode", "live", "--apply",
				"--confirm-project", "production", "--confirm-profile-writes-quiesced",
				"--manifest", "/path/that/must/not/be-read.json",
			},
			want: "--confirm-rollback-reference",
		},
		{
			name: "unsafe rollback confirmation",
			args: []string{
				"--project", "production", "--mode", "live", "--apply",
				"--confirm-project", "production", "--confirm-rollback-reference", " backup/export-1",
				"--confirm-profile-writes-quiesced",
			},
			want: "--confirm-rollback-reference",
		},
		{
			name: "missing quiescence confirmation",
			args: []string{
				"--project", "production", "--mode", "live", "--apply",
				"--confirm-project", "production", "--confirm-rollback-reference", "backup/export-1",
			},
			want: "--confirm-profile-writes-quiesced",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := run(t.Context(), test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("run error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateApplyConfirmations(t *testing.T) {
	for _, test := range []struct {
		name              string
		apply             bool
		project           string
		projectConfirm    string
		rollbackReference string
		writesQuiesced    bool
		wantErr           bool
	}{
		{name: "audit"},
		{
			name:              "apply",
			apply:             true,
			project:           "production",
			projectConfirm:    "production",
			rollbackReference: "provider-export/2026-08-20T12:00:00Z",
			writesQuiesced:    true,
		},
		{name: "audit with rollback confirmation", rollbackReference: "provider-export/1", wantErr: true},
		{
			name:              "apply with control in rollback reference",
			apply:             true,
			project:           "production",
			projectConfirm:    "production",
			rollbackReference: "provider-export/1\nforged-output",
			writesQuiesced:    true,
			wantErr:           true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateApplyConfirmations(
				test.apply,
				test.project,
				test.projectConfirm,
				test.rollbackReference,
				test.writesQuiesced,
			)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateApplyConfirmations() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestRunRejectsAmbientEmulatorForLiveTarget(t *testing.T) {
	t.Setenv("FIRESTORE_EMULATOR_HOST", "127.0.0.1:7130")
	err := run(t.Context(), []string{"--project", "production", "--mode", "live"})
	if err == nil || !strings.Contains(err.Error(), "FIRESTORE_EMULATOR_HOST") {
		t.Fatalf("run error = %v, want ambient emulator rejection", err)
	}
}

func TestValidateMigrationTarget(t *testing.T) {
	for _, test := range []struct {
		name    string
		mode    string
		project string
		host    string
		wantErr bool
	}{
		{name: "live", mode: "live", project: "production"},
		{name: "emulator", mode: "emulator", project: "demo-project", host: "127.0.0.1:7130"},
		{name: "missing mode", project: "production", wantErr: true},
		{name: "live demo project", mode: "live", project: "demo-project", wantErr: true},
		{name: "live emulator host", mode: "live", project: "production", host: "127.0.0.1:7130", wantErr: true},
		{name: "emulator live project", mode: "emulator", project: "production", host: "127.0.0.1:7130", wantErr: true},
		{name: "emulator missing host", mode: "emulator", project: "demo-project", wantErr: true},
		{name: "emulator host whitespace", mode: "emulator", project: "demo-project", host: " 127.0.0.1:7130", wantErr: true},
		{name: "emulator URL", mode: "emulator", project: "demo-project", host: "http://127.0.0.1:7130", wantErr: true},
		{name: "emulator empty hostname", mode: "emulator", project: "demo-project", host: ":7130", wantErr: true},
		{name: "emulator zero port", mode: "emulator", project: "demo-project", host: "127.0.0.1:0", wantErr: true},
		{name: "emulator oversized port", mode: "emulator", project: "demo-project", host: "127.0.0.1:65536", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateMigrationTarget(test.mode, test.project, test.host)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateMigrationTarget() error = %v, wantErr %v", err, test.wantErr)
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

func TestPrintResultsIsDeterministicAndPropagatesWriteFailures(t *testing.T) {
	results := []profilesvc.MigrationResult{
		{DocumentFingerprint: "b", State: profilesvc.MigrationApplied, Reason: "applied reason"},
		{DocumentFingerprint: "a", State: profilesvc.MigrationBlocked, Reason: "blocked reason"},
	}
	var output strings.Builder
	if err := printResults(&output, results); err != nil {
		t.Fatalf("printResults() error = %v", err)
	}
	want := "a blocked blocked reason\nb applied applied reason\n" +
		"summary verified=0 required=0 blocked=1 applied=1\n"
	if output.String() != want {
		t.Fatalf("printResults() = %q, want %q", output.String(), want)
	}

	for _, short := range []bool{false, true} {
		writer := &reportTestWriter{failAt: 2, short: short}
		err := printResults(writer, results)
		if !errors.Is(err, errMigrationReportWrite) || writer.writes != 2 {
			t.Fatalf("printResults(short=%t) = %v after %d writes", short, err, writer.writes)
		}
	}
}
