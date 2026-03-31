package inboundrouter

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"nekobot/pkg/accountbindings"
	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/session"
	"nekobot/pkg/storage/ent"
)

type stubAgent struct {
	response   string
	lastPrompt agent.PromptContext
	lastInput  string
}

func (s *stubAgent) ChatWithPromptContextDetailed(
	ctx context.Context,
	sess agent.SessionInterface,
	userMessage string,
	promptCtx agent.PromptContext,
) (string, agent.ChatRouteResult, error) {
	s.lastPrompt = promptCtx
	s.lastInput = userMessage
	return s.response, agent.ChatRouteResult{
		RequestedProvider: promptCtx.RequestedProvider,
		RequestedModel:    promptCtx.RequestedModel,
	}, nil
}

func TestHandleInboundRoutesSingleAgentBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log, err := logger.New(&logger.Config{Level: "error", OutputPath: ""})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}
	runtimeMgr, err := runtimeagents.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new runtime manager: %v", err)
	}
	bindingMgr, err := accountbindings.NewManager(cfg, log, client, runtimeMgr, accountMgr)
	if err != nil {
		t.Fatalf("new binding manager: %v", err)
	}

	accountItem, err := accountMgr.Create(context.Background(), channelaccounts.ChannelAccount{
		ChannelType: "slack",
		AccountKey:  "team-a",
		DisplayName: "Team A",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	runtimeItem, err := runtimeMgr.Create(context.Background(), runtimeagents.AgentRuntime{
		Name:        "support-main",
		DisplayName: "Support Main",
		Enabled:     true,
		Provider:    "openai",
		Model:       "gpt-5",
		PromptID:    "prompt-runtime-1",
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	if _, err := bindingMgr.Create(context.Background(), accountbindings.AccountBinding{
		ChannelAccountID: accountItem.ID,
		AgentRuntimeID:   runtimeItem.ID,
		BindingMode:      accountbindings.ModeSingleAgent,
		Enabled:          true,
		Priority:         100,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	messageBus := bus.NewLocalBus(log, 8)
	if err := messageBus.Start(); err != nil {
		t.Fatalf("start bus: %v", err)
	}
	t.Cleanup(func() {
		if err := messageBus.Stop(); err != nil {
			t.Fatalf("stop bus: %v", err)
		}
	})

	replyCh := make(chan *bus.Message, 1)
	messageBus.RegisterOutboundHandler("slack:team-a", func(ctx context.Context, msg *bus.Message) error {
		replyCh <- msg
		return nil
	})

	agentStub := &stubAgent{response: "reply from runtime"}
	router, err := New(
		log,
		messageBus,
		agentStub,
		session.NewManager(t.TempDir(), cfg.Sessions),
		accountMgr,
		bindingMgr,
		runtimeMgr,
	)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	if err := router.HandleInbound(context.Background(), &bus.Message{
		ChannelID: "slack:team-a",
		SessionID: "thread-1",
		UserID:    "u-1",
		Username:  "alice",
		Type:      bus.MessageTypeText,
		Content:   "hello",
	}); err != nil {
		t.Fatalf("handle inbound: %v", err)
	}

	select {
	case reply := <-replyCh:
		if reply.Content != "reply from runtime" {
			t.Fatalf("unexpected reply: %q", reply.Content)
		}
		if got := agentStub.lastPrompt.RequestedProvider; got != "openai" {
			t.Fatalf("unexpected provider: %q", got)
		}
		if got := agentStub.lastPrompt.RequestedModel; got != "gpt-5" {
			t.Fatalf("unexpected model: %q", got)
		}
		if got := agentStub.lastPrompt.ExplicitPromptIDs; len(got) != 1 || got[0] != "prompt-runtime-1" {
			t.Fatalf("unexpected explicit prompt ids: %+v", got)
		}
		if got := agentStub.lastPrompt.SessionID; got != "route:"+runtimeItem.ID+":thread-1" {
			t.Fatalf("unexpected routed session: %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected outbound reply")
	}
}

func TestChatWebsocketFallsBackWithoutTopologyBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log, err := logger.New(&logger.Config{Level: "error", OutputPath: ""})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}
	runtimeMgr, err := runtimeagents.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new runtime manager: %v", err)
	}
	bindingMgr, err := accountbindings.NewManager(cfg, log, client, runtimeMgr, accountMgr)
	if err != nil {
		t.Fatalf("new binding manager: %v", err)
	}

	agentStub := &stubAgent{response: "gateway default"}
	router, err := New(
		log,
		bus.NewLocalBus(log, 4),
		agentStub,
		session.NewManager(t.TempDir(), cfg.Sessions),
		accountMgr,
		bindingMgr,
		runtimeMgr,
	)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	reply, metadata, err := router.ChatWebsocket(context.Background(), "u-1", "alice", "gateway-session", "hello")
	if err != nil {
		t.Fatalf("chat websocket: %v", err)
	}
	if reply != "gateway default" {
		t.Fatalf("unexpected websocket reply: %q", reply)
	}
	if metadata != nil {
		raw, _ := json.Marshal(metadata)
		t.Fatalf("expected nil metadata, got %s", raw)
	}
	if agentStub.lastPrompt.SessionID != "gateway-session" {
		t.Fatalf("unexpected session id: %q", agentStub.lastPrompt.SessionID)
	}
}

func TestHandleInboundFallsBackForLegacyChannelWithoutTopology(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log, err := logger.New(&logger.Config{Level: "error", OutputPath: ""})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	client := newTestEntClient(t, cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close ent client: %v", err)
		}
	})

	accountMgr, err := channelaccounts.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new account manager: %v", err)
	}
	runtimeMgr, err := runtimeagents.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new runtime manager: %v", err)
	}
	bindingMgr, err := accountbindings.NewManager(cfg, log, client, runtimeMgr, accountMgr)
	if err != nil {
		t.Fatalf("new binding manager: %v", err)
	}

	messageBus := bus.NewLocalBus(log, 8)
	if err := messageBus.Start(); err != nil {
		t.Fatalf("start bus: %v", err)
	}
	t.Cleanup(func() {
		if err := messageBus.Stop(); err != nil {
			t.Fatalf("stop bus: %v", err)
		}
	})

	replyCh := make(chan *bus.Message, 1)
	messageBus.RegisterOutboundHandler("telegram", func(ctx context.Context, msg *bus.Message) error {
		replyCh <- msg
		return nil
	})

	agentStub := &stubAgent{response: "legacy reply"}
	router, err := New(
		log,
		messageBus,
		agentStub,
		session.NewManager(t.TempDir(), cfg.Sessions),
		accountMgr,
		bindingMgr,
		runtimeMgr,
	)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	err = router.HandleInbound(context.Background(), &bus.Message{
		ChannelID: "telegram",
		SessionID: "telegram:123",
		UserID:    "u-1",
		Username:  "alice",
		Type:      bus.MessageTypeText,
		Content:   "hello",
		Data: map[string]interface{}{
			"reply_to_message_id": 7,
		},
	})
	if err != nil {
		t.Fatalf("handle inbound: %v", err)
	}

	select {
	case reply := <-replyCh:
		if reply.Content != "legacy reply" {
			t.Fatalf("unexpected reply: %q", reply.Content)
		}
		if got, ok := reply.Data["reply_to_message_id"].(int); !ok || got != 7 {
			t.Fatalf("expected reply_to_message_id=7, got %#v", reply.Data["reply_to_message_id"])
		}
		if agentStub.lastPrompt.SessionID != "telegram:123" {
			t.Fatalf("unexpected legacy session id: %q", agentStub.lastPrompt.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected outbound reply")
	}
}

func newTestEntClient(t *testing.T, cfg *config.Config) *ent.Client {
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
