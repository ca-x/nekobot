package wechat

import (
	"strings"
	"testing"
)

func TestBuildWeChatAgentInput(t *testing.T) {
	input := buildWeChatAgentInput("请总结这个错误", "/tmp/nekobot-workspace")

	if !strings.Contains(input, "[WeChat Channel Instructions]") {
		t.Fatalf("expected channel instruction header, got %q", input)
	}
	if !strings.Contains(input, "WeChat does not render Markdown") {
		t.Fatalf("expected presenter instructions, got %q", input)
	}
	if !strings.Contains(input, "Preferred workspace root: /tmp/nekobot-workspace") {
		t.Fatalf("expected workspace hint, got %q", input)
	}
	if !strings.Contains(input, "/select N") {
		t.Fatalf("expected select guidance, got %q", input)
	}
	if !strings.Contains(input, "[User Message]\n请总结这个错误") {
		t.Fatalf("expected user message section, got %q", input)
	}
}

func TestBuildWeChatAgentInput_EmptyContent(t *testing.T) {
	input := buildWeChatAgentInput("   ", "/tmp/work")
	if !strings.Contains(input, "WeChat does not render Markdown") {
		t.Fatalf("expected presenter instructions, got %q", input)
	}
	if !strings.Contains(input, "Preferred workspace root: /tmp/work") {
		t.Fatalf("expected workspace hint for empty content, got %q", input)
	}
	if !strings.Contains(input, "1. ...") || !strings.Contains(input, "/select N") {
		t.Fatalf("expected numbered-choice presenter guidance, got %q", input)
	}
}

func TestFormatForConfirmationIncludesSelectHints(t *testing.T) {
	formatted := FormatForConfirmation("是否继续？")
	if !strings.Contains(formatted, "/yes") || !strings.Contains(formatted, "/no") {
		t.Fatalf("expected yes/no hints, got %q", formatted)
	}
	if !strings.Contains(formatted, "/select 1") || !strings.Contains(formatted, "/select 2") {
		t.Fatalf("expected /select hints, got %q", formatted)
	}
	if !strings.Contains(formatted, "直接回复 1、2") {
		t.Fatalf("expected numeric reply hint, got %q", formatted)
	}
}
