package policy

import "testing"

func TestEvaluateToolMatchesGlobAllowlist(t *testing.T) {
	result := Evaluate(Policy{
		Tools: ToolsPolicy{Allow: []string{"claude*"}},
	}, EvaluationInput{ToolName: "claude-code"})

	if !result.Allowed {
		t.Fatalf("expected allowlist glob to allow tool, got %+v", result)
	}
	if result.Area != "tool" {
		t.Fatalf("expected tool area, got %+v", result)
	}
}

func TestEvaluateToolMatchesGlobDenyBeforeAllow(t *testing.T) {
	result := Evaluate(Policy{
		Tools: ToolsPolicy{
			Allow: []string{"*"},
			Deny:  []string{"mcp:*"},
		},
	}, EvaluationInput{ToolName: "mcp:browser"})

	if result.Allowed {
		t.Fatalf("expected deny glob to block tool, got %+v", result)
	}
	if result.Area != "tool" {
		t.Fatalf("expected tool area, got %+v", result)
	}
}

func TestEvaluatePathAllowsDirectoryPrefix(t *testing.T) {
	result := Evaluate(Policy{
		Filesystem: FSPolicy{
			AllowRead: []string{"/workspace"},
		},
	}, EvaluationInput{Path: "/workspace/project/README.md"})

	if !result.Allowed {
		t.Fatalf("expected directory prefix allowlist to allow read, got %+v", result)
	}
	if result.Area != "filesystem" {
		t.Fatalf("expected filesystem area, got %+v", result)
	}
}

func TestEvaluatePathDeniesBasenamePattern(t *testing.T) {
	result := Evaluate(Policy{
		Filesystem: FSPolicy{
			DenyRead: []string{"secret.txt"},
		},
	}, EvaluationInput{Path: "/tmp/nested/secret.txt"})

	if result.Allowed {
		t.Fatalf("expected basename denylist to block read, got %+v", result)
	}
	if result.Area != "filesystem" {
		t.Fatalf("expected filesystem area, got %+v", result)
	}
}

func TestEvaluateNetworkAllowsPathPrefix(t *testing.T) {
	result := Evaluate(Policy{
		Network: NetPolicy{
			Mode: "allowlist",
			Outbound: []NetRule{
				{
					Host:    "api.openai.com",
					Methods: []string{"POST"},
					Paths:   []string{"/v1"},
				},
			},
		},
	}, EvaluationInput{
		Host:    "api.openai.com",
		Method:  "post",
		URLPath: "/v1/chat/completions",
	})

	if !result.Allowed {
		t.Fatalf("expected path prefix allowlist to allow network request, got %+v", result)
	}
	if result.Area != "network" {
		t.Fatalf("expected network area, got %+v", result)
	}
}
