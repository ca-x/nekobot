package wechat

import (
	"context"
	"strings"
	"testing"
	"time"

	"nekobot/pkg/config"
	"nekobot/pkg/process"
	"nekobot/pkg/toolsessions"
)

func TestParseControlCommandParsesUseAndNew(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    controlCommandKind
		wantArg string
	}{
		{name: "use", input: "/use code1", want: controlCommandUse, wantArg: "code1"},
		{name: "bindings", input: "/bindings", want: controlCommandBindings},
		{name: "list", input: "/list", want: controlCommandList},
		{name: "logs", input: "/logs code1", want: controlCommandLogs, wantArg: "code1"},
		{name: "restart", input: "/restart code1", want: controlCommandRestart, wantArg: "code1"},
		{name: "delete", input: "/delete code1", want: controlCommandDelete, wantArg: "code1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := parseControlCommand(tt.input)
			if err != nil {
				t.Fatalf("parseControlCommand failed: %v", err)
			}
			if cmd.Kind != tt.want {
				t.Fatalf("expected kind %q, got %q", tt.want, cmd.Kind)
			}
			if tt.wantArg != "" && cmd.RuntimeName != tt.wantArg {
				t.Fatalf("expected runtime %q, got %q", tt.wantArg, cmd.RuntimeName)
			}
		})
	}

	cmd, err := parseControlCommand("/new code1 --driver acp -- codex-acp")
	if err != nil {
		t.Fatalf("parseControlCommand(new) failed: %v", err)
	}
	if cmd.Kind != controlCommandNew {
		t.Fatalf("expected new kind, got %q", cmd.Kind)
	}
	if cmd.RuntimeName != "code1" {
		t.Fatalf("expected runtime name code1, got %q", cmd.RuntimeName)
	}
	if cmd.Spec.Driver != "acp" {
		t.Fatalf("expected driver acp, got %q", cmd.Spec.Driver)
	}
	if cmd.Spec.Command != "codex-acp" {
		t.Fatalf("expected command codex-acp, got %q", cmd.Spec.Command)
	}
}

func TestControlServiceCreateBindAndList(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	ctx := context.Background()

	created, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "code1",
		Driver: "process",
		Tool:   "cat",
	})
	if err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}
	if created.Tool != "cat" {
		t.Fatalf("expected created tool cat, got %q", created.Tool)
	}
	if created.ConversationKey != "wx:chat-1" {
		t.Fatalf("expected conversation key wx:chat-1, got %q", created.ConversationKey)
	}

	status, err := processMgr.GetStatus(created.ID)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if !strings.Contains(status.Command, "cat") {
		t.Fatalf("expected process command to contain cat, got %q", status.Command)
	}

	listOutput, err := controlSvc.ListRuntimes(ctx)
	if err != nil {
		t.Fatalf("ListRuntimes failed: %v", err)
	}
	if len(listOutput) != 1 {
		t.Fatalf("expected 1 runtime, got %d", len(listOutput))
	}
	if listOutput[0].Title != "code1" {
		t.Fatalf("expected runtime title code1, got %q", listOutput[0].Title)
	}

	bindingText, err := controlSvc.DescribeBindings(ctx)
	if err != nil {
		t.Fatalf("DescribeBindings failed: %v", err)
	}
	if !strings.Contains(bindingText, "chat-1") || !strings.Contains(bindingText, "code1") {
		t.Fatalf("expected bindings output to mention chat and runtime, got %q", bindingText)
	}

	second, err := controlSvc.CreateRuntime(ctx, "chat-2", RuntimeCreateRequest{
		Name:   "code2",
		Driver: "process",
		Tool:   "cat",
	})
	if err != nil {
		t.Fatalf("CreateRuntime(second) failed: %v", err)
	}

	if err := controlSvc.BindRuntime(ctx, "chat-1", "code2"); err != nil {
		t.Fatalf("BindRuntime failed: %v", err)
	}
	resolved, err := bindingSvc.ResolveConversation(ctx, "chat-1")
	if err != nil {
		t.Fatalf("ResolveConversation failed: %v", err)
	}
	if resolved == nil || resolved.ID != second.ID {
		t.Fatalf("expected resolved runtime %q, got %+v", second.ID, resolved)
	}
}

func TestControlServiceCreateACPRuntimeDoesNotStartPTYAndCanStop(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	controlSvc.acpFactory = func(p RuntimePreset) (acpConversationClient, error) {
		return &fakeACPClient{reply: "ready"}, nil
	}
	ctx := context.Background()

	created, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "claude1",
		Driver: "acp",
		Tool:   "claude",
	})
	if err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}

	if _, err := processMgr.GetStatus(created.ID); err == nil {
		t.Fatalf("expected no PTY process for ACP runtime")
	}

	status, err := controlSvc.GetRuntimeStatus(ctx, "claude1")
	if err != nil {
		t.Fatalf("GetRuntimeStatus failed: %v", err)
	}
	if !status.Running {
		t.Fatalf("expected ACP runtime to be reported running")
	}

	if err := controlSvc.StopRuntime(ctx, "claude1"); err != nil {
		t.Fatalf("StopRuntime failed: %v", err)
	}

	resolved, err := bindingSvc.ResolveConversation(ctx, "chat-1")
	if err != nil {
		t.Fatalf("ResolveConversation failed: %v", err)
	}
	if resolved != nil {
		t.Fatalf("expected binding to be cleared after stop, got %+v", resolved)
	}
}

func TestControlServiceRouteMessageToBoundRuntime(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	ctx := context.Background()

	created, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "echo1",
		Driver: "process",
		Tool:   "cat",
	})
	if err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}

	reply, err := controlSvc.SendToRuntime(ctx, "chat-1", "", "hello runtime")
	if err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}
	if reply != "" {
		t.Fatalf("expected no direct reply for process runtime, got %q", reply)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		chunks, _, err := processMgr.GetOutput(created.ID, 0, 100)
		if err != nil {
			t.Fatalf("GetOutput failed: %v", err)
		}
		if strings.Contains(strings.Join(chunks, ""), "hello runtime") {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatal("expected runtime output to contain routed message")
}

func TestControlServiceReadRuntimeOutputReturnsOnlyNewChunks(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	ctx := context.Background()

	created, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "echo2",
		Driver: "process",
		Tool:   "cat",
	})
	if err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}

	reply, err := controlSvc.SendToRuntime(ctx, "chat-1", "", "first line")
	if err != nil {
		t.Fatalf("SendToRuntime(first) failed: %v", err)
	}
	if reply != "" {
		t.Fatalf("expected no direct reply for process runtime, got %q", reply)
	}

	var firstText string
	var cursor int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		firstText, cursor, err = controlSvc.ReadRuntimeOutput(ctx, created.ID, 0)
		if err != nil {
			t.Fatalf("ReadRuntimeOutput(first) failed: %v", err)
		}
		if strings.Contains(firstText, "first line") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(firstText, "first line") {
		t.Fatalf("expected first output, got %q", firstText)
	}

	reply, err = controlSvc.SendToRuntime(ctx, "chat-1", "", "second line")
	if err != nil {
		t.Fatalf("SendToRuntime(second) failed: %v", err)
	}
	if reply != "" {
		t.Fatalf("expected no direct reply for process runtime, got %q", reply)
	}

	var secondText string
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		secondText, cursor, err = controlSvc.ReadRuntimeOutput(ctx, created.ID, cursor)
		if err != nil {
			t.Fatalf("ReadRuntimeOutput(second) failed: %v", err)
		}
		if strings.Contains(secondText, "second line") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(secondText, "second line") {
		t.Fatalf("expected second output, got %q", secondText)
	}
	if cursor <= 0 {
		t.Fatalf("expected cursor to advance, got %d", cursor)
	}
}

func TestControlServiceGetRuntimeLogsReturnsRecentOutput(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	ctx := context.Background()

	_, err = controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "echo3",
		Driver: "process",
		Tool:   "cat",
	})
	if err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}
	if _, err := controlSvc.SendToRuntime(ctx, "chat-1", "", "hello logs"); err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}

	var logs string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		logs, err = controlSvc.GetRuntimeLogs(ctx, "echo3", 50)
		if err != nil {
			t.Fatalf("GetRuntimeLogs failed: %v", err)
		}
		if strings.Contains(logs, "hello logs") {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("expected logs to include runtime output, got %q", logs)
}

func TestControlServiceGetRuntimeLogsReturnsACPEvents(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	fake := &fakeACPInteractiveClient{
		prompt: "Claude 需要确认:\n\n是否允许执行 ReadFile？\n\n/yes 允许，/no 拒绝。",
		reply:  "继续执行后的最终回复",
	}
	controlSvc.acpFactory = func(p RuntimePreset) (acpConversationClient, error) {
		return fake, nil
	}
	ctx := context.Background()

	if _, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "claude1",
		Driver: "acp",
		Tool:   "claude",
	}); err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}
	if _, err := controlSvc.SendToRuntime(ctx, "chat-1", "", "hello acp"); err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}
	if _, _, err := controlSvc.ResolvePendingInteraction(ctx, "chat-1", &interactionAction{Type: interactionActionConfirm}); err != nil {
		t.Fatalf("ResolvePendingInteraction failed: %v", err)
	}

	logs, err := controlSvc.GetRuntimeLogs(ctx, "claude1", 20)
	if err != nil {
		t.Fatalf("GetRuntimeLogs failed: %v", err)
	}
	if !strings.Contains(logs, "hello acp") {
		t.Fatalf("expected ACP logs to include input, got %q", logs)
	}
	if !strings.Contains(logs, "Claude 需要确认") {
		t.Fatalf("expected ACP logs to include pending prompt, got %q", logs)
	}
	if !strings.Contains(logs, "继续执行后的最终回复") {
		t.Fatalf("expected ACP logs to include final reply, got %q", logs)
	}
}

func TestControlServiceReadRuntimeOutputReturnsACPEventsIncrementally(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	fake := &fakeACPInteractiveClient{
		prompt: "Claude 需要确认:\n\n是否允许执行 ReadFile？\n\n/yes 允许，/no 拒绝。",
		reply:  "继续执行后的最终回复",
	}
	controlSvc.acpFactory = func(p RuntimePreset) (acpConversationClient, error) {
		return fake, nil
	}
	ctx := context.Background()

	created, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "claude1",
		Driver: "acp",
		Tool:   "claude",
	})
	if err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}

	bound, err := controlSvc.GetConversationRuntime(ctx, "chat-1")
	if err != nil {
		t.Fatalf("GetConversationRuntime failed: %v", err)
	}
	if bound == nil || bound.Session == nil {
		t.Fatal("expected bound runtime")
	}
	if bound.NextRead != 0 {
		t.Fatalf("expected initial cursor 0, got %d", bound.NextRead)
	}

	reply, err := controlSvc.SendToRuntime(ctx, "chat-1", "", "hello acp")
	if err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}
	if !strings.Contains(reply, "Claude 需要确认") {
		t.Fatalf("expected pending prompt, got %q", reply)
	}

	firstText, cursor, err := controlSvc.ReadRuntimeOutput(ctx, created.ID, 0)
	if err != nil {
		t.Fatalf("ReadRuntimeOutput(first) failed: %v", err)
	}
	if !strings.Contains(firstText, "User: hello acp") {
		t.Fatalf("expected first output to include input, got %q", firstText)
	}
	if !strings.Contains(firstText, "Claude 需要确认") {
		t.Fatalf("expected first output to include prompt, got %q", firstText)
	}
	if cursor != 2 {
		t.Fatalf("expected first cursor 2, got %d", cursor)
	}

	reply, handled, err := controlSvc.ResolvePendingInteraction(ctx, "chat-1", &interactionAction{Type: interactionActionConfirm})
	if err != nil {
		t.Fatalf("ResolvePendingInteraction failed: %v", err)
	}
	if !handled {
		t.Fatal("expected pending interaction to be handled")
	}
	if reply != "继续执行后的最终回复" {
		t.Fatalf("unexpected interaction reply: %q", reply)
	}

	secondText, next, err := controlSvc.ReadRuntimeOutput(ctx, created.ID, cursor)
	if err != nil {
		t.Fatalf("ReadRuntimeOutput(second) failed: %v", err)
	}
	if !strings.Contains(secondText, "User action: /yes") {
		t.Fatalf("expected second output to include interaction, got %q", secondText)
	}
	if !strings.Contains(secondText, "继续执行后的最终回复") {
		t.Fatalf("expected second output to include final reply, got %q", secondText)
	}
	if next != 4 {
		t.Fatalf("expected next cursor 4, got %d", next)
	}
}

func TestControlServiceRestartAndDeleteRuntime(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	ctx := context.Background()

	created, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "echo4",
		Driver: "process",
		Tool:   "cat",
	})
	if err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}

	firstStatus, err := processMgr.GetStatus(created.ID)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	firstPID := firstStatus.ID
	if err := controlSvc.RestartRuntime(ctx, "echo4"); err != nil {
		t.Fatalf("RestartRuntime failed: %v", err)
	}
	restartedStatus, err := processMgr.GetStatus(created.ID)
	if err != nil {
		t.Fatalf("GetStatus after restart failed: %v", err)
	}
	if !restartedStatus.Running {
		t.Fatal("expected restarted process to be running")
	}
	if restartedStatus.ID != firstPID {
		t.Fatalf("expected same session id after restart, got %q", restartedStatus.ID)
	}

	if err := controlSvc.DeleteRuntime(ctx, "echo4"); err != nil {
		t.Fatalf("DeleteRuntime failed: %v", err)
	}
	if _, err := sessionMgr.GetSession(ctx, created.ID); err == nil {
		t.Fatal("expected session to be deleted")
	}
}

type fakeACPClient struct {
	reply    string
	sessions map[string]string
	synced   map[string]string
}

func (f *fakeACPClient) Chat(ctx context.Context, conversationID, message string) (acpChatResult, error) {
	if f.sessions == nil {
		f.sessions = map[string]string{}
	}
	if _, ok := f.sessions[conversationID]; !ok {
		f.sessions[conversationID] = "sess-" + conversationID
	}
	return acpChatResult{Reply: f.reply}, nil
}

func (f *fakeACPClient) Respond(ctx context.Context, conversationID string, action interactionAction) (acpChatResult, error) {
	return acpChatResult{Reply: f.reply}, nil
}

func (f *fakeACPClient) SyncSessions(ctx context.Context, sessions map[string]string) error {
	f.synced = make(map[string]string, len(sessions))
	f.sessions = make(map[string]string, len(sessions))
	for conversationID, sessionID := range sessions {
		f.synced[conversationID] = sessionID
		f.sessions[conversationID] = sessionID
	}
	return nil
}

func (f *fakeACPClient) SessionMap() map[string]string {
	cloned := make(map[string]string, len(f.sessions))
	for conversationID, sessionID := range f.sessions {
		cloned[conversationID] = sessionID
	}
	return cloned
}

func (f *fakeACPClient) Close() error {
	return nil
}

type fakeACPInteractiveClient struct {
	prompt  string
	reply   string
	actions []interactionActionType
	options []acpPendingOption
	last    interactionAction
}

func (f *fakeACPInteractiveClient) Chat(ctx context.Context, conversationID, message string) (acpChatResult, error) {
	options := f.options
	if len(options) == 0 {
		options = []acpPendingOption{
			{ID: "allow", Name: "允许一次", Kind: "allow_once", Index: 1},
			{ID: "deny", Name: "拒绝", Kind: "reject_once", Index: 2},
		}
	}
	return acpChatResult{
		Pending: &acpPendingPrompt{
			Kind:    acpPendingKindPermission,
			Prompt:  f.prompt,
			Options: options,
		},
	}, nil
}

func (f *fakeACPInteractiveClient) Respond(ctx context.Context, conversationID string, action interactionAction) (acpChatResult, error) {
	f.actions = append(f.actions, action.Type)
	f.last = action
	return acpChatResult{Reply: f.reply}, nil
}

func (f *fakeACPInteractiveClient) SyncSessions(ctx context.Context, sessions map[string]string) error {
	return nil
}

func (f *fakeACPInteractiveClient) SessionMap() map[string]string {
	return map[string]string{}
}

func (f *fakeACPInteractiveClient) Close() error {
	return nil
}

func TestControlServiceSendToACPRuntimeReturnsDirectReply(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	controlSvc.acpFactory = func(p RuntimePreset) (acpConversationClient, error) {
		return &fakeACPClient{reply: "acp reply"}, nil
	}
	ctx := context.Background()

	created, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "claude1",
		Driver: "acp",
		Tool:   "claude",
	})
	if err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}
	if created == nil {
		t.Fatal("expected created runtime")
	}

	reply, err := controlSvc.SendToRuntime(ctx, "chat-1", "", "hello acp")
	if err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}
	if reply != "acp reply" {
		t.Fatalf("expected direct ACP reply, got %q", reply)
	}

	stored, err := sessionMgr.GetSession(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	acpSessions, ok := stored.Metadata["acp_sessions"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected persisted acp_sessions metadata, got %#v", stored.Metadata)
	}
	if acpSessions["chat-1"] != "sess-chat-1" {
		t.Fatalf("expected persisted session mapping, got %#v", acpSessions)
	}
}

func TestControlServiceSendToACPRuntimeReturnsPendingPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	fake := &fakeACPInteractiveClient{
		prompt: "Claude 需要确认:\n\n是否允许执行 ReadFile？\n\n/yes 允许，/no 拒绝。",
		reply:  "done",
	}
	controlSvc.acpFactory = func(p RuntimePreset) (acpConversationClient, error) {
		return fake, nil
	}
	ctx := context.Background()

	if _, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "claude1",
		Driver: "acp",
		Tool:   "claude",
	}); err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}

	reply, err := controlSvc.SendToRuntime(ctx, "chat-1", "", "hello acp")
	if err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}
	if !strings.Contains(reply, "Claude 需要确认") {
		t.Fatalf("expected pending prompt reply, got %q", reply)
	}
}

func TestControlServiceResolvePendingInteractionContinuesACPRuntime(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	fake := &fakeACPInteractiveClient{
		prompt: "Claude 需要确认:\n\n是否允许执行 ReadFile？\n\n/yes 允许，/no 拒绝。",
		reply:  "继续执行后的最终回复",
	}
	controlSvc.acpFactory = func(p RuntimePreset) (acpConversationClient, error) {
		return fake, nil
	}
	ctx := context.Background()

	if _, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "claude1",
		Driver: "acp",
		Tool:   "claude",
	}); err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}
	if _, err := controlSvc.SendToRuntime(ctx, "chat-1", "", "hello acp"); err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}

	reply, handled, err := controlSvc.ResolvePendingInteraction(ctx, "chat-1", &interactionAction{Type: interactionActionConfirm})
	if err != nil {
		t.Fatalf("ResolvePendingInteraction failed: %v", err)
	}
	if !handled {
		t.Fatal("expected pending interaction to be handled")
	}
	if reply != "继续执行后的最终回复" {
		t.Fatalf("unexpected interaction reply: %q", reply)
	}
	if len(fake.actions) != 1 || fake.actions[0] != interactionActionConfirm {
		t.Fatalf("expected confirm action to be forwarded, got %#v", fake.actions)
	}
}

func TestControlServiceResolvePendingInteractionSelectsACPOption(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	fake := &fakeACPInteractiveClient{
		prompt: "运行时需要确认: WriteFile\n\n1. 允许一次\n2. 总是允许\n3. 拒绝\n\n回复 /select N 选择对应选项。",
		reply:  "已按选择继续执行",
		options: []acpPendingOption{
			{ID: "allow-once", Name: "允许一次", Kind: "allow_once", Index: 1},
			{ID: "allow-always", Name: "总是允许", Kind: "allow_always", Index: 2},
			{ID: "reject-once", Name: "拒绝", Kind: "reject_once", Index: 3},
		},
	}
	controlSvc.acpFactory = func(p RuntimePreset) (acpConversationClient, error) {
		return fake, nil
	}
	ctx := context.Background()

	if _, err := controlSvc.CreateRuntime(ctx, "chat-1", RuntimeCreateRequest{
		Name:   "claude1",
		Driver: "acp",
		Tool:   "claude",
	}); err != nil {
		t.Fatalf("CreateRuntime failed: %v", err)
	}
	reply, err := controlSvc.SendToRuntime(ctx, "chat-1", "", "hello acp")
	if err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}
	if !strings.Contains(reply, "/select") {
		t.Fatalf("expected select prompt, got %q", reply)
	}

	reply, handled, err := controlSvc.ResolvePendingInteraction(ctx, "chat-1", &interactionAction{
		Type:  interactionActionSelect,
		Value: "2",
	})
	if err != nil {
		t.Fatalf("ResolvePendingInteraction failed: %v", err)
	}
	if !handled {
		t.Fatal("expected select interaction to be handled")
	}
	if reply != "已按选择继续执行" {
		t.Fatalf("unexpected interaction reply: %q", reply)
	}
	if fake.last.Type != interactionActionSelect || fake.last.Value != "2" {
		t.Fatalf("expected select action to be forwarded, got %#v", fake.last)
	}
}

func TestSelectPermissionOption(t *testing.T) {
	req := &acpPermissionRequest{
		AllowOptionID: "allow-once",
		DenyOptionID:  "reject-once",
		Options: []acpPendingOption{
			{ID: "allow-once", Kind: "allow_once", Index: 1},
			{ID: "allow-always", Kind: "allow_always", Index: 2},
			{ID: "reject-once", Kind: "reject_once", Index: 3},
		},
	}

	got, err := selectPermissionOption(req, interactionAction{Type: interactionActionConfirm})
	if err != nil {
		t.Fatalf("selectPermissionOption(confirm): %v", err)
	}
	if got != "allow-once" {
		t.Fatalf("expected allow-once, got %q", got)
	}

	got, err = selectPermissionOption(req, interactionAction{Type: interactionActionDeny})
	if err != nil {
		t.Fatalf("selectPermissionOption(deny): %v", err)
	}
	if got != "reject-once" {
		t.Fatalf("expected reject-once, got %q", got)
	}

	got, err = selectPermissionOption(req, interactionAction{Type: interactionActionSelect, Value: "2"})
	if err != nil {
		t.Fatalf("selectPermissionOption(select): %v", err)
	}
	if got != "allow-always" {
		t.Fatalf("expected allow-always, got %q", got)
	}
}

func TestControlServiceRestoresPersistedACPSessions(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()

	log := newRuntimeTestLogger(t)
	client := newRuntimeTestEntClient(t, cfg)
	defer client.Close()

	sessionMgr, err := toolsessions.NewManager(cfg, log, client)
	if err != nil {
		t.Fatalf("new tool session manager: %v", err)
	}
	processMgr := process.NewManager(log)

	bindingSvc := NewRuntimeBindingService(sessionMgr, cfg)
	controlSvc := NewControlService(cfg, sessionMgr, processMgr, bindingSvc)
	fake := &fakeACPClient{reply: "restored"}
	controlSvc.acpFactory = func(p RuntimePreset) (acpConversationClient, error) {
		return fake, nil
	}
	ctx := context.Background()

	_, err = sessionMgr.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:           "wechat",
		Source:          toolsessions.SourceChannel,
		Channel:         "wechat",
		ConversationKey: "wx:chat-1",
		Tool:            "claude",
		Title:           "claude1",
		Command:         "claude-agent-acp",
		Workdir:         cfg.Agents.Defaults.Workspace,
		State:           toolsessions.StateRunning,
		Metadata: map[string]interface{}{
			"driver": "acp",
			"acp_sessions": map[string]interface{}{
				"chat-1": "sess-existing",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	reply, err := controlSvc.SendToRuntime(ctx, "chat-1", "claude1", "hello again")
	if err != nil {
		t.Fatalf("SendToRuntime failed: %v", err)
	}
	if reply != "restored" {
		t.Fatalf("expected restored reply, got %q", reply)
	}
	if fake.synced["chat-1"] != "sess-existing" {
		t.Fatalf("expected persisted acp session restored, got %#v", fake.synced)
	}
	if fake.sessions["chat-1"] != "sess-existing" {
		t.Fatalf("expected session map to keep restored session, got %#v", fake.sessions)
	}
}
