package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"nekobot/pkg/approval"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/providers"
	"nekobot/pkg/skills"
	"nekobot/pkg/tools"
	"nekobot/pkg/toolsessions"
)

// SessionInterface defines the interface for a conversation session.
type SessionInterface interface {
	GetMessages() []Message
	AddMessage(Message)
}

// Agent represents an AI agent that can interact with users and use tools.
type Agent struct {
	config   *config.Config
	logger   *logger.Logger
	client   *providers.Client
	tools    *tools.Registry
	context  *ContextBuilder
	approval *approval.Manager

	maxIterations int
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

// New creates a new agent with the given configuration.
func New(cfg *config.Config, log *logger.Logger, providerClient *providers.Client, processMgr *process.Manager, approvalMgr *approval.Manager, toolSessionMgr *toolsessions.Manager) (*Agent, error) {
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
		toolRegistry.MustRegister(tools.NewToolSessionTool(processMgr, toolSessionMgr, workspace))
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

	// Create context builder
	contextBuilder := NewContextBuilder(workspace)

	// Set tool descriptions function
	contextBuilder.SetToolDescriptionsFunc(toolRegistry.GetDescriptions)

	agent := &Agent{
		config:        cfg,
		logger:        log,
		client:        providerClient,
		tools:         toolRegistry,
		context:       contextBuilder,
		approval:      approvalMgr,
		maxIterations: cfg.Agents.Defaults.MaxToolIterations,
	}

	return agent, nil
}

// RegisterSkillTool registers the skill tool with the agent.
// This should be called after agent creation when skills manager is available.
func (a *Agent) RegisterSkillTool(skillsManager *skills.Manager) {
	a.tools.MustRegister(tools.NewSkillTool(a.logger, skillsManager))
	a.logger.Info("Skill tool registered")
}

// Chat processes a user message and returns the agent's response.
// It handles tool calls and iterates until the agent produces a final response.
func (a *Agent) Chat(ctx context.Context, sess SessionInterface, userMessage string) (string, error) {
	return a.chatWithProviderModel(ctx, sess, userMessage, "", a.config.Agents.Defaults.Model, nil)
}

// ChatWithModel processes a message using a specific model override.
func (a *Agent) ChatWithModel(ctx context.Context, sess SessionInterface, userMessage, model string) (string, error) {
	return a.chatWithProviderModel(ctx, sess, userMessage, "", model, nil)
}

// ChatWithProviderModel processes a message using provider/model overrides.
func (a *Agent) ChatWithProviderModel(ctx context.Context, sess SessionInterface, userMessage, provider, model string) (string, error) {
	return a.chatWithProviderModel(ctx, sess, userMessage, provider, model, nil)
}

// ChatWithProviderModelAndFallback processes a message using provider/model/fallback overrides.
func (a *Agent) ChatWithProviderModelAndFallback(ctx context.Context, sess SessionInterface, userMessage, provider, model string, fallback []string) (string, error) {
	return a.chatWithProviderModel(ctx, sess, userMessage, provider, model, fallback)
}

func (a *Agent) chatWithProviderModel(ctx context.Context, sess SessionInterface, userMessage, provider, model string, fallback []string) (string, error) {
	a.logger.Info("Processing chat message",
		zap.String("message", truncate(userMessage, 100)),
	)
	if model == "" {
		model = a.config.Agents.Defaults.Model
	}

	providerOrder, err := a.buildProviderOrder(provider, fallback)
	if err != nil {
		return "", err
	}
	primaryProvider := providerOrder[0]
	clientCache := make(map[string]*providers.Client)

	// Build initial messages with session history
	history := sess.GetMessages() // Get messages from session
	messages := a.context.BuildMessages(history, userMessage)

	// Convert to provider format
	providerMessages := a.convertToProviderMessages(messages)

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
			return "", fmt.Errorf("LLM call failed: %w", err)
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
			return resp.Content, nil
		}

		// Execute tool calls
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

	return "", fmt.Errorf("max iterations (%d) reached without final response", a.maxIterations)
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
	primary := strings.TrimSpace(provider)
	if primary == "" {
		primary = strings.TrimSpace(a.config.Agents.Defaults.Provider)
	}

	fallbackOrder := fallback
	if len(fallbackOrder) == 0 {
		fallbackOrder = a.config.Agents.Defaults.Fallback
	}

	seen := make(map[string]struct{})
	order := make([]string, 0, 1+len(fallbackOrder))

	addProvider := func(name string) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		order = append(order, trimmed)
	}

	addProvider(primary)
	for _, name := range fallbackOrder {
		addProvider(name)
	}

	if len(order) == 0 && len(a.config.Providers) > 0 {
		addProvider(a.config.Providers[0].Name)
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
	var lastErr error

	for _, providerName := range providerOrder {
		model := a.resolveModelForProvider(providerName, primaryProvider, requestedModel)

		client, err := a.getProviderClient(providerName, model, clientCache)
		if err != nil {
			lastErr = err
			a.logger.Warn("Provider unavailable", zap.String("provider", providerName), zap.Error(err))
			continue
		}

		reqCopy := *req
		reqCopy.Model = model

		resp, err := client.Chat(ctx, &reqCopy)
		if err != nil {
			lastErr = err
			a.logger.Warn("Provider request failed", zap.String("provider", providerName), zap.String("model", model), zap.Error(err))
			continue
		}

		return resp, providerName, model, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no provider attempt made")
	}
	return nil, "", "", lastErr
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

func (a *Agent) resolveModelForProvider(providerName, primaryProvider, requestedModel string) string {
	model := strings.TrimSpace(requestedModel)
	if model == "" {
		model = strings.TrimSpace(a.config.Agents.Defaults.Model)
	}
	if providerName == primaryProvider {
		return model
	}

	providerCfg := a.config.GetProviderConfig(providerName)
	if providerCfg == nil {
		return model
	}

	// If this provider declares no model list, keep caller's model.
	if len(providerCfg.Models) == 0 {
		return model
	}

	for _, candidate := range providerCfg.Models {
		if strings.TrimSpace(candidate) == model {
			return model
		}
	}

	if fallbackModel := strings.TrimSpace(providerCfg.GetDefaultModel()); fallbackModel != "" {
		return fallbackModel
	}

	return model
}

// executeToolCall executes a single tool call with approval checking.
func (a *Agent) executeToolCall(ctx context.Context, toolCall providers.UnifiedToolCall) (string, error) {
	a.logger.Info("Executing tool",
		zap.String("tool", toolCall.Name),
		zap.Any("args", toolCall.Arguments),
	)

	// Check approval
	if a.approval != nil {
		decision, _, err := a.approval.CheckApproval(toolCall.Name, toolCall.Arguments, "")
		if err != nil {
			return "", fmt.Errorf("approval check failed: %w", err)
		}
		switch decision {
		case approval.Denied:
			return "Tool call denied by approval policy", nil
		case approval.Pending:
			return "Tool call pending approval", nil
		case approval.Approved:
			// continue
		}
	}

	result, err := a.tools.Execute(ctx, toolCall.Name, toolCall.Arguments)
	if err != nil {
		return "", err
	}

	return result, nil
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
