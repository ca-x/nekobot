package process

import (
	"context"
	"sync/atomic"
	"testing"

	"nekobot/pkg/execenv"
	"nekobot/pkg/logger"
)

type stubPreparer struct {
	prepared execenv.Prepared
	called   atomic.Int32
}

func (s *stubPreparer) Prepare(_ context.Context, spec execenv.StartSpec) (execenv.Prepared, error) {
	s.called.Add(1)
	s.prepared.Env = append([]string{}, s.prepared.Env...)
	s.prepared.Env = append(s.prepared.Env, execenv.EnvSessionID+"="+spec.SessionID)
	return s.prepared, nil
}

func TestManagerStartWithSpecUsesPreparerAndRunsCleanupOnReset(t *testing.T) {
	log := newTestLogger(t)
	mgr := NewManager(log)
	cleanupCalled := atomic.Int32{}
	workdir := t.TempDir()
	mgr.SetPreparer(&stubPreparer{prepared: execenv.Prepared{
		Workdir: workdir,
		Env:     []string{"TERM=xterm-256color"},
		Cleanup: func() error {
			cleanupCalled.Add(1)
			return nil
		},
	}})

	err := mgr.StartWithSpec(context.Background(), execenv.StartSpec{
		SessionID: "sess-prepared",
		Command:   "sleep 30",
		Workdir:   "/ignored",
		TaskID:    "task-prepared",
	})
	if err != nil {
		t.Fatalf("StartWithSpec failed: %v", err)
	}

	status, err := mgr.GetStatus("sess-prepared")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Workdir != workdir {
		t.Fatalf("expected prepared workdir %q, got %q", workdir, status.Workdir)
	}

	if err := mgr.Reset("sess-prepared"); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	if cleanupCalled.Load() != 1 {
		t.Fatalf("expected cleanup to run exactly once, got %d", cleanupCalled.Load())
	}
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
