package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/config"
	"nekobot/pkg/execenv"
	"nekobot/pkg/process"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/tasks"
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
	tool := NewToolSessionTool(pm, mgr, cfg)

	ctx := context.WithValue(context.Background(), execenv.MetadataRuntimeID, "runtime-agent")
	ctx = context.WithValue(ctx, execenv.MetadataTaskID, "task-agent")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":  "spawn",
		"tool":    "custom-tool",
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
	if isTmuxAvailable() {
		if got, _ := sess.Metadata["runtime_transport"].(string); got != "tmux" {
			t.Fatalf("expected runtime transport tmux in metadata, got %q", got)
		}
		if got, _ := sess.Metadata[runtimeagents.MetadataRuntimeSession].(string); strings.TrimSpace(got) == "" {
			t.Fatal("expected runtime session metadata to be populated")
		}
		if got, _ := sess.Metadata["tmux_session"].(string); strings.TrimSpace(got) == "" {
			t.Fatal("expected tmux session metadata to be populated")
		}
		if got, _ := sess.Metadata["launch_cmd"].(string); !strings.Contains(got, "tmux new-session") {
			t.Fatalf("expected launch_cmd to capture tmux wrapper, got %q", got)
		}
	}

	if err := pm.Reset(sessionID); err != nil {
		t.Fatalf("reset tool session process: %v", err)
	}
}

func TestToolSessionToolSpawnCreatesManagedTaskWhenTaskIDMissing(t *testing.T) {
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

	store := tasks.NewStore()
	taskSvc := tasks.NewService(store)
	pm := process.NewManager(log)
	pm.SetTaskService(taskSvc)
	tool := NewToolSessionTool(pm, mgr, cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "spawn",
		"tool":    "custom-tool",
		"command": "/bin/sh -lc 'printf ok'",
	})
	if err != nil {
		t.Fatalf("spawn tool session: %v", err)
	}
	if !strings.Contains(result, "Tool session created successfully") {
		t.Fatalf("unexpected result: %s", result)
	}

	task := waitForBackgroundTaskTerminal(t, store, 3*time.Second)
	if task.ID == "" {
		t.Fatal("expected generated task id")
	}
	if task.ID != task.SessionID {
		t.Fatalf("expected generated task id to match session id, got task=%q session=%q", task.ID, task.SessionID)
	}
	if task.Type != tasks.TypeRuntimeWorker {
		t.Fatalf("expected runtime worker type, got %q", task.Type)
	}

	sess, err := mgr.GetSession(context.Background(), task.SessionID)
	if err != nil {
		t.Fatalf("get created session: %v", err)
	}
	if got, _ := sess.Metadata[execenv.MetadataTaskID].(string); got != task.ID {
		t.Fatalf("expected task id %q in session metadata, got %q", task.ID, got)
	}
}

func TestToolSessionToolSpawnNormalizesSupportedExternalAgentTool(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

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
	tool := NewToolSessionTool(pm, mgr, cfg)
	cfg.Agents.Defaults.RestrictToWorkspace = false
	if err := os.MkdirAll(cfg.WorkspacePath(), 0o755); err != nil {
		t.Fatalf("mkdir workspace root: %v", err)
	}
	installFakeShellCommand(t)
	installFakeToolCommand(t, "codex")
	wantWorkdir := filepath.Join(cfg.WorkspacePath(), "projects", "demo")
	if err := os.MkdirAll(wantWorkdir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "spawn",
		"tool":    "codex",
		"workdir": "projects/demo",
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

	sess, err := mgr.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("get created session: %v", err)
	}
	if sess.Workdir != wantWorkdir {
		t.Fatalf("expected workdir %q, got %q", wantWorkdir, sess.Workdir)
	}
	if sess.Tool != "codex" {
		t.Fatalf("expected codex tool, got %q", sess.Tool)
	}
	if sess.Command != "codex" {
		t.Fatalf("expected codex command, got %q", sess.Command)
	}
}

func TestToolSessionToolSpawnRejectsMismatchedSupportedExternalAgentCommand(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	log := newExecTestLogger(t)
	client := newTestEntClientForTools(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	pm := process.NewManager(log)
	tool := NewToolSessionTool(pm, mgr, cfg)

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action":  "spawn",
		"tool":    "codex",
		"command": "claude --print",
	})
	if err == nil {
		t.Fatal("expected command policy error")
	}
	if !strings.Contains(err.Error(), "command must launch the selected tool") {
		t.Fatalf("expected command policy error, got %v", err)
	}
}

func TestToolSessionToolSpawnRejectsSupportedExternalAgentWorkspaceOutsideRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	log := newExecTestLogger(t)
	client := newTestEntClientForTools(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	pm := process.NewManager(log)
	tool := NewToolSessionTool(pm, mgr, cfg)

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action":  "spawn",
		"tool":    "claude",
		"workdir": filepath.Join(t.TempDir(), "outside"),
	})
	if err == nil {
		t.Fatal("expected workspace restriction error")
	}
	if !strings.Contains(err.Error(), "workspace must stay within configured workspace") {
		t.Fatalf("expected workspace restriction error, got %v", err)
	}
}

func TestToolSessionToolSpawnSupportedExternalAgentAppendsProcessStartedEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")
	cfg.WebUI.ToolSessionEvents.Enabled = true

	log := newExecTestLogger(t)
	client := newTestEntClientForTools(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	mgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}

	pm := process.NewManager(log)
	tool := NewToolSessionTool(pm, mgr, cfg)
	cfg.Agents.Defaults.RestrictToWorkspace = false
	fakeStarter := &fakeProcessStarter{}
	tool.processProbe = fakeStarter
	tool.processStarter = fakeStarter
	tool.runtimeTransport = stubRuntimeTransport{
		launchInfo: runtimeagents.LaunchInfo{
			TransportName: runtimeagents.TransportTmux,
			SessionName:   "neko_test",
			LaunchCommand: "codex --wrapped",
		},
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "spawn",
		"tool":   "codex",
	}); err != nil {
		t.Fatalf("spawn tool session: %v", err)
	}

	sessions, err := mgr.ListSessions(context.Background(), toolsessions.ListSessionsInput{Source: toolsessions.SourceAgent})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one session, got %d", len(sessions))
	}
	sessionID := sessions[0].ID
	if sessionID == "" {
		t.Fatal("expected session id to be captured")
	}
	if fakeStarter.calls != 1 {
		t.Fatalf("expected one process start, got %d", fakeStarter.calls)
	}
	if got := fakeStarter.last.Command; got != "codex --wrapped" {
		t.Fatalf("expected wrapped launch command, got %q", got)
	}

	events, err := mgr.ListEvents(context.Background(), sessionID, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}

	var started *toolsessions.Event
	for _, event := range events {
		if event != nil && event.Type == "process_started" {
			started = event
			break
		}
	}
	if started == nil {
		t.Fatalf("expected process_started event, got %+v", events)
	}
	if got, _ := started.Payload["command"].(string); got != "codex" {
		t.Fatalf("expected process_started command %q, got %q", "codex", got)
	}
	if got, _ := started.Payload["launch_cmd"].(string); got != "codex --wrapped" {
		t.Fatalf("expected wrapped launch command in event, got %q", got)
	}
}

func waitForBackgroundTaskTerminal(t *testing.T, store *tasks.Store, timeout time.Duration) tasks.Task {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		all := store.List()
		if len(all) == 1 && tasks.IsFinal(all[0].State) {
			return all[0]
		}
		time.Sleep(10 * time.Millisecond)
	}

	var states []string
	for _, task := range store.List() {
		states = append(states, fmt.Sprintf("%s=%s", task.ID, task.State))
	}
	t.Fatalf("background task did not reach terminal state before timeout; saw %v", states)
	return tasks.Task{}
}

func installFakeToolCommand(t *testing.T, name string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tool command: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func installFakeShellCommand(t *testing.T) {
	t.Helper()
	installFakeToolCommand(t, "sh")
}

type fakeProcessStarter struct {
	calls int
	last  execenv.StartSpec
}

type stubRuntimeTransport struct {
	launchInfo runtimeagents.LaunchInfo
}

func (s stubRuntimeTransport) Name() string {
	return s.launchInfo.TransportName
}

func (s stubRuntimeTransport) Available() bool {
	return true
}

func (s stubRuntimeTransport) WrapStart(command, sessionID string) runtimeagents.LaunchInfo {
	return s.launchInfo
}

func (s stubRuntimeTransport) BuildReattach(sessionID string) (runtimeagents.ReattachInfo, bool) {
	return runtimeagents.ReattachInfo{}, false
}

func (s stubRuntimeTransport) KillSession(sessionID string) {}

func (f *fakeProcessStarter) HasProcess(string) bool {
	return false
}

func (f *fakeProcessStarter) StartWithSpec(_ context.Context, spec execenv.StartSpec) error {
	f.calls++
	f.last = spec
	return nil
}
