# Channels Implementation Status

## Summary

Nanobot supports multi-channel message routing through a unified bus system. Each channel implements the `Channel` interface and can send/receive messages independently.

## Completed Channels (3/8)

### ✅ Telegram
- **Status**: Complete with slash commands
- **SDK**: github.com/go-telegram-bot-api/telegram-bot-api/v5
- **Features**: Polling mode, inline commands, authorization
- **File**: `pkg/channels/telegram/telegram.go`

### ✅ Discord
- **Status**: Complete with slash commands
- **SDK**: github.com/bwmarrin/discordgo
- **Features**: WebSocket, intents, guild messages, slash commands
- **File**: `pkg/channels/discord/discord.go`

### ✅ Slack
- **Status**: Complete with slash commands
- **SDK**: github.com/slack-go/slack
- **Features**: Socket Mode, Events API, slash commands, ephemeral messages
- **File**: `pkg/channels/slack/slack.go`

## Pending Channels (5/8)

### ⏳ WhatsApp
- **Type**: WebSocket bridge
- **Config Fields**: `BridgeURL`, `AllowFrom`
- **Implementation**: Connect to external WhatsApp bridge via WebSocket
- **Proxy**: Not needed (handled by bridge)
- **Reference**: `~/code/go/picoclaw/pkg/channels/whatsapp.go`

### ⏳ Feishu (飞书/Lark)
- **Type**: HTTP API + Event callback
- **Config Fields**: `AppID`, `AppSecret`, `EncryptKey`, `VerificationToken`, `AllowFrom`
- **Implementation**: Enterprise messaging app (China)
- **Proxy**: May be needed
- **Reference**: `~/code/go/picoclaw/pkg/channels/feishu.go`

### ⏳ QQ
- **Type**: HTTP API
- **Config Fields**: `AppID`, `AppSecret`, `AllowFrom`
- **Implementation**: QQ Official Bot API
- **Proxy**: May be needed
- **Reference**: `~/code/go/picoclaw/pkg/channels/qq.go`

### ⏳ DingTalk (钉钉)
- **Type**: HTTP webhook
- **Config Fields**: `ClientID`, `ClientSecret`, `AllowFrom`
- **Implementation**: Enterprise IM (China)
- **Proxy**: May be needed
- **Reference**: `~/code/go/picoclaw/pkg/channels/dingtalk.go`

### ⏳ MaixCAM
- **Type**: HTTP server
- **Config Fields**: `Host`, `Port`, `AllowFrom`
- **Implementation**: Embedded device communication
- **Proxy**: Not needed (local device)
- **Reference**: `~/code/go/picoclaw/pkg/channels/maixcam.go`

## Channel Interface

All channels must implement:

```go
type Channel interface {
    ID() string                                    // Unique identifier
    Name() string                                  // Human-readable name
    Start(ctx context.Context) error              // Start listening
    Stop(ctx context.Context) error               // Graceful shutdown
    IsEnabled() bool                               // Check if enabled
    SendMessage(ctx context.Context, msg *bus.Message) error  // Send message
}
```

## Implementation Pattern

### 1. Channel Structure

```go
package channelname

type Channel struct {
    log      *logger.Logger
    config   config.ChannelNameConfig
    bus      bus.Bus
    commands *commands.Registry
    running  bool
    // ... channel-specific fields
}
```

### 2. Constructor

```go
func NewChannel(
    log *logger.Logger,
    cfg config.ChannelNameConfig,
    b bus.Bus,
    cmdRegistry *commands.Registry,
) (*Channel, error) {
    // Initialize channel-specific clients/connections
    return &Channel{
        log:      log,
        config:   cfg,
        bus:      b,
        commands: cmdRegistry,
        running:  false,
    }, nil
}
```

### 3. Start Method

```go
func (c *Channel) Start(ctx context.Context) error {
    c.log.Info("Starting ChannelName channel")

    // Register outbound message handler
    c.bus.RegisterHandler("channelname", c.handleOutbound)

    // Start listening for inbound messages
    go c.listen(ctx)

    c.running = true
    return nil
}
```

### 4. Stop Method

```go
func (c *Channel) Stop(ctx context.Context) error {
    c.log.Info("Stopping ChannelName channel")
    c.running = false

    // Unregister handler
    c.bus.UnregisterHandlers("channelname")

    // Clean up connections
    return nil
}
```

### 5. Inbound Messages

```go
func (c *Channel) handleInbound(rawMsg interface{}) {
    // Check authorization
    if !c.isAllowed(userID) {
        return
    }

    // Check for slash commands
    if c.commands.IsCommand(content) {
        c.handleCommand(...)
        return
    }

    // Create bus message
    msg := &bus.Message{
        ID:        fmt.Sprintf("channelname:%s", messageID),
        ChannelID: "channelname",
        SessionID: fmt.Sprintf("channelname:%s", chatID),
        UserID:    userID,
        Username:  username,
        Type:      bus.MessageTypeText,
        Content:   content,
        Timestamp: time.Now(),
    }

    // Send to bus
    c.bus.SendInbound(msg)
}
```

### 6. Outbound Messages

```go
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
    return c.SendMessage(ctx, msg)
}

func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
    // Extract chat ID from session ID
    chatID := extractChatID(msg.SessionID)

    // Send using channel-specific API
    return c.client.Send(chatID, msg.Content)
}
```

### 7. Authorization

```go
func (c *Channel) isAllowed(userID string) bool {
    if len(c.config.AllowFrom) == 0 {
        return true  // Allow all if not configured
    }

    for _, allowed := range c.config.AllowFrom {
        if allowed == userID || allowed == "*" {
            return true
        }
    }

    return false
}
```

### 8. Slash Commands

```go
func (c *Channel) handleCommand(rawMsg interface{}) {
    cmdName, args := c.commands.Parse(content)

    cmd, exists := c.commands.Get(cmdName)
    if !exists {
        return
    }

    req := commands.CommandRequest{
        Channel:  "channelname",
        ChatID:   chatID,
        UserID:   userID,
        Username: username,
        Command:  cmdName,
        Args:     args,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    resp, err := cmd.Handler(ctx, req)
    if err != nil {
        // Send error response
        return
    }

    // Send command response
}
```

## Registration in fx.go

```go
// Register ChannelName channel
if cfg.Channels.ChannelName.Enabled {
    channel, err := channelname.NewChannel(log, cfg.Channels.ChannelName, messageBus, cmdRegistry)
    if err != nil {
        log.Warn("Failed to create ChannelName channel, skipping", zap.Error(err))
    } else {
        if err := manager.Register(channel); err != nil {
            return err
        }
    }
}
```

## Proxy Support

For channels that support proxy configuration:

```go
// If SDK supports proxy
if cfg.Proxy != "" {
    proxyURL, err := url.Parse(cfg.Proxy)
    if err != nil {
        return nil, fmt.Errorf("invalid proxy URL: %w", err)
    }

    client := &http.Client{
        Transport: &http.Transport{
            Proxy: http.ProxyURL(proxyURL),
        },
    }

    // Use client with proxy
}
```

## Testing Checklist

For each channel implementation:

- [ ] Authorization working (`isAllowed`)
- [ ] Inbound messages sent to bus
- [ ] Outbound messages received from bus
- [ ] Slash commands detected and executed
- [ ] Message formatting correct
- [ ] Session ID format consistent
- [ ] Proxy configuration (if applicable)
- [ ] Graceful shutdown
- [ ] Error handling
- [ ] Logging
