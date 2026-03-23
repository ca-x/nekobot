package wechat

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

var reMarkdownImage = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)

// Channel implements WeChat channel via WeChat iLink.
type Channel struct {
	log      *logger.Logger
	config   config.WeChatConfig
	bus      bus.Bus
	agent    *agent.Agent
	commands *commands.Registry
	store    *CredentialStore

	mu      sync.RWMutex
	client  *Client
	creds   *Credentials
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewChannel creates a new WeChat channel.
func NewChannel(
	log *logger.Logger,
	cfg config.WeChatConfig,
	b bus.Bus,
	ag *agent.Agent,
	cmdRegistry *commands.Registry,
	store *CredentialStore,
) (*Channel, error) {
	if store == nil {
		return nil, fmt.Errorf("credential store is required")
	}
	creds, err := store.LoadCredentials()
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w", err)
	}
	var client *Client
	if creds != nil {
		client = NewClient(creds)
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		agent:    ag,
		commands: cmdRegistry,
		store:    store,
		client:   client,
		creds:    creds,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "wechat"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "WeChat"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the WeChat channel.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.running = true
	client := c.client
	c.mu.Unlock()

	c.log.Info("Starting WeChat channel", zap.Bool("bound", client != nil))
	if client == nil {
		c.log.Info("WeChat channel started without bound account")
		return nil
	}

	go c.monitorLoop()
	return nil
}

// Stop stops the WeChat channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}
	c.running = false
	if c.cancel != nil {
		c.cancel()
	}
	c.log.Info("WeChat channel stopped")
	return nil
}

func (c *Channel) monitorLoop() {
	creds := c.currentCredentials()
	if creds == nil {
		return
	}

	cursor, err := c.store.ReadSyncState(creds.ILinkBotID)
	if err != nil {
		c.log.Warn("Failed to read WeChat sync state", zap.Error(err))
	}

	pollInterval := time.Duration(c.config.PollIntervalSeconds) * time.Second
	if pollInterval <= 0 {
		pollInterval = 3 * time.Second
	}

	failures := 0
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		client := c.currentClient()
		if client == nil {
			return
		}

		resp, err := client.GetUpdates(c.ctx, cursor)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				select {
				case <-time.After(pollInterval):
				case <-c.ctx.Done():
					return
				}
				continue
			}
			failures++
			backoff := time.Duration(minInt(failures, 5)) * pollInterval
			c.log.Warn("WeChat GetUpdates failed", zap.Error(err), zap.Int("failures", failures))
			select {
			case <-time.After(backoff):
			case <-c.ctx.Done():
				return
			}
			continue
		}

		failures = 0
		if resp.ErrCode == errCodeSessionExpired {
			cursor = ""
			if err := c.store.WriteSyncState(creds.ILinkBotID, cursor); err != nil {
				c.log.Warn("Failed to clear expired WeChat sync state", zap.Error(err))
			}
			select {
			case <-time.After(pollInterval):
			case <-c.ctx.Done():
				return
			}
			continue
		}
		if resp.GetUpdatesBuf != "" && resp.GetUpdatesBuf != cursor {
			cursor = resp.GetUpdatesBuf
			if err := c.store.WriteSyncState(creds.ILinkBotID, cursor); err != nil {
				c.log.Warn("Failed to persist WeChat sync state", zap.Error(err))
			}
		}
		for _, msg := range resp.Msgs {
			c.handleInbound(msg)
		}
	}
}

func (c *Channel) handleInbound(msg WeixinMessage) {
	if msg.MessageType != MessageTypeUser || msg.MessageState != MessageStateFinish {
		return
	}

	content := extractText(msg)
	if strings.TrimSpace(content) == "" {
		return
	}

	if !c.isAllowed(msg.FromUserID) {
		c.log.Debug("Unauthorized WeChat sender", zap.String("user_id", msg.FromUserID))
		return
	}

	c.log.Info("Received WeChat message",
		zap.String("user_id", msg.FromUserID),
		zap.Int("length", len(content)))

	if c.commands != nil && c.commands.IsCommand(content) {
		c.handleCommand(msg, content)
		return
	}

	if c.agent == nil {
		sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := c.sendText(sendCtx, msg.FromUserID, "❌ Agent 不可用（未初始化）", msg.ContextToken); err != nil {
			c.log.Error("Failed to send WeChat unavailable message", zap.Error(err))
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	go func() {
		if err := c.sendTyping(ctx, msg.FromUserID, msg.ContextToken); err != nil {
			c.log.Debug("WeChat typing indicator failed", zap.Error(err))
		}
	}()

	sess := &simpleSession{messages: make([]agent.Message, 0, 8)}
	reply, err := c.agent.Chat(ctx, sess, content)
	if err != nil {
		c.log.Error("WeChat agent chat failed", zap.Error(err))
		reply = "❌ 抱歉，处理消息时出现错误。"
	}
	if strings.TrimSpace(reply) == "" {
		reply = "（无输出）"
	}

	if err := c.sendText(ctx, msg.FromUserID, reply, msg.ContextToken); err != nil {
		c.log.Error("Failed to send WeChat reply", zap.Error(err))
	}
}

func (c *Channel) handleCommand(msg WeixinMessage, content string) {
	cmdName, args := c.commands.Parse(content)
	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   msg.FromUserID,
		UserID:   msg.FromUserID,
		Username: msg.FromUserID,
		Command:  cmdName,
		Args:     args,
	})
	if err != nil {
		_ = c.sendText(ctx, msg.FromUserID, "❌ Command failed: "+err.Error(), msg.ContextToken)
		return
	}
	_ = c.sendText(ctx, msg.FromUserID, resp.Content, msg.ContextToken)
}

// SendMessage sends a message through WeChat.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	userID := strings.TrimPrefix(msg.SessionID, "wechat:")
	if strings.TrimSpace(userID) == "" {
		userID = strings.TrimSpace(msg.UserID)
	}
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("wechat session/user id is empty")
	}
	return c.sendText(ctx, userID, msg.Content, "")
}

func (c *Channel) sendText(ctx context.Context, toUserID, text, contextToken string) error {
	client := c.currentClient()
	if client == nil {
		return fmt.Errorf("wechat client not bound")
	}

	plainText := MarkdownToPlainText(text)
	req := &SendMessageRequest{
		Msg: SendMsg{
			FromUserID:   client.BotID(),
			ToUserID:     toUserID,
			ClientID:     uuid.NewString(),
			MessageType:  MessageTypeBot,
			MessageState: MessageStateFinish,
			ItemList: []MessageItem{
				{
					Type:     ItemTypeText,
					TextItem: &TextItem{Text: plainText},
				},
			},
			ContextToken: contextToken,
		},
		BaseInfo: BaseInfo{},
	}

	resp, err := client.SendMessage(ctx, req)
	if err != nil {
		return fmt.Errorf("send wechat message: %w", err)
	}
	if resp.Ret != 0 {
		return fmt.Errorf("send wechat message failed: ret=%d errmsg=%s", resp.Ret, resp.ErrMsg)
	}
	return nil
}

func (c *Channel) sendTyping(ctx context.Context, userID, contextToken string) error {
	client := c.currentClient()
	if client == nil {
		return fmt.Errorf("wechat client not bound")
	}
	cfgResp, err := client.GetConfig(ctx, userID, contextToken)
	if err != nil {
		return fmt.Errorf("get config for typing: %w", err)
	}
	if strings.TrimSpace(cfgResp.TypingTicket) == "" {
		return nil
	}
	return client.SendTyping(ctx, userID, cfgResp.TypingTicket, TypingStatusTyping)
}

func (c *Channel) currentClient() *Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

func (c *Channel) currentCredentials() *Credentials {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.creds
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

func extractText(msg WeixinMessage) string {
	for _, item := range msg.ItemList {
		if item.Type == ItemTypeText && item.TextItem != nil {
			return strings.TrimSpace(item.TextItem.Text)
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

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

// MarkdownToPlainText converts markdown to readable plain text for WeChat.
func MarkdownToPlainText(text string) string {
	result := text
	result = reMarkdownImage.ReplaceAllString(result, "")
	result = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`).ReplaceAllString(result, "$1")
	result = regexp.MustCompile("(?s)```[^\\n]*\\n?(.*?)```").ReplaceAllString(result, "$1")
	result = regexp.MustCompile("`([^`]+)`").ReplaceAllString(result, "$1")
	result = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(result, "")
	result = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`).ReplaceAllStringFunc(result, func(match string) string {
		m := regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`).FindStringSubmatch(match)
		if m[1] != "" {
			return m[1]
		}
		return m[2]
	})
	result = regexp.MustCompile(`~~(.+?)~~`).ReplaceAllString(result, "$1")
	result = regexp.MustCompile(`(?m)^>\s?`).ReplaceAllString(result, "")
	result = regexp.MustCompile(`(?m)^(\s*)[-*+]\s+`).ReplaceAllString(result, "${1}• ")
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")
	return strings.TrimSpace(result)
}
