package logging

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sync"
)

const traceparentHeader = "traceparent"

// W3C Trace Context format: {version}-{trace-id}-{parent-id}-{trace-flags}
var traceHeaderRe = regexp.MustCompile(
	`^([0-9a-fA-F]{2})-([0-9a-fA-F]{32})-([0-9a-fA-F]{16})-([0-9a-fA-F]{2})$`,
)

var (
	projectIDOnce   sync.Once
	cachedProjectID string
)

func loggerWithTrace(base *slog.Logger, header, projectID, requestID string) *slog.Logger {
	if base == nil {
		base = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	attrs := traceAttrs(header, projectID)
	if requestID != "" {
		attrs = append(attrs, slog.String("requestId", requestID))
	}
	if len(attrs) == 0 {
		return base
	}
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return base.With(args...)
}

func traceAttrs(header, projectID string) []slog.Attr {
	if projectID == "" {
		return nil
	}
	matches := traceHeaderRe.FindStringSubmatch(header)
	if len(matches) != 5 {
		return nil
	}
	traceID := matches[2]
	spanID := matches[3]
	flags := matches[4]
	sampled := flags == "01"
	resource := fmt.Sprintf("projects/%s/traces/%s", projectID, traceID)

	return []slog.Attr{
		slog.String("logging.googleapis.com/trace", resource),
		slog.String("logging.googleapis.com/spanId", spanID),
		slog.Bool("logging.googleapis.com/trace_sampled", sampled),
	}
}

func traceResource(header, projectID string) string {
	if projectID == "" {
		return ""
	}
	matches := traceHeaderRe.FindStringSubmatch(header)
	if len(matches) != 5 {
		return ""
	}
	return fmt.Sprintf("projects/%s/traces/%s", projectID, matches[2])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolveProjectID() string {
	projectIDOnce.Do(func() {
		cachedProjectID = firstNonEmpty(
			os.Getenv("FIREBASE_PROJECT_ID"),
			os.Getenv("GOOGLE_CLOUD_PROJECT"),
			os.Getenv("GCP_PROJECT"),
			os.Getenv("GCLOUD_PROJECT"),
			os.Getenv("PROJECT_ID"),
		)
	})
	return cachedProjectID
}
