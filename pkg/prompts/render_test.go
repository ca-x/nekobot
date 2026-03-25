package prompts

import (
	"strings"
	"testing"
)

func TestRenderPromptTemplate_SupportsShorthandAndDotRootSyntax(t *testing.T) {
	input := ResolveInput{
		Channel:           "wechat",
		SessionID:         "session-1",
		UserID:            "user-1",
		Username:          "alice",
		RequestedProvider: "openai",
		RequestedModel:    "gpt-4.1",
		RequestedFallback: []string{"anthropic", "gemini"},
		Workspace:         "/tmp/workspace",
		Custom: map[string]any{
			"team": "ops",
		},
	}

	rendered, err := renderPromptTemplate(
		"test-prompt",
		`channel={{channel.id}} session={{session.id}} user={{user.name}} provider={{route.provider}} workspace={{workspace.path}} team={{custom.team}} fallback={{index route.fallback 1}} dot={{.channel.id}}`,
		input,
	)
	if err != nil {
		t.Fatalf("renderPromptTemplate failed: %v", err)
	}

	expectedParts := []string{
		"channel=wechat",
		"session=session-1",
		"user=alice",
		"provider=openai",
		"workspace=/tmp/workspace",
		"team=ops",
		"fallback=gemini",
		"dot=wechat",
	}
	for _, part := range expectedParts {
		if !strings.Contains(rendered, part) {
			t.Fatalf("expected rendered template to contain %q, got %q", part, rendered)
		}
	}
}
