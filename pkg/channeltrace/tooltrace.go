package channeltrace

import (
	"encoding/json"
	"strings"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
)

// FormatToolCallTrace renders assistant tool calls and tool results into a
// compact multi-line trace suitable for channel replies.
func FormatToolCallTrace(messages []agent.Message) string {
	lines := make([]string, 0)
	for _, msg := range messages {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				name := strings.TrimSpace(tc.Name)
				if name == "" {
					continue
				}
				argSummary := summarizeToolCallArgs(tc.Arguments)
				if argSummary != "" {
					lines = append(lines, "Tool call: "+name+" "+argSummary)
				} else {
					lines = append(lines, "Tool call: "+name)
				}
			}
			continue
		}
		if msg.Role != "tool" {
			continue
		}
		id := strings.TrimSpace(msg.ToolCallID)
		if id == "" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			lines = append(lines, "Tool result: "+id)
			continue
		}
		lines = append(lines, "Tool result: "+id+" → "+summarizeToolResult(content))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// PrependToolCallTrace prefixes a rendered tool trace to the reply body when
// tool activity exists.
func PrependToolCallTrace(reply string, messages []agent.Message) string {
	trace := strings.TrimSpace(FormatToolCallTrace(messages))
	return PrependRawTrace(reply, trace)
}

// PrependRawTrace prefixes an already-rendered trace to the reply body.
func PrependRawTrace(reply string, trace string) string {
	trace = strings.TrimSpace(trace)
	body := strings.TrimSpace(reply)
	switch {
	case trace == "":
		return body
	case body == "":
		return trace
	default:
		return trace + "\n\n" + body
	}
}

// MessageToolCallTrace extracts a rendered tool trace from outbound bus metadata.
func MessageToolCallTrace(msg *bus.Message) string {
	if msg == nil || msg.Data == nil {
		return ""
	}
	raw, ok := msg.Data["tool_call_trace"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(raw)
}

// PrependBusToolTrace prefixes outbound bus tool trace metadata to a reply body.
func PrependBusToolTrace(reply string, msg *bus.Message) string {
	return PrependRawTrace(reply, MessageToolCallTrace(msg))
}

func summarizeToolCallArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return summarizeToolResult(string(raw))
}

func summarizeToolResult(content string) string {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) <= 120 {
		return trimmed
	}
	return trimmed[:117] + "..."
}
