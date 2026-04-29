package daemonhost

import (
	"fmt"
	"strings"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/tasks"
)

func CollaborationTaskToProto(item tasks.Task) *daemonv1.Task {
	return taskToProto(item)
}

func ValidateCollaborationTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("target is required")
	}
	if strings.ContainsAny(target, "\x00\r\n") {
		return "", fmt.Errorf("target contains invalid control characters")
	}
	return target, nil
}
