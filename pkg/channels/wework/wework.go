// Package wework provides WeWork (企业微信) channel implementation.
package wework

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Channel implements WeWork channel with webhook server and API.
type Channel struct {
	log      *logger.Logger
	config   config.WeWorkConfig
	bus      bus.Bus
	commands *commands.Registry

	accessToken    string
	tokenExpiresAt int64
	mu             sync.Mutex
	httpClient     *http.Client
	httpServer     *http.Server
	running        bool
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewChannel creates a new WeWork channel.
func NewChannel(
	log *logger.Logger,
	cfg config.WeWorkConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
) (*Channel, error) {
	if cfg.CorpID == "" || cfg.AgentID == "" || cfg.CorpSecret == "" {
		return nil, fmt.Errorf("wework corp_id, agent_id and corp_secret are required")
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		commands: cmdRegistry,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		running: false,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "wework"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "WeWork"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the WeWork channel with webhook server.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting WeWork channel")

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Register outbound message handler
	c.bus.RegisterHandler("wework", c.handleOutbound)

	// Start webhook server
	go c.startWebhookServer()

	c.running = true
	c.log.Info("WeWork channel started")
	return nil
}

// Stop stops the WeWork channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping WeWork channel")

	c.running = false

	if c.cancel != nil {
		c.cancel()
	}

	// Unregister handler
	c.bus.UnregisterHandlers("wework")

	// Stop HTTP server
	if c.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c.httpServer.Shutdown(shutdownCtx)
	}

	c.log.Info("WeWork channel stopped")
	return nil
}

// startWebhookServer starts the webhook HTTP server.
func (c *Channel) startWebhookServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/wework/event", c.handleWebhook)

	port := 8766 // Default port
	c.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	c.log.Info("WeWork webhook server starting", zap.String("addr", c.httpServer.Addr))
	if err := c.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		c.log.Error("WeWork webhook server error", zap.Error(err))
	}
}

// handleWebhook handles incoming webhook requests.
func (c *Channel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	signature := query.Get("msg_signature")
	timestamp := query.Get("timestamp")
	nonce := query.Get("nonce")
	echostr := query.Get("echostr")

	// Handle GET request (URL verification)
	if r.Method == http.MethodGet {
		if !c.verifySignature(c.config.Token, timestamp, nonce, echostr, signature) {
			c.log.Warn("Invalid signature for GET")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Decrypt echostr if encryption is enabled
		if c.config.EncodingAESKey != "" {
			decrypted, err := c.decryptMsg(echostr)
			if err != nil {
				c.log.Error("Failed to decrypt echostr", zap.Error(err))
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Write(decrypted)
		} else {
			w.Write([]byte(echostr))
		}
		return
	}

	// Handle POST request (message event)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.log.Error("Failed to read body", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Parse encrypted message
	var encryptedMsg struct {
		XMLName    xml.Name `xml:"xml"`
		ToUserName string   `xml:"ToUserName"`
		Encrypt    string   `xml:"Encrypt"`
		AgentID    string   `xml:"AgentID"`
	}

	// Try to parse as encrypted format
	if err := xml.Unmarshal(body, &encryptedMsg); err == nil && encryptedMsg.Encrypt != "" {
		if !c.verifySignature(c.config.Token, timestamp, nonce, encryptedMsg.Encrypt, signature) {
			c.log.Warn("Invalid signature for POST")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Decrypt message
		decryptedBody, err := c.decryptMsg(encryptedMsg.Encrypt)
		if err != nil {
			c.log.Error("Failed to decrypt message", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body = decryptedBody
	}

	// Parse plain XML message
	var msg struct {
		XMLName      xml.Name `xml:"xml"`
		ToUserName   string   `xml:"ToUserName"`
		FromUserName string   `xml:"FromUserName"`
		CreateTime   int64    `xml:"CreateTime"`
		MsgType      string   `xml:"MsgType"`
		Content      string   `xml:"Content"`
		MsgId        string   `xml:"MsgId"`
		AgentID      string   `xml:"AgentID"`
	}

	if err := xml.Unmarshal(body, &msg); err != nil {
		c.log.Error("Failed to unmarshal XML", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Check authorization
	if !c.isAllowed(msg.FromUserName) {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Process text messages
	if msg.MsgType == "text" {
		c.processMessage(msg.FromUserName, msg.Content, msg.MsgId)
	}

	w.WriteHeader(http.StatusOK)
}

// processMessage processes an incoming message.
func (c *Channel) processMessage(senderID, content, messageID string) {
	c.log.Info("WeWork message received",
		zap.String("sender_id", senderID))

	// Check for slash commands
	if c.commands.IsCommand(content) {
		c.handleCommand(senderID, content)
		return
	}

	// Create bus message
	msg := &bus.Message{
		ID:        fmt.Sprintf("wework:%s", messageID),
		ChannelID: "wework",
		SessionID: fmt.Sprintf("wework:%s", senderID),
		UserID:    senderID,
		Username:  senderID,
		Type:      bus.MessageTypeText,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleCommand processes a command message.
func (c *Channel) handleCommand(senderID, content string) {
	cmdName, args := c.commands.Parse(content)

	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		c.log.Debug("Unknown command", zap.String("command", cmdName))
		return
	}

	c.log.Info("Executing command",
		zap.String("command", cmdName),
		zap.String("user", senderID))

	// Create command request
	req := commands.CommandRequest{
		Channel:  "wework",
		ChatID:   senderID,
		UserID:   senderID,
		Username: senderID,
		Command:  cmdName,
		Args:     args,
	}

	// Execute command
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		c.log.Error("Command execution failed",
			zap.String("command", cmdName),
			zap.Error(err))

		// Send error response
		c.sendMessageToUser(senderID, "❌ Command failed: "+err.Error())
		return
	}

	// Send response
	c.sendMessageToUser(senderID, resp.Content)
}

// handleOutbound handles outbound messages from the bus.
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to WeWork.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	// Extract user ID from session ID (format: "wework:user_id")
	userID := msg.SessionID
	if len(userID) > 7 && userID[:7] == "wework:" {
		userID = userID[7:]
	}

	return c.sendMessageToUser(userID, msg.Content)
}

// sendMessageToUser sends a message to a specific user.
func (c *Channel) sendMessageToUser(userID, content string) error {
	token, err := c.getAccessToken()
	if err != nil {
		return fmt.Errorf("getting access token: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	payload := map[string]interface{}{
		"touser":  userID,
		"msgtype": "text",
		"agentid": c.config.AgentID,
		"text": map[string]string{
			"content": content,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("wework api error: %s", result.ErrMsg)
	}

	c.log.Debug("Sent WeWork message", zap.String("user_id", userID))
	return nil
}

// getAccessToken retrieves or refreshes the access token.
func (c *Channel) getAccessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Unix() < c.tokenExpiresAt {
		return c.accessToken, nil
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		c.config.CorpID, c.config.CorpSecret)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("json decode failed: %w", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("wework api error: %s", result.ErrMsg)
	}

	c.accessToken = result.AccessToken
	c.tokenExpiresAt = time.Now().Unix() + result.ExpiresIn - 200
	return c.accessToken, nil
}

// decryptMsg decrypts an encrypted message.
func (c *Channel) decryptMsg(encrypted string) ([]byte, error) {
	if c.config.EncodingAESKey == "" {
		return nil, fmt.Errorf("encoding_aes_key not configured")
	}

	// Base64 decode
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	// Decode key
	key := c.config.EncodingAESKey + "="
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("key decode failed: %w", err)
	}

	// AES-CBC decrypt
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("aes cipher failed: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// Remove PKCS7 padding
	padding := int(ciphertext[len(ciphertext)-1])
	if padding < 1 || padding > aes.BlockSize {
		return nil, fmt.Errorf("invalid padding")
	}
	ciphertext = ciphertext[:len(ciphertext)-padding]

	// Remove 16-byte random prefix
	if len(ciphertext) < 16 {
		return nil, fmt.Errorf("decrypted text too short")
	}
	content := ciphertext[16:]

	// Read 4-byte message length
	if len(content) < 4 {
		return nil, fmt.Errorf("content too short for length header")
	}
	msgLen := int(content[0])<<24 | int(content[1])<<16 | int(content[2])<<8 | int(content[3])

	if len(content) < 4+msgLen {
		return nil, fmt.Errorf("content too short for message")
	}

	// Extract message
	message := content[4 : 4+msgLen]

	// Verify CorpID
	if len(content) < 4+msgLen+len(c.config.CorpID) {
		return nil, fmt.Errorf("content too short for corp_id")
	}
	receivedCorpID := string(content[4+msgLen:])
	if receivedCorpID != c.config.CorpID {
		return nil, fmt.Errorf("corp_id mismatch")
	}

	return message, nil
}

// computeSignature computes SHA1 signature.
func (c *Channel) computeSignature(token, timestamp, nonce, data string) string {
	strs := []string{token, timestamp, nonce, data}
	sort.Strings(strs)
	str := strings.Join(strs, "")
	h := sha1.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// verifySignature verifies the signature.
func (c *Channel) verifySignature(token, timestamp, nonce, data, signature string) bool {
	expected := c.computeSignature(token, timestamp, nonce, data)
	return expected == signature
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
