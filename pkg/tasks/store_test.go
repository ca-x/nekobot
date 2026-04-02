package tasks

import "testing"

func TestStoreListAggregatesSourcesDeterministically(t *testing.T) {
	store := NewStore()
	store.SetSource("zeta", func() []Task {
		return []Task{{ID: "z-1", State: StateRunning}}
	})
	store.SetSource("alpha", func() []Task {
		return []Task{{ID: "a-1", State: StatePending}, {ID: "a-2", State: StateCompleted}}
	})

	got := store.List()
	if len(got) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(got))
	}
	if got[0].ID != "a-1" || got[1].ID != "a-2" || got[2].ID != "z-1" {
		t.Fatalf("unexpected task ordering: %+v", got)
	}
}

func TestStoreRemoveSource(t *testing.T) {
	store := NewStore()
	store.SetSource("subagents", func() []Task {
		return []Task{{ID: "task-1", State: StateRunning}}
	})

	if got := store.List(); len(got) != 1 {
		t.Fatalf("expected one task before removal, got %d", len(got))
	}

	store.RemoveSource("subagents")
	if got := store.List(); len(got) != 0 {
		t.Fatalf("expected no tasks after removal, got %d", len(got))
	}
}
