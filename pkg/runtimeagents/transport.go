package runtimeagents

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	MetadataRuntimeTransport = "runtime_transport"
	MetadataRuntimeSession   = "runtime_session"
	MetadataTmuxSession      = "tmux_session"
	MetadataLaunchCommand    = "launch_cmd"

	TransportTmux   = "tmux"
	TransportZellij = "zellij"
)

type LaunchInfo struct {
	TransportName string
	SessionName   string
	LaunchCommand string
}

type ReattachInfo struct {
	TransportName string
	SessionName   string
	LaunchCommand string
}

type RuntimeTransport interface {
	Name() string
	Available() bool
	WrapStart(command, sessionID string) LaunchInfo
	BuildReattach(sessionID string) (ReattachInfo, bool)
	KillSession(sessionID string)
}

type tmuxTransport struct{}
type zellijTransport struct{}

func DefaultTransport() RuntimeTransport {
	return tmuxTransport{}
}

func TransportByName(name string) RuntimeTransport {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case TransportZellij:
		return zellijTransport{}
	case TransportTmux:
		fallthrough
	default:
		return tmuxTransport{}
	}
}

func (tmuxTransport) Name() string {
	return TransportTmux
}

func (tmuxTransport) Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func (t tmuxTransport) WrapStart(command, sessionID string) LaunchInfo {
	if !t.Available() {
		return LaunchInfo{LaunchCommand: command}
	}
	name := TmuxSessionName(sessionID)
	wrapped := fmt.Sprintf("tmux new-session -A -s %s %s -c %s", name, strconv.Quote(toolShellPath()), strconv.Quote(command))
	return LaunchInfo{
		TransportName: t.Name(),
		SessionName:   name,
		LaunchCommand: wrapped,
	}
}

func (t tmuxTransport) BuildReattach(sessionID string) (ReattachInfo, bool) {
	if !t.Available() {
		return ReattachInfo{}, false
	}
	name := TmuxSessionName(sessionID)
	if exec.Command("tmux", "has-session", "-t", strings.TrimSpace(name)).Run() != nil {
		return ReattachInfo{}, false
	}
	return ReattachInfo{
		TransportName: t.Name(),
		SessionName:   name,
		LaunchCommand: fmt.Sprintf("tmux attach-session -t %s", name),
	}, true
}

func (t tmuxTransport) KillSession(sessionID string) {
	if !t.Available() {
		return
	}
	_ = exec.Command("tmux", "kill-session", "-t", TmuxSessionName(sessionID)).Run()
}

func (zellijTransport) Name() string {
	return TransportZellij
}

func (zellijTransport) Available() bool {
	_, err := exec.LookPath("zellij")
	return err == nil
}

func (t zellijTransport) WrapStart(command, sessionID string) LaunchInfo {
	if !t.Available() {
		return LaunchInfo{LaunchCommand: command}
	}
	name := TmuxSessionName(sessionID)
	shell := toolShellPath()
	ensureSession := fmt.Sprintf("zellij attach -b -c %s >/dev/null 2>&1", strconv.Quote(name))
	runCommand := fmt.Sprintf("zellij --session %s run --cwd . -- %s -lc %s", strconv.Quote(name), strconv.Quote(shell), strconv.Quote(command))
	return LaunchInfo{
		TransportName: t.Name(),
		SessionName:   name,
		LaunchCommand: ensureSession + " && " + runCommand,
	}
}

func (t zellijTransport) BuildReattach(sessionID string) (ReattachInfo, bool) {
	if !t.Available() {
		return ReattachInfo{}, false
	}
	name := TmuxSessionName(sessionID)
	if exec.Command("zellij", "--session", name, "action", "list-panes").Run() != nil {
		return ReattachInfo{}, false
	}
	return ReattachInfo{
		TransportName: t.Name(),
		SessionName:   name,
		LaunchCommand: fmt.Sprintf("zellij attach %s", strconv.Quote(name)),
	}, true
}

func (t zellijTransport) KillSession(sessionID string) {
	if !t.Available() {
		return
	}
	_ = exec.Command("zellij", "kill-session", TmuxSessionName(sessionID)).Run()
}

func ApplyLaunchMetadata(metadata map[string]interface{}, info LaunchInfo) map[string]interface{} {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata[MetadataLaunchCommand] = strings.TrimSpace(info.LaunchCommand)
	if strings.TrimSpace(info.TransportName) == "" {
		delete(metadata, MetadataRuntimeTransport)
		delete(metadata, MetadataRuntimeSession)
		delete(metadata, MetadataTmuxSession)
		return metadata
	}
	metadata[MetadataRuntimeTransport] = strings.TrimSpace(info.TransportName)
	metadata[MetadataRuntimeSession] = strings.TrimSpace(info.SessionName)
	if strings.TrimSpace(info.TransportName) == TransportTmux {
		metadata[MetadataTmuxSession] = strings.TrimSpace(info.SessionName)
	}
	return metadata
}

func MetadataString(values map[string]interface{}, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func TmuxSessionName(sessionID string) string {
	raw := strings.TrimSpace(strings.ToLower(sessionID))
	if raw == "" {
		return "nekobot_session"
	}
	var b strings.Builder
	b.WriteString("nekobot_")
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	name := b.String()
	if len(name) > 40 {
		name = name[:40]
	}
	if name == "nekobot_" {
		return "nekobot_session"
	}
	return name
}

func toolShellPath() string {
	candidates := []string{
		"/bin/sh",
		"/usr/bin/sh",
		"/bin/bash",
		"/usr/bin/bash",
		"/usr/local/bin/bash",
		"/bin/zsh",
		"/usr/bin/zsh",
		"/usr/local/bin/zsh",
		"/bin/ash",
		"/usr/bin/ash",
		"/system/bin/sh",
		"/usr/bin/fish",
		"/bin/fish",
		"/usr/local/bin/fish",
	}
	for _, path := range candidates {
		if !isExecutableShell(path) {
			continue
		}
		return path
	}
	lookupNames := []string{"sh", "bash", "zsh", "ash", "fish"}
	for _, name := range lookupNames {
		lookedUp, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if isExecutableShell(lookedUp) {
			return lookedUp
		}
	}
	return "sh"
}

func isExecutableShell(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}
