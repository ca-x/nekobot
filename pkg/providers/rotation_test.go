package providers

import (
	"testing"
	"time"

	"nekobot/pkg/logger"
)

func TestRotationManagerSelectRoundRobinSkipsUnavailableProfiles(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: "error", OutputPath: ""})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	rm := NewRotationManager(log, RotationConfig{
		Enabled:  true,
		Strategy: StrategyRoundRobin,
	})
	rm.profiles = []*Profile{
		{Name: "alpha"},
		{Name: "beta", CooldownUntil: time.Now().Add(time.Hour)},
		{Name: "gamma"},
	}
	rm.currentIndex = 1

	selected := rm.selectRoundRobin(rm.getAvailableProfiles())
	if selected == nil {
		t.Fatalf("expected selected profile")
	}
	if selected.Name != "gamma" {
		t.Fatalf("expected gamma, got %q", selected.Name)
	}
	if rm.currentIndex != 0 {
		t.Fatalf("expected currentIndex to wrap to 0, got %d", rm.currentIndex)
	}
}

func TestRotationManagerSelectRoundRobinFallsBackToFirstAvailable(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: "error", OutputPath: ""})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	rm := NewRotationManager(log, RotationConfig{
		Enabled:  true,
		Strategy: StrategyRoundRobin,
	})
	rm.profiles = []*Profile{
		{Name: "alpha", CooldownUntil: time.Now().Add(time.Hour)},
		{Name: "beta", CooldownUntil: time.Now().Add(time.Hour)},
	}

	selected := rm.selectRoundRobin([]*Profile{{Name: "external"}})
	if selected == nil || selected.Name != "external" {
		t.Fatalf("expected fallback available profile, got %#v", selected)
	}
	if rm.currentIndex != 0 {
		t.Fatalf("expected currentIndex reset to 0, got %d", rm.currentIndex)
	}
}
