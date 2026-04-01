package execenv

import (
	"context"
	"os"
	"strings"
)

const (
	// MetadataRuntimeID is the common metadata key used to persist a runtime binding.
	MetadataRuntimeID = "runtime_id"
	// MetadataTaskID is the common metadata key used to persist a task binding.
	MetadataTaskID = "task_id"
)

// StartSpecFromContext builds a process start spec from runtime context and optional persisted metadata.
func StartSpecFromContext(ctx context.Context, sessionID, command, workdir string, metadata map[string]any) StartSpec {
	return StartSpec{
		SessionID: strings.TrimSpace(sessionID),
		Command:   command,
		Workdir:   workdir,
		RuntimeID: firstNonEmpty(
			stringContextValue(ctx, MetadataRuntimeID),
			stringMetadataValue(metadata, MetadataRuntimeID),
		),
		TaskID: firstNonEmpty(
			stringContextValue(ctx, MetadataTaskID),
			stringMetadataValue(metadata, MetadataTaskID),
		),
		Env: os.Environ(),
	}
}

func stringContextValue(ctx context.Context, key string) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(key).(string)
	return strings.TrimSpace(value)
}

func stringMetadataValue(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
