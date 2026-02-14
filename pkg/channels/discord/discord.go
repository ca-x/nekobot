// Package discord provides Discord channel implementation.
package discord

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/transcription"
)

// Channel implements Discord channel.
type Channel struct {
	log         *logger.Logger
	config      config.DiscordConfig
	bus         bus.Bus
	commands    *commands.Registry
	transcriber transcription.Transcriber
	httpClient  *http.Client
	session     *discordgo.Session
	running     bool

	pendingSkillMu       sync.Mutex
	pendingSkillInstalls map[string]pendingSkillInstall
}

type pendingSkillInstall struct {
	UserID    string
	ChannelID string
	Command   string
	Repo      string
	CreatedAt time.Time
}

// NewChannel creates a new Discord channel.
func NewChannel(
	log *logger.Logger,
	cfg config.DiscordConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
	transcriber transcription.Transcriber,
) (*Channel, error) {
	// Create Discord session
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("creating discord session: %w", err)
	}

	return &Channel{
		log:         log,
		config:      cfg,
		bus:         b,
		commands:    cmdRegistry,
		transcriber: transcriber,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		session:              session,
		running:              false,
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "discord"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "Discord"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the Discord bot.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting Discord channel")

	// Register message handler
	c.session.AddHandler(c.handleMessage)
	c.session.AddHandler(c.handleInteraction)

	// Register outbound message handler
	c.bus.RegisterHandler("discord", c.handleOutbound)

	// Set intents
	c.session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	// Open WebSocket connection
	if err := c.session.Open(); err != nil {
		return fmt.Errorf("opening discord connection: %w", err)
	}

	c.running = true

	// Get bot user info
	botUser, err := c.session.User("@me")
	if err != nil {
		c.log.Warn("Failed to get bot user", zap.Error(err))
	} else {
		c.log.Info("Discord bot connected",
			zap.String("username", botUser.Username),
			zap.String("user_id", botUser.ID))
	}

	return nil
}

// Stop stops the Discord bot.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Discord channel")
	c.running = false

	// Unregister handler
	c.bus.UnregisterHandlers("discord")

	if c.session != nil {
		if err := c.session.Close(); err != nil {
			return fmt.Errorf("closing discord session: %w", err)
		}
	}

	return nil
}

// handleMessage handles incoming Discord messages.
func (c *Channel) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if user is allowed
	if !c.isAllowed(m.Author.ID) {
		c.log.Warn("Unauthorized user",
			zap.String("user_id", m.Author.ID),
			zap.String("username", m.Author.Username))
		return
	}

	content := strings.TrimSpace(m.Content)
	msgType := bus.MessageTypeText

	// Check if it's a command
	if c.commands.IsCommand(content) {
		c.handleCommand(s, m)
		return
	}

	if content == "" && c.transcriber != nil {
		transcribed, ok := c.transcribeAttachmentAudio(m.Attachments)
		if ok {
			content = transcribed
			msgType = bus.MessageTypeAudio
		}
	}
	if content == "" {
		return
	}

	// Create inbound message
	msg := &bus.Message{
		ID:        fmt.Sprintf("discord:%s", m.ID),
		ChannelID: "discord",
		SessionID: fmt.Sprintf("discord:%s", m.ChannelID),
		UserID:    m.Author.ID,
		Username:  m.Author.Username,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleCommand processes a command message.
func (c *Channel) handleCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	cmdName, args := c.commands.Parse(m.Content)

	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		c.log.Debug("Unknown command", zap.String("command", cmdName))
		return
	}

	c.log.Info("Executing command",
		zap.String("command", cmdName),
		zap.String("user", m.Author.Username))

	// Create command request
	req := commands.CommandRequest{
		Channel:  "discord",
		ChatID:   m.ChannelID,
		UserID:   m.Author.ID,
		Username: m.Author.Username,
		Command:  cmdName,
		Args:     args,
		Metadata: map[string]string{
			"message_id": m.ID,
			"guild_id":   m.GuildID,
		},
	}

	// Execute command
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		c.log.Error("Command execution failed",
			zap.String("command", cmdName),
			zap.Error(err))

		s.ChannelMessageSend(m.ChannelID, "❌ Command failed: "+err.Error())
		return
	}

	// Send response
	if resp.Interaction != nil && resp.Interaction.Type == commands.InteractionTypeSkillInstallConfirm {
		if err := c.sendSkillInstallConfirmation(s, m, cmdName, resp); err != nil {
			c.log.Error("Failed to send interaction response", zap.Error(err))
			_, _ = s.ChannelMessageSend(m.ChannelID, "❌ Failed to create install confirmation: "+err.Error())
		}
		return
	}

	if _, err := s.ChannelMessageSend(m.ChannelID, resp.Content); err != nil {
		c.log.Error("Failed to send command response", zap.Error(err))
	}
}

func (c *Channel) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil || i.Type != discordgo.InteractionMessageComponent {
		return
	}
	data := i.MessageComponentData()
	if !strings.HasPrefix(data.CustomID, "skillinstall:") {
		return
	}

	if i.Message == nil {
		return
	}
	pending, ok := c.getPendingSkillInstall(i.Message.ID)
	if !ok {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This install request has expired. Please run command again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	userID := c.interactionUserID(i)
	if userID == "" || userID != pending.UserID {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Only the requester can confirm this installation.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	switch data.CustomID {
	case "skillinstall:cancel":
		c.clearPendingSkillInstall(i.Message.ID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Installation canceled.",
				Components: []discordgo.MessageComponent{},
			},
		})
	case "skillinstall:confirm":
		result := c.executeConfirmedSkillInstall(s, i, pending)
		c.clearPendingSkillInstall(i.Message.ID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    result,
				Components: []discordgo.MessageComponent{},
			},
		})
	default:
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
	}
}

func (c *Channel) sendSkillInstallConfirmation(s *discordgo.Session, m *discordgo.MessageCreate, cmdName string, resp commands.CommandResponse) error {
	if resp.Interaction == nil {
		return fmt.Errorf("missing interaction payload")
	}
	repo := strings.TrimSpace(resp.Interaction.Repo)
	if repo == "" {
		return fmt.Errorf("missing interaction repo")
	}

	content := strings.TrimSpace(resp.Interaction.Message)
	if content == "" {
		content = strings.TrimSpace(resp.Content)
	}
	if content == "" {
		content = fmt.Sprintf("Found candidate skill `%s`. Confirm install?", repo)
	}
	if reason := strings.TrimSpace(resp.Interaction.Reason); reason != "" {
		content += "\n\nReason: " + reason
	}

	commandName := cmdName
	if custom := strings.TrimSpace(resp.Interaction.Command); custom != "" {
		commandName = custom
	}

	msg, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content: content,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Confirm Install",
						Style:    discordgo.SuccessButton,
						CustomID: "skillinstall:confirm",
					},
					discordgo.Button{
						Label:    "Cancel",
						Style:    discordgo.SecondaryButton,
						CustomID: "skillinstall:cancel",
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	c.setPendingSkillInstall(msg.ID, pendingSkillInstall{
		UserID:    m.Author.ID,
		ChannelID: m.ChannelID,
		Command:   commandName,
		Repo:      repo,
		CreatedAt: time.Now(),
	})
	return nil
}

func (c *Channel) executeConfirmedSkillInstall(s *discordgo.Session, i *discordgo.InteractionCreate, pending pendingSkillInstall) string {
	cmd, exists := c.commands.Get(pending.Command)
	if !exists {
		return "❌ Install failed: command not found."
	}

	req := commands.CommandRequest{
		Channel:  "discord",
		ChatID:   pending.ChannelID,
		UserID:   pending.UserID,
		Username: c.interactionUserName(i),
		Command:  pending.Command,
		Args:     "__confirm_install__ " + pending.Repo,
		Metadata: map[string]string{
			"message_id":                   i.Message.ID,
			"skill_install_confirmed_repo": pending.Repo,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		return "❌ Install failed: " + err.Error()
	}
	if strings.TrimSpace(resp.Content) == "" {
		return "✅ Installation flow executed."
	}
	return resp.Content
}

func (c *Channel) interactionUserID(i *discordgo.InteractionCreate) string {
	if i == nil {
		return ""
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func (c *Channel) interactionUserName(i *discordgo.InteractionCreate) string {
	if i == nil {
		return ""
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.Username
	}
	if i.User != nil {
		return i.User.Username
	}
	return ""
}

func (c *Channel) setPendingSkillInstall(messageID string, pending pendingSkillInstall) {
	c.pendingSkillMu.Lock()
	defer c.pendingSkillMu.Unlock()
	c.pendingSkillInstalls[messageID] = pending
}

func (c *Channel) getPendingSkillInstall(messageID string) (pendingSkillInstall, bool) {
	c.pendingSkillMu.Lock()
	defer c.pendingSkillMu.Unlock()

	pending, ok := c.pendingSkillInstalls[messageID]
	if !ok {
		return pendingSkillInstall{}, false
	}
	if time.Since(pending.CreatedAt) > 15*time.Minute {
		delete(c.pendingSkillInstalls, messageID)
		return pendingSkillInstall{}, false
	}
	return pending, true
}

func (c *Channel) clearPendingSkillInstall(messageID string) {
	c.pendingSkillMu.Lock()
	defer c.pendingSkillMu.Unlock()
	delete(c.pendingSkillInstalls, messageID)
}

// handleOutbound handles outbound messages from the bus.
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to Discord.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	if c.session == nil {
		return fmt.Errorf("session not initialized")
	}

	// Extract channel ID from session ID (format: "discord:channel_id")
	channelID := msg.SessionID
	if len(channelID) > 8 && channelID[:8] == "discord:" {
		channelID = channelID[8:]
	}

	// Send message
	_, err := c.session.ChannelMessageSend(channelID, msg.Content)
	if err != nil {
		return fmt.Errorf("sending discord message: %w", err)
	}

	c.log.Debug("Sent Discord message",
		zap.String("channel_id", channelID),
		zap.Int("length", len(msg.Content)))

	return nil
}

// isAllowed checks if a user is allowed to use the bot.
func (c *Channel) isAllowed(userID string) bool {
	if len(c.config.AllowFrom) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowFrom {
		if allowed == userID || allowed == "*" {
			return true
		}
	}

	return false
}

func (c *Channel) transcribeAttachmentAudio(attachments []*discordgo.MessageAttachment) (string, bool) {
	for _, att := range attachments {
		if att == nil || att.URL == "" {
			continue
		}
		contentType := strings.ToLower(att.ContentType)
		ext := strings.ToLower(filepath.Ext(att.Filename))
		if !strings.HasPrefix(contentType, "audio/") &&
			ext != ".ogg" && ext != ".mp3" && ext != ".wav" && ext != ".m4a" && ext != ".webm" {
			continue
		}

		req, err := http.NewRequest(http.MethodGet, att.URL, nil)
		if err != nil {
			continue
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.log.Warn("Failed to download Discord audio", zap.Error(err))
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024))
		resp.Body.Close()
		if err != nil {
			c.log.Warn("Failed reading Discord audio", zap.Error(err))
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		text, err := c.transcriber.Transcribe(ctx, data, att.Filename)
		cancel()
		if err != nil {
			c.log.Warn("Discord audio transcription failed", zap.Error(err))
			continue
		}
		text = strings.TrimSpace(text)
		if text != "" {
			return text, true
		}
	}
	return "", false
}
