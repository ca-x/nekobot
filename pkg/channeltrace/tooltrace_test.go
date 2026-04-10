package channeltrace

import (
	"strings"
	"testing"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
)

func TestFormatToolCallTraceIncludesAssistantToolCallsAndToolResults(t *testing.T) {
	trace := FormatToolCallTrace([]agent.Message{
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
	})

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
}

func TestFormatToolCallTraceSkipsNonToolMessages(t *testing.T) {
	trace := FormatToolCallTrace([]agent.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "plain reply"},
	})
	if trace != "" {
		t.Fatalf("expected empty trace, got %q", trace)
	}
}

func TestPrependToolCallTracePrependsTraceToReply(t *testing.T) {
	reply := PrependToolCallTrace("done", []agent.Message{
		{
			Role: "assistant",
			ToolCalls: []agent.ToolCall{
				{Name: "read_file", Arguments: map[string]interface{}{"path": "README.md"}},
			},
		},
	})

	if !strings.HasPrefix(reply, "Tool call: read_file") {
		t.Fatalf("expected prefixed trace, got %q", reply)
	}
	if !strings.Contains(reply, "\n\ndone") {
		t.Fatalf("expected reply body after blank line, got %q", reply)
	}
}

func TestPrependRawTracePrependsTraceToReply(t *testing.T) {
	reply := PrependRawTrace("done", "Tool call: read_file")
	if reply != "Tool call: read_file\n\ndone" {
		t.Fatalf("unexpected prepended reply: %q", reply)
	}
}

func TestMessageToolCallTraceReadsOutboundMetadata(t *testing.T) {
	msg := &bus.Message{
		Data: map[string]interface{}{
			"tool_call_trace": "Tool call: read_file {\"path\":\"README.md\"}",
		},
	}
	if got := MessageToolCallTrace(msg); !strings.Contains(got, "Tool call: read_file") {
		t.Fatalf("expected tool trace from bus message, got %q", got)
	}
	if got := MessageToolCallTrace(&bus.Message{Data: map[string]interface{}{"tool_call_trace": 1}}); got != "" {
		t.Fatalf("expected empty trace for non-string metadata, got %q", got)
	}
}

func TestPrependBusToolTracePrependsMessageTraceToReply(t *testing.T) {
	reply := PrependBusToolTrace("done", &bus.Message{
		Data: map[string]interface{}{
			"tool_call_trace": "Tool call: read_file",
		},
	})
	if reply != "Tool call: read_file\n\ndone" {
		t.Fatalf("unexpected prepended reply: %q", reply)
	}
}
