package externalagent

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/execenv"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/toolsessions"
)

type ProcessProbe interface {
	HasProcess(sessionID string) bool
}

type ProcessStarter interface {
	StartWithSpec(ctx context.Context, spec execenv.StartSpec) error
}

type SessionUpdater interface {
	UpdateSessionMetadata(ctx context.Context, id string, metadata map[string]interface{}) error
	UpdateSessionLaunch(ctx context.Context, id, tool, title, command, workdir string) (*toolsessions.Session, error)
	AppendEvent(ctx context.Context, id, eventType string, payload map[string]interface{}) error
}

type RuntimeTransport = runtimeagents.RuntimeTransport

func EnsureProcess(
	ctx context.Context,
	cfgWorkspacePath string,
	processProbe ProcessProbe,
	processMgr ProcessStarter,
	sessionMgr SessionUpdater,
	transport RuntimeTransport,
	sess *toolsessions.Session,
) error {
	if processMgr == nil || sessionMgr == nil || sess == nil {
		return nil
	}
	sessionID := strings.TrimSpace(sess.ID)
	if sessionID == "" {
		return nil
	}
	if processProbe != nil && processProbe.HasProcess(sessionID) {
		return nil
	}

	workdir := strings.TrimSpace(sess.Workdir)
	if workdir == "" {
		workdir = strings.TrimSpace(cfgWorkspacePath)
	}
	command := strings.TrimSpace(sess.Command)
	if command == "" {
		return fmt.Errorf("command is required")
	}

	metadata := cloneMetadata(sess.Metadata)
	launchInfo := runtimeagents.LaunchInfo{LaunchCommand: command}
	if transport != nil {
		launchInfo = transport.WrapStart(command, sessionID)
	}
	launchCommand := launchInfo.LaunchCommand
	metadata = runtimeagents.ApplyLaunchMetadata(metadata, launchInfo)

	if err := sessionMgr.UpdateSessionMetadata(ctx, sessionID, metadata); err != nil {
		return fmt.Errorf("persist external agent metadata: %w", err)
	}
	spec := execenv.StartSpecFromContext(ctx, sessionID, launchCommand, workdir, metadata)
	if err := processMgr.StartWithSpec(context.Background(), spec); err != nil {
		return err
	}
	if _, err := sessionMgr.UpdateSessionLaunch(ctx, sessionID, strings.TrimSpace(sess.Tool), strings.TrimSpace(sess.Title), command, workdir); err != nil {
		return fmt.Errorf("update external agent launch session: %w", err)
	}
	eventPayload := runtimeagents.ApplyRuntimeSessionPayload(map[string]interface{}{
		"command":         command,
		"launch_cmd":      launchCommand,
		"workdir":         workdir,
	}, launchInfo.TransportName, launchInfo.SessionName)
	eventPayload[runtimeagents.MetadataRuntimeTransport] = strings.TrimSpace(launchInfo.TransportName)
	_ = sessionMgr.AppendEvent(context.Background(), sessionID, "process_started", eventPayload)
	return nil
}

func cloneMetadata(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
