package tools

import (
	"context"
	"strings"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/execenv"
	"nekobot/pkg/process"
	"nekobot/pkg/toolsessions"
)

func TestToolSessionToolSpawnPersistsResumeMetadata(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newExecTestLogger(t)
	client := newTestEntClientForTools(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	preparer := &capturePreparer{}
	pm := process.NewManager(log)
	pm.SetPreparer(preparer)
	tool := NewToolSessionTool(pm, mgr, cfg.WorkspacePath())

	ctx := context.WithValue(context.Background(), execenv.MetadataRuntimeID, "runtime-agent")
	ctx = context.WithValue(ctx, execenv.MetadataTaskID, "task-agent")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":  "spawn",
		"tool":    "codex",
		"command": "sleep 5",
	})
	if err != nil {
		t.Fatalf("spawn tool session: %v", err)
	}
	if !strings.Contains(result, "Tool session created successfully") {
		t.Fatalf("unexpected result: %s", result)
	}

	sessionID := preparer.last.SessionID
	if sessionID == "" {
		t.Fatal("expected session id to be captured")
	}
	if preparer.last.RuntimeID != "runtime-agent" {
		t.Fatalf("expected runtime id in start spec, got %q", preparer.last.RuntimeID)
	}
	if preparer.last.TaskID != "task-agent" {
		t.Fatalf("expected task id in start spec, got %q", preparer.last.TaskID)
	}

	sess, err := mgr.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("get created session: %v", err)
	}
	if got, _ := sess.Metadata[execenv.MetadataRuntimeID].(string); got != "runtime-agent" {
		t.Fatalf("expected runtime id in metadata, got %q", got)
	}
	if got, _ := sess.Metadata[execenv.MetadataTaskID].(string); got != "task-agent" {
		t.Fatalf("expected task id in metadata, got %q", got)
	}

	if err := pm.Reset(sessionID); err != nil {
		t.Fatalf("reset tool session process: %v", err)
	}
}
