package wechat

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	"nekobot/pkg/process"
	"nekobot/pkg/richtext"
	"nekobot/pkg/toolsessions"
	"nekobot/pkg/transcription"
	wxmedia "nekobot/pkg/wechat/media"
)

// Channel implements WeChat channel via WeChat iLink.
type Channel struct {
	log      *logger.Logger
	config   config.WeChatConfig
	bus      bus.Bus
	agent    *agent.Agent
	commands *commands.Registry
	store    *CredentialStore
	runtime  *ControlService
	renderer richtext.MarkdownImageRenderer
	inbound  *wxmedia.InboundProcessor

	mu      sync.RWMutex
	client  *Client
	creds   *Credentials
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	cursors map[string]int
}

// NewChannel creates a new WeChat channel.
func NewChannel(
	log *logger.Logger,
	cfg config.WeChatConfig,
	b bus.Bus,
	ag *agent.Agent,
	cmdRegistry *commands.Registry,
	store *CredentialStore,
	toolSessionMgr *toolsessions.Manager,
	processMgr *process.Manager,
	rootCfg *config.Config,
	transcriber transcription.Transcriber,
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

	var runtimeControl *ControlService
	if toolSessionMgr != nil && processMgr != nil {
		bindingSvc := NewRuntimeBindingService(toolSessionMgr, rootCfg)
		runtimeControl = NewControlService(rootCfg, toolSessionMgr, processMgr, bindingSvc)
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		agent:    ag,
		commands: cmdRegistry,
		store:    store,
		runtime:  runtimeControl,
		renderer: richtext.NewBrowserMarkdownRenderer(log, filepath.Join(rootCfg.WorkspacePath(), "screenshots", "wechat")),
		inbound:  wxmedia.NewInboundProcessor(wxmedia.NewDownloader(filepath.Join(rootCfg.DatabaseDir(), "wechat", "media")), transcriber),
		client:   client,
		creds:    creds,
		cursors:  map[string]int{},
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

	content := c.buildInboundContent(msg)
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

	if c.runtime != nil && strings.HasPrefix(strings.TrimSpace(content), "/") {
		handled, err := c.handleControlCommand(msg, content)
		if err != nil {
			sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = c.sendReply(sendCtx, msg.FromUserID, "❌ "+err.Error(), msg.ContextToken)
			return
		}
		if handled {
			return
		}
	}

	if c.commands != nil && c.commands.IsCommand(content) {
		c.handleCommand(msg, content)
		return
	}

	if c.runtime != nil {
		if handled, err := c.handleRuntimeChat(msg, content); err != nil {
			sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = c.sendReply(sendCtx, msg.FromUserID, "❌ "+err.Error(), msg.ContextToken)
			return
		} else if handled {
			return
		}
	}

	if c.agent == nil {
		sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := c.sendReply(sendCtx, msg.FromUserID, "❌ Agent 不可用（未初始化）", msg.ContextToken); err != nil {
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
	reply, err := c.agent.ChatWithPromptContext(ctx, sess, content, agent.PromptContext{
		Channel:   c.ID(),
		SessionID: msg.FromUserID,
		UserID:    msg.FromUserID,
		Username:  msg.FromUserID,
	})
	if err != nil {
		c.log.Error("WeChat agent chat failed", zap.Error(err))
		reply = "❌ 抱歉，处理消息时出现错误。"
	}
	if strings.TrimSpace(reply) == "" {
		reply = "（无输出）"
	}

	if err := c.sendReply(ctx, msg.FromUserID, reply, msg.ContextToken); err != nil {
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
		_ = c.sendReply(ctx, msg.FromUserID, "❌ Command failed: "+err.Error(), msg.ContextToken)
		return
	}
	_ = c.sendReply(ctx, msg.FromUserID, resp.Content, msg.ContextToken)
}

func (c *Channel) handleControlCommand(msg WeixinMessage, content string) (bool, error) {
	if c.runtime == nil {
		return false, nil
	}

	cmd, err := parseControlCommand(content)
	if err != nil {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var reply string
	switch cmd.Kind {
	case controlCommandList:
		items, err := c.runtime.ListRuntimes(ctx)
		if err != nil {
			return true, err
		}
		if len(items) == 0 {
			reply = "No WeChat runtimes."
			break
		}
		lines := make([]string, 0, len(items))
		for _, item := range items {
			if item == nil {
				continue
			}
			name := strings.TrimSpace(item.Title)
			if name == "" {
				name = strings.TrimSpace(item.Tool)
			}
			lines = append(lines, fmt.Sprintf("%s [%s]", name, item.State))
		}
		reply = strings.Join(lines, "\n")
	case controlCommandBindings:
		text, err := c.runtime.DescribeBindings(ctx)
		if err != nil {
			return true, err
		}
		reply = text
	case controlCommandUse:
		if err := c.runtime.BindRuntime(ctx, msg.FromUserID, cmd.RuntimeName); err != nil {
			return true, err
		}
		reply = fmt.Sprintf("Bound current chat to %s.", cmd.RuntimeName)
	case controlCommandNew:
		created, err := c.runtime.CreateRuntime(ctx, msg.FromUserID, RuntimeCreateRequest{
			Name:    cmd.RuntimeName,
			Driver:  cmd.Spec.Driver,
			Tool:    cmd.Spec.Tool,
			Command: cmd.Spec.Command,
			Workdir: cmd.Spec.Workdir,
		})
		if err != nil {
			return true, err
		}
		reply = fmt.Sprintf("Created runtime %s (%s).", strings.TrimSpace(created.Title), created.Tool)
	case controlCommandStatus:
		runtimeName := strings.TrimSpace(cmd.RuntimeName)
		if runtimeName == "" {
			bound, err := c.runtime.GetConversationRuntime(ctx, msg.FromUserID)
			if err != nil {
				return true, err
			}
			if bound == nil || bound.Session == nil {
				reply = "No runtime bound for current chat."
				break
			}
			runtimeName = strings.TrimSpace(bound.Session.Title)
			if runtimeName == "" {
				runtimeName = strings.TrimSpace(bound.Session.Tool)
			}
		}
		status, err := c.runtime.GetRuntimeStatus(ctx, runtimeName)
		if err != nil {
			return true, err
		}
		reply = formatRuntimeStatus(status)
	case controlCommandLogs:
		runtimeName := strings.TrimSpace(cmd.RuntimeName)
		if runtimeName == "" {
			bound, err := c.runtime.GetConversationRuntime(ctx, msg.FromUserID)
			if err != nil {
				return true, err
			}
			if bound == nil || bound.Session == nil {
				reply = "No runtime bound for current chat."
				break
			}
			runtimeName = strings.TrimSpace(bound.Session.Title)
			if runtimeName == "" {
				runtimeName = strings.TrimSpace(bound.Session.Tool)
			}
		}
		logs, err := c.runtime.GetRuntimeLogs(ctx, runtimeName, 120)
		if err != nil {
			return true, err
		}
		reply = logs
	case controlCommandRestart:
		if err := c.runtime.RestartRuntime(ctx, cmd.RuntimeName); err != nil {
			return true, err
		}
		reply = fmt.Sprintf("Restarted runtime %s.", cmd.RuntimeName)
	case controlCommandStop:
		if err := c.runtime.StopRuntime(ctx, cmd.RuntimeName); err != nil {
			return true, err
		}
		reply = fmt.Sprintf("Stopped runtime %s.", cmd.RuntimeName)
	case controlCommandDelete:
		if err := c.runtime.DeleteRuntime(ctx, cmd.RuntimeName); err != nil {
			return true, err
		}
		reply = fmt.Sprintf("Deleted runtime %s.", cmd.RuntimeName)
	default:
		return false, nil
	}

	if strings.TrimSpace(reply) == "" {
		reply = "OK"
	}
	if err := c.sendReply(ctx, msg.FromUserID, reply, msg.ContextToken); err != nil {
		return true, err
	}
	return true, nil
}

func formatRuntimeStatus(status *runtimeStatusDetails) string {
	if status == nil {
		return "Runtime not found."
	}

	runningText := "stopped"
	if status.Running {
		runningText = "running"
	}

	parts := []string{
		fmt.Sprintf("%s [%s]", status.Name, runningText),
		fmt.Sprintf("driver: %s", status.Driver),
		fmt.Sprintf("tool: %s", status.Tool),
	}
	if status.Command != "" {
		parts = append(parts, fmt.Sprintf("command: %s", status.Command))
	}
	if status.Workdir != "" {
		parts = append(parts, fmt.Sprintf("cwd: %s", status.Workdir))
	}
	if status.Driver != "acp" {
		parts = append(parts, fmt.Sprintf("exit_code: %d", status.ExitCode))
	}
	return strings.Join(parts, "\n")
}

func (c *Channel) handleRuntimeChat(msg WeixinMessage, content string) (bool, error) {
	if c.runtime == nil {
		return false, nil
	}
	bound, err := c.runtime.GetConversationRuntime(context.Background(), msg.FromUserID)
	if err != nil {
		return false, err
	}
	if bound == nil {
		return false, nil
	}
	reply, err := c.runtime.SendToRuntime(context.Background(), msg.FromUserID, "", content)
	if err != nil {
		return true, err
	}
	if strings.TrimSpace(reply) != "" {
		if err := c.sendReply(context.Background(), msg.FromUserID, reply, msg.ContextToken); err != nil {
			return true, err
		}
		return true, nil
	}

	cursor := bound.NextRead
	if stored, ok := c.loadCursor(bound.Session.ID); ok {
		cursor = stored
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		output, next, err := c.runtime.ReadRuntimeOutput(context.Background(), bound.Session.ID, cursor)
		if err != nil {
			return true, err
		}
		cursor = next
		c.storeCursor(bound.Session.ID, cursor)
		if strings.TrimSpace(output) == "" {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if err := c.sendReply(context.Background(), msg.FromUserID, output, msg.ContextToken); err != nil {
			return true, err
		}
		return true, nil
	}

	name := strings.TrimSpace(bound.Session.Title)
	if name == "" {
		name = strings.TrimSpace(bound.Session.Tool)
	}
	if err := c.sendReply(context.Background(), msg.FromUserID, fmt.Sprintf("已发送到 %s，等待输出中。", name), msg.ContextToken); err != nil {
		return true, err
	}
	return true, nil
}

func (c *Channel) loadCursor(sessionID string) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	offset, ok := c.cursors[strings.TrimSpace(sessionID)]
	return offset, ok
}

func (c *Channel) storeCursor(sessionID string, offset int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cursors[strings.TrimSpace(sessionID)] = offset
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
	return c.sendReply(ctx, userID, msg.Content, "")
}

func (c *Channel) sendReply(ctx context.Context, toUserID, text, contextToken string) error {
	if c.renderer != nil && richtext.ShouldRenderAsImage(text) {
		imagePath, err := c.renderer.RenderMarkdown(ctx, text)
		if err == nil {
			sendErr := c.sendImage(ctx, toUserID, imagePath, contextToken)
			if sendErr == nil {
				return nil
			}
			c.log.Warn("Failed to send rendered WeChat image reply", zap.Error(sendErr))
		} else {
			c.log.Warn("Failed to render markdown image reply", zap.Error(err))
		}
	}

	for _, segment := range richtext.SplitPlainText(richtext.MarkdownToPlainText(text), 1500) {
		if err := c.sendPlainText(ctx, toUserID, segment, contextToken); err != nil {
			return err
		}
	}
	return nil
}

func (c *Channel) sendPlainText(ctx context.Context, toUserID, text, contextToken string) error {
	client := c.currentClient()
	if client == nil {
		return fmt.Errorf("wechat client not bound")
	}

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
					TextItem: &TextItem{Text: text},
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

func (c *Channel) sendImage(ctx context.Context, toUserID, imagePath, contextToken string) error {
	client := c.currentClient()
	if client == nil {
		return fmt.Errorf("wechat client not bound")
	}
	raw, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("read image reply: %w", err)
	}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(imagePath)), ".")
	if format == "" {
		format = "png"
	}
	req := &SendMessageRequest{
		Msg: SendMsg{
			FromUserID:   client.BotID(),
			ToUserID:     toUserID,
			ClientID:     uuid.NewString(),
			MessageType:  MessageTypeBot,
			MessageState: MessageStateFinish,
			ItemList: []MessageItem{
				{
					Type: ItemTypeImage,
					ImageItem: &ImageItem{
						Data:   base64.StdEncoding.EncodeToString(raw),
						Format: format,
					},
				},
			},
			ContextToken: contextToken,
		},
		BaseInfo: BaseInfo{},
	}
	resp, err := client.SendMessage(ctx, req)
	if err != nil {
		return fmt.Errorf("send wechat image: %w", err)
	}
	if resp.Ret != 0 {
		return fmt.Errorf("send wechat image failed: ret=%d errmsg=%s", resp.Ret, resp.ErrMsg)
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

func (c *Channel) buildInboundContent(msg WeixinMessage) string {
	items := make([]wxmedia.Item, 0, len(msg.ItemList))
	for _, item := range msg.ItemList {
		items = append(items, convertMessageItem(item))
	}
	if c.inbound == nil {
		return strings.TrimSpace(wxmedia.BuildBody(items, nil))
	}
	body, err := c.inbound.Process(context.Background(), items)
	if err != nil {
		c.log.Warn("Failed to process WeChat inbound media", zap.Error(err), zap.String("user_id", msg.FromUserID))
	}
	return strings.TrimSpace(body)
}

func convertMessageItem(item MessageItem) wxmedia.Item {
	converted := wxmedia.Item{Type: item.Type}
	if item.TextItem != nil {
		converted.Text = &wxmedia.TextItem{Text: item.TextItem.Text}
	}
	if item.VoiceItem != nil {
		converted.Voice = &wxmedia.VoiceItem{
			EncodeType: item.VoiceItem.EncodeType,
			Text:       item.VoiceItem.Text,
			Media:      convertCDNMedia(item.VoiceItem.Media),
		}
	}
	if item.ImageItem != nil {
		converted.Image = &wxmedia.ImageItem{
			AESKey:  item.ImageItem.AESKey,
			Data:    item.ImageItem.Data,
			Format:  item.ImageItem.Format,
			MidSize: item.ImageItem.MidSize,
			Media:   convertCDNMedia(item.ImageItem.Media),
			Thumb:   convertCDNMedia(item.ImageItem.Thumb),
		}
	}
	if item.FileItem != nil {
		converted.File = &wxmedia.FileItem{
			FileName: item.FileItem.FileName,
			Media:    convertCDNMedia(item.FileItem.Media),
		}
	}
	if item.VideoItem != nil {
		converted.Video = &wxmedia.VideoItem{
			Media: convertCDNMedia(item.VideoItem.Media),
			Thumb: convertCDNMedia(item.VideoItem.Thumb),
		}
	}
	return converted
}

func convertCDNMedia(media *CDNMedia) *wxmedia.CDNMedia {
	if media == nil {
		return nil
	}
	return &wxmedia.CDNMedia{
		EncryptQueryParam: media.EncryptQueryParam,
		AESKey:            media.AESKey,
		EncryptType:       media.EncryptType,
	}
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
