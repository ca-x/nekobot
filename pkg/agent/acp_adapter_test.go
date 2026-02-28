package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"nekobot/pkg/config"
)

func TestACPMCPServersToConfig_MapsAllTransports(t *testing.T) {
	servers := []acp.McpServer{
		{
			Stdio: &acp.McpServerStdio{
				Name:    "stdio-server",
				Command: "npx",
				Args:    []string{"server"},
				Env: []acp.EnvVariable{
					{Name: "FOO", Value: "bar"},
				},
			},
		},
		{
			Http: &acp.McpServerHttp{
				Name: "http-server",
				Url:  "https://example.com/mcp",
				Headers: []acp.HttpHeader{
					{Name: "Authorization", Value: "Bearer token"},
				},
			},
		},
		{
			Sse: &acp.McpServerSse{
				Name: "sse-server",
				Url:  "https://example.com/sse",
				Headers: []acp.HttpHeader{
					{Name: "X-Trace", Value: "trace-id"},
				},
			},
		},
	}

	got := acpMCPServersToConfig(servers)
	if len(got) != 3 {
		t.Fatalf("expected 3 mapped configs, got %d", len(got))
	}

	if got[0].Transport != "stdio" || got[0].Command != "npx" {
		t.Fatalf("unexpected stdio mapping: %+v", got[0])
	}
	if got[0].Env["FOO"] != "bar" {
		t.Fatalf("expected stdio env FOO=bar, got %+v", got[0].Env)
	}

	if got[1].Transport != "http" || got[1].Endpoint != "https://example.com/mcp" {
		t.Fatalf("unexpected http mapping: %+v", got[1])
	}
	if got[1].Headers["Authorization"] != "Bearer token" {
		t.Fatalf("expected http header mapping, got %+v", got[1].Headers)
	}

	if got[2].Transport != "sse" || got[2].Endpoint != "https://example.com/sse" {
		t.Fatalf("unexpected sse mapping: %+v", got[2])
	}
	if got[2].Headers["X-Trace"] != "trace-id" {
		t.Fatalf("expected sse header mapping, got %+v", got[2].Headers)
	}
}

func TestACPAdapterInitialize_AdvertisesLoadSessionCapability(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.Initialize(context.Background(), acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersion(acp.ProtocolVersionNumber),
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if !resp.AgentCapabilities.LoadSession {
		t.Fatalf("expected loadSession capability to be true")
	}
}

func TestACPAdapterLoadSession_StoresMCPServerOverrides(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	loadResp, err := adapter.LoadSession(context.Background(), acp.LoadSessionRequest{
		SessionId: acp.SessionId("acp:loaded"),
		Cwd:       t.TempDir(),
		McpServers: []acp.McpServer{
			{
				Http: &acp.McpServerHttp{
					Name: "remote-http",
					Url:  "https://example.com/mcp",
					Headers: []acp.HttpHeader{
						{Name: "Authorization", Value: "Bearer token"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if loadResp.Models == nil || loadResp.Models.CurrentModelId == "" {
		t.Fatalf("expected model state in load session response, got %+v", loadResp.Models)
	}
	if loadResp.Modes == nil || loadResp.Modes.CurrentModeId != acp.SessionModeId("default") {
		t.Fatalf("expected default mode in load session response, got %+v", loadResp.Modes)
	}

	state, err := adapter.getACPSessionState("acp:loaded")
	if err != nil {
		t.Fatalf("getACPSessionState failed: %v", err)
	}
	if len(state.mcpServers) != 1 {
		t.Fatalf("expected 1 mcp server override, got %d", len(state.mcpServers))
	}
	if state.mcpServers[0].Transport != "http" {
		t.Fatalf("expected http transport, got %q", state.mcpServers[0].Transport)
	}
	if state.mcpServers[0].Headers["Authorization"] != "Bearer token" {
		t.Fatalf("expected header mapping, got %+v", state.mcpServers[0].Headers)
	}
}

func TestACPAdapterLoadSession_RejectsEmptySessionID(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	_, err := adapter.LoadSession(context.Background(), acp.LoadSessionRequest{
		SessionId:  "",
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err == nil {
		t.Fatalf("expected LoadSession error")
	}
	if !strings.Contains(err.Error(), "sessionId is required") {
		t.Fatalf("expected sessionId validation error, got %v", err)
	}
}

func TestACPAdapterLoadSession_RejectsRelativeCWD(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	_, err := adapter.LoadSession(context.Background(), acp.LoadSessionRequest{
		SessionId:  acp.SessionId("acp:loaded"),
		Cwd:        "relative/path",
		McpServers: []acp.McpServer{},
	})
	if err == nil {
		t.Fatalf("expected LoadSession error")
	}
	if !strings.Contains(err.Error(), "cwd must be an absolute path") {
		t.Fatalf("expected absolute-path validation error, got %v", err)
	}
}

func TestACPAdapterLoadSession_PromptUsesLoadedSession(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	ag.config.Agents.Defaults.Orchestrator = orchestratorLegacy
	configureOpenAIProvider(t, ag)
	adapter := NewACPAdapter(ag)

	_, err := adapter.LoadSession(context.Background(), acp.LoadSessionRequest{
		SessionId:  acp.SessionId("acp:loaded"),
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	promptResp, err := adapter.Prompt(context.Background(), acp.PromptRequest{
		SessionId: acp.SessionId("acp:loaded"),
		Prompt: []acp.ContentBlock{
			{Text: &acp.ContentBlockText{Text: "hello", Type: "text"}},
		},
	})
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
	if promptResp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("expected stop reason %q, got %q", acp.StopReasonEndTurn, promptResp.StopReason)
	}

	state, err := adapter.getACPSessionState("acp:loaded")
	if err != nil {
		t.Fatalf("getACPSessionState failed: %v", err)
	}
	messages := state.session.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("expected 2 stored messages, got %d", len(messages))
	}
	if messages[0].Role != "user" || strings.TrimSpace(messages[0].Content) != "hello" {
		t.Fatalf("unexpected user message: %+v", messages[0])
	}
	if messages[1].Role != "assistant" || strings.TrimSpace(messages[1].Content) == "" {
		t.Fatalf("unexpected assistant message: %+v", messages[1])
	}
}

func TestACPAdapterNewSession_StoresMCPServerOverrides(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd: t.TempDir(),
		McpServers: []acp.McpServer{
			{
				Sse: &acp.McpServerSse{
					Name: "remote-sse",
					Url:  "https://example.com/sse",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	if resp.Models == nil || resp.Models.CurrentModelId == "" {
		t.Fatalf("expected model state in new session response, got %+v", resp.Models)
	}

	state, err := adapter.getACPSessionState(string(resp.SessionId))
	if err != nil {
		t.Fatalf("getACPSessionState failed: %v", err)
	}
	if len(state.mcpServers) != 1 {
		t.Fatalf("expected 1 mcp server override, got %d", len(state.mcpServers))
	}
	if state.mcpServers[0].Transport != "sse" {
		t.Fatalf("expected sse transport, got %q", state.mcpServers[0].Transport)
	}
}

func TestACPAdapterNewSession_RejectsRelativeCWD(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	_, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        "relative/path",
		McpServers: []acp.McpServer{},
	})
	if err == nil {
		t.Fatalf("expected NewSession error")
	}
	if !strings.Contains(err.Error(), "cwd must be an absolute path") {
		t.Fatalf("expected absolute-path validation error, got %v", err)
	}
}

func TestACPAdapterPromptCancelled_ReturnsCancelledStopReason(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	ag.config.Agents.Defaults.Orchestrator = orchestratorLegacy
	configureOpenAIProvider(t, ag)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	promptResp, promptErr := adapter.Prompt(ctx, acp.PromptRequest{
		SessionId: resp.SessionId,
		Prompt: []acp.ContentBlock{
			{Text: &acp.ContentBlockText{Text: "hello", Type: "text"}},
		},
	})
	if promptErr != nil {
		t.Fatalf("expected prompt cancellation to map to stop reason, got error: %v", promptErr)
	}
	if promptResp.StopReason != acp.StopReasonCancelled {
		t.Fatalf("expected stop reason %q, got %q", acp.StopReasonCancelled, promptResp.StopReason)
	}
}

func TestACPAdapterPromptUnknownSession_ReturnsInvalidParams(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	_, err := adapter.Prompt(context.Background(), acp.PromptRequest{
		SessionId: acp.SessionId("acp:missing"),
		Prompt: []acp.ContentBlock{
			{Text: &acp.ContentBlockText{Text: "hello", Type: "text"}},
		},
	})
	if err == nil {
		t.Fatalf("expected prompt error")
	}
	if !strings.Contains(err.Error(), "unknown sessionId") {
		t.Fatalf("expected unknown sessionId error, got %v", err)
	}
}

func TestACPAdapterPromptStreamsAgentMessageUpdate(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	ag.config.Agents.Defaults.Orchestrator = orchestratorLegacy
	configureOpenAIProvider(t, ag)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	var updates []acp.SessionNotification
	adapter.setSessionUpdateFunc(func(ctx context.Context, notification acp.SessionNotification) error {
		updates = append(updates, notification)
		return nil
	})

	promptResp, err := adapter.Prompt(context.Background(), acp.PromptRequest{
		SessionId: resp.SessionId,
		Prompt: []acp.ContentBlock{
			{Text: &acp.ContentBlockText{Text: "hello", Type: "text"}},
		},
	})
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
	if promptResp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("expected stop reason %q, got %q", acp.StopReasonEndTurn, promptResp.StopReason)
	}

	if len(updates) != 1 {
		t.Fatalf("expected 1 session update, got %d", len(updates))
	}
	if updates[0].SessionId != resp.SessionId {
		t.Fatalf("expected session update for %q, got %q", resp.SessionId, updates[0].SessionId)
	}
	if updates[0].Update.AgentMessageChunk == nil {
		t.Fatalf("expected agent_message_chunk update")
	}
	textBlock := updates[0].Update.AgentMessageChunk.Content.Text
	if textBlock == nil || strings.TrimSpace(textBlock.Text) == "" {
		t.Fatalf("expected non-empty streamed response text")
	}
}

func TestACPAdapterPromptSessionUpdateFailure_ReturnsInternalError(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	ag.config.Agents.Defaults.Orchestrator = orchestratorLegacy
	configureOpenAIProvider(t, ag)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	adapter.setSessionUpdateFunc(func(ctx context.Context, notification acp.SessionNotification) error {
		return fmt.Errorf("send failed")
	})

	_, err = adapter.Prompt(context.Background(), acp.PromptRequest{
		SessionId: resp.SessionId,
		Prompt: []acp.ContentBlock{
			{Text: &acp.ContentBlockText{Text: "hello", Type: "text"}},
		},
	})
	if err == nil {
		t.Fatalf("expected prompt error")
	}
	if !strings.Contains(err.Error(), "send session update") {
		t.Fatalf("expected session update error, got %v", err)
	}
}

func TestACPAdapterPromptCancelledDuringSessionUpdate_ReturnsCancelledStopReason(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	ag.config.Agents.Defaults.Orchestrator = orchestratorLegacy
	configureOpenAIProvider(t, ag)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	adapter.setSessionUpdateFunc(func(ctx context.Context, notification acp.SessionNotification) error {
		return context.Canceled
	})

	promptResp, promptErr := adapter.Prompt(context.Background(), acp.PromptRequest{
		SessionId: resp.SessionId,
		Prompt: []acp.ContentBlock{
			{Text: &acp.ContentBlockText{Text: "hello", Type: "text"}},
		},
	})
	if promptErr != nil {
		t.Fatalf("expected prompt cancellation to map to stop reason, got error: %v", promptErr)
	}
	if promptResp.StopReason != acp.StopReasonCancelled {
		t.Fatalf("expected stop reason %q, got %q", acp.StopReasonCancelled, promptResp.StopReason)
	}
}

func TestACPAdapterSetAgentConnectionNil_ClearsSessionUpdateSender(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	called := false
	adapter.setSessionUpdateFunc(func(ctx context.Context, notification acp.SessionNotification) error {
		called = true
		return nil
	})
	adapter.SetAgentConnection(nil)

	err := adapter.sendSessionUpdate(context.Background(), "acp:test", acp.UpdateAgentMessageText("hello"))
	if err != nil {
		t.Fatalf("sendSessionUpdate failed: %v", err)
	}
	if called {
		t.Fatalf("expected session update sender to be cleared")
	}
}

func TestACPAdapterPromptEmptyText_ReturnsInvalidParams(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	_, err = adapter.Prompt(context.Background(), acp.PromptRequest{
		SessionId: resp.SessionId,
		Prompt: []acp.ContentBlock{
			{Text: &acp.ContentBlockText{Text: "   ", Type: "text"}},
		},
	})
	if err == nil {
		t.Fatalf("expected prompt validation error")
	}
	if !strings.Contains(err.Error(), "prompt text is empty") {
		t.Fatalf("expected empty prompt validation error, got %v", err)
	}
}

func TestACPAdapterCancelClearsCancelFunc(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	called := false
	err = adapter.setSessionCancel(string(resp.SessionId), func() {
		called = true
	})
	if err != nil {
		t.Fatalf("setSessionCancel failed: %v", err)
	}

	if err := adapter.Cancel(context.Background(), acp.CancelNotification{SessionId: resp.SessionId}); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	if !called {
		t.Fatalf("expected cancel func to be invoked")
	}

	state, err := adapter.getACPSessionState(string(resp.SessionId))
	if err != nil {
		t.Fatalf("getACPSessionState failed: %v", err)
	}
	if state.cancel != nil {
		t.Fatalf("expected cancel func to be cleared")
	}
}

func TestACPAdapterSetSessionModelUpdatesState(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	_, err = adapter.SetSessionModel(context.Background(), acp.SetSessionModelRequest{
		SessionId: resp.SessionId,
		ModelId:   acp.ModelId("gpt-4o-mini"),
	})
	if err != nil {
		t.Fatalf("SetSessionModel failed: %v", err)
	}

	state, err := adapter.getACPSessionState(string(resp.SessionId))
	if err != nil {
		t.Fatalf("getACPSessionState failed: %v", err)
	}
	if state.model != "gpt-4o-mini" {
		t.Fatalf("expected model gpt-4o-mini, got %q", state.model)
	}
}

func TestACPAdapterSetSessionModelUnknownSession_ReturnsInvalidParams(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	_, err := adapter.SetSessionModel(context.Background(), acp.SetSessionModelRequest{
		SessionId: acp.SessionId("acp:missing"),
		ModelId:   acp.ModelId("gpt-4o-mini"),
	})
	if err == nil {
		t.Fatalf("expected set session model error")
	}
	if !strings.Contains(err.Error(), "unknown sessionId") {
		t.Fatalf("expected unknown sessionId error, got %v", err)
	}
}

func TestACPAdapterSetSessionModelEmptyModel_ReturnsInvalidParams(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	_, err = adapter.SetSessionModel(context.Background(), acp.SetSessionModelRequest{
		SessionId: resp.SessionId,
		ModelId:   "",
	})
	if err == nil {
		t.Fatalf("expected set session model validation error")
	}
	if !strings.Contains(err.Error(), "modelId is required") {
		t.Fatalf("expected modelId validation error, got %v", err)
	}
}

func TestACPAdapterSetSessionModeUpdatesState(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	_, err = adapter.SetSessionMode(context.Background(), acp.SetSessionModeRequest{
		SessionId: resp.SessionId,
		ModeId:    acp.SessionModeId("review"),
	})
	if err != nil {
		t.Fatalf("SetSessionMode failed: %v", err)
	}

	state, err := adapter.getACPSessionState(string(resp.SessionId))
	if err != nil {
		t.Fatalf("getACPSessionState failed: %v", err)
	}
	if state.modeID != "review" {
		t.Fatalf("expected modeID review, got %q", state.modeID)
	}
}

func TestACPAdapterSetSessionModeSendsCurrentModeUpdate(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	var updates []acp.SessionNotification
	adapter.setSessionUpdateFunc(func(ctx context.Context, notification acp.SessionNotification) error {
		updates = append(updates, notification)
		return nil
	})

	_, err = adapter.SetSessionMode(context.Background(), acp.SetSessionModeRequest{
		SessionId: resp.SessionId,
		ModeId:    acp.SessionModeId("review"),
	})
	if err != nil {
		t.Fatalf("SetSessionMode failed: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 session update, got %d", len(updates))
	}
	if updates[0].Update.CurrentModeUpdate == nil {
		t.Fatalf("expected current_mode_update notification")
	}
	if updates[0].Update.CurrentModeUpdate.CurrentModeId != acp.SessionModeId("review") {
		t.Fatalf("expected current mode review, got %q", updates[0].Update.CurrentModeUpdate.CurrentModeId)
	}
}

func TestACPAdapterSetSessionModeUpdateFailure_ReturnsInternalError(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	adapter.setSessionUpdateFunc(func(ctx context.Context, notification acp.SessionNotification) error {
		return fmt.Errorf("send failed")
	})

	_, err = adapter.SetSessionMode(context.Background(), acp.SetSessionModeRequest{
		SessionId: resp.SessionId,
		ModeId:    acp.SessionModeId("review"),
	})
	if err == nil {
		t.Fatalf("expected set session mode error")
	}
	if !strings.Contains(err.Error(), "send session update") {
		t.Fatalf("expected session update error, got %v", err)
	}
}

func TestACPAdapterSetSessionModeCancelledDuringUpdate_ReturnsNil(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	resp, err := adapter.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        t.TempDir(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	adapter.setSessionUpdateFunc(func(ctx context.Context, notification acp.SessionNotification) error {
		return context.Canceled
	})

	_, err = adapter.SetSessionMode(context.Background(), acp.SetSessionModeRequest{
		SessionId: resp.SessionId,
		ModeId:    acp.SessionModeId("review"),
	})
	if err != nil {
		t.Fatalf("expected cancellation to return nil, got %v", err)
	}
}

func TestACPAdapterSetSessionModeUnknownSession_ReturnsInvalidParams(t *testing.T) {
	ag := newACPSessionTestAgent(t)
	adapter := NewACPAdapter(ag)

	_, err := adapter.SetSessionMode(context.Background(), acp.SetSessionModeRequest{
		SessionId: acp.SessionId("acp:missing"),
		ModeId:    acp.SessionModeId("default"),
	})
	if err == nil {
		t.Fatalf("expected set session mode error")
	}
	if !strings.Contains(err.Error(), "unknown sessionId") {
		t.Fatalf("expected unknown sessionId error, got %v", err)
	}
}

func newACPSessionTestAgent(t *testing.T) *Agent {
	t.Helper()
	ag := newRoutingTestAgent(t, orchestratorBlades)
	ag.acpSessions = make(map[string]*acpSessionState)
	return ag
}

func configureOpenAIProvider(t *testing.T, ag *Agent) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-4o-mini",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "hello from adapter test",
					},
					"finish_reason": "stop",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(server.Close)

	ag.config.Agents.Defaults.Provider = "openai"
	ag.config.Agents.Defaults.Model = "gpt-4o-mini"
	ag.config.Providers = []config.ProviderProfile{{
		Name:         "openai",
		ProviderKind: "openai",
		APIKey:       "test-key",
		APIBase:      server.URL,
	}}
}
