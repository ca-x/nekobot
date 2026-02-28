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
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/providers"
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

func TestBuildToolsSection_SortsToolDescriptionsDeterministically(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilderWithMemory(workspace, NewMemoryStoreWithBackend(workspace, &memoryNoopBackend{}))
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
	if !(alphaIdx < middleIdx && middleIdx < zetaIdx) {
		t.Fatalf("expected sorted tool descriptions, got section: %q", section)
	}
}

func TestBuildSystemPrompt_UsesCurrentTimePlaceholderReplacement(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilderWithMemory(workspace, NewMemoryStoreWithBackend(workspace, &memoryNoopBackend{}))
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
	cb := NewContextBuilderWithMemory(workspace, NewMemoryStoreWithBackend(workspace, &memoryNoopBackend{}))
	cb.SetToolDescriptionsFunc(func() []string { return nil })

	first := cb.BuildSystemPrompt()
	if err := os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte("agent-note"), 0644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	second := cb.BuildSystemPrompt()
	if first == second {
		t.Fatalf("expected prompt to change after bootstrap file update")
	}
	if !strings.Contains(second, "agent-note") {
		t.Fatalf("expected updated bootstrap content in prompt")
	}
}

func TestBuildSystemPrompt_CacheRefreshesOnToolDescriptionChange(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilderWithMemory(workspace, NewMemoryStoreWithBackend(workspace, &memoryNoopBackend{}))
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

func TestBuildMessages_DeduplicatesTrailingCurrentUserMessage(t *testing.T) {
	workspace := t.TempDir()
	cb := NewContextBuilderWithMemory(workspace, NewMemoryStoreWithBackend(workspace, &memoryNoopBackend{}))
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
	cb := NewContextBuilderWithMemory(workspace, NewMemoryStoreWithBackend(workspace, &memoryNoopBackend{}))
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
	_, _, _, err := ag.callLLMWithFallback(
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

	var failoverErr *providers.FailoverError
	if !errors.As(err, &failoverErr) {
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
	if !strings.Contains(err.Error(), "blades runner run: llm call with fallback: provider not found: missing") {
		t.Fatalf("expected blades path provider error, got %v", err)
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
	ag := &Agent{
		config:           cfg,
		logger:           log,
		context:          NewContextBuilderWithMemory(workspace, memoryStore),
		tools:            tools.NewRegistry(),
		failoverCooldown: providers.NewCooldownTracker(),
		maxIterations:    1,
	}
	ag.context.SetToolDescriptionsFunc(ag.tools.GetDescriptions)

	return ag
}

type failoverTestAdaptor struct {
	callCount *int
	content   string
	err       error
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
	t.Helper()
	providers.Register(providerKind, func() providers.Adaptor {
		return &failoverTestAdaptor{callCount: callCount, content: content, err: err}
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
		maxIterations:    1,
	}
	ag.context.SetToolDescriptionsFunc(ag.tools.GetDescriptions)
	return ag
}

type toolExecutionResultStubTool struct {
	name        string
	description string
	err         error

	mu          sync.Mutex
	executeHits int
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
	_ = ctx
	_ = args

	t.mu.Lock()
	t.executeHits++
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

type testSession struct {
	messages []Message
}

func (s *testSession) GetMessages() []Message {
	return s.messages
}

func (s *testSession) AddMessage(msg Message) {
	s.messages = append(s.messages, msg)
}
