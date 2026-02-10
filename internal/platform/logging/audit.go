package logging

import (
	"context"
	"log/slog"
)

// LogAuditEvent logs a structured audit event for security and compliance.
func LogAuditEvent(
	ctx context.Context,
	action, userID, resourceType, resourceID, result string,
	details map[string]any,
) {
	LoggerFromContext(ctx).LogAttrs(ctx, slog.LevelInfo, "Audit event",
		slog.String("audit.action", action),
		slog.String("audit.user_id", userID),
		slog.String("audit.resource_type", resourceType),
		slog.String("audit.resource_id", resourceID),
		slog.String("audit.result", result),
		slog.Any("audit.details", details),
	)
}
