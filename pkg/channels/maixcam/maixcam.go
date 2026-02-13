// Package maixcam provides MaixCAM device channel implementation.
package maixcam

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// MaixCamMessage represents a message from MaixCAM device.
type MaixCamMessage struct {
	Type      string                 `json:"type"`
	Tips      string                 `json:"tips"`
	Timestamp float64                `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Channel implements MaixCAM channel as a TCP server.
type Channel struct {
	log      *logger.Logger
	config   config.MaixCamConfig
	bus      bus.Bus
	commands *commands.Registry

	listener   net.Listener
	clients    map[net.Conn]bool
	clientsMux sync.RWMutex
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewChannel creates a new MaixCAM channel.
func NewChannel(
	log *logger.Logger,
	cfg config.MaixCamConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
) (*Channel, error) {
	if cfg.Port == 0 {
		cfg.Port = 8888 // Default port
	}

	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		commands: cmdRegistry,
		clients:  make(map[net.Conn]bool),
		running:  false,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "maixcam"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "MaixCAM"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the MaixCAM TCP server.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting MaixCAM channel server",
		zap.String("host", c.config.Host),
		zap.Int("port", c.config.Port))

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Register outbound message handler
	c.bus.RegisterHandler("maixcam", c.handleOutbound)

	// Start TCP listener
	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	c.listener = listener
	c.running = true

	c.log.Info("MaixCAM server listening",
		zap.String("address", addr))

	// Accept connections
	go c.acceptConnections()

	return nil
}

// Stop stops the MaixCAM server.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping MaixCAM channel")

	c.running = false

	if c.cancel != nil {
		c.cancel()
	}

	// Unregister handler
	c.bus.UnregisterHandlers("maixcam")

	// Close listener
	if c.listener != nil {
		c.listener.Close()
	}

	// Close all client connections
	c.clientsMux.Lock()
	defer c.clientsMux.Unlock()

	for conn := range c.clients {
		conn.Close()
	}
	c.clients = make(map[net.Conn]bool)

	c.log.Info("MaixCAM channel stopped")
	return nil
}

// acceptConnections accepts incoming TCP connections.
func (c *Channel) acceptConnections() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			conn, err := c.listener.Accept()
			if err != nil {
				if c.running {
					c.log.Error("Failed to accept connection", zap.Error(err))
				}
				return
			}

			c.log.Info("New MaixCAM device connected",
				zap.String("remote_addr", conn.RemoteAddr().String()))

			c.clientsMux.Lock()
			c.clients[conn] = true
			c.clientsMux.Unlock()

			go c.handleConnection(conn)
		}
	}
}

// handleConnection handles a single client connection.
func (c *Channel) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		c.clientsMux.Lock()
		delete(c.clients, conn)
		c.clientsMux.Unlock()
		c.log.Debug("MaixCAM connection closed")
	}()

	decoder := json.NewDecoder(conn)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			var msg MaixCamMessage
			if err := decoder.Decode(&msg); err != nil {
				if err.Error() != "EOF" {
					c.log.Error("Failed to decode message", zap.Error(err))
				}
				return
			}

			c.processMessage(msg, conn)
		}
	}
}

// processMessage processes a message from MaixCAM device.
func (c *Channel) processMessage(msg MaixCamMessage, conn net.Conn) {
	switch msg.Type {
	case "person_detected":
		c.handlePersonDetection(msg, conn)
	case "heartbeat":
		c.log.Debug("Received heartbeat from MaixCAM")
	case "status":
		c.handleStatusUpdate(msg)
	case "message":
		c.handleTextMessage(msg, conn)
	default:
		c.log.Warn("Unknown message type", zap.String("type", msg.Type))
	}
}

// handlePersonDetection handles person detection events.
func (c *Channel) handlePersonDetection(msg MaixCamMessage, conn net.Conn) {
	deviceID := conn.RemoteAddr().String()

	// Check authorization
	if !c.isAllowed(deviceID) {
		c.log.Warn("Unauthorized device",
			zap.String("device_id", deviceID))
		return
	}

	classInfo, _ := msg.Data["class_name"].(string)
	if classInfo == "" {
		classInfo = "person"
	}

	score, _ := msg.Data["score"].(float64)
	x, _ := msg.Data["x"].(float64)
	y, _ := msg.Data["y"].(float64)
	w, _ := msg.Data["w"].(float64)
	h, _ := msg.Data["h"].(float64)

	content := fmt.Sprintf("📷 Person detected!\nClass: %s\nConfidence: %.2f%%\nPosition: (%.0f, %.0f)\nSize: %.0fx%.0f",
		classInfo, score*100, x, y, w, h)

	// Create bus message
	busMsg := &bus.Message{
		ID:        fmt.Sprintf("maixcam:%.0f", msg.Timestamp),
		ChannelID: "maixcam",
		SessionID: fmt.Sprintf("maixcam:%s", deviceID),
		UserID:    deviceID,
		Username:  "MaixCAM",
		Type:      bus.MessageTypeText,
		Content:   content,
		Data: map[string]interface{}{
			"class_name": classInfo,
			"score":      score,
			"x":          x,
			"y":          y,
			"w":          w,
			"h":          h,
		},
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(busMsg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleStatusUpdate handles status update messages.
func (c *Channel) handleStatusUpdate(msg MaixCamMessage) {
	c.log.Info("Status update from MaixCAM",
		zap.Any("status", msg.Data))
}

// handleTextMessage handles text messages from device.
func (c *Channel) handleTextMessage(msg MaixCamMessage, conn net.Conn) {
	deviceID := conn.RemoteAddr().String()

	// Check authorization
	if !c.isAllowed(deviceID) {
		c.log.Warn("Unauthorized device",
			zap.String("device_id", deviceID))
		return
	}

	content, _ := msg.Data["content"].(string)
	if content == "" {
		return
	}

	// Check for slash commands
	if c.commands.IsCommand(content) {
		c.handleCommand(msg, deviceID, content)
		return
	}

	// Create bus message
	busMsg := &bus.Message{
		ID:        fmt.Sprintf("maixcam:%.0f", msg.Timestamp),
		ChannelID: "maixcam",
		SessionID: fmt.Sprintf("maixcam:%s", deviceID),
		UserID:    deviceID,
		Username:  "MaixCAM",
		Type:      bus.MessageTypeText,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(busMsg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleCommand processes a command message.
func (c *Channel) handleCommand(msg MaixCamMessage, deviceID, content string) {
	cmdName, args := c.commands.Parse(content)

	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		c.log.Debug("Unknown command", zap.String("command", cmdName))
		return
	}

	c.log.Info("Executing command",
		zap.String("command", cmdName),
		zap.String("device", deviceID))

	// Create command request
	req := commands.CommandRequest{
		Channel:  "maixcam",
		ChatID:   deviceID,
		UserID:   deviceID,
		Username: "MaixCAM",
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
		return
	}

	// Send response back to device (not implemented in this version)
	c.log.Debug("Command response", zap.String("content", resp.Content))
}

// handleOutbound handles outbound messages from the bus.
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to MaixCAM device(s).
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	// Extract device ID from session ID (format: "maixcam:device_addr")
	// For broadcast, send to all connected devices

	response := map[string]interface{}{
		"type":    "message",
		"content": msg.Content,
		"time":    time.Now().Unix(),
	}

	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	// Send to all connected devices
	c.clientsMux.RLock()
	defer c.clientsMux.RUnlock()

	for conn := range c.clients {
		if _, err := conn.Write(append(data, '\n')); err != nil {
			c.log.Error("Failed to send to device", zap.Error(err))
		}
	}

	c.log.Debug("Sent message to MaixCAM devices",
		zap.Int("device_count", len(c.clients)))

	return nil
}

// isAllowed checks if a device is allowed.
func (c *Channel) isAllowed(deviceID string) bool {
	if len(c.config.AllowFrom) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowFrom {
		if allowed == deviceID || allowed == "*" {
			return true
		}
	}

	return false
}
