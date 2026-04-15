package wechat

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nekobot/pkg/acpstate"
	"nekobot/pkg/approval"
	"nekobot/pkg/config"
	"nekobot/pkg/externalagent"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/storage/ent"
	"nekobot/pkg/tasks"
	"nekobot/pkg/toolsessions"
)

type controlCommandKind string

const (
	controlCommandList     controlCommandKind = "list"
	controlCommandBindings controlCommandKind = "bindings"
	controlCommandUse      controlCommandKind = "use"
	controlCommandNew      controlCommandKind = "new"
	controlCommandStatus   controlCommandKind = "status"
	controlCommandLogs     controlCommandKind = "logs"
	controlCommandShare    controlCommandKind = "share"
	controlCommandYolo     controlCommandKind = "yolo"
	controlCommandSafe     controlCommandKind = "safe"
	controlCommandRestart  controlCommandKind = "restart"
	controlCommandStop     controlCommandKind = "stop"
	controlCommandDelete   controlCommandKind = "delete"
)

type controlCommand struct {
	Kind        controlCommandKind
	RuntimeName string
	Spec        RuntimeSpec
}

// RuntimeCreateRequest describes a new runtime request issued from WeChat.
type RuntimeCreateRequest struct {
	Name    string
	Driver  string
	Tool    string
	Command string
	Workdir string
}

// RuntimeApprovalResult describes a resolve-orchestrator approval decision.
type RuntimeApprovalResult struct {
	Status       string
	RequestID    string
	Reason       string
	LaunchPolicy map[string]any
}

// ControlService manages WeChat runtime commands on top of tool sessions.
type ControlService struct {
	cfg        *config.Config
	log        *logger.Logger
	sessions   *toolsessions.Manager
	process    *process.Manager
	bindings   *RuntimeBindingService
	entClient  *ent.Client
	approval   *approval.Manager
	taskStore  *tasks.Store
	acpMu      sync.Mutex
	acp        map[string]acpConversationClient
	pendingMu  sync.Mutex
	pending    map[string]*runtimePendingInteraction
	acpFactory func(RuntimePreset) (acpConversationClient, error)
}

type acpConversationClient interface {
	Chat(ctx context.Context, conversationID, message string) (acpChatResult, error)
	Respond(ctx context.Context, conversationID string, action interactionAction) (acpChatResult, error)
	SyncSessions(ctx context.Context, sessions map[string]string) error
	SessionMap() map[string]string
	Close() error
}

type acpChatResult struct {
	Reply   string
	Pending *acpPendingPrompt
}

type acpPendingKind string

const (
	acpPendingKindPermission acpPendingKind = "permission"
)

const (
	wechatRuntimeEventInput       = "wechat_runtime_input"
	wechatRuntimeEventPrompt      = "wechat_runtime_prompt"
	wechatRuntimeEventReply       = "wechat_runtime_reply"
	wechatRuntimeEventInteraction = "wechat_runtime_interaction"
)

type acpPendingPrompt struct {
	Kind    acpPendingKind
	Prompt  string
	Options []acpPendingOption
}

type acpPendingOption struct {
	ID    string
	Name  string
	Kind  string
	Index int
}

type runtimePendingInteraction struct {
	SessionID string
	Prompt    *acpPendingPrompt
	RequestID string
	Driver    string
	CreatedAt time.Time
}

var errNoPendingACPInteraction = errors.New("no pending acp interaction")

type runtimeStatusDetails struct {
	Name       string
	Driver     string
	Tool       string
	Command    string
	Workdir    string
	State      string
	Running    bool
	ExitCode   int
	OutputSize int
}

// ConversationRuntime identifies a runtime session plus its bound chat.
type ConversationRuntime struct {
	ChatID   string
	Session  *toolsessions.Session
	NextRead int
}

// NewControlService creates a WeChat runtime control service.
func NewControlService(
	cfg *config.Config,
	log *logger.Logger,
	sessionMgr *toolsessions.Manager,
	processMgr *process.Manager,
	bindingSvc *RuntimeBindingService,
	entClient *ent.Client,
	approvalMgr *approval.Manager,
	taskStore *tasks.Store,
) *ControlService {
	return &ControlService{
		cfg:       cfg,
		log:       log,
		sessions:  sessionMgr,
		process:   processMgr,
		bindings:  bindingSvc,
		entClient: entClient,
		approval:  approvalMgr,
		taskStore: taskStore,
		acp:       map[string]acpConversationClient{},
		pending:   map[string]*runtimePendingInteraction{},
	}
}

func parseControlCommand(input string) (controlCommand, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || trimmed[0] != '/' {
		return controlCommand{}, fmt.Errorf("control command must start with /")
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return controlCommand{}, fmt.Errorf("empty command")
	}

	switch fields[0] {
	case "/list":
		return controlCommand{Kind: controlCommandList}, nil
	case "/bindings":
		return controlCommand{Kind: controlCommandBindings}, nil
	case "/use":
		if len(fields) < 2 {
			return controlCommand{}, fmt.Errorf("usage: /use <runtime>")
		}
		return controlCommand{Kind: controlCommandUse, RuntimeName: strings.TrimSpace(fields[1])}, nil
	case "/status":
		name := ""
		if len(fields) > 1 {
			name = strings.TrimSpace(fields[1])
		}
		return controlCommand{Kind: controlCommandStatus, RuntimeName: name}, nil
	case "/logs":
		name := ""
		if len(fields) > 1 {
			name = strings.TrimSpace(fields[1])
		}
		return controlCommand{Kind: controlCommandLogs, RuntimeName: name}, nil
	case "/share":
		return controlCommand{Kind: controlCommandShare}, nil
	case "/whosyourdaddy", "/yolo":
		return controlCommand{Kind: controlCommandYolo}, nil
	case "/imyourdaddy", "/safe":
		return controlCommand{Kind: controlCommandSafe}, nil
	case "/restart":
		if len(fields) < 2 {
			return controlCommand{}, fmt.Errorf("usage: /restart <runtime>")
		}
		return controlCommand{Kind: controlCommandRestart, RuntimeName: strings.TrimSpace(fields[1])}, nil
	case "/stop":
		if len(fields) < 2 {
			return controlCommand{}, fmt.Errorf("usage: /stop <runtime>")
		}
		return controlCommand{Kind: controlCommandStop, RuntimeName: strings.TrimSpace(fields[1])}, nil
	case "/delete":
		if len(fields) < 2 {
			return controlCommand{}, fmt.Errorf("usage: /delete <runtime>")
		}
		return controlCommand{Kind: controlCommandDelete, RuntimeName: strings.TrimSpace(fields[1])}, nil
	case "/new":
		return parseNewControlCommand(trimmed)
	default:
		return controlCommand{}, fmt.Errorf("unsupported control command: %s", fields[0])
	}
}

func parseNewControlCommand(input string) (controlCommand, error) {
	parts := strings.SplitN(strings.TrimSpace(input), " -- ", 2)
	left := strings.Fields(parts[0])
	if len(left) < 2 {
		return controlCommand{}, fmt.Errorf("usage: /new <name> [--driver <acp|codex|claude|opencode|aider|process>] [--cwd <dir>] -- <command>")
	}

	cmd := controlCommand{
		Kind:        controlCommandNew,
		RuntimeName: strings.TrimSpace(left[1]),
	}

	spec := RuntimeSpec{Driver: "process"}
	requestedManagedTool := ""
	for i := 2; i < len(left); i++ {
		switch left[i] {
		case "--driver":
			i++
			if i >= len(left) {
				return controlCommand{}, fmt.Errorf("usage: --driver <acp|codex|claude|opencode|aider|process>")
			}
			rawDriver := strings.TrimSpace(left[i])
			if rawDriver != "" && normalizeManagedDriver(rawDriver) == "codex" && strings.ToLower(rawDriver) != "codex" {
				requestedManagedTool = strings.ToLower(rawDriver)
			}
			spec.Driver = normalizeManagedDriver(rawDriver)
		case "--cwd":
			i++
			if i >= len(left) {
				return controlCommand{}, fmt.Errorf("usage: --cwd <dir>")
			}
			spec.Workdir = strings.TrimSpace(left[i])
		default:
			return controlCommand{}, fmt.Errorf("unknown option: %s", left[i])
		}
	}

	if len(parts) == 2 {
		spec.Command = strings.TrimSpace(parts[1])
	}
	if spec.Tool == "" && requestedManagedTool != "" {
		spec.Tool = requestedManagedTool
	}
	if spec.Command == "" && isManagedExternalAgentDriver(spec.Driver) {
		if strings.TrimSpace(spec.Tool) != "" {
			spec.Command = strings.TrimSpace(spec.Tool)
		} else {
			spec.Command = defaultManagedDriverTool(spec.Driver)
		}
	}
	if spec.Command == "" && strings.EqualFold(spec.Driver, "acp") {
		spec.Command = cmd.RuntimeName
	}
	spec.Tool = strings.TrimSpace(spec.Command)
	if spec.Tool == "" && isManagedExternalAgentDriver(spec.Driver) {
		spec.Tool = defaultManagedDriverTool(spec.Driver)
	}
	cmd.Spec = spec
	return cmd, nil
}

// ResolveRuntime creates or validates a WeChat runtime session without always starting it.
func (s *ControlService) ResolveRuntime(
	ctx context.Context,
	chatID string,
	req RuntimeCreateRequest,
) (*toolsessions.Session, *RuntimeApprovalResult, error) {
	if s == nil || s.sessions == nil || s.process == nil || s.bindings == nil {
		return nil, nil, fmt.Errorf("wechat runtime control is not available")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil, nil, fmt.Errorf("chat id is required")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("runtime name is required")
	}

	preset, err := BuildRuntimePreset(s.cfg, RuntimeSpec{
		Driver:  req.Driver,
		Tool:    req.Tool,
		Command: req.Command,
		Workdir: req.Workdir,
	})
	if err != nil {
		return nil, nil, err
	}

	title := name
	sessionState := toolsessions.StateRunning
	if preset.Driver == "acp" || isManagedExternalAgentDriver(preset.Driver) {
		sessionState = toolsessions.StateDetached
	}
	sess, err := s.sessions.CreateSession(ctx, toolsessions.CreateSessionInput{
		Owner:           "wechat",
		Source:          toolsessions.SourceChannel,
		Channel:         "wechat",
		ConversationKey: wechatConversationKey(chatID),
		Tool:            preset.Tool,
		Title:           title,
		Command:         preset.Command,
		Workdir:         preset.Workdir,
		State:           sessionState,
		Metadata:        preset.Metadata,
	})
	if err != nil {
		return nil, nil, err
	}
	metadata := cloneSessionMetadata(sess.Metadata)
	metadata["chat_id"] = strings.TrimSpace(chatID)
	if err := s.sessions.UpdateSessionMetadata(ctx, sess.ID, metadata); err != nil {
		return nil, nil, err
	}
	sess.Metadata = metadata

	if err := s.bindings.BindConversation(ctx, chatID, sess.ID); err != nil {
		return nil, nil, err
	}

	approvalResult := (*RuntimeApprovalResult)(nil)
	switch preset.Driver {
	case "acp":
		if _, err := s.getACPClient(ctx, sess); err != nil {
			_ = s.sessions.TerminateSession(context.Background(), sess.ID, "failed to start acp runtime: "+err.Error())
			return nil, nil, fmt.Errorf("start acp runtime: %w", err)
		}
		if err := s.sessions.TouchSession(ctx, sess.ID, toolsessions.StateRunning); err != nil {
			return nil, nil, fmt.Errorf("touch acp runtime session: %w", err)
		}
	case "codex", "claude", "opencode", "aider":
		agentKind := runtimeManagedAgentKind(sess)
		approvalResult, err = s.applyManagedResolveOrchestrator(ctx, sess, agentKind, req.Tool)
		if err != nil {
			_ = s.sessions.TerminateSession(context.Background(), sess.ID, "failed to resolve managed runtime: "+err.Error())
			return nil, nil, fmt.Errorf("resolve managed runtime: %w", err)
		}
		if approvalResult != nil {
			break
		}
		if err := externalagent.EnsureProcess(
			ctx,
			s.cfg.WorkspacePath(),
			wechatProcessProbe{s.process},
			s.process,
			s.sessions,
			runtimeagents.DefaultTransport(),
			sess,
		); err != nil {
			_ = s.sessions.TerminateSession(context.Background(), sess.ID, "failed to start managed runtime: "+err.Error())
			return nil, nil, fmt.Errorf("start managed runtime: %w", err)
		}
	default:
		if err := s.process.Start(context.Background(), sess.ID, preset.Command, preset.Workdir); err != nil {
			_ = s.sessions.TerminateSession(context.Background(), sess.ID, "failed to start process: "+err.Error())
			return nil, nil, fmt.Errorf("start runtime process: %w", err)
		}
	}

	_ = s.sessions.AppendEvent(ctx, sess.ID, "wechat_runtime_created", map[string]interface{}{
		"driver":  preset.Driver,
		"tool":    preset.Tool,
		"chat_id": strings.TrimSpace(chatID),
	})
	session, err := s.sessions.GetSession(ctx, sess.ID)
	if err != nil {
		return nil, nil, err
	}
	return session, approvalResult, nil
}

// CreateRuntime creates a tool session, launches its process, and binds it to the WeChat chat.
func (s *ControlService) CreateRuntime(
	ctx context.Context,
	chatID string,
	req RuntimeCreateRequest,
) (*toolsessions.Session, error) {
	session, approvalResult, err := s.ResolveRuntime(ctx, chatID, req)
	if err != nil {
		return nil, err
	}
	if approvalResult != nil {
		toolLabel := runtimeApprovalToolLabel(session)
		switch approvalResult.Status {
		case "pending":
			return nil, fmt.Errorf("%s runtime approval pending", toolLabel)
		case "denied":
			if approvalResult.Reason != "" {
				return nil, fmt.Errorf("%s runtime denied: %s", toolLabel, approvalResult.Reason)
			}
			return nil, fmt.Errorf("%s runtime denied", toolLabel)
		default:
			return nil, fmt.Errorf("%s runtime approval unresolved", toolLabel)
		}
	}
	return session, nil
}

type wechatProcessProbe struct {
	mgr *process.Manager
}

func (p wechatProcessProbe) HasProcess(sessionID string) bool {
	if p.mgr == nil {
		return false
	}
	_, err := p.mgr.GetStatus(sessionID)
	return err == nil
}

func (s *ControlService) applyManagedResolveOrchestrator(
	ctx context.Context,
	sess *toolsessions.Session,
	agentKind string,
	requestedTool string,
) (*RuntimeApprovalResult, error) {
	orchestrator := externalagent.NewResolveOrchestrator(s.cfg, s.log, s.entClient, s.approval, s.taskStore)
	launchPolicy, err := orchestrator.PreviewLaunchPolicy(ctx, sess, agentKind, requestedTool)
	if err != nil {
		return nil, err
	}
	approvalResult, handled, err := orchestrator.HandleLaunchApproval(sess, launchPolicy)
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, nil
	}

	summary := externalagent.ApprovalSummaryFromResult(approvalResult, launchPolicy)
	result := &RuntimeApprovalResult{
		Status:       summary.Status,
		RequestID:    summary.RequestID,
		Reason:       summary.Reason,
		LaunchPolicy: summary.LaunchPolicy,
	}
	if result.Status == "pending" {
		metadata := cloneSessionMetadata(sess.Metadata)
		metadata["launch_policy"] = launchPolicy
		if err := s.sessions.UpdateSessionMetadata(ctx, sess.ID, metadata); err != nil {
			return nil, fmt.Errorf("persist managed launch policy metadata: %w", err)
		}
		sess.Metadata = metadata
		s.setPendingInteraction(strings.TrimSpace(metadataStringValue(metadata, "chat_id")), &runtimePendingInteraction{
			SessionID: sess.ID,
			RequestID: result.RequestID,
			Driver:    agentKind,
			CreatedAt: time.Now(),
		})
		return result, nil
	}
	if result.Status == "denied" {
		return result, nil
	}
	return result, nil
}

func runtimeManagedAgentKind(session *toolsessions.Session) string {
	if session == nil {
		return "codex"
	}
	agentKind := strings.TrimSpace(strings.ToLower(session.Tool))
	if agentKind == "" {
		agentKind = "codex"
	}
	return agentKind
}

func isManagedExternalAgentDriver(driver string) bool {
	switch strings.TrimSpace(strings.ToLower(driver)) {
	case "codex", "claude", "opencode", "aider":
		return true
	default:
		return false
	}
}

func normalizeManagedDriver(driver string) string {
	switch strings.TrimSpace(strings.ToLower(driver)) {
	case "claude", "opencode", "aider":
		return "codex"
	default:
		return strings.TrimSpace(strings.ToLower(driver))
	}
}

func defaultManagedDriverTool(driver string) string {
	trimmed := strings.TrimSpace(strings.ToLower(driver))
	if trimmed == "codex" {
		return "codex"
	}
	return trimmed
}

func runtimeApprovalToolLabel(session *toolsessions.Session) string {
	if session == nil {
		return "runtime"
	}
	tool := strings.TrimSpace(session.Tool)
	if tool == "" {
		return "runtime"
	}
	return tool
}

func cloneSessionMetadata(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

// GetRuntimeStatus returns the current status snapshot for a named runtime.
func (s *ControlService) GetRuntimeStatus(
	ctx context.Context,
	runtimeName string,
) (*runtimeStatusDetails, error) {
	target, err := s.findRuntimeByName(ctx, runtimeName)
	if err != nil {
		return nil, err
	}

	status := &runtimeStatusDetails{
		Name:    strings.TrimSpace(target.Title),
		Driver:  runtimeDriver(target),
		Tool:    strings.TrimSpace(target.Tool),
		Command: strings.TrimSpace(target.Command),
		Workdir: strings.TrimSpace(target.Workdir),
		State:   strings.TrimSpace(target.State),
	}
	if status.Name == "" {
		status.Name = status.Tool
	}

	if status.Driver == "acp" {
		s.acpMu.Lock()
		_, running := s.acp[target.ID]
		s.acpMu.Unlock()
		status.Running = running
		status.ExitCode = 0
		return status, nil
	}

	procStatus, err := s.process.GetStatus(target.ID)
	if err != nil {
		return nil, fmt.Errorf("get runtime process status: %w", err)
	}
	status.Running = procStatus.Running
	status.ExitCode = procStatus.ExitCode
	status.OutputSize = procStatus.OutputSize
	return status, nil
}

// StopRuntime stops a named runtime and clears any active WeChat binding to it.
func (s *ControlService) StopRuntime(ctx context.Context, runtimeName string) error {
	target, err := s.findRuntimeByName(ctx, runtimeName)
	if err != nil {
		return err
	}

	if runtimeDriver(target) == "acp" {
		if err := s.closeACPClient(target.ID); err != nil {
			return err
		}
	} else {
		if err := s.process.Kill(target.ID); err != nil && !strings.Contains(err.Error(), "session not running") {
			return fmt.Errorf("stop runtime process: %w", err)
		}
	}

	if err := s.clearRuntimeBindings(ctx, target.ID); err != nil {
		return fmt.Errorf("clear runtime binding: %w", err)
	}

	if err := s.sessions.TerminateSession(ctx, target.ID, "stopped from wechat control"); err != nil {
		return fmt.Errorf("terminate runtime session: %w", err)
	}
	return nil
}

// DeleteRuntime removes a named runtime session and clears any active WeChat binding to it.
func (s *ControlService) DeleteRuntime(ctx context.Context, runtimeName string) error {
	target, err := s.findRuntimeByName(ctx, runtimeName)
	if err != nil {
		return err
	}

	if runtimeDriver(target) == "acp" {
		if err := s.closeACPClient(target.ID); err != nil {
			return err
		}
	} else {
		if err := s.process.Kill(target.ID); err != nil &&
			!strings.Contains(strings.ToLower(err.Error()), "session not found") &&
			!strings.Contains(strings.ToLower(err.Error()), "session not running") {
			return fmt.Errorf("stop runtime process: %w", err)
		}
	}

	if err := s.clearRuntimeBindings(ctx, target.ID); err != nil {
		return fmt.Errorf("clear runtime binding: %w", err)
	}
	if err := s.process.Reset(target.ID); err != nil {
		return fmt.Errorf("reset runtime process: %w", err)
	}
	if err := s.sessions.DeleteSession(ctx, target.ID); err != nil {
		return fmt.Errorf("delete runtime session: %w", err)
	}
	return nil
}

// RestartRuntime restarts a named runtime in place.
func (s *ControlService) RestartRuntime(ctx context.Context, runtimeName string) error {
	target, err := s.findRuntimeByName(ctx, runtimeName)
	if err != nil {
		return err
	}

	if runtimeDriver(target) == "acp" {
		if err := s.closeACPClient(target.ID); err != nil {
			return err
		}
		if _, err := s.getACPClient(ctx, target); err != nil {
			return fmt.Errorf("restart acp runtime: %w", err)
		}
		if err := s.sessions.TouchSession(ctx, target.ID, toolsessions.StateRunning); err != nil {
			return fmt.Errorf("touch acp runtime session: %w", err)
		}
		return nil
	}

	if err := s.process.Kill(target.ID); err != nil &&
		!strings.Contains(strings.ToLower(err.Error()), "session not found") &&
		!strings.Contains(strings.ToLower(err.Error()), "session not running") {
		return fmt.Errorf("stop runtime process: %w", err)
	}
	if err := s.process.Reset(target.ID); err != nil {
		return fmt.Errorf("reset runtime process: %w", err)
	}
	if err := s.process.Start(context.Background(), target.ID, target.Command, target.Workdir); err != nil {
		return fmt.Errorf("restart runtime process: %w", err)
	}
	if err := s.sessions.TouchSession(ctx, target.ID, toolsessions.StateRunning); err != nil {
		return fmt.Errorf("touch runtime session: %w", err)
	}
	return nil
}

// GetRuntimeLogs returns recent output for a named runtime.
func (s *ControlService) GetRuntimeLogs(ctx context.Context, runtimeName string, limit int) (string, error) {
	target, err := s.findRuntimeByName(ctx, runtimeName)
	if err != nil {
		return "", err
	}
	if runtimeDriver(target) == "acp" {
		return s.renderACPLogs(ctx, target.ID, limit)
	}
	if limit <= 0 {
		limit = 120
	}
	lines, _, err := s.process.GetOutput(target.ID, 0, limit)
	if err != nil {
		return "", fmt.Errorf("get runtime logs: %w", err)
	}
	text := strings.TrimSpace(strings.Join(lines, ""))
	if text == "" {
		return "No runtime output yet.", nil
	}
	return text, nil
}

// BindRuntime binds a named runtime to a WeChat chat.
func (s *ControlService) BindRuntime(ctx context.Context, chatID, runtimeName string) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return fmt.Errorf("chat id is required")
	}
	session, err := s.findRuntimeByName(ctx, runtimeName)
	if err != nil {
		return err
	}
	return s.bindings.BindConversation(ctx, chatID, session.ID)
}

// ListRuntimes returns visible WeChat-controlled runtimes.
func (s *ControlService) ListRuntimes(ctx context.Context) ([]*toolsessions.Session, error) {
	if s == nil || s.sessions == nil {
		return nil, fmt.Errorf("tool session manager is required")
	}
	all, err := s.sessions.ListSessions(ctx, toolsessions.ListSessionsInput{
		Source: toolsessions.SourceChannel,
		Limit:  200,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*toolsessions.Session, 0, len(all))
	for _, item := range all {
		if item == nil || strings.TrimSpace(item.Channel) != "wechat" {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

// DescribeBindings renders a short WeChat binding summary.
func (s *ControlService) DescribeBindings(ctx context.Context) (string, error) {
	bindings, err := s.bindings.ListBindingRecords(ctx)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(bindings))
	for _, item := range bindings {
		if item == nil || strings.TrimSpace(item.Conversation.Channel) != "wechat" {
			continue
		}
		chatID := strings.TrimSpace(item.Conversation.ConversationID)
		if chatID == "" {
			continue
		}
		target, err := s.sessions.GetSession(ctx, item.TargetSessionID)
		if err != nil {
			return "", fmt.Errorf("get runtime session for binding: %w", err)
		}
		name := strings.TrimSpace(target.Title)
		if name == "" {
			name = strings.TrimSpace(target.Tool)
		}
		lines = append(lines, fmt.Sprintf("%s -> %s", chatID, name))
	}
	if len(lines) == 0 {
		return "No active WeChat bindings.", nil
	}
	return strings.Join(lines, "\n"), nil
}

// SendToRuntime routes a chat message to an explicit or bound runtime.
func (s *ControlService) SendToRuntime(
	ctx context.Context,
	chatID, runtimeName, text string,
) (string, error) {
	if s == nil || s.process == nil || s.bindings == nil {
		return "", fmt.Errorf("wechat runtime control is not available")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return "", fmt.Errorf("chat id is required")
	}

	message := strings.TrimSpace(text)
	if message == "" {
		return "", fmt.Errorf("message is empty")
	}

	var (
		target *toolsessions.Session
		err    error
	)
	if strings.TrimSpace(runtimeName) != "" {
		target, err = s.findRuntimeByName(ctx, runtimeName)
		if err != nil {
			return "", err
		}
	} else {
		target, err = s.bindings.ResolveConversation(ctx, chatID)
		if err != nil {
			return "", err
		}
		if target == nil {
			return "", fmt.Errorf("no runtime bound for current chat")
		}
	}

	if runtimeDriver(target) == "acp" {
		reply, err := s.sendToACP(ctx, target, chatID, message)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(reply), nil
	}

	if err := s.process.Write(target.ID, message+"\n"); err != nil {
		return "", fmt.Errorf("write runtime input: %w", err)
	}
	if err := s.sessions.TouchSession(ctx, target.ID, toolsessions.StateRunning); err != nil {
		return "", fmt.Errorf("touch runtime session: %w", err)
	}
	_ = s.sessions.AppendEvent(ctx, target.ID, wechatRuntimeEventInput, map[string]interface{}{
		"chat_id": chatID,
		"bytes":   len(message),
	})
	return "", nil
}

// ReadRuntimeOutput returns incremental runtime output and the next cursor.
func (s *ControlService) ReadRuntimeOutput(
	ctx context.Context,
	sessionID string,
	cursor int,
) (string, int, error) {
	if s == nil || s.process == nil || s.sessions == nil {
		return "", cursor, fmt.Errorf("wechat runtime control is not available")
	}
	id := strings.TrimSpace(sessionID)
	if id == "" {
		return "", cursor, fmt.Errorf("session id is required")
	}
	if _, err := s.sessions.GetSession(ctx, id); err != nil {
		return "", cursor, err
	}
	session, err := s.sessions.GetSession(ctx, id)
	if err != nil {
		return "", cursor, err
	}
	if runtimeDriver(session) == "acp" {
		return s.readACPOutput(ctx, id, cursor)
	}

	chunks, total, err := s.process.GetOutput(id, cursor, 500)
	if err != nil {
		return "", cursor, fmt.Errorf("get runtime output: %w", err)
	}
	return strings.TrimSpace(strings.Join(chunks, "")), total, nil
}

// GetConversationRuntime resolves the bound runtime and returns its current output cursor.
func (s *ControlService) GetConversationRuntime(
	ctx context.Context,
	chatID string,
) (*ConversationRuntime, error) {
	if s == nil || s.bindings == nil || s.process == nil {
		return nil, fmt.Errorf("wechat runtime control is not available")
	}
	session, err := s.bindings.ResolveConversation(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	if runtimeDriver(session) == "acp" {
		_, total, err := s.readACPOutput(ctx, session.ID, 0)
		if err != nil {
			return nil, fmt.Errorf("get acp runtime cursor: %w", err)
		}
		return &ConversationRuntime{
			ChatID:   strings.TrimSpace(chatID),
			Session:  session,
			NextRead: total,
		}, nil
	}
	_, total, err := s.process.GetOutput(session.ID, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("get runtime cursor: %w", err)
	}
	return &ConversationRuntime{
		ChatID:   strings.TrimSpace(chatID),
		Session:  session,
		NextRead: total,
	}, nil
}

func (s *ControlService) findRuntimeByName(ctx context.Context, runtimeName string) (*toolsessions.Session, error) {
	name := strings.TrimSpace(runtimeName)
	if name == "" {
		return nil, fmt.Errorf("runtime name is required")
	}
	runtimes, err := s.ListRuntimes(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range runtimes {
		if item == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Title), name) {
			return item, nil
		}
	}
	return nil, fmt.Errorf("runtime not found: %s", name)
}

func (s *ControlService) sendToACP(
	ctx context.Context,
	target *toolsessions.Session,
	chatID, message string,
) (string, error) {
	client, err := s.getACPClient(ctx, target)
	if err != nil {
		return "", err
	}
	result, err := client.Chat(ctx, strings.TrimSpace(chatID), message)
	if err != nil {
		return "", fmt.Errorf("acp chat failed: %w", err)
	}
	s.appendACPEvent(ctx, target.ID, wechatRuntimeEventInput, map[string]interface{}{
		"chat_id": chatID,
		"driver":  "acp",
		"text":    message,
	})
	if result.Pending != nil {
		s.appendACPEvent(ctx, target.ID, wechatRuntimeEventPrompt, map[string]interface{}{
			"chat_id": chatID,
			"driver":  "acp",
			"kind":    string(result.Pending.Kind),
			"prompt":  result.Pending.Prompt,
		})
		s.setPendingInteraction(strings.TrimSpace(chatID), &runtimePendingInteraction{
			SessionID: target.ID,
			Prompt:    result.Pending,
			CreatedAt: time.Now(),
		})
		return strings.TrimSpace(result.Pending.Prompt), nil
	}
	s.clearPendingInteraction(strings.TrimSpace(chatID))
	if err := s.persistACPSessionMap(ctx, target, client.SessionMap()); err != nil {
		return "", err
	}
	if err := s.sessions.TouchSession(ctx, target.ID, toolsessions.StateRunning); err != nil {
		return "", fmt.Errorf("touch runtime session: %w", err)
	}
	s.appendACPEvent(ctx, target.ID, wechatRuntimeEventReply, map[string]interface{}{
		"chat_id": chatID,
		"driver":  "acp",
		"reply":   strings.TrimSpace(result.Reply),
	})
	return strings.TrimSpace(result.Reply), nil
}

// ResolvePendingInteraction resolves a pending ACP interaction for the bound WeChat chat.
func (s *ControlService) ResolvePendingInteraction(
	ctx context.Context,
	chatID string,
	action *interactionAction,
) (string, bool, error) {
	if s == nil || action == nil || s.bindings == nil {
		return "", false, nil
	}
	if action.Type == interactionActionPassthrough {
		return "", false, nil
	}

	chatID = strings.TrimSpace(chatID)
	pending := s.getPendingInteraction(chatID)
	if pending == nil {
		return "", false, nil
	}

	target, err := s.bindings.ResolveConversation(ctx, chatID)
	if err != nil {
		return "", true, err
	}
	if target == nil || target.ID != pending.SessionID {
		s.clearPendingInteraction(chatID)
		return "", false, nil
	}
	if runtimeDriver(target) != "acp" {
		return s.resolveManagedPendingInteraction(ctx, chatID, target, pending, action)
	}

	client, err := s.getACPClient(ctx, target)
	if err != nil {
		return "", true, err
	}

	result, err := client.Respond(ctx, chatID, *action)
	if err != nil {
		if errors.Is(err, errNoPendingACPInteraction) {
			s.clearPendingInteraction(chatID)
			return "", false, nil
		}
		return "", true, fmt.Errorf("resolve acp interaction: %w", err)
	}

	s.clearPendingInteraction(chatID)
	if err := s.persistACPSessionMap(ctx, target, client.SessionMap()); err != nil {
		return "", true, err
	}
	if err := s.sessions.TouchSession(ctx, target.ID, toolsessions.StateRunning); err != nil {
		return "", true, fmt.Errorf("touch runtime session: %w", err)
	}

	reply := strings.TrimSpace(result.Reply)
	if reply == "" {
		switch action.Type {
		case interactionActionConfirm:
			reply = "已允许，继续执行中。"
		case interactionActionDeny:
			reply = "已拒绝。"
		default:
			reply = "已处理。"
		}
	}
	s.appendACPEvent(ctx, target.ID, wechatRuntimeEventInteraction, map[string]interface{}{
		"chat_id": chatID,
		"driver":  "acp",
		"action":  string(action.Type),
	})
	s.appendACPEvent(ctx, target.ID, wechatRuntimeEventReply, map[string]interface{}{
		"chat_id": chatID,
		"driver":  "acp",
		"reply":   reply,
	})
	return reply, true, nil
}

func (s *ControlService) resolveManagedPendingInteraction(
	ctx context.Context,
	chatID string,
	target *toolsessions.Session,
	pending *runtimePendingInteraction,
	action *interactionAction,
) (string, bool, error) {
	if pending == nil || strings.TrimSpace(pending.RequestID) == "" || s.approval == nil {
		s.clearPendingInteraction(chatID)
		return "", false, nil
	}

	switch action.Type {
	case interactionActionConfirm:
		if err := s.approval.Approve(pending.RequestID); err != nil {
			return "", true, err
		}
		s.approval.SetSessionMode(target.ID, approval.ModeAuto)
		if s.taskStore != nil {
			s.taskStore.ClearSessionPendingAction(target.ID)
			s.taskStore.SetSessionPermissionMode(target.ID, string(approval.ModeAuto))
		}
		if err := externalagent.EnsureProcess(ctx, s.cfg.WorkspacePath(), wechatProcessProbe{s.process}, s.process, s.sessions, runtimeagents.DefaultTransport(), target); err != nil {
			return "", true, err
		}
		s.clearPendingInteraction(chatID)
		return "已允许，继续启动中。", true, nil
	case interactionActionDeny:
		if err := s.approval.Deny(pending.RequestID, "denied from wechat interaction"); err != nil {
			return "", true, err
		}
		if s.taskStore != nil {
			s.taskStore.ClearSessionPendingAction(target.ID)
		}
		s.clearPendingInteraction(chatID)
		return "已拒绝。", true, nil
	case interactionActionSelect:
		switch strings.TrimSpace(action.Value) {
		case "1":
			return s.resolveManagedPendingInteraction(ctx, chatID, target, pending, &interactionAction{Type: interactionActionConfirm})
		case "2":
			return s.resolveManagedPendingInteraction(ctx, chatID, target, pending, &interactionAction{Type: interactionActionDeny})
		default:
			return "当前操作仅支持 /select 1 允许，或 /select 2 拒绝。", true, nil
		}
	default:
		return "", false, nil
	}
}

func metadataStringValue(metadata map[string]interface{}, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func (s *ControlService) renderACPLogs(ctx context.Context, sessionID string, limit int) (string, error) {
	if s == nil || s.sessions == nil {
		return "", fmt.Errorf("tool session manager is required")
	}
	if limit <= 0 {
		limit = 120
	}
	events, err := s.sessions.ListEvents(ctx, sessionID, limit*4)
	if err != nil {
		return "", fmt.Errorf("list acp runtime events: %w", err)
	}

	lines := make([]string, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		line := renderACPLogEvent(events[i])
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return "No runtime output yet.", nil
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return strings.Join(lines, "\n"), nil
}

func (s *ControlService) readACPOutput(ctx context.Context, sessionID string, cursor int) (string, int, error) {
	if s == nil || s.sessions == nil {
		return "", cursor, fmt.Errorf("tool session manager is required")
	}
	if cursor < 0 {
		cursor = 0
	}

	events, err := s.sessions.ListEvents(ctx, sessionID, 1000)
	if err != nil {
		return "", cursor, fmt.Errorf("list acp runtime events: %w", err)
	}
	rendered := make([]string, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		line := renderACPLogEvent(events[i])
		if strings.TrimSpace(line) == "" {
			continue
		}
		rendered = append(rendered, line)
	}
	total := len(rendered)
	if cursor >= total {
		return "", total, nil
	}
	return strings.TrimSpace(strings.Join(rendered[cursor:], "\n")), total, nil
}

func (s *ControlService) appendACPEvent(
	ctx context.Context,
	sessionID, eventType string,
	payload map[string]interface{},
) {
	if s == nil || s.sessions == nil {
		return
	}
	_ = s.sessions.AppendEvent(ctx, sessionID, eventType, payload)
}

func renderACPLogEvent(event *toolsessions.Event) string {
	if event == nil {
		return ""
	}
	switch strings.TrimSpace(event.Type) {
	case wechatRuntimeEventInput:
		text := payloadString(event.Payload, "text")
		if text == "" {
			return ""
		}
		return "User: " + text
	case wechatRuntimeEventPrompt:
		prompt := payloadString(event.Payload, "prompt")
		if prompt == "" {
			return ""
		}
		return "Assistant: " + prompt
	case wechatRuntimeEventReply:
		reply := payloadString(event.Payload, "reply")
		if reply == "" {
			return ""
		}
		return "Assistant: " + reply
	case wechatRuntimeEventInteraction:
		action := payloadString(event.Payload, "action")
		if action == "" {
			return ""
		}
		switch action {
		case string(interactionActionConfirm):
			return "User action: /yes"
		case string(interactionActionDeny):
			return "User action: /no"
		default:
			return "User action: " + action
		}
	default:
		return ""
	}
}

func payloadString(payload map[string]interface{}, key string) string {
	if len(payload) == 0 {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func (s *ControlService) setPendingInteraction(chatID string, pending *runtimePendingInteraction) {
	if s == nil {
		return
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return
	}
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if pending == nil {
		delete(s.pending, chatID)
		return
	}
	s.pending[chatID] = pending
}

func (s *ControlService) getPendingInteraction(chatID string) *runtimePendingInteraction {
	if s == nil {
		return nil
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil
	}
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	item := s.pending[chatID]
	if item == nil {
		return nil
	}
	if time.Since(item.CreatedAt) > 15*time.Minute {
		delete(s.pending, chatID)
		return nil
	}
	return item
}

func (s *ControlService) clearPendingInteraction(chatID string) {
	if s == nil {
		return
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return
	}
	s.pendingMu.Lock()
	delete(s.pending, chatID)
	s.pendingMu.Unlock()
}

func (s *ControlService) clearRuntimeBindings(ctx context.Context, sessionID string) error {
	if s == nil || s.bindings == nil {
		return fmt.Errorf("conversation binding service is required")
	}
	records, err := s.bindings.GetBindingsBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record == nil {
			continue
		}
		chatID := strings.TrimSpace(record.Conversation.ConversationID)
		if chatID == "" {
			continue
		}
		if err := s.bindings.ClearConversation(ctx, chatID); err != nil {
			return err
		}
	}
	return nil
}

func (s *ControlService) getACPClient(ctx context.Context, target *toolsessions.Session) (acpConversationClient, error) {
	if target == nil {
		return nil, fmt.Errorf("runtime session is nil")
	}
	s.acpMu.Lock()
	defer s.acpMu.Unlock()

	if existing, ok := s.acp[target.ID]; ok {
		return existing, nil
	}

	preset := RuntimePreset{
		Driver:  runtimeDriver(target),
		Tool:    strings.TrimSpace(target.Tool),
		Command: strings.TrimSpace(target.Command),
		Workdir: strings.TrimSpace(target.Workdir),
	}
	factory := s.acpFactory
	if factory == nil {
		factory = newACPConversationClient
	}
	client, err := factory(preset)
	if err != nil {
		return nil, err
	}
	if err := client.SyncSessions(ctx, acpstate.SessionMap(target.Metadata)); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("restore acp sessions: %w", err)
	}
	s.acp[target.ID] = client
	return client, nil
}

func (s *ControlService) persistACPSessionMap(
	ctx context.Context,
	target *toolsessions.Session,
	sessions map[string]string,
) error {
	if s == nil || s.sessions == nil || target == nil || len(sessions) == 0 {
		return nil
	}

	nextMetadata := acpstate.SetConversationSession(target.Metadata, "", "")
	for conversationID, sessionID := range sessions {
		nextMetadata = acpstate.SetConversationSession(nextMetadata, conversationID, sessionID)
	}
	if err := s.sessions.UpdateSessionMetadata(ctx, target.ID, nextMetadata); err != nil {
		return fmt.Errorf("persist acp session metadata: %w", err)
	}
	target.Metadata = nextMetadata
	return nil
}

func runtimeDriver(session *toolsessions.Session) string {
	if session == nil {
		return ""
	}
	if session.Metadata != nil {
		if raw, ok := session.Metadata["driver"].(string); ok {
			return strings.TrimSpace(strings.ToLower(raw))
		}
	}
	return ""
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type acpInitParams struct {
	ProtocolVersion    int                    `json:"protocolVersion"`
	ClientCapabilities map[string]interface{} `json:"clientCapabilities"`
}

type acpNewSessionParams struct {
	Cwd        string        `json:"cwd"`
	McpServers []interface{} `json:"mcpServers"`
}

type acpNewSessionResult struct {
	SessionID string `json:"sessionId"`
}

type acpLoadSessionParams struct {
	SessionID  string        `json:"sessionId"`
	Cwd        string        `json:"cwd"`
	McpServers []interface{} `json:"mcpServers"`
}

type acpPromptParams struct {
	SessionID string           `json:"sessionId"`
	Prompt    []acpPromptEntry `json:"prompt"`
}

type acpPromptEntry struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type acpSessionUpdateParams struct {
	SessionID string           `json:"sessionId"`
	Update    acpSessionUpdate `json:"update"`
}

type acpSessionUpdate struct {
	SessionUpdate string          `json:"sessionUpdate"`
	Content       json.RawMessage `json:"content,omitempty"`
	Type          string          `json:"type,omitempty"`
	Text          string          `json:"text,omitempty"`
}

type acpSessionEvent struct {
	Update     *acpSessionUpdate
	Permission *acpPermissionRequest
}

type acpPermissionRequest struct {
	SessionID     string
	RequestID     int64
	AllowOptionID string
	DenyOptionID  string
	Prompt        string
	Options       []acpPendingOption
}

type acpActivePrompt struct {
	sessionID string
	events    chan acpSessionEvent
	done      chan error
	request   *acpPermissionRequest

	mu        sync.Mutex
	textParts []string
	resultCh  chan struct{}
	reply     string
	err       error
}

type acpConversationProcess struct {
	command   string
	args      []string
	workdir   string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	scanner   *bufio.Scanner
	stderr    *acpStderrWriter
	nextID    atomic.Int64
	pending   map[int64]chan *rpcResponse
	pendingMu sync.Mutex
	notify    map[string]chan acpSessionEvent
	notifyMu  sync.Mutex
	sessions  map[string]string
	active    map[string]*acpActivePrompt
	mu        sync.Mutex
}

func newACPConversationClient(p RuntimePreset) (acpConversationClient, error) {
	base := firstCommandToken(p.Command)
	if strings.TrimSpace(base) == "" {
		return nil, fmt.Errorf("acp command is required")
	}
	args := strings.Fields(strings.TrimSpace(p.Command))
	if len(args) == 0 {
		return nil, fmt.Errorf("acp command is required")
	}

	cmd := exec.Command(args[0], args[1:]...)
	if strings.TrimSpace(p.Workdir) != "" {
		cmd.Dir = strings.TrimSpace(p.Workdir)
	}
	stderr := &acpStderrWriter{}
	cmd.Stderr = stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create acp stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create acp stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start acp command: %w", err)
	}

	client := &acpConversationProcess{
		command:  args[0],
		args:     args[1:],
		workdir:  strings.TrimSpace(p.Workdir),
		cmd:      cmd,
		stdin:    stdin,
		scanner:  bufio.NewScanner(stdout),
		stderr:   stderr,
		pending:  map[int64]chan *rpcResponse{},
		notify:   map[string]chan acpSessionEvent{},
		sessions: map[string]string{},
		active:   map[string]*acpActivePrompt{},
	}
	client.scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	go client.readLoop()

	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := client.call(initCtx, "initialize", acpInitParams{
		ProtocolVersion: 1,
		ClientCapabilities: map[string]interface{}{
			"fs": map[string]bool{
				"readTextFile":  true,
				"writeTextFile": true,
			},
		},
	}); err != nil {
		if detail := strings.TrimSpace(client.stderr.LastError()); detail != "" {
			err = fmt.Errorf("%w: %s", err, detail)
		}
		_ = client.Close()
		return nil, err
	}

	return client, nil
}

func (a *acpConversationProcess) Chat(ctx context.Context, conversationID, message string) (acpChatResult, error) {
	sessionID, err := a.getOrCreateSession(ctx, conversationID)
	if err != nil {
		return acpChatResult{}, err
	}

	events := make(chan acpSessionEvent, 256)
	a.notifyMu.Lock()
	a.notify[sessionID] = events
	a.notifyMu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, err := a.call(ctx, "session/prompt", acpPromptParams{
			SessionID: sessionID,
			Prompt:    []acpPromptEntry{{Type: "text", Text: message}},
		})
		done <- err
	}()

	active := &acpActivePrompt{
		sessionID: sessionID,
		events:    events,
		done:      done,
		resultCh:  make(chan struct{}),
	}
	for {
		select {
		case <-ctx.Done():
			a.unregisterNotify(sessionID)
			return acpChatResult{}, ctx.Err()
		case event := <-events:
			switch {
			case event.Update != nil && event.Update.SessionUpdate == "agent_message_chunk":
				if text := extractACPChunkText(event.Update); text != "" {
					active.appendText(text)
				}
			case event.Permission != nil:
				active.request = event.Permission
				a.storeActivePrompt(conversationID, active)
				go a.awaitPromptCompletion(conversationID, active)
				return acpChatResult{
					Pending: &acpPendingPrompt{
						Kind:   acpPendingKindPermission,
						Prompt: strings.TrimSpace(event.Permission.Prompt),
					},
				}, nil
			}
		case err := <-done:
			a.unregisterNotify(sessionID)
			if err != nil {
				return acpChatResult{}, err
			}
			return acpChatResult{Reply: active.replyText()}, nil
		}
	}
}

func (a *acpConversationProcess) Respond(
	ctx context.Context,
	conversationID string,
	action interactionAction,
) (acpChatResult, error) {
	active := a.getActivePrompt(conversationID)
	if active == nil {
		return acpChatResult{}, errNoPendingACPInteraction
	}

	request := active.request
	if request == nil {
		a.clearActivePrompt(conversationID)
		return acpChatResult{}, errNoPendingACPInteraction
	}
	if err := a.replyPermissionRequest(request, action); err != nil {
		return acpChatResult{}, err
	}
	active.mu.Lock()
	active.request = nil
	active.mu.Unlock()

	select {
	case <-ctx.Done():
		return acpChatResult{}, ctx.Err()
	case <-active.resultCh:
		if active.err != nil {
			return acpChatResult{}, active.err
		}
		return acpChatResult{Reply: active.replyText()}, nil
	}
}

func (a *acpConversationProcess) SyncSessions(ctx context.Context, sessions map[string]string) error {
	for conversationID, sessionID := range sessions {
		if err := a.loadSession(ctx, conversationID, sessionID); err != nil {
			return err
		}
	}
	return nil
}

func (a *acpConversationProcess) SessionMap() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()

	cloned := make(map[string]string, len(a.sessions))
	for conversationID, sessionID := range a.sessions {
		cloned[conversationID] = sessionID
	}
	return cloned
}

func (a *acpConversationProcess) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stdin != nil {
		_ = a.stdin.Close()
	}
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
		_, _ = a.cmd.Process.Wait()
	}
	return nil
}

func (a *acpConversationProcess) unregisterNotify(sessionID string) {
	a.notifyMu.Lock()
	delete(a.notify, sessionID)
	a.notifyMu.Unlock()
}

func (a *acpConversationProcess) storeActivePrompt(conversationID string, active *acpActivePrompt) {
	a.mu.Lock()
	a.active[conversationID] = active
	a.mu.Unlock()
}

func (a *acpConversationProcess) getActivePrompt(conversationID string) *acpActivePrompt {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.active[conversationID]
}

func (a *acpConversationProcess) clearActivePrompt(conversationID string) {
	a.mu.Lock()
	delete(a.active, conversationID)
	a.mu.Unlock()
}

func (a *acpConversationProcess) awaitPromptCompletion(conversationID string, active *acpActivePrompt) {
	defer close(active.resultCh)
	defer a.unregisterNotify(active.sessionID)
	defer a.clearActivePrompt(conversationID)

	for {
		select {
		case event := <-active.events:
			if event.Update != nil && event.Update.SessionUpdate == "agent_message_chunk" {
				if text := extractACPChunkText(event.Update); text != "" {
					active.appendText(text)
				}
			}
		case err := <-active.done:
			if err != nil {
				active.setResult("", err)
				return
			}
			active.setResult(active.replyText(), nil)
			return
		}
	}
}

func (a *acpConversationProcess) replyPermissionRequest(req *acpPermissionRequest, action interactionAction) error {
	if req == nil {
		return errNoPendingACPInteraction
	}

	optionID, err := selectPermissionOption(req, action)
	if err != nil {
		return err
	}

	data, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      req.RequestID,
		Params: map[string]interface{}{
			"outcome": map[string]interface{}{
				"outcome":  "selected",
				"optionId": optionID,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("marshal permission reply: %w", err)
	}

	a.mu.Lock()
	_, err = fmt.Fprintf(a.stdin, "%s\n", data)
	a.mu.Unlock()
	if err != nil {
		return fmt.Errorf("write permission reply: %w", err)
	}
	return nil
}

func selectPermissionOption(req *acpPermissionRequest, action interactionAction) (string, error) {
	if req == nil {
		return "", errNoPendingACPInteraction
	}

	switch action.Type {
	case interactionActionConfirm:
		if optionID := strings.TrimSpace(req.AllowOptionID); optionID != "" {
			return optionID, nil
		}
	case interactionActionDeny:
		if optionID := strings.TrimSpace(req.DenyOptionID); optionID != "" {
			return optionID, nil
		}
	case interactionActionSelect:
		for _, option := range req.Options {
			if option.Index > 0 && action.Value == fmt.Sprintf("%d", option.Index) {
				return strings.TrimSpace(option.ID), nil
			}
		}
		return "", fmt.Errorf("unknown permission selection: %s", action.Value)
	default:
		return "", fmt.Errorf("unsupported interaction action: %s", action.Type)
	}

	return "", fmt.Errorf("no matching permission option for action: %s", action.Type)
}

func (a *acpActivePrompt) appendText(text string) {
	a.mu.Lock()
	a.textParts = append(a.textParts, text)
	a.mu.Unlock()
}

func (a *acpActivePrompt) replyText() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return strings.TrimSpace(strings.Join(a.textParts, ""))
}

func (a *acpActivePrompt) setResult(reply string, err error) {
	a.mu.Lock()
	a.reply = strings.TrimSpace(reply)
	a.err = err
	a.mu.Unlock()
}

func (a *acpConversationProcess) getOrCreateSession(ctx context.Context, conversationID string) (string, error) {
	a.mu.Lock()
	if existing, ok := a.sessions[conversationID]; ok {
		a.mu.Unlock()
		return existing, nil
	}
	a.mu.Unlock()

	result, err := a.call(ctx, "session/new", acpNewSessionParams{
		Cwd:        a.workdir,
		McpServers: []interface{}{},
	})
	if err != nil {
		return "", err
	}
	var payload acpNewSessionResult
	if err := json.Unmarshal(result, &payload); err != nil {
		return "", fmt.Errorf("parse acp session result: %w", err)
	}

	a.mu.Lock()
	a.sessions[conversationID] = payload.SessionID
	a.mu.Unlock()
	return payload.SessionID, nil
}

func (a *acpConversationProcess) loadSession(ctx context.Context, conversationID, sessionID string) error {
	conversationID = strings.TrimSpace(conversationID)
	sessionID = strings.TrimSpace(sessionID)
	if conversationID == "" || sessionID == "" {
		return nil
	}

	a.mu.Lock()
	if existing, ok := a.sessions[conversationID]; ok && existing == sessionID {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	if _, err := a.call(ctx, "session/load", acpLoadSessionParams{
		SessionID:  sessionID,
		Cwd:        a.workdir,
		McpServers: []interface{}{},
	}); err != nil {
		return fmt.Errorf("load acp session %s for %s: %w", sessionID, conversationID, err)
	}

	a.mu.Lock()
	a.sessions[conversationID] = sessionID
	a.mu.Unlock()
	return nil
}

func (a *acpConversationProcess) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	id := a.nextID.Add(1)
	ch := make(chan *rpcResponse, 1)
	a.pendingMu.Lock()
	a.pending[id] = ch
	a.pendingMu.Unlock()
	defer func() {
		a.pendingMu.Lock()
		delete(a.pending, id)
		a.pendingMu.Unlock()
	}()

	data, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal acp request: %w", err)
	}

	a.mu.Lock()
	_, err = fmt.Fprintf(a.stdin, "%s\n", data)
	a.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write acp request: %w", err)
	}

	select {
	case <-callCtx.Done():
		return nil, callCtx.Err()
	case resp := <-ch:
		if resp == nil {
			return nil, fmt.Errorf("acp response missing")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("acp error: %s", resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (a *acpConversationProcess) readLoop() {
	for a.scanner.Scan() {
		line := strings.TrimSpace(a.scanner.Text())
		if line == "" {
			continue
		}

		var msg rpcResponse
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.ID != nil && msg.Method == "" {
			a.pendingMu.Lock()
			ch := a.pending[*msg.ID]
			a.pendingMu.Unlock()
			if ch != nil {
				ch <- &msg
			}
			continue
		}
		if msg.Method != "session/update" {
			if msg.Method == "session/request_permission" {
				a.handlePermissionRequest(&msg)
			}
			continue
		}

		var params acpSessionUpdateParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			continue
		}
		a.notifyMu.Lock()
		ch := a.notify[params.SessionID]
		a.notifyMu.Unlock()
		if ch != nil {
			ch <- acpSessionEvent{Update: &params.Update}
		}
	}
}

func (a *acpConversationProcess) handlePermissionRequest(msg *rpcResponse) {
	if msg == nil || msg.ID == nil {
		return
	}

	var params struct {
		SessionID    string `json:"sessionId"`
		ToolName     string `json:"toolName"`
		Description  string `json:"description"`
		Message      string `json:"message"`
		InputPreview string `json:"inputPreview"`
		Options      []struct {
			OptionID string `json:"optionId"`
			Kind     string `json:"kind"`
		} `json:"options"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return
	}

	req := &acpPermissionRequest{
		SessionID: strings.TrimSpace(params.SessionID),
		RequestID: *msg.ID,
	}
	options := make([]acpPendingOption, 0, len(params.Options))
	for _, option := range params.Options {
		if strings.TrimSpace(option.OptionID) == "" {
			continue
		}
		item := acpPendingOption{
			ID:    strings.TrimSpace(option.OptionID),
			Kind:  strings.TrimSpace(strings.ToLower(option.Kind)),
			Index: len(options) + 1,
		}
		switch item.Kind {
		case "allow", "allow_once", "allow_always":
			req.AllowOptionID = item.ID
		case "deny", "reject", "reject_once", "reject_always":
			req.DenyOptionID = item.ID
		}
		item.Name = permissionOptionLabel(item.Kind, item.Index)
		options = append(options, item)
	}
	req.Options = options
	req.Prompt = formatACPPermissionPrompt(
		strings.TrimSpace(params.Message),
		strings.TrimSpace(params.ToolName),
		strings.TrimSpace(params.Description),
		strings.TrimSpace(params.InputPreview),
		req.Options,
	)
	if req.SessionID == "" || len(req.Options) == 0 {
		return
	}

	a.notifyMu.Lock()
	ch := a.notify[req.SessionID]
	a.notifyMu.Unlock()
	if ch != nil {
		ch <- acpSessionEvent{Permission: req}
	}
}

func extractACPChunkText(update *acpSessionUpdate) string {
	if update == nil {
		return ""
	}
	if strings.TrimSpace(update.Text) != "" {
		return update.Text
	}

	var payload struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(update.Content, &payload); err == nil && strings.TrimSpace(payload.Text) != "" {
		return payload.Text
	}
	return ""
}

func permissionOptionLabel(kind string, index int) string {
	switch kind {
	case "allow", "allow_once":
		return "允许一次"
	case "allow_always":
		return "总是允许"
	case "deny", "reject", "reject_once":
		return "拒绝"
	case "reject_always":
		return "总是拒绝"
	default:
		return fmt.Sprintf("选项 %d", index)
	}
}

func formatACPPermissionPrompt(
	message, toolName, description, inputPreview string,
	options []acpPendingOption,
) string {
	lines := make([]string, 0, 6)

	head := "运行时需要确认操作。"
	if strings.TrimSpace(toolName) != "" {
		head = "运行时需要确认: " + strings.TrimSpace(toolName)
	}
	lines = append(lines, head)

	if strings.TrimSpace(message) != "" {
		lines = append(lines, strings.TrimSpace(message))
	}
	if strings.TrimSpace(description) != "" {
		lines = append(lines, "说明: "+strings.TrimSpace(description))
	}
	if strings.TrimSpace(inputPreview) != "" {
		lines = append(lines, "输入: "+strings.TrimSpace(inputPreview))
	}
	if len(options) > 0 {
		lines = append(lines, "")
		for _, option := range options {
			if option.Index <= 0 {
				continue
			}
			lines = append(lines, fmt.Sprintf("%d. %s", option.Index, option.Name))
		}
	}
	switch len(options) {
	case 0:
		lines = append(lines, "回复 /yes 允许，/no 或 /cancel 拒绝。")
	case 1:
		lines = append(lines, "回复 /select 1。")
	case 2:
		lines = append(lines, "回复 /yes 允许，/no 或 /cancel 拒绝，也可以用 /select 1 或 /select 2。")
	default:
		lines = append(lines, "回复 /select N 选择对应选项，也可以直接回复对应编号。")
	}
	return strings.TrimSpace(strings.Join(lines, "\n\n"))
}

func (s *ControlService) closeACPClient(sessionID string) error {
	s.acpMu.Lock()
	client := s.acp[sessionID]
	delete(s.acp, sessionID)
	s.acpMu.Unlock()
	if client == nil {
		return nil
	}
	if err := client.Close(); err != nil {
		return fmt.Errorf("close acp runtime: %w", err)
	}
	return nil
}

type acpStderrWriter struct {
	mu   sync.Mutex
	last string
}

func (w *acpStderrWriter) Write(p []byte) (int, error) {
	text := strings.TrimSpace(string(p))
	if text == "" {
		return len(p), nil
	}
	w.mu.Lock()
	w.last = text
	w.mu.Unlock()
	return len(p), nil
}

func (w *acpStderrWriter) LastError() string {
	if w == nil {
		return ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.last
}

var _ io.Writer = (*acpStderrWriter)(nil)
