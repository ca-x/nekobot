// Package teams provides Microsoft Teams channel implementation.
package teams

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

const (
	defaultWebhookAddr = ":3978"
	webhookPath        = "/api/messages"
	botTokenScope      = "https://api.botframework.com/.default"
	botTokenURL        = "https://login.microsoftonline.com/botframework.com/oauth2/v2.0/token"
)

type channelAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type conversationRef struct {
	ID string `json:"id"`
}

type activity struct {
	ID           string          `json:"id,omitempty"`
	Type         string          `json:"type"`
	ServiceURL   string          `json:"serviceUrl,omitempty"`
	Timestamp    time.Time       `json:"timestamp,omitempty"`
	From         channelAccount  `json:"from,omitempty"`
	Recipient    channelAccount  `json:"recipient,omitempty"`
	Conversation conversationRef `json:"conversation,omitempty"`
	Text         string          `json:"text,omitempty"`
	ReplyToID    string          `json:"replyToId,omitempty"`
	ChannelID    string          `json:"channelId,omitempty"`
}

type conversationContext struct {
	ServiceURL    string
	Conversation  string
	BotChannelID  string
	LastReplyToID string
}

// Channel implements the Microsoft Teams channel.
type Channel struct {
	log      *logger.Logger
	config   config.TeamsConfig
	bus      bus.Bus
	commands *commands.Registry

	ctx        context.Context
	cancel     context.CancelFunc
	running    bool
	httpServer *http.Server
	httpClient *http.Client

	tokenMu      sync.Mutex
	accessToken  string
	tokenExpires time.Time

	contextsMu sync.RWMutex
	contexts   map[string]conversationContext
}

// NewChannel creates a new Teams channel.
func NewChannel(
	log *logger.Logger,
	cfg config.TeamsConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
) (*Channel, error) {
	if cfg.AppID == "" || cfg.AppPassword == "" {
		return nil, fmt.Errorf("teams app_id and app_password are required")
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		commands: cmdRegistry,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		contexts: make(map[string]conversationContext),
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "teams"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "Microsoft Teams"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts Teams webhook handling.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting Teams channel")
	c.ctx, c.cancel = context.WithCancel(ctx)

	c.bus.RegisterHandler(c.ID(), c.handleOutbound)

	mux := http.NewServeMux()
	mux.HandleFunc(webhookPath, c.handleWebhook)
	c.httpServer = &http.Server{
		Addr:         defaultWebhookAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := c.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			c.log.Error("Teams webhook server failed", zap.Error(err))
		}
	}()

	c.running = true
	c.log.Info("Teams channel started",
		zap.String("webhook_path", webhookPath),
		zap.String("listen_addr", defaultWebhookAddr))
	return nil
}

// Stop stops Teams channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Teams channel")
	c.running = false
	if c.cancel != nil {
		c.cancel()
	}
	c.bus.UnregisterHandlers(c.ID())

	if c.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutting down teams webhook server: %w", err)
		}
	}
	return nil
}

func (c *Channel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var act activity
	if err := json.NewDecoder(io.LimitReader(r.Body, 2*1024*1024)).Decode(&act); err != nil {
		c.log.Warn("Failed to decode Teams activity", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Bot framework expects a fast ack.
	w.WriteHeader(http.StatusOK)

	if act.Type != "message" || strings.TrimSpace(act.Text) == "" {
		return
	}
	if !c.isAllowed(act.From.ID) {
		c.log.Warn("Unauthorized Teams sender", zap.String("user_id", act.From.ID))
		return
	}

	sessionID := "teams:" + act.Conversation.ID
	c.contextsMu.Lock()
	c.contexts[sessionID] = conversationContext{
		ServiceURL:    strings.TrimSpace(act.ServiceURL),
		Conversation:  act.Conversation.ID,
		BotChannelID:  act.ChannelID,
		LastReplyToID: act.ID,
	}
	c.contextsMu.Unlock()

	content := strings.TrimSpace(act.Text)

	if c.commands.IsCommand(content) {
		c.handleCommand(sessionID, act, content)
		return
	}

	msg := &bus.Message{
		ID:        "teams:" + act.ID,
		ChannelID: c.ID(),
		SessionID: sessionID,
		UserID:    act.From.ID,
		Username:  act.From.Name,
		Type:      bus.MessageTypeText,
		Content:   content,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"service_url": act.ServiceURL,
			"reply_to_id": act.ID,
		},
	}
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to dispatch Teams message to bus", zap.Error(err))
	}
}

func (c *Channel) handleCommand(sessionID string, act activity, content string) {
	cmdName, args := c.commands.Parse(content)
	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   act.Conversation.ID,
		UserID:   act.From.ID,
		Username: act.From.Name,
		Command:  cmdName,
		Args:     args,
		Metadata: map[string]string{
			"service_url": act.ServiceURL,
			"activity_id": act.ID,
		},
	})
	if err != nil {
		c.log.Error("Teams command failed", zap.String("command", cmdName), zap.Error(err))
		_ = c.sendActivity(ctx, sessionID, "❌ Command failed: "+err.Error(), act.ID)
		return
	}

	if err := c.sendActivity(ctx, sessionID, resp.Content, act.ID); err != nil {
		c.log.Error("Failed to send Teams command response", zap.Error(err))
	}
}

func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to Teams conversation.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	return c.sendActivity(ctx, msg.SessionID, msg.Content, msg.ReplyTo)
}

func (c *Channel) sendActivity(ctx context.Context, sessionID, text, replyTo string) error {
	c.contextsMu.RLock()
	conv, ok := c.contexts[sessionID]
	c.contextsMu.RUnlock()
	if !ok {
		return fmt.Errorf("no conversation context for session %s", sessionID)
	}
	if conv.ServiceURL == "" || conv.Conversation == "" {
		return fmt.Errorf("incomplete conversation context for session %s", sessionID)
	}

	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("getting teams token: %w", err)
	}

	payload := activity{
		Type:      "message",
		Text:      text,
		ReplyToID: replyTo,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling teams payload: %w", err)
	}

	endpoint := strings.TrimRight(conv.ServiceURL, "/") + "/v3/conversations/" + url.PathEscape(conv.Conversation) + "/activities"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(raw)))
	if err != nil {
		return fmt.Errorf("creating teams request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending teams request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("teams api status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *Channel) getAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpires) {
		return c.accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.config.AppID)
	form.Set("client_secret", c.config.AppPassword)
	form.Set("scope", botTokenScope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, botTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access_token in token response")
	}

	c.accessToken = tokenResp.AccessToken
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	c.tokenExpires = time.Now().Add(time.Duration(expiresIn-120) * time.Second)
	return c.accessToken, nil
}

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
