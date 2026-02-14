package agent

import (
	"reflect"
	"testing"

	"nekobot/pkg/config"
)

func TestBuildProviderOrder_UsesOverrideAndFallback(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "anthropic"
	cfg.Agents.Defaults.Fallback = []string{"openai", "ollama"}
	cfg.Providers = []config.ProviderProfile{
		{Name: "anthropic", ProviderKind: "anthropic"},
		{Name: "openai", ProviderKind: "openai"},
		{Name: "ollama", ProviderKind: "openai"},
	}

	ag := &Agent{config: cfg}

	got, err := ag.buildProviderOrder("openai", []string{"ollama", "openai", "anthropic"})
	if err != nil {
		t.Fatalf("buildProviderOrder failed: %v", err)
	}

	want := []string{"openai", "ollama", "anthropic"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected provider order %v, got %v", want, got)
	}
}

func TestBuildProviderOrder_UsesConfigDefaultsWhenRequestFallbackEmpty(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "anthropic"
	cfg.Agents.Defaults.Fallback = []string{"openai"}
	cfg.Providers = []config.ProviderProfile{
		{Name: "anthropic", ProviderKind: "anthropic"},
		{Name: "openai", ProviderKind: "openai"},
	}

	ag := &Agent{config: cfg}

	got, err := ag.buildProviderOrder("", nil)
	if err != nil {
		t.Fatalf("buildProviderOrder failed: %v", err)
	}

	want := []string{"anthropic", "openai"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected provider order %v, got %v", want, got)
	}
}

func TestResolveModelForProvider_FallsBackToProviderDefaultModel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "claude-sonnet-4-5-20250929"
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "anthropic",
			ProviderKind: "anthropic",
			Models:       []string{"claude-sonnet-4-5-20250929"},
			DefaultModel: "claude-sonnet-4-5-20250929",
		},
		{
			Name:         "openai",
			ProviderKind: "openai",
			Models:       []string{"gpt-4o-mini"},
			DefaultModel: "gpt-4o-mini",
		},
	}

	ag := &Agent{config: cfg}

	got := ag.resolveModelForProvider("openai", "anthropic", "claude-sonnet-4-5-20250929")
	want := "gpt-4o-mini"
	if got != want {
		t.Fatalf("expected model %q, got %q", want, got)
	}
}
