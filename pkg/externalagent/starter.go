package externalagent

import (
	"context"
	"fmt"
	"strings"

	"nekobot/pkg/execenv"
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

type StartRuntimeCommandFunc func(command, sessionID string) (string, string)

func EnsureProcess(
	ctx context.Context,
	cfgWorkspacePath string,
	processProbe ProcessProbe,
	processMgr ProcessStarter,
	sessionMgr SessionUpdater,
	buildRuntimeCommand StartRuntimeCommandFunc,
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
	launchCommand := command
	tmuxSession := ""
	if buildRuntimeCommand != nil {
		if wrapped, sessionName := buildRuntimeCommand(launchCommand, sessionID); sessionName != "" {
			launchCommand = wrapped
			tmuxSession = sessionName
			metadata["runtime_transport"] = "tmux"
			metadata["tmux_session"] = tmuxSession
		} else {
			delete(metadata, "runtime_transport")
			delete(metadata, "tmux_session")
		}
	}
	metadata["launch_cmd"] = launchCommand

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
	_ = sessionMgr.AppendEvent(context.Background(), sessionID, "process_started", map[string]interface{}{
		"command":      command,
		"launch_cmd":   launchCommand,
		"tmux_session": tmuxSession,
		"workdir":      workdir,
	})
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
