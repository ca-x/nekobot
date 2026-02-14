// Package telegram provides Telegram bot integration.
package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/transcription"
	"nekobot/pkg/userprefs"
)

// simpleSession is a simple session implementation for telegram messages.
type simpleSession struct {
	messages []agent.Message
	mu       sync.RWMutex
}

func (s *simpleSession) GetMessages() []agent.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messages
}

func (s *simpleSession) AddMessage(msg agent.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

// Channel implements the Telegram channel.
type Channel struct {
	log         *logger.Logger
	bus         bus.Bus // Use interface, not pointer to interface
	agent       *agent.Agent
	commands    *commands.Registry
	config      *config.TelegramConfig
	transcriber transcription.Transcriber
	prefs       *userprefs.Manager

	bot      *tgbotapi.BotAPI
	stopOnce sync.Once
	ctx      context.Context
	cancel   context.CancelFunc

	settingsMu    sync.Mutex
	settingsInput map[string]string
}

// New creates a new Telegram channel.
func New(
	log *logger.Logger,
	messageBus bus.Bus,
	ag *agent.Agent,
	cmdRegistry *commands.Registry,
	cfg *config.TelegramConfig,
	transcriber transcription.Transcriber,
	prefsMgr *userprefs.Manager,
) (*Channel, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram token is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Channel{
		log:           log,
		bus:           messageBus,
		agent:         ag,
		commands:      cmdRegistry,
		config:        cfg,
		transcriber:   transcriber,
		prefs:         prefsMgr,
		ctx:           ctx,
		cancel:        cancel,
		settingsInput: map[string]string{},
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "telegram"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "Telegram"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the Telegram bot and begins listening for messages.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting Telegram channel")

	// Keep HTTP timeout longer than long-poll timeout to avoid periodic forced reconnects.
	httpClient := &http.Client{Timeout: 75 * time.Second}
	if c.config.Proxy != "" {
		proxyURL, err := url.Parse(c.config.Proxy)
		if err != nil {
			return fmt.Errorf("parsing telegram proxy: %w", err)
		}
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		c.log.Info("Telegram proxy enabled", zap.String("proxy", proxyURL.String()))
	}

	// Create bot
	bot, err := tgbotapi.NewBotAPIWithClient(c.config.Token, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		return fmt.Errorf("creating telegram bot: %w", err)
	}

	c.bot = bot
	c.stopOnce = sync.Once{}
	c.bot.Debug = false

	c.log.Info("Telegram bot connected",
		zap.String("username", bot.Self.UserName))
	c.syncSlashCommands()

	// Setup updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 50

	updates := bot.GetUpdatesChan(u)

	// Process updates
	for {
		select {
		case update := <-updates:
			c.handleUpdate(update)

		case <-ctx.Done():
			c.log.Info("Telegram channel stopping")
			c.stopReceivingUpdates()
			return nil

		case <-c.ctx.Done():
			c.log.Info("Telegram channel stopping")
			c.stopReceivingUpdates()
			return nil
		}
	}
}

// Stop stops the Telegram channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Telegram channel")
	c.cancel()
	c.stopReceivingUpdates()

	return nil
}

func (c *Channel) stopReceivingUpdates() {
	if c.bot == nil {
		return
	}
	c.stopOnce.Do(func() {
		c.bot.StopReceivingUpdates()
	})
}

func (c *Channel) requestTimeout() time.Duration {
	if c.config.TimeoutSeconds > 0 {
		return time.Duration(c.config.TimeoutSeconds) * time.Second
	}
	return 60 * time.Second
}

// SendMessage sends a message through Telegram.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	if c.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	// Parse chat ID from session ID (format: "telegram:123456")
	chatID, err := c.extractChatID(msg.SessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	// Create message
	reply := tgbotapi.NewMessage(chatID, msg.Content)

	// Handle reply
	if msg.ReplyTo != "" {
		// Parse message ID
		if msgID, err := c.extractMessageID(msg.ReplyTo); err == nil {
			reply.ReplyToMessageID = msgID
		}
	}

	// Send message
	if _, err := c.bot.Send(reply); err != nil {
		return fmt.Errorf("sending telegram message: %w", err)
	}

	return nil
}

// handleUpdate processes a Telegram update.
func (c *Channel) handleUpdate(update tgbotapi.Update) {
	// Handle messages
	if update.Message != nil {
		msg := *update.Message
		go c.handleMessage(&msg)
		return
	}

	if update.CallbackQuery != nil {
		cb := *update.CallbackQuery
		go c.handleCallbackQuery(&cb)
		return
	}

	// Handle other update types as needed
}

func (c *Channel) syncSlashCommands() {
	if c.bot == nil || c.commands == nil {
		return
	}

	cmds := c.commands.List()
	telegramCmds := make([]tgbotapi.BotCommand, 0, len(cmds))
	seen := make(map[string]struct{})

	for _, cmd := range cmds {
		name := sanitizeTelegramCommandName(cmd.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}

		desc := strings.TrimSpace(cmd.Description)
		if desc == "" {
			desc = strings.TrimSpace(cmd.Usage)
		}
		if desc == "" {
			desc = "Command"
		}
		if len(desc) > 256 {
			desc = desc[:256]
		}

		telegramCmds = append(telegramCmds, tgbotapi.BotCommand{
			Command:     name,
			Description: desc,
		})
	}

	if len(telegramCmds) == 0 {
		return
	}

	// Telegram supports at most 100 commands.
	sort.Slice(telegramCmds, func(i, j int) bool {
		return telegramCmds[i].Command < telegramCmds[j].Command
	})
	if len(telegramCmds) > 100 {
		telegramCmds = telegramCmds[:100]
	}

	if _, err := c.bot.Request(tgbotapi.NewSetMyCommands(telegramCmds...)); err != nil {
		c.log.Warn("Failed to sync Telegram slash commands (default scope)", zap.Error(err))
		return
	}
	if _, err := c.bot.Request(
		tgbotapi.NewSetMyCommandsWithScope(
			tgbotapi.NewBotCommandScopeAllPrivateChats(),
			telegramCmds...,
		),
	); err != nil {
		c.log.Warn("Failed to sync Telegram slash commands (private scope)", zap.Error(err))
	}

	c.log.Info("Synced Telegram slash commands", zap.Int("count", len(telegramCmds)))
}

func sanitizeTelegramCommandName(name string) string {
	normalized := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), "/"))
	if normalized == "" {
		return ""
	}

	var b strings.Builder
	lastUnderscore := false
	for _, r := range normalized {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case r == '-' || r == '_':
			if b.Len() > 0 && !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		default:
			// Ignore unsupported characters.
		}

		if b.Len() >= 32 {
			break
		}
	}

	result := strings.Trim(b.String(), "_")
	if result == "" {
		return ""
	}
	return result
}

// handleMessage processes an incoming message.
func (c *Channel) handleMessage(message *tgbotapi.Message) {
	// Check if user is allowed
	if !c.isUserAllowed(message.From.ID, message.Chat.ID, message.From.UserName) {
		c.log.Warn("Unauthorized access attempt",
			zap.Int64("user_id", message.From.ID),
			zap.String("username", message.From.UserName))
		c.sendAccessDenied(message)
		return
	}

	content := strings.TrimSpace(message.Text)
	msgType := bus.MessageTypeText

	// Support voice/audio messages via Whisper transcription.
	if content == "" && c.transcriber != nil {
		transcribed, ok := c.tryTranscribeAudio(message)
		if ok {
			content = transcribed
			msgType = bus.MessageTypeAudio
		}
	}
	if content == "" {
		return
	}

	if msgType == bus.MessageTypeText && c.consumePendingSettingsInput(message, content) {
		return
	}

	c.log.Info("Received Telegram message",
		zap.Int64("chat_id", message.Chat.ID),
		zap.String("from", message.From.UserName),
		zap.String("text", content))

	// Check if it's a command
	if c.commands.IsCommand(content) {
		if msgType == bus.MessageTypeText {
			c.handleCommand(message)
			return
		}

		// For transcribed voice command text, run command path with synthetic message text.
		clone := *message
		clone.Text = content
		c.handleCommand(&clone)
		return
	}

	// Create bus message
	busMsg := &bus.Message{
		ID:        fmt.Sprintf("telegram:%d", message.MessageID),
		ChannelID: c.ID(),
		SessionID: fmt.Sprintf("telegram:%d", message.Chat.ID),
		UserID:    fmt.Sprintf("%d", message.From.ID),
		Username:  message.From.UserName,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Unix(int64(message.Date), 0),
	}

	if message.ReplyToMessage != nil {
		busMsg.ReplyTo = fmt.Sprintf("telegram:%d", message.ReplyToMessage.MessageID)
	}

	// Send to bus for processing
	ctx, cancel := context.WithTimeout(context.Background(), c.requestTimeout())
	defer cancel()

	// Create a simple session for this message
	sess := &simpleSession{
		messages: make([]agent.Message, 0),
	}

	thinkingMsgID := c.sendThinkingMessage(message.Chat.ID, message.MessageID, "🤔 正在思考中...")
	agentInput := c.applyUserProfile(context.Background(), busMsg.UserID, content)

	// Process with agent
	response, err := c.agent.Chat(ctx, sess, agentInput)
	if err != nil {
		c.log.Error("Agent chat failed", zap.Error(err))

		// Send error message
		c.finishThinkingMessage(message.Chat.ID, message.MessageID, thinkingMsgID, "❌ 抱歉，处理消息时出现错误。")
		return
	}

	c.finishThinkingMessage(message.Chat.ID, message.MessageID, thinkingMsgID, response)
}

func (c *Channel) tryTranscribeAudio(message *tgbotapi.Message) (string, bool) {
	fileID := ""
	filename := "voice.ogg"
	switch {
	case message.Voice != nil:
		fileID = message.Voice.FileID
		filename = "voice.ogg"
	case message.Audio != nil:
		fileID = message.Audio.FileID
		if message.Audio.FileName != "" {
			filename = message.Audio.FileName
		}
	case message.Document != nil:
		fileID = message.Document.FileID
		if message.Document.FileName != "" {
			filename = message.Document.FileName
		}
	default:
		return "", false
	}
	if fileID == "" {
		return "", false
	}

	audioBytes, err := c.downloadFile(fileID)
	if err != nil {
		c.log.Warn("Failed to download Telegram audio for transcription", zap.Error(err))
		return "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.requestTimeout())
	defer cancel()

	text, err := c.transcriber.Transcribe(ctx, audioBytes, filename)
	if err != nil {
		c.log.Warn("Telegram audio transcription failed", zap.Error(err))
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	return text, true
}

func (c *Channel) downloadFile(fileID string) ([]byte, error) {
	url, err := c.bot.GetFileDirectURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("resolving file URL: %w", err)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloading file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading file body: %w", err)
	}
	return data, nil
}

// handleCommand processes a command message.
func (c *Channel) handleCommand(message *tgbotapi.Message) {
	cmdName, args := c.commands.Parse(message.Text)

	if cmdName == "settings" && strings.TrimSpace(args) == "" {
		c.sendSettingsMenu(message.Chat.ID, message.From.ID, message.MessageID, "")
		return
	}

	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		c.log.Debug("Unknown command", zap.String("command", cmdName))
		return
	}

	c.log.Info("Executing command",
		zap.String("command", cmdName),
		zap.String("user", message.From.UserName))

	// Create command request
	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   fmt.Sprintf("%d", message.Chat.ID),
		UserID:   fmt.Sprintf("%d", message.From.ID),
		Username: message.From.UserName,
		Command:  cmdName,
		Args:     args,
		Metadata: map[string]string{
			"message_id": fmt.Sprintf("%d", message.MessageID),
			"chat_type":  message.Chat.Type,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.requestTimeout())
	defer cancel()

	thinkingMsgID := c.sendThinkingMessage(message.Chat.ID, message.MessageID, "🤔 正在处理命令...")

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		c.log.Error("Command execution failed",
			zap.String("command", cmdName),
			zap.Error(err))

		c.finishThinkingMessage(message.Chat.ID, message.MessageID, thinkingMsgID, "❌ Command failed: "+err.Error())
		return
	}

	c.finishThinkingMessage(message.Chat.ID, message.MessageID, thinkingMsgID, resp.Content)
}

func (c *Channel) handleCallbackQuery(cb *tgbotapi.CallbackQuery) {
	if cb == nil {
		return
	}
	if cb.Message == nil {
		c.answerCallback(cb.ID, "ok", false)
		return
	}

	if !strings.HasPrefix(cb.Data, "settings:") {
		c.answerCallback(cb.ID, "ok", false)
		return
	}

	chatID := cb.Message.Chat.ID
	userID := cb.From.ID
	messageID := cb.Message.MessageID
	if !c.isUserAllowed(userID, chatID, cb.From.UserName) {
		c.answerCallback(cb.ID, "你不在 allow_from 白名单中", true)
		return
	}

	ctx := context.Background()
	profile, _, _ := c.getProfile(ctx, userID)
	lang := userprefs.NormalizeLanguage(profile.Language)

	switch cb.Data {
	case "settings:view", "settings:back":
		c.clearSettingsInput(chatID, userID)
		c.renderSettingsMenu(chatID, userID, messageID, "", lang)
		c.answerCallback(cb.ID, c.settingsText(lang, "已打开设置", "Opened settings", "設定を開きました"), false)
	case "settings:lang_menu":
		text := c.settingsText(lang,
			"请选择语言：",
			"Choose your language:",
			"言語を選択してください:",
		)
		c.editSettingsMessage(chatID, messageID, text, c.settingsLanguageKeyboard(lang))
		c.answerCallback(cb.ID, "ok", false)
	case "settings:lang:zh", "settings:lang:en", "settings:lang:ja":
		langCode := strings.TrimPrefix(cb.Data, "settings:lang:")
		profile.Language = userprefs.NormalizeLanguage(langCode)
		if err := c.saveProfile(ctx, userID, profile); err != nil {
			c.answerCallback(cb.ID, c.settingsText(lang, "保存失败", "Save failed", "保存に失敗しました"), true)
			return
		}
		lang = profile.Language
		notice := c.settingsText(lang,
			"✅ 语言已更新",
			"✅ Language updated",
			"✅ 言語を更新しました",
		)
		c.renderSettingsMenu(chatID, userID, messageID, notice, lang)
		c.answerCallback(cb.ID, notice, false)
	case "settings:name":
		c.setSettingsInput(chatID, userID, "name")
		text := c.settingsText(lang,
			"请直接发送你希望的称呼（发送 /cancel 取消）",
			"Send your preferred display name now (send /cancel to cancel)",
			"希望する呼び名を送ってください（/cancel でキャンセル）",
		)
		c.editSettingsMessage(chatID, messageID, text, c.settingsMainKeyboard(lang))
		c.answerCallback(cb.ID, "ok", false)
	case "settings:prefs":
		c.setSettingsInput(chatID, userID, "prefs")
		text := c.settingsText(lang,
			"请直接发送你的偏好说明（发送 /cancel 取消）",
			"Send your preference note now (send /cancel to cancel)",
			"好みの説明を送ってください（/cancel でキャンセル）",
		)
		c.editSettingsMessage(chatID, messageID, text, c.settingsMainKeyboard(lang))
		c.answerCallback(cb.ID, "ok", false)
	case "settings:clear":
		if err := c.clearProfile(ctx, userID); err != nil {
			c.answerCallback(cb.ID, c.settingsText(lang, "清除失败", "Clear failed", "クリア失敗"), true)
			return
		}
		c.clearSettingsInput(chatID, userID)
		notice := c.settingsText(lang,
			"✅ 已清除设置",
			"✅ Settings cleared",
			"✅ 設定をクリアしました",
		)
		c.renderSettingsMenu(chatID, userID, messageID, notice, lang)
		c.answerCallback(cb.ID, notice, false)
	case "settings:close":
		c.clearSettingsInput(chatID, userID)
		text := c.settingsText(lang,
			"✅ 设置面板已关闭，输入 /settings 可再次打开。",
			"✅ Settings closed. Type /settings to open again.",
			"✅ 設定パネルを閉じました。/settings で再度開けます。",
		)
		c.editSettingsMessage(chatID, messageID, text, tgbotapi.NewInlineKeyboardMarkup())
		c.answerCallback(cb.ID, "ok", false)
	default:
		c.answerCallback(cb.ID, "ok", false)
	}
}

func (c *Channel) consumePendingSettingsInput(message *tgbotapi.Message, content string) bool {
	mode, ok := c.getSettingsInput(message.Chat.ID, message.From.ID)
	if !ok {
		return false
	}

	if strings.HasPrefix(content, "/") {
		if strings.EqualFold(strings.TrimSpace(content), "/cancel") {
			c.clearSettingsInput(message.Chat.ID, message.From.ID)
			c.sendSettingsMenu(message.Chat.ID, message.From.ID, message.MessageID, c.settingsText("zh", "已取消输入", "Input canceled", "入力をキャンセルしました"))
			return true
		}
		return false
	}

	ctx := context.Background()
	profile, _, _ := c.getProfile(ctx, message.From.ID)
	lang := userprefs.NormalizeLanguage(profile.Language)

	switch mode {
	case "name":
		profile.PreferredName = strings.TrimSpace(content)
	case "prefs":
		profile.Preferences = strings.TrimSpace(content)
	default:
		c.clearSettingsInput(message.Chat.ID, message.From.ID)
		return false
	}

	if err := c.saveProfile(ctx, message.From.ID, profile); err != nil {
		reply := tgbotapi.NewMessage(message.Chat.ID, c.settingsText(lang, "❌ 保存失败", "❌ Save failed", "❌ 保存失敗"))
		reply.ReplyToMessageID = message.MessageID
		_, _ = c.bot.Send(reply)
		return true
	}

	c.clearSettingsInput(message.Chat.ID, message.From.ID)
	notice := c.settingsText(lang,
		"✅ 设置已更新",
		"✅ Settings updated",
		"✅ 設定を更新しました",
	)
	c.sendSettingsMenu(message.Chat.ID, message.From.ID, message.MessageID, notice)
	return true
}

func (c *Channel) sendSettingsMenu(chatID, userID int64, replyTo int, notice string) {
	ctx := context.Background()
	profile, _, _ := c.getProfile(ctx, userID)
	lang := userprefs.NormalizeLanguage(profile.Language)
	text := c.settingsSummaryText(profile, notice, lang)
	msg := tgbotapi.NewMessage(chatID, text)
	if replyTo > 0 {
		msg.ReplyToMessageID = replyTo
	}
	kb := c.settingsMainKeyboard(lang)
	msg.ReplyMarkup = kb
	if _, err := c.bot.Send(msg); err != nil {
		c.log.Warn("Failed to send settings menu", zap.Error(err))
	}
}

func (c *Channel) renderSettingsMenu(chatID, userID int64, messageID int, notice, lang string) {
	ctx := context.Background()
	profile, _, _ := c.getProfile(ctx, userID)
	if lang == "" {
		lang = userprefs.NormalizeLanguage(profile.Language)
	}
	text := c.settingsSummaryText(profile, notice, lang)
	c.editSettingsMessage(chatID, messageID, text, c.settingsMainKeyboard(lang))
}

func (c *Channel) settingsSummaryText(profile userprefs.Profile, notice, lang string) string {
	langCode := profile.Language
	if langCode == "" {
		langCode = "zh"
	}
	name := strings.TrimSpace(profile.PreferredName)
	if name == "" {
		name = c.settingsText(lang, "(未设置)", "(not set)", "(未設定)")
	}
	pref := strings.TrimSpace(profile.Preferences)
	if pref == "" {
		pref = c.settingsText(lang, "(未设置)", "(not set)", "(未設定)")
	}

	var sb strings.Builder
	if strings.TrimSpace(notice) != "" {
		sb.WriteString(notice)
		sb.WriteString("\n\n")
	}
	sb.WriteString(c.settingsText(lang, "⚙️ 个人设置", "⚙️ Personal Settings", "⚙️ 個人設定"))
	sb.WriteString("\n\n")
	sb.WriteString(c.settingsText(lang, "语言", "Language", "言語"))
	sb.WriteString(": ")
	sb.WriteString(langCode)
	sb.WriteString("\n")
	sb.WriteString(c.settingsText(lang, "称呼", "Name", "呼び名"))
	sb.WriteString(": ")
	sb.WriteString(name)
	sb.WriteString("\n")
	sb.WriteString(c.settingsText(lang, "偏好", "Preferences", "好み"))
	sb.WriteString(": ")
	sb.WriteString(pref)
	return sb.String()
}

func (c *Channel) settingsMainKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(c.settingsText(lang, "🌐 语言", "🌐 Language", "🌐 言語"), "settings:lang_menu"),
			tgbotapi.NewInlineKeyboardButtonData(c.settingsText(lang, "🔄 刷新", "🔄 Refresh", "🔄 更新"), "settings:view"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(c.settingsText(lang, "📝 设置称呼", "📝 Set Name", "📝 呼び名設定"), "settings:name"),
			tgbotapi.NewInlineKeyboardButtonData(c.settingsText(lang, "💡 设置偏好", "💡 Set Preferences", "💡 好み設定"), "settings:prefs"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(c.settingsText(lang, "🧹 清除", "🧹 Clear", "🧹 クリア"), "settings:clear"),
			tgbotapi.NewInlineKeyboardButtonData(c.settingsText(lang, "❌ 关闭", "❌ Close", "❌ 閉じる"), "settings:close"),
		),
	)
}

func (c *Channel) settingsLanguageKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("中文", "settings:lang:zh"),
			tgbotapi.NewInlineKeyboardButtonData("English", "settings:lang:en"),
			tgbotapi.NewInlineKeyboardButtonData("日本語", "settings:lang:ja"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(c.settingsText(lang, "⬅️ 返回", "⬅️ Back", "⬅️ 戻る"), "settings:back"),
		),
	)
}

func (c *Channel) editSettingsMessage(chatID int64, messageID int, text string, kb tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ReplyMarkup = &kb
	if _, err := c.bot.Send(edit); err != nil {
		c.log.Warn("Failed to edit settings message", zap.Error(err))
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = kb
		_, _ = c.bot.Send(msg)
	}
}

func (c *Channel) answerCallback(id, text string, alert bool) {
	if c.bot == nil || strings.TrimSpace(id) == "" {
		return
	}
	cb := tgbotapi.NewCallback(id, text)
	cb.ShowAlert = alert
	_, _ = c.bot.Request(cb)
}

func (c *Channel) settingsText(lang, zh, en, ja string) string {
	switch userprefs.NormalizeLanguage(lang) {
	case "en":
		return en
	case "ja":
		return ja
	default:
		return zh
	}
}

func (c *Channel) settingsKey(chatID, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}

func (c *Channel) setSettingsInput(chatID, userID int64, mode string) {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	c.settingsInput[c.settingsKey(chatID, userID)] = mode
}

func (c *Channel) getSettingsInput(chatID, userID int64) (string, bool) {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	mode, ok := c.settingsInput[c.settingsKey(chatID, userID)]
	return mode, ok
}

func (c *Channel) clearSettingsInput(chatID, userID int64) {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	delete(c.settingsInput, c.settingsKey(chatID, userID))
}

func (c *Channel) getProfile(ctx context.Context, userID int64) (userprefs.Profile, bool, error) {
	if c.prefs == nil {
		return userprefs.Profile{}, false, nil
	}
	return c.prefs.Get(ctx, c.ID(), fmt.Sprintf("%d", userID))
}

func (c *Channel) saveProfile(ctx context.Context, userID int64, profile userprefs.Profile) error {
	if c.prefs == nil {
		return nil
	}
	return c.prefs.Save(ctx, c.ID(), fmt.Sprintf("%d", userID), profile)
}

func (c *Channel) clearProfile(ctx context.Context, userID int64) error {
	if c.prefs == nil {
		return nil
	}
	return c.prefs.Clear(ctx, c.ID(), fmt.Sprintf("%d", userID))
}

func (c *Channel) sendThinkingMessage(chatID int64, replyTo int, text string) int {
	if c.bot == nil {
		return 0
	}

	msg := tgbotapi.NewMessage(chatID, text)
	if replyTo > 0 {
		msg.ReplyToMessageID = replyTo
	}
	sent, err := c.bot.Send(msg)
	if err != nil {
		c.log.Debug("Failed to send thinking message", zap.Error(err))
		return 0
	}
	return sent.MessageID
}

func (c *Channel) finishThinkingMessage(chatID int64, replyTo int, thinkingMsgID int, text string) {
	if c.bot == nil {
		return
	}

	if thinkingMsgID > 0 {
		edit := tgbotapi.NewEditMessageText(chatID, thinkingMsgID, text)
		if _, err := c.bot.Send(edit); err == nil {
			return
		} else {
			c.log.Warn("Failed to edit thinking message", zap.Error(err))
		}
	}

	reply := tgbotapi.NewMessage(chatID, text)
	if replyTo > 0 {
		reply.ReplyToMessageID = replyTo
	}
	if _, err := c.bot.Send(reply); err != nil {
		c.log.Error("Failed to send Telegram reply", zap.Error(err))
	}
}

func (c *Channel) sendAccessDenied(message *tgbotapi.Message) {
	if c.bot == nil || message == nil {
		return
	}

	reply := tgbotapi.NewMessage(message.Chat.ID, "❌ 你不在 allow_from 白名单中，暂时不能使用这个 agent。")
	reply.ReplyToMessageID = message.MessageID
	if _, err := c.bot.Send(reply); err != nil {
		c.log.Debug("Failed to send access denied message", zap.Error(err))
	}
}

func (c *Channel) applyUserProfile(ctx context.Context, userID, content string) string {
	if c.prefs == nil {
		return content
	}

	profile, ok, err := c.prefs.Get(ctx, c.ID(), userID)
	if err != nil || !ok {
		return content
	}

	lang := userprefs.NormalizeLanguage(profile.Language)
	langHint := map[string]string{
		"zh": "请使用中文回复。",
		"en": "Please reply in English.",
		"ja": "日本語で回答してください。",
	}[lang]

	var sb strings.Builder
	sb.WriteString("你必须遵循以下用户偏好。\n")
	if langHint != "" {
		sb.WriteString(langHint)
		sb.WriteString("\n")
	}
	if profile.PreferredName != "" {
		sb.WriteString("用户希望被称呼为：")
		sb.WriteString(profile.PreferredName)
		sb.WriteString("\n")
	}
	if profile.Preferences != "" {
		sb.WriteString("用户偏好：")
		sb.WriteString(profile.Preferences)
		sb.WriteString("\n")
	}
	sb.WriteString("\n用户消息：")
	sb.WriteString(content)
	return sb.String()
}

// isUserAllowed checks if a user is allowed to use the bot.
func (c *Channel) isUserAllowed(userID, chatID int64, username string) bool {
	// If no allow list is configured, allow all
	if len(c.config.AllowFrom) == 0 {
		return true
	}

	// Check allow list
	userIDStr := fmt.Sprintf("%d", userID)
	chatIDStr := fmt.Sprintf("%d", chatID)
	username = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(username)), "@")

	for _, allowed := range c.config.AllowFrom {
		normalizedAllowed := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(allowed)), "@")
		if normalizedAllowed == userIDStr || normalizedAllowed == chatIDStr {
			return true
		}
		if username != "" && normalizedAllowed == username {
			return true
		}
	}

	return false
}

// extractChatID extracts the chat ID from a session ID.
func (c *Channel) extractChatID(sessionID string) (int64, error) {
	// Format: "telegram:123456"
	parts := strings.Split(sessionID, ":")
	if len(parts) != 2 || parts[0] != "telegram" {
		return 0, fmt.Errorf("invalid telegram session ID format")
	}

	var chatID int64
	if _, err := fmt.Sscanf(parts[1], "%d", &chatID); err != nil {
		return 0, fmt.Errorf("invalid chat ID: %w", err)
	}

	return chatID, nil
}

// extractMessageID extracts the message ID from a message reference.
func (c *Channel) extractMessageID(msgRef string) (int, error) {
	// Format: "telegram:123456"
	parts := strings.Split(msgRef, ":")
	if len(parts) != 2 || parts[0] != "telegram" {
		return 0, fmt.Errorf("invalid telegram message reference format")
	}

	var msgID int
	if _, err := fmt.Sscanf(parts[1], "%d", &msgID); err != nil {
		return 0, fmt.Errorf("invalid message ID: %w", err)
	}

	return msgID, nil
}
