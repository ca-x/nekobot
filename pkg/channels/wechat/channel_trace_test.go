package wechat

import (
	"strings"
	"testing"

	"nekobot/pkg/agent"
	"nekobot/pkg/channeltrace"
)

func TestFormatToolCallTraceIncludesAssistantToolCallsAndToolResults(t *testing.T) {
	messages := []agent.Message{
		{
			Role: "assistant",
			ToolCalls: []agent.ToolCall{
				{ID: "call-1", Name: "read_file", Arguments: map[string]interface{}{"path": "README.md"}},
				{ID: "call-2", Name: "exec"},
			},
		},
		{
			Role:       "tool",
			ToolCallID: "call-1",
			Content:    "README content",
		},
	}
	trace := formatToolCallTrace(messages)

	if !strings.Contains(trace, "Tool call: read_file") {
		t.Fatalf("expected read_file tool call in trace, got %q", trace)
	}
	if !strings.Contains(trace, "README.md") {
		t.Fatalf("expected summarized args in trace, got %q", trace)
	}
	if !strings.Contains(trace, "Tool call: exec") {
		t.Fatalf("expected exec tool call in trace, got %q", trace)
	}
	if !strings.Contains(trace, "Tool result: call-1") {
		t.Fatalf("expected tool result in trace, got %q", trace)
	}
	if trace != channeltrace.FormatToolCallTrace(messages) {
		t.Fatalf("expected wechat trace wrapper to match shared formatter, got %q", trace)
	}
}

func TestFormatToolCallTraceSkipsNonToolMessages(t *testing.T) {
	trace := formatToolCallTrace([]agent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "plain reply"},
	})
	if trace != "" {
		t.Fatalf("expected empty trace, got %q", trace)
	}
}
