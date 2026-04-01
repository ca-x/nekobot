package execenv

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPreparerPrepareNormalizesWorkdirAndCreatesDirectory(t *testing.T) {
	t.Setenv("NEKOBOT_EXECENV_TESTROOT", t.TempDir())
	preparer := NewDefaultPreparer()

	prepared, err := preparer.Prepare(context.Background(), StartSpec{
		SessionID: "sess-1",
		Workdir:   "$NEKOBOT_EXECENV_TESTROOT/subdir/work",
		Env:       []string{"TERM=dumb"},
	})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if !filepath.IsAbs(prepared.Workdir) {
		t.Fatalf("expected absolute workdir, got %q", prepared.Workdir)
	}
	if filepath.Base(prepared.Workdir) != "work" {
		t.Fatalf("unexpected workdir: %q", prepared.Workdir)
	}
	info, err := os.Stat(prepared.Workdir)
	if err != nil {
		t.Fatalf("stat prepared workdir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected prepared workdir directory, got file: %q", prepared.Workdir)
	}
	if got := getEnvValue(prepared.Env, EnvSessionID); got != "sess-1" {
		t.Fatalf("expected session env, got %q", got)
	}
	if got := getEnvValue(prepared.Env, "TERM"); got != "xterm-256color" {
		t.Fatalf("expected TERM override, got %q", got)
	}
	if got := getEnvValue(prepared.Env, "COLORTERM"); got != "truecolor" {
		t.Fatalf("expected COLORTERM default, got %q", got)
	}
}

func TestDefaultPreparerPrepareInjectsRuntimeAndTaskMetadata(t *testing.T) {
	preparer := NewDefaultPreparer()
	prepared, err := preparer.Prepare(context.Background(), StartSpec{
		SessionID: "sess-2",
		RuntimeID: "runtime-a",
		TaskID:    "task-7",
		Env: []string{
			"TERM=screen-256color",
			"COLORTERM=24bit",
			"NEKOBOT_RUNTIME_ID=stale",
		},
	})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if got := getEnvValue(prepared.Env, EnvExecenv); got != "default" {
		t.Fatalf("expected execenv marker, got %q", got)
	}
	if got := getEnvValue(prepared.Env, EnvRuntimeID); got != "runtime-a" {
		t.Fatalf("expected runtime env, got %q", got)
	}
	if got := getEnvValue(prepared.Env, EnvTaskID); got != "task-7" {
		t.Fatalf("expected task env, got %q", got)
	}
	if got := getEnvValue(prepared.Env, "TERM"); got != "screen-256color" {
		t.Fatalf("expected existing TERM to be preserved, got %q", got)
	}
	if got := getEnvValue(prepared.Env, "COLORTERM"); got != "24bit" {
		t.Fatalf("expected existing COLORTERM to be preserved, got %q", got)
	}
}

func TestStartSpecFromContextPrefersContextAndFallsBackToMetadata(t *testing.T) {
	ctx := context.WithValue(context.Background(), MetadataRuntimeID, "runtime-from-context")
	ctx = context.WithValue(ctx, MetadataTaskID, "task-from-context")

	spec := StartSpecFromContext(ctx, "sess-ctx", "echo hi", "/tmp/work", map[string]any{
		MetadataRuntimeID: "runtime-from-metadata",
		MetadataTaskID:    "task-from-metadata",
	})
	if spec.SessionID != "sess-ctx" {
		t.Fatalf("expected session id, got %q", spec.SessionID)
	}
	if spec.RuntimeID != "runtime-from-context" {
		t.Fatalf("expected runtime id from context, got %q", spec.RuntimeID)
	}
	if spec.TaskID != "task-from-context" {
		t.Fatalf("expected task id from context, got %q", spec.TaskID)
	}
	if len(spec.Env) == 0 {
		t.Fatal("expected inherited environment")
	}

	fallback := StartSpecFromContext(context.Background(), "sess-meta", "echo hi", "/tmp/work", map[string]any{
		MetadataRuntimeID: "runtime-from-metadata",
		MetadataTaskID:    "task-from-metadata",
	})
	if fallback.RuntimeID != "runtime-from-metadata" {
		t.Fatalf("expected runtime id from metadata, got %q", fallback.RuntimeID)
	}
	if fallback.TaskID != "task-from-metadata" {
		t.Fatalf("expected task id from metadata, got %q", fallback.TaskID)
	}
}
