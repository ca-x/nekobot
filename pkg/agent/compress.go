package agent

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"nekobot/pkg/providers"
)

// estimateTokens estimates the number of tokens in a message list.
// Uses 2.5 characters per token heuristic (handles CJK better than 3-4 chars).
func estimateTokens(messages []providers.UnifiedMessage) int {
	totalChars := 0
	for _, m := range messages {
		totalChars += utf8.RuneCountInString(m.Content)
		for _, tc := range m.ToolCalls {
			totalChars += utf8.RuneCountInString(tc.Name)
			for k, v := range tc.Arguments {
				totalChars += utf8.RuneCountInString(k)
				totalChars += utf8.RuneCountInString(fmt.Sprint(v))
			}
		}
	}
	// 2.5 chars per token = totalChars * 2 / 5
	return totalChars * 2 / 5
}

// forceCompressMessages drops the oldest 50% of conversation messages
// to recover from context window errors. Preserves:
// - System prompt (first message)
// - The latest half of the conversation
// Appends a compression note to the system prompt.
func forceCompressMessages(messages []providers.UnifiedMessage) []providers.UnifiedMessage {
	if len(messages) <= 4 {
		return messages
	}

	// messages[0] is system prompt, messages[1:] is conversation
	conversation := messages[1:]
	if len(conversation) <= 2 {
		return messages
	}

	mid := len(conversation) / 2
	droppedCount := mid
	keptConversation := conversation[mid:]

	result := make([]providers.UnifiedMessage, 0, 1+len(keptConversation))

	// Append compression note to system prompt
	systemMsg := messages[0]
	systemMsg.Content += fmt.Sprintf(
		"\n\n[System Note: Emergency compression dropped %d oldest messages due to context limit. "+
			"Some earlier conversation context may be missing.]",
		droppedCount,
	)
	result = append(result, systemMsg)
	result = append(result, keptConversation...)

	return result
}

// isContextLimitError checks if an error is related to context window/token limits.
func isContextLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	patterns := []string{
		"token",
		"context length",
		"context window",
		"maximum context",
		"max_tokens",
		"too long",
		"too many tokens",
		"input too large",
		"request too large",
		"payload too large",
		"content length exceeded",
		"maximum.*length",
		"invalidparameter",
	}

	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}
