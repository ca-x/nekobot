package events

import (
	"context"
	"errors"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/storage/ent"
)

func newTestManager(t *testing.T) (*Manager, *ent.Client) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		t.Fatalf("ensure runtime schema: %v", err)
	}
	mgr, err := NewManager(client)
	if err != nil {
		t.Fatalf("new event manager: %v", err)
	}
	return mgr, client
}

func TestAppendAssignsMonotonicSequencePerTenantStream(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	first, err := mgr.Append(ctx, EventRecord{EventType: "message.created", Target: "#ops"})
	if err != nil {
		t.Fatalf("append first: %v", err)
	}
	second, err := mgr.Append(ctx, EventRecord{EventType: "run.step_appended", Target: "#ops"})
	if err != nil {
		t.Fatalf("append second: %v", err)
	}
	otherStream, err := mgr.Append(ctx, EventRecord{Stream: "tenant:other", EventType: "message.created"})
	if err != nil {
		t.Fatalf("append other stream: %v", err)
	}

	if first.Sequence != 1 || second.Sequence != 2 {
		t.Fatalf("default stream sequences = %d,%d; want 1,2", first.Sequence, second.Sequence)
	}
	if otherStream.Sequence != 1 {
		t.Fatalf("other stream sequence = %d, want 1", otherStream.Sequence)
	}
	if first.EventID == "" {
		t.Fatal("expected generated event_id")
	}
}

func TestListSinceUsesOpaqueCursorAndTargetFilter(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	for _, item := range []EventRecord{
		{EventType: "message.created", Target: "#ops", SubjectKind: "message", SubjectID: "m1"},
		{EventType: "message.created", Target: "#alerts", SubjectKind: "message", SubjectID: "m2"},
		{EventType: "run.step_appended", Target: "#ops", SubjectKind: "run_step", SubjectID: "s1"},
	} {
		if _, err := mgr.Append(ctx, item); err != nil {
			t.Fatalf("append %s: %v", item.SubjectID, err)
		}
	}

	filter := ListFilter{Target: "#ops"}
	events, cursor, err := mgr.ListSince(ctx, "", filter, 1)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(events) != 1 || events[0].SubjectID != "m1" {
		t.Fatalf("first page = %+v, want only m1", events)
	}
	if cursor == "" {
		t.Fatal("expected non-empty opaque cursor")
	}

	events, nextCursor, err := mgr.ListSince(ctx, cursor, filter, 10)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(events) != 1 || events[0].SubjectID != "s1" {
		t.Fatalf("second page = %+v, want only s1", events)
	}
	if nextCursor == cursor {
		t.Fatal("expected cursor to advance after second page")
	}

	events, _, err = mgr.ListSince(ctx, nextCursor, filter, 10)
	if err != nil {
		t.Fatalf("list empty page: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("empty page len = %d, want 0", len(events))
	}
}

func TestListSinceRejectsCursorFilterMismatch(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	if _, err := mgr.Append(ctx, EventRecord{EventType: "message.created", Target: "#ops"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	_, cursor, err := mgr.ListSince(ctx, "", ListFilter{Target: "#ops"}, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	_, _, err = mgr.ListSince(ctx, cursor, ListFilter{Target: "#alerts"}, 10)
	if !errors.Is(err, ErrCursorFilterMismatch) {
		t.Fatalf("err = %v, want ErrCursorFilterMismatch", err)
	}
}

func TestListSinceFiltersEventTypes(t *testing.T) {
	mgr, _ := newTestManager(t)
	ctx := context.Background()

	for _, item := range []EventRecord{
		{EventType: "message.created", Target: "#ops", SubjectID: "m1"},
		{EventType: "activity.logged", Target: "#ops", SubjectID: "a1"},
		{EventType: "run.step_appended", Target: "#ops", SubjectID: "s1"},
	} {
		if _, err := mgr.Append(ctx, item); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	events, _, err := mgr.ListSince(ctx, "", ListFilter{
		Target:     "#ops",
		EventTypes: []string{"run.step_appended", "activity.logged"},
	}, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].SubjectID != "a1" || events[1].SubjectID != "s1" {
		t.Fatalf("events = %+v, want activity then run step", events)
	}
}
