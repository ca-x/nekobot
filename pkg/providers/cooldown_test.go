package providers

import (
	"testing"
	"time"
)

func TestCooldownTrackerSnapshot_DefaultAvailable(t *testing.T) {
	tracker := NewCooldownTracker()

	snapshot := tracker.Snapshot("missing")
	if !snapshot.Available {
		t.Fatalf("expected missing provider to be available")
	}
	if snapshot.InCooldown {
		t.Fatalf("expected missing provider not to be in cooldown")
	}
	if snapshot.ErrorCount != 0 {
		t.Fatalf("expected zero errors, got %d", snapshot.ErrorCount)
	}
	if len(snapshot.FailureCounts) != 0 {
		t.Fatalf("expected empty failure counts, got %+v", snapshot.FailureCounts)
	}
}

func TestCooldownTrackerSnapshot_TracksCooldownAndFailureCounts(t *testing.T) {
	tracker := NewCooldownTracker()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	tracker.nowFunc = func() time.Time { return now }

	tracker.MarkFailure("primary", FailoverReasonRateLimit)

	snapshot := tracker.Snapshot("primary")
	if snapshot.Available {
		t.Fatalf("expected provider to be unavailable during cooldown")
	}
	if !snapshot.InCooldown {
		t.Fatalf("expected provider to be in cooldown")
	}
	if snapshot.ErrorCount != 1 {
		t.Fatalf("expected error count 1, got %d", snapshot.ErrorCount)
	}
	if got := snapshot.FailureCounts[FailoverReasonRateLimit]; got != 1 {
		t.Fatalf("expected one rate limit failure, got %d", got)
	}
	if snapshot.CooldownRemaining <= 0 {
		t.Fatalf("expected positive cooldown remaining, got %s", snapshot.CooldownRemaining)
	}
	if snapshot.LastFailure != now {
		t.Fatalf("expected last failure %v, got %v", now, snapshot.LastFailure)
	}
	if snapshot.CooldownRemaining != 5*time.Second {
		t.Fatalf("expected first failure cooldown to be 5s, got %s", snapshot.CooldownRemaining)
	}
}

func TestCooldownTrackerSnapshot_TracksBillingDisable(t *testing.T) {
	tracker := NewCooldownTracker()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	tracker.nowFunc = func() time.Time { return now }

	tracker.MarkFailure("billing", FailoverReasonBilling)

	snapshot := tracker.Snapshot("billing")
	if snapshot.Available {
		t.Fatalf("expected billing-disabled provider to be unavailable")
	}
	if snapshot.DisabledReason != FailoverReasonBilling {
		t.Fatalf("expected billing disabled reason, got %q", snapshot.DisabledReason)
	}
	if snapshot.DisabledUntil.IsZero() {
		t.Fatalf("expected disabled until to be populated")
	}
	if snapshot.CooldownRemaining < 5*time.Hour {
		t.Fatalf("expected billing cooldown around five hours, got %s", snapshot.CooldownRemaining)
	}
}

func TestCalculateStandardCooldown_UsesShortEscalation(t *testing.T) {
	cases := []struct {
		count int
		want  time.Duration
	}{
		{count: 1, want: 5 * time.Second},
		{count: 2, want: 30 * time.Second},
		{count: 3, want: 2 * time.Minute},
		{count: 4, want: 5 * time.Minute},
		{count: 9, want: 5 * time.Minute},
	}

	for _, tc := range cases {
		if got := calculateStandardCooldown(tc.count); got != tc.want {
			t.Fatalf("count=%d: want %s, got %s", tc.count, tc.want, got)
		}
	}
}

func TestCooldownTrackerClearProvider(t *testing.T) {
	tracker := NewCooldownTracker()
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	tracker.nowFunc = func() time.Time { return now }

	tracker.MarkFailure("primary", FailoverReasonRateLimit)
	if tracker.IsAvailable("primary") {
		t.Fatalf("expected provider to be unavailable before clear")
	}

	tracker.ClearProvider("primary")
	snapshot := tracker.Snapshot("primary")
	if !snapshot.Available {
		t.Fatalf("expected provider to be available after clear")
	}
	if snapshot.ErrorCount != 0 {
		t.Fatalf("expected error count reset, got %d", snapshot.ErrorCount)
	}
	if len(snapshot.FailureCounts) != 0 {
		t.Fatalf("expected failure counts reset, got %+v", snapshot.FailureCounts)
	}
}
