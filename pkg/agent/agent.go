package agent

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/providers"
	"nekobot/pkg/skills"
	"nekobot/pkg/tools"
)

// Agent represents an AI agent that can interact with users and use tools.
type Agent struct {
	config  *config.Config
	logger  *logger.Logger
	client  *providers.Client
	tools   *tools.Registry
	context *ContextBuilder

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
func New(cfg *config.Config, log *logger.Logger, providerClient *providers.Client) (*Agent, error) {
	workspace := cfg.WorkspacePath()

	// Create tool registry
	toolRegistry := tools.NewRegistry()

	// Register built-in tools
	toolRegistry.MustRegister(tools.NewReadFileTool(workspace, cfg.Agents.Defaults.RestrictToWorkspace))
	toolRegistry.MustRegister(tools.NewWriteFileTool(workspace, cfg.Agents.Defaults.RestrictToWorkspace))
	toolRegistry.MustRegister(tools.NewListDirTool(workspace, cfg.Agents.Defaults.RestrictToWorkspace))
	toolRegistry.MustRegister(tools.NewExecTool(workspace, cfg.Agents.Defaults.RestrictToWorkspace, 0))

	// Register web tools if configured
	if cfg.Tools.Web.Search.APIKey != "" {
		toolRegistry.MustRegister(tools.NewWebSearchTool(
			cfg.Tools.Web.Search.APIKey,
			cfg.Tools.Web.Search.MaxResults,
		))
		log.Info("Web search tool enabled")
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
func (a *Agent) Chat(ctx context.Context, userMessage string) (string, error) {
	a.logger.Info("Processing chat message",
		zap.String("message", truncate(userMessage, 100)),
	)

	// Build initial messages
	history := []Message{} // TODO: load from session
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
			Model:       a.config.Agents.Defaults.Model,
			Messages:    providerMessages,
			Tools:       toolDefs,
			MaxTokens:   a.config.Agents.Defaults.MaxTokens,
			Temperature: a.config.Agents.Defaults.Temperature,
		}

		// Call LLM
		resp, err := a.client.Chat(ctx, req)
		if err != nil {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		a.logger.Debug("LLM response",
			zap.String("content", truncate(resp.Content, 100)),
			zap.Int("tool_calls", len(resp.ToolCalls)),
			zap.String("finish_reason", resp.FinishReason),
		)

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

// executeToolCall executes a single tool call.
func (a *Agent) executeToolCall(ctx context.Context, toolCall providers.UnifiedToolCall) (string, error) {
	a.logger.Info("Executing tool",
		zap.String("tool", toolCall.Name),
		zap.Any("args", toolCall.Arguments),
	)

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
