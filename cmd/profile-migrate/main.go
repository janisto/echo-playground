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
	"os"
	"sort"

	"cloud.google.com/go/firestore"

	"github.com/janisto/echo-playground/internal/platform/strictjson"
	profilesvc "github.com/janisto/echo-playground/internal/service/profile"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "profile-migrate: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string) (runErr error) {
	flags := flag.NewFlagSet("profile-migrate", flag.ContinueOnError)
	project := flags.String("project", "", "exact Firebase project ID")
	manifestPath := flags.String("manifest", "", "versioned terms-evidence manifest")
	apply := flags.Bool("apply", false, "apply authorized replacements instead of auditing")
	confirmation := flags.String("confirm-project", "", "must exactly equal --project when --apply is used")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *project == "" {
		return errors.New("--project is required and positional arguments are not accepted")
	}
	if *apply && (*confirmation == "" || *confirmation != *project) {
		return errors.New("--apply requires --confirm-project to exactly equal --project")
	}
	manifest, err := readManifest(*manifestPath)
	if err != nil {
		return err
	}
	client, err := firestore.NewClient(ctx, *project)
	if err != nil {
		return fmt.Errorf("initialize Firestore client: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, client.Close()) }()

	var results []profilesvc.MigrationResult
	if *apply {
		results, err = profilesvc.ApplyProfileMigration(ctx, client, manifest)
	} else {
		results, err = profilesvc.AuditProfileMigration(ctx, client, manifest)
	}
	printResults(results)
	return err
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

func printResults(results []profilesvc.MigrationResult) {
	sort.Slice(results, func(left, right int) bool {
		return results[left].DocumentFingerprint < results[right].DocumentFingerprint
	})
	counts := make(map[profilesvc.MigrationState]int)
	for _, result := range results {
		counts[result.State]++
		_, _ = fmt.Printf("%s %s %s\n", result.DocumentFingerprint, result.State, result.Reason)
	}
	_, _ = fmt.Printf(
		"summary verified=%d required=%d blocked=%d applied=%d\n",
		counts[profilesvc.MigrationVerified], counts[profilesvc.MigrationRequired], counts[profilesvc.MigrationBlocked],
		counts[profilesvc.MigrationApplied],
	)
}
