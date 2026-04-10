// Package slack provides Slack channel implementation.
package slack

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/channeltrace"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/transcription"
)

type slackAPI interface {
	AuthTest() (*slack.AuthTestResponse, error)
	PostEphemeral(channelID, userID string, options ...slack.MsgOption) (string, error)
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)
	PostMessageContext(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error)
	OpenView(triggerID string, view slack.ModalViewRequest) (*slack.ViewResponse, error)
	UpdateMessage(channelID, timestamp string, options ...slack.MsgOption) (string, string, string, error)
}

// Channel implements Slack channel using Socket Mode.
type Channel struct {
	log                  *logger.Logger
	config               config.SlackConfig
	bus                  bus.Bus
	commands             *commands.Registry
	api                  slackAPI
	socketClient         *socketmode.Client
	botUserID            string
	running              bool
	ctx                  context.Context
	cancel               context.CancelFunc
	transcriber          transcription.Transcriber
	httpClient           *http.Client
	id                   string
	channelType          string
	name                 string
	pendingSkillMu       sync.Mutex
	pendingSkillInstalls map[string]pendingSkillInstall
}

type pendingSkillInstall struct {
	UserID      string
	ChannelID   string
	ThreadTS    string
	MessageTS   string
	Command     string
	Repo        string
	CreatedAt   time.Time
	ResponseURL string
}

const (
	findSkillsShortcutCallbackID = "find_skills"
	findSkillsModalCallbackID    = "find_skills_modal"
	findSkillsModalBlockID       = "skill_query"
	findSkillsModalActionID      = "query_input"
	startShortcutCallbackID      = "start"
	startModalCallbackID         = "start_modal"
	helpShortcutCallbackID       = "help"
	helpModalCallbackID          = "help_modal"
	helpModalBlockID             = "help_query"
	helpModalActionID            = "help_query_input"
	statusShortcutCallbackID     = "status"
	statusModalCallbackID        = "status_modal"
	agentShortcutCallbackID      = "agent"
	agentModalCallbackID         = "agent_modal"
	agentModalBlockID            = "agent_query"
	agentModalActionID           = "agent_query_input"
	modelShortcutCallbackID      = "model"
	modelModalCallbackID         = "model_modal"
	modelModalBlockID            = "model_query"
	modelModalActionID           = "model_query_input"
	settingsShortcutCallbackID   = "settings"
	settingsModalCallbackID      = "settings_modal"
	settingsActionBlockID        = "settings_action"
	settingsActionInputID        = "settings_action_input"
	settingsValueBlockID         = "settings_value"
	settingsValueInputID         = "settings_value_input"
)

// NewChannel creates a new Slack channel.
func NewChannel(
	log *logger.Logger,
	cfg config.SlackConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
	transcriber transcription.Transcriber,
) (*Channel, error) {
	return NewAccountChannel(log, cfg, b, cmdRegistry, transcriber, "slack", "Slack")
}

// NewAccountChannel creates an account-scoped Slack channel instance.
func NewAccountChannel(
	log *logger.Logger,
	cfg config.SlackConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
	transcriber transcription.Transcriber,
	channelID string,
	displayName string,
) (*Channel, error) {
	if cfg.BotToken == "" || cfg.AppToken == "" {
		return nil, fmt.Errorf("slack bot_token and app_token are required")
	}

	// Create API client
	api := slack.New(
		cfg.BotToken,
		slack.OptionAppLevelToken(cfg.AppToken),
	)

	// Create Socket Mode client
	socketClient := socketmode.New(api)

	return &Channel{
		log:          log,
		config:       cfg,
		bus:          b,
		commands:     cmdRegistry,
		api:          api,
		socketClient: socketClient,
		running:      false,
		transcriber:  transcriber,
		id:           strings.TrimSpace(channelID),
		channelType:  "slack",
		name:         defaultSlackName(displayName),
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		pendingSkillInstalls: map[string]pendingSkillInstall{},
	}, nil
}

// Start starts the Slack bot.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting Slack channel (Socket Mode)")

	c.ctx, c.cancel = context.WithCancel(ctx)

	authResp, err := c.authTest()
	if err != nil {
		return err
	}
	c.botUserID = authResp.UserID

	c.log.Info("Slack bot connected",
		zap.String("bot_user_id", c.botUserID),
		zap.String("team", authResp.Team))

	// Start event loop
	go c.eventLoop()

	// Run Socket Mode client
	go func() {
		if err := c.socketClient.RunContext(c.ctx); err != nil {
			if c.ctx.Err() == nil {
				c.log.Error("Socket Mode connection error", zap.Error(err))
			}
		}
	}()

	c.running = true
	c.log.Info("Slack channel started")

	return nil
}

// HealthCheck verifies that Slack credentials are accepted by the API.
func (c *Channel) HealthCheck(ctx context.Context) error {
	_, err := c.authTest()
	return err
}

// Stop stops the Slack bot.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Slack channel")

	if c.cancel != nil {
		c.cancel()
	}

	c.running = false
	c.log.Info("Slack channel stopped")
	return nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return c.id
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return c.name
}

// ChannelType returns the stable Slack family key.
func (c *Channel) ChannelType() string {
	return c.channelType
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// IsRunning returns whether the channel is running.
func (c *Channel) IsRunning() bool {
	return c.running
}

func (c *Channel) authTest() (*slack.AuthTestResponse, error) {
	authResp, err := c.api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("slack auth test failed: %w", err)
	}
	return authResp, nil
}

// eventLoop processes Socket Mode events.
func (c *Channel) eventLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case evt := <-c.socketClient.Events:
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				c.handleEventsAPI(evt)
			case socketmode.EventTypeSlashCommand:
				c.handleSlashCommand(evt)
			case socketmode.EventTypeInteractive:
				c.handleInteractive(evt)
			}
		}
	}
}

// handleEventsAPI handles Events API events.
func (c *Channel) handleEventsAPI(evt socketmode.Event) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		c.log.Warn("Failed to parse Events API event")
		c.socketClient.Ack(*evt.Request)
		return
	}

	// Acknowledge the event
	c.socketClient.Ack(*evt.Request)

	// Handle inner event
	switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		c.handleMessageEvent(ev)
	case *slackevents.AppMentionEvent:
		c.handleAppMentionEvent(ev)
	}
}

// handleMessageEvent handles message events.
func (c *Channel) handleMessageEvent(ev *slackevents.MessageEvent) {
	// Ignore bot messages
	if ev.BotID != "" || ev.User == c.botUserID {
		return
	}

	// Check if user is allowed
	if !c.isAllowed(ev.User) {
		c.log.Warn("Unauthorized user",
			zap.String("user_id", ev.User))
		return
	}

	content := strings.TrimSpace(ev.Text)
	msgType := bus.MessageTypeText

	if content == "" && c.transcriber != nil && ev.Message != nil && len(ev.Message.Files) > 0 {
		if transcribed, ok := c.transcribeFiles(ev.Message.Files); ok {
			content = transcribed
			msgType = bus.MessageTypeAudio
		}
	}
	if content == "" {
		return
	}

	// Determine chat ID (channel_id or channel_id:thread_ts)
	sessionID := c.sessionID(ev.Channel)
	if ev.ThreadTimeStamp != "" {
		sessionID = c.sessionThreadID(ev.Channel, ev.ThreadTimeStamp)
	}

	// Create inbound message
	msg := &bus.Message{
		ID:        fmt.Sprintf("slack:%s", ev.TimeStamp),
		ChannelID: c.ID(),
		SessionID: sessionID,
		UserID:    ev.User,
		Username:  ev.User, // Slack uses user ID
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleAppMentionEvent handles app mention events.
func (c *Channel) handleAppMentionEvent(ev *slackevents.AppMentionEvent) {
	// Check if user is allowed
	if !c.isAllowed(ev.User) {
		c.log.Warn("Unauthorized user",
			zap.String("user_id", ev.User))
		return
	}

	// Determine session ID
	sessionID := c.sessionID(ev.Channel)
	if ev.ThreadTimeStamp != "" {
		sessionID = c.sessionThreadID(ev.Channel, ev.ThreadTimeStamp)
	}

	// Remove bot mention from text
	text := strings.TrimSpace(strings.Replace(ev.Text, fmt.Sprintf("<@%s>", c.botUserID), "", 1))

	// Create inbound message
	msg := &bus.Message{
		ID:        fmt.Sprintf("slack:%s", ev.TimeStamp),
		ChannelID: c.ID(),
		SessionID: sessionID,
		UserID:    ev.User,
		Username:  ev.User,
		Type:      bus.MessageTypeText,
		Content:   text,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleSlashCommand handles slash commands.
func (c *Channel) handleSlashCommand(evt socketmode.Event) {
	cmd, ok := evt.Data.(slack.SlashCommand)
	if !ok {
		if c.socketClient != nil && evt.Request != nil {
			c.socketClient.Ack(*evt.Request)
		}
		return
	}

	// Acknowledge the event
	if c.socketClient != nil && evt.Request != nil {
		c.socketClient.Ack(*evt.Request)
	}

	c.log.Debug("Received slash command",
		zap.String("command", cmd.Command),
		zap.String("text", cmd.Text))

	// Remove leading / from command name
	cmdName := strings.TrimPrefix(cmd.Command, "/")

	// Get command from registry
	command, exists := c.commands.Get(cmdName)
	if !exists {
		c.log.Debug("Unknown slash command, fallback to help", zap.String("command", cmdName))
		helpCmd, ok := c.commands.Get("help")
		if !ok {
			return
		}
		command = helpCmd
		cmdName = "help"
		cmd.Text = ""
	}
	if command.AdminOnly {
		if _, err := c.api.PostEphemeral(
			cmd.ChannelID,
			cmd.UserID,
			slack.MsgOptionText("❌ This command is only available to admins.", false),
		); err != nil {
			c.log.Error("Failed to send Slack admin-only rejection", zap.Error(err))
		}
		return
	}

	// Create command request
	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   cmd.ChannelID,
		UserID:   cmd.UserID,
		Username: cmd.UserName,
		Command:  cmdName,
		Args:     cmd.Text,
		Metadata: map[string]string{
			"channel_name": cmd.ChannelName,
			"team_id":      cmd.TeamID,
			"team_domain":  cmd.TeamDomain,
			"trigger_id":   cmd.TriggerID,
			"runtime_id":   c.ID(),
		},
	}

	// Execute command
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	resp, err := command.Handler(ctx, req)
	if err != nil {
		c.log.Error("Slash command execution failed",
			zap.String("command", cmdName),
			zap.Error(err))

		// Send error as ephemeral message
		if _, sendErr := c.api.PostEphemeral(cmd.ChannelID, cmd.UserID,
			slack.MsgOptionText("❌ Command failed: "+err.Error(), false)); sendErr != nil {
			c.log.Error("Failed to send Slack command error", zap.Error(sendErr))
		}
		return
	}

	if resp.Interaction != nil && resp.Interaction.Type == commands.InteractionTypeSkillInstallConfirm {
		if err := c.sendSkillInstallConfirmation(cmd, cmdName, resp); err != nil {
			c.log.Error("Failed to send skill install confirmation", zap.Error(err))
			if _, sendErr := c.api.PostEphemeral(cmd.ChannelID, cmd.UserID,
				slack.MsgOptionText("Failed to create install confirmation: "+err.Error(), false)); sendErr != nil {
				c.log.Error("Failed to send Slack install confirmation error", zap.Error(sendErr))
			}
		}
		return
	}

	// Send response
	opts := []slack.MsgOption{
		slack.MsgOptionText(resp.Content, false),
	}

	if resp.Ephemeral {
		// Send as ephemeral message (only visible to user)
		if _, err := c.api.PostEphemeral(cmd.ChannelID, cmd.UserID, opts...); err != nil {
			c.log.Error("Failed to send Slack ephemeral response", zap.Error(err))
		}
	} else {
		// Send as regular message
		if _, _, err := c.api.PostMessage(cmd.ChannelID, opts...); err != nil {
			c.log.Error("Failed to send Slack response", zap.Error(err))
		}
	}
}

// handleInteractive handles interactive components.
func (c *Channel) handleInteractive(evt socketmode.Event) {
	callback, ok := evt.Data.(slack.InteractionCallback)
	if !ok {
		c.socketClient.Ack(*evt.Request)
		return
	}

	c.socketClient.Ack(*evt.Request)

	c.log.Debug("Received interactive callback",
		zap.String("type", string(callback.Type)))

	switch callback.Type {
	case slack.InteractionTypeBlockActions:
		c.handleBlockActions(callback)
	case slack.InteractionTypeShortcut:
		c.handleShortcut(callback)
	case slack.InteractionTypeViewSubmission:
		c.handleViewSubmission(callback)
	default:
		c.log.Debug("Unhandled interaction type", zap.String("type", string(callback.Type)))
	}
}

// handleBlockActions processes button clicks and other block actions.
func (c *Channel) handleBlockActions(callback slack.InteractionCallback) {
	for _, action := range callback.ActionCallback.BlockActions {
		c.log.Debug("Block action received",
			zap.String("action_id", action.ActionID),
			zap.String("value", action.Value))

		switch {
		case strings.HasPrefix(action.ActionID, "skill_install_confirm:"):
			repo := strings.TrimPrefix(action.ActionID, "skill_install_confirm:")
			c.handleSkillInstallConfirm(callback, repo)
		case action.ActionID == "skill_install_cancel":
			c.handleSkillInstallCancel(callback)
		default:
			c.log.Debug("Unknown action", zap.String("action_id", action.ActionID))
		}
	}
}

// handleSkillInstallConfirm handles skill install confirmation button clicks.
func (c *Channel) handleSkillInstallConfirm(callback slack.InteractionCallback, repo string) {
	if repo == "" {
		return
	}
	messageTS := c.interactionMessageTS(callback)
	pending, ok := c.getPendingSkillInstall(messageTS)
	if !ok {
		if _, err := c.api.PostEphemeral(callback.Channel.ID, callback.User.ID,
			slack.MsgOptionText("This install request has expired. Please run the command again.", false)); err != nil {
			c.log.Error("Failed to send Slack interaction expiry message", zap.Error(err))
		}
		return
	}
	if callback.User.ID != pending.UserID {
		if _, err := c.api.PostEphemeral(callback.Channel.ID, callback.User.ID,
			slack.MsgOptionText("Only the requester can confirm this installation.", false)); err != nil {
			c.log.Error("Failed to send Slack interaction authorization message", zap.Error(err))
		}
		return
	}
	if repo != pending.Repo {
		c.log.Warn("Skill install repo mismatch",
			zap.String("expected_repo", pending.Repo),
			zap.String("received_repo", repo),
		)
	}

	result := c.executeConfirmedSkillInstall(callback, pending)
	c.clearPendingSkillInstall(messageTS)
	c.updateInteractionMessage(pending, result)
}

func (c *Channel) handleSkillInstallCancel(callback slack.InteractionCallback) {
	messageTS := c.interactionMessageTS(callback)
	pending, ok := c.getPendingSkillInstall(messageTS)
	if !ok {
		if _, err := c.api.PostEphemeral(callback.Channel.ID, callback.User.ID,
			slack.MsgOptionText("This install request has expired. Please run the command again.", false)); err != nil {
			c.log.Error("Failed to send Slack cancel expiry message", zap.Error(err))
		}
		return
	}
	if callback.User.ID != pending.UserID {
		if _, err := c.api.PostEphemeral(callback.Channel.ID, callback.User.ID,
			slack.MsgOptionText("Only the requester can cancel this installation.", false)); err != nil {
			c.log.Error("Failed to send Slack unauthorized cancel message", zap.Error(err))
		}
		return
	}

	c.clearPendingSkillInstall(messageTS)
	c.updateInteractionMessage(pending, "Installation cancelled.")
}

func (c *Channel) handleShortcut(callback slack.InteractionCallback) {
	c.log.Debug("Shortcut interaction received",
		zap.String("callback_id", callback.CallbackID),
		zap.String("user_id", callback.User.ID))

	if callback.TriggerID == "" {
		c.log.Warn("Slack shortcut missing trigger id", zap.String("callback_id", callback.CallbackID))
		return
	}

	var view slack.ModalViewRequest
	switch callback.CallbackID {
	case findSkillsShortcutCallbackID:
		view = buildFindSkillsModal()
	case startShortcutCallbackID:
		view = buildStartModal()
	case helpShortcutCallbackID:
		view = buildHelpModal()
	case statusShortcutCallbackID:
		view = buildStatusModal()
	case agentShortcutCallbackID:
		view = buildAgentModal()
	case modelShortcutCallbackID:
		view = buildModelModal()
	case settingsShortcutCallbackID:
		view = buildSettingsModal()
	default:
		return
	}

	if _, err := c.api.OpenView(callback.TriggerID, view); err != nil {
		c.log.Error("Failed to open Slack shortcut modal",
			zap.String("callback_id", callback.CallbackID),
			zap.Error(err),
		)
	}
}

func (c *Channel) handleViewSubmission(callback slack.InteractionCallback) {
	c.log.Debug("View submission received",
		zap.String("callback_id", callback.View.CallbackID),
		zap.String("user_id", callback.User.ID))

	switch callback.View.CallbackID {
	case findSkillsModalCallbackID:
		c.handleFindSkillsViewSubmission(callback)
	case startModalCallbackID:
		c.handleStartViewSubmission(callback)
	case helpModalCallbackID:
		c.handleHelpViewSubmission(callback)
	case statusModalCallbackID:
		c.handleStatusViewSubmission(callback)
	case agentModalCallbackID:
		c.handleAgentViewSubmission(callback)
	case modelModalCallbackID:
		c.handleModelViewSubmission(callback)
	case settingsModalCallbackID:
		c.handleSettingsViewSubmission(callback)
	default:
		return
	}
}

func (c *Channel) handleFindSkillsViewSubmission(callback slack.InteractionCallback) {
	commandName := strings.TrimSpace(callback.View.PrivateMetadata)
	if commandName == "" {
		commandName = "find-skills"
	}

	query := strings.TrimSpace(readViewInputValue(
		callback.View.State,
		findSkillsModalBlockID,
		findSkillsModalActionID,
	))
	if query == "" {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("Please provide a search query.", false),
		); err != nil {
			c.log.Error("Failed to send Slack modal validation message", zap.Error(err))
		}
		return
	}

	cmd, exists := c.commands.Get(commandName)
	if !exists {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command not found: "+commandName, false),
		); err != nil {
			c.log.Error("Failed to send Slack modal command-not-found message", zap.Error(err))
		}
		return
	}

	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   callback.Channel.ID,
		UserID:   callback.User.ID,
		Username: callback.User.Name,
		Command:  commandName,
		Args:     query,
		Metadata: map[string]string{
			"team_id":    callback.Team.ID,
			"trigger_id": callback.TriggerID,
			"runtime_id": c.ID(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		if _, sendErr := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command failed: "+err.Error(), false),
		); sendErr != nil {
			c.log.Error("Failed to send Slack modal command error", zap.Error(sendErr))
		}
		return
	}

	if resp.Interaction != nil && resp.Interaction.Type == commands.InteractionTypeSkillInstallConfirm {
		cmd := slack.SlashCommand{
			ChannelID: callback.Channel.ID,
			UserID:    callback.User.ID,
		}
		if err := c.sendSkillInstallConfirmation(cmd, commandName, resp); err != nil {
			c.log.Error("Failed to send Slack modal skill install confirmation", zap.Error(err))
			if _, sendErr := c.api.PostEphemeral(
				callback.Channel.ID,
				callback.User.ID,
				slack.MsgOptionText("Failed to create install confirmation: "+err.Error(), false),
			); sendErr != nil {
				c.log.Error("Failed to send Slack modal confirmation error", zap.Error(sendErr))
			}
		}
		return
	}

	if strings.TrimSpace(resp.Content) == "" {
		return
	}

	if _, err := c.api.PostEphemeral(
		callback.Channel.ID,
		callback.User.ID,
		slack.MsgOptionText(resp.Content, false),
	); err != nil {
		c.log.Error("Failed to send Slack modal response", zap.Error(err))
	}
}

func (c *Channel) handleStartViewSubmission(callback slack.InteractionCallback) {
	commandName := strings.TrimSpace(callback.View.PrivateMetadata)
	if commandName == "" {
		commandName = "start"
	}

	cmd, exists := c.commands.Get(commandName)
	if !exists {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command not found: "+commandName, false),
		); err != nil {
			c.log.Error("Failed to send Slack start modal command-not-found message", zap.Error(err))
		}
		return
	}

	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   callback.Channel.ID,
		UserID:   callback.User.ID,
		Username: callback.User.Name,
		Command:  commandName,
		Metadata: map[string]string{
			"team_id":    callback.Team.ID,
			"trigger_id": callback.TriggerID,
			"runtime_id": c.ID(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		if _, sendErr := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command failed: "+err.Error(), false),
		); sendErr != nil {
			c.log.Error("Failed to send Slack start modal command error", zap.Error(sendErr))
		}
		return
	}

	if strings.TrimSpace(resp.Content) == "" {
		return
	}

	if _, err := c.api.PostEphemeral(
		callback.Channel.ID,
		callback.User.ID,
		slack.MsgOptionText(resp.Content, false),
	); err != nil {
		c.log.Error("Failed to send Slack start modal response", zap.Error(err))
	}
}

func (c *Channel) handleHelpViewSubmission(callback slack.InteractionCallback) {
	commandName := strings.TrimSpace(callback.View.PrivateMetadata)
	if commandName == "" {
		commandName = "help"
	}

	query := strings.TrimSpace(readViewInputValue(
		callback.View.State,
		helpModalBlockID,
		helpModalActionID,
	))
	if query == "" {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("Please provide a command name or topic.", false),
		); err != nil {
			c.log.Error("Failed to send Slack help modal validation message", zap.Error(err))
		}
		return
	}

	cmd, exists := c.commands.Get(commandName)
	if !exists {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command not found: "+commandName, false),
		); err != nil {
			c.log.Error("Failed to send Slack help modal command-not-found message", zap.Error(err))
		}
		return
	}

	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   callback.Channel.ID,
		UserID:   callback.User.ID,
		Username: callback.User.Name,
		Command:  commandName,
		Args:     query,
		Metadata: map[string]string{
			"team_id":    callback.Team.ID,
			"trigger_id": callback.TriggerID,
			"runtime_id": c.ID(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		if _, sendErr := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command failed: "+err.Error(), false),
		); sendErr != nil {
			c.log.Error("Failed to send Slack help modal command error", zap.Error(sendErr))
		}
		return
	}

	if strings.TrimSpace(resp.Content) == "" {
		return
	}

	if _, err := c.api.PostEphemeral(
		callback.Channel.ID,
		callback.User.ID,
		slack.MsgOptionText(resp.Content, false),
	); err != nil {
		c.log.Error("Failed to send Slack help modal response", zap.Error(err))
	}
}

func (c *Channel) handleStatusViewSubmission(callback slack.InteractionCallback) {
	commandName := strings.TrimSpace(callback.View.PrivateMetadata)
	if commandName == "" {
		commandName = "status"
	}

	cmd, exists := c.commands.Get(commandName)
	if !exists {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command not found: "+commandName, false),
		); err != nil {
			c.log.Error("Failed to send Slack status modal command-not-found message", zap.Error(err))
		}
		return
	}

	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   callback.Channel.ID,
		UserID:   callback.User.ID,
		Username: callback.User.Name,
		Command:  commandName,
		Metadata: map[string]string{
			"team_id":    callback.Team.ID,
			"trigger_id": callback.TriggerID,
			"runtime_id": c.ID(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		if _, sendErr := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command failed: "+err.Error(), false),
		); sendErr != nil {
			c.log.Error("Failed to send Slack status modal command error", zap.Error(sendErr))
		}
		return
	}

	if strings.TrimSpace(resp.Content) == "" {
		return
	}

	if _, err := c.api.PostEphemeral(
		callback.Channel.ID,
		callback.User.ID,
		slack.MsgOptionText(resp.Content, false),
	); err != nil {
		c.log.Error("Failed to send Slack status modal response", zap.Error(err))
	}
}

func (c *Channel) handleAgentViewSubmission(callback slack.InteractionCallback) {
	commandName := strings.TrimSpace(callback.View.PrivateMetadata)
	if commandName == "" {
		commandName = "agent"
	}

	query := strings.TrimSpace(readViewInputValue(
		callback.View.State,
		agentModalBlockID,
		agentModalActionID,
	))
	if query == "" {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("Please provide an agent action, for example `info` or `list`.", false),
		); err != nil {
			c.log.Error("Failed to send Slack agent modal validation message", zap.Error(err))
		}
		return
	}

	cmd, exists := c.commands.Get(commandName)
	if !exists {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command not found: "+commandName, false),
		); err != nil {
			c.log.Error("Failed to send Slack agent modal command-not-found message", zap.Error(err))
		}
		return
	}

	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   callback.Channel.ID,
		UserID:   callback.User.ID,
		Username: callback.User.Name,
		Command:  commandName,
		Args:     query,
		Metadata: map[string]string{
			"team_id":    callback.Team.ID,
			"trigger_id": callback.TriggerID,
			"runtime_id": c.ID(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		if _, sendErr := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command failed: "+err.Error(), false),
		); sendErr != nil {
			c.log.Error("Failed to send Slack agent modal command error", zap.Error(sendErr))
		}
		return
	}

	if strings.TrimSpace(resp.Content) == "" {
		return
	}

	if _, err := c.api.PostEphemeral(
		callback.Channel.ID,
		callback.User.ID,
		slack.MsgOptionText(resp.Content, false),
	); err != nil {
		c.log.Error("Failed to send Slack agent modal response", zap.Error(err))
	}
}

func (c *Channel) handleSettingsViewSubmission(callback slack.InteractionCallback) {
	commandName := strings.TrimSpace(callback.View.PrivateMetadata)
	if commandName == "" {
		commandName = "settings"
	}

	action := strings.TrimSpace(readViewInputValue(
		callback.View.State,
		settingsActionBlockID,
		settingsActionInputID,
	))
	value := strings.TrimSpace(readViewInputValue(
		callback.View.State,
		settingsValueBlockID,
		settingsValueInputID,
	))
	args := strings.TrimSpace(action)
	if value != "" {
		args = strings.TrimSpace(action + " " + value)
	}
	if args == "" {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("Please provide a settings action.", false),
		); err != nil {
			c.log.Error("Failed to send Slack settings modal validation message", zap.Error(err))
		}
		return
	}

	cmd, exists := c.commands.Get(commandName)
	if !exists {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command not found: "+commandName, false),
		); err != nil {
			c.log.Error("Failed to send Slack settings modal command-not-found message", zap.Error(err))
		}
		return
	}

	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   callback.Channel.ID,
		UserID:   callback.User.ID,
		Username: callback.User.Name,
		Command:  commandName,
		Args:     args,
		Metadata: map[string]string{
			"team_id":    callback.Team.ID,
			"trigger_id": callback.TriggerID,
			"runtime_id": c.ID(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		if _, sendErr := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command failed: "+err.Error(), false),
		); sendErr != nil {
			c.log.Error("Failed to send Slack settings modal command error", zap.Error(sendErr))
		}
		return
	}

	if strings.TrimSpace(resp.Content) == "" {
		return
	}

	if _, err := c.api.PostEphemeral(
		callback.Channel.ID,
		callback.User.ID,
		slack.MsgOptionText(resp.Content, false),
	); err != nil {
		c.log.Error("Failed to send Slack settings modal response", zap.Error(err))
	}
}

func (c *Channel) handleModelViewSubmission(callback slack.InteractionCallback) {
	commandName := strings.TrimSpace(callback.View.PrivateMetadata)
	if commandName == "" {
		commandName = "model"
	}

	query := strings.TrimSpace(readViewInputValue(
		callback.View.State,
		modelModalBlockID,
		modelModalActionID,
	))
	if query == "" {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("Please provide a provider name or `list`.", false),
		); err != nil {
			c.log.Error("Failed to send Slack model modal validation message", zap.Error(err))
		}
		return
	}

	cmd, exists := c.commands.Get(commandName)
	if !exists {
		if _, err := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command not found: "+commandName, false),
		); err != nil {
			c.log.Error("Failed to send Slack model modal command-not-found message", zap.Error(err))
		}
		return
	}

	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   callback.Channel.ID,
		UserID:   callback.User.ID,
		Username: callback.User.Name,
		Command:  commandName,
		Args:     query,
		Metadata: map[string]string{
			"team_id":    callback.Team.ID,
			"trigger_id": callback.TriggerID,
			"runtime_id": c.ID(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		if _, sendErr := c.api.PostEphemeral(
			callback.Channel.ID,
			callback.User.ID,
			slack.MsgOptionText("❌ Command failed: "+err.Error(), false),
		); sendErr != nil {
			c.log.Error("Failed to send Slack model modal command error", zap.Error(sendErr))
		}
		return
	}

	if strings.TrimSpace(resp.Content) == "" {
		return
	}

	if _, err := c.api.PostEphemeral(
		callback.Channel.ID,
		callback.User.ID,
		slack.MsgOptionText(resp.Content, false),
	); err != nil {
		c.log.Error("Failed to send Slack model modal response", zap.Error(err))
	}
}

func buildFindSkillsModal() slack.ModalViewRequest {
	queryPlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "Search skills", false, false)
	queryLabel := slack.NewTextBlockObject(slack.PlainTextType, "What do you want to install?", false, false)
	queryHint := slack.NewTextBlockObject(
		slack.PlainTextType,
		"Describe the capability you need, for example: qmd memory search",
		false,
		false,
	)

	queryInput := slack.NewPlainTextInputBlockElement(queryPlaceholder, findSkillsModalActionID)
	queryBlock := slack.NewInputBlock(findSkillsModalBlockID, queryLabel, queryHint, queryInput)

	return slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      findSkillsModalCallbackID,
		PrivateMetadata: "find-skills",
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Find Skills", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Search", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		Blocks:          slack.Blocks{BlockSet: []slack.Block{queryBlock}},
	}
}

func buildStartModal() slack.ModalViewRequest {
	startText := slack.NewTextBlockObject(
		slack.PlainTextType,
		"Run the current /start command and show the welcome message as an ephemeral reply.",
		false,
		false,
	)
	startSection := slack.NewSectionBlock(startText, nil, nil)

	return slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      startModalCallbackID,
		PrivateMetadata: "start",
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Start", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Run", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		Blocks:          slack.Blocks{BlockSet: []slack.Block{startSection}},
	}
}

func buildHelpModal() slack.ModalViewRequest {
	queryPlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "status | help find-skills", false, false)
	queryLabel := slack.NewTextBlockObject(slack.PlainTextType, "Command or topic", false, false)
	queryHint := slack.NewTextBlockObject(
		slack.PlainTextType,
		"Examples: status, find-skills, settings",
		false,
		false,
	)

	queryInput := slack.NewPlainTextInputBlockElement(queryPlaceholder, helpModalActionID)
	queryBlock := slack.NewInputBlock(helpModalBlockID, queryLabel, queryHint, queryInput)

	return slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      helpModalCallbackID,
		PrivateMetadata: "help",
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Help", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Run", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		Blocks:          slack.Blocks{BlockSet: []slack.Block{queryBlock}},
	}
}

func buildStatusModal() slack.ModalViewRequest {
	statusText := slack.NewTextBlockObject(
		slack.PlainTextType,
		"Run the current /status command and show the result as an ephemeral reply.",
		false,
		false,
	)
	statusSection := slack.NewSectionBlock(statusText, nil, nil)

	return slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      statusModalCallbackID,
		PrivateMetadata: "status",
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Status", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Run", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		Blocks:          slack.Blocks{BlockSet: []slack.Block{statusSection}},
	}
}

func buildAgentModal() slack.ModalViewRequest {
	queryPlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "info | list", false, false)
	queryLabel := slack.NewTextBlockObject(slack.PlainTextType, "Agent action", false, false)
	queryHint := slack.NewTextBlockObject(
		slack.PlainTextType,
		"Examples: info, list",
		false,
		false,
	)

	queryInput := slack.NewPlainTextInputBlockElement(queryPlaceholder, agentModalActionID)
	queryBlock := slack.NewInputBlock(agentModalBlockID, queryLabel, queryHint, queryInput)

	return slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      agentModalCallbackID,
		PrivateMetadata: "agent",
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Agent", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Run", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		Blocks:          slack.Blocks{BlockSet: []slack.Block{queryBlock}},
	}
}

func buildModelModal() slack.ModalViewRequest {
	queryPlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "openai | anthropic | list", false, false)
	queryLabel := slack.NewTextBlockObject(slack.PlainTextType, "Provider or action", false, false)
	queryHint := slack.NewTextBlockObject(
		slack.PlainTextType,
		"Examples: openai, anthropic, list",
		false,
		false,
	)

	queryInput := slack.NewPlainTextInputBlockElement(queryPlaceholder, modelModalActionID)
	queryBlock := slack.NewInputBlock(modelModalBlockID, queryLabel, queryHint, queryInput)

	return slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      modelModalCallbackID,
		PrivateMetadata: "model",
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Model", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Run", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		Blocks:          slack.Blocks{BlockSet: []slack.Block{queryBlock}},
	}
}

func buildSettingsModal() slack.ModalViewRequest {
	actionPlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "lang | name | prefs | skillmode | show", false, false)
	actionLabel := slack.NewTextBlockObject(slack.PlainTextType, "Action", false, false)
	actionHint := slack.NewTextBlockObject(
		slack.PlainTextType,
		"Examples: lang, name, prefs, skillmode, show",
		false,
		false,
	)
	actionInput := slack.NewPlainTextInputBlockElement(actionPlaceholder, settingsActionInputID)
	actionBlock := slack.NewInputBlock(settingsActionBlockID, actionLabel, actionHint, actionInput)

	valuePlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "en | Alice | concise replies | npx", false, false)
	valueLabel := slack.NewTextBlockObject(slack.PlainTextType, "Value", false, false)
	valueHint := slack.NewTextBlockObject(
		slack.PlainTextType,
		"Optional for show. Examples: en, Alice, concise replies, npx",
		false,
		false,
	)
	valueInput := slack.NewPlainTextInputBlockElement(valuePlaceholder, settingsValueInputID)
	valueBlock := slack.NewInputBlock(settingsValueBlockID, valueLabel, valueHint, valueInput).WithOptional(true)

	return slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      settingsModalCallbackID,
		PrivateMetadata: "settings",
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Settings", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Save", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		Blocks:          slack.Blocks{BlockSet: []slack.Block{actionBlock, valueBlock}},
	}
}

func readViewInputValue(state *slack.ViewState, blockID, actionID string) string {
	if state == nil {
		return ""
	}
	blockValues, ok := state.Values[blockID]
	if !ok {
		return ""
	}
	value, ok := blockValues[actionID]
	if !ok {
		return ""
	}
	return strings.TrimSpace(value.Value)
}

func (c *Channel) sendSkillInstallConfirmation(
	cmd slack.SlashCommand,
	cmdName string,
	resp commands.CommandResponse,
) error {
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

	confirmButton := slack.NewButtonBlockElement(
		"skill_install_confirm:"+repo,
		repo,
		slack.NewTextBlockObject(slack.PlainTextType, "Confirm Install", false, false),
	)
	cancelButton := slack.NewButtonBlockElement(
		"skill_install_cancel",
		"cancel",
		slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
	)
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, content, false, false),
			nil,
			nil,
		),
		slack.NewActionBlock("skill_install_actions", confirmButton, cancelButton),
	}

	_, ts, err := c.api.PostMessage(
		cmd.ChannelID,
		slack.MsgOptionText(content, false),
		slack.MsgOptionBlocks(blocks...),
	)
	if err != nil {
		return err
	}

	c.setPendingSkillInstall(ts, pendingSkillInstall{
		UserID:      cmd.UserID,
		ChannelID:   cmd.ChannelID,
		MessageTS:   ts,
		Command:     commandName,
		Repo:        repo,
		CreatedAt:   time.Now(),
		ResponseURL: cmd.ResponseURL,
	})

	return nil
}

func (c *Channel) executeConfirmedSkillInstall(
	callback slack.InteractionCallback,
	pending pendingSkillInstall,
) string {
	cmd, exists := c.commands.Get(pending.Command)
	if !exists {
		return "❌ Install failed: command not found."
	}

	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   pending.ChannelID,
		UserID:   pending.UserID,
		Username: callback.User.Name,
		Command:  pending.Command,
		Args:     "__confirm_install__ " + pending.Repo,
		Metadata: map[string]string{
			"trigger_id":                   callback.TriggerID,
			"team_id":                      callback.Team.ID,
			"channel_id":                   pending.ChannelID,
			"message_ts":                   pending.MessageTS,
			"skill_install_confirmed_repo": pending.Repo,
			"runtime_id":                   c.ID(),
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

func (c *Channel) updateInteractionMessage(pending pendingSkillInstall, text string) {
	opts := []slack.MsgOption{
		slack.MsgOptionText(text, false),
	}
	if _, _, _, err := c.api.UpdateMessage(pending.ChannelID, pending.MessageTS, opts...); err != nil {
		c.log.Warn("Failed to update Slack interaction message",
			zap.String("channel_id", pending.ChannelID),
			zap.String("message_ts", pending.MessageTS),
			zap.Error(err),
		)
	}
}

func (c *Channel) interactionMessageTS(callback slack.InteractionCallback) string {
	if ts := strings.TrimSpace(callback.Message.Timestamp); ts != "" {
		return ts
	}
	if ts := strings.TrimSpace(callback.MessageTs); ts != "" {
		return ts
	}
	return strings.TrimSpace(callback.Container.MessageTs)
}

func (c *Channel) setPendingSkillInstall(messageTS string, pending pendingSkillInstall) {
	c.pendingSkillMu.Lock()
	defer c.pendingSkillMu.Unlock()
	c.pendingSkillInstalls[messageTS] = pending
}

func (c *Channel) getPendingSkillInstall(messageTS string) (pendingSkillInstall, bool) {
	c.pendingSkillMu.Lock()
	defer c.pendingSkillMu.Unlock()

	pending, ok := c.pendingSkillInstalls[messageTS]
	if !ok {
		return pendingSkillInstall{}, false
	}
	if time.Since(pending.CreatedAt) > 15*time.Minute {
		delete(c.pendingSkillInstalls, messageTS)
		return pendingSkillInstall{}, false
	}
	return pending, true
}

func (c *Channel) clearPendingSkillInstall(messageTS string) {
	c.pendingSkillMu.Lock()
	defer c.pendingSkillMu.Unlock()
	delete(c.pendingSkillInstalls, messageTS)
}

// handleOutbound handles outbound messages from the bus.
// SendMessage sends a message to Slack.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	// Parse session ID (format: "slack:channel_id" or "slack:channel_id:thread_ts")
	channelID, threadTS := c.parseSessionID(msg.SessionID)
	if channelID == "" {
		return fmt.Errorf("invalid session ID: %s", msg.SessionID)
	}

	// Build message options
	opts := []slack.MsgOption{
		slack.MsgOptionText(prependBusToolTrace(msg.Content, msg), false),
	}

	if threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	// Send message
	_, _, err := c.api.PostMessageContext(ctx, channelID, opts...)
	if err != nil {
		return fmt.Errorf("sending slack message: %w", err)
	}

	c.log.Debug("Sent Slack message",
		zap.String("channel_id", channelID),
		zap.String("thread_ts", threadTS))

	return nil
}

func prependBusToolTrace(content string, msg *bus.Message) string {
	return channeltrace.PrependBusToolTrace(content, msg)
}

// parseSessionID parses session ID into channel ID and optional thread timestamp.
func (c *Channel) parseSessionID(sessionID string) (string, string) {
	parts := strings.Split(sessionID, ":")
	if len(parts) < 2 {
		return "", ""
	}

	prefix := parts[0]
	if prefix != c.ChannelType() && prefix != c.ID() {
		return "", ""
	}

	if c.ID() != c.ChannelType() {
		instanceParts := strings.Split(c.ID(), ":")
		if len(parts) >= len(instanceParts)+1 {
			matches := true
			for idx, want := range instanceParts {
				if parts[idx] != want {
					matches = false
					break
				}
			}
			if matches {
				channelID := parts[len(instanceParts)]
				if len(parts) > len(instanceParts)+1 {
					return channelID, strings.Join(parts[len(instanceParts)+1:], ":")
				}
				return channelID, ""
			}
		}
	}

	if len(parts) > 2 {
		return parts[1], strings.Join(parts[2:], ":")
	}
	return parts[1], ""
}

func (c *Channel) sessionID(channelID string) string {
	trimmed := strings.TrimSpace(channelID)
	if c.ID() == c.ChannelType() {
		return c.ChannelType() + ":" + trimmed
	}
	return c.ID() + ":" + trimmed
}

func (c *Channel) sessionThreadID(channelID, threadTS string) string {
	base := c.sessionID(channelID)
	thread := strings.TrimSpace(threadTS)
	if thread == "" {
		return base
	}
	return base + ":" + thread
}

func defaultSlackName(displayName string) string {
	name := strings.TrimSpace(displayName)
	if name == "" {
		return "Slack"
	}
	return name
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

func (c *Channel) transcribeFiles(files []slack.File) (string, bool) {
	for _, f := range files {
		mime := strings.ToLower(f.Mimetype)
		ext := strings.ToLower(filepath.Ext(f.Name))
		if !strings.HasPrefix(mime, "audio/") &&
			ext != ".ogg" && ext != ".mp3" && ext != ".wav" && ext != ".m4a" && ext != ".webm" {
			continue
		}
		url := f.URLPrivateDownload
		if url == "" {
			url = f.URLPrivate
		}
		if url == "" {
			continue
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+c.config.BotToken)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.log.Warn("Failed to download Slack audio", zap.Error(err))
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_ = resp.Body.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024))
		_ = resp.Body.Close()
		if err != nil {
			c.log.Warn("Failed reading Slack audio", zap.Error(err))
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		text, err := c.transcriber.Transcribe(ctx, data, f.Name)
		cancel()
		if err != nil {
			c.log.Warn("Slack audio transcription failed", zap.Error(err))
			continue
		}
		text = strings.TrimSpace(text)
		if text != "" {
			return text, true
		}
	}
	return "", false
}
