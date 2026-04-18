package agent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/go-kratos/blades"
	bladestools "github.com/go-kratos/blades/tools"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"nekobot/pkg/approval"
	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/memory"
	promptmemory "nekobot/pkg/memory/prompt"
	"nekobot/pkg/modelroute"
	"nekobot/pkg/modelstore"
	"nekobot/pkg/permissionrules"
	"nekobot/pkg/process"
	"nekobot/pkg/prompts"
	"nekobot/pkg/providers"
	"nekobot/pkg/providerstore"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/tasks"
	"nekobot/pkg/tools"
)

func TestBuildProviderOrder_UsesOverrideAndFallback(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "anthropic"
	cfg.Agents.Defaults.Fallback = []string{"openai", "ollama"}
	cfg.Providers = []config.ProviderProfile{
		{Name: "anthropic", ProviderKind: "anthropic", APIKey: "anthropic-key"},
		{Name: "openai", ProviderKind: "openai", APIKey: "openai-key"},
		{Name: "ollama", ProviderKind: "openai", APIKey: "ollama-key"},
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
		{Name: "anthropic", ProviderKind: "anthropic", APIKey: "anthropic-key"},
		{Name: "openai", ProviderKind: "openai", APIKey: "openai-key"},
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

func TestBuildProviderOrder_SkipsProvidersMissingRequiredCredentials(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "openai-bad"
	cfg.Agents.Defaults.Fallback = []string{"anthropic-good"}
	cfg.Providers = []config.ProviderProfile{
		{Name: "openai-bad", ProviderKind: "openai", APIKey: ""},
		{Name: "anthropic-good", ProviderKind: "anthropic", APIKey: "anthropic-key"},
	}

	ag := &Agent{
		config:         cfg,
		logger:         testLogger(t),
		providerGroups: newProviderGroupPlanner(),
	}

	got, err := ag.buildProviderOrder("", nil)
	if err != nil {
		t.Fatalf("buildProviderOrder failed: %v", err)
	}

	want := []string{"anthropic-good"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected provider order %v, got %v", want, got)
	}
}

func TestProvideAgent_AllowsStartupWhenDefaultProviderConfigIsInvalid(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "openai-main"
	cfg.Agents.Defaults.Model = "gpt-4o-mini"
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "openai-main",
			ProviderKind: "openai",
			APIKey:       "",
			APIBase:      "https://api.openai.com/v1",
			Timeout:      30,
		},
	}

	log := testLogger(t)
	ag, err := ProvideAgent(provideAgentDeps{
		Cfg:         cfg,
		Log:         log,
		SkillsMgr:   skills.NewManager(log, filepath.Join(t.TempDir(), "skills"), false),
		ProcessMgr:  process.NewManager(log),
		ApprovalMgr: approval.NewManager(approval.Config{}),
		Bus:         bus.NewLocalBus(log, 16),
		LC:          fxtest.NewLifecycle(t),
	})
	if err != nil {
		t.Fatalf("ProvideAgent should not fail startup on invalid default provider config: %v", err)
	}
	if ag == nil {
		t.Fatalf("expected agent instance")
	}
	if ag.client != nil {
		t.Fatalf("expected nil provider client when default provider config is invalid")
	}
}

func TestBuildProviderOrder_ExpandsProviderGroupRoundRobin(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "openai-pool"
	cfg.Agents.Defaults.ProviderGroups = []config.ProviderGroupConfig{
		{
			Name:     "openai-pool",
			Strategy: "round_robin",
			Members:  []string{"openai-a", "openai-b"},
		},
	}
	cfg.Providers = []config.ProviderProfile{
		{Name: "openai-a", ProviderKind: "openai", APIKey: "openai-a-key"},
		{Name: "openai-b", ProviderKind: "openai", APIKey: "openai-b-key"},
	}

	ag := &Agent{
		config:         cfg,
		logger:         testLogger(t),
		providerGroups: newProviderGroupPlanner(),
	}

	first, err := ag.buildProviderOrder("", nil)
	if err != nil {
		t.Fatalf("first buildProviderOrder failed: %v", err)
	}
	second, err := ag.buildProviderOrder("", nil)
	if err != nil {
		t.Fatalf("second buildProviderOrder failed: %v", err)
	}

	if !reflect.DeepEqual(first, []string{"openai-a", "openai-b"}) {
		t.Fatalf("unexpected first order: %v", first)
	}
	if !reflect.DeepEqual(second, []string{"openai-b", "openai-a"}) {
		t.Fatalf("unexpected second order: %v", second)
	}
}

func TestBuildProviderOrder_ExpandsProviderGroupLeastUsed(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "openai-pool"
	cfg.Agents.Defaults.ProviderGroups = []config.ProviderGroupConfig{
		{
			Name:     "openai-pool",
			Strategy: "least_used",
			Members:  []string{"openai-a", "openai-b"},
		},
	}
	cfg.Providers = []config.ProviderProfile{
		{Name: "openai-a", ProviderKind: "openai", APIKey: "openai-a-key"},
		{Name: "openai-b", ProviderKind: "openai", APIKey: "openai-b-key"},
	}

	ag := &Agent{
		config:         cfg,
		logger:         testLogger(t),
		providerGroups: newProviderGroupPlanner(),
	}
	if _, err := ag.buildProviderOrder("", nil); err != nil {
		t.Fatalf("warmup buildProviderOrder failed: %v", err)
	}
	ag.providerGroups.recordSuccess("openai-a")

	got, err := ag.buildProviderOrder("", nil)
	if err != nil {
		t.Fatalf("buildProviderOrder failed: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"openai-b", "openai-a"}) {
		t.Fatalf("unexpected least-used order: %v", got)
	}
}

func TestResolveModelForProvider_UsesModelRoute(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.Model = "claude-sonnet-4-5-20250929"
	cfg.Providers = []config.ProviderProfile{
		{Name: "anthropic", ProviderKind: "anthropic", Enabled: true, DefaultWeight: 1},
		{Name: "openai", ProviderKind: "openai", Enabled: true, DefaultWeight: 1},
	}

	log := testLogger(t)
	client := newRuntimeEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})

	if err := createRouteFixtures(t, cfg, log, client); err != nil {
		t.Fatalf("createRouteFixtures failed: %v", err)
	}

	ag := &Agent{config: cfg, entClient: client, logger: log}

	got, err := ag.resolveModelForProvider(context.Background(), "openai", "anthropic", "claude-sonnet-4-5-20250929")
	if err != nil {
		t.Fatalf("resolveModelForProvider failed: %v", err)
	}
	want := "gpt-4o-mini"
	if got != want {
		t.Fatalf("expected model %q, got %q", want, got)
	}
}

func TestGetFailoverSnapshots_ReturnsTrackedState(t *testing.T) {
	cfg := config.DefaultConfig()
	ag := &Agent{
		config:           cfg,
		failoverCooldown: providers.NewCooldownTracker(),
	}

	ag.failoverCooldown.MarkFailure("primary", providers.FailoverReasonRateLimit)

	snapshots := ag.GetFailoverSnapshots([]string{"primary", "secondary"})
	primary, ok := snapshots["primary"]
	if !ok {
		t.Fatalf("expected primary snapshot")
	}
	if primary.Available {
		t.Fatalf("expected primary to be unavailable")
	}
	if got := primary.FailureCounts[providers.FailoverReasonRateLimit]; got != 1 {
		t.Fatalf("expected one rate limit failure, got %d", got)
	}

	secondary, ok := snapshots["secondary"]
	if !ok {
		t.Fatalf("expected secondary snapshot")
	}
	if !secondary.Available {
		t.Fatalf("expected secondary to be available")
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

func TestBuildToolsSection_SortsToolDescriptionsDeterministically(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilderWithMemory(workspace, promptmemory.NewStoreWithBackend(workspace, promptmemory.NewNoopBackend()))
	cb.SetToolDescriptionsFunc(func() []string {
		return []string{"zeta tool", "alpha tool", "middle tool"}
	})

	section := cb.buildToolsSection()
	alphaIdx := strings.Index(section, "alpha tool")
	middleIdx := strings.Index(section, "middle tool")
	zetaIdx := strings.Index(section, "zeta tool")

	if alphaIdx == -1 || middleIdx == -1 || zetaIdx == -1 {
		t.Fatalf("expected all tool descriptions in section: %q", section)
	}
	if alphaIdx >= middleIdx || middleIdx >= zetaIdx {
		t.Fatalf("expected sorted tool descriptions, got section: %q", section)
	}
}

func TestNewSemanticMemoryManagerFromConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Memory.Enabled = true
	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	mgr, err := newSemanticMemoryManagerFromConfig(log, cfg)
	if err != nil {
		t.Fatalf("newSemanticMemoryManagerFromConfig failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("expected memory manager")
	}
}

func TestAgentRegistersMemoryToolWhenSemanticMemoryEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Memory.Enabled = true
	cfg.Memory.Semantic.Enabled = true
	cfg.Memory.Semantic.DefaultTopK = 4
	cfg.Memory.Semantic.MaxTopK = 9
	cfg.Memory.Semantic.SearchPolicy = "vector"

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	tool, ok := ag.tools.Get("memory")
	if !ok {
		t.Fatal("expected memory tool to be registered")
	}
	memTool, ok := tool.(*tools.MemoryTool)
	if !ok {
		t.Fatalf("expected *tools.MemoryTool, got %T", tool)
	}
	if memTool == nil {
		t.Fatal("expected memory tool instance")
	}
}

func TestAgentRegistersWikiQueryTool(t *testing.T) {
	cfg := config.DefaultConfig()

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	tool, ok := ag.tools.Get("wiki_query")
	if !ok {
		t.Fatal("expected wiki_query tool to be registered")
	}
	if _, ok := tool.(*tools.WikiQueryTool); !ok {
		t.Fatalf("expected *tools.WikiQueryTool, got %T", tool)
	}
}

func TestAgentRegistersWikiLintTool(t *testing.T) {
	cfg := config.DefaultConfig()

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	tool, ok := ag.tools.Get("wiki_lint")
	if !ok {
		t.Fatal("expected wiki_lint tool to be registered")
	}
	if _, ok := tool.(*tools.WikiLintTool); !ok {
		t.Fatalf("expected *tools.WikiLintTool, got %T", tool)
	}
}

func createRouteFixtures(
	t *testing.T,
	cfg *config.Config,
	log *logger.Logger,
	client *ent.Client,
) error {
	t.Helper()

	modelMgr, err := modelstore.NewManager(cfg, log, client)
	if err != nil {
		return err
	}
	if _, err := modelMgr.Create(context.Background(), modelstore.ModelCatalog{
		ModelID:       "claude-sonnet-4-5-20250929",
		DisplayName:   "Claude Sonnet",
		CatalogSource: "builtin",
		Enabled:       true,
	}); err != nil && !errors.Is(err, modelstore.ErrModelExists) {
		return err
	}
	if _, err := modelMgr.Create(context.Background(), modelstore.ModelCatalog{
		ModelID:       "gpt-4o-mini",
		DisplayName:   "GPT-4o mini",
		CatalogSource: "provider_discovery",
		Enabled:       true,
	}); err != nil && !errors.Is(err, modelstore.ErrModelExists) {
		return err
	}

	routeMgr, err := modelroute.NewManager(cfg, log, client)
	if err != nil {
		return err
	}
	if _, err := routeMgr.Create(context.Background(), modelroute.ModelRoute{
		ModelID:      "claude-sonnet-4-5-20250929",
		ProviderName: "anthropic",
		Enabled:      true,
		IsDefault:    true,
	}); err != nil && !errors.Is(err, modelroute.ErrRouteExists) {
		return err
	}
	if _, err := routeMgr.Create(context.Background(), modelroute.ModelRoute{
		ModelID:      "claude-sonnet-4-5-20250929",
		ProviderName: "openai",
		Enabled:      true,
		IsDefault:    false,
		Aliases:      []string{"claude-sonnet-4-5-20250929"},
		Metadata: map[string]interface{}{
			"provider_model_id": "gpt-4o-mini",
		},
	}); err != nil && !errors.Is(err, modelroute.ErrRouteExists) {
		return err
	}

	return nil
}

func newRuntimeEntClient(t *testing.T, cfg *config.Config) *ent.Client {
	t.Helper()

	client, err := config.OpenRuntimeEntClient(cfg)
	if err != nil {
		t.Fatalf("open runtime ent client: %v", err)
	}
	if err := config.EnsureRuntimeEntSchema(client); err != nil {
		_ = client.Close()
		t.Fatalf("ensure runtime schema: %v", err)
	}
	return client
}

func TestEnableSubagentsRegistersSpawnTool(t *testing.T) {
	cfg := config.DefaultConfig()

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ag.EnableSubagents(nil)
	defer ag.DisableSubagents()

	if _, ok := ag.tools.Get("spawn"); !ok {
		t.Fatal("expected spawn tool to be registered")
	}
}

func TestEnableSubagentsRegistersManagedTaskLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ag.EnableSubagents(nil)
	defer ag.DisableSubagents()

	if ag.subagents == nil {
		t.Fatal("expected subagents manager to be initialized")
	}
	if !ag.subagents.HasTaskService() {
		t.Fatal("expected subagents task service to be initialized")
	}
}

func TestBuildSystemPrompt_UsesCurrentTimePlaceholderReplacement(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilderWithMemory(workspace, promptmemory.NewStoreWithBackend(workspace, promptmemory.NewNoopBackend()))
	cb.SetToolDescriptionsFunc(func() []string { return nil })

	prompt := cb.BuildSystemPrompt()
	if strings.Contains(prompt, currentTimePlaceholder) {
		t.Fatalf("expected current time placeholder to be replaced")
	}
	if !strings.Contains(prompt, "## Current Time\n") {
		t.Fatalf("expected current time section in prompt")
	}
}

func TestBuildSystemPrompt_CacheRefreshesOnBootstrapFileChange(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilderWithMemory(workspace, promptmemory.NewStoreWithBackend(workspace, promptmemory.NewNoopBackend()))
	cb.SetToolDescriptionsFunc(func() []string { return nil })

	first := cb.BuildSystemPrompt()
	if err := os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("soul-note"), 0644); err != nil {
		t.Fatalf("write SOUL.md: %v", err)
	}

	second := cb.BuildSystemPrompt()
	if first == second {
		t.Fatalf("expected prompt to change after bootstrap file update")
	}
	if !strings.Contains(second, "soul-note") {
		t.Fatalf("expected updated bootstrap content in prompt")
	}
}

func TestBuildSystemPrompt_CacheRefreshesOnToolDescriptionChange(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilderWithMemory(workspace, promptmemory.NewStoreWithBackend(workspace, promptmemory.NewNoopBackend()))
	descriptions := []string{"alpha tool"}
	cb.SetToolDescriptionsFunc(func() []string { return append([]string(nil), descriptions...) })

	first := cb.BuildSystemPrompt()
	descriptions = append(descriptions, "beta tool")
	second := cb.BuildSystemPrompt()

	if first == second {
		t.Fatalf("expected prompt to change after tool descriptions update")
	}
	if !strings.Contains(second, "beta tool") {
		t.Fatalf("expected updated tool description in prompt")
	}
}

func TestBuildSystemPrompt_IncludesLayeredMemoryContext(t *testing.T) {
	workspace := t.TempDir()
	store := promptmemory.NewStore(workspace)
	if err := os.WriteFile(filepath.Join(workspace, "MEMORY.md"), []byte("workspace memory"), 0644); err != nil {
		t.Fatalf("write workspace memory: %v", err)
	}
	if err := store.WriteLongTerm("long term memory that should be truncated after the configured prompt budget is exceeded"); err != nil {
		t.Fatalf("write long term memory: %v", err)
	}
	if err := store.AppendToday("today note"); err != nil {
		t.Fatalf("append today note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "memory", "active_learnings.md"), []byte("active learning"), 0644); err != nil {
		t.Fatalf("write active learnings: %v", err)
	}

	cb := NewContextBuilderWithMemory(workspace, store)
	cb.SetToolDescriptionsFunc(func() []string { return nil })

	prompt := cb.BuildSystemPrompt()
	if !strings.Contains(prompt, "# Memory\n\n## Recalled Memory Context") {
		t.Fatalf("expected recalled memory heading in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "```memory\n## Workspace Memory\n\nworkspace memory") {
		t.Fatalf("expected workspace memory in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "## Long-term Memory\n\nlong term memory") {
		t.Fatalf("expected long-term memory in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "## Today's Notes\n\n# ") {
		t.Fatalf("expected today's notes heading in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "today note") {
		t.Fatalf("expected today's note content in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "## Active Learnings\n\nactive learning") {
		t.Fatalf("expected active learnings in prompt, got %q", prompt)
	}
}

func TestBuildSystemPrompt_RespectsMemoryContextOptions(t *testing.T) {
	workspace := t.TempDir()
	store := promptmemory.NewStore(workspace)
	if err := os.WriteFile(filepath.Join(workspace, "MEMORY.md"), []byte("workspace memory"), 0644); err != nil {
		t.Fatalf("write workspace memory: %v", err)
	}
	if err := store.WriteLongTerm(
		"long term memory that should be truncated after the configured prompt budget is exceeded",
	); err != nil {
		t.Fatalf("write long term memory: %v", err)
	}
	if err := store.AppendToday("today note"); err != nil {
		t.Fatalf("append today note: %v", err)
	}

	cb := NewContextBuilderWithMemory(workspace, store)
	cb.SetToolDescriptionsFunc(func() []string { return nil })
	cb.SetMemoryContextOptions(promptmemory.ContextOptions{
		IncludeWorkspaceMemory: false,
		IncludeLongTerm:        true,
		RecentDailyNoteDays:    0,
		MaxChars:               48,
	})

	prompt := cb.BuildSystemPrompt()
	if strings.Contains(prompt, "workspace memory") {
		t.Fatalf("expected workspace memory to be omitted, got %q", prompt)
	}
	if strings.Contains(prompt, "today note") {
		t.Fatalf("expected daily note to be omitted, got %q", prompt)
	}
	if !strings.Contains(prompt, "## Long-term Memory") && !strings.Contains(prompt, "## Recalled Memory C") {
		t.Fatalf("expected long-term memory section to remain, got %q", prompt)
	}
	if !strings.Contains(prompt, "[Memory context truncated]") {
		t.Fatalf("expected truncation marker, got %q", prompt)
	}
}

func TestContextBuilderBuildPromptSections_SeparatesStaticAndDynamic(t *testing.T) {
	workspace := t.TempDir()
	store := promptmemory.NewStore(workspace)
	if err := os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("bootstrap-note"), 0644); err != nil {
		t.Fatalf("write SOUL.md: %v", err)
	}
	if err := store.WriteLongTerm("long-term-note"); err != nil {
		t.Fatalf("write long term memory: %v", err)
	}

	cb := NewContextBuilderWithMemory(workspace, store)
	cb.SetToolDescriptionsFunc(func() []string { return []string{"read file tool"} })

	sections := cb.BuildPromptSections()
	if len(sections) == 0 {
		t.Fatal("expected prompt sections")
	}

	seenIdentity := false
	seenBootstrap := false
	seenMemory := false
	for _, section := range sections {
		switch section.ID {
		case "identity":
			seenIdentity = true
			if !section.Stable {
				t.Fatalf("expected identity section to be stable")
			}
		case "bootstrap":
			seenBootstrap = true
			if !section.Stable {
				t.Fatalf("expected bootstrap section to be stable")
			}
		case "memory":
			seenMemory = true
			if section.Stable {
				t.Fatalf("expected memory section to be dynamic")
			}
		}
	}

	if !seenIdentity || !seenBootstrap || !seenMemory {
		t.Fatalf("missing expected sections: identity=%v bootstrap=%v memory=%v", seenIdentity, seenBootstrap, seenMemory)
	}
}

func TestAgentDefinitionFromRuntimeConfig_BridgesCurrentDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "openai-primary"
	cfg.Agents.Defaults.Model = "gpt-5.4"
	cfg.Agents.Defaults.Orchestrator = orchestratorBlades
	cfg.Agents.Defaults.MaxToolIterations = 7
	cfg.Approval.Mode = "manual"
	cfg.Approval.Allowlist = []string{"read_file"}
	cfg.Approval.Denylist = []string{"exec"}

	def := AgentDefinitionFromRuntimeConfig(cfg)
	if def.ID != "main" {
		t.Fatalf("expected main definition id, got %q", def.ID)
	}
	if def.Route.Provider != "openai-primary" || def.Route.Model != "gpt-5.4" {
		t.Fatalf("unexpected route: %+v", def.Route)
	}
	if def.Orchestrator != orchestratorBlades {
		t.Fatalf("unexpected orchestrator %q", def.Orchestrator)
	}
	if def.PermissionMode != approval.ModeManual {
		t.Fatalf("unexpected permission mode %q", def.PermissionMode)
	}
	if def.MaxToolIterations != 7 {
		t.Fatalf("unexpected max tool iterations %d", def.MaxToolIterations)
	}
	if !reflect.DeepEqual(def.ToolPolicy.Allowlist, []string{"read_file"}) {
		t.Fatalf("unexpected allowlist: %+v", def.ToolPolicy.Allowlist)
	}
	if !reflect.DeepEqual(def.ToolPolicy.Denylist, []string{"exec"}) {
		t.Fatalf("unexpected denylist: %+v", def.ToolPolicy.Denylist)
	}
	if len(def.PromptSections.Static) == 0 {
		t.Fatalf("expected prompt sections metadata to be present")
	}
}

func TestNewAgent_SeedsDefinitionSnapshot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "anthropic"
	cfg.Agents.Defaults.Model = "claude-sonnet"
	cfg.Approval.Mode = "prompt"

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModePrompt}), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	def := ag.Definition()
	if def.Route.Provider != "anthropic" || def.Route.Model != "claude-sonnet" {
		t.Fatalf("unexpected definition route: %+v", def.Route)
	}
	if def.PermissionMode != approval.ModePrompt {
		t.Fatalf("unexpected definition permission mode: %q", def.PermissionMode)
	}
}

func TestPreviewContextSources_IncludesKeySourceTypes(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte("project-rules"), 0644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Agents.Defaults.MCPServers = []config.MCPServerConfig{{Name: "filesystem", Transport: "stdio"}}
	cfg.Memory.Context.Enabled = true

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	client := newRuntimeEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	promptMgr, err := prompts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}
	promptItem, err := promptMgr.CreatePrompt(context.Background(), prompts.Prompt{
		Key:      "ops",
		Name:     "Ops",
		Mode:     prompts.ModeSystem,
		Template: "provider={{route.provider}}",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create prompt: %v", err)
	}
	if _, err := promptMgr.CreateBinding(context.Background(), prompts.Binding{
		Scope:    prompts.ScopeGlobal,
		PromptID: promptItem.ID,
		Enabled:  true,
		Priority: 100,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, client, promptMgr)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if err := ag.context.GetMemory().WriteLongTerm("long-term-memory"); err != nil {
		t.Fatalf("write long-term memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("ops-rules"), 0644); err != nil {
		t.Fatalf("write SOUL.md: %v", err)
	}

	preview, err := ag.PreviewContextSources(context.Background(), PromptContext{
		Channel:           "wechat",
		SessionID:         "s-1",
		RequestedProvider: "openai",
		RequestedModel:    "gpt-5.4",
		Custom: map[string]any{
			"runtime_id": "runtime-a",
			"role":       "ops",
		},
	}, "hello")
	if err != nil {
		t.Fatalf("PreviewContextSources failed: %v", err)
	}

	kinds := make(map[string]bool)
	for _, item := range preview.Sources {
		kinds[item.Kind] = true
	}
	for _, required := range []string{
		"project_rules",
		"memory",
		"managed_prompts",
		"runtime_context",
		"mcp",
	} {
		if !kinds[required] {
			t.Fatalf("expected source kind %q in %+v", required, preview.Sources)
		}
	}
	if preview.Footprint.SystemChars <= 0 {
		t.Fatalf("expected positive system footprint, got %+v", preview.Footprint)
	}
	if preview.Footprint.MemoryLimitChars != cfg.Memory.Context.MaxChars {
		t.Fatalf("expected memory limit %d, got %+v", cfg.Memory.Context.MaxChars, preview.Footprint)
	}
	if preview.Footprint.FinalUserChars <= 0 {
		t.Fatalf("expected positive final user chars, got %+v", preview.Footprint)
	}
	if preview.Footprint.TotalChars < preview.Footprint.SystemChars+preview.Footprint.FinalUserChars {
		t.Fatalf("expected total chars to cover system and user content, got %+v", preview.Footprint)
	}
	if len(preview.Warnings) != 0 {
		t.Fatalf("expected no warnings for simple preview, got %+v", preview.Warnings)
	}
	if preview.BudgetStatus != "ok" {
		t.Fatalf("expected ok budget status, got %+v", preview)
	}
	if preview.Preflight.Action != "proceed" {
		t.Fatalf("expected proceed preflight action, got %+v", preview.Preflight)
	}
	if len(preview.BudgetReasons) != 0 {
		t.Fatalf("expected no budget reasons for simple preview, got %+v", preview.BudgetReasons)
	}
	if preview.Compaction.Recommended {
		t.Fatalf("expected no compaction recommendation for simple preview, got %+v", preview.Compaction)
	}
}

func TestPreviewContextSources_ReportsCriticalBudgetStatus(t *testing.T) {
	workspace := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Agents.Defaults.MaxTokens = 20

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	client := newRuntimeEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	promptMgr, err := prompts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, client, promptMgr)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	preview, err := ag.PreviewContextSources(context.Background(), PromptContext{}, strings.Repeat("x", 400))
	if err != nil {
		t.Fatalf("PreviewContextSources failed: %v", err)
	}
	if preview.BudgetStatus != "critical" {
		t.Fatalf("expected critical budget status, got %+v", preview)
	}
	if preview.Preflight.Action != "compact_before_run" {
		t.Fatalf("expected compact_before_run preflight action, got %+v", preview.Preflight)
	}
	if len(preview.BudgetReasons) == 0 {
		t.Fatalf("expected budget reasons, got %+v", preview)
	}
	if !preview.Compaction.Recommended {
		t.Fatalf("expected compaction recommendation, got %+v", preview.Compaction)
	}
	if preview.Compaction.Strategy == "" {
		t.Fatalf("expected compaction strategy, got %+v", preview.Compaction)
	}
	if preview.Compaction.EstimatedCharsSaved <= 0 {
		t.Fatalf("expected estimated savings, got %+v", preview.Compaction)
	}
}

func TestPreviewContextSources_ReportsWarningBudgetStatusForMemoryPressure(t *testing.T) {
	workspace := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Agents.Defaults.MaxTokens = 10_000
	cfg.Memory.Context.Enabled = true
	cfg.Memory.Context.MaxChars = 100

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	client := newRuntimeEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})
	promptMgr, err := prompts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, client, promptMgr)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if err := ag.context.GetMemory().WriteLongTerm(strings.Repeat("memory ", 20)); err != nil {
		t.Fatalf("write long-term memory: %v", err)
	}

	preview, err := ag.PreviewContextSources(context.Background(), PromptContext{}, "hello")
	if err != nil {
		t.Fatalf("PreviewContextSources failed: %v", err)
	}
	if preview.BudgetStatus != "warning" {
		t.Fatalf("expected warning budget status, got %+v", preview)
	}
	if preview.Preflight.BudgetStatus != "warning" {
		t.Fatalf("expected warning preflight budget status, got %+v", preview.Preflight)
	}
	if preview.Preflight.Action != "consider_compaction" {
		t.Fatalf("expected consider_compaction preflight action, got %+v", preview.Preflight)
	}
	if len(preview.BudgetReasons) == 0 {
		t.Fatalf("expected budget reasons, got %+v", preview)
	}
	if !preview.Compaction.Recommended {
		t.Fatalf("expected compaction recommendation, got %+v", preview.Compaction)
	}
	if preview.Compaction.Strategy != "compress_memory" {
		t.Fatalf("expected compress_memory strategy, got %+v", preview.Compaction)
	}
	if preview.Compaction.EstimatedCharsSaved <= 0 {
		t.Fatalf("expected estimated savings, got %+v", preview.Compaction)
	}
}

func TestChatWithPromptContextDetailed_IncludesContextPressurePreview(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = orchestratorLegacy
	cfg.Agents.Defaults.Provider = "test-primary"
	cfg.Agents.Defaults.Model = "gpt-5.4"
	cfg.Agents.Defaults.MaxTokens = 20
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "test-primary",
			ProviderKind: failoverTestProviderKind(t, "primary"),
		},
	}

	callCount := 0
	registerFailoverTestProvider(t, cfg.Providers[0].ProviderKind, &callCount, "final answer", nil)

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	response, routeResult, err := ag.ChatWithPromptContextDetailed(
		context.Background(),
		&testSession{},
		strings.Repeat("x", 400),
		PromptContext{
			SessionID:         "webui-chat:tester",
			Channel:           "webui",
			RequestedProvider: "test-primary",
			RequestedModel:    "gpt-5.4",
		},
	)
	if err != nil {
		t.Fatalf("ChatWithPromptContextDetailed failed: %v", err)
	}
	if response != "final answer" {
		t.Fatalf("expected final answer, got %q", response)
	}
	if routeResult.ContextBudgetStatus != "critical" {
		t.Fatalf("expected critical budget status, got %+v", routeResult)
	}
	if len(routeResult.ContextBudgetReasons) == 0 {
		t.Fatalf("expected budget reasons, got %+v", routeResult)
	}
	if !routeResult.CompactionRecommended {
		t.Fatalf("expected compaction recommendation, got %+v", routeResult)
	}
	if routeResult.CompactionStrategy == "" {
		t.Fatalf("expected compaction strategy, got %+v", routeResult)
	}
	if routeResult.Preflight.BudgetStatus != "critical" {
		t.Fatalf("expected critical preflight budget status, got %+v", routeResult.Preflight)
	}
	if routeResult.Preflight.Action != "compact_before_run" {
		t.Fatalf("expected compact_before_run preflight action, got %+v", routeResult.Preflight)
	}
	if !routeResult.Preflight.Applied {
		t.Fatalf("expected preflight applied flag to be true, got %+v", routeResult.Preflight)
	}
	if !routeResult.Preflight.Compaction.Recommended {
		t.Fatalf("expected preflight compaction recommendation, got %+v", routeResult.Preflight)
	}
	if callCount != 1 {
		t.Fatalf("expected one provider call, got %d", callCount)
	}
}

func TestChatWithPromptContextDetailed_BladesIncludesContextPressurePreview(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = orchestratorBlades
	cfg.Agents.Defaults.Provider = "test-primary"
	cfg.Agents.Defaults.Model = "gpt-5.4"
	cfg.Agents.Defaults.MaxTokens = 20
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "test-primary",
			ProviderKind: failoverTestProviderKind(t, "primary"),
		},
	}

	callCount := 0
	registerFailoverTestProvider(t, cfg.Providers[0].ProviderKind, &callCount, "final answer", nil)

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	response, routeResult, err := ag.ChatWithPromptContextDetailed(
		context.Background(),
		&testSession{},
		strings.Repeat("x", 400),
		PromptContext{
			SessionID:         "webui-chat:tester",
			Channel:           "webui",
			RequestedProvider: "test-primary",
			RequestedModel:    "gpt-5.4",
		},
	)
	if err != nil {
		t.Fatalf("ChatWithPromptContextDetailed failed: %v", err)
	}
	if response != "final answer" {
		t.Fatalf("expected final answer, got %q", response)
	}
	if routeResult.ContextBudgetStatus != "critical" {
		t.Fatalf("expected critical budget status, got %+v", routeResult)
	}
	if len(routeResult.ContextBudgetReasons) == 0 {
		t.Fatalf("expected budget reasons, got %+v", routeResult)
	}
	if !routeResult.CompactionRecommended {
		t.Fatalf("expected compaction recommendation, got %+v", routeResult)
	}
	if routeResult.CompactionStrategy == "" {
		t.Fatalf("expected compaction strategy, got %+v", routeResult)
	}
	if routeResult.Preflight.BudgetStatus != "critical" {
		t.Fatalf("expected critical preflight budget status, got %+v", routeResult.Preflight)
	}
	if routeResult.Preflight.Action != "compact_before_run" {
		t.Fatalf("expected compact_before_run preflight action, got %+v", routeResult.Preflight)
	}
	if !routeResult.Preflight.Compaction.Recommended {
		t.Fatalf("expected preflight compaction recommendation, got %+v", routeResult.Preflight)
	}
	if callCount != 1 {
		t.Fatalf("expected one provider call, got %d", callCount)
	}
}

func TestChatWithPromptContextDetailed_AutoCompressesCriticalPreflightBeforeLegacyCall(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = orchestratorLegacy
	cfg.Agents.Defaults.Provider = "test-primary"
	cfg.Agents.Defaults.Model = "gpt-5.4"
	cfg.Agents.Defaults.MaxTokens = 20
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "test-primary",
			ProviderKind: failoverTestProviderKind(t, "primary"),
		},
	}

	callCount := 0
	var captured *providers.UnifiedRequest
	registerFailoverTestProviderWithCapture(
		t,
		cfg.Providers[0].ProviderKind,
		&callCount,
		"final answer",
		nil,
		func(req *providers.UnifiedRequest) {
			captured = req
		},
	)

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	sess := &testSession{
		messages: []Message{
			{Role: "user", Content: "first"},
			{Role: "assistant", Content: "second"},
			{Role: "user", Content: "third"},
			{Role: "assistant", Content: "fourth"},
		},
	}

	_, routeResult, err := ag.ChatWithPromptContextDetailed(
		context.Background(),
		sess,
		strings.Repeat("x", 400),
		PromptContext{
			SessionID:         "webui-chat:tester",
			Channel:           "webui",
			RequestedProvider: "test-primary",
			RequestedModel:    "gpt-5.4",
		},
	)
	if err != nil {
		t.Fatalf("ChatWithPromptContextDetailed failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected one provider call, got %d", callCount)
	}
	if captured == nil {
		t.Fatal("expected captured provider request")
	}
	if routeResult.Preflight.Action != "compact_before_run" {
		t.Fatalf("expected compact_before_run preflight action, got %+v", routeResult.Preflight)
	}

	expectedBefore := ag.convertToProviderMessages(
		ag.context.BuildMessagesWithPromptSet(
			sess.GetMessages(),
			strings.Repeat("x", 400),
			prompts.ResolvedPromptSet{},
		),
	)
	expectedAfter := forceCompressMessages(expectedBefore)
	if len(captured.Messages) != len(expectedAfter) {
		t.Fatalf("expected %d outbound messages after compression, got %d", len(expectedAfter), len(captured.Messages))
	}
	if captured.Messages[0].Content != expectedAfter[0].Content {
		t.Fatalf("expected compressed system message note, got %q", captured.Messages[0].Content)
	}
	if reflect.DeepEqual(captured.Messages, expectedBefore) {
		t.Fatalf("expected outbound request to differ from uncompressed messages")
	}
	if !reflect.DeepEqual(captured.Messages, expectedAfter) {
		t.Fatalf("expected outbound request to match force-compressed messages")
	}
	if !reflect.DeepEqual(sess.GetMessages(), []Message{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "second"},
		{Role: "user", Content: "third"},
		{Role: "assistant", Content: "fourth"},
	}) {
		t.Fatalf("expected session history to remain unchanged, got %#v", sess.GetMessages())
	}
}

func TestChatWithPromptContextDetailed_DoesNotAutoCompressWarningPreflightBeforeLegacyCall(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = orchestratorLegacy
	cfg.Agents.Defaults.Provider = "test-primary"
	cfg.Agents.Defaults.Model = "gpt-5.4"
	cfg.Agents.Defaults.MaxTokens = 10_000
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Memory.Context.Enabled = true
	cfg.Memory.Context.MaxChars = 100
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "test-primary",
			ProviderKind: failoverTestProviderKind(t, "primary"),
		},
	}

	callCount := 0
	var captured *providers.UnifiedRequest
	registerFailoverTestProviderWithCapture(
		t,
		cfg.Providers[0].ProviderKind,
		&callCount,
		"final answer",
		nil,
		func(req *providers.UnifiedRequest) {
			captured = req
		},
	)

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if err := ag.context.GetMemory().WriteLongTerm(strings.Repeat("memory ", 20)); err != nil {
		t.Fatalf("write long-term memory: %v", err)
	}

	_, routeResult, err := ag.ChatWithPromptContextDetailed(
		context.Background(),
		&testSession{},
		"hello",
		PromptContext{
			SessionID:         "webui-chat:tester",
			Channel:           "webui",
			RequestedProvider: "test-primary",
			RequestedModel:    "gpt-5.4",
		},
	)
	if err != nil {
		t.Fatalf("ChatWithPromptContextDetailed failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected one provider call, got %d", callCount)
	}
	if captured == nil {
		t.Fatal("expected captured provider request")
	}
	if routeResult.Preflight.Action != "consider_compaction" {
		t.Fatalf("expected consider_compaction preflight action, got %+v", routeResult.Preflight)
	}
	if routeResult.Preflight.Applied {
		t.Fatalf("expected preflight applied flag to stay false for warning path, got %+v", routeResult.Preflight)
	}

	expectedMessages := ag.convertToProviderMessages(
		ag.context.BuildMessagesWithPromptSet(
			nil,
			"hello",
			prompts.ResolvedPromptSet{},
		),
	)
	if !reflect.DeepEqual(captured.Messages, expectedMessages) {
		t.Fatalf("expected warning request to remain uncompressed")
	}
}

func TestChatWithPromptContextDetailed_AutoCompressesCriticalPreflightBeforeBladesCall(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = orchestratorBlades
	cfg.Agents.Defaults.Provider = "test-primary"
	cfg.Agents.Defaults.Model = "gpt-5.4"
	cfg.Agents.Defaults.MaxTokens = 20
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "test-primary",
			ProviderKind: failoverTestProviderKind(t, "primary"),
		},
	}

	callCount := 0
	var captured *providers.UnifiedRequest
	registerFailoverTestProviderWithCapture(
		t,
		cfg.Providers[0].ProviderKind,
		&callCount,
		"final answer",
		nil,
		func(req *providers.UnifiedRequest) {
			captured = req
		},
	)

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	ag, err := New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	sess := &testSession{
		messages: []Message{
			{Role: "user", Content: "first"},
			{Role: "assistant", Content: "second"},
			{Role: "user", Content: "third"},
			{Role: "assistant", Content: "fourth"},
		},
	}

	_, routeResult, err := ag.ChatWithPromptContextDetailed(
		context.Background(),
		sess,
		strings.Repeat("x", 400),
		PromptContext{
			SessionID:         "webui-chat:tester",
			Channel:           "webui",
			RequestedProvider: "test-primary",
			RequestedModel:    "gpt-5.4",
		},
	)
	if err != nil {
		t.Fatalf("ChatWithPromptContextDetailed failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected one provider call, got %d", callCount)
	}
	if captured == nil {
		t.Fatal("expected captured provider request")
	}
	if routeResult.Preflight.Action != "compact_before_run" {
		t.Fatalf("expected compact_before_run preflight action, got %+v", routeResult.Preflight)
	}
	if !routeResult.Preflight.Applied {
		t.Fatalf("expected preflight applied flag to be true, got %+v", routeResult.Preflight)
	}

	expectedBefore := &providers.UnifiedRequest{
		Model:       "gpt-5.4",
		MaxTokens:   ag.config.Agents.Defaults.MaxTokens,
		Temperature: ag.config.Agents.Defaults.Temperature,
		Messages: append(
			append(
				[]providers.UnifiedMessage{{
					Role:    "system",
					Content: ag.context.BuildSystemPromptWithInjected(prompts.ResolvedPromptSet{}),
				}},
				ag.convertToProviderMessages(sanitizeHistory(trimTrailingCurrentUserMessage(sess.GetMessages(), strings.Repeat("x", 400))))...,
			),
			providers.UnifiedMessage{
				Role:    "user",
				Content: strings.Repeat("x", 400),
			},
		),
	}
	expectedAfter := forceCompressMessages(expectedBefore.Messages)
	if reflect.DeepEqual(captured.Messages, expectedBefore.Messages) {
		t.Fatalf("expected blades outbound request to differ from uncompressed messages")
	}
	if !reflect.DeepEqual(captured.Messages, expectedAfter) {
		t.Fatalf("expected blades outbound request to match force-compressed messages")
	}
	if !reflect.DeepEqual(sess.GetMessages(), []Message{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "second"},
		{Role: "user", Content: "third"},
		{Role: "assistant", Content: "fourth"},
	}) {
		t.Fatalf("expected session history to remain unchanged, got %#v", sess.GetMessages())
	}
}

func TestBuildMessages_DeduplicatesTrailingCurrentUserMessage(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilderWithMemory(workspace, promptmemory.NewStoreWithBackend(workspace, promptmemory.NewNoopBackend()))
	cb.SetToolDescriptionsFunc(func() []string { return nil })

	history := []Message{{Role: "user", Content: "hello"}}
	messages := cb.BuildMessages(history, "  hello  ")

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (system + current user), got %d", len(messages))
	}
	if messages[1].Role != "user" || messages[1].Content != "  hello  " {
		t.Fatalf("expected current user message to be preserved, got %#v", messages[1])
	}
}

func TestBuildMessages_KeepsNonMatchingTrailingUserHistory(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilderWithMemory(workspace, promptmemory.NewStoreWithBackend(workspace, promptmemory.NewNoopBackend()))
	cb.SetToolDescriptionsFunc(func() []string { return nil })

	history := []Message{{Role: "user", Content: "hello"}}
	messages := cb.BuildMessages(history, "hello again")

	if len(messages) != 3 {
		t.Fatalf("expected 3 messages (system + history user + current user), got %d", len(messages))
	}
	if messages[1].Role != "user" || messages[1].Content != "hello" {
		t.Fatalf("expected history user message to remain, got %#v", messages[1])
	}
	if messages[2].Role != "user" || messages[2].Content != "hello again" {
		t.Fatalf("expected current user message, got %#v", messages[2])
	}
}

func TestTrimTrailingCurrentUserMessage(t *testing.T) {
	history := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "  ping  "},
	}
	trimmed := trimTrailingCurrentUserMessage(history, "ping")
	if len(trimmed) != 2 {
		t.Fatalf("expected trailing matching user message to be trimmed, got %d messages", len(trimmed))
	}

	unchangedByRole := trimTrailingCurrentUserMessage([]Message{{Role: "assistant", Content: "ping"}}, "ping")
	if len(unchangedByRole) != 1 {
		t.Fatalf("expected assistant tail to remain unchanged, got %d messages", len(unchangedByRole))
	}

	unchangedByEmptyCurrent := trimTrailingCurrentUserMessage([]Message{{Role: "user", Content: "ping"}}, "   ")
	if len(unchangedByEmptyCurrent) != 1 {
		t.Fatalf("expected empty current message to keep history unchanged, got %d messages", len(unchangedByEmptyCurrent))
	}
}

func TestSessionHistoryUsesSafeWindowWhenAvailable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Memory.ShortTerm.Enabled = true
	cfg.Memory.ShortTerm.RawHistoryLimit = 2

	ag := &Agent{config: cfg}
	sess := &testSession{
		messages: []Message{
			{Role: "user", Content: "older"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call-1", Name: "read_file", Arguments: map[string]interface{}{"path": "/tmp/a"}},
				},
			},
			{Role: "tool", Content: "result", ToolCallID: "call-1"},
			{Role: "assistant", Content: "done"},
		},
	}

	history := ag.sessionHistory(sess)
	if len(history) != 3 {
		t.Fatalf("expected safe history expansion to 3 messages, got %d", len(history))
	}
	if history[0].Role != "assistant" || len(history[0].ToolCalls) != 1 {
		t.Fatalf("expected assistant tool-call message first, got %#v", history[0])
	}
	if history[1].Role != "tool" || history[1].ToolCallID != "call-1" {
		t.Fatalf("expected matching tool result second, got %#v", history[1])
	}
}

func TestContextBuilderPreprocessorCanBeDisabledViaConfig(t *testing.T) {
	workspace := t.TempDir()
	target := filepath.Join(workspace, "README.md")
	if err := os.WriteFile(target, []byte("hello from file"), 0644); err != nil {
		t.Fatalf("write referenced file: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Preprocess.FileMentions.Enabled = false

	store := promptmemory.NewStore(workspace)
	cb := NewContextBuilderWithMemory(workspace, store)
	cb.SetToolDescriptionsFunc(func() []string { return nil })
	cb.SetPreprocessorConfig(preprocessConfigFromConfig(cfg, workspace))

	messages := cb.BuildMessages(nil, "check @README.md")
	if len(messages) != 2 {
		t.Fatalf("expected system and user messages, got %d", len(messages))
	}
	if strings.Contains(messages[1].Content, "# Referenced Files") {
		t.Fatalf("expected file references to be disabled, got %q", messages[1].Content)
	}
	if messages[1].Content != "check @README.md" {
		t.Fatalf("expected original user content, got %q", messages[1].Content)
	}
}

func TestNewMemoryStoreFromConfig_FileBackendDefaultPath(t *testing.T) {
	workspace := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Memory.Enabled = true
	cfg.Memory.Backend = ""
	cfg.Memory.FilePath = ""

	store := newMemoryStoreFromConfig(cfg, workspace, nil, nil)
	if store == nil {
		t.Fatalf("expected memory store")
	}

	if err := store.WriteLongTerm("hello"); err != nil {
		t.Fatalf("write long-term memory: %v", err)
	}

	content := store.ReadLongTerm()
	if content != "hello" {
		t.Fatalf("expected long-term memory content %q, got %q", "hello", content)
	}

	memoryFile := filepath.Join(workspace, "memory", "MEMORY.md")
	if _, err := os.Stat(memoryFile); err != nil {
		t.Fatalf("expected file backend to create %s: %v", memoryFile, err)
	}
}

func TestNewMemoryStoreFromConfig_ExplicitFilePath(t *testing.T) {
	workspace := t.TempDir()
	memoryDir := filepath.Join(t.TempDir(), "custom-memory")

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Memory.Enabled = true
	cfg.Memory.Backend = "file"
	cfg.Memory.FilePath = memoryDir

	store := newMemoryStoreFromConfig(cfg, workspace, nil, nil)
	if err := store.AppendToday("entry"); err != nil {
		t.Fatalf("append daily memory: %v", err)
	}

	if got := store.ReadToday(); !strings.Contains(got, "entry") {
		t.Fatalf("expected daily note to contain entry, got %q", got)
	}
}

func TestNewMemoryStoreFromConfig_NoopWhenMemoryDisabled(t *testing.T) {
	workspace := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Memory.Enabled = false

	store := newMemoryStoreFromConfig(cfg, workspace, nil, nil)
	if err := store.WriteLongTerm("noop"); err != nil {
		t.Fatalf("write long-term memory on noop backend: %v", err)
	}

	if got := store.ReadLongTerm(); got != "" {
		t.Fatalf("expected noop backend to ignore writes, got %q", got)
	}
}

func TestCallLLMWithFallback_RetriableErrorFallsBackAndMarksCooldown(t *testing.T) {
	primaryKind := failoverTestProviderKind(t, "primary")
	fallbackKind := failoverTestProviderKind(t, "fallback")

	primaryCalls := 0
	fallbackCalls := 0
	registerFailoverTestProvider(t, primaryKind, &primaryCalls, "", errors.New("status 429: too many requests"))
	registerFailoverTestProvider(t, fallbackKind, &fallbackCalls, "fallback-response", nil)

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "primary-model"
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "primary",
			ProviderKind: primaryKind,
			Models:       []string{"primary-model"},
			DefaultModel: "primary-model",
		},
		{
			Name:         "fallback",
			ProviderKind: fallbackKind,
			Models:       []string{"fallback-model"},
			DefaultModel: "fallback-model",
		},
	}

	ag := newFailoverTestAgent(t, cfg)
	clientCache := map[string]*providers.Client{}
	resp, providerUsed, modelUsed, err := ag.callLLMWithFallback(
		context.Background(),
		&providers.UnifiedRequest{Model: "primary-model"},
		"primary",
		[]string{"primary", "fallback"},
		"primary-model",
		clientCache,
	)
	if err != nil {
		t.Fatalf("callLLMWithFallback failed: %v", err)
	}
	if resp == nil || resp.Content != "fallback-response" {
		t.Fatalf("expected fallback response, got %#v", resp)
	}
	if providerUsed != "fallback" {
		t.Fatalf("expected fallback provider, got %q", providerUsed)
	}
	if modelUsed != "fallback-model" {
		t.Fatalf("expected fallback model, got %q", modelUsed)
	}
	if primaryCalls != 1 {
		t.Fatalf("expected primary to be called once, got %d", primaryCalls)
	}
	if fallbackCalls != 1 {
		t.Fatalf("expected fallback to be called once, got %d", fallbackCalls)
	}

	tracker := ag.getFailoverCooldown()
	if got := tracker.FailureCount("primary", providers.FailoverReasonRateLimit); got != 1 {
		t.Fatalf("expected one primary rate limit failure, got %d", got)
	}
	if tracker.IsAvailable("primary") {
		t.Fatalf("expected primary to be in cooldown after retriable failure")
	}
}

func TestCallLLMWithFallback_NonRetriableErrorStopsFallback(t *testing.T) {
	primaryKind := failoverTestProviderKind(t, "primary")
	fallbackKind := failoverTestProviderKind(t, "fallback")

	primaryCalls := 0
	fallbackCalls := 0
	registerFailoverTestProvider(t, primaryKind, &primaryCalls, "", errors.New("status 400: invalid request format"))
	registerFailoverTestProvider(t, fallbackKind, &fallbackCalls, "fallback-response", nil)

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "primary-model"
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "primary",
			ProviderKind: primaryKind,
			Models:       []string{"primary-model"},
			DefaultModel: "primary-model",
		},
		{
			Name:         "fallback",
			ProviderKind: fallbackKind,
			Models:       []string{"fallback-model"},
			DefaultModel: "fallback-model",
		},
	}

	ag := newFailoverTestAgent(t, cfg)
	_, providerUsed, modelUsed, err := ag.callLLMWithFallback(
		context.Background(),
		&providers.UnifiedRequest{Model: "primary-model"},
		"primary",
		[]string{"primary", "fallback"},
		"primary-model",
		map[string]*providers.Client{},
	)
	if err == nil {
		t.Fatalf("expected callLLMWithFallback error")
	}
	if providerUsed != "primary" {
		t.Fatalf("expected failed provider to still be reported, got %q", providerUsed)
	}
	if modelUsed != "primary-model" {
		t.Fatalf("expected failed model to still be reported, got %q", modelUsed)
	}

	failoverErr, ok := errors.AsType[*providers.FailoverError](err)
	if !ok {
		t.Fatalf("expected failover error, got %T: %v", err, err)
	}
	if failoverErr.Reason != providers.FailoverReasonFormat {
		t.Fatalf("expected format reason, got %s", failoverErr.Reason)
	}
	if primaryCalls != 1 {
		t.Fatalf("expected primary to be called once, got %d", primaryCalls)
	}
	if fallbackCalls != 0 {
		t.Fatalf("expected fallback not to be called, got %d", fallbackCalls)
	}
}

func TestCallLLMWithFallback_SkipsProviderInCooldownOnSubsequentAttempt(t *testing.T) {
	primaryKind := failoverTestProviderKind(t, "primary")
	fallbackKind := failoverTestProviderKind(t, "fallback")

	primaryCalls := 0
	fallbackCalls := 0
	registerFailoverTestProvider(t, primaryKind, &primaryCalls, "", errors.New("status 429: too many requests"))
	registerFailoverTestProvider(t, fallbackKind, &fallbackCalls, "fallback-response", nil)

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "primary-model"
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "primary",
			ProviderKind: primaryKind,
			Models:       []string{"primary-model"},
			DefaultModel: "primary-model",
		},
		{
			Name:         "fallback",
			ProviderKind: fallbackKind,
			Models:       []string{"fallback-model"},
			DefaultModel: "fallback-model",
		},
	}

	ag := newFailoverTestAgent(t, cfg)
	clientCache := map[string]*providers.Client{}
	providerOrder := []string{"primary", "fallback"}
	request := &providers.UnifiedRequest{Model: "primary-model"}

	firstResp, _, _, err := ag.callLLMWithFallback(
		context.Background(),
		request,
		"primary",
		providerOrder,
		"primary-model",
		clientCache,
	)
	if err != nil {
		t.Fatalf("first callLLMWithFallback failed: %v", err)
	}
	if firstResp == nil || firstResp.Content != "fallback-response" {
		t.Fatalf("expected fallback response in first attempt, got %#v", firstResp)
	}

	secondResp, _, _, err := ag.callLLMWithFallback(
		context.Background(),
		request,
		"primary",
		providerOrder,
		"primary-model",
		clientCache,
	)
	if err != nil {
		t.Fatalf("second callLLMWithFallback failed: %v", err)
	}
	if secondResp == nil || secondResp.Content != "fallback-response" {
		t.Fatalf("expected fallback response in second attempt, got %#v", secondResp)
	}

	if primaryCalls != 1 {
		t.Fatalf("expected primary to be called once due to cooldown skip, got %d", primaryCalls)
	}
	if fallbackCalls != 2 {
		t.Fatalf("expected fallback to be called twice, got %d", fallbackCalls)
	}
}

func TestCallLLMWithFallback_AllProvidersInCooldownReturnsExhaustedError(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "primary-model"
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "primary",
			ProviderKind: failoverTestProviderKind(t, "primary-cooldown"),
			Models:       []string{"primary-model"},
			DefaultModel: "primary-model",
		},
		{
			Name:         "fallback",
			ProviderKind: failoverTestProviderKind(t, "fallback-cooldown"),
			Models:       []string{"fallback-model"},
			DefaultModel: "fallback-model",
		},
	}

	ag := newFailoverTestAgent(t, cfg)
	tracker := ag.getFailoverCooldown()
	tracker.MarkFailure("primary", providers.FailoverReasonRateLimit)
	tracker.MarkFailure("fallback", providers.FailoverReasonRateLimit)

	_, _, _, err := ag.callLLMWithFallback(
		context.Background(),
		&providers.UnifiedRequest{Model: "primary-model"},
		"primary",
		[]string{"primary", "fallback"},
		"primary-model",
		map[string]*providers.Client{},
	)
	if err == nil {
		t.Fatal("expected fallback exhausted error")
	}

	exhaustedErr, ok := errors.AsType[*providers.FallbackExhaustedError](err)
	if !ok {
		t.Fatalf("expected fallback exhausted error, got %T: %v", err, err)
	}
	if len(exhaustedErr.Attempts) != 2 {
		t.Fatalf("expected two cooldown skips, got %d", len(exhaustedErr.Attempts))
	}
	if strings.Contains(err.Error(), "temporarily unavailable") == false {
		t.Fatalf("expected user-safe cooldown aggregate error, got %v", err)
	}
	if strings.Contains(err.Error(), "provider primary in cooldown") {
		t.Fatalf("expected final error not to expose raw cooldown line, got %v", err)
	}
}

func TestChatWithProviderModelDetailed_ReturnsActualRouteOnFailure(t *testing.T) {
	primaryKind := failoverTestProviderKind(t, "primary")
	registerFailoverTestProvider(t, primaryKind, new(int), "", errors.New("status 400: invalid request format"))

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = orchestratorLegacy
	cfg.Agents.Defaults.Provider = "primary"
	cfg.Agents.Defaults.Model = "primary-model"
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "primary",
			ProviderKind: primaryKind,
			Models:       []string{"primary-model"},
			DefaultModel: "primary-model",
		},
	}

	ag := newFailoverTestAgent(t, cfg)
	_, routeResult, err := ag.chatWithProviderModelDetailed(
		context.Background(),
		&testSession{},
		"hello",
		"primary",
		"primary-model",
		nil,
		PromptContext{},
	)
	if err == nil {
		t.Fatal("expected chatWithProviderModelDetailed error")
	}
	if routeResult.ActualProvider != "primary" {
		t.Fatalf("expected actual provider on failure, got %+v", routeResult)
	}
	if routeResult.ActualModel != "primary-model" {
		t.Fatalf("expected actual model on failure, got %+v", routeResult)
	}
}

func TestChatWithPromptContextDetailed_BladesReturnsActualRouteOnFailure(t *testing.T) {
	primaryKind := failoverTestProviderKind(t, "blades-primary")
	registerFailoverTestProvider(t, primaryKind, new(int), "", errors.New("status 400: invalid request format"))

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = orchestratorBlades
	cfg.Agents.Defaults.Provider = "primary"
	cfg.Agents.Defaults.Model = "primary-model"
	cfg.Providers = []config.ProviderProfile{
		{
			Name:         "primary",
			ProviderKind: primaryKind,
			Models:       []string{"primary-model"},
			DefaultModel: "primary-model",
		},
	}

	ag := newFailoverTestAgent(t, cfg)
	_, routeResult, err := ag.ChatWithPromptContextDetailed(
		context.Background(),
		&testSession{},
		"hello",
		PromptContext{
			RequestedProvider: "primary",
			RequestedModel:    "primary-model",
		},
	)
	if err == nil {
		t.Fatal("expected blades ChatWithPromptContextDetailed error")
	}
	if routeResult.ActualProvider != "primary" {
		t.Fatalf("expected actual provider on blades failure, got %+v", routeResult)
	}
	if routeResult.ActualModel != "primary-model" {
		t.Fatalf("expected actual model on blades failure, got %+v", routeResult)
	}
}

func TestChatRoutesThroughLegacyOrchestratorPath(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorLegacy)

	sess := &testSession{}
	_, err := ag.Chat(context.Background(), sess, "hello")
	if err == nil {
		t.Fatalf("expected chat error")
	}
	if !strings.Contains(err.Error(), "no providers configured") {
		t.Fatalf("expected legacy path no providers configured error, got %v", err)
	}
}

func TestChatRoutesThroughBladesOrchestratorPath(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)

	sess := &testSession{}
	_, err := ag.Chat(context.Background(), sess, "hello")
	if err == nil {
		t.Fatalf("expected chat error")
	}
	if !strings.Contains(err.Error(), "no providers configured") {
		t.Fatalf("expected blades path no providers configured error, got %v", err)
	}
}

func TestBuildBladesToolsResolver_MCPConfigError(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.config.Agents.Defaults.MCPServers = []config.MCPServerConfig{
		{
			Name:      "bad-timeout",
			Transport: "stdio",
			Command:   "npx",
			Timeout:   "invalid",
		},
	}

	_, _, err := ag.buildBladesToolsResolver()
	if err == nil {
		t.Fatalf("expected mcp resolver build error")
	}
	if !strings.Contains(err.Error(), "bad-timeout") {
		t.Fatalf("expected mcp server name in error, got %v", err)
	}
}

func TestBuildBladesToolsResolver_NoMCPConfig(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.config.Agents.Defaults.MCPServers = nil

	resolver, mcpResolver, err := ag.buildBladesToolsResolver()
	if err != nil {
		t.Fatalf("buildBladesToolsResolver failed: %v", err)
	}
	if resolver == nil {
		t.Fatalf("expected resolver")
	}
	if mcpResolver != nil {
		t.Fatalf("expected nil mcp resolver")
	}
}

func TestBuildBladesToolsResolver_ToolErrorReturnsResultInsteadOfAbort(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	failingTool := &toolExecutionResultStubTool{
		name:        "failing_tool",
		description: "always fails",
		err:         errors.New("boom"),
	}
	ag.tools.MustRegister(failingTool)

	resolver, _, err := ag.buildBladesToolsResolver()
	if err != nil {
		t.Fatalf("buildBladesToolsResolver failed: %v", err)
	}

	resolvedTools, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve tools failed: %v", err)
	}

	var selected bladestools.Tool
	for _, tool := range resolvedTools {
		if tool.Name() == failingTool.name {
			selected = tool
			break
		}
	}
	if selected == nil {
		t.Fatalf("expected tool %q in resolved tools", failingTool.name)
	}

	result, err := selected.Handle(context.Background(), "{}")
	if err != nil {
		t.Fatalf("expected tool handler to return result, got error: %v", err)
	}
	if result != "Error: boom" {
		t.Fatalf("expected error-as-result, got %q", result)
	}
	if failingTool.callCount() != 1 {
		t.Fatalf("expected failing tool execute once, got %d", failingTool.callCount())
	}
}

func TestExecuteToolCallPassesSessionIDToApproval(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	approvalMgr := approval.NewManager(approval.Config{Mode: approval.ModeManual})
	ag.approval = approvalMgr
	ag.tools.MustRegister(&toolExecutionResultStubTool{
		name:        "approval_test_tool",
		description: "captures approval flow",
	})

	ctx := context.WithValue(context.Background(), promptContextSessionKey, "wechat-user-1")
	result, err := ag.executeToolCall(ctx, providers.UnifiedToolCall{
		Name:      "approval_test_tool",
		Arguments: map[string]interface{}{"x": 1},
	})
	if err != nil {
		t.Fatalf("executeToolCall failed: %v", err)
	}
	if result != "Tool call pending approval" {
		t.Fatalf("unexpected tool result: %q", result)
	}

	pending := approvalMgr.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(pending))
	}
	if pending[0].SessionID != "wechat-user-1" {
		t.Fatalf("expected session ID to propagate, got %q", pending[0].SessionID)
	}
}

func TestExecuteToolCallInjectsSessionIDIntoToolContext(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	tool := &toolExecutionResultStubTool{
		name:        "context_session_tool",
		description: "captures tool context session id",
	}
	ag.tools.MustRegister(tool)

	ctx := context.WithValue(context.Background(), promptContextSessionKey, "wechat-user-2")
	result, err := ag.executeToolCall(ctx, providers.UnifiedToolCall{
		Name:      "context_session_tool",
		Arguments: map[string]interface{}{"x": 1},
	})
	if err != nil {
		t.Fatalf("executeToolCall failed: %v", err)
	}
	if result != "ok" {
		t.Fatalf("unexpected tool result: %q", result)
	}
	if tool.lastSessionID != "wechat-user-2" {
		t.Fatalf("expected session id in tool context, got %q", tool.lastSessionID)
	}
}

func TestRegisterUndoToolReplacesExistingUndoTool(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Undo.Enabled = true
	cfg.Undo.MaxTurns = 20

	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.config = cfg
	ag.snapshotMgr = session.NewSnapshotManager(t.TempDir(), cfg.Undo)

	ag.RegisterUndoTool("session-a")
	first, ok := ag.tools.Get("undo")
	if !ok {
		t.Fatal("expected undo tool to be registered")
	}

	ag.RegisterUndoTool("session-b")
	second, ok := ag.tools.Get("undo")
	if !ok {
		t.Fatal("expected undo tool to remain registered")
	}
	if first == second {
		t.Fatal("expected undo tool registration to replace existing instance")
	}
}

func TestSetApprovalModeForSessionRejectsUnknownMode(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.approval = approval.NewManager(approval.Config{Mode: approval.ModePrompt})

	err := ag.SetApprovalModeForSession("wechat-user-1", approval.Mode("weird"))
	if err == nil {
		t.Fatal("expected error for unsupported approval mode")
	}
}

func TestSetApprovalModeForSessionUpdatesTaskStore(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.approval = approval.NewManager(approval.Config{Mode: approval.ModePrompt})
	ag.taskStore = tasks.NewStore()

	if err := ag.SetApprovalModeForSession("wechat-user-1", approval.ModeManual); err != nil {
		t.Fatalf("SetApprovalModeForSession failed: %v", err)
	}

	states := ag.taskStore.ListSessionStates()
	if len(states) != 1 {
		t.Fatalf("expected one session state, got %d", len(states))
	}
	if states[0].PermissionMode != string(approval.ModeManual) {
		t.Fatalf("expected manual permission mode, got %q", states[0].PermissionMode)
	}

	if err := ag.ClearApprovalModeForSession("wechat-user-1"); err != nil {
		t.Fatalf("ClearApprovalModeForSession failed: %v", err)
	}
	if states := ag.taskStore.ListSessionStates(); len(states) != 0 {
		t.Fatalf("expected no session states after clear, got %+v", states)
	}
}

func TestExecuteToolCallTracksPendingApprovalInTaskStore(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.approval = approval.NewManager(approval.Config{Mode: approval.ModeManual})
	ag.taskStore = tasks.NewStore()
	ag.tools.MustRegister(&toolExecutionResultStubTool{
		name:        "approval_runtime_tool",
		description: "captures runtime approval state",
	})

	ctx := context.WithValue(context.Background(), promptContextSessionKey, "wechat-user-1")
	result, err := ag.executeToolCall(ctx, providers.UnifiedToolCall{
		Name:      "approval_runtime_tool",
		Arguments: map[string]interface{}{"x": 1},
	})
	if err != nil {
		t.Fatalf("executeToolCall failed: %v", err)
	}
	if result != "Tool call pending approval" {
		t.Fatalf("unexpected tool result: %q", result)
	}

	states := ag.taskStore.ListSessionStates()
	if len(states) != 1 {
		t.Fatalf("expected one session state, got %d", len(states))
	}
	if states[0].SessionID != "wechat-user-1" {
		t.Fatalf("expected session id wechat-user-1, got %q", states[0].SessionID)
	}
	if states[0].PendingAction != "approval_runtime_tool" {
		t.Fatalf("expected pending action approval_runtime_tool, got %q", states[0].PendingAction)
	}
	if states[0].PendingRequestID == "" {
		t.Fatal("expected pending request id to be tracked")
	}
}

func TestExecuteToolCallPermissionRuleAllowBypassesApproval(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.approval = approval.NewManager(approval.Config{Mode: approval.ModeManual})
	tool := &toolExecutionResultStubTool{
		name:        "permission_allow_tool",
		description: "allowed by permission rule",
	}
	ag.tools.MustRegister(tool)
	ag.permissionRules = newTestPermissionRuleManager(t, permissionrules.Rule{
		ToolName: "permission_allow_tool",
		Action:   permissionrules.ActionAllow,
		Enabled:  true,
	})

	ctx := context.WithValue(context.Background(), promptContextSessionKey, "wechat-user-allow")
	result, err := ag.executeToolCall(ctx, providers.UnifiedToolCall{
		Name:      "permission_allow_tool",
		Arguments: map[string]interface{}{"x": 1},
	})
	if err != nil {
		t.Fatalf("executeToolCall failed: %v", err)
	}
	if result != "ok" {
		t.Fatalf("unexpected tool result: %q", result)
	}
	if tool.callCount() != 1 {
		t.Fatalf("expected tool to execute once, got %d", tool.callCount())
	}
	if len(ag.approval.GetPending()) != 0 {
		t.Fatalf("expected no pending approvals, got %d", len(ag.approval.GetPending()))
	}
}

func TestExecuteToolCallPermissionRuleDenyBlocksTool(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.approval = approval.NewManager(approval.Config{Mode: approval.ModeAuto})
	tool := &toolExecutionResultStubTool{
		name:        "permission_deny_tool",
		description: "denied by permission rule",
	}
	ag.tools.MustRegister(tool)
	ag.permissionRules = newTestPermissionRuleManager(t, permissionrules.Rule{
		ToolName: "permission_deny_tool",
		Action:   permissionrules.ActionDeny,
		Enabled:  true,
	})

	result, err := ag.executeToolCall(context.Background(), providers.UnifiedToolCall{
		Name:      "permission_deny_tool",
		Arguments: map[string]interface{}{"x": 1},
	})
	if err != nil {
		t.Fatalf("executeToolCall failed: %v", err)
	}
	if result != "Tool call denied by permission rule" {
		t.Fatalf("unexpected tool result: %q", result)
	}
	if tool.callCount() != 0 {
		t.Fatalf("expected tool not to execute, got %d", tool.callCount())
	}
}

func TestExecuteToolCallPermissionRuleAskQueuesApproval(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.approval = approval.NewManager(approval.Config{Mode: approval.ModeAuto})
	ag.taskStore = tasks.NewStore()
	ag.tools.MustRegister(&toolExecutionResultStubTool{
		name:        "permission_ask_tool",
		description: "asks by permission rule",
	})
	ag.permissionRules = newTestPermissionRuleManager(t, permissionrules.Rule{
		ToolName: "permission_ask_tool",
		Action:   permissionrules.ActionAsk,
		Enabled:  true,
	})

	ctx := context.WithValue(context.Background(), promptContextSessionKey, "wechat-user-ask")
	result, err := ag.executeToolCall(ctx, providers.UnifiedToolCall{
		Name:      "permission_ask_tool",
		Arguments: map[string]interface{}{"x": 1},
	})
	if err != nil {
		t.Fatalf("executeToolCall failed: %v", err)
	}
	if result != "Tool call pending approval" {
		t.Fatalf("unexpected tool result: %q", result)
	}
	pending := ag.approval.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(pending))
	}
	if pending[0].ToolName != "permission_ask_tool" {
		t.Fatalf("unexpected pending request: %+v", pending[0])
	}
	states := ag.taskStore.ListSessionStates()
	if len(states) != 1 || states[0].PendingRequestID == "" {
		t.Fatalf("expected pending approval state to be tracked, got %+v", states)
	}
}

func TestExecuteToolCallPermissionRuleFallsBackToApprovalMode(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.approval = approval.NewManager(approval.Config{Mode: approval.ModeManual})
	ag.permissionRules = newTestPermissionRuleManager(t)
	ag.tools.MustRegister(&toolExecutionResultStubTool{
		name:        "permission_fallback_tool",
		description: "falls back to approval mode",
	})

	ctx := context.WithValue(context.Background(), promptContextSessionKey, "wechat-user-fallback")
	result, err := ag.executeToolCall(ctx, providers.UnifiedToolCall{
		Name:      "permission_fallback_tool",
		Arguments: map[string]interface{}{"x": 1},
	})
	if err != nil {
		t.Fatalf("executeToolCall failed: %v", err)
	}
	if result != "Tool call pending approval" {
		t.Fatalf("unexpected tool result: %q", result)
	}
	if len(ag.approval.GetPending()) != 1 {
		t.Fatalf("expected fallback approval pending request, got %d", len(ag.approval.GetPending()))
	}
}

func TestExecuteToolCallPermissionRuleUsesRuntimeID(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.approval = approval.NewManager(approval.Config{Mode: approval.ModeManual})
	tool := &toolExecutionResultStubTool{
		name:        "permission_runtime_tool",
		description: "allowed only for runtime scoped rule",
	}
	ag.tools.MustRegister(tool)
	ag.permissionRules = newTestPermissionRuleManager(t,
		permissionrules.Rule{
			ToolName:  "permission_runtime_tool",
			RuntimeID: "runtime-1",
			Action:    permissionrules.ActionAllow,
			Enabled:   true,
		},
		permissionrules.Rule{
			ToolName: "permission_runtime_tool",
			Action:   permissionrules.ActionDeny,
			Priority: -1,
			Enabled:  true,
		},
	)

	ctx := context.WithValue(context.Background(), promptContextRuntimeKey, "runtime-1")
	result, err := ag.executeToolCall(ctx, providers.UnifiedToolCall{
		Name:      "permission_runtime_tool",
		Arguments: map[string]interface{}{"x": 1},
	})
	if err != nil {
		t.Fatalf("executeToolCall failed: %v", err)
	}
	if result != "ok" {
		t.Fatalf("unexpected tool result: %q", result)
	}
	if tool.callCount() != 1 {
		t.Fatalf("expected tool to execute, got %d", tool.callCount())
	}
}

func TestBuildBladesToolsResolver_UsesBladesMemoryToolAndSkipsLegacyMemoryTool(t *testing.T) {
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.config.Memory.Enabled = true
	ag.config.Memory.Semantic.Enabled = true
	ag.config.Memory.Semantic.DefaultTopK = 3
	ag.semanticMemory = &stubSearchManager{enabled: true}
	ag.tools.MustRegister(&toolExecutionResultStubTool{
		name:        "memory",
		description: "legacy memory tool",
	})

	resolver, _, err := ag.buildBladesToolsResolver()
	if err != nil {
		t.Fatalf("buildBladesToolsResolver failed: %v", err)
	}

	resolvedTools, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve tools failed: %v", err)
	}

	count := 0
	for _, tool := range resolvedTools {
		if tool.Name() == "memory" || tool.Name() == "Memory" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one blades memory tool, got %d", count)
	}
}

func TestBladesModelProvider_ToolCallResponseDropsAssistantText(t *testing.T) {
	toolCall := providers.UnifiedToolCall{
		ID:   "call-1",
		Name: "tool-1",
		Arguments: map[string]interface{}{
			"k": "v",
		},
	}

	provider := &bladesModelProvider{}
	modelResp := provider.toModelResponse(&providers.UnifiedResponse{
		Content:   "assistant preamble",
		ToolCalls: []providers.UnifiedToolCall{toolCall},
	})

	if modelResp == nil || modelResp.Message == nil {
		t.Fatalf("expected model response message")
	}
	if modelResp.Message.Role != blades.RoleTool {
		t.Fatalf("expected role %q, got %q", blades.RoleTool, modelResp.Message.Role)
	}
	if len(modelResp.Message.Parts) != 1 {
		t.Fatalf("expected exactly one tool part, got %d", len(modelResp.Message.Parts))
	}

	part, ok := modelResp.Message.Parts[0].(blades.ToolPart)
	if !ok {
		t.Fatalf("expected first part to be ToolPart, got %T", modelResp.Message.Parts[0])
	}
	if part.ID != toolCall.ID {
		t.Fatalf("expected tool id %q, got %q", toolCall.ID, part.ID)
	}
	if part.Name != toolCall.Name {
		t.Fatalf("expected tool name %q, got %q", toolCall.Name, part.Name)
	}
}

func TestBladesModelProvider_ConvertMessagesPreservesMultipleToolResults(t *testing.T) {
	provider := &bladesModelProvider{}
	messages, err := provider.convertMessages([]*blades.Message{
		{
			Role: blades.RoleTool,
			Parts: []blades.Part{
				blades.ToolPart{ID: "call-1", Response: "result-1"},
				blades.ToolPart{ID: "call-2", Response: "result-2"},
			},
		},
	})
	if err != nil {
		t.Fatalf("convertMessages failed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected two tool messages, got %d", len(messages))
	}
	if messages[0].Role != "tool" || messages[0].ToolCallID != "call-1" || messages[0].Content != "result-1" {
		t.Fatalf("unexpected first tool message: %#v", messages[0])
	}
	if messages[1].Role != "tool" || messages[1].ToolCallID != "call-2" || messages[1].Content != "result-2" {
		t.Fatalf("unexpected second tool message: %#v", messages[1])
	}
}

func TestBladesModelProvider_ConvertMessagesToolFallbackToRequest(t *testing.T) {
	provider := &bladesModelProvider{}
	messages, err := provider.convertMessages([]*blades.Message{
		{
			Role: blades.RoleTool,
			Parts: []blades.Part{
				blades.ToolPart{ID: "call-1", Request: "{\"x\":1}"},
			},
		},
	})
	if err != nil {
		t.Fatalf("convertMessages failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one tool message, got %d", len(messages))
	}
	if messages[0].ToolCallID != "call-1" {
		t.Fatalf("expected tool call id %q, got %q", "call-1", messages[0].ToolCallID)
	}
	if messages[0].Content != "{\"x\":1}" {
		t.Fatalf("expected request fallback content, got %q", messages[0].Content)
	}
}

func TestToBladesMessage_AssistantToolCallsPreserved(t *testing.T) {
	msg := Message{
		Role:    "assistant",
		Content: "thinking",
		ToolCalls: []ToolCall{{
			ID:   "call-1",
			Name: "read_file",
			Arguments: map[string]interface{}{
				"path": "README.md",
			},
		}},
	}

	bladesMsg := toBladesMessage(msg)
	if bladesMsg == nil {
		t.Fatalf("expected non-nil blades message")
	}
	if bladesMsg.Role != blades.RoleAssistant {
		t.Fatalf("expected role %q, got %q", blades.RoleAssistant, bladesMsg.Role)
	}
	if len(bladesMsg.Parts) != 2 {
		t.Fatalf("expected 2 parts (text+tool), got %d", len(bladesMsg.Parts))
	}
	if _, ok := bladesMsg.Parts[0].(blades.TextPart); !ok {
		t.Fatalf("expected first part to be TextPart, got %T", bladesMsg.Parts[0])
	}
	part, ok := bladesMsg.Parts[1].(blades.ToolPart)
	if !ok {
		t.Fatalf("expected second part to be ToolPart, got %T", bladesMsg.Parts[1])
	}
	if part.ID != "call-1" {
		t.Fatalf("expected tool id %q, got %q", "call-1", part.ID)
	}
	if part.Name != "read_file" {
		t.Fatalf("expected tool name %q, got %q", "read_file", part.Name)
	}
	if part.Request == "" {
		t.Fatalf("expected non-empty tool request")
	}
}

func TestHasBladesHistoryContent(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
		want bool
	}{
		{
			name: "assistant tool calls without text",
			msg: Message{
				Role: "assistant",
				ToolCalls: []ToolCall{{
					ID:   "call-1",
					Name: "tool",
				}},
			},
			want: true,
		},
		{
			name: "tool with call id only",
			msg: Message{
				Role:       "tool",
				ToolCallID: "call-1",
			},
			want: true,
		},
		{
			name: "empty user text",
			msg: Message{
				Role:    "user",
				Content: "   ",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasBladesHistoryContent(tt.msg); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestBladesModelProvider_AssistantResponseKeepsTextPart(t *testing.T) {
	provider := &bladesModelProvider{}
	modelResp := provider.toModelResponse(&providers.UnifiedResponse{Content: "final answer"})

	if modelResp == nil || modelResp.Message == nil {
		t.Fatalf("expected model response message")
	}
	if modelResp.Message.Role != blades.RoleAssistant {
		t.Fatalf("expected role %q, got %q", blades.RoleAssistant, modelResp.Message.Role)
	}
	if len(modelResp.Message.Parts) != 1 {
		t.Fatalf("expected one text part, got %d", len(modelResp.Message.Parts))
	}

	part, ok := modelResp.Message.Parts[0].(blades.TextPart)
	if !ok {
		t.Fatalf("expected first part to be TextPart, got %T", modelResp.Message.Parts[0])
	}
	if part.Text != "final answer" {
		t.Fatalf("expected text %q, got %q", "final answer", part.Text)
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

	workspace := t.TempDir()
	memoryStore := newMemoryStoreFromConfig(cfg, workspace, nil, nil)
	kvStore := newTestKVStore(t)
	ag := &Agent{
		config:           cfg,
		logger:           log,
		context:          NewContextBuilderWithMemory(workspace, memoryStore),
		tools:            tools.NewRegistry(),
		acpSessions:      make(map[string]*acpSessionState),
		acpRuntime:       make(map[string]string),
		kvStore:          kvStore,
		failoverCooldown: providers.NewCooldownTracker(),
		providerGroups:   newProviderGroupPlanner(),
		maxIterations:    1,
	}
	ag.context.SetToolDescriptionsFunc(ag.tools.GetDescriptions)

	return ag
}

func newTestPermissionRuleManager(t *testing.T, rules ...permissionrules.Rule) *permissionrules.Manager {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	client := newRuntimeEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})

	mgr, err := permissionrules.NewManager(cfg, testLogger(t), client)
	if err != nil {
		t.Fatalf("new permission rule manager: %v", err)
	}
	for _, rule := range rules {
		if _, err := mgr.Create(context.Background(), rule); err != nil {
			t.Fatalf("seed permission rule failed: %v", err)
		}
	}
	return mgr
}

func TestProvideAgent_WiresPermissionRuleManager(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := testLogger(t)
	client := newRuntimeEntClient(t, cfg)
	t.Cleanup(func() {
		_ = client.Close()
	})

	skillsMgr := skills.NewManager(log, filepath.Join(cfg.WorkspacePath(), "skills"), false)
	processMgr := process.NewManager(log)
	approvalMgr := approval.NewManager(approval.Config{Mode: approval.ModeManual})
	providerMgr, err := providerstore.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	promptMgr, err := prompts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}
	rulesMgr, err := permissionrules.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new permission rules manager: %v", err)
	}
	localBus := bus.NewLocalBus(log, 8)
	if err := localBus.Start(); err != nil {
		t.Fatalf("start local bus: %v", err)
	}
	t.Cleanup(func() {
		if err := localBus.Stop(); err != nil {
			t.Fatalf("stop local bus: %v", err)
		}
	})

	var ag *Agent
	app := fx.New(
		fx.Supply(cfg, log, client, skillsMgr, processMgr, approvalMgr, providerMgr, promptMgr, rulesMgr),
		fx.Provide(func() bus.Bus { return localBus }),
		Module,
		fx.Populate(&ag),
		fx.NopLogger,
	)
	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("start fx app: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Stop(context.Background()); err != nil {
			t.Fatalf("stop fx app: %v", err)
		}
	})

	if ag == nil {
		t.Fatal("expected agent to be populated")
	}
	if ag.permissionRules == nil {
		t.Fatal("expected permission rules manager to be wired")
	}
	if ag.permissionRules != rulesMgr {
		t.Fatal("expected provided permission rules manager instance to be attached to agent")
	}
}

type failoverTestAdaptor struct {
	callCount *int
	content   string
	err       error
	onRequest func(*providers.UnifiedRequest)
	responses []*providers.UnifiedResponse
	responseN int
}

func (a *failoverTestAdaptor) Init(info *providers.RelayInfo) error {
	_ = info
	return nil
}

func (a *failoverTestAdaptor) GetRequestURL(info *providers.RelayInfo) (string, error) {
	_ = info
	return "https://example.com/v1/chat/completions", nil
}

func (a *failoverTestAdaptor) SetupRequestHeader(req *http.Request, info *providers.RelayInfo) error {
	_ = info
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *failoverTestAdaptor) ConvertRequest(unified *providers.UnifiedRequest, info *providers.RelayInfo) ([]byte, error) {
	if a.onRequest != nil && unified != nil {
		clone := &providers.UnifiedRequest{
			Model:       unified.Model,
			MaxTokens:   unified.MaxTokens,
			Temperature: unified.Temperature,
		}
		if len(unified.Messages) > 0 {
			clone.Messages = append([]providers.UnifiedMessage(nil), unified.Messages...)
		}
		if len(unified.Tools) > 0 {
			clone.Tools = append([]providers.UnifiedTool(nil), unified.Tools...)
		}
		if unified.Extra != nil {
			clone.Extra = map[string]interface{}{}
			for k, v := range unified.Extra {
				clone.Extra[k] = v
			}
		}
		a.onRequest(clone)
	}
	_ = unified
	_ = info
	return []byte(`{"ok":true}`), nil
}

func (a *failoverTestAdaptor) DoRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	_ = ctx
	_ = req
	if a.callCount != nil {
		*a.callCount++
	}
	if a.err != nil {
		return nil, a.err
	}
	return []byte(a.content), nil
}

func (a *failoverTestAdaptor) DoResponse(body []byte, info *providers.RelayInfo) (*providers.UnifiedResponse, error) {
	_ = body
	_ = info
	if len(a.responses) > 0 {
		idx := a.responseN
		if idx >= len(a.responses) {
			idx = len(a.responses) - 1
		}
		a.responseN++
		return a.responses[idx], nil
	}
	return &providers.UnifiedResponse{
		Content:      a.content,
		FinishReason: "stop",
	}, nil
}

func (a *failoverTestAdaptor) DoStreamResponse(ctx context.Context, reader io.Reader, handler providers.StreamHandler, info *providers.RelayInfo) error {
	_ = ctx
	_ = reader
	_ = handler
	_ = info
	return nil
}

func (a *failoverTestAdaptor) GetModelList() ([]string, error) {
	return nil, nil
}

func failoverTestProviderKind(t *testing.T, label string) string {
	t.Helper()
	replacer := strings.NewReplacer(" ", "-", "/", "-", ":", "-")
	return "failover-test-" + replacer.Replace(t.Name()) + "-" + label
}

func registerFailoverTestProvider(t *testing.T, providerKind string, callCount *int, content string, err error) {
	registerFailoverTestProviderWithCapture(t, providerKind, callCount, content, err, nil)
}

func registerFailoverTestProviderWithCapture(
	t *testing.T,
	providerKind string,
	callCount *int,
	content string,
	err error,
	onRequest func(*providers.UnifiedRequest),
) {
	t.Helper()
	providers.Register(providerKind, func() providers.Adaptor {
		return &failoverTestAdaptor{
			callCount: callCount,
			content:   content,
			err:       err,
			onRequest: onRequest,
		}
	})
	t.Cleanup(func() {
		providers.Unregister(providerKind)
	})
}

func registerFailoverTestProviderWithResponses(
	t *testing.T,
	providerKind string,
	callCount *int,
	responses []*providers.UnifiedResponse,
	onRequest func(*providers.UnifiedRequest),
) {
	t.Helper()
	providers.Register(providerKind, func() providers.Adaptor {
		return &failoverTestAdaptor{
			callCount: callCount,
			onRequest: onRequest,
			responses: responses,
		}
	})
	t.Cleanup(func() {
		providers.Unregister(providerKind)
	})
}

func newFailoverTestAgent(t *testing.T, cfg *config.Config) *Agent {
	t.Helper()

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	workspace := t.TempDir()
	memoryStore := newMemoryStoreFromConfig(cfg, workspace, nil, nil)
	ag := &Agent{
		config:           cfg,
		logger:           log,
		context:          NewContextBuilderWithMemory(workspace, memoryStore),
		tools:            tools.NewRegistry(),
		failoverCooldown: providers.NewCooldownTracker(),
		providerGroups:   newProviderGroupPlanner(),
		maxIterations:    1,
	}
	ag.context.SetToolDescriptionsFunc(ag.tools.GetDescriptions)
	return ag
}

func TestChatEnforcesMaxToolRoundsPerSession(t *testing.T) {
	providerKind := failoverTestProviderKind(t, "tool-round-limit")
	callCount := new(int)
	registerFailoverTestProviderWithResponses(t, providerKind, callCount, []*providers.UnifiedResponse{
		{
			ToolCalls: []providers.UnifiedToolCall{{
				ID:        "call-1",
				Name:      "stub_tool",
				Arguments: map[string]interface{}{},
			}},
			FinishReason: "tool_calls",
		},
		{
			ToolCalls: []providers.UnifiedToolCall{{
				ID:        "call-2",
				Name:      "stub_tool",
				Arguments: map[string]interface{}{},
			}},
			FinishReason: "tool_calls",
		},
	}, nil)

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Orchestrator = orchestratorLegacy
	cfg.Agents.Defaults.Provider = "primary"
	cfg.Agents.Defaults.Model = "test-model"
	cfg.Providers = []config.ProviderProfile{{
		Name:         "primary",
		ProviderKind: providerKind,
		Models:       []string{"test-model"},
		DefaultModel: "test-model",
	}}

	ag := newFailoverTestAgent(t, cfg)
	ag.maxIterations = 3
	stubTool := &toolExecutionResultStubTool{
		name:        "stub_tool",
		description: "stub tool",
	}
	ag.tools.MustRegister(stubTool)
	ag.taskStore = tasks.NewStore()
	ag.taskStore.SetSessionToolRoundLimit("sess-1", 1)

	sess := &testSession{}
	_, _, err := ag.ChatWithPromptContextDetailed(context.Background(), sess, "hello", PromptContext{
		SessionID:         "sess-1",
		RequestedProvider: "primary",
		RequestedModel:    "test-model",
	})
	if err == nil {
		t.Fatal("expected tool round limit error")
	}
	if !strings.Contains(err.Error(), "max tool rounds (1) reached for session sess-1") {
		t.Fatalf("unexpected error: %v", err)
	}
	if stubTool.callCount() != 1 {
		t.Fatalf("expected exactly one executed tool call before enforcement, got %d", stubTool.callCount())
	}
	state, ok := ag.taskStore.GetSessionState("sess-1")
	if !ok {
		t.Fatal("expected session state to exist")
	}
	if state.ToolRounds != 1 {
		t.Fatalf("expected tool rounds to stop at 1, got %d", state.ToolRounds)
	}
	if state.ToolCalls["stub_tool"] != 1 {
		t.Fatalf("expected tool call count to stop at 1, got %+v", state.ToolCalls)
	}
}

func TestExecuteToolCallEnforcesPerToolLimitsPerSession(t *testing.T) {
	cfg := config.DefaultConfig()
	log := testLogger(t)
	ag, err := New(cfg, log, nil, nil, approval.NewManager(approval.Config{Mode: approval.ModeAuto}), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New agent failed: %v", err)
	}
	stubTool := &toolExecutionResultStubTool{
		name:        "stub_tool",
		description: "stub tool",
	}
	ag.tools.MustRegister(stubTool)
	ag.taskStore = tasks.NewStore()
	ag.taskStore.SetSessionToolCallLimit("sess-1", "stub_tool", 1)

	ctx := context.WithValue(context.Background(), promptContextSessionKey, "sess-1")

	if _, err := ag.executeToolCall(ctx, providers.UnifiedToolCall{
		Name:      "stub_tool",
		Arguments: map[string]interface{}{},
	}); err != nil {
		t.Fatalf("expected first tool call to succeed, got %v", err)
	}
	if _, err := ag.executeToolCall(ctx, providers.UnifiedToolCall{
		Name:      "stub_tool",
		Arguments: map[string]interface{}{},
	}); err == nil {
		t.Fatal("expected second tool call to be blocked by per-tool limit")
	} else if !strings.Contains(err.Error(), "tool stub_tool reached per-session call limit (1) for session sess-1") {
		t.Fatalf("unexpected per-tool limit error: %v", err)
	}
	if stubTool.callCount() != 1 {
		t.Fatalf("expected only one actual tool execution, got %d", stubTool.callCount())
	}
}

func testLogger(t *testing.T) *logger.Logger {
	t.Helper()

	logCfg := logger.DefaultConfig()
	logCfg.OutputPath = ""
	logCfg.Development = true
	log, err := logger.New(logCfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	return log
}

type toolExecutionResultStubTool struct {
	name        string
	description string
	err         error

	mu            sync.Mutex
	executeHits   int
	lastSessionID string
}

func (t *toolExecutionResultStubTool) Name() string {
	return t.name
}

func (t *toolExecutionResultStubTool) Description() string {
	return t.description
}

func (t *toolExecutionResultStubTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *toolExecutionResultStubTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	_ = args

	t.mu.Lock()
	t.executeHits++
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		t.lastSessionID = sessionID
	}
	t.mu.Unlock()

	if t.err != nil {
		return "", t.err
	}
	return "ok", nil
}

func (t *toolExecutionResultStubTool) callCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.executeHits
}

type stubSearchManager struct {
	enabled bool
}

func (s *stubSearchManager) Search(ctx context.Context, query string, opts memory.SearchOptions) ([]*memory.SearchResult, error) {
	_ = ctx
	_ = query
	_ = opts
	return nil, nil
}

func (s *stubSearchManager) Add(ctx context.Context, text string, source memory.Source, typ memory.Type, metadata memory.Metadata) error {
	_ = ctx
	_ = text
	_ = source
	_ = typ
	_ = metadata
	return nil
}

func (s *stubSearchManager) Status() map[string]interface{} {
	return map[string]interface{}{"backend": "stub"}
}

func (s *stubSearchManager) Close() error {
	return nil
}

func (s *stubSearchManager) IsEnabled() bool {
	return s.enabled
}

type testSession struct {
	messages []Message
}

func (s *testSession) GetMessages() []Message {
	return s.messages
}

func (s *testSession) GetHistorySafe(limit int) []Message {
	if limit <= 0 || limit >= len(s.messages) {
		return s.messages
	}

	start := len(s.messages) - limit
	for start > 0 {
		msg := s.messages[start]
		if msg.Role == "tool" {
			start--
			continue
		}
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			complete := true
			for _, tc := range msg.ToolCalls {
				found := false
				for i := start + 1; i < len(s.messages); i++ {
					if s.messages[i].Role == "assistant" || s.messages[i].Role == "user" || s.messages[i].Role == "system" {
						break
					}
					if s.messages[i].Role == "tool" && s.messages[i].ToolCallID == tc.ID {
						found = true
						break
					}
				}
				if !found {
					complete = false
					break
				}
			}
			if !complete {
				start--
				continue
			}
		}
		break
	}

	return s.messages[start:]
}

func (s *testSession) AddMessage(msg Message) {
	s.messages = append(s.messages, msg)
}

func TestLoadBootstrapFiles_IgnoresStaticAgentsBootstrap(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte("- **Model:** claude-3-5-sonnet-20241022"), 0644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("soul-note"), 0644); err != nil {
		t.Fatalf("write SOUL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "IDENTITY.md"), []byte("identity-note"), 0644); err != nil {
		t.Fatalf("write IDENTITY.md: %v", err)
	}

	cb := NewContextBuilder(workspace)
	content := cb.LoadBootstrapFiles()
	if strings.Contains(content, "claude-3-5-sonnet-20241022") {
		t.Fatalf("expected AGENTS.md bootstrap to be ignored, got %q", content)
	}
	if !strings.Contains(content, "soul-note") {
		t.Fatalf("expected SOUL.md content, got %q", content)
	}
	if !strings.Contains(content, "identity-note") {
		t.Fatalf("expected IDENTITY.md content, got %q", content)
	}
}
