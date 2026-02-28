package agent

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"nekobot/pkg/config"
)

const (
	acpDefaultModeID   = "default"
	acpDefaultModeName = "Default"
)

// ACPAdapter bridges nekobot Agent to ACP Agent interface.
type ACPAdapter struct {
	agent *Agent

	sessionUpdateMu sync.RWMutex
	sessionUpdateFn func(context.Context, acp.SessionNotification) error
}

var (
	_ acp.Agent             = (*ACPAdapter)(nil)
	_ acp.AgentLoader       = (*ACPAdapter)(nil)
	_ acp.AgentExperimental = (*ACPAdapter)(nil)
)

// NewACPAdapter creates a new ACP adapter for the given agent.
func NewACPAdapter(agentInstance *Agent) *ACPAdapter {
	return &ACPAdapter{agent: agentInstance}
}

// SetAgentConnection wires the ACP connection so Prompt can emit session updates.
func (a *ACPAdapter) SetAgentConnection(conn *acp.AgentSideConnection) {
	if conn == nil {
		a.setSessionUpdateFunc(nil)
		return
	}
	a.setSessionUpdateFunc(func(ctx context.Context, notification acp.SessionNotification) error {
		return conn.SessionUpdate(ctx, notification)
	})
}

func (a *ACPAdapter) setSessionUpdateFunc(fn func(context.Context, acp.SessionNotification) error) {
	a.sessionUpdateMu.Lock()
	defer a.sessionUpdateMu.Unlock()
	a.sessionUpdateFn = fn
}

func (a *ACPAdapter) sendSessionUpdate(ctx context.Context, sessionID string, update acp.SessionUpdate) error {
	a.sessionUpdateMu.RLock()
	sessionUpdate := a.sessionUpdateFn
	a.sessionUpdateMu.RUnlock()
	if sessionUpdate == nil {
		return nil
	}

	notification := acp.SessionNotification{
		SessionId: acp.SessionId(sessionID),
		Update:    update,
	}
	if err := sessionUpdate(ctx, notification); err != nil {
		return fmt.Errorf("send session update for session %s: %w", sessionID, err)
	}
	return nil
}

// Authenticate accepts no-op authentication for local ACP usage.
func (a *ACPAdapter) Authenticate(ctx context.Context, params acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

// Initialize returns protocol and capability metadata for this agent.
func (a *ACPAdapter) Initialize(ctx context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersion(acp.ProtocolVersionNumber),
		AgentInfo: &acp.Implementation{
			Name:    "nekobot",
			Version: "unknown",
		},
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: true,
			McpCapabilities: acp.McpCapabilities{
				Http: true,
				Sse:  true,
			},
			PromptCapabilities: acp.PromptCapabilities{
				EmbeddedContext: true,
			},
		},
		AuthMethods: []acp.AuthMethod{},
	}, nil
}

// Cancel records cancellation for an in-flight session prompt.
func (a *ACPAdapter) Cancel(ctx context.Context, params acp.CancelNotification) error {
	sessID := strings.TrimSpace(string(params.SessionId))
	if sessID == "" {
		return nil
	}

	a.agent.acpMu.Lock()
	defer a.agent.acpMu.Unlock()

	state, ok := a.agent.acpSessions[sessID]
	if !ok || state == nil {
		return nil
	}
	if state.cancel != nil {
		state.cancel()
		state.cancel = nil
	}

	return nil
}

// LoadSession binds a client-provided ACP session ID to a fresh in-memory session state.
func (a *ACPAdapter) LoadSession(ctx context.Context, params acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	sessID := strings.TrimSpace(string(params.SessionId))
	if sessID == "" {
		return acp.LoadSessionResponse{}, acp.NewInvalidParams(map[string]interface{}{
			"error": "sessionId is required",
		})
	}

	workspace := strings.TrimSpace(params.Cwd)
	if !filepath.IsAbs(workspace) {
		return acp.LoadSessionResponse{}, acp.NewInvalidParams(map[string]interface{}{
			"error": "cwd must be an absolute path",
		})
	}

	sessionState := &acpEphemeralSession{messages: make([]Message, 0, 16)}
	provider := strings.TrimSpace(a.agent.config.Agents.Defaults.Provider)
	model := strings.TrimSpace(a.agent.config.Agents.Defaults.Model)
	fallback := append([]string(nil), a.agent.config.Agents.Defaults.Fallback...)
	mcpServers := acpMCPServersToConfig(params.McpServers)

	state := &acpSessionState{
		session:    sessionState,
		provider:   provider,
		model:      model,
		fallback:   fallback,
		modeID:     acpDefaultModeID,
		mcpServers: mcpServers,
	}

	a.agent.acpMu.Lock()
	a.agent.acpSessions[sessID] = state
	a.agent.acpMu.Unlock()

	return acp.LoadSessionResponse{
		Models: state.modelState(a.agent.config.Agents.Defaults.Model),
		Modes:  state.modeState(),
	}, nil
}

// NewSession creates a session and stores ACP-scoped routing defaults.
func (a *ACPAdapter) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	workspace := strings.TrimSpace(params.Cwd)
	if workspace == "" {
		workspace = a.agent.config.WorkspacePath()
	}
	if !filepath.IsAbs(workspace) {
		return acp.NewSessionResponse{}, acp.NewInvalidParams(map[string]interface{}{
			"error": "cwd must be an absolute path",
		})
	}

	sessionID := "acp:" + uuid.NewString()
	sessionState := &acpEphemeralSession{messages: make([]Message, 0, 16)}

	provider := strings.TrimSpace(a.agent.config.Agents.Defaults.Provider)
	model := strings.TrimSpace(a.agent.config.Agents.Defaults.Model)
	fallback := append([]string(nil), a.agent.config.Agents.Defaults.Fallback...)
	mcpServers := acpMCPServersToConfig(params.McpServers)

	state := &acpSessionState{
		session:    sessionState,
		provider:   provider,
		model:      model,
		fallback:   fallback,
		modeID:     acpDefaultModeID,
		mcpServers: mcpServers,
	}

	a.agent.acpMu.Lock()
	a.agent.acpSessions[sessionID] = state
	a.agent.acpMu.Unlock()

	return acp.NewSessionResponse{
		SessionId: acp.SessionId(sessionID),
		Models:    state.modelState(a.agent.config.Agents.Defaults.Model),
		Modes:     state.modeState(),
	}, nil
}

// Prompt converts ACP content blocks into a user message and runs the existing agent pipeline.
func (a *ACPAdapter) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	sessID := strings.TrimSpace(string(params.SessionId))
	if sessID == "" {
		return acp.PromptResponse{}, acp.NewInvalidParams(map[string]interface{}{
			"error": "sessionId is required",
		})
	}

	state, err := a.getACPSessionState(sessID)
	if err != nil {
		return acp.PromptResponse{}, err
	}

	userMessage := acpPromptBlocksToText(params.Prompt)
	if strings.TrimSpace(userMessage) == "" {
		return acp.PromptResponse{}, acp.NewInvalidParams(map[string]interface{}{
			"error": "prompt text is empty",
		})
	}

	promptCtx, promptCancel := context.WithCancel(ctx)
	if err := a.setSessionCancel(sessID, promptCancel); err != nil {
		promptCancel()
		return acp.PromptResponse{}, err
	}
	defer func() {
		a.clearSessionCancel(sessID)
		promptCancel()
	}()

	response, chatErr := a.agent.ChatWithProviderModelAndFallback(
		promptCtx,
		state.session,
		userMessage,
		state.provider,
		state.model,
		state.fallback,
	)
	if chatErr != nil {
		if errors.Is(chatErr, context.Canceled) || errors.Is(promptCtx.Err(), context.Canceled) {
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
		}
		return acp.PromptResponse{}, acp.NewInternalError(map[string]interface{}{
			"error": chatErr.Error(),
		})
	}

	if strings.TrimSpace(response) != "" {
		if err := a.sendSessionUpdate(promptCtx, sessID, acp.UpdateAgentMessageText(response)); err != nil {
			if errors.Is(promptCtx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
				return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
			}
			return acp.PromptResponse{}, acp.NewInternalError(map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	state.session.AddMessage(Message{Role: "user", Content: userMessage})
	state.session.AddMessage(Message{Role: "assistant", Content: response})

	if strings.TrimSpace(response) != "" {
		a.agent.logger.Debug("ACP prompt completed",
			zap.String("session_id", sessID),
			zap.String("mode_id", state.modeID),
		)
	}

	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

// SetSessionMode updates the session mode (stored for protocol compatibility).
func (a *ACPAdapter) SetSessionMode(ctx context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	sessID := strings.TrimSpace(string(params.SessionId))
	if sessID == "" {
		return acp.SetSessionModeResponse{}, acp.NewInvalidParams(map[string]interface{}{
			"error": "sessionId is required",
		})
	}

	modeID := strings.TrimSpace(string(params.ModeId))
	if modeID == "" {
		modeID = acpDefaultModeID
	}

	a.agent.acpMu.Lock()
	state, ok := a.agent.acpSessions[sessID]
	if !ok || state == nil {
		a.agent.acpMu.Unlock()
		return acp.SetSessionModeResponse{}, acp.NewInvalidParams(map[string]interface{}{
			"error": fmt.Sprintf("unknown sessionId: %s", sessID),
		})
	}
	state.modeID = modeID
	a.agent.acpMu.Unlock()

	update := acp.SessionUpdate{
		CurrentModeUpdate: &acp.SessionCurrentModeUpdate{
			CurrentModeId: acp.SessionModeId(modeID),
		},
	}
	if err := a.sendSessionUpdate(ctx, sessID, update); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			return acp.SetSessionModeResponse{}, nil
		}
		return acp.SetSessionModeResponse{}, acp.NewInternalError(map[string]interface{}{
			"error": err.Error(),
		})
	}

	return acp.SetSessionModeResponse{}, nil
}

func (a *ACPAdapter) SetSessionModel(ctx context.Context, params acp.SetSessionModelRequest) (acp.SetSessionModelResponse, error) {
	sessID := strings.TrimSpace(string(params.SessionId))
	if sessID == "" {
		return acp.SetSessionModelResponse{}, acp.NewInvalidParams(map[string]interface{}{
			"error": "sessionId is required",
		})
	}

	modelID := strings.TrimSpace(string(params.ModelId))
	if modelID == "" {
		return acp.SetSessionModelResponse{}, acp.NewInvalidParams(map[string]interface{}{
			"error": "modelId is required",
		})
	}

	a.agent.acpMu.Lock()
	defer a.agent.acpMu.Unlock()

	state, ok := a.agent.acpSessions[sessID]
	if !ok || state == nil {
		return acp.SetSessionModelResponse{}, acp.NewInvalidParams(map[string]interface{}{
			"error": fmt.Sprintf("unknown sessionId: %s", sessID),
		})
	}

	state.model = modelID
	return acp.SetSessionModelResponse{}, nil
}

func (a *ACPAdapter) getACPSessionState(sessionID string) (*acpSessionState, error) {
	a.agent.acpMu.RLock()
	state, ok := a.agent.acpSessions[sessionID]
	a.agent.acpMu.RUnlock()
	if !ok || state == nil {
		return nil, acp.NewInvalidParams(map[string]interface{}{
			"error": fmt.Sprintf("unknown sessionId: %s", sessionID),
		})
	}
	return state, nil
}

func (a *ACPAdapter) setSessionCancel(sessionID string, cancel context.CancelFunc) error {
	a.agent.acpMu.Lock()
	defer a.agent.acpMu.Unlock()

	state, ok := a.agent.acpSessions[sessionID]
	if !ok || state == nil {
		return acp.NewInvalidParams(map[string]interface{}{
			"error": fmt.Sprintf("unknown sessionId: %s", sessionID),
		})
	}
	state.cancel = cancel
	return nil
}

func (a *ACPAdapter) clearSessionCancel(sessionID string) {
	a.agent.acpMu.Lock()
	defer a.agent.acpMu.Unlock()

	state, ok := a.agent.acpSessions[sessionID]
	if !ok || state == nil {
		return
	}
	state.cancel = nil
}

func acpMCPServersToConfig(servers []acp.McpServer) []config.MCPServerConfig {
	if len(servers) == 0 {
		return nil
	}

	configs := make([]config.MCPServerConfig, 0, len(servers))
	for _, server := range servers {
		switch {
		case server.Stdio != nil:
			cfg := config.MCPServerConfig{
				Name:      strings.TrimSpace(server.Stdio.Name),
				Transport: "stdio",
				Command:   strings.TrimSpace(server.Stdio.Command),
				Args:      append([]string(nil), server.Stdio.Args...),
				Env:       map[string]string{},
			}
			for _, item := range server.Stdio.Env {
				name := strings.TrimSpace(item.Name)
				if name == "" {
					continue
				}
				cfg.Env[name] = item.Value
			}
			configs = append(configs, cfg)
		case server.Http != nil:
			cfg := config.MCPServerConfig{
				Name:      strings.TrimSpace(server.Http.Name),
				Transport: "http",
				Endpoint:  strings.TrimSpace(server.Http.Url),
				Headers:   map[string]string{},
			}
			for _, header := range server.Http.Headers {
				name := strings.TrimSpace(header.Name)
				if name == "" {
					continue
				}
				cfg.Headers[name] = header.Value
			}
			configs = append(configs, cfg)
		case server.Sse != nil:
			cfg := config.MCPServerConfig{
				Name:      strings.TrimSpace(server.Sse.Name),
				Transport: "sse",
				Endpoint:  strings.TrimSpace(server.Sse.Url),
				Headers:   map[string]string{},
			}
			for _, header := range server.Sse.Headers {
				name := strings.TrimSpace(header.Name)
				if name == "" {
					continue
				}
				cfg.Headers[name] = header.Value
			}
			configs = append(configs, cfg)
		}
	}

	return configs
}

func (s *acpSessionState) modelState(defaultModel string) *acp.SessionModelState {
	currentModel := strings.TrimSpace(s.model)
	if currentModel == "" {
		currentModel = strings.TrimSpace(defaultModel)
	}
	if currentModel == "" {
		return nil
	}

	models := []acp.ModelInfo{{
		ModelId: acp.ModelId(currentModel),
		Name:    currentModel,
	}}

	return &acp.SessionModelState{
		CurrentModelId:  acp.ModelId(currentModel),
		AvailableModels: models,
	}
}

func (s *acpSessionState) modeState() *acp.SessionModeState {
	currentMode := strings.TrimSpace(s.modeID)
	if currentMode == "" {
		currentMode = acpDefaultModeID
	}

	return &acp.SessionModeState{
		CurrentModeId: acp.SessionModeId(currentMode),
		AvailableModes: []acp.SessionMode{
			{Id: acp.SessionModeId(acpDefaultModeID), Name: acpDefaultModeName},
		},
	}
}

func acpPromptBlocksToText(blocks []acp.ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch {
		case block.Text != nil:
			text := strings.TrimSpace(block.Text.Text)
			if text != "" {
				parts = append(parts, text)
			}
		case block.Resource != nil && block.Resource.Resource.TextResourceContents != nil:
			text := strings.TrimSpace(block.Resource.Resource.TextResourceContents.Text)
			if text != "" {
				parts = append(parts, text)
			}
		case block.ResourceLink != nil:
			uri := strings.TrimSpace(block.ResourceLink.Uri)
			if uri != "" {
				parts = append(parts, uri)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

type acpEphemeralSession struct {
	messages []Message
}

func (s *acpEphemeralSession) GetMessages() []Message {
	copied := make([]Message, len(s.messages))
	copy(copied, s.messages)
	return copied
}

func (s *acpEphemeralSession) AddMessage(msg Message) {
	s.messages = append(s.messages, msg)
}
