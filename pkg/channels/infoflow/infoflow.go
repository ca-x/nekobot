// Package infoflow provides an infoflow webhook channel.
package infoflow

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

const (
	defaultInfoflowListen = ":8768"
	defaultInfoflowPath   = "/infoflow/webhook"
)

// Channel implements an infoflow webhook channel.
type Channel struct {
	log      *logger.Logger
	config   config.InfoflowConfig
	bus      bus.Bus
	commands *commands.Registry

	ctx        context.Context
	cancel     context.CancelFunc
	running    bool
	httpServer *http.Server
	httpClient *http.Client
	listenPath string
}

// NewChannel creates an infoflow channel.
func NewChannel(
	log *logger.Logger,
	cfg config.InfoflowConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
) (*Channel, error) {
	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("infoflow webhook_url is required")
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		commands: cmdRegistry,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}, nil
}

// ID returns channel ID.
func (c *Channel) ID() string { return "infoflow" }

// Name returns channel name.
func (c *Channel) Name() string { return "Infoflow" }

// IsEnabled returns whether channel is enabled.
func (c *Channel) IsEnabled() bool { return c.config.Enabled }

// Start starts webhook listener.
func (c *Channel) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.bus.RegisterHandler(c.ID(), c.handleOutbound)

	listenAddr := defaultInfoflowListen
	listenPath := defaultInfoflowPath
	if u, err := url.Parse(c.config.WebhookURL); err == nil {
		if u.Host != "" {
			listenAddr = u.Host
		}
		if u.Path != "" {
			listenPath = u.Path
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc(listenPath, c.handleWebhook)

	c.httpServer = &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	c.listenPath = listenPath

	go func() {
		if err := c.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			c.log.Error("Infoflow webhook server failed", zap.Error(err))
		}
	}()

	c.running = true
	c.log.Info("Infoflow channel started",
		zap.String("listen_addr", listenAddr),
		zap.String("webhook_path", listenPath))
	return nil
}

// Stop stops webhook listener.
func (c *Channel) Stop(ctx context.Context) error {
	c.running = false
	if c.cancel != nil {
		c.cancel()
	}
	c.bus.UnregisterHandlers(c.ID())

	if c.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutting down infoflow server: %w", err)
		}
	}
	return nil
}

type infoflowInbound struct {
	MessageID    string `json:"message_id"`
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	SessionID    string `json:"session_id"`
	Content      string `json:"content"`
	Text         string `json:"text"`
	Ciphertext   string `json:"ciphertext"`
	IV           string `json:"iv"`
	ReplyWebhook string `json:"reply_webhook"`
}

func (c *Channel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload infoflowInbound
	if err := json.NewDecoder(io.LimitReader(r.Body, 2*1024*1024)).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)

	userID := strings.TrimSpace(payload.UserID)
	if userID == "" {
		userID = "unknown"
	}
	if !c.isAllowed(userID) {
		c.log.Warn("Unauthorized Infoflow sender", zap.String("user_id", userID))
		return
	}

	content := strings.TrimSpace(payload.Content)
	if content == "" {
		content = strings.TrimSpace(payload.Text)
	}
	if content == "" && payload.Ciphertext != "" {
		decrypted, err := c.decrypt(payload.Ciphertext, payload.IV)
		if err != nil {
			c.log.Warn("Failed to decrypt Infoflow payload", zap.Error(err))
			return
		}
		content = strings.TrimSpace(decrypted)
	}
	if content == "" {
		return
	}

	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		sessionID = "infoflow:" + userID
	}
	if !strings.HasPrefix(sessionID, "infoflow:") {
		sessionID = "infoflow:" + sessionID
	}

	if c.commands.IsCommand(content) {
		c.handleCommand(sessionID, userID, payload.Username, content, payload.ReplyWebhook)
		return
	}

	msgID := strings.TrimSpace(payload.MessageID)
	if msgID == "" {
		msgID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	msg := &bus.Message{
		ID:        "infoflow:" + msgID,
		ChannelID: c.ID(),
		SessionID: sessionID,
		UserID:    userID,
		Username:  payload.Username,
		Type:      bus.MessageTypeText,
		Content:   content,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"reply_webhook": payload.ReplyWebhook,
		},
	}
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to send Infoflow inbound message", zap.Error(err))
	}
}

func (c *Channel) handleCommand(sessionID, userID, username, content, replyWebhook string) {
	cmdName, args := c.commands.Parse(content)
	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := cmd.Handler(ctx, commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   sessionID,
		UserID:   userID,
		Username: username,
		Command:  cmdName,
		Args:     args,
		Metadata: map[string]string{
			"reply_webhook": replyWebhook,
		},
	})
	if err != nil {
		c.log.Error("Infoflow command failed", zap.String("command", cmdName), zap.Error(err))
		return
	}

	data := map[string]interface{}{}
	if replyWebhook != "" {
		data["reply_webhook"] = replyWebhook
	}
	_ = c.SendMessage(ctx, &bus.Message{
		ChannelID: c.ID(),
		SessionID: sessionID,
		Content:   resp.Content,
		Data:      data,
	})
}

func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends message to infoflow webhook.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	webhook := c.config.WebhookURL
	if msg.Data != nil {
		if value, ok := msg.Data["reply_webhook"].(string); ok && strings.TrimSpace(value) != "" {
			webhook = strings.TrimSpace(value)
		}
	}

	payload := map[string]interface{}{
		"session_id": msg.SessionID,
		"content":    msg.Content,
	}
	if strings.TrimSpace(c.config.AESKey) != "" {
		ciphertext, iv, err := c.encrypt(msg.Content)
		if err != nil {
			return fmt.Errorf("encrypting infoflow payload: %w", err)
		}
		delete(payload, "content")
		payload["ciphertext"] = ciphertext
		payload["iv"] = iv
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling infoflow payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("creating infoflow request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending infoflow request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("infoflow webhook status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
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

func (c *Channel) getAESKey() ([]byte, error) {
	key := strings.TrimSpace(c.config.AESKey)
	if key == "" {
		return nil, fmt.Errorf("aes_key is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err == nil && (len(decoded) == 16 || len(decoded) == 24 || len(decoded) == 32) {
		return decoded, nil
	}
	raw := []byte(key)
	if len(raw) == 16 || len(raw) == 24 || len(raw) == 32 {
		return raw, nil
	}
	return nil, fmt.Errorf("invalid aes key length %d", len(raw))
}

func (c *Channel) encrypt(plain string) (ciphertext string, iv string, err error) {
	key, err := c.getAESKey()
	if err != nil {
		return "", "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	padded := pkcs7Pad([]byte(plain), block.BlockSize())
	ivBytes := make([]byte, block.BlockSize())
	if _, err := rand.Read(ivBytes); err != nil {
		return "", "", err
	}
	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, ivBytes).CryptBlocks(encrypted, padded)
	return base64.StdEncoding.EncodeToString(encrypted), base64.StdEncoding.EncodeToString(ivBytes), nil
}

func (c *Channel) decrypt(ciphertext, iv string) (string, error) {
	key, err := c.getAESKey()
	if err != nil {
		return "", err
	}
	encrypted, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	ivBytes, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	if len(encrypted)%block.BlockSize() != 0 {
		return "", fmt.Errorf("invalid encrypted payload size")
	}
	if len(ivBytes) != block.BlockSize() {
		return "", fmt.Errorf("invalid iv length")
	}
	decrypted := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, ivBytes).CryptBlocks(decrypted, encrypted)
	unpadded, err := pkcs7Unpad(decrypted, block.BlockSize())
	if err != nil {
		return "", err
	}
	return string(unpadded), nil
}

func pkcs7Pad(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padtext...)
}

func pkcs7Unpad(src []byte, blockSize int) ([]byte, error) {
	if len(src) == 0 || len(src)%blockSize != 0 {
		return nil, fmt.Errorf("invalid padded data")
	}
	padding := int(src[len(src)-1])
	if padding <= 0 || padding > blockSize || padding > len(src) {
		return nil, fmt.Errorf("invalid padding")
	}
	for _, b := range src[len(src)-padding:] {
		if int(b) != padding {
			return nil, fmt.Errorf("invalid padding")
		}
	}
	return src[:len(src)-padding], nil
}
