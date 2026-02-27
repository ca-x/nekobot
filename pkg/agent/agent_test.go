package agent

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/tools"
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

func TestResolveOrchestratorDefaultsToBlades(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = ""

	ag := &Agent{config: cfg}

	got, err := ag.resolveOrchestrator()
	if err != nil {
		t.Fatalf("resolveOrchestrator failed: %v", err)
	}
	if got != orchestratorBlades {
		t.Fatalf("expected orchestrator %q, got %q", orchestratorBlades, got)
	}
}

func TestResolveOrchestratorAcceptsKnownValues(t *testing.T) {
	tests := []struct {
		name         string
		orchestrator string
		want         string
	}{
		{name: "legacy", orchestrator: "legacy", want: orchestratorLegacy},
		{name: "blades", orchestrator: "blades", want: orchestratorBlades},
		{name: "uppercase trimmed", orchestrator: "  BLADES  ", want: orchestratorBlades},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Agents.Defaults.Orchestrator = tt.orchestrator

			ag := &Agent{config: cfg}

			got, err := ag.resolveOrchestrator()
			if err != nil {
				t.Fatalf("resolveOrchestrator failed: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected orchestrator %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveOrchestratorRejectsUnknownValue(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = "unsupported"

	ag := &Agent{config: cfg}

	_, err := ag.resolveOrchestrator()
	if err == nil {
		t.Fatalf("expected resolveOrchestrator error")
	}
	if !strings.Contains(err.Error(), "unsupported orchestrator") {
		t.Fatalf("expected unsupported orchestrator error, got %v", err)
	}
}

func TestChatRoutesThroughLegacyOrchestratorPath(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorLegacy)

	sess := &testSession{}
	_, err := ag.Chat(context.Background(), sess, "hello")
	if err == nil {
		t.Fatalf("expected chat error")
	}
	if !strings.Contains(err.Error(), "LLM call failed: provider not found: missing") {
		t.Fatalf("expected legacy path provider error, got %v", err)
	}
}

func TestChatRoutesThroughBladesOrchestratorPath(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)

	sess := &testSession{}
	_, err := ag.Chat(context.Background(), sess, "hello")
	if err == nil {
		t.Fatalf("expected chat error")
	}
	if !strings.Contains(err.Error(), "LLM call failed: provider not found: missing") {
		t.Fatalf("expected blades path provider error, got %v", err)
	}
}

func TestChatRejectsUnsupportedOrchestrator(t *testing.T) {
	ag := newRoutingTestAgent(t, "unknown")

	sess := &testSession{}
	_, err := ag.Chat(context.Background(), sess, "hello")
	if err == nil {
		t.Fatalf("expected chat error")
	}
	if !strings.Contains(err.Error(), "unsupported orchestrator") {
		t.Fatalf("expected unsupported orchestrator error, got %v", err)
	}
}

func newRoutingTestAgent(t *testing.T, orchestrator string) *Agent {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = orchestrator
	cfg.Agents.Defaults.Provider = "missing"
	cfg.Providers = nil

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag := &Agent{
		config:        cfg,
		logger:        log,
		context:       NewContextBuilder(t.TempDir()),
		tools:         tools.NewRegistry(),
		maxIterations: 1,
	}
	ag.context.SetToolDescriptionsFunc(ag.tools.GetDescriptions)

	return ag
}

type testSession struct {
	messages []Message
}

func (s *testSession) GetMessages() []Message {
	return s.messages
}

func (s *testSession) AddMessage(msg Message) {
	s.messages = append(s.messages, msg)
}
