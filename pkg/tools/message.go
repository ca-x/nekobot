package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nekobot/pkg/bus"
	"nekobot/pkg/logger"
)

// MessageTool sends messages directly to users via the message bus.
type MessageTool struct {
	log         *logger.Logger
	bus         bus.Bus
	currentChan string
	currentChat string
}

// NewMessageTool creates a new message tool.
func NewMessageTool(log *logger.Logger, msgBus bus.Bus) *MessageTool {
	return &MessageTool{
		log: log,
		bus: msgBus,
	}
}

// SetCurrent sets the current channel and chat ID context.
func (t *MessageTool) SetCurrent(channel, chatID string) {
	t.currentChan = channel
	t.currentChat = chatID
}

// Name returns the tool name.
func (t *MessageTool) Name() string {
	return "message"
}

// Description returns the tool description.
func (t *MessageTool) Description() string {
	return `Send a message directly to the user. Use this tool to:
- Provide status updates during long operations
- Ask clarifying questions
- Show progress or intermediate results
- Communicate errors or warnings

DO NOT send generic LLM refusal messages or meta-commentary about being an AI.`
}

// Parameters returns the tool parameters schema.
func (t *MessageTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Message content to send to the user",
			},
			"channel": map[string]interface{}{
				"type":        "string",
				"description": "Target channel (optional, uses current if not specified)",
			},
			"chat_id": map[string]interface{}{
				"type":        "string",
				"description": "Target chat ID (optional, uses current if not specified)",
			},
		},
		"required": []string{"content"},
	}
}

// Execute sends a message to the user.
func (t *MessageTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	content, ok := params["content"].(string)
	if !ok {
		return "", fmt.Errorf("content parameter is required")
	}

	// Filter out unwanted content
	if t.isFilteredContent(content) {
		t.log.Warn("Message filtered",
			logger.Int("length", len(content)))
		return "Message was filtered and not sent", nil
	}

	// Get target channel and chat
	channel := t.currentChan
	if ch, ok := params["channel"].(string); ok && ch != "" {
		channel = ch
	}

	chatID := t.currentChat
	if cid, ok := params["chat_id"].(string); ok && cid != "" {
		chatID = cid
	}

	if channel == "" || chatID == "" {
		return "", fmt.Errorf("channel and chat_id must be set")
	}

	// Send message
	msg := &bus.Message{
		Channel:   channel,
		ChatID:    chatID,
		Content:   content,
		Timestamp: time.Now(),
		Direction: bus.DirectionOutbound,
	}

	if err := t.bus.SendOutbound(msg); err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	t.log.Info("Message sent",
		logger.String("channel", channel),
		logger.String("chat_id", chatID),
		logger.Int("length", len(content)))

	return fmt.Sprintf("Message sent to %s:%s", channel, chatID), nil
}

// isFilteredContent checks if content should be filtered out.
func (t *MessageTool) isFilteredContent(content string) bool {
	if strings.TrimSpace(content) == "" {
		return true
	}

	// Filter common LLM refusal patterns
	refusalPatterns := []string{
		"作为一个人工智能语言模型",
		"作为AI语言模型",
		"作为一个AI",
		"作为一个人工智能",
		"我还没有学习",
		"我无法回答",
		"我不能回答",
		"I'm sorry, but I cannot",
		"As an AI language model",
		"As an AI assistant",
		"I cannot answer",
		"I'm not able to answer",
		"I cannot provide",
		"I apologize, but I cannot",
		"I'm an AI",
		"I'm just an AI",
	}

	contentLower := strings.ToLower(content)
	for _, pattern := range refusalPatterns {
		if strings.Contains(contentLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
