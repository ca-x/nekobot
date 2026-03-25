package slack

import (
	"context"
	"strings"
	"testing"
	"time"

	slackapi "github.com/slack-go/slack"

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

func (s *stubSlackAPI) UpdateMessage(channelID, timestamp string, options ...slackapi.MsgOption) (string, string, string, error) {
	s.updateMessageChannel = channelID
	s.updateMessageTS = timestamp
	s.updateMessageOpts = append([]slackapi.MsgOption(nil), options...)
	return channelID, timestamp, "", s.updateMessageErr
}

type stubBus struct {
	inbound []*bus.Message
}

func (b *stubBus) Start() error                                          { return nil }
func (b *stubBus) Stop() error                                           { return nil }
func (b *stubBus) RegisterHandler(channelID string, handler bus.Handler) {}
func (b *stubBus) UnregisterHandlers(channelID string)                   {}
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
