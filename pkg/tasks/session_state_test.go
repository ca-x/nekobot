package tasks

import "testing"

func TestStoreSessionStateLifecycle(t *testing.T) {
	store := NewStore()

	store.SetSessionPermissionMode("sess-1", "manual")
	states := store.ListSessionStates()
	if len(states) != 1 {
		t.Fatalf("expected one session state, got %d", len(states))
	}
	if states[0].SessionID != "sess-1" {
		t.Fatalf("expected session id sess-1, got %q", states[0].SessionID)
	}
	if states[0].PermissionMode != "manual" {
		t.Fatalf("expected permission mode manual, got %q", states[0].PermissionMode)
	}
	if states[0].LifecycleState != SessionLifecycleIdle {
		t.Fatalf("expected idle lifecycle state, got %q", states[0].LifecycleState)
	}
	if states[0].PendingAction != "" {
		t.Fatalf("expected empty pending action, got %q", states[0].PendingAction)
	}

	store.SetSessionPendingAction("sess-1", "approve exec", "approval-1")
	states = store.ListSessionStates()
	if len(states) != 1 {
		t.Fatalf("expected one session state after pending action, got %d", len(states))
	}
	if states[0].PendingAction != "approve exec" {
		t.Fatalf("expected pending action to be tracked, got %q", states[0].PendingAction)
	}
	if states[0].PendingRequestID != "approval-1" {
		t.Fatalf("expected request id approval-1, got %q", states[0].PendingRequestID)
	}
	if states[0].LifecycleState != SessionLifecycleAwaitingInput {
		t.Fatalf("expected awaiting_input lifecycle, got %q", states[0].LifecycleState)
	}
	if states[0].LifecycleDetail != "approve exec" {
		t.Fatalf("expected lifecycle detail to track pending action, got %q", states[0].LifecycleDetail)
	}

	store.ClearSessionPendingAction("sess-1")
	states = store.ListSessionStates()
	if len(states) != 1 {
		t.Fatalf("expected session state to remain while permission mode exists, got %d", len(states))
	}
	if states[0].PendingAction != "" || states[0].PendingRequestID != "" {
		t.Fatalf("expected pending action to be cleared, got %+v", states[0])
	}
	if states[0].LifecycleState != SessionLifecycleIdle {
		t.Fatalf("expected idle lifecycle after clearing pending action, got %q", states[0].LifecycleState)
	}

	store.ClearSessionPermissionMode("sess-1")
	if states := store.ListSessionStates(); len(states) != 0 {
		t.Fatalf("expected session state to be removed, got %+v", states)
	}
}

func TestStoreSessionStatesSortMostRecentFirst(t *testing.T) {
	store := NewStore()
	store.SetSessionPermissionMode("sess-a", "manual")
	store.SetSessionPermissionMode("sess-b", "auto")

	states := store.ListSessionStates()
	if len(states) != 2 {
		t.Fatalf("expected two session states, got %d", len(states))
	}
	if states[0].SessionID != "sess-b" {
		t.Fatalf("expected most recent session first, got %q", states[0].SessionID)
	}
}

func TestStoreGetSessionState(t *testing.T) {
	store := NewStore()
	store.SetSessionPermissionMode("sess-1", "manual")
	store.SetSessionPendingAction("sess-1", "approve exec", "approval-1")

	state, ok := store.GetSessionState("sess-1")
	if !ok {
		t.Fatal("expected session state lookup to succeed")
	}
	if state.SessionID != "sess-1" {
		t.Fatalf("expected session id sess-1, got %q", state.SessionID)
	}
	if state.PermissionMode != "manual" {
		t.Fatalf("expected permission mode manual, got %q", state.PermissionMode)
	}
	if state.PendingRequestID != "approval-1" {
		t.Fatalf("expected pending request id approval-1, got %q", state.PendingRequestID)
	}
	if state.LifecycleState != SessionLifecycleAwaitingInput {
		t.Fatalf("expected awaiting_input lifecycle, got %q", state.LifecycleState)
	}

	if _, ok := store.GetSessionState("missing"); ok {
		t.Fatal("expected missing session state lookup to fail")
	}
}

func TestStoreExplicitSessionLifecycleState(t *testing.T) {
	store := NewStore()
	store.SetSessionLifecycleState("sess-1", SessionLifecycleProcessing, "executing tool")

	state, ok := store.GetSessionState("sess-1")
	if !ok {
		t.Fatal("expected session state lookup to succeed")
	}
	if state.LifecycleState != SessionLifecycleProcessing {
		t.Fatalf("expected processing lifecycle, got %q", state.LifecycleState)
	}
	if state.LifecycleDetail != "executing tool" {
		t.Fatalf("expected lifecycle detail, got %q", state.LifecycleDetail)
	}

	store.ClearSessionLifecycleState("sess-1")
	if _, ok := store.GetSessionState("sess-1"); ok {
		t.Fatal("expected explicit lifecycle-only state to be cleared")
	}
}

func TestStoreSessionUsageTracking(t *testing.T) {
	store := NewStore()
	store.SetSessionToolRoundLimit("sess-1", 5)
	store.RecordSessionToolRound("sess-1")
	store.RecordSessionToolCall("sess-1", "shell")
	store.RecordSessionToolCall("sess-1", "shell")
	store.RecordSessionToolCall("sess-1", "read_file")

	state, ok := store.GetSessionState("sess-1")
	if !ok {
		t.Fatal("expected session state lookup to succeed")
	}
	if state.MaxToolRounds != 5 {
		t.Fatalf("expected max tool rounds 5, got %d", state.MaxToolRounds)
	}
	if state.ToolRounds != 1 {
		t.Fatalf("expected tool rounds 1, got %d", state.ToolRounds)
	}
	if state.ToolCalls["shell"] != 2 || state.ToolCalls["read_file"] != 1 {
		t.Fatalf("unexpected tool call counts %+v", state.ToolCalls)
	}
}

func TestStoreCanStartSessionToolRound(t *testing.T) {
	store := NewStore()
	store.SetSessionToolRoundLimit("sess-1", 2)
	if !store.CanStartSessionToolRound("sess-1") {
		t.Fatal("expected first round to be allowed")
	}
	store.RecordSessionToolRound("sess-1")
	if !store.CanStartSessionToolRound("sess-1") {
		t.Fatal("expected second round to be allowed")
	}
	store.RecordSessionToolRound("sess-1")
	if store.CanStartSessionToolRound("sess-1") {
		t.Fatal("expected additional rounds to be blocked after reaching limit")
	}
}

func TestStorePerToolLimits(t *testing.T) {
	store := NewStore()
	store.SetSessionToolCallLimit("sess-1", "shell", 2)
	if !store.CanExecuteSessionToolCall("sess-1", "shell") {
		t.Fatal("expected first shell call to be allowed")
	}
	store.RecordSessionToolCall("sess-1", "shell")
	if !store.CanExecuteSessionToolCall("sess-1", "shell") {
		t.Fatal("expected second shell call to be allowed")
	}
	store.RecordSessionToolCall("sess-1", "shell")
	if store.CanExecuteSessionToolCall("sess-1", "shell") {
		t.Fatal("expected third shell call to be blocked")
	}
	if !store.CanExecuteSessionToolCall("sess-1", "read_file") {
		t.Fatal("expected other tools to remain allowed")
	}
}
