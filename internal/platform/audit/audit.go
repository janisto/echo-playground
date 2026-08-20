package audit

import (
	"context"

	"github.com/janisto/echo-observability/v2"
	"go.uber.org/zap"
)

// LogEvent logs a structured audit event for security and compliance.
func LogEvent(
	ctx context.Context,
	action, resourceType, result string,
	details map[string]any,
) {
	obs.Logger(ctx).Info("Audit event",
		zap.String("audit.action", action),
		zap.String("audit.resource_type", resourceType),
		zap.String("audit.result", result),
		zap.Any("audit.details", details),
	)
}
