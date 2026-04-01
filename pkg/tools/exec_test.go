package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/execenv"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
)

func TestExecToolReportsStreamingFallbackWithoutHandler(t *testing.T) {
	tool := NewExecTool(t.TempDir(), false, ExecConfig{Timeout: 5 * time.Second}, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command":   "printf 'ok'",
		"streaming": true,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "Streaming requested but no streaming handler was attached") {
		t.Fatalf("expected streaming fallback notice, got:\n%s", result)
	}
	if !strings.Contains(result, "ok") {
		t.Fatalf("expected command output in result, got:\n%s", result)
	}
}

type capturePreparer struct {
	last execenv.StartSpec
}

func (c *capturePreparer) Prepare(_ context.Context, spec execenv.StartSpec) (execenv.Prepared, error) {
	c.last = spec
	return execenv.Prepared{
		Workdir: spec.Workdir,
		Env:     append([]string{}, spec.Env...),
		Cleanup: func() error { return nil },
	}, nil
}

func TestExecToolBackgroundPassesRuntimeMetadataToProcessSpec(t *testing.T) {
	log := newExecTestLogger(t)
	pm := process.NewManager(log)
	preparer := &capturePreparer{}
	pm.SetPreparer(preparer)
	tool := NewExecTool(t.TempDir(), false, ExecConfig{Timeout: 5 * time.Second}, pm)

	ctx := context.WithValue(context.Background(), execenv.MetadataRuntimeID, "runtime-exec")
	ctx = context.WithValue(ctx, execenv.MetadataTaskID, "task-exec")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"command":    "sleep 5",
		"background": true,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "Background process started") {
		t.Fatalf("expected background start message, got:\n%s", result)
	}
	if preparer.last.RuntimeID != "runtime-exec" {
		t.Fatalf("expected runtime id to propagate, got %q", preparer.last.RuntimeID)
	}
	if preparer.last.TaskID != "task-exec" {
		t.Fatalf("expected task id to propagate, got %q", preparer.last.TaskID)
	}
	if preparer.last.SessionID == "" {
		t.Fatal("expected generated session id")
	}
	if err := pm.Reset(preparer.last.SessionID); err != nil {
		t.Fatalf("reset background process: %v", err)
	}
}

func newExecTestLogger(t *testing.T) *logger.Logger {
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
