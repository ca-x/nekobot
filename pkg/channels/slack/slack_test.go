package slack

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

type stubSlackAPI struct {
	postMessageChannel string
	postMessageOpts    []slackapi.MsgOption
	postMessageTS      string
	postMessageErr     error

	openViewTriggerID string
	openViewRequest   *slackapi.ModalViewRequest
	openViewErr       error

	updateMessageChannel string
	updateMessageTS      string
	updateMessageOpts    []slackapi.MsgOption
	updateMessageErr     error

	ephemeralChannel string
	ephemeralUser    string
	ephemeralOpts    []slackapi.MsgOption
	ephemeralErr     error
}

func (s *stubSlackAPI) AuthTest() (*slackapi.AuthTestResponse, error) {
	return &slackapi.AuthTestResponse{}, nil
}

func (s *stubSlackAPI) PostEphemeral(channelID, userID string, options ...slackapi.MsgOption) (string, error) {
	s.ephemeralChannel = channelID
	s.ephemeralUser = userID
	s.ephemeralOpts = append([]slackapi.MsgOption(nil), options...)
	return "", s.ephemeralErr
}

func (s *stubSlackAPI) PostMessage(channelID string, options ...slackapi.MsgOption) (string, string, error) {
	s.postMessageChannel = channelID
	s.postMessageOpts = append([]slackapi.MsgOption(nil), options...)
	if s.postMessageTS == "" {
		s.postMessageTS = "1710000000.000100"
	}
	return channelID, s.postMessageTS, s.postMessageErr
}

func (s *stubSlackAPI) PostMessageContext(ctx context.Context, channelID string, options ...slackapi.MsgOption) (string, string, error) {
	return s.PostMessage(channelID, options...)
}

func (s *stubSlackAPI) OpenView(triggerID string, view slackapi.ModalViewRequest) (*slackapi.ViewResponse, error) {
	s.openViewTriggerID = triggerID
	viewCopy := view
	s.openViewRequest = &viewCopy
	if s.openViewErr != nil {
		return nil, s.openViewErr
	}
	return &slackapi.ViewResponse{}, nil
}

func (s *stubSlackAPI) UpdateMessage(channelID, timestamp string, options ...slackapi.MsgOption) (string, string, string, error) {
	s.updateMessageChannel = channelID
	s.updateMessageTS = timestamp
	s.updateMessageOpts = append([]slackapi.MsgOption(nil), options...)
	return channelID, timestamp, "", s.updateMessageErr
}

type stubBus struct {
	inbound []*bus.Message
}

func (b *stubBus) Start() error                                                  { return nil }
func (b *stubBus) Stop() error                                                   { return nil }
func (b *stubBus) RegisterInboundHandler(channelID string, handler bus.Handler)  {}
func (b *stubBus) UnregisterInboundHandlers(channelID string)                    {}
func (b *stubBus) RegisterOutboundHandler(channelID string, handler bus.Handler) {}
func (b *stubBus) UnregisterOutboundHandlers(channelID string)                   {}
func (b *stubBus) RegisterHandler(channelID string, handler bus.Handler)         {}
func (b *stubBus) UnregisterHandlers(channelID string)                           {}
func (b *stubBus) SendInbound(msg *bus.Message) error {
	b.inbound = append(b.inbound, msg)
	return nil
}
func (b *stubBus) SendOutbound(msg *bus.Message) error { return nil }
func (b *stubBus) GetMetrics() map[string]uint64       { return map[string]uint64{} }

func newTestChannel(t *testing.T) *Channel {
	t.Helper()

	log, err := logger.New(&logger.Config{
		Level:       logger.LevelDebug,
		Development: true,
	})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	return &Channel{
		log:                  log,
		config:               config.SlackConfig{},
		bus:                  &stubBus{},
		commands:             commands.NewRegistry(),
		id:                   "slack",
		channelType:          "slack",
		name:                 "Slack",
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}
}

func TestSendSkillInstallConfirmationStoresPendingState(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	resp := commands.CommandResponse{
		Content: "candidate found",
		Interaction: &commands.CommandInteraction{
			Type:    commands.InteractionTypeSkillInstallConfirm,
			Repo:    "owner/repo",
			Reason:  "best match",
			Message: "Please confirm install.",
			Command: "find-skills",
		},
	}

	cmd := slackapi.SlashCommand{
		ChannelID:   "C123",
		UserID:      "U123",
		ResponseURL: "https://example.invalid/response",
	}

	if err := ch.sendSkillInstallConfirmation(cmd, "find-skills", resp); err != nil {
		t.Fatalf("send skill install confirmation: %v", err)
	}

	pending, ok := ch.getPendingSkillInstall(api.postMessageTS)
	if !ok {
		t.Fatal("expected pending skill install to be stored")
	}
	if pending.UserID != "U123" || pending.ChannelID != "C123" || pending.Repo != "owner/repo" {
		t.Fatalf("unexpected pending state: %+v", pending)
	}
	if pending.Command != "find-skills" {
		t.Fatalf("unexpected command: %q", pending.Command)
	}
}

func TestHandleSkillInstallConfirmExecutesCommandAndUpdatesMessage(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	if err := ch.commands.Register(&commands.Command{
		Name: "find-skills",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if got := strings.TrimSpace(req.Args); got != "__confirm_install__ owner/repo" {
				t.Fatalf("unexpected args: %q", got)
			}
			if got := req.Metadata["skill_install_confirmed_repo"]; got != "owner/repo" {
				t.Fatalf("unexpected repo metadata: %q", got)
			}
			return commands.CommandResponse{Content: "installed"}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	ch.setPendingSkillInstall("1710000000.000100", pendingSkillInstall{
		UserID:    "U123",
		ChannelID: "C123",
		MessageTS: "1710000000.000100",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now(),
	})

	callback := slackapi.InteractionCallback{
		User: slackapi.User{ID: "U123", Name: "alice"},
		Channel: slackapi.Channel{
			GroupConversation: slackapi.GroupConversation{
				Conversation: slackapi.Conversation{ID: "C123"},
			},
		},
		Message:   slackapi.Message{Msg: slackapi.Msg{Timestamp: "1710000000.000100"}},
		Team:      slackapi.Team{ID: "T123"},
		TriggerID: "trigger-1",
	}

	ch.handleSkillInstallConfirm(callback, "owner/repo")

	if _, ok := ch.getPendingSkillInstall("1710000000.000100"); ok {
		t.Fatal("expected pending skill install to be cleared")
	}
	if api.updateMessageChannel != "C123" || api.updateMessageTS != "1710000000.000100" {
		t.Fatalf("unexpected update target: channel=%q ts=%q", api.updateMessageChannel, api.updateMessageTS)
	}
}

func TestHandleSkillInstallCancelUpdatesMessage(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	ch.setPendingSkillInstall("1710000000.000100", pendingSkillInstall{
		UserID:    "U123",
		ChannelID: "C123",
		MessageTS: "1710000000.000100",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now(),
	})

	callback := slackapi.InteractionCallback{
		User: slackapi.User{ID: "U123", Name: "alice"},
		Channel: slackapi.Channel{
			GroupConversation: slackapi.GroupConversation{
				Conversation: slackapi.Conversation{ID: "C123"},
			},
		},
		Message: slackapi.Message{Msg: slackapi.Msg{Timestamp: "1710000000.000100"}},
	}

	ch.handleSkillInstallCancel(callback)

	if _, ok := ch.getPendingSkillInstall("1710000000.000100"); ok {
		t.Fatal("expected pending skill install to be cleared")
	}
	if api.updateMessageTS != "1710000000.000100" {
		t.Fatalf("expected update on original message, got %q", api.updateMessageTS)
	}
}

func TestPendingSkillInstallExpires(t *testing.T) {
	ch := newTestChannel(t)
	ch.setPendingSkillInstall("1710000000.000100", pendingSkillInstall{
		UserID:    "U123",
		ChannelID: "C123",
		MessageTS: "1710000000.000100",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now().Add(-16 * time.Minute),
	})

	if _, ok := ch.getPendingSkillInstall("1710000000.000100"); ok {
		t.Fatal("expected expired pending interaction to be evicted")
	}
}

func TestAccountChannelUsesRuntimeScopedIdentifiers(t *testing.T) {
	log, err := logger.New(&logger.Config{
		Level:       logger.LevelDebug,
		Development: true,
	})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	channel, err := NewAccountChannel(
		log,
		config.SlackConfig{Enabled: true, BotToken: "xoxb-test", AppToken: "xapp-test"},
		&stubBus{},
		commands.NewRegistry(),
		nil,
		"slack:team-a",
		"Slack Team A",
	)
	if err != nil {
		t.Fatalf("NewAccountChannel failed: %v", err)
	}

	if channel.ID() != "slack:team-a" {
		t.Fatalf("unexpected channel id: %s", channel.ID())
	}
	if channel.ChannelType() != "slack" {
		t.Fatalf("unexpected channel type: %s", channel.ChannelType())
	}
	if channel.Name() != "Slack Team A" {
		t.Fatalf("unexpected channel name: %s", channel.Name())
	}

	sessionID := channel.sessionThreadID("C123", "1710000000.000100")
	if sessionID != "slack:team-a:C123:1710000000.000100" {
		t.Fatalf("unexpected session id: %s", sessionID)
	}

	channelID, threadTS := channel.parseSessionID(sessionID)
	if channelID != "C123" || threadTS != "1710000000.000100" {
		t.Fatalf("unexpected parsed session: channel=%q thread=%q", channelID, threadTS)
	}
}

func TestParseSessionIDSupportsAccountRuntimePrefix(t *testing.T) {
	ch := newTestChannel(t)
	ch.id = "slack:workspace-a"

	channelID, threadTS := ch.parseSessionID("slack:workspace-a:C123:1710000000.000100")
	if channelID != "C123" || threadTS != "1710000000.000100" {
		t.Fatalf("unexpected parsed session: channel=%q thread=%q", channelID, threadTS)
	}

	channelID, threadTS = ch.parseSessionID("slack:C123")
	if channelID != "C123" || threadTS != "" {
		t.Fatalf("expected legacy prefix compatibility, got channel=%q thread=%q", channelID, threadTS)
	}
}

func TestHandleMessageEventUsesAccountRuntimeIdentifiers(t *testing.T) {
	ch := newTestChannel(t)
	ch.id = "slack:workspace-a"
	b := &stubBus{}
	ch.bus = b

	ch.handleMessageEvent((&slackapieventsMessageEvent{
		Channel:         "C123",
		ThreadTimeStamp: "1710000000.000100",
		User:            "U123",
		Text:            "hello",
		TimeStamp:       "1710000001.000200",
	}).toSlack())

	if len(b.inbound) != 1 {
		t.Fatalf("expected one inbound message, got %d", len(b.inbound))
	}
	if b.inbound[0].ChannelID != "slack:workspace-a" {
		t.Fatalf("unexpected inbound channel id: %q", b.inbound[0].ChannelID)
	}
	if b.inbound[0].SessionID != "slack:workspace-a:C123:1710000000.000100" {
		t.Fatalf("unexpected inbound session id: %q", b.inbound[0].SessionID)
	}
}

func TestExecuteConfirmedSkillInstallUsesRuntimeChannelID(t *testing.T) {
	ch := newTestChannel(t)
	ch.id = "slack:workspace-a"
	api := &stubSlackAPI{}
	ch.api = api

	if err := ch.commands.Register(&commands.Command{
		Name: "find-skills",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if req.Channel != "slack:workspace-a" {
				t.Fatalf("unexpected command channel: %q", req.Channel)
			}
			return commands.CommandResponse{Content: "installed"}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	result := ch.executeConfirmedSkillInstall(slackapi.InteractionCallback{
		User: slackapi.User{ID: "U123", Name: "alice"},
		Team: slackapi.Team{ID: "T123"},
	}, pendingSkillInstall{
		UserID:    "U123",
		ChannelID: "C123",
		MessageTS: "1710000000.000100",
		Command:   "find-skills",
		Repo:      "owner/repo",
		CreatedAt: time.Now(),
	})
	if result != "installed" {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestHandleShortcutOpensFindSkillsModal(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	callback := slackapi.InteractionCallback{
		Type:       slackapi.InteractionTypeShortcut,
		CallbackID: "find_skills",
		TriggerID:  "trigger-123",
		User:       slackapi.User{ID: "U123", Name: "alice"},
	}

	ch.handleShortcut(callback)

	if api.openViewTriggerID != "trigger-123" {
		t.Fatalf("expected modal to open with trigger trigger-123, got %q", api.openViewTriggerID)
	}
	if api.openViewRequest == nil {
		t.Fatal("expected modal view request")
	}
	if api.openViewRequest.CallbackID != "find_skills_modal" {
		t.Fatalf("unexpected modal callback id: %q", api.openViewRequest.CallbackID)
	}
	if api.openViewRequest.PrivateMetadata != "find-skills" {
		t.Fatalf("unexpected private metadata: %q", api.openViewRequest.PrivateMetadata)
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	ch := newTestChannel(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.postMessage" {
			t.Fatalf("unexpected slack api path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}
		text := values.Get("text")
		if !strings.Contains(text, "Tool call: read_file") {
			t.Fatalf("expected tool trace in slack text, got %q", text)
		}
		if !strings.Contains(text, "\n\ndone") {
			t.Fatalf("expected original reply after blank line, got %q", text)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1710000000.000100","message":{"text":"ok"}}`))
	}))
	defer server.Close()

	ch.api = slackapi.New("test-token", slackapi.OptionAPIURL(server.URL+"/"))

	err := ch.SendMessage(context.Background(), &bus.Message{
		SessionID: "slack:C123",
		Content:   "done",
		Data: map[string]interface{}{
			"tool_call_trace": "Tool call: read_file {\"path\":\"README.md\"}",
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
}

func TestHandleViewSubmissionExecutesFindSkillsCommand(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	if err := ch.commands.Register(&commands.Command{
		Name: "find-skills",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if req.Channel != "slack" {
				t.Fatalf("unexpected channel: %q", req.Channel)
			}
			if req.Command != "find-skills" {
				t.Fatalf("unexpected command: %q", req.Command)
			}
			if req.Args != "search qmd memory" {
				t.Fatalf("unexpected args: %q", req.Args)
			}
			if req.UserID != "U123" {
				t.Fatalf("unexpected user: %q", req.UserID)
			}
			if req.Metadata["team_id"] != "T123" {
				t.Fatalf("unexpected team id: %q", req.Metadata["team_id"])
			}
			if req.Metadata["runtime_id"] != "slack" {
				t.Fatalf("unexpected runtime id: %q", req.Metadata["runtime_id"])
			}
			return commands.CommandResponse{
				Content: "candidate found",
				Interaction: &commands.CommandInteraction{
					Type:    commands.InteractionTypeSkillInstallConfirm,
					Repo:    "owner/repo",
					Reason:  "best match",
					Message: "Please confirm install.",
					Command: "find-skills",
				},
			}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	callback := slackapi.InteractionCallback{
		Type: slackapi.InteractionTypeViewSubmission,
		User: slackapi.User{ID: "U123", Name: "alice"},
		Team: slackapi.Team{ID: "T123"},
		View: slackapi.View{
			CallbackID:      "find_skills_modal",
			PrivateMetadata: "find-skills",
			State: &slackapi.ViewState{
				Values: map[string]map[string]slackapi.BlockAction{
					"skill_query": {
						"query_input": {
							Value: "search qmd memory",
						},
					},
				},
			},
		},
	}

	ch.handleViewSubmission(callback)

	pending, ok := ch.getPendingSkillInstall(api.postMessageTS)
	if !ok {
		t.Fatal("expected pending skill install to be stored")
	}
	if pending.UserID != "U123" || pending.Command != "find-skills" || pending.Repo != "owner/repo" {
		t.Fatalf("unexpected pending state: %+v", pending)
	}
}

func TestHandleShortcutOpensSettingsModal(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	callback := slackapi.InteractionCallback{
		Type:       slackapi.InteractionTypeShortcut,
		CallbackID: "settings",
		TriggerID:  "trigger-settings",
		User:       slackapi.User{ID: "U123", Name: "alice"},
	}

	ch.handleShortcut(callback)

	if api.openViewTriggerID != "trigger-settings" {
		t.Fatalf("expected settings modal to open with trigger trigger-settings, got %q", api.openViewTriggerID)
	}
	if api.openViewRequest == nil {
		t.Fatal("expected modal view request")
	}
	if api.openViewRequest.CallbackID != "settings_modal" {
		t.Fatalf("unexpected settings modal callback id: %q", api.openViewRequest.CallbackID)
	}
	if api.openViewRequest.PrivateMetadata != "settings" {
		t.Fatalf("unexpected settings modal private metadata: %q", api.openViewRequest.PrivateMetadata)
	}
}

func TestHandleShortcutOpensModelModal(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	callback := slackapi.InteractionCallback{
		Type:       slackapi.InteractionTypeShortcut,
		CallbackID: "model",
		TriggerID:  "trigger-model",
		User:       slackapi.User{ID: "U123", Name: "alice"},
	}

	ch.handleShortcut(callback)

	if api.openViewTriggerID != "trigger-model" {
		t.Fatalf("expected model modal to open with trigger trigger-model, got %q", api.openViewTriggerID)
	}
	if api.openViewRequest == nil {
		t.Fatal("expected modal view request")
	}
	if api.openViewRequest.CallbackID != "model_modal" {
		t.Fatalf("unexpected model modal callback id: %q", api.openViewRequest.CallbackID)
	}
	if api.openViewRequest.PrivateMetadata != "model" {
		t.Fatalf("unexpected model modal private metadata: %q", api.openViewRequest.PrivateMetadata)
	}
}

func TestHandleShortcutOpensHelpModal(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	callback := slackapi.InteractionCallback{
		Type:       slackapi.InteractionTypeShortcut,
		CallbackID: "help",
		TriggerID:  "trigger-help",
		User:       slackapi.User{ID: "U123", Name: "alice"},
	}

	ch.handleShortcut(callback)

	if api.openViewTriggerID != "trigger-help" {
		t.Fatalf("expected help modal to open with trigger trigger-help, got %q", api.openViewTriggerID)
	}
	if api.openViewRequest == nil {
		t.Fatal("expected modal view request")
	}
	if api.openViewRequest.CallbackID != "help_modal" {
		t.Fatalf("unexpected help modal callback id: %q", api.openViewRequest.CallbackID)
	}
	if api.openViewRequest.PrivateMetadata != "help" {
		t.Fatalf("unexpected help modal private metadata: %q", api.openViewRequest.PrivateMetadata)
	}
}

func TestHandleShortcutOpensStatusModal(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	callback := slackapi.InteractionCallback{
		Type:       slackapi.InteractionTypeShortcut,
		CallbackID: "status",
		TriggerID:  "trigger-status",
		User:       slackapi.User{ID: "U123", Name: "alice"},
	}

	ch.handleShortcut(callback)

	if api.openViewTriggerID != "trigger-status" {
		t.Fatalf("expected status modal to open with trigger trigger-status, got %q", api.openViewTriggerID)
	}
	if api.openViewRequest == nil {
		t.Fatal("expected modal view request")
	}
	if api.openViewRequest.CallbackID != "status_modal" {
		t.Fatalf("unexpected status modal callback id: %q", api.openViewRequest.CallbackID)
	}
	if api.openViewRequest.PrivateMetadata != "status" {
		t.Fatalf("unexpected status modal private metadata: %q", api.openViewRequest.PrivateMetadata)
	}
}

func TestHandleShortcutOpensAgentModal(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	callback := slackapi.InteractionCallback{
		Type:       slackapi.InteractionTypeShortcut,
		CallbackID: "agent",
		TriggerID:  "trigger-agent",
		User:       slackapi.User{ID: "U123", Name: "alice"},
	}

	ch.handleShortcut(callback)

	if api.openViewTriggerID != "trigger-agent" {
		t.Fatalf("expected agent modal to open with trigger trigger-agent, got %q", api.openViewTriggerID)
	}
	if api.openViewRequest == nil {
		t.Fatal("expected modal view request")
	}
	if api.openViewRequest.CallbackID != "agent_modal" {
		t.Fatalf("unexpected agent modal callback id: %q", api.openViewRequest.CallbackID)
	}
	if api.openViewRequest.PrivateMetadata != "agent" {
		t.Fatalf("unexpected agent modal private metadata: %q", api.openViewRequest.PrivateMetadata)
	}
}

func TestHandleShortcutOpensStartModal(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	callback := slackapi.InteractionCallback{
		Type:       slackapi.InteractionTypeShortcut,
		CallbackID: "start",
		TriggerID:  "trigger-start",
		User:       slackapi.User{ID: "U123", Name: "alice"},
	}

	ch.handleShortcut(callback)

	if api.openViewTriggerID != "trigger-start" {
		t.Fatalf("expected start modal to open with trigger trigger-start, got %q", api.openViewTriggerID)
	}
	if api.openViewRequest == nil {
		t.Fatal("expected modal view request")
	}
	if api.openViewRequest.CallbackID != "start_modal" {
		t.Fatalf("unexpected start modal callback id: %q", api.openViewRequest.CallbackID)
	}
	if api.openViewRequest.PrivateMetadata != "start" {
		t.Fatalf("unexpected start modal private metadata: %q", api.openViewRequest.PrivateMetadata)
	}
}

func TestHandleViewSubmissionExecutesSettingsCommand(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	if err := ch.commands.Register(&commands.Command{
		Name: "settings",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if req.Channel != "slack" {
				t.Fatalf("unexpected channel: %q", req.Channel)
			}
			if req.Command != "settings" {
				t.Fatalf("unexpected command: %q", req.Command)
			}
			if req.Args != "lang en" {
				t.Fatalf("unexpected args: %q", req.Args)
			}
			if req.UserID != "U123" {
				t.Fatalf("unexpected user: %q", req.UserID)
			}
			if req.Metadata["team_id"] != "T123" {
				t.Fatalf("unexpected team id: %q", req.Metadata["team_id"])
			}
			if req.Metadata["runtime_id"] != "slack" {
				t.Fatalf("unexpected runtime id: %q", req.Metadata["runtime_id"])
			}
			return commands.CommandResponse{
				Content: "✅ 语言已更新为: en",
			}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	callback := slackapi.InteractionCallback{
		Type: slackapi.InteractionTypeViewSubmission,
		User: slackapi.User{ID: "U123", Name: "alice"},
		Team: slackapi.Team{ID: "T123"},
		Channel: slackapi.Channel{
			GroupConversation: slackapi.GroupConversation{
				Conversation: slackapi.Conversation{ID: "C123"},
			},
		},
		View: slackapi.View{
			CallbackID:      "settings_modal",
			PrivateMetadata: "settings",
			State: &slackapi.ViewState{
				Values: map[string]map[string]slackapi.BlockAction{
					"settings_action": {
						"settings_action_input": {
							Value: "lang",
						},
					},
					"settings_value": {
						"settings_value_input": {
							Value: "en",
						},
					},
				},
			},
		},
	}

	ch.handleViewSubmission(callback)

	if api.ephemeralChannel != "C123" {
		t.Fatalf("unexpected ephemeral target channel: %q", api.ephemeralChannel)
	}
	if api.ephemeralUser != "U123" {
		t.Fatalf("expected ephemeral response for U123, got %q", api.ephemeralUser)
	}
	if len(api.ephemeralOpts) == 0 {
		t.Fatal("expected ephemeral response options to be sent")
	}
}

func TestHandleViewSubmissionExecutesModelCommand(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	if err := ch.commands.Register(&commands.Command{
		Name: "model",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if req.Channel != "slack" {
				t.Fatalf("unexpected channel: %q", req.Channel)
			}
			if req.Command != "model" {
				t.Fatalf("unexpected command: %q", req.Command)
			}
			if req.Args != "openai" {
				t.Fatalf("unexpected args: %q", req.Args)
			}
			if req.UserID != "U123" {
				t.Fatalf("unexpected user: %q", req.UserID)
			}
			if req.Metadata["team_id"] != "T123" {
				t.Fatalf("unexpected team id: %q", req.Metadata["team_id"])
			}
			if req.Metadata["runtime_id"] != "slack" {
				t.Fatalf("unexpected runtime id: %q", req.Metadata["runtime_id"])
			}
			return commands.CommandResponse{
				Content: "🤖 **Provider: openai**",
			}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	callback := slackapi.InteractionCallback{
		Type: slackapi.InteractionTypeViewSubmission,
		User: slackapi.User{ID: "U123", Name: "alice"},
		Team: slackapi.Team{ID: "T123"},
		Channel: slackapi.Channel{
			GroupConversation: slackapi.GroupConversation{
				Conversation: slackapi.Conversation{ID: "C123"},
			},
		},
		View: slackapi.View{
			CallbackID:      "model_modal",
			PrivateMetadata: "model",
			State: &slackapi.ViewState{
				Values: map[string]map[string]slackapi.BlockAction{
					"model_query": {
						"model_query_input": {
							Value: "openai",
						},
					},
				},
			},
		},
	}

	ch.handleViewSubmission(callback)

	if api.ephemeralChannel != "C123" {
		t.Fatalf("unexpected ephemeral target channel: %q", api.ephemeralChannel)
	}
	if api.ephemeralUser != "U123" {
		t.Fatalf("expected ephemeral response for U123, got %q", api.ephemeralUser)
	}
	if len(api.ephemeralOpts) == 0 {
		t.Fatal("expected ephemeral response options to be sent")
	}
}

func TestHandleViewSubmissionExecutesHelpCommand(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	if err := ch.commands.Register(&commands.Command{
		Name: "help",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if req.Channel != "slack" {
				t.Fatalf("unexpected channel: %q", req.Channel)
			}
			if req.Command != "help" {
				t.Fatalf("unexpected command: %q", req.Command)
			}
			if req.Args != "status" {
				t.Fatalf("unexpected args: %q", req.Args)
			}
			if req.UserID != "U123" {
				t.Fatalf("unexpected user: %q", req.UserID)
			}
			if req.Metadata["team_id"] != "T123" {
				t.Fatalf("unexpected team id: %q", req.Metadata["team_id"])
			}
			if req.Metadata["runtime_id"] != "slack" {
				t.Fatalf("unexpected runtime id: %q", req.Metadata["runtime_id"])
			}
			return commands.CommandResponse{
				Content: "**/status**",
			}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	callback := slackapi.InteractionCallback{
		Type: slackapi.InteractionTypeViewSubmission,
		User: slackapi.User{ID: "U123", Name: "alice"},
		Team: slackapi.Team{ID: "T123"},
		Channel: slackapi.Channel{
			GroupConversation: slackapi.GroupConversation{
				Conversation: slackapi.Conversation{ID: "C123"},
			},
		},
		View: slackapi.View{
			CallbackID:      "help_modal",
			PrivateMetadata: "help",
			State: &slackapi.ViewState{
				Values: map[string]map[string]slackapi.BlockAction{
					"help_query": {
						"help_query_input": {
							Value: "status",
						},
					},
				},
			},
		},
	}

	ch.handleViewSubmission(callback)

	if api.ephemeralChannel != "C123" {
		t.Fatalf("unexpected ephemeral target channel: %q", api.ephemeralChannel)
	}
	if api.ephemeralUser != "U123" {
		t.Fatalf("expected ephemeral response for U123, got %q", api.ephemeralUser)
	}
	if len(api.ephemeralOpts) == 0 {
		t.Fatal("expected ephemeral response options to be sent")
	}
}

func TestHandleViewSubmissionExecutesStatusCommand(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	if err := ch.commands.Register(&commands.Command{
		Name: "status",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if req.Channel != "slack" {
				t.Fatalf("unexpected channel: %q", req.Channel)
			}
			if req.Command != "status" {
				t.Fatalf("unexpected command: %q", req.Command)
			}
			if req.Args != "" {
				t.Fatalf("unexpected args: %q", req.Args)
			}
			if req.UserID != "U123" {
				t.Fatalf("unexpected user: %q", req.UserID)
			}
			if req.Metadata["team_id"] != "T123" {
				t.Fatalf("unexpected team id: %q", req.Metadata["team_id"])
			}
			if req.Metadata["runtime_id"] != "slack" {
				t.Fatalf("unexpected runtime id: %q", req.Metadata["runtime_id"])
			}
			return commands.CommandResponse{
				Content: "📊 Status OK",
			}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	callback := slackapi.InteractionCallback{
		Type: slackapi.InteractionTypeViewSubmission,
		User: slackapi.User{ID: "U123", Name: "alice"},
		Team: slackapi.Team{ID: "T123"},
		Channel: slackapi.Channel{
			GroupConversation: slackapi.GroupConversation{
				Conversation: slackapi.Conversation{ID: "C123"},
			},
		},
		View: slackapi.View{
			CallbackID:      "status_modal",
			PrivateMetadata: "status",
		},
	}

	ch.handleViewSubmission(callback)

	if api.ephemeralChannel != "C123" {
		t.Fatalf("unexpected ephemeral target channel: %q", api.ephemeralChannel)
	}
	if api.ephemeralUser != "U123" {
		t.Fatalf("expected ephemeral response for U123, got %q", api.ephemeralUser)
	}
	if len(api.ephemeralOpts) == 0 {
		t.Fatal("expected ephemeral response options to be sent")
	}
}

func TestHandleViewSubmissionExecutesAgentCommand(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	if err := ch.commands.Register(&commands.Command{
		Name: "agent",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if req.Channel != "slack" {
				t.Fatalf("unexpected channel: %q", req.Channel)
			}
			if req.Command != "agent" {
				t.Fatalf("unexpected command: %q", req.Command)
			}
			if req.Args != "list" {
				t.Fatalf("unexpected args: %q", req.Args)
			}
			if req.UserID != "U123" {
				t.Fatalf("unexpected user: %q", req.UserID)
			}
			if req.Metadata["team_id"] != "T123" {
				t.Fatalf("unexpected team id: %q", req.Metadata["team_id"])
			}
			if req.Metadata["runtime_id"] != "slack" {
				t.Fatalf("unexpected runtime id: %q", req.Metadata["runtime_id"])
			}
			return commands.CommandResponse{
				Content: "🤖 **Available Providers**",
			}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	callback := slackapi.InteractionCallback{
		Type: slackapi.InteractionTypeViewSubmission,
		User: slackapi.User{ID: "U123", Name: "alice"},
		Team: slackapi.Team{ID: "T123"},
		Channel: slackapi.Channel{
			GroupConversation: slackapi.GroupConversation{
				Conversation: slackapi.Conversation{ID: "C123"},
			},
		},
		View: slackapi.View{
			CallbackID:      "agent_modal",
			PrivateMetadata: "agent",
			State: &slackapi.ViewState{
				Values: map[string]map[string]slackapi.BlockAction{
					"agent_query": {
						"agent_query_input": {
							Value: "list",
						},
					},
				},
			},
		},
	}

	ch.handleViewSubmission(callback)

	if api.ephemeralChannel != "C123" {
		t.Fatalf("unexpected ephemeral target channel: %q", api.ephemeralChannel)
	}
	if api.ephemeralUser != "U123" {
		t.Fatalf("expected ephemeral response for U123, got %q", api.ephemeralUser)
	}
	if len(api.ephemeralOpts) == 0 {
		t.Fatal("expected ephemeral response options to be sent")
	}
}

func TestHandleViewSubmissionExecutesStartCommand(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api

	if err := ch.commands.Register(&commands.Command{
		Name: "start",
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			if req.Channel != "slack" {
				t.Fatalf("unexpected channel: %q", req.Channel)
			}
			if req.Command != "start" {
				t.Fatalf("unexpected command: %q", req.Command)
			}
			if req.Args != "" {
				t.Fatalf("unexpected args: %q", req.Args)
			}
			if req.UserID != "U123" {
				t.Fatalf("unexpected user: %q", req.UserID)
			}
			if req.Metadata["team_id"] != "T123" {
				t.Fatalf("unexpected team id: %q", req.Metadata["team_id"])
			}
			if req.Metadata["runtime_id"] != "slack" {
				t.Fatalf("unexpected runtime id: %q", req.Metadata["runtime_id"])
			}
			return commands.CommandResponse{
				Content: "👋 **Welcome to Nanobot!**",
			}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	callback := slackapi.InteractionCallback{
		Type: slackapi.InteractionTypeViewSubmission,
		User: slackapi.User{ID: "U123", Name: "alice"},
		Team: slackapi.Team{ID: "T123"},
		Channel: slackapi.Channel{
			GroupConversation: slackapi.GroupConversation{
				Conversation: slackapi.Conversation{ID: "C123"},
			},
		},
		View: slackapi.View{
			CallbackID:      "start_modal",
			PrivateMetadata: "start",
		},
	}

	ch.handleViewSubmission(callback)

	if api.ephemeralChannel != "C123" {
		t.Fatalf("unexpected ephemeral target channel: %q", api.ephemeralChannel)
	}
	if api.ephemeralUser != "U123" {
		t.Fatalf("expected ephemeral response for U123, got %q", api.ephemeralUser)
	}
	if len(api.ephemeralOpts) == 0 {
		t.Fatal("expected ephemeral response options to be sent")
	}
}

func TestHandleSlashCommandRejectsAdminOnlyCommandForNonAdmin(t *testing.T) {
	ch := newTestChannel(t)
	api := &stubSlackAPI{}
	ch.api = api
	ch.config.AllowFrom = []string{"U123"}
	ch.ctx = context.Background()

	if err := ch.commands.Register(&commands.Command{
		Name:      "gateway",
		AdminOnly: true,
		Handler: func(ctx context.Context, req commands.CommandRequest) (commands.CommandResponse, error) {
			t.Fatal("admin-only command handler should not be executed")
			return commands.CommandResponse{}, nil
		},
	}); err != nil {
		t.Fatalf("register command: %v", err)
	}

	ch.handleSlashCommand(socketmode.Event{
		Request: &socketmode.Request{},
		Data: slackapi.SlashCommand{
			Command:     "/gateway",
			Text:        "status",
			ChannelID:   "C123",
			UserID:      "U123",
			UserName:    "alice",
			ChannelName: "general",
			TeamID:      "T123",
			TeamDomain:  "test-team",
			TriggerID:   "trigger-1",
		},
	})

	if api.ephemeralChannel != "C123" {
		t.Fatalf("unexpected ephemeral target channel: %q", api.ephemeralChannel)
	}
	if api.ephemeralUser != "U123" {
		t.Fatalf("unexpected ephemeral target user: %q", api.ephemeralUser)
	}
	if len(api.ephemeralOpts) == 0 {
		t.Fatal("expected ephemeral rejection options to be sent")
	}
}

type slackapieventsMessageEvent struct {
	Channel         string
	ThreadTimeStamp string
	User            string
	Text            string
	TimeStamp       string
}

func (e *slackapieventsMessageEvent) toSlack() *slackevents.MessageEvent {
	return &slackevents.MessageEvent{
		Channel:         e.Channel,
		ThreadTimeStamp: e.ThreadTimeStamp,
		User:            e.User,
		Text:            e.Text,
		TimeStamp:       e.TimeStamp,
	}
}
