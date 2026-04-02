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

	store.ClearSessionPendingAction("sess-1")
	states = store.ListSessionStates()
	if len(states) != 1 {
		t.Fatalf("expected session state to remain while permission mode exists, got %d", len(states))
	}
	if states[0].PendingAction != "" || states[0].PendingRequestID != "" {
		t.Fatalf("expected pending action to be cleared, got %+v", states[0])
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
