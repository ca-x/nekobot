package externalagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/toolsessions"
)

func TestResolveSessionReusesExistingSession(t *testing.T) {
	mgr, sessionMgr := newTestManager(t)
	ctx := context.Background()

	existing, err := sessionMgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "alice",
		Source:  toolsessions.SourceAgent,
		Tool:    "claude",
		Title:   "Claude Session",
		Command: "claude",
		Workdir: "/tmp/ws-a",
		State:   toolsessions.StateDetached,
		Metadata: map[string]interface{}{
			metadataAgentKind: "claude",
			metadataWorkspace: "/tmp/ws-a",
		},
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "claude",
		Workspace: "/tmp/ws-a",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if created {
		t.Fatal("expected existing session reuse")
	}
	if resolved.ID != existing.ID {
		t.Fatalf("expected existing session %q, got %q", existing.ID, resolved.ID)
	}
}

func TestResolveSessionCreatesNewSessionWhenExistingCommandOrToolDiffers(t *testing.T) {
	mgr, sessionMgr := newTestManager(t)
	ctx := context.Background()

	existing, err := sessionMgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "alice",
		Source:  toolsessions.SourceAgent,
		Tool:    "claude",
		Title:   "Claude Session",
		Command: "claude --old",
		Workdir: "/tmp/ws-a",
		State:   toolsessions.StateDetached,
		Metadata: map[string]interface{}{
			metadataAgentKind: "claude",
			metadataWorkspace: "/tmp/ws-a",
		},
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "claude",
		Workspace: "/tmp/ws-a",
		Tool:      "claude",
		Command:   "claude",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected new session when command differs")
	}
	if resolved.ID == existing.ID {
		t.Fatalf("expected a new session instead of reusing %q", existing.ID)
	}
}

func TestResolveSessionCreatesNewDetachedSession(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
		Workspace: "/tmp/ws-b",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected new session creation")
	}
	if resolved.State != toolsessions.StateDetached {
		t.Fatalf("expected detached state, got %q", resolved.State)
	}
	if got, _ := resolved.Metadata[metadataAgentKind].(string); got != "codex" {
		t.Fatalf("expected agent kind codex, got %q", got)
	}
	if got, _ := resolved.Metadata[metadataWorkspace].(string); got != "/tmp/ws-b" {
		t.Fatalf("expected workspace /tmp/ws-b, got %q", got)
	}
}

func TestResolveSessionDefaultsBlankWorkspaceToConfiguredRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	mgr, _ := newTestManagerWithConfig(t, cfg)
	ctx := context.Background()

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected new session creation")
	}
	if resolved.Workdir != cfg.WorkspacePath() {
		t.Fatalf("expected workdir %q, got %q", cfg.WorkspacePath(), resolved.Workdir)
	}
}

func TestResolveSessionResolvesRelativeWorkspaceWithinConfiguredRoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	mgr, _ := newTestManagerWithConfig(t, cfg)
	ctx := context.Background()

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
		Workspace: "projects/demo",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected new session creation")
	}
	want := filepath.Join(cfg.WorkspacePath(), "projects", "demo")
	if resolved.Workdir != want {
		t.Fatalf("expected workdir %q, got %q", want, resolved.Workdir)
	}
}

func TestResolveSessionRejectsRelativeWorkspaceTraversalEscapeWhenRestricted(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	mgr, _ := newTestManagerWithConfig(t, cfg)
	ctx := context.Background()

	_, _, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
		Workspace: "../outside",
	})
	if err == nil {
		t.Fatal("expected workspace restriction error")
	}
	if !strings.Contains(err.Error(), "workspace must stay within configured workspace") {
		t.Fatalf("expected workspace restriction error, got %v", err)
	}
}

func TestResolveSessionIgnoresNonStringMetadataWithoutPanicking(t *testing.T) {
	mgr, sessionMgr := newTestManager(t)
	ctx := context.Background()

	if _, err := sessionMgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "alice",
		Source:  toolsessions.SourceAgent,
		Tool:    "claude",
		Title:   "Broken Metadata Session",
		Command: "claude",
		Workdir: "/tmp/ws-a",
		State:   toolsessions.StateDetached,
		Metadata: map[string]interface{}{
			metadataAgentKind: 123,
			metadataWorkspace: true,
		},
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "claude",
		Workspace: "/tmp/ws-a",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected broken metadata session not to be reused")
	}
	if resolved == nil || resolved.ID == "" {
		t.Fatal("expected a newly created session")
	}
}

func TestNormalizeSpecRejectsRelativeWorkspace(t *testing.T) {
	_, err := normalizeSpec(SessionSpec{
		Owner:     "alice",
		AgentKind: "claude",
		Workspace: "relative/project",
	})
	if err == nil {
		t.Fatal("expected relative workspace error")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "absolute workspace path is required") {
		t.Fatalf("expected absolute workspace error, got %v", err)
	}
}

func TestNormalizeSpecRejectsWorkspaceTraversalEscape(t *testing.T) {
	_, err := normalizeSpec(SessionSpec{
		Owner:     "alice",
		AgentKind: "claude",
		Workspace: "../project-a",
	})
	if err == nil {
		t.Fatal("expected workspace traversal error")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "absolute workspace path is required") {
		t.Fatalf("expected absolute workspace error, got %v", err)
	}
}

func TestResolveSessionRejectsDifferentToolForSameAgentKind(t *testing.T) {
	mgr, sessionMgr := newTestManager(t)
	ctx := context.Background()

	if _, err := sessionMgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "alice",
		Source:  toolsessions.SourceAgent,
		Tool:    "claude",
		Title:   "Claude Session",
		Command: "claude",
		Workdir: "/tmp/ws-a",
		State:   toolsessions.StateDetached,
		Metadata: map[string]interface{}{
			metadataAgentKind: "claude",
			metadataWorkspace: "/tmp/ws-a",
		},
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "claude",
		Workspace: "/tmp/ws-a",
		Tool:      "codex",
	})
	if err == nil {
		t.Fatal("expected tool mismatch error")
	}
	if resolved != nil || created {
		t.Fatalf("expected no resolved session on tool mismatch, got %+v created=%v", resolved, created)
	}
	if !strings.Contains(err.Error(), "tool must match agent_kind launcher") {
		t.Fatalf("expected tool mismatch error, got %v", err)
	}
}

func TestResolveSessionDoesNotReuseDifferentCommand(t *testing.T) {
	mgr, sessionMgr := newTestManager(t)
	ctx := context.Background()

	if _, err := sessionMgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "alice",
		Source:  toolsessions.SourceAgent,
		Tool:    "claude",
		Title:   "Claude Session",
		Command: "claude --dangerously-skip-permissions",
		Workdir: "/tmp/ws-a",
		State:   toolsessions.StateDetached,
		Metadata: map[string]interface{}{
			metadataAgentKind: "claude",
			metadataWorkspace: "/tmp/ws-a",
		},
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "claude",
		Workspace: "/tmp/ws-a",
		Command:   "claude",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected different command to require a new session")
	}
	if resolved.Command != "claude" {
		t.Fatalf("expected created command claude, got %q", resolved.Command)
	}
}

func TestNormalizeSpecFillsDefaults(t *testing.T) {
	spec, err := normalizeSpec(SessionSpec{
		Owner:     " alice ",
		AgentKind: " Claude ",
		Workspace: "/tmp/project/..//project-a",
	})
	if err != nil {
		t.Fatalf("normalizeSpec failed: %v", err)
	}
	if spec.Owner != "alice" {
		t.Fatalf("expected trimmed owner, got %q", spec.Owner)
	}
	if spec.AgentKind != "claude" {
		t.Fatalf("expected normalized agent kind, got %q", spec.AgentKind)
	}
	if spec.Tool != "claude" {
		t.Fatalf("expected default tool claude, got %q", spec.Tool)
	}
	if spec.Command != "claude" {
		t.Fatalf("expected default command claude, got %q", spec.Command)
	}
	if spec.Title != "Claude Session" {
		t.Fatalf("expected default title, got %q", spec.Title)
	}
	if spec.Workspace != "/tmp/project-a" {
		t.Fatalf("expected cleaned workspace, got %q", spec.Workspace)
	}
}

func TestResolveSessionRejectsWorkspaceOutsideConfiguredRootWhenRestricted(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")

	mgr, _ := newTestManagerWithConfig(t, cfg)
	ctx := context.Background()
	outside := filepath.Join(t.TempDir(), "outside")

	_, _, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
		Workspace: outside,
	})
	if err == nil {
		t.Fatal("expected workspace restriction error")
	}
	if !strings.Contains(err.Error(), "workspace must stay within configured workspace") {
		t.Fatalf("expected workspace restriction error, got %v", err)
	}
}

func TestResolveSessionAllowsWorkspaceOutsideConfiguredRootWhenRestrictionDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace-root")
	cfg.Agents.Defaults.RestrictToWorkspace = false

	mgr, _ := newTestManagerWithConfig(t, cfg)
	ctx := context.Background()
	outside := filepath.Join(t.TempDir(), "outside")

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
		Workspace: outside,
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected new session creation")
	}
	if resolved.Workdir != outside {
		t.Fatalf("expected workdir %q, got %q", outside, resolved.Workdir)
	}
}

func TestResolveSessionRejectsCommandThatDoesNotMatchTool(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	_, _, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
		Workspace: "/tmp/ws-command-policy",
		Tool:      "codex",
		Command:   "claude --print",
	})
	if err == nil {
		t.Fatal("expected command policy error")
	}
	if !strings.Contains(err.Error(), "command must launch the selected tool") {
		t.Fatalf("expected command policy error, got %v", err)
	}
}

func TestResolveSessionAllowsCommandThatMatchesTool(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
		Workspace: "/tmp/ws-command-policy",
		Tool:      "codex",
		Command:   "codex",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected new session creation")
	}
	if resolved.Command != "codex" {
		t.Fatalf("expected command preserved, got %q", resolved.Command)
	}
}

func TestResolveSessionRejectsUnknownAgentKind(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	_, _, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "unknown",
		Workspace: "/tmp/ws-unknown",
	})
	if err == nil {
		t.Fatal("expected unsupported agent kind error")
	}
	if !strings.Contains(err.Error(), "unsupported agent_kind") {
		t.Fatalf("expected unsupported agent kind error, got %v", err)
	}
}

func TestResolveSessionRejectsToolMismatchForKnownAgentKind(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	_, _, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
		Workspace: "/tmp/ws-tool-mismatch",
		Tool:      "claude",
	})
	if err == nil {
		t.Fatal("expected tool mismatch error")
	}
	if !strings.Contains(err.Error(), "tool must match agent_kind launcher") {
		t.Fatalf("expected tool mismatch error, got %v", err)
	}
}

func newTestManager(t *testing.T) (*Manager, *toolsessions.Manager) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = filepath.Clean(os.TempDir())

	return newTestManagerWithConfig(t, cfg)
}

func newTestManagerWithConfig(t *testing.T, cfg *config.Config) (*Manager, *toolsessions.Manager) {
	t.Helper()

	log := newTestLogger(t)
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() { _ = client.Close() })

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	manager, err := NewManager(cfg, sessionMgr)
	if err != nil {
		t.Fatalf("new external agent manager: %v", err)
	}
	return manager, sessionMgr
}

func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	cfg := logger.DefaultConfig()
	cfg.OutputPath = ""
	cfg.Development = true
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	return log
}

func newTestEntClient(t *testing.T, cfg *config.Config) *ent.Client {
	t.Helper()
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		_ = client.Close()
		t.Fatalf("ensure runtime schema: %v", err)
	}
	return client
}

func TestResolveSessionIgnoresIncompleteLaunchIdentity(t *testing.T) {
	mgr, sessionMgr := newTestManager(t)
	ctx := context.Background()

	create := func(tool string, command string, workdir string) {
		t.Helper()
		if _, err := sessionMgr.CreateSession(ctx, toolsessions.CreateSessionInput{
			Owner:   "alice",
			Source:  toolsessions.SourceAgent,
			Tool:    tool,
			Title:   "Broken Session",
			Command: command,
			Workdir: workdir,
			State:   toolsessions.StateDetached,
			Metadata: map[string]interface{}{
				metadataAgentKind: "claude",
				metadataWorkspace: "/tmp/ws-a",
			},
		}); err != nil {
			t.Fatalf("create session: %v", err)
		}
	}

	create("", "claude", "/tmp/ws-a")
	create("claude", "", "/tmp/ws-a")
	create("claude", "claude", "")

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "claude",
		Workspace: "/tmp/ws-a",
		Tool:      "claude",
		Command:   "claude",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected incomplete launch identity sessions not to be reused")
	}
	if resolved.Tool != "claude" || resolved.Command != "claude" || resolved.Workdir != "/tmp/ws-a" {
		t.Fatalf("expected a clean newly created session, got %+v", resolved)
	}
}

func TestResolveSessionCreatesNormalizedLaunchMetadata(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: " Claude ",
		Workspace: "/tmp/ws-normalized",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected new session creation")
	}
	if resolved.Tool != "claude" {
		t.Fatalf("expected normalized default tool claude, got %q", resolved.Tool)
	}
	if resolved.Command != "claude" {
		t.Fatalf("expected normalized default command claude, got %q", resolved.Command)
	}
	if resolved.Title != "Claude Session" {
		t.Fatalf("expected normalized default title, got %q", resolved.Title)
	}
	if got, _ := resolved.Metadata[metadataAgentKind].(string); got != "claude" {
		t.Fatalf("expected normalized metadata agent kind, got %q", got)
	}
}

func TestResolveSessionDoesNotReuseDifferentOwner(t *testing.T) {
	mgr, sessionMgr := newTestManager(t)
	ctx := context.Background()

	if _, err := sessionMgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "alice",
		Source:  toolsessions.SourceAgent,
		Tool:    "claude",
		Title:   "Claude Session",
		Command: "claude",
		Workdir: "/tmp/ws-a",
		State:   toolsessions.StateDetached,
		Metadata: map[string]interface{}{
			metadataAgentKind: "claude",
			metadataWorkspace: "/tmp/ws-a",
		},
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "bob",
		AgentKind: "claude",
		Workspace: "/tmp/ws-a",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected different owner to require a new session")
	}
	if resolved.Owner != "bob" {
		t.Fatalf("expected new session owner bob, got %q", resolved.Owner)
	}
}

func TestResolveSessionPersistsLaunchIdentityMetadata(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "codex",
		Workspace: "/tmp/ws-metadata",
		Tool:      "codex",
		Command:   "codex",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected new session creation")
	}
	if got, _ := resolved.Metadata[metadataTool].(string); got != "codex" {
		t.Fatalf("expected metadata tool codex, got %q", got)
	}
	if got, _ := resolved.Metadata[metadataCommand].(string); got != "codex" {
		t.Fatalf("expected metadata command persisted, got %q", got)
	}
}

func TestResolveSessionDoesNotReuseWhenLaunchMetadataDisagrees(t *testing.T) {
	mgr, sessionMgr := newTestManager(t)
	ctx := context.Background()

	existing, err := sessionMgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:   "alice",
		Source:  toolsessions.SourceAgent,
		Tool:    "claude",
		Title:   "Claude Session",
		Command: "claude",
		Workdir: "/tmp/ws-a",
		State:   toolsessions.StateDetached,
		Metadata: map[string]interface{}{
			metadataAgentKind: "claude",
			metadataWorkspace: "/tmp/ws-a",
			metadataTool:      "claude",
			metadataCommand:   "claude --old",
		},
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	resolved, created, err := mgr.ResolveSession(ctx, SessionSpec{
		Owner:     "alice",
		AgentKind: "claude",
		Workspace: "/tmp/ws-a",
		Tool:      "claude",
		Command:   "claude",
	})
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if !created {
		t.Fatal("expected mismatched launch metadata to require a new session")
	}
	if resolved.ID == existing.ID {
		t.Fatalf("expected a new session instead of reusing %q", existing.ID)
	}
}
