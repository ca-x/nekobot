package runtimeagents

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestApplyLaunchMetadataKeepsTmuxCompatibilityFields(t *testing.T) {
	metadata := ApplyLaunchMetadata(map[string]interface{}{}, LaunchInfo{
		TransportName: TransportTmux,
		SessionName:   "nekobot_123",
		LaunchCommand: "tmux attach-session -t nekobot_123",
	})

	if got, _ := metadata[MetadataRuntimeTransport].(string); got != TransportTmux {
		t.Fatalf("expected runtime transport %q, got %q", TransportTmux, got)
	}
	if got, _ := metadata[MetadataRuntimeSession].(string); got != "nekobot_123" {
		t.Fatalf("expected runtime session to be persisted, got %q", got)
	}
	if got, _ := metadata[MetadataTmuxSession].(string); got != "nekobot_123" {
		t.Fatalf("expected tmux session compatibility field, got %q", got)
	}
}

func TestApplyLaunchMetadataDoesNotInventTmuxCompatibilityForZellij(t *testing.T) {
	metadata := ApplyLaunchMetadata(map[string]interface{}{}, LaunchInfo{
		TransportName: TransportZellij,
		SessionName:   "nekobot_123",
		LaunchCommand: "zellij attach nekobot_123",
	})

	if got, _ := metadata[MetadataRuntimeTransport].(string); got != TransportZellij {
		t.Fatalf("expected runtime transport %q, got %q", TransportZellij, got)
	}
	if got, _ := metadata[MetadataRuntimeSession].(string); got != "nekobot_123" {
		t.Fatalf("expected runtime session to be persisted, got %q", got)
	}
	if _, exists := metadata[MetadataTmuxSession]; exists {
		t.Fatal("did not expect tmux compatibility field for zellij metadata")
	}
}

func TestApplyLaunchMetadataClearsStaleTmuxCompatibilityWhenSwitchingTransports(t *testing.T) {
	metadata := ApplyLaunchMetadata(map[string]interface{}{
		MetadataRuntimeTransport: TransportTmux,
		MetadataRuntimeSession:   "nekobot_old",
		MetadataTmuxSession:      "nekobot_old",
	}, LaunchInfo{
		TransportName: TransportZellij,
		SessionName:   "nekobot_new",
		LaunchCommand: "zellij attach nekobot_new",
	})

	if got, _ := metadata[MetadataRuntimeTransport].(string); got != TransportZellij {
		t.Fatalf("expected runtime transport %q, got %q", TransportZellij, got)
	}
	if got, _ := metadata[MetadataRuntimeSession].(string); got != "nekobot_new" {
		t.Fatalf("expected runtime session to be updated, got %q", got)
	}
	if _, exists := metadata[MetadataTmuxSession]; exists {
		t.Fatalf("expected stale tmux compatibility field to be removed, got %+v", metadata)
	}
}

func TestApplyRuntimeSessionPayloadUsesTransportAwareCompatibilityField(t *testing.T) {
	tmuxPayload := ApplyRuntimeSessionPayload(map[string]interface{}{}, TransportTmux, "nekobot_tmux")
	if got, _ := tmuxPayload[MetadataRuntimeSession].(string); got != "nekobot_tmux" {
		t.Fatalf("expected runtime session for tmux payload, got %+v", tmuxPayload)
	}
	if got, _ := tmuxPayload[MetadataTmuxSession].(string); got != "nekobot_tmux" {
		t.Fatalf("expected tmux compatibility field for tmux payload, got %+v", tmuxPayload)
	}

	zellijPayload := ApplyRuntimeSessionPayload(map[string]interface{}{
		MetadataTmuxSession: "stale",
	}, TransportZellij, "nekobot_zellij")
	if got, _ := zellijPayload[MetadataRuntimeSession].(string); got != "nekobot_zellij" {
		t.Fatalf("expected runtime session for zellij payload, got %+v", zellijPayload)
	}
	if _, exists := zellijPayload[MetadataTmuxSession]; exists {
		t.Fatalf("did not expect tmux compatibility field for zellij payload, got %+v", zellijPayload)
	}
}

func TestTransportByNameResolvesKnownBackends(t *testing.T) {
	if got := TransportByName(TransportTmux).Name(); got != TransportTmux {
		t.Fatalf("expected tmux transport, got %q", got)
	}
	if got := TransportByName(TransportZellij).Name(); got != TransportZellij {
		t.Fatalf("expected zellij transport, got %q", got)
	}
	if got := TransportByName("unknown").Name(); got != TransportTmux {
		t.Fatalf("expected unknown names to default to tmux, got %q", got)
	}
}

func TestZellijWrapStartBuildsSessionBootstrapCommand(t *testing.T) {
	transport := zellijTransport{}
	if !transport.Available() {
		t.Skip("zellij not available")
	}

	launchInfo := transport.WrapStart("sleep 1", "demo-session")
	if launchInfo.TransportName != TransportZellij {
		t.Fatalf("expected zellij transport, got %+v", launchInfo)
	}
	if launchInfo.SessionName == "" {
		t.Fatalf("expected runtime session name, got %+v", launchInfo)
	}
	if !strings.Contains(launchInfo.LaunchCommand, "zellij attach -b -c") {
		t.Fatalf("expected bootstrap attach command, got %q", launchInfo.LaunchCommand)
	}
	if !strings.Contains(launchInfo.LaunchCommand, "zellij --session") {
		t.Fatalf("expected zellij run command, got %q", launchInfo.LaunchCommand)
	}
}

func TestZellijKillSessionDeletesResurrectableSession(t *testing.T) {
	transport := zellijTransport{}
	if !transport.Available() {
		t.Skip("zellij not available")
	}

	name := TmuxSessionName("kill-zellij-test")
	_ = exec.Command("zellij", "delete-session", "--force", name).Run()

	if output, err := exec.Command("zellij", "attach", "-b", "-c", name).CombinedOutput(); err != nil {
		t.Fatalf("create zellij session: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.Command("zellij", "--session", name, "run", "--cwd", "/tmp", "--", "/bin/sh", "-lc", "true").CombinedOutput(); err != nil {
		t.Fatalf("seed zellij pane: %v (%s)", err, strings.TrimSpace(string(output)))
	}

	time.Sleep(300 * time.Millisecond)
	transport.KillSession("kill-zellij-test")

	if output, err := exec.Command("zellij", "list-sessions").CombinedOutput(); err == nil && strings.Contains(string(output), name) {
		t.Fatalf("expected zellij session %q to be removed after KillSession", name)
	}
}
