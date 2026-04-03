package telegram

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

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
		channelType: "telegram",
	}
}

var _ = tgbotapi.InlineKeyboardMarkup{}
