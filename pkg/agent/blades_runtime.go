package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/blades"
	bladesmcp "github.com/go-kratos/blades/contrib/mcp"
	bladesmiddleware "github.com/go-kratos/blades/middleware"
	bladestools "github.com/go-kratos/blades/tools"
	"github.com/google/jsonschema-go/jsonschema"
	"go.uber.org/zap"
	"nekobot/pkg/config"
	"nekobot/pkg/providers"
	"nekobot/pkg/tools"
)

type bladesModelProvider struct {
	agent *Agent

	primaryProvider string
	providerOrder   []string
	requestedModel  string
	clientCache     map[string]*providers.Client
}

func newBladesModelProvider(a *Agent, primaryProvider string, providerOrder []string, requestedModel string) *bladesModelProvider {
	return &bladesModelProvider{
		agent:           a,
		primaryProvider: primaryProvider,
		providerOrder:   providerOrder,
		requestedModel:  requestedModel,
		clientCache:     make(map[string]*providers.Client),
	}
}

func (p *bladesModelProvider) Name() string {
	if strings.TrimSpace(p.requestedModel) != "" {
		return strings.TrimSpace(p.requestedModel)
	}
	return strings.TrimSpace(p.agent.config.Agents.Defaults.Model)
}

func (p *bladesModelProvider) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	unifiedReq, err := p.toUnifiedRequest(req)
	if err != nil {
		return nil, err
	}

	const maxContextRetries = 2
	for retry := 0; retry <= maxContextRetries; retry++ {
		resp, providerUsed, modelUsed, err := p.agent.callLLMWithFallback(
			ctx,
			unifiedReq,
			p.primaryProvider,
			p.providerOrder,
			p.requestedModel,
			p.clientCache,
		)
		if err == nil {
			p.agent.logger.Debug("Blades model response",
				zap.String("provider", providerUsed),
				zap.String("model", modelUsed),
				zap.Int("tool_calls", len(resp.ToolCalls)),
				zap.String("finish_reason", resp.FinishReason),
			)
			return p.toModelResponse(resp), nil
		}

		if isContextLimitError(err) && retry < maxContextRetries {
			p.agent.logger.Warn("Context window error in blades model call, compressing and retrying",
				zap.Error(err),
				zap.Int("retry", retry+1),
				zap.Int("messages_before", len(unifiedReq.Messages)),
			)
			unifiedReq.Messages = forceCompressMessages(unifiedReq.Messages)
			p.agent.logger.Info("Compressed blades model messages",
				zap.Int("messages_after", len(unifiedReq.Messages)),
				zap.Int("estimated_tokens", estimateTokens(unifiedReq.Messages)),
			)
			continue
		}

		return nil, fmt.Errorf("llm call with fallback: %w", err)
	}

	return nil, fmt.Errorf("llm call with fallback: retry exhausted")
}

func (p *bladesModelProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		resp, err := p.Generate(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}
		yield(resp, nil)
	}
}

func (p *bladesModelProvider) toUnifiedRequest(req *blades.ModelRequest) (*providers.UnifiedRequest, error) {
	messages, err := p.convertMessages(req.Messages)
	if err != nil {
		return nil, err
	}

	instructionMessages, err := p.convertMessages([]*blades.Message{req.Instruction})
	if err != nil {
		return nil, err
	}

	allMessages := make([]providers.UnifiedMessage, 0, len(instructionMessages)+len(messages))
	allMessages = append(allMessages, instructionMessages...)
	allMessages = append(allMessages, messages...)

	tools, err := p.convertTools(req.Tools)
	if err != nil {
		return nil, err
	}

	unifiedReq := &providers.UnifiedRequest{
		Model:       p.requestedModel,
		Messages:    allMessages,
		Tools:       tools,
		MaxTokens:   p.agent.config.Agents.Defaults.MaxTokens,
		Temperature: p.agent.config.Agents.Defaults.Temperature,
	}

	if p.agent.config.Agents.Defaults.ExtendedThinking {
		unifiedReq.Extra = map[string]interface{}{
			"extended_thinking": true,
			"thinking_budget":   p.agent.config.Agents.Defaults.ThinkingBudget,
		}
	}

	return unifiedReq, nil
}

func (p *bladesModelProvider) convertMessages(messages []*blades.Message) ([]providers.UnifiedMessage, error) {
	unified := make([]providers.UnifiedMessage, 0, len(messages))

	for _, msg := range messages {
		if msg == nil {
			continue
		}

		if msg.Role == blades.RoleTool {
			for _, rawPart := range msg.Parts {
				part, ok := rawPart.(blades.ToolPart)
				if !ok {
					continue
				}
				content := part.Response
				if strings.TrimSpace(content) == "" {
					content = part.Request
				}
				unified = append(unified, providers.UnifiedMessage{
					Role:       string(blades.RoleTool),
					ToolCallID: part.ID,
					Content:    content,
				})
			}
			continue
		}

		item := providers.UnifiedMessage{Role: string(msg.Role)}
		for _, rawPart := range msg.Parts {
			switch part := rawPart.(type) {
			case blades.TextPart:
				if strings.TrimSpace(part.Text) == "" {
					continue
				}
				if item.Content == "" {
					item.Content = part.Text
				} else {
					item.Content += "\n" + part.Text
				}
			case blades.ToolPart:
				if item.Role != string(blades.RoleAssistant) {
					continue
				}
				args := map[string]interface{}{}
				if strings.TrimSpace(part.Request) != "" {
					if err := json.Unmarshal([]byte(part.Request), &args); err != nil {
						return nil, fmt.Errorf("decode tool request for %s: %w", part.Name, err)
					}
				}
				item.ToolCalls = append(item.ToolCalls, providers.UnifiedToolCall{
					ID:        part.ID,
					Name:      part.Name,
					Arguments: args,
				})
			}
		}

		if item.Role == "" {
			continue
		}
		unified = append(unified, item)
	}

	return unified, nil
}

func (p *bladesModelProvider) convertTools(defs []bladestools.Tool) ([]providers.UnifiedTool, error) {
	unified := make([]providers.UnifiedTool, 0, len(defs))
	for _, tool := range defs {
		if tool == nil {
			continue
		}
		params := schemaToMap(tool.InputSchema())
		if params == nil {
			params = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		unified = append(unified, providers.UnifiedTool{
			Type:        "function",
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  params,
		})
	}
	return unified, nil
}

func (p *bladesModelProvider) toModelResponse(resp *providers.UnifiedResponse) *blades.ModelResponse {
	message := blades.NewAssistantMessage(blades.StatusCompleted)
	message.FinishReason = resp.FinishReason
	if strings.TrimSpace(resp.Content) != "" {
		message.Parts = append(message.Parts, blades.TextPart{Text: resp.Content})
	}
	if len(resp.ToolCalls) > 0 {
		message.Parts = make([]blades.Part, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			argJSON := "{}"
			if len(tc.Arguments) > 0 {
				if b, err := json.Marshal(tc.Arguments); err == nil {
					argJSON = string(b)
				}
			}
			message.Parts = append(message.Parts, blades.ToolPart{
				ID:      tc.ID,
				Name:    tc.Name,
				Request: argJSON,
			})
		}
		message.Role = blades.RoleTool
	}
	return &blades.ModelResponse{Message: message}
}

type bladesToolResolver struct {
	registry *tools.Registry
	agent    *Agent
}

type multiToolResolver struct {
	resolvers []bladestools.Resolver
}

func (r *multiToolResolver) Resolve(ctx context.Context) ([]bladestools.Tool, error) {
	resolved := make([]bladestools.Tool, 0)
	for _, resolver := range r.resolvers {
		toolsFromResolver, err := resolver.Resolve(ctx)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, toolsFromResolver...)
	}
	return resolved, nil
}

func newBladesToolResolver(agentInstance *Agent, registry *tools.Registry) *bladesToolResolver {
	return &bladesToolResolver{registry: registry, agent: agentInstance}
}

func (r *bladesToolResolver) Resolve(ctx context.Context) ([]bladestools.Tool, error) {
	names := r.registry.List()
	resolved := make([]bladestools.Tool, 0, len(names))

	for _, toolName := range names {
		toolImpl, ok := r.registry.Get(toolName)
		if !ok {
			continue
		}

		inputSchema := mapToSchema(toolImpl.Parameters())
		outputSchema := mapToSchema(map[string]interface{}{"type": "string"})
		capturedName := toolName

		handler := bladestools.HandleFunc(func(toolCtx context.Context, input string) (string, error) {
			args := map[string]interface{}{}
			trimmed := strings.TrimSpace(input)
			if trimmed != "" && trimmed != "null" {
				if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
					return "", fmt.Errorf("decode args for tool %s: %w", capturedName, err)
				}
			}

			result, err := r.agent.executeToolCall(toolCtx, providers.UnifiedToolCall{
				ID:        "",
				Name:      capturedName,
				Arguments: args,
			})
			if err != nil {
				r.agent.logger.Error("Tool execution failed",
					zap.String("tool", capturedName),
					zap.Error(err),
				)
				return fmt.Sprintf("Error: %v", err), nil
			}
			return result, nil
		})

		resolved = append(resolved, bladestools.NewTool(
			capturedName,
			toolImpl.Description(),
			handler,
			bladestools.WithInputSchema(inputSchema),
			bladestools.WithOutputSchema(outputSchema),
		))
	}

	return resolved, nil
}

func mapToSchema(m map[string]interface{}) *jsonschema.Schema {
	if len(m) == 0 {
		return &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{}}
	}

	b, err := json.Marshal(m)
	if err != nil {
		return &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{}}
	}

	var s jsonschema.Schema
	if err := json.Unmarshal(b, &s); err != nil {
		return &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{}}
	}
	return &s
}

func newBladesToolsResolver() *multiToolResolver {
	return &multiToolResolver{resolvers: make([]bladestools.Resolver, 0, 2)}
}

func (r *multiToolResolver) appendResolver(resolver bladestools.Resolver) {
	if resolver == nil {
		return
	}
	r.resolvers = append(r.resolvers, resolver)
}

func parseMCPTransport(raw string) (bladesmcp.TransportType, error) {
	transport := strings.TrimSpace(strings.ToLower(raw))
	if transport == "" {
		transport = string(bladesmcp.TransportStdio)
	}
	if transport == "sse" {
		transport = string(bladesmcp.TransportHTTP)
	}

	switch bladesmcp.TransportType(transport) {
	case bladesmcp.TransportStdio, bladesmcp.TransportHTTP, bladesmcp.TransportWebSocket:
		return bladesmcp.TransportType(transport), nil
	default:
		return "", fmt.Errorf("unsupported transport: %s", raw)
	}
}

func parseMCPTimeout(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}

	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parse timeout duration: %w", err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("timeout duration must be greater than 0")
	}
	return d, nil
}

func mcpServerName(cfg config.MCPServerConfig, idx int) string {
	name := strings.TrimSpace(cfg.Name)
	if name != "" {
		return name
	}
	return fmt.Sprintf("index-%d", idx)
}

func toMCPClientConfigs(serverConfigs []config.MCPServerConfig) ([]bladesmcp.ClientConfig, error) {
	if len(serverConfigs) == 0 {
		return nil, nil
	}

	res := make([]bladesmcp.ClientConfig, 0, len(serverConfigs))
	for i, server := range serverConfigs {
		transport, err := parseMCPTransport(server.Transport)
		if err != nil {
			return nil, fmt.Errorf("mcp server %s transport: %w", mcpServerName(server, i), err)
		}

		timeout, err := parseMCPTimeout(server.Timeout)
		if err != nil {
			return nil, fmt.Errorf("mcp server %s timeout: %w", mcpServerName(server, i), err)
		}

		res = append(res, bladesmcp.ClientConfig{
			Name:      strings.TrimSpace(server.Name),
			Transport: transport,
			Command:   strings.TrimSpace(server.Command),
			Args:      server.Args,
			Env:       server.Env,
			WorkDir:   strings.TrimSpace(server.WorkDir),
			Endpoint:  strings.TrimSpace(server.Endpoint),
			Headers:   server.Headers,
			Timeout:   timeout,
		})
	}

	return res, nil
}

func (a *Agent) buildBladesToolsResolverWithMCP(serverConfigs []config.MCPServerConfig) (bladestools.Resolver, *bladesmcp.ToolsResolver, error) {
	resolver := newBladesToolsResolver()
	resolver.appendResolver(newBladesToolResolver(a, a.tools))

	mcpConfigs, err := toMCPClientConfigs(serverConfigs)
	if err != nil {
		return nil, nil, err
	}
	if len(mcpConfigs) == 0 {
		return resolver, nil, nil
	}

	mcpResolver, err := bladesmcp.NewToolsResolver(mcpConfigs...)
	if err != nil {
		return nil, nil, fmt.Errorf("create mcp tools resolver: %w", err)
	}

	resolver.appendResolver(mcpResolver)
	return resolver, mcpResolver, nil
}

func (a *Agent) buildBladesToolsResolver() (bladestools.Resolver, *bladesmcp.ToolsResolver, error) {
	return a.buildBladesToolsResolverWithMCP(a.config.Agents.Defaults.MCPServers)
}

func schemaToMap(s *jsonschema.Schema) map[string]interface{} {
	if s == nil {
		return nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

func (a *Agent) chatWithBladesOrchestrator(ctx context.Context, sess SessionInterface, userMessage, provider, model string, fallback []string) (string, error) {
	a.logger.Info("Processing chat message with blades orchestrator",
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

	modelProvider := newBladesModelProvider(a, primaryProvider, providerOrder, model)
	toolResolver, mcpResolver, err := a.buildBladesToolsResolver()
	if state, ok := a.lookupACPSessionState(sess); ok && len(state.mcpServers) > 0 {
		toolResolver, mcpResolver, err = a.buildBladesToolsResolverWithMCP(state.mcpServers)
	}
	if err != nil {
		return "", fmt.Errorf("build blades tools resolver: %w", err)
	}
	if mcpResolver != nil {
		defer func() {
			if closeErr := mcpResolver.Close(); closeErr != nil {
				a.logger.Warn("Close MCP resolver failed", zap.Error(closeErr))
			}
		}()
	}

	instruction := a.context.BuildSystemPrompt()
	agentInstance, err := blades.NewAgent(
		"nekobot-orchestrator",
		blades.WithModel(modelProvider),
		blades.WithInstruction(instruction),
		blades.WithToolsResolver(toolResolver),
		blades.WithMiddleware(bladesmiddleware.ConversationBuffered(a.maxIterations*4)),
		blades.WithMaxIterations(a.maxIterations),
	)
	if err != nil {
		return "", fmt.Errorf("create blades agent: %w", err)
	}

	normalizedHistory := trimTrailingCurrentUserMessage(sess.GetMessages(), userMessage)
	history := sanitizeHistory(normalizedHistory)
	bladesSession := blades.NewSession()
	for _, msg := range history {
		if msg.Role == "system" {
			continue
		}
		if !hasBladesHistoryContent(msg) {
			continue
		}
		if err := bladesSession.Append(ctx, toBladesMessage(msg)); err != nil {
			return "", fmt.Errorf("append session history: %w", err)
		}
	}

	runner := blades.NewRunner(agentInstance)
	output, err := runner.Run(ctx, blades.UserMessage(userMessage), blades.WithSession(bladesSession))
	if err != nil {
		return "", fmt.Errorf("blades runner run: %w", err)
	}

	return output.Text(), nil
}

func (a *Agent) lookupACPSessionState(sess SessionInterface) (*acpSessionState, bool) {
	a.acpMu.RLock()
	defer a.acpMu.RUnlock()

	for _, state := range a.acpSessions {
		if state == nil || state.session == nil {
			continue
		}
		if state.session == sess {
			return state, true
		}
	}

	return nil, false
}

func hasBladesHistoryContent(msg Message) bool {
	switch msg.Role {
	case "assistant":
		return strings.TrimSpace(msg.Content) != "" || len(msg.ToolCalls) > 0
	case "tool":
		return strings.TrimSpace(msg.Content) != "" || strings.TrimSpace(msg.ToolCallID) != ""
	default:
		return strings.TrimSpace(msg.Content) != ""
	}
}

func toBladesMessage(msg Message) *blades.Message {
	switch msg.Role {
	case "assistant":
		if len(msg.ToolCalls) == 0 {
			return blades.AssistantMessage(msg.Content)
		}
		parts := make([]any, 0, len(msg.ToolCalls)+1)
		if strings.TrimSpace(msg.Content) != "" {
			parts = append(parts, blades.TextPart{Text: msg.Content})
		}
		for _, tc := range msg.ToolCalls {
			request := "{}"
			if len(tc.Arguments) > 0 {
				if b, err := json.Marshal(tc.Arguments); err == nil {
					request = string(b)
				}
			}
			parts = append(parts, blades.ToolPart{ID: tc.ID, Name: tc.Name, Request: request})
		}
		return blades.AssistantMessage(parts...)
	case "tool":
		parts := []any{blades.ToolPart{ID: msg.ToolCallID, Response: msg.Content}}
		return &blades.Message{
			ID:     blades.NewMessageID(),
			Role:   blades.RoleTool,
			Status: blades.StatusCompleted,
			Parts:  blades.NewMessageParts(parts...),
		}
	case "user":
		fallthrough
	default:
		return blades.UserMessage(msg.Content)
	}
}
