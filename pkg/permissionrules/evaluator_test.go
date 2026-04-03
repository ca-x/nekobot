package permissionrules

import (
	"context"
	"testing"

	"nekobot/pkg/config"
)

func TestEvaluatorMatchesGlobalRule(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.Create(context.Background(), Rule{
		ToolName: "exec",
		Action:   ActionAllow,
		Priority: 10,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	result, err := mgr.Evaluate(context.Background(), Input{ToolName: "exec"})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result.Matched || result.Action != ActionAllow {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Explanation.Source != "rule" {
		t.Fatalf("expected rule explanation, got %+v", result.Explanation)
	}
}

func TestEvaluatorPrefersMoreSpecificScopeOverGlobal(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.Create(context.Background(), Rule{
		ToolName: "exec",
		Action:   ActionAllow,
		Priority: 50,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Create global rule failed: %v", err)
	}
	_, err = mgr.Create(context.Background(), Rule{
		ToolName:  "exec",
		SessionID: "sess-1",
		Action:    ActionDeny,
		Priority:  50,
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("Create session rule failed: %v", err)
	}

	result, err := mgr.Evaluate(context.Background(), Input{
		ToolName:  "exec",
		SessionID: "sess-1",
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if result.Action != ActionDeny {
		t.Fatalf("expected deny from session-scoped rule, got %+v", result)
	}
	if result.Explanation.MatchedScope != "session" {
		t.Fatalf("expected session scope, got %+v", result.Explanation)
	}
}

func TestEvaluatorPrefersHigherPriorityRule(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.Create(context.Background(), Rule{
		ToolName: "exec",
		Action:   ActionAllow,
		Priority: 10,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Create low priority rule failed: %v", err)
	}
	_, err = mgr.Create(context.Background(), Rule{
		ToolName: "exec",
		Action:   ActionAsk,
		Priority: 100,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Create high priority rule failed: %v", err)
	}

	result, err := mgr.Evaluate(context.Background(), Input{ToolName: "exec"})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if result.Action != ActionAsk {
		t.Fatalf("expected higher priority ask rule, got %+v", result)
	}
}

func TestEvaluatorIgnoresDisabledRules(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.Create(context.Background(), Rule{
		ToolName: "exec",
		Action:   ActionDeny,
		Priority: 100,
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("Create disabled rule failed: %v", err)
	}

	result, err := mgr.Evaluate(context.Background(), Input{ToolName: "exec"})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if result.Matched {
		t.Fatalf("expected no match from disabled rule, got %+v", result)
	}
	if result.Explanation.Source != "approval_mode_fallback" {
		t.Fatalf("expected fallback explanation, got %+v", result.Explanation)
	}
}

func TestEvaluatorMatchesRuntimeScopedRule(t *testing.T) {
	mgr := newTestManager(t)
	_, err := mgr.Create(context.Background(), Rule{
		ToolName:  "spawn",
		RuntimeID: "runtime-a",
		Action:    ActionAsk,
		Priority:  10,
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("Create runtime rule failed: %v", err)
	}

	result, err := mgr.Evaluate(context.Background(), Input{
		ToolName:  "spawn",
		RuntimeID: "runtime-a",
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if result.Action != ActionAsk {
		t.Fatalf("expected ask action, got %+v", result)
	}
	if result.Explanation.MatchedScope != "runtime" {
		t.Fatalf("expected runtime scope, got %+v", result.Explanation)
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()

	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	mgr, err := NewManager(cfg, newTestLogger(t), client)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	return mgr
}
