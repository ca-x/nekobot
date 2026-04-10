package policy

import (
	"fmt"
	"path/filepath"
	"strings"
)

type EvaluationInput struct {
	ToolName string `json:"tool_name,omitempty"`
	Path     string `json:"path,omitempty"`
	Write    bool   `json:"write,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Method   string `json:"method,omitempty"`
	URLPath  string `json:"url_path,omitempty"`
}

type EvaluationResult struct {
	Allowed bool   `json:"allowed"`
	Area    string `json:"area"`
	Reason  string `json:"reason"`
}

func Evaluate(p Policy, in EvaluationInput) EvaluationResult {
	if strings.TrimSpace(in.ToolName) != "" {
		return evaluateTool(p, in.ToolName)
	}
	if strings.TrimSpace(in.Path) != "" {
		return evaluatePath(p, in.Path, in.Write)
	}
	if strings.TrimSpace(in.Host) != "" {
		return evaluateNetwork(p, in)
	}
	return EvaluationResult{Allowed: true, Area: "none", Reason: "no policy target provided"}
}

func evaluateTool(p Policy, toolName string) EvaluationResult {
	name := strings.TrimSpace(toolName)
	for _, denied := range p.Tools.Deny {
		if matchToolName(denied, name) {
			return EvaluationResult{Allowed: false, Area: "tool", Reason: fmt.Sprintf("tool %q denied", name)}
		}
	}
	if len(p.Tools.Allow) == 0 {
		return EvaluationResult{Allowed: true, Area: "tool", Reason: "no tool allowlist configured"}
	}
	for _, allowed := range p.Tools.Allow {
		if matchToolName(allowed, name) {
			return EvaluationResult{Allowed: true, Area: "tool", Reason: "matched tool allowlist"}
		}
	}
	return EvaluationResult{Allowed: false, Area: "tool", Reason: fmt.Sprintf("tool %q not allowed", name)}
}

func matchToolName(pattern, name string) bool {
	pattern = strings.TrimSpace(pattern)
	name = strings.TrimSpace(name)
	if pattern == "" || name == "" {
		return false
	}
	if pattern == "*" || pattern == name {
		return true
	}
	if matched, err := filepath.Match(pattern, name); err == nil && matched {
		return true
	}
	return strings.HasSuffix(pattern, "*") && strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
}

func evaluatePath(p Policy, rawPath string, write bool) EvaluationResult {
	path := filepath.Clean(strings.TrimSpace(rawPath))
	if write {
		for _, denied := range p.Filesystem.DenyWrite {
			if matchPath(denied, path) {
				return EvaluationResult{Allowed: false, Area: "filesystem", Reason: fmt.Sprintf("write denied by %q", denied)}
			}
		}
		if len(p.Filesystem.AllowWrite) == 0 {
			return EvaluationResult{Allowed: true, Area: "filesystem", Reason: "no write allowlist configured"}
		}
		for _, allowed := range p.Filesystem.AllowWrite {
			if matchPath(allowed, path) {
				return EvaluationResult{Allowed: true, Area: "filesystem", Reason: fmt.Sprintf("write allowed by %q", allowed)}
			}
		}
		return EvaluationResult{Allowed: false, Area: "filesystem", Reason: "write not allowed"}
	}
	for _, denied := range p.Filesystem.DenyRead {
		if matchPath(denied, path) {
			return EvaluationResult{Allowed: false, Area: "filesystem", Reason: fmt.Sprintf("read denied by %q", denied)}
		}
	}
	if len(p.Filesystem.AllowRead) == 0 {
		return EvaluationResult{Allowed: true, Area: "filesystem", Reason: "no read allowlist configured"}
	}
	for _, allowed := range p.Filesystem.AllowRead {
		if matchPath(allowed, path) {
			return EvaluationResult{Allowed: true, Area: "filesystem", Reason: fmt.Sprintf("read allowed by %q", allowed)}
		}
	}
	return EvaluationResult{Allowed: false, Area: "filesystem", Reason: "read not allowed"}
}

func evaluateNetwork(p Policy, in EvaluationInput) EvaluationResult {
	mode := strings.TrimSpace(strings.ToLower(p.Network.Mode))
	switch mode {
	case "", "permissive":
		return EvaluationResult{Allowed: true, Area: "network", Reason: "permissive mode"}
	case "none":
		return EvaluationResult{Allowed: false, Area: "network", Reason: "all network access denied"}
	case "allowlist":
		for _, rule := range p.Network.Outbound {
			if !hostMatches(rule.Host, in.Host) {
				continue
			}
			if len(rule.Ports) > 0 && !containsInt(rule.Ports, in.Port) {
				continue
			}
			if len(rule.Methods) > 0 && !containsFold(rule.Methods, in.Method) {
				continue
			}
			if len(rule.Paths) > 0 && !matchAny(rule.Paths, in.URLPath) {
				continue
			}
			return EvaluationResult{Allowed: true, Area: "network", Reason: "matched network allowlist"}
		}
		return EvaluationResult{Allowed: false, Area: "network", Reason: "network request not allowlisted"}
	default:
		return EvaluationResult{Allowed: false, Area: "network", Reason: fmt.Sprintf("unsupported network mode %q", mode)}
	}
}

func matchPath(pattern, path string) bool {
	pattern = strings.TrimSpace(pattern)
	path = filepath.Clean(strings.TrimSpace(path))
	if pattern == "" {
		return false
	}
	pattern = filepath.Clean(pattern)
	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}
	if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
		return true
	}
	if strings.HasSuffix(pattern, "*") && strings.HasPrefix(path, strings.TrimSuffix(pattern, "*")) {
		return true
	}
	if strings.HasPrefix(path, pattern) {
		if path == pattern {
			return true
		}
		if strings.HasSuffix(pattern, string(filepath.Separator)) {
			return true
		}
		return strings.HasPrefix(path, pattern+string(filepath.Separator))
	}
	return false
}

func matchAny(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if matchPath(pattern, path) {
			return true
		}
	}
	return false
}

func hostMatches(pattern, host string) bool {
	pattern = strings.TrimSpace(strings.ToLower(pattern))
	host = strings.TrimSpace(strings.ToLower(host))
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(host, suffix) || host == strings.TrimPrefix(pattern, "*.")
	}
	return pattern == host
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}
