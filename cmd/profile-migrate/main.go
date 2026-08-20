// Command profile-migrate audits or explicitly applies the one-time profile
// persistence cutover. Audit is the default and performs no writes.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"cloud.google.com/go/firestore"

	"github.com/janisto/echo-playground/internal/platform/strictjson"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

const (
	migrationModeEmulator = "emulator"
	migrationModeLive     = "live"
)

var errMigrationReportWrite = errors.New("write migration report failed")

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "profile-migrate: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string) (runErr error) {
	flags := flag.NewFlagSet("profile-migrate", flag.ContinueOnError)
	project := flags.String("project", "", "exact Firebase project ID")
	mode := flags.String("mode", "", "explicit Firestore target mode: live or emulator")
	manifestPath := flags.String("manifest", "", "versioned terms-evidence manifest")
	apply := flags.Bool("apply", false, "apply authorized replacements instead of auditing")
	confirmation := flags.String("confirm-project", "", "must exactly equal --project when --apply is used")
	rollbackReference := flags.String(
		"confirm-rollback-reference",
		"",
		"verified backup and rollback reference required by --apply",
	)
	quiesced := flags.Bool(
		"confirm-profile-writes-quiesced",
		false,
		"affirm that profile writes are quiesced for --apply",
	)
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *project == "" {
		return errors.New("--project is required and positional arguments are not accepted")
	}
	if err := validateMigrationTarget(*mode, *project, os.Getenv("FIRESTORE_EMULATOR_HOST")); err != nil {
		return err
	}
	if err := validateApplyConfirmations(
		*apply,
		*project,
		*confirmation,
		*rollbackReference,
		*quiesced,
	); err != nil {
		return err
	}
	manifest, err := readManifest(*manifestPath)
	if err != nil {
		return err
	}
	client, err := firestore.NewClient(ctx, *project)
	if err != nil {
		return errors.New("initialize firestore client failed")
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			runErr = errors.Join(runErr, errors.New("close firestore client failed"))
		}
	}()

	var results []profilesvc.MigrationResult
	if *apply {
		results, err = profilesvc.ApplyProfileMigration(ctx, client, manifest)
	} else {
		results, err = profilesvc.AuditProfileMigration(ctx, client, manifest)
	}
	reportErr := printResults(os.Stdout, results)
	return errors.Join(err, reportErr)
}

func validateApplyConfirmations(
	apply bool,
	project string,
	projectConfirmation string,
	rollbackReference string,
	writesQuiesced bool,
) error {
	if !apply {
		if projectConfirmation != "" || rollbackReference != "" || writesQuiesced {
			return errors.New("apply confirmation flags require --apply")
		}
		return nil
	}
	if projectConfirmation == "" || projectConfirmation != project {
		return errors.New("--apply requires --confirm-project to exactly equal --project")
	}
	if !validRollbackReference(rollbackReference) {
		return errors.New("--apply requires a safe non-empty --confirm-rollback-reference")
	}
	if !writesQuiesced {
		return errors.New("--apply requires --confirm-profile-writes-quiesced")
	}
	return nil
}

func validRollbackReference(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value ||
		utf8.RuneCountInString(value) > 500 {
		return false
	}
	for _, character := range value {
		if character <= 0x1f || character >= 0x7f && character <= 0x9f {
			return false
		}
	}
	return true
}

func validateMigrationTarget(mode, project, firestoreEmulatorHost string) error {
	switch mode {
	case migrationModeLive:
		if strings.HasPrefix(project, "demo-") {
			return errors.New("--mode live rejects demo-* project IDs")
		}
		if firestoreEmulatorHost != "" {
			return errors.New("--mode live requires FIRESTORE_EMULATOR_HOST to be unset")
		}
	case migrationModeEmulator:
		if !strings.HasPrefix(project, "demo-") {
			return errors.New("--mode emulator requires a demo-* project ID")
		}
		if firestoreEmulatorHost == "" {
			return errors.New("--mode emulator requires FIRESTORE_EMULATOR_HOST")
		}
		if strings.TrimSpace(firestoreEmulatorHost) != firestoreEmulatorHost {
			return errors.New("FIRESTORE_EMULATOR_HOST must not contain leading or trailing whitespace")
		}
		if err := validateFirestoreEmulatorHost(firestoreEmulatorHost); err != nil {
			return err
		}
	default:
		return errors.New("--mode must be live or emulator")
	}
	return nil
}

func validateFirestoreEmulatorHost(value string) error {
	host, port, err := net.SplitHostPort(value)
	if err != nil || strings.TrimSpace(host) == "" {
		return errors.New("FIRESTORE_EMULATOR_HOST must use host:port without a URL scheme")
	}
	endpoint, err := url.Parse("http://" + value)
	if err != nil || endpoint.Host != value || endpoint.Hostname() != host || endpoint.Port() != port {
		return errors.New("FIRESTORE_EMULATOR_HOST must contain a valid host and port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("FIRESTORE_EMULATOR_HOST port must be an integer from 1 to 65535")
	}
	return nil
}

func readManifest(path string) (profilesvc.MigrationManifest, error) {
	manifest := profilesvc.MigrationManifest{
		Version: profilesvc.ProfileMigrationManifestVersion,
		Entries: map[string]profilesvc.MigrationAuthorization{},
	}
	if path == "" {
		return manifest, nil
	}
	// #nosec G304 -- the manifest is an explicit local operator input to this one-time CLI.
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, fmt.Errorf("read migration manifest: %w", err)
	}
	if err := strictjson.Validate(data); err != nil {
		return manifest, errors.New("migration manifest must be one strict JSON document")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, errors.New("migration manifest does not match the closed schema")
	}
	if err := manifest.Validate(); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func printResults(writer io.Writer, results []profilesvc.MigrationResult) error {
	sort.Slice(results, func(left, right int) bool {
		return results[left].DocumentFingerprint < results[right].DocumentFingerprint
	})
	counts := make(map[profilesvc.MigrationState]int)
	for _, result := range results {
		counts[result.State]++
		line := fmt.Sprintf(
			"%s %s %s\n",
			result.DocumentFingerprint,
			result.State,
			result.Reason,
		)
		if written, err := io.WriteString(writer, line); err != nil || written != len(line) {
			return errMigrationReportWrite
		}
	}
	summary := fmt.Sprintf(
		"summary verified=%d required=%d blocked=%d applied=%d\n",
		counts[profilesvc.MigrationVerified], counts[profilesvc.MigrationRequired], counts[profilesvc.MigrationBlocked],
		counts[profilesvc.MigrationApplied],
	)
	if written, err := io.WriteString(writer, summary); err != nil || written != len(summary) {
		return errMigrationReportWrite
	}
	return nil
}
