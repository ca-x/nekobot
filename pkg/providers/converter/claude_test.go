package converter

import (
	"encoding/json"
	"strings"
	"testing"

	"nekobot/pkg/providers"
)

func TestToProviderRequest_Basic(t *testing.T) {
	c := NewClaudeConverter()

	req := &providers.UnifiedRequest{
		Model:       "claude-sonnet-4-5-20250929",
		Messages:    []providers.UnifiedMessage{{Role: "user", Content: "Hello"}},
		MaxTokens:   1024,
		Temperature: 0.7,
	}

	result, err := c.ToProviderRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := json.Marshal(result)
	var claudeReq claudeRequest
	json.Unmarshal(data, &claudeReq)

	if claudeReq.Model != "claude-sonnet-4-5-20250929" {
		t.Fatalf("expected model claude-sonnet-4-5-20250929, got %s", claudeReq.Model)
	}
	if claudeReq.MaxTokens != 1024 {
		t.Fatalf("expected max_tokens 1024, got %d", claudeReq.MaxTokens)
	}
	if claudeReq.Thinking != nil {
		t.Fatal("expected no thinking config")
	}
}

func TestToProviderRequest_ExtendedThinking(t *testing.T) {
	c := NewClaudeConverter()

	req := &providers.UnifiedRequest{
		Model:       "claude-sonnet-4-5-20250929",
		Messages:    []providers.UnifiedMessage{{Role: "user", Content: "Think about this"}},
		MaxTokens:   8192,
		Temperature: 0.7,
		Extra: map[string]interface{}{
			"extended_thinking": true,
			"thinking_budget":   16000,
		},
	}

	result, err := c.ToProviderRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	// Marshal to JSON to check thinking field
	data, _ := json.Marshal(result)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	thinking, ok := raw["thinking"].(map[string]interface{})
	if !ok {
		t.Fatal("expected thinking config in request")
	}
	if thinking["type"] != "enabled" {
		t.Fatalf("expected thinking type 'enabled', got %v", thinking["type"])
	}
	budget, _ := thinking["budget_tokens"].(float64)
	if int(budget) != 16000 {
		t.Fatalf("expected budget 16000, got %v", budget)
	}

	// Temperature and TopP should be zeroed (omitted via omitempty)
	if _, hasTemp := raw["temperature"]; hasTemp {
		t.Fatal("temperature should be omitted when thinking is enabled")
	}
}

func TestToProviderRequest_ExtendedThinkingDefaultBudget(t *testing.T) {
	c := NewClaudeConverter()

	req := &providers.UnifiedRequest{
		Model:    "claude-sonnet-4-5-20250929",
		Messages: []providers.UnifiedMessage{{Role: "user", Content: "Hi"}},
		Extra: map[string]interface{}{
			"extended_thinking": true,
			// No thinking_budget specified — should default to 10000
		},
	}

	result, err := c.ToProviderRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := json.Marshal(result)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	thinking := raw["thinking"].(map[string]interface{})
	budget, _ := thinking["budget_tokens"].(float64)
	if int(budget) != 10000 {
		t.Fatalf("expected default budget 10000, got %v", budget)
	}
}

func TestToProviderRequest_SystemMessage(t *testing.T) {
	c := NewClaudeConverter()

	req := &providers.UnifiedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Messages: []providers.UnifiedMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hi"},
		},
		MaxTokens: 1024,
	}

	result, err := c.ToProviderRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := json.Marshal(result)
	var claudeReq claudeRequest
	json.Unmarshal(data, &claudeReq)

	if claudeReq.System != "You are a helpful assistant." {
		t.Fatalf("expected system message, got %q", claudeReq.System)
	}
	// Messages should only contain user message (system extracted)
	if len(claudeReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(claudeReq.Messages))
	}
}

func TestToProviderRequest_ToolResult(t *testing.T) {
	c := NewClaudeConverter()

	req := &providers.UnifiedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Messages: []providers.UnifiedMessage{
			{Role: "user", Content: "List files"},
			{Role: "assistant", ToolCalls: []providers.UnifiedToolCall{
				{ID: "tc_1", Name: "list_dir", Arguments: map[string]interface{}{"path": "."}},
			}},
			{Role: "tool", Content: "file1.go\nfile2.go", ToolCallID: "tc_1"},
		},
		MaxTokens: 1024,
	}

	result, err := c.ToProviderRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := json.Marshal(result)
	var claudeReq claudeRequest
	json.Unmarshal(data, &claudeReq)

	if len(claudeReq.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(claudeReq.Messages))
	}

	// Tool result should be mapped to user role with tool_result block
	toolMsg := claudeReq.Messages[2]
	if toolMsg.Role != "user" {
		t.Fatalf("expected tool result as user role, got %s", toolMsg.Role)
	}
	if len(toolMsg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(toolMsg.Content))
	}
	if toolMsg.Content[0]["type"] != "tool_result" {
		t.Fatalf("expected tool_result block type, got %v", toolMsg.Content[0]["type"])
	}
}

func TestFromProviderResponse_WithThinking(t *testing.T) {
	c := NewClaudeConverter()

	resp := map[string]interface{}{
		"id":   "msg_123",
		"type": "message",
		"role": "assistant",
		"content": []interface{}{
			map[string]interface{}{
				"type":     "thinking",
				"thinking": "Let me analyze this step by step...",
			},
			map[string]interface{}{
				"type": "text",
				"text": "Here's the answer.",
			},
		},
		"model":       "claude-sonnet-4-5-20250929",
		"stop_reason": "end_turn",
		"usage": map[string]interface{}{
			"input_tokens":  float64(100),
			"output_tokens": float64(200),
		},
	}

	unified, err := c.FromProviderResponse(resp)
	if err != nil {
		t.Fatal(err)
	}

	if unified.Thinking != "Let me analyze this step by step..." {
		t.Fatalf("expected thinking content, got %q", unified.Thinking)
	}
	if unified.Content != "Here's the answer." {
		t.Fatalf("expected text content, got %q", unified.Content)
	}
	if unified.FinishReason != "stop" {
		t.Fatalf("expected finish_reason stop, got %s", unified.FinishReason)
	}
}

func TestFromProviderResponse_ToolUse(t *testing.T) {
	c := NewClaudeConverter()

	resp := map[string]interface{}{
		"id":   "msg_456",
		"type": "message",
		"role": "assistant",
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "I'll list the files.",
			},
			map[string]interface{}{
				"type":  "tool_use",
				"id":    "tc_1",
				"name":  "list_dir",
				"input": map[string]interface{}{"path": "."},
			},
		},
		"model":       "claude-sonnet-4-5-20250929",
		"stop_reason": "tool_use",
		"usage": map[string]interface{}{
			"input_tokens":  float64(50),
			"output_tokens": float64(100),
		},
	}

	unified, err := c.FromProviderResponse(resp)
	if err != nil {
		t.Fatal(err)
	}

	if unified.FinishReason != "tool_calls" {
		t.Fatalf("expected finish_reason tool_calls, got %s", unified.FinishReason)
	}
	if len(unified.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(unified.ToolCalls))
	}
	if unified.ToolCalls[0].Name != "list_dir" {
		t.Fatalf("expected tool name list_dir, got %s", unified.ToolCalls[0].Name)
	}
}

func TestFromProviderStreamChunk_ThinkingDelta(t *testing.T) {
	c := NewClaudeConverter()

	chunk := `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"step 1..."}}`

	unified, err := c.FromProviderStreamChunk([]byte(chunk))
	if err != nil {
		t.Fatal(err)
	}
	if unified.Delta.Thinking != "step 1..." {
		t.Fatalf("expected thinking delta, got %q", unified.Delta.Thinking)
	}
}

func TestFromProviderStreamChunk_TextDelta(t *testing.T) {
	c := NewClaudeConverter()

	chunk := `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Hello!"}}`

	unified, err := c.FromProviderStreamChunk([]byte(chunk))
	if err != nil {
		t.Fatal(err)
	}
	if unified.Delta.Content != "Hello!" {
		t.Fatalf("expected text delta 'Hello!', got %q", unified.Delta.Content)
	}
}

func TestFromProviderStreamChunk_SSEFormat(t *testing.T) {
	c := NewClaudeConverter()

	// Claude SSE format with "data: " prefix
	chunk := `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`

	unified, err := c.FromProviderStreamChunk([]byte(chunk))
	if err != nil {
		t.Fatal(err)
	}
	if unified.Delta.Content != "Hi" {
		t.Fatalf("expected text Hi, got %q", unified.Delta.Content)
	}
}

func TestFromProviderStreamChunk_MessageStop(t *testing.T) {
	c := NewClaudeConverter()

	chunk := `{"type":"message_stop"}`
	unified, err := c.FromProviderStreamChunk([]byte(chunk))
	if err != nil {
		t.Fatal(err)
	}
	if unified != nil {
		t.Fatal("expected nil for message_stop")
	}
}

func TestFromProviderStreamChunk_MessageDelta(t *testing.T) {
	c := NewClaudeConverter()

	chunk := `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":150}}`

	unified, err := c.FromProviderStreamChunk([]byte(chunk))
	if err != nil {
		t.Fatal(err)
	}
	if unified.FinishReason != "stop" {
		t.Fatalf("expected finish_reason stop, got %s", unified.FinishReason)
	}
	if unified.Usage == nil || unified.Usage.CompletionTokens != 150 {
		t.Fatal("expected usage with 150 completion tokens")
	}
}

func TestToProviderRequest_DefaultMaxTokens(t *testing.T) {
	c := NewClaudeConverter()

	req := &providers.UnifiedRequest{
		Model:    "claude-sonnet-4-5-20250929",
		Messages: []providers.UnifiedMessage{{Role: "user", Content: "Hi"}},
		// MaxTokens is 0 (not set)
	}

	result, err := c.ToProviderRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := json.Marshal(result)
	if !strings.Contains(string(data), `"max_tokens":4096`) {
		t.Fatal("expected default max_tokens of 4096")
	}
}
