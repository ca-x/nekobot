package idempotency

import (
	"context"
	"testing"
	"time"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
)

func newTestStore(t *testing.T) (*Store, *ent.Client) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	_ = log
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		t.Fatalf("ensure runtime schema: %v", err)
	}
	return NewStore(client), client
}

func testKey() Key {
	return Key{
		TenantID:   "t1",
		CallerKind: "agent",
		CallerID:   "agent-1",
		Method:     "SendMessage",
		RequestID:  "req-abc-123",
	}
}

func TestReserveNewRecord(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	result, err := store.Reserve(ctx, testKey(), "hash-1", 7*24*time.Hour)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if result.Outcome != OutcomeReserved {
		t.Errorf("outcome = %q, want reserved", result.Outcome)
	}
	if result.Record == nil {
		t.Fatal("expected record")
	}
	if result.Record.Status != StatusPending {
		t.Errorf("status = %q, want pending", result.Record.Status)
	}
}

func TestReserveReplaySucceeded(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	key := testKey()

	// First reserve
	r1, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve 1: %v", err)
	}
	if r1.Outcome != OutcomeReserved {
		t.Fatalf("expected reserved, got %v", r1.Outcome)
	}

	// Complete
	_, err = store.Complete(ctx, key, CompleteRequest{
		ResponseType: "json:result",
		ResponseJSON: `{"ok":true}`,
		ResourceKind: "message",
		ResourceID:   "msg-1",
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	// Replay — same hash
	r2, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve 2: %v", err)
	}
	if r2.Outcome != OutcomeReplay {
		t.Errorf("outcome = %q, want replay", r2.Outcome)
	}
	if r2.Record.ResponseJSON != `{"ok":true}` {
		t.Errorf("response_json = %q", r2.Record.ResponseJSON)
	}
	if r2.Record.ResourceID != "msg-1" {
		t.Errorf("resource_id = %q", r2.Record.ResourceID)
	}
}

func TestReserveReplayFailed(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	key := testKey()

	r1, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if r1.Outcome != OutcomeReserved {
		t.Fatalf("expected reserved, got %v", r1.Outcome)
	}

	_, err = store.Fail(ctx, key, FailRequest{
		ErrorCode:    "INVALID_ARG",
		ErrorMessage: "bad request",
	})
	if err != nil {
		t.Fatalf("fail: %v", err)
	}

	// Replay failed
	r2, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve 2: %v", err)
	}
	if r2.Outcome != OutcomeReplay {
		t.Errorf("outcome = %q, want replay", r2.Outcome)
	}
	if r2.Record.ErrorCode != "INVALID_ARG" {
		t.Errorf("error_code = %q", r2.Record.ErrorCode)
	}
}

func TestReserveConflict(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	key := testKey()

	// First reserve with hash-1
	r1, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve 1: %v", err)
	}
	if r1.Outcome != OutcomeReserved {
		t.Fatalf("expected reserved, got %v", r1.Outcome)
	}

	// Different hash → conflict
	r2, err := store.Reserve(ctx, key, "hash-DIFFERENT", time.Hour)
	if err != nil {
		t.Fatalf("reserve 2: %v", err)
	}
	if r2.Outcome != OutcomeConflict {
		t.Errorf("outcome = %q, want conflict", r2.Outcome)
	}
	if r2.ExistingHash != "hash-1" {
		t.Errorf("existing_hash = %q, want hash-1", r2.ExistingHash)
	}
}

func TestReserveInProgress(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	key := testKey()

	// First reserve (pending)
	r1, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve 1: %v", err)
	}
	if r1.Outcome != OutcomeReserved {
		t.Fatalf("expected reserved, got %v", r1.Outcome)
	}

	// Same request, still pending → in_progress
	r2, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve 2: %v", err)
	}
	if r2.Outcome != OutcomeInProgress {
		t.Errorf("outcome = %q, want in_progress", r2.Outcome)
	}
}

func TestCheckNonExistent(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	result, err := store.Check(ctx, testKey())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if result.Outcome != OutcomeReserved {
		t.Errorf("outcome = %q, want reserved (no record)", result.Outcome)
	}
}

func TestCheckAfterComplete(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	key := testKey()

	_, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	_, err = store.Complete(ctx, key, CompleteRequest{
		ResponseType: "json:ok",
		ResponseJSON: `{"id":"msg-1"}`,
		ResourceKind: "message",
		ResourceID:   "msg-1",
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	result, err := store.Check(ctx, key)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if result.Outcome != OutcomeReplay {
		t.Errorf("outcome = %q, want replay", result.Outcome)
	}
	if result.Record.ResourceID != "msg-1" {
		t.Errorf("resource_id = %q", result.Record.ResourceID)
	}
}

func TestCompleteNoopOnTerminal(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	key := testKey()

	_, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	_, err = store.Complete(ctx, key, CompleteRequest{
		ResponseType: "json:first",
		ResponseJSON: `{"first":true}`,
	})
	if err != nil {
		t.Fatalf("complete 1: %v", err)
	}

	// Second complete should be a no-op
	rec, err := store.Complete(ctx, key, CompleteRequest{
		ResponseType: "json:second",
		ResponseJSON: `{"second":true}`,
	})
	if err != nil {
		t.Fatalf("complete 2: %v", err)
	}
	if rec.ResponseJSON != `{"first":true}` {
		t.Errorf("response should still be first result, got %q", rec.ResponseJSON)
	}
}

func TestValidateKeyMissingFields(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	_, err := store.Reserve(ctx, Key{Method: "SendMessage"}, "hash", time.Hour)
	if err == nil {
		t.Fatal("expected error for missing request_id")
	}

	_, err = store.Reserve(ctx, Key{RequestID: "req-1"}, "hash", time.Hour)
	if err == nil {
		t.Fatal("expected error for missing method")
	}

	_, err = store.Reserve(ctx, Key{Method: "SendMessage", RequestID: "req-1"}, "hash", time.Hour)
	if err == nil {
		t.Fatal("expected error for missing caller_kind")
	}
}

func TestHashDeterministic(t *testing.T) {
	h1 := Hash([]byte(`{"target":"#ops","content":"hello"}`))
	h2 := Hash([]byte(`{"target":"#ops","content":"hello"}`))
	h3 := Hash([]byte(`{"target":"#ops","content":"world"}`))

	if h1 != h2 {
		t.Errorf("same input should produce same hash: %s != %s", h1, h2)
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
}

func TestMultipleMethodsIndependent(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	key1 := Key{TenantID: "t1", CallerKind: "agent", CallerID: "a1", Method: "SendMessage", RequestID: "req-1"}
	key2 := Key{TenantID: "t1", CallerKind: "agent", CallerID: "a1", Method: "AppendRunStep", RequestID: "req-1"}

	// Same request_id but different methods are independent
	r1, err := store.Reserve(ctx, key1, "hash-a", time.Hour)
	if err != nil {
		t.Fatalf("reserve 1: %v", err)
	}
	r2, err := store.Reserve(ctx, key2, "hash-b", time.Hour)
	if err != nil {
		t.Fatalf("reserve 2: %v", err)
	}

	if r1.Outcome != OutcomeReserved || r2.Outcome != OutcomeReserved {
		t.Errorf("both should be reserved: r1=%v r2=%v", r1.Outcome, r2.Outcome)
	}
}

// Regression: Reserve→Fail must not leave the record permanently in_progress.
// After Fail, a retry with the same request_id should get OutcomeReplay (failed record).
func TestReserveThenFailAllowsReplay(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	key := testKey()

	r1, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if r1.Outcome != OutcomeReserved {
		t.Fatalf("expected reserved, got %v", r1.Outcome)
	}

	// Simulate mutation failure.
	_, err = store.Fail(ctx, key, FailRequest{
		ErrorCode:    "MUTATION_FAILED",
		ErrorMessage: "save thread: disk full",
	})
	if err != nil {
		t.Fatalf("fail: %v", err)
	}

	// Retry with same request_id — should get replay of the failed record, not in_progress.
	r2, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve 2: %v", err)
	}
	if r2.Outcome != OutcomeReplay {
		t.Errorf("outcome = %q, want replay (not in_progress)", r2.Outcome)
	}
	if r2.Record.ErrorCode != "MUTATION_FAILED" {
		t.Errorf("error_code = %q, want MUTATION_FAILED", r2.Record.ErrorCode)
	}
	if r2.Record.ErrorMessage != "save thread: disk full" {
		t.Errorf("error_message = %q", r2.Record.ErrorMessage)
	}
}

// Regression: Complete must cache the full response so replay returns it.
func TestCompleteCachesResponseForReplay(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	key := testKey()

	r1, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if r1.Outcome != OutcomeReserved {
		t.Fatalf("expected reserved, got %v", r1.Outcome)
	}

	// Complete with full response payload (simulating AppendRunStep response).
	completeResp := `{"accepted":true,"step":{"step_id":"step-abc","run_id":"run-1","sequence":3,"kind":"tool_call","status":"completed"}}`
	_, err = store.Complete(ctx, key, CompleteRequest{
		ResponseType: "json:AppendRunStepResponse",
		ResponseJSON: completeResp,
		ResourceKind: "run_step",
		ResourceID:   "step-abc",
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	// Replay — should return the cached response with full Step.
	r2, err := store.Reserve(ctx, key, "hash-1", time.Hour)
	if err != nil {
		t.Fatalf("reserve 2: %v", err)
	}
	if r2.Outcome != OutcomeReplay {
		t.Errorf("outcome = %q, want replay", r2.Outcome)
	}
	if r2.Record.ResponseJSON != completeResp {
		t.Errorf("response_json = %q, want full cached response", r2.Record.ResponseJSON)
	}
	if r2.Record.ResourceID != "step-abc" {
		t.Errorf("resource_id = %q, want step-abc", r2.Record.ResourceID)
	}
}
