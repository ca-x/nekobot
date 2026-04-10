package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"nekobot/pkg/bus"
	"nekobot/pkg/logger"
)

func TestSupportsInlineButtonsRespectsDefaultCapabilityScope(t *testing.T) {
	channel := newTestChannel(t)

	if !channel.supportsInlineButtons("private") {
		t.Fatal("expected inline buttons to be enabled for private chats")
	}
	if channel.supportsInlineButtons("group") {
		t.Fatal("expected inline buttons to be disabled for group chats")
	}
	if channel.supportsInlineButtons("supergroup") {
		t.Fatal("expected inline buttons to be disabled for supergroup chats")
	}
}

func TestScopedInlineKeyboardDropsButtonsOutsideSupportedScope(t *testing.T) {
	channel := newTestChannel(t)
	keyboard := channel.settingsMainKeyboard("en")

	privateKeyboard := channel.scopedInlineKeyboard("private", keyboard)
	if privateKeyboard == nil {
		t.Fatal("expected private chat keyboard to be preserved")
	}
	if len(privateKeyboard.InlineKeyboard) == 0 {
		t.Fatal("expected private chat keyboard rows to be preserved")
	}

	groupKeyboard := channel.scopedInlineKeyboard("group", keyboard)
	if groupKeyboard != nil {
		t.Fatal("expected group chat keyboard to be suppressed")
	}
}

func TestSkillInstallPromptFallsBackToTextConfirmationWithoutInlineButtons(t *testing.T) {
	channel := newTestChannel(t)

	groupPrompt := channel.skillInstallPromptText("group", "en", "owner/repo")
	if !strings.Contains(groupPrompt, "/yes") || !strings.Contains(groupPrompt, "/no") {
		t.Fatalf("expected text confirmation hints in group prompt, got %q", groupPrompt)
	}

	privatePrompt := channel.skillInstallPromptText("private", "en", "owner/repo")
	if strings.Contains(privatePrompt, "/yes") || strings.Contains(privatePrompt, "/no") {
		t.Fatalf("expected private prompt to rely on buttons, got %q", privatePrompt)
	}
}

func TestSendThinkingMessageSkipsGroupsWhenStreamingUnsupported(t *testing.T) {
	channel := newTestChannel(t)

	var sendMessageCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest-token/getMe":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Test","username":"testbot"}}`))
		case "/bottest-token/sendMessage":
			sendMessageCalls++
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":41}}`))
		default:
			t.Fatalf("unexpected telegram API path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint("test-token", server.URL+"/bot%s/%s")
	if err != nil {
		t.Fatalf("create bot api: %v", err)
	}
	channel.bot = bot

	groupThinkingID := channel.sendThinkingMessage(-10001, 9, "thinking")
	if groupThinkingID != 0 {
		t.Fatalf("expected group thinking message to be skipped, got id %d", groupThinkingID)
	}
	if sendMessageCalls != 0 {
		t.Fatalf("expected no telegram send for unsupported group streaming, got %d calls", sendMessageCalls)
	}

	privateThinkingID := channel.sendThinkingMessage(10001, 9, "thinking")
	if privateThinkingID != 41 {
		t.Fatalf("expected private thinking message id 41, got %d", privateThinkingID)
	}
	if sendMessageCalls != 1 {
		t.Fatalf("expected one telegram send for supported private streaming, got %d calls", sendMessageCalls)
	}
}

func newTestChannel(t *testing.T) *Channel {
	t.Helper()

	cfg := logger.DefaultConfig()
	cfg.OutputPath = ""
	cfg.Development = true
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	return &Channel{
		log:         log,
		id:          "telegram",
		channelType: "telegram",
	}
}

var _ = tgbotapi.InlineKeyboardMarkup{}

func TestSupportsNativeCommandsRespectsDefaultCapabilityScope(t *testing.T) {
	channel := newTestChannel(t)
	if !channel.supportsNativeCommands("private") {
		t.Fatal("expected native commands enabled for private chats")
	}
	if !channel.supportsNativeCommands("group") {
		t.Fatal("expected native commands enabled for group chats")
	}
	if !channel.supportsNativeCommands("supergroup") {
		t.Fatal("expected native commands enabled for supergroup chats")
	}
}

func TestSendMessagePrependsToolTraceFromBusMetadata(t *testing.T) {
	channel := newTestChannel(t)

	var sentTexts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest-token/getMe":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Test","username":"testbot"}}`))
		case "/bottest-token/sendMessage":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			sentTexts = append(sentTexts, r.Form.Get("text"))
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
		default:
			t.Fatalf("unexpected telegram API path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint("test-token", server.URL+"/bot%s/%s")
	if err != nil {
		t.Fatalf("create bot api: %v", err)
	}
	channel.bot = bot

	err = channel.SendMessage(context.Background(), &bus.Message{
		SessionID: "telegram:123",
		Content:   "done",
		Data: map[string]interface{}{
			"tool_call_trace": "Tool call: read_file {\"path\":\"README.md\"}",
		},
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if len(sentTexts) != 1 {
		t.Fatalf("expected exactly one telegram send, got %d", len(sentTexts))
	}
	if !strings.Contains(sentTexts[0], "Tool call: read_file") {
		t.Fatalf("expected tool trace in telegram text, got %q", sentTexts[0])
	}
	if !strings.Contains(sentTexts[0], "\n\ndone") {
		t.Fatalf("expected original reply after blank line, got %q", sentTexts[0])
	}
}
