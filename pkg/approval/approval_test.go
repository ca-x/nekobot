package approval

import (
	"fmt"
	"testing"
)

func TestAutoMode(t *testing.T) {
	mgr := NewManager(Config{Mode: ModeAuto})

	decision, _, err := mgr.CheckApproval("exec", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if decision != Approved {
		t.Fatalf("expected Approved, got %s", decision)
	}
}

func TestDenylistBlocksBeforeMode(t *testing.T) {
	mgr := NewManager(Config{
		Mode:     ModeAuto,
		Denylist: []string{"exec"},
	})

	decision, _, err := mgr.CheckApproval("exec", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if decision != Denied {
		t.Fatalf("expected Denied, got %s", decision)
	}
}

func TestAllowlistBypassesManualMode(t *testing.T) {
	mgr := NewManager(Config{
		Mode:      ModeManual,
		Allowlist: []string{"read_file"},
	})

	decision, _, err := mgr.CheckApproval("read_file", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if decision != Approved {
		t.Fatalf("expected Approved, got %s", decision)
	}
}

func TestWildcardAllowlist(t *testing.T) {
	mgr := NewManager(Config{
		Mode:      ModeManual,
		Allowlist: []string{"*"},
	})

	decision, _, err := mgr.CheckApproval("anything", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if decision != Approved {
		t.Fatalf("expected Approved, got %s", decision)
	}
}

func TestManualModeQueuesPending(t *testing.T) {
	mgr := NewManager(Config{Mode: ModeManual})

	decision, id, err := mgr.CheckApproval("exec", map[string]interface{}{"cmd": "ls"}, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if decision != Pending {
		t.Fatalf("expected Pending, got %s", decision)
	}
	if id == "" {
		t.Fatal("expected non-empty request ID")
	}

	// Verify it's in pending list
	pending := mgr.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].ToolName != "exec" {
		t.Fatalf("expected tool_name exec, got %s", pending[0].ToolName)
	}
}

func TestApproveAndDeny(t *testing.T) {
	mgr := NewManager(Config{Mode: ModeManual})

	// Queue two requests
	_, id1, _ := mgr.CheckApproval("exec", nil, "")
	_, id2, _ := mgr.CheckApproval("write_file", nil, "")

	// Approve first
	if err := mgr.Approve(id1); err != nil {
		t.Fatal(err)
	}
	d1, _ := mgr.GetDecision(id1)
	if d1 != Approved {
		t.Fatalf("expected Approved, got %s", d1)
	}

	// Deny second
	if err := mgr.Deny(id2, "not allowed"); err != nil {
		t.Fatal(err)
	}
	d2, _ := mgr.GetDecision(id2)
	if d2 != Denied {
		t.Fatalf("expected Denied, got %s", d2)
	}

	// Pending should be empty now
	pending := mgr.GetPending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending, got %d", len(pending))
	}
}

func TestApproveNonExistent(t *testing.T) {
	mgr := NewManager(Config{Mode: ModeManual})

	if err := mgr.Approve("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent request")
	}
}

func TestCleanup(t *testing.T) {
	mgr := NewManager(Config{Mode: ModeManual})

	_, id1, _ := mgr.CheckApproval("exec", nil, "")
	_, _, _ = mgr.CheckApproval("read_file", nil, "")

	// Approve first, leave second pending
	mgr.Approve(id1)

	mgr.Cleanup()

	// Only pending one should remain
	pending := mgr.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending after cleanup, got %d", len(pending))
	}
}

func TestPromptModeApproved(t *testing.T) {
	mgr := NewManager(Config{Mode: ModePrompt})
	mgr.PromptFunc = func(req *Request) (bool, error) {
		return true, nil
	}

	decision, _, err := mgr.CheckApproval("exec", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if decision != Approved {
		t.Fatalf("expected Approved, got %s", decision)
	}
}

func TestPromptModeDenied(t *testing.T) {
	mgr := NewManager(Config{Mode: ModePrompt})
	mgr.PromptFunc = func(req *Request) (bool, error) {
		return false, nil
	}

	decision, _, err := mgr.CheckApproval("exec", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if decision != Denied {
		t.Fatalf("expected Denied, got %s", decision)
	}
}

func TestPromptModeError(t *testing.T) {
	mgr := NewManager(Config{Mode: ModePrompt})
	mgr.PromptFunc = func(req *Request) (bool, error) {
		return false, fmt.Errorf("connection lost")
	}

	decision, _, err := mgr.CheckApproval("exec", nil, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if decision != Denied {
		t.Fatalf("expected Denied on error, got %s", decision)
	}
}

func TestPromptModeNilFunc(t *testing.T) {
	// When PromptFunc is nil, should auto-approve
	mgr := NewManager(Config{Mode: ModePrompt})

	decision, _, err := mgr.CheckApproval("exec", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if decision != Approved {
		t.Fatalf("expected Approved when PromptFunc is nil, got %s", decision)
	}
}
