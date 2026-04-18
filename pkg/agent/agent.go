package agent

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/approval"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/memory"
	promptmemory "nekobot/pkg/memory/prompt"
	"nekobot/pkg/modelroute"
	"nekobot/pkg/permissionrules"
	"nekobot/pkg/preprocess"
	"nekobot/pkg/process"
	"nekobot/pkg/prompts"
	"nekobot/pkg/providers"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/state"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/subagent"
	"nekobot/pkg/tasks"
	"nekobot/pkg/tools"
	"nekobot/pkg/toolsessions"
)

const (
	orchestratorLegacy = "legacy"
	orchestratorBlades = "blades"
)

// SessionInterface defines the interface for a conversation session.
type SessionInterface interface {
	GetMessages() []Message
	AddMessage(Message)
}

type safeHistorySession interface {
	GetHistorySafe(int) []Message
}

// Agent represents an AI agent that can interact with users and use tools.
type Agent struct {
	config   *config.Config
	logger   *logger.Logger
	client   *providers.Client
	tools    *tools.Registry
	context  *ContextBuilder
	approval *approval.Manager

	permissionRules *permissionrules.Manager
	definition      AgentDefinition

	skillsManager  *skills.Manager
	semanticMemory memory.SearchManager
	promptManager  *prompts.Manager
	snapshotMgr    *session.SnapshotManager

	acpMu       sync.RWMutex
	acpSessions map[string]*acpSessionState
	acpRuntime  map[string]string
	kvStore     state.KV

	failoverMu       sync.Mutex
	failoverCooldown *providers.CooldownTracker
	providerGroups   *providerGroupPlanner

	maxIterations int
	entClient     *ent.Client
	taskStore     *tasks.Store
	taskService   *tasks.Service
	subagents     *subagent.SubagentManager
}

type subagentAgentAdapter struct {
	agent *Agent
}

// ChatRouteResult describes the routing request and the actual provider/model used.
type ChatRouteResult struct {
	RequestedProvider     string
	RequestedModel        string
	RequestedFallback     []string
	ResolvedOrder         []string
	ActualProvider        string
	ActualModel           string
	Preflight             ContextPreflightDecision
	ContextBudgetStatus   string
	ContextBudgetReasons  []string
	CompactionRecommended bool
	CompactionStrategy    string
}

func markPreflightApplied(routeResult ChatRouteResult) ChatRouteResult {
	if strings.TrimSpace(routeResult.Preflight.Action) == "" {
		return routeResult
	}
	routeResult.Preflight.Applied = true
	return routeResult
}

// acpSessionState stores ACP session-scoped routing and cancellation state.
type acpSessionState struct {
	session    SessionInterface
	provider   string
	model      string
	fallback   []string
	modeID     string
	cancel     context.CancelFunc
	mcpServers []config.MCPServerConfig
}

// Config holds agent configuration.
type Config struct {
	Workspace     string
	Restrict      bool // Restrict file access to workspace
	Model         string
	MaxTokens     int
	Temperature   float64
	MaxIterations int
}

// PromptContext describes managed prompt resolution input for one chat turn.
type PromptContext struct {
	Channel           string
	SessionID         string
	UserID            string
	Username          string
	RequestedProvider string
	RequestedModel    string
	RequestedFallback []string
	ExplicitPromptIDs []string
	Custom            map[string]any
}

// New creates a new agent with the given configuration.
func New(
	cfg *config.Config,
	log *logger.Logger,
	providerClient *providers.Client,
	processMgr *process.Manager,
	approvalMgr *approval.Manager,
	toolSessionMgr *toolsessions.Manager,
	kvStore state.KV,
	runtimeEntClient *ent.Client,
	promptMgr *prompts.Manager,
) (*Agent, error) {
	workspace := cfg.WorkspacePath()

	// Create tool registry
	toolRegistry := tools.NewRegistry()

	// Register built-in tools
	toolRegistry.MustRegister(tools.NewReadFileTool(workspace, cfg.Agents.Defaults.RestrictToWorkspace))
	toolRegistry.MustRegister(tools.NewWriteFileTool(workspace, cfg.Agents.Defaults.RestrictToWorkspace))
	toolRegistry.MustRegister(tools.NewEditFileTool(workspace, cfg.Agents.Defaults.RestrictToWorkspace))
	toolRegistry.MustRegister(tools.NewAppendFileTool(workspace, cfg.Agents.Defaults.RestrictToWorkspace))
	toolRegistry.MustRegister(tools.NewListDirTool(workspace, cfg.Agents.Defaults.RestrictToWorkspace))
	toolRegistry.MustRegister(tools.NewExecTool(workspace, cfg.Agents.Defaults.RestrictToWorkspace, tools.ExecConfig{
		Timeout: time.Duration(cfg.Tools.Exec.TimeoutSeconds) * time.Second,
		Sandbox: tools.DockerSandboxConfig{
			Enabled:     cfg.Tools.Exec.Sandbox.Enabled,
			Image:       cfg.Tools.Exec.Sandbox.Image,
			NetworkMode: cfg.Tools.Exec.Sandbox.NetworkMode,
			Mounts:      cfg.Tools.Exec.Sandbox.Mounts,
			Timeout:     time.Duration(cfg.Tools.Exec.Sandbox.Timeout) * time.Second,
			AutoCleanup: cfg.Tools.Exec.Sandbox.AutoCleanup,
		},
	}, processMgr))

	// Register process tool
	toolRegistry.MustRegister(tools.NewProcessTool(processMgr))
	log.Info("PTY process management enabled")

	// Register tool session tool (if tool session manager is available)
	if toolSessionMgr != nil {
		toolRegistry.MustRegister(tools.NewToolSessionTool(processMgr, toolSessionMgr, cfg))
		log.Info("Tool session tool enabled")
	}

	// Register web search tool (Brave first, optional DuckDuckGo fallback)
	if webSearch := tools.NewWebSearchTool(tools.WebSearchToolOptions{
		BraveAPIKey:          cfg.Tools.Web.Search.GetBraveAPIKey(),
		BraveMaxResults:      cfg.Tools.Web.Search.MaxResults,
		DuckDuckGoEnabled:    cfg.Tools.Web.Search.DuckDuckGoEnabled,
		DuckDuckGoMaxResults: cfg.Tools.Web.Search.DuckDuckGoMaxResults,
	}); webSearch != nil {
		toolRegistry.MustRegister(webSearch)
		log.Info("Web search tool enabled", zap.String("providers", webSearch.ProviderSummary()))
	}

	// Web fetch tool (always available)
	toolRegistry.MustRegister(tools.NewWebFetchTool(50000))

	// Browser tool (if Chrome is available)
	outputDir := cfg.WorkspacePath() + "/screenshots"
	toolRegistry.MustRegister(tools.NewBrowserTool(log, true, 30, outputDir))
	log.Info("Browser tool enabled")

	// Message tool (will be configured later by gateway)
	toolRegistry.MustRegister(tools.NewMessageTool(nil))
	toolRegistry.MustRegister(tools.NewWikiQueryTool(workspace))
	toolRegistry.MustRegister(tools.NewWikiLintTool(workspace))

	var semanticMemory memory.SearchManager
	if cfg.Memory.Enabled && cfg.Memory.Semantic.Enabled {
		memoryMgr, err := newSemanticMemoryManagerFromConfig(log, cfg)
		if err != nil {
			log.Warn("Failed to initialize semantic memory tool", zap.Error(err))
		} else {
			semanticMemory = memoryMgr
			toolRegistry.MustRegister(tools.NewMemoryTool(log, memoryMgr, tools.MemoryToolOptions{
				DefaultTopK:   cfg.Memory.Semantic.DefaultTopK,
				MaxTopK:       cfg.Memory.Semantic.MaxTopK,
				SearchPolicy:  cfg.Memory.Semantic.SearchPolicy,
				IncludeScores: cfg.Memory.Semantic.IncludeScores,
			}))
			log.Info("Memory tool enabled",
				zap.String("search_policy", cfg.Memory.Semantic.SearchPolicy),
				zap.Int("default_top_k", cfg.Memory.Semantic.DefaultTopK),
			)
		}
	}

	if cfg.Learnings.Enabled {
		learningsMgr, err := memory.NewLearningsManager(cfg)
		if err != nil {
			log.Warn("Failed to initialize learning tool", zap.Error(err))
		} else {
			toolRegistry.MustRegister(tools.NewLearningTool(learningsMgr))
			log.Info("Learning tool enabled")
		}
	}

	// Initialize snapshot manager for turn undo functionality
	var snapshotMgr *session.SnapshotManager
	if cfg.Undo.Enabled {
		snapshotDir := filepath.Join(workspace, ".nekobot", "sessions")
		snapshotMgr = session.NewSnapshotManager(snapshotDir, cfg.Undo)
		log.Info("Turn undo system enabled", zap.Int("max_turns", cfg.Undo.MaxTurns))
	}

	// Create context builder.
	memoryStore := newMemoryStoreFromConfig(cfg, workspace, kvStore, runtimeEntClient)
	contextBuilder := NewContextBuilderWithMemory(workspace, memoryStore)
	contextBuilder.SetMemoryContextOptions(promptmemory.ContextOptions{
		IncludeWorkspaceMemory: cfg.Memory.Context.Enabled && cfg.Memory.Context.IncludeWorkspaceMemory,
		IncludeLongTerm:        cfg.Memory.Context.Enabled && cfg.Memory.Context.IncludeLongTerm,
		IncludeActiveLearnings: cfg.Learnings.Enabled,
		RecentDailyNoteDays:    cfg.Memory.Context.RecentDailyNoteDays,
		MaxChars:               cfg.Memory.Context.MaxChars,
	})
	contextBuilder.SetPreprocessorConfig(preprocessConfigFromConfig(cfg, workspace))

	// Set tool descriptions function
	contextBuilder.SetToolDescriptionsFunc(toolRegistry.GetDescriptions)

	agent := &Agent{
		config:           cfg,
		logger:           log,
		client:           providerClient,
		tools:            toolRegistry,
		context:          contextBuilder,
		approval:         approvalMgr,
		definition:       AgentDefinitionFromRuntimeConfig(cfg),
		semanticMemory:   semanticMemory,
		promptManager:    promptMgr,
		snapshotMgr:      snapshotMgr,
		acpSessions:      make(map[string]*acpSessionState),
		acpRuntime:       make(map[string]string),
		kvStore:          kvStore,
		failoverCooldown: providers.NewCooldownTracker(),
		providerGroups:   newProviderGroupPlanner(),
		maxIterations:    cfg.Agents.Defaults.MaxToolIterations,
		entClient:        runtimeEntClient,
		taskStore:        tasks.NewStore(),
	}
	agent.taskService = tasks.NewService(agent.taskStore)
	if processMgr != nil {
		processMgr.SetTaskService(agent.taskService)
	}

	// Set orchestrator mode on context builder so skills section adapts.
	if mode, err := agent.resolveOrchestrator(); err == nil {
		contextBuilder.SetOrchestratorMode(mode)
	}

	return agent, nil
}

func preprocessConfigFromConfig(cfg *config.Config, workspace string) preprocess.PreprocessorConfig {
	preprocessCfg := preprocess.DefaultConfig()
	preprocessCfg.Workspace = workspace
	if cfg == nil {
		return preprocessCfg
	}

	fileMentions := cfg.Preprocess.FileMentions
	preprocessCfg.Enabled = fileMentions.Enabled
	preprocessCfg.MaxFileSize = fileMentions.MaxFileSize
	preprocessCfg.MaxTotalSize = fileMentions.MaxTotalSize
	preprocessCfg.MaxFiles = fileMentions.MaxFiles
	return preprocessCfg
}

// RegisterSkillTool registers the skill tool with the agent.
// This should be called after agent creation when skills manager is available.
func (a *Agent) RegisterSkillTool(skillsManager *skills.Manager) {
	a.skillsManager = skillsManager
	a.tools.MustRegister(tools.NewSkillTool(a.logger, skillsManager))
	a.logger.Info("Skill tool registered")
}

// RegisterUndoTool registers the undo tool with the agent.
// This should be called after agent creation when snapshot manager is available.
func (a *Agent) RegisterUndoTool(sessionID string) {
	if a.snapshotMgr == nil {
		a.logger.Debug("Undo tool not registered - snapshot manager not initialized")
		return
	}
	a.tools.Replace(tools.NewUndoTool(tools.UndoToolOptions{
		SnapshotMgr: a.snapshotMgr,
		SessionID:   sessionID,
	}))
	a.logger.Info("Undo tool registered", zap.String("session_id", sessionID))
}

// SetApprovalModeForSession overrides approval mode for one chat session.
func (a *Agent) SetApprovalModeForSession(sessionID string, mode approval.Mode) error {
	if a == nil {
		return fmt.Errorf("agent is nil")
	}
	if a.approval == nil {
		return fmt.Errorf("approval manager is unavailable")
	}
	switch mode {
	case approval.ModeAuto, approval.ModePrompt, approval.ModeManual:
	default:
		return fmt.Errorf("unsupported approval mode: %s", mode)
	}
	trimmedID := strings.TrimSpace(sessionID)
	a.approval.SetSessionMode(trimmedID, mode)
	if a.taskStore != nil {
		a.taskStore.SetSessionPermissionMode(trimmedID, string(mode))
	}
	return nil
}

// ClearApprovalModeForSession removes the approval override for one chat session.
func (a *Agent) ClearApprovalModeForSession(sessionID string) error {
	if a == nil {
		return fmt.Errorf("agent is nil")
	}
	if a.approval == nil {
		return fmt.Errorf("approval manager is unavailable")
	}
	trimmedID := strings.TrimSpace(sessionID)
	a.approval.ClearSessionMode(trimmedID)
	if a.taskStore != nil {
		a.taskStore.ClearSessionPermissionMode(trimmedID)
	}
	return nil
}

// SnapshotManager exposes the undo snapshot manager for higher-level workflows.
func (a *Agent) SnapshotManager() *session.SnapshotManager {
	if a == nil {
		return nil
	}
	return a.snapshotMgr
}

// GetTaskSnapshots exposes shared task runtime snapshots for higher-level control planes.
func (a *Agent) GetTaskSnapshots() []tasks.Task {
	if a == nil || a.taskStore == nil {
		return nil
	}
	return a.taskStore.List()
}

// TaskStore exposes the shared runtime task store.
func (a *Agent) TaskStore() *tasks.Store {
	if a == nil {
		return nil
	}
	return a.taskStore
}

// ApprovalManager exposes the shared approval manager for higher-level control planes.
func (a *Agent) ApprovalManager() *approval.Manager {
	if a == nil {
		return nil
	}
	return a.approval
}

// EntClient exposes the shared runtime ent client for higher-level control planes.
func (a *Agent) EntClient() *ent.Client {
	if a == nil {
		return nil
	}
	return a.entClient
}

// TaskService exposes the shared runtime task lifecycle service.
func (a *Agent) TaskService() *tasks.Service {
	if a == nil {
		return nil
	}
	return a.taskService
}

// Definition exposes the current AgentDefinition compatibility snapshot.
func (a *Agent) Definition() AgentDefinition {
	if a == nil {
		return AgentDefinition{}
	}
	return a.definition
}

// PreviewPreprocessedInput exposes the configured file-mention preprocessing
// result so UI layers can show lightweight feedback.
func (a *Agent) PreviewPreprocessedInput(input string) (*preprocess.Result, error) {
	if a == nil || a.context == nil {
		return &preprocess.Result{
			OriginalInput:  input,
			ProcessedInput: input,
		}, nil
	}
	return a.context.PreviewPreprocessedInput(input)
}

// EnableSubagents registers the spawn tool and optional completion notifications.
func (a *Agent) EnableSubagents(notify subagent.NotifyFunc) {
	if a == nil {
		return
	}
	if a.subagents != nil {
		if notify != nil {
			a.subagents.SetNotifyFunc(notify)
		}
		return
	}

	manager := subagent.NewSubagentManager(a.logger, &subagentAgentAdapter{agent: a}, 10)
	if notify != nil {
		manager.SetNotifyFunc(notify)
	}
	if a.taskService != nil {
		manager.SetTaskService(a.taskService)
	}

	a.subagents = manager
	if _, exists := a.tools.Get("spawn"); !exists {
		a.tools.MustRegister(tools.NewSpawnTool(a.logger, manager))
		a.logger.Info("Spawn tool registered")
	}
}

// DisableSubagents stops subagent workers. Primarily used by tests.
func (a *Agent) DisableSubagents() {
	if a == nil || a.subagents == nil {
		return
	}

	a.subagents.Stop()
	a.subagents = nil
}

// Chat processes a user message and returns the agent's response.
// It handles tool calls and iterates until the agent produces a final response.
func (a *Agent) Chat(ctx context.Context, sess SessionInterface, userMessage string) (string, error) {
	return a.chatWithProviderModelAndPromptContext(ctx, sess, userMessage, "", a.config.Agents.Defaults.Model, nil, PromptContext{})
}

// ChatWithModel processes a message using a specific model override.
func (a *Agent) ChatWithModel(ctx context.Context, sess SessionInterface, userMessage, model string) (string, error) {
	return a.chatWithProviderModelAndPromptContext(ctx, sess, userMessage, "", model, nil, PromptContext{})
}

// ChatWithProviderModel processes a message using provider/model overrides.
func (a *Agent) ChatWithProviderModel(ctx context.Context, sess SessionInterface, userMessage, provider, model string) (string, error) {
	return a.chatWithProviderModelAndPromptContext(ctx, sess, userMessage, provider, model, nil, PromptContext{})
}

// ChatWithProviderModelAndFallback processes a message using provider/model/fallback overrides.
func (a *Agent) ChatWithProviderModelAndFallback(ctx context.Context, sess SessionInterface, userMessage, provider, model string, fallback []string) (string, error) {
	return a.chatWithProviderModelAndPromptContext(ctx, sess, userMessage, provider, model, fallback, PromptContext{})
}

// ChatWithPromptContext applies managed prompt overlays for this request.
func (a *Agent) ChatWithPromptContext(
	ctx context.Context,
	sess SessionInterface,
	userMessage string,
	promptCtx PromptContext,
) (string, error) {
	response, _, err := a.ChatWithPromptContextDetailed(ctx, sess, userMessage, promptCtx)
	return response, err
}

// ChatWithPromptContextDetailed applies managed prompt overlays and returns routing diagnostics.
func (a *Agent) ChatWithPromptContextDetailed(
	ctx context.Context,
	sess SessionInterface,
	userMessage string,
	promptCtx PromptContext,
) (string, ChatRouteResult, error) {
	return a.chatWithProviderModelDetailed(
		ctx,
		sess,
		userMessage,
		promptCtx.RequestedProvider,
		promptCtx.RequestedModel,
		promptCtx.RequestedFallback,
		promptCtx,
	)
}

// ReplayApprovedToolCall replays a previously blocked ordinary tool call after approval.
func (a *Agent) ReplayApprovedToolCall(
	ctx context.Context,
	sessionID string,
	call providers.UnifiedToolCall,
) (string, error) {
	if a == nil {
		return "", fmt.Errorf("agent is nil")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID != "" {
		ctx = context.WithValue(ctx, promptContextSessionKey, sessionID)
	}
	return a.executeToolCall(ctx, call)
}

// ChatWithProviderModelAndFallbackDetailed returns the response plus routing diagnostics.
func (a *Agent) ChatWithProviderModelAndFallbackDetailed(
	ctx context.Context,
	sess SessionInterface,
	userMessage, provider, model string,
	fallback []string,
) (string, ChatRouteResult, error) {
	return a.chatWithProviderModelDetailed(ctx, sess, userMessage, provider, model, fallback, PromptContext{})
}

func (a *Agent) chatWithProviderModelAndPromptContext(
	ctx context.Context,
	sess SessionInterface,
	userMessage, provider, model string,
	fallback []string,
	promptCtx PromptContext,
) (string, error) {
	response, _, err := a.chatWithProviderModelDetailed(ctx, sess, userMessage, provider, model, fallback, promptCtx)
	return response, err
}

func (a *Agent) chatWithProviderModelDetailed(
	ctx context.Context,
	sess SessionInterface,
	userMessage, provider, model string,
	fallback []string,
	promptCtx PromptContext,
) (string, ChatRouteResult, error) {
	ctx = context.WithValue(ctx, promptContextChannelKey, strings.TrimSpace(promptCtx.Channel))
	ctx = context.WithValue(ctx, promptContextSessionKey, strings.TrimSpace(promptCtx.SessionID))
	if promptCtx.Custom != nil {
		if runtimeID, ok := promptCtx.Custom["runtime_id"].(string); ok {
			ctx = context.WithValue(ctx, promptContextRuntimeKey, strings.TrimSpace(runtimeID))
		}
	}

	sessionID := strings.TrimSpace(promptCtx.SessionID)
	if sessionID == "" {
		if identifiable, ok := sess.(interface{ GetID() string }); ok {
			sessionID = strings.TrimSpace(identifiable.GetID())
		}
	}
	if sessionID != "" {
		a.RegisterUndoTool(sessionID)
	}

	// Save snapshot before each turn (for undo functionality)
	if a.snapshotMgr != nil && sess != nil {
		store := a.snapshotMgr.GetStore(sessionID)
		if store != nil {
			messages := sess.GetMessages()
			snapshotMessages := convertToSnapshotMessages(messages)
			summary := ""
			if summarizable, ok := sess.(interface{ GetSummary() string }); ok {
				summary = summarizable.GetSummary()
			}
			if err := store.SaveSnapshot(snapshotMessages, summary); err != nil {
				a.logger.Warn("Failed to save snapshot", zap.Error(err))
			}
		}
	}

	orchestrator, err := a.resolveOrchestrator()
	if err != nil {
		return "", ChatRouteResult{}, err
	}

	a.logger.Debug("Dispatching chat orchestration",
		zap.String("orchestrator", orchestrator),
	)

	switch orchestrator {
	case orchestratorBlades:
		return a.chatWithBladesOrchestrator(ctx, sess, userMessage, provider, model, fallback, promptCtx)
	case orchestratorLegacy:
		return a.chatWithLegacyOrchestrator(ctx, sess, userMessage, provider, model, fallback, promptCtx)
	default:
		return "", ChatRouteResult{}, fmt.Errorf("unsupported orchestrator: %s", orchestrator)
	}
}

// convertToSnapshotMessages converts agent.Message slice to session.MessageSnapshot slice.
func convertToSnapshotMessages(messages []Message) []session.MessageSnapshot {
	result := make([]session.MessageSnapshot, len(messages))
	for i, msg := range messages {
		result[i] = session.MessageSnapshot{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			result[i].ToolCalls = make([]session.ToolCallSnapshot, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				result[i].ToolCalls[j] = session.ToolCallSnapshot{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				}
			}
		}
	}
	return result
}

func (a *Agent) sessionHistory(sess SessionInterface) []Message {
	if sess == nil {
		return nil
	}

	limit := 0
	if a != nil && a.config != nil && a.config.Memory.ShortTerm.Enabled {
		limit = a.config.Memory.ShortTerm.RawHistoryLimit
	}

	if limit > 0 {
		if historySession, ok := sess.(safeHistorySession); ok {
			return historySession.GetHistorySafe(limit)
		}
		history := sess.GetMessages()
		if len(history) > limit {
			return history[len(history)-limit:]
		}
		return history
	}

	return sess.GetMessages()
}

func newMemoryStoreFromConfig(cfg *config.Config, workspace string, kvStore state.KV, runtimeEntClient *ent.Client) *promptmemory.Store {
	if cfg == nil || !cfg.Memory.Enabled {
		return promptmemory.NewStoreWithBackend(workspace, promptmemory.NewNoopBackend())
	}

	backendKind := strings.TrimSpace(strings.ToLower(cfg.Memory.Backend))
	if backendKind == "" {
		backendKind = "file"
	}

	switch backendKind {
	case "db":
		if runtimeEntClient != nil {
			backend, err := promptmemory.NewDBBackend(runtimeEntClient, cfg.Memory.DBPrefix)
			if err == nil {
				return promptmemory.NewStoreWithBackend(workspace, backend)
			}
		}
	case "kv":
		if kvStore != nil {
			backend, err := promptmemory.NewKVBackend(kvStore, cfg.Memory.KVPrefix)
			if err == nil {
				return promptmemory.NewStoreWithBackend(workspace, backend)
			}
		}
	}

	memoryPath := strings.TrimSpace(cfg.Memory.FilePath)
	if memoryPath == "" {
		memoryPath = filepath.Join(workspace, "memory")
	}
	backend, err := promptmemory.NewFileBackend(memoryPath)
	if err != nil {
		return promptmemory.NewStoreWithBackend(workspace, promptmemory.NewNoopBackend())
	}
	return promptmemory.NewStoreWithBackend(workspace, backend)
}

func newSemanticMemoryManagerFromConfig(log *logger.Logger, cfg *config.Config) (memory.SearchManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	return memory.NewSearchManagerFromConfig(log, cfg)
}

func (a *Agent) resolveOrchestrator() (string, error) {
	orchestrator := strings.TrimSpace(strings.ToLower(a.config.Agents.Defaults.Orchestrator))
	if orchestrator == "" {
		return orchestratorBlades, nil
	}

	switch orchestrator {
	case orchestratorLegacy, orchestratorBlades:
		return orchestrator, nil
	default:
		return "", fmt.Errorf("unsupported orchestrator: %s", orchestrator)
	}
}

func (a *Agent) chatWithLegacyOrchestrator(
	ctx context.Context,
	sess SessionInterface,
	userMessage, provider, model string,
	fallback []string,
	promptCtx PromptContext,
) (string, ChatRouteResult, error) {
	a.logger.Info("Processing chat message",
		zap.String("message", truncate(userMessage, 100)),
	)
	routeResult := ChatRouteResult{
		RequestedProvider: strings.TrimSpace(provider),
		RequestedModel:    strings.TrimSpace(model),
		RequestedFallback: append([]string(nil), fallback...),
	}
	if model == "" {
		model = a.config.Agents.Defaults.Model
	}

	providerOrder, err := a.buildProviderOrder(provider, fallback)
	if err != nil {
		return "", routeResult, err
	}
	routeResult.ResolvedOrder = append([]string(nil), providerOrder...)
	primaryProvider := providerOrder[0]
	clientCache := make(map[string]*providers.Client)

	// Build initial messages with session history
	history := a.sessionHistory(sess)
	resolvedPrompts, err := a.resolvePromptSet(ctx, provider, model, fallback, promptCtx)
	if err != nil {
		return "", routeResult, err
	}
	routeResult = a.enrichChatRouteResultWithContextPreview(routeResult, resolvedPrompts, promptCtx, userMessage)
	messages := a.context.BuildMessagesWithPromptSet(history, userMessage, resolvedPrompts)

	// Convert to provider format
	providerMessages := a.convertToProviderMessages(messages)
	if routeResult.Preflight.Action == "compact_before_run" {
		compressedMessages := forceCompressMessages(providerMessages)
		if len(compressedMessages) != len(providerMessages) {
			a.logger.Info("Applying preflight message compression before first legacy model call",
				zap.Int("messages_before", len(providerMessages)),
				zap.Int("messages_after", len(compressedMessages)),
				zap.Int("estimated_tokens", estimateTokens(compressedMessages)),
			)
		}
		providerMessages = compressedMessages
		routeResult = markPreflightApplied(routeResult)
	}

	// Tool definitions
	toolDefs := a.convertToolDefinitions()

	// Main agent loop
	iteration := 0
	for iteration < a.maxIterations {
		iteration++

		a.logger.Debug("Agent iteration",
			zap.Int("iteration", iteration),
			zap.Int("max", a.maxIterations),
		)

		// Create request
		req := &providers.UnifiedRequest{
			Model:       model,
			Messages:    providerMessages,
			Tools:       toolDefs,
			MaxTokens:   a.config.Agents.Defaults.MaxTokens,
			Temperature: a.config.Agents.Defaults.Temperature,
		}

		// Pass extended thinking config via Extra
		if a.config.Agents.Defaults.ExtendedThinking {
			req.Extra = map[string]interface{}{
				"extended_thinking": true,
				"thinking_budget":   a.config.Agents.Defaults.ThinkingBudget,
			}
		}

		// Call LLM with provider fallback, with retry on context errors.
		var resp *providers.UnifiedResponse
		var providerUsed, modelUsed string
		const maxContextRetries = 2
		for retry := 0; retry <= maxContextRetries; retry++ {
			resp, providerUsed, modelUsed, err = a.callLLMWithFallback(ctx, req, primaryProvider, providerOrder, model, clientCache)
			if err == nil {
				break
			}

			if isContextLimitError(err) && retry < maxContextRetries {
				a.logger.Warn("Context window error, compressing and retrying",
					zap.Error(err),
					zap.Int("retry", retry+1),
					zap.Int("messages_before", len(providerMessages)),
				)
				providerMessages = forceCompressMessages(providerMessages)
				req.Messages = providerMessages
				a.logger.Info("Compressed messages",
					zap.Int("messages_after", len(providerMessages)),
					zap.Int("estimated_tokens", estimateTokens(providerMessages)),
				)
				continue
			}
			break
		}
		if err != nil {
			if routeResult.ActualProvider == "" {
				routeResult.ActualProvider = providerUsed
			}
			if routeResult.ActualModel == "" {
				routeResult.ActualModel = modelUsed
			}
			return "", routeResult, fmt.Errorf("LLM call failed: %w", err)
		}
		if routeResult.ActualProvider == "" {
			routeResult.ActualProvider = providerUsed
		}
		if routeResult.ActualModel == "" {
			routeResult.ActualModel = modelUsed
		}

		a.logger.Debug("LLM response",
			zap.String("provider", providerUsed),
			zap.String("model", modelUsed),
			zap.String("content", truncate(resp.Content, 100)),
			zap.Int("tool_calls", len(resp.ToolCalls)),
			zap.String("finish_reason", resp.FinishReason),
		)

		// Log thinking content if present
		if resp.Thinking != "" {
			a.logger.Debug("LLM thinking",
				zap.String("thinking", truncate(resp.Thinking, 200)),
			)
		}

		// Add assistant message to history
		assistantMsg := providers.UnifiedMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		providerMessages = append(providerMessages, assistantMsg)

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 {
			return resp.Content, routeResult, nil
		}

		// Execute tool calls
		trackedSessionID := strings.TrimSpace(promptCtx.SessionID)
		if a.taskStore != nil && trackedSessionID != "" {
			a.taskStore.EnsureSessionToolRoundLimit(trackedSessionID, a.maxIterations)
			if !a.taskStore.CanStartSessionToolRound(trackedSessionID) {
				state, _ := a.taskStore.GetSessionState(trackedSessionID)
				return "", routeResult, fmt.Errorf("max tool rounds (%d) reached for session %s", state.MaxToolRounds, trackedSessionID)
			}
			a.taskStore.RecordSessionToolRound(trackedSessionID)
		}
		for _, toolCall := range resp.ToolCalls {
			result, err := a.executeToolCall(ctx, toolCall)
			if err != nil {
				a.logger.Error("Tool execution failed",
					zap.String("tool", toolCall.Name),
					zap.Error(err),
				)
				result = fmt.Sprintf("Error: %v", err)
			}

			// Add tool result to messages
			providerMessages = append(providerMessages, providers.UnifiedMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			})

			a.logger.Debug("Tool executed",
				zap.String("tool", toolCall.Name),
				zap.String("result", truncate(result, 100)),
			)
		}
	}

	return "", routeResult, fmt.Errorf("max iterations (%d) reached without final response", a.maxIterations)
}

func (a *Agent) enrichChatRouteResultWithContextPreview(
	routeResult ChatRouteResult,
	resolved prompts.ResolvedPromptSet,
	promptCtx PromptContext,
	userMessage string,
) ChatRouteResult {
	if a == nil || a.context == nil {
		return routeResult
	}

	preview := a.buildContextSourcesPreviewFromResolved(resolved, promptCtx, userMessage)
	routeResult.Preflight = preview.Preflight
	routeResult.ContextBudgetStatus = preview.BudgetStatus
	routeResult.ContextBudgetReasons = append([]string(nil), preview.BudgetReasons...)
	routeResult.CompactionRecommended = preview.Compaction.Recommended
	routeResult.CompactionStrategy = preview.Compaction.Strategy
	return routeResult
}

func (a *Agent) resolvePromptSet(
	ctx context.Context,
	provider, model string,
	fallback []string,
	promptCtx PromptContext,
) (prompts.ResolvedPromptSet, error) {
	if a == nil || a.promptManager == nil {
		return prompts.ResolvedPromptSet{}, nil
	}

	input := a.buildPromptResolveInput(provider, model, fallback, promptCtx)

	resolved, err := a.promptManager.Resolve(ctx, input)
	if err != nil {
		return prompts.ResolvedPromptSet{}, fmt.Errorf("resolve prompts: %w", err)
	}
	if resolved == nil {
		return prompts.ResolvedPromptSet{}, nil
	}
	return *resolved, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizePromptFallback(primary, secondary []string) []string {
	source := primary
	if len(source) == 0 {
		source = secondary
	}
	if len(source) == 0 {
		return nil
	}
	out := make([]string, 0, len(source))
	for _, item := range source {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func clonePromptCustom(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func (a *Agent) newClientForProvider(providerName, model string) (*providers.Client, error) {
	providerCfg := a.config.GetProviderConfig(providerName)
	if providerCfg == nil {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}

	providerKind := strings.TrimSpace(providerCfg.ProviderKind)
	if providerKind == "" {
		providerKind = providerName
	}

	client, err := providers.NewClient(providerKind, &providers.RelayInfo{
		ProviderName: providerName,
		APIKey:       providerCfg.APIKey,
		APIBase:      providerCfg.APIBase,
		Model:        model,
		Proxy:        providerCfg.Proxy,
		Timeout:      providerCfg.GetTimeout(),
	})
	if err != nil {
		return nil, fmt.Errorf("create provider client for %s: %w", providerName, err)
	}

	return client, nil
}

func (a *Agent) buildProviderOrder(provider string, fallback []string) ([]string, error) {
	if a.providerGroups == nil {
		a.providerGroups = newProviderGroupPlanner()
	}
	if a.logger == nil {
		logCfg := logger.DefaultConfig()
		logCfg.OutputPath = ""
		logCfg.EnableCaller = false
		logCfg.EnableStacktrace = false
		if fallbackLogger, err := logger.New(logCfg); err == nil {
			a.logger = fallbackLogger
		}
	}

	primary := strings.TrimSpace(provider)
	if primary == "" {
		primary = strings.TrimSpace(a.config.Agents.Defaults.Provider)
	}

	fallbackOrder := fallback
	if len(fallbackOrder) == 0 {
		fallbackOrder = a.config.Agents.Defaults.Fallback
	}

	order, err := a.providerGroups.expand(a.config, a.logger, primary, fallbackOrder)
	if err != nil {
		return nil, err
	}

	filteredOrder := make([]string, 0, len(order))
	for _, name := range order {
		profile := a.config.GetProviderConfig(name)
		if !providerConfigUsable(profile) {
			a.logger.Warn("Skipping unusable provider from routing order",
				zap.String("provider", strings.TrimSpace(name)),
			)
			continue
		}
		filteredOrder = append(filteredOrder, name)
	}
	order = filteredOrder

	if len(order) == 0 && len(a.config.Providers) > 0 {
		for _, provider := range a.config.Providers {
			profile := a.config.GetProviderConfig(provider.Name)
			if !providerConfigUsable(profile) {
				continue
			}
			order = append(order, provider.Name)
			break
		}
	}

	if len(order) == 0 {
		return nil, fmt.Errorf("no providers configured")
	}

	return order, nil
}

func (a *Agent) callLLMWithFallback(
	ctx context.Context,
	req *providers.UnifiedRequest,
	primaryProvider string,
	providerOrder []string,
	requestedModel string,
	clientCache map[string]*providers.Client,
) (*providers.UnifiedResponse, string, string, error) {
	tracker := a.getFailoverCooldown()
	var lastErr error
	var lastProviderUsed string
	var lastModelUsed string
	var attempts []providers.FallbackAttempt

	for _, providerName := range providerOrder {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, "", "", ctxErr
		}

		if !tracker.IsAvailable(providerName) {
			remaining := tracker.CooldownRemaining(providerName)
			lastErr = fmt.Errorf("provider %s in cooldown (%s remaining)", providerName, remaining.Round(time.Second))
			attempts = append(attempts, providers.FallbackAttempt{
				Provider: providerName,
				Skipped:  true,
				Reason:   providers.FailoverReasonRateLimit,
				Error:    lastErr,
			})
			a.logger.Warn("Provider skipped due to cooldown",
				zap.String("provider", providerName),
				zap.Duration("remaining", remaining),
			)
			continue
		}

		model, err := a.resolveModelForProvider(ctx, providerName, primaryProvider, requestedModel)
		if err != nil {
			lastErr = err
			a.logger.Warn("Provider route resolution failed",
				zap.String("provider", providerName),
				zap.String("requested_model", requestedModel),
				zap.Error(err),
			)
			continue
		}
		lastProviderUsed = providerName
		lastModelUsed = model

		client, err := a.getProviderClient(providerName, model, clientCache)
		if err != nil {
			lastErr = err
			tracker.MarkFailure(providerName, providers.FailoverReasonUnknown)
			if a.providerGroups != nil {
				a.providerGroups.recordFailure(providerName, err)
			}
			a.logger.Warn("Provider unavailable", zap.String("provider", providerName), zap.Error(err))
			continue
		}

		reqCopy := *req
		reqCopy.Model = model

		resp, err := client.Chat(ctx, &reqCopy)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, "", "", ctxErr
			}

			failoverErr := providers.ClassifyError(err, providerName, model)
			reason := providers.FailoverReasonUnknown
			retriable := true
			loggedErr := err
			if failoverErr != nil {
				reason = failoverErr.Reason
				retriable = failoverErr.IsRetriable()
				loggedErr = failoverErr
			}
			lastErr = loggedErr
			attempts = append(attempts, providers.FallbackAttempt{
				Provider: providerName,
				Model:    model,
				Error:    loggedErr,
				Reason:   reason,
			})

			a.logger.Warn("Provider request failed",
				zap.String("provider", providerName),
				zap.String("model", model),
				zap.String("reason", string(reason)),
				zap.Bool("retriable", retriable),
				zap.Error(loggedErr),
			)

			if !retriable {
				return nil, lastProviderUsed, lastModelUsed, loggedErr
			}

			tracker.MarkFailure(providerName, reason)
			if a.providerGroups != nil {
				a.providerGroups.recordFailure(providerName, loggedErr)
			}
			continue
		}

		tracker.MarkSuccess(providerName)
		if a.providerGroups != nil {
			a.providerGroups.recordSuccess(providerName)
		}
		return resp, providerName, model, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no provider attempt made")
	}
	if len(attempts) > 0 {
		lastErr = &providers.FallbackExhaustedError{Attempts: attempts}
	}
	return nil, lastProviderUsed, lastModelUsed, lastErr
}

func (a *Agent) getFailoverCooldown() *providers.CooldownTracker {
	a.failoverMu.Lock()
	defer a.failoverMu.Unlock()

	if a.failoverCooldown == nil {
		a.failoverCooldown = providers.NewCooldownTracker()
	}

	return a.failoverCooldown
}

// GetFailoverSnapshots returns current runtime cooldown snapshots keyed by provider name.
func (a *Agent) GetFailoverSnapshots(providerNames []string) map[string]providers.CooldownSnapshot {
	tracker := a.getFailoverCooldown()
	snapshots := make(map[string]providers.CooldownSnapshot, len(providerNames))
	for _, providerName := range providerNames {
		trimmed := strings.TrimSpace(providerName)
		if trimmed == "" {
			continue
		}
		snapshots[trimmed] = tracker.Snapshot(trimmed)
	}
	return snapshots
}

func (a *Agent) getProviderClient(providerName, model string, cache map[string]*providers.Client) (*providers.Client, error) {
	key := providerName + "::" + model
	if client, ok := cache[key]; ok {
		return client, nil
	}

	client, err := a.newClientForProvider(providerName, model)
	if err != nil {
		return nil, err
	}
	cache[key] = client
	return client, nil
}

func (a *Agent) resolveModelForProvider(
	ctx context.Context,
	providerName,
	primaryProvider,
	requestedModel string,
) (string, error) {
	model := strings.TrimSpace(requestedModel)
	if model == "" {
		model = strings.TrimSpace(a.config.Agents.Defaults.Model)
	}
	if model == "" {
		return "", fmt.Errorf("model is required")
	}
	if a != nil && a.entClient != nil {
		resolved, err := a.resolveModelFromRoutes(ctx, providerName, model)
		if err == nil {
			return resolved, nil
		}
		if !errors.Is(err, modelroute.ErrRouteNotFound) {
			return "", err
		}
	}
	if providerName == primaryProvider {
		return model, nil
	}

	providerCfg := a.config.GetProviderConfig(providerName)
	if providerCfg == nil {
		return model, nil
	}

	// If this provider declares no model list, keep caller's model.
	if len(providerCfg.Models) == 0 {
		return model, nil
	}

	for _, candidate := range providerCfg.Models {
		if strings.TrimSpace(candidate) == model {
			return model, nil
		}
	}

	if fallbackModel := strings.TrimSpace(providerCfg.GetDefaultModel()); fallbackModel != "" {
		return fallbackModel, nil
	}

	return model, nil
}

func (a *Agent) resolveModelFromRoutes(
	ctx context.Context,
	providerName string,
	requestedModel string,
) (string, error) {
	if a == nil || a.entClient == nil {
		return "", modelroute.ErrRouteNotFound
	}

	routeMgr, err := modelroute.NewManager(a.config, a.logger, a.entClient)
	if err != nil {
		return "", fmt.Errorf("create model route manager: %w", err)
	}

	logicalModelID := strings.TrimSpace(requestedModel)
	matchedRoute, err := routeMgr.ResolveInput(ctx, requestedModel)
	if err == nil {
		logicalModelID = matchedRoute.ModelID
	} else if !errors.Is(err, modelroute.ErrRouteNotFound) {
		return "", err
	}

	routes, err := routeMgr.ListByModel(ctx, logicalModelID)
	if err != nil {
		return "", err
	}

	for _, route := range routes {
		if !route.Enabled || strings.TrimSpace(route.ProviderName) != strings.TrimSpace(providerName) {
			continue
		}
		if providerModelID, ok := route.Metadata["provider_model_id"].(string); ok && strings.TrimSpace(providerModelID) != "" {
			return strings.TrimSpace(providerModelID), nil
		}
		return route.ModelID, nil
	}

	return "", fmt.Errorf("resolve route for provider %s model %s: %w", providerName, logicalModelID, modelroute.ErrRouteNotFound)
}

// executeToolCall executes a single tool call with approval checking.
func (a *Agent) executeToolCall(ctx context.Context, toolCall providers.UnifiedToolCall) (string, error) {
	a.logger.Info("Executing tool",
		zap.String("tool", toolCall.Name),
		zap.Any("args", toolCall.Arguments),
	)

	sessionID := ctxStringValue(ctx, promptContextSessionKey)
	runtimeID := ctxStringValue(ctx, promptContextRuntimeKey)
	skipApproval := false
	if a.taskStore != nil && sessionID != "" {
		if !a.taskStore.CanExecuteSessionToolCall(sessionID, toolCall.Name) {
			state, _ := a.taskStore.GetSessionState(sessionID)
			limit := state.PerToolLimits[strings.TrimSpace(toolCall.Name)]
			return "", fmt.Errorf("tool %s reached per-session call limit (%d) for session %s", toolCall.Name, limit, sessionID)
		}
		a.taskStore.RecordSessionToolCall(sessionID, toolCall.Name)
	}

	if a.permissionRules != nil {
		result, err := a.permissionRules.Evaluate(ctx, permissionrules.Input{
			ToolName:  toolCall.Name,
			SessionID: sessionID,
			RuntimeID: runtimeID,
		})
		if err != nil {
			return "", fmt.Errorf("permission rule evaluation failed: %w", err)
		}
		if result.Matched {
			switch result.Action {
			case permissionrules.ActionDeny:
				if a.taskStore != nil {
					a.taskStore.ClearSessionPendingAction(sessionID)
					a.taskStore.SetSessionLifecycleState(sessionID, tasks.SessionLifecycleIdle, "")
				}
				return "Tool call denied by permission rule", nil
			case permissionrules.ActionAsk:
				if a.approval == nil {
					return "", fmt.Errorf("approval manager is unavailable")
				}
				requestID, err := a.approval.EnqueueRequest(toolCall.Name, toolCall.Arguments, sessionID)
				if err != nil {
					return "", fmt.Errorf("enqueue approval request: %w", err)
				}
				if err := approval.RememberPendingToolCall(requestID, sessionID, toolCall); err != nil {
					return "", fmt.Errorf("track pending tool call: %w", err)
				}
				if a.taskStore != nil {
					a.taskStore.SetSessionPendingAction(sessionID, toolCall.Name, requestID)
				}
				return "Tool call pending approval", nil
			case permissionrules.ActionAllow:
				if a.taskStore != nil {
					a.taskStore.ClearSessionPendingAction(sessionID)
					a.taskStore.SetSessionLifecycleState(sessionID, tasks.SessionLifecycleProcessing, toolCall.Name)
				}
				skipApproval = true
			}
		}
	}

	// Check approval
	if a.approval != nil && !skipApproval {
		decision, requestID, err := a.approval.CheckApproval(
			toolCall.Name,
			toolCall.Arguments,
			sessionID,
		)
		if err != nil {
			return "", fmt.Errorf("approval check failed: %w", err)
		}
		switch decision {
		case approval.Denied:
			if a.taskStore != nil {
				a.taskStore.ClearSessionPendingAction(sessionID)
				a.taskStore.SetSessionLifecycleState(sessionID, tasks.SessionLifecycleIdle, "")
			}
			return "Tool call denied by approval policy", nil
		case approval.Pending:
			if err := approval.RememberPendingToolCall(requestID, sessionID, toolCall); err != nil {
				return "", fmt.Errorf("track pending tool call: %w", err)
			}
			if a.taskStore != nil {
				a.taskStore.SetSessionPendingAction(sessionID, toolCall.Name, requestID)
				if mode, ok := a.approval.GetSessionMode(sessionID); ok {
					a.taskStore.SetSessionPermissionMode(sessionID, string(mode))
				}
			}
			return "Tool call pending approval", nil
		case approval.Approved:
			if a.taskStore != nil {
				a.taskStore.ClearSessionPendingAction(sessionID)
				a.taskStore.SetSessionLifecycleState(sessionID, tasks.SessionLifecycleProcessing, toolCall.Name)
			}
			// continue
		}
	}

	if a.taskStore != nil && sessionID != "" {
		a.taskStore.SetSessionLifecycleState(sessionID, tasks.SessionLifecycleProcessing, toolCall.Name)
	}

	if toolCall.Name == "spawn" {
		ctx = tools.WithSpawnContext(
			ctx,
			ctxStringValue(ctx, promptContextChannelKey),
			ctxStringValue(ctx, promptContextSessionKey),
		)
	}

	if sessionID := ctxStringValue(ctx, promptContextSessionKey); sessionID != "" {
		ctx = context.WithValue(ctx, "session_id", sessionID)
	}
	if runtimeID := ctxStringValue(ctx, promptContextRuntimeKey); runtimeID != "" {
		ctx = context.WithValue(ctx, "runtime_id", runtimeID)
	}

	result, err := a.tools.Execute(ctx, toolCall.Name, toolCall.Arguments)
	if err != nil {
		if a.taskStore != nil {
			a.taskStore.SetSessionLifecycleState(sessionID, tasks.SessionLifecycleIdle, "")
		}
		return "", err
	}
	if a.taskStore != nil {
		a.taskStore.SetSessionLifecycleState(sessionID, tasks.SessionLifecycleIdle, "")
	}

	return result, nil
}

type promptContextKey string

const (
	promptContextChannelKey promptContextKey = "prompt_channel"
	promptContextSessionKey promptContextKey = "prompt_session_id"
	promptContextRuntimeKey promptContextKey = "prompt_runtime_id"
)

func ctxStringValue(ctx context.Context, key promptContextKey) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(key).(string)
	return value
}

func (a *subagentAgentAdapter) Chat(ctx context.Context, message string) (string, error) {
	if a == nil || a.agent == nil {
		return "", fmt.Errorf("agent adapter is nil")
	}

	sess := &subagentSession{messages: make([]Message, 0, 8)}
	return a.agent.Chat(ctx, sess, message)
}

type subagentSession struct {
	messages []Message
}

func (s *subagentSession) GetMessages() []Message {
	return s.messages
}

func (s *subagentSession) AddMessage(msg Message) {
	s.messages = append(s.messages, msg)
}

// convertToProviderMessages converts agent messages to provider format.
func (a *Agent) convertToProviderMessages(messages []Message) []providers.UnifiedMessage {
	providerMsgs := make([]providers.UnifiedMessage, len(messages))
	for i, msg := range messages {
		providerMsgs[i] = providers.UnifiedMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}

		// Convert tool calls
		if len(msg.ToolCalls) > 0 {
			providerMsgs[i].ToolCalls = make([]providers.UnifiedToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				providerMsgs[i].ToolCalls[j] = providers.UnifiedToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				}
			}
		}
	}
	return providerMsgs
}

// convertToolDefinitions converts tool registry definitions to provider format.
func (a *Agent) convertToolDefinitions() []providers.UnifiedTool {
	toolDefs := a.tools.GetToolDefinitions()
	unified := make([]providers.UnifiedTool, len(toolDefs))

	for i, def := range toolDefs {
		fn := def["function"].(map[string]interface{})
		unified[i] = providers.UnifiedTool{
			Type:        "function",
			Name:        fn["name"].(string),
			Description: fn["description"].(string),
			Parameters:  fn["parameters"].(map[string]interface{}),
		}
	}

	return unified
}

// GetContext returns the context builder.
func (a *Agent) GetContext() *ContextBuilder {
	return a.context
}

// GetTools returns the tool registry.
func (a *Agent) GetTools() *tools.Registry {
	return a.tools
}

// truncate truncates a string to the given length.
func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}
