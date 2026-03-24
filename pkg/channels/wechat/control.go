package wechat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nekobot/pkg/acpstate"
	"nekobot/pkg/config"
	"nekobot/pkg/process"
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

// ControlService manages WeChat runtime commands on top of tool sessions.
type ControlService struct {
	cfg        *config.Config
	sessions   *toolsessions.Manager
	process    *process.Manager
	bindings   *RuntimeBindingService
	acpMu      sync.Mutex
	acp        map[string]acpConversationClient
	acpFactory func(RuntimePreset) (acpConversationClient, error)
}

type acpConversationClient interface {
	Chat(ctx context.Context, conversationID, message string) (string, error)
	SyncSessions(ctx context.Context, sessions map[string]string) error
	SessionMap() map[string]string
	Close() error
}

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
	sessionMgr *toolsessions.Manager,
	processMgr *process.Manager,
	bindingSvc *RuntimeBindingService,
) *ControlService {
	return &ControlService{
		cfg:      cfg,
		sessions: sessionMgr,
		process:  processMgr,
		bindings: bindingSvc,
		acp:      map[string]acpConversationClient{},
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
		return controlCommand{}, fmt.Errorf("usage: /new <name> [--driver <acp|codex|process>] [--cwd <dir>] -- <command>")
	}

	cmd := controlCommand{
		Kind:        controlCommandNew,
		RuntimeName: strings.TrimSpace(left[1]),
	}

	spec := RuntimeSpec{Driver: "process"}
	for i := 2; i < len(left); i++ {
		switch left[i] {
		case "--driver":
			i++
			if i >= len(left) {
				return controlCommand{}, fmt.Errorf("usage: --driver <acp|codex|process>")
			}
			spec.Driver = strings.TrimSpace(left[i])
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
	if spec.Command == "" && strings.EqualFold(spec.Driver, "codex") {
		spec.Command = "codex"
	}
	if spec.Command == "" && strings.EqualFold(spec.Driver, "acp") {
		spec.Command = cmd.RuntimeName
	}
	spec.Tool = strings.TrimSpace(spec.Command)
	if spec.Tool == "" && strings.EqualFold(spec.Driver, "codex") {
		spec.Tool = "codex"
	}
	cmd.Spec = spec
	return cmd, nil
}

// CreateRuntime creates a tool session, launches its process, and binds it to the WeChat chat.
func (s *ControlService) CreateRuntime(
	ctx context.Context,
	chatID string,
	req RuntimeCreateRequest,
) (*toolsessions.Session, error) {
	if s == nil || s.sessions == nil || s.process == nil || s.bindings == nil {
		return nil, fmt.Errorf("wechat runtime control is not available")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("runtime name is required")
	}

	preset, err := BuildRuntimePreset(s.cfg, RuntimeSpec{
		Driver:  req.Driver,
		Tool:    req.Tool,
		Command: req.Command,
		Workdir: req.Workdir,
	})
	if err != nil {
		return nil, err
	}

	title := name
	sessionState := toolsessions.StateRunning
	if preset.Driver == "acp" {
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
		return nil, err
	}

	switch preset.Driver {
	case "acp":
		if _, err := s.getACPClient(ctx, sess); err != nil {
			_ = s.sessions.TerminateSession(context.Background(), sess.ID, "failed to start acp runtime: "+err.Error())
			return nil, fmt.Errorf("start acp runtime: %w", err)
		}
		if err := s.sessions.TouchSession(ctx, sess.ID, toolsessions.StateRunning); err != nil {
			return nil, fmt.Errorf("touch acp runtime session: %w", err)
		}
	default:
		if err := s.process.Start(context.Background(), sess.ID, preset.Command, preset.Workdir); err != nil {
			_ = s.sessions.TerminateSession(context.Background(), sess.ID, "failed to start process: "+err.Error())
			return nil, fmt.Errorf("start runtime process: %w", err)
		}
	}

	if err := s.bindings.BindConversation(ctx, chatID, sess.ID); err != nil {
		return nil, err
	}

	_ = s.sessions.AppendEvent(ctx, sess.ID, "wechat_runtime_created", map[string]interface{}{
		"driver":  preset.Driver,
		"tool":    preset.Tool,
		"chat_id": strings.TrimSpace(chatID),
	})
	return s.sessions.GetSession(ctx, sess.ID)
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

	if chatID := boundWechatChatID(target); chatID != "" {
		if err := s.bindings.ClearConversation(ctx, chatID); err != nil {
			return fmt.Errorf("clear runtime binding: %w", err)
		}
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

	if chatID := boundWechatChatID(target); chatID != "" {
		if err := s.bindings.ClearConversation(ctx, chatID); err != nil {
			return fmt.Errorf("clear runtime binding: %w", err)
		}
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
		return "ACP runtime logs are not buffered yet. Use the bound chat to observe live output.", nil
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
	bindings, err := s.bindings.ListBindings(ctx)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(bindings))
	for _, item := range bindings {
		if item == nil || strings.TrimSpace(item.Channel) != "wechat" || strings.TrimSpace(item.ConversationKey) == "" {
			continue
		}
		chatID := strings.TrimPrefix(strings.TrimSpace(item.ConversationKey), wechatConversationPrefix)
		name := strings.TrimSpace(item.Title)
		if name == "" {
			name = strings.TrimSpace(item.Tool)
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
	_ = s.sessions.AppendEvent(ctx, target.ID, "wechat_runtime_input", map[string]interface{}{
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
	reply, err := client.Chat(ctx, strings.TrimSpace(chatID), message)
	if err != nil {
		return "", fmt.Errorf("acp chat failed: %w", err)
	}
	if err := s.persistACPSessionMap(ctx, target, client.SessionMap()); err != nil {
		return "", err
	}
	if err := s.sessions.TouchSession(ctx, target.ID, toolsessions.StateRunning); err != nil {
		return "", fmt.Errorf("touch runtime session: %w", err)
	}
	_ = s.sessions.AppendEvent(ctx, target.ID, "wechat_runtime_input", map[string]interface{}{
		"chat_id": chatID,
		"bytes":   len(message),
		"driver":  "acp",
	})
	return reply, nil
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

func boundWechatChatID(session *toolsessions.Session) string {
	if session == nil {
		return ""
	}
	if strings.TrimSpace(session.Channel) != "wechat" {
		return ""
	}
	conversationKey := strings.TrimSpace(session.ConversationKey)
	if !strings.HasPrefix(conversationKey, wechatConversationPrefix) {
		return ""
	}
	return strings.TrimPrefix(conversationKey, wechatConversationPrefix)
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
	notify    map[string]chan *acpSessionUpdate
	notifyMu  sync.Mutex
	sessions  map[string]string
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
		notify:   map[string]chan *acpSessionUpdate{},
		sessions: map[string]string{},
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

func (a *acpConversationProcess) Chat(ctx context.Context, conversationID, message string) (string, error) {
	sessionID, err := a.getOrCreateSession(ctx, conversationID)
	if err != nil {
		return "", err
	}

	updates := make(chan *acpSessionUpdate, 256)
	a.notifyMu.Lock()
	a.notify[sessionID] = updates
	a.notifyMu.Unlock()
	defer func() {
		a.notifyMu.Lock()
		delete(a.notify, sessionID)
		a.notifyMu.Unlock()
	}()

	done := make(chan error, 1)
	go func() {
		_, err := a.call(ctx, "session/prompt", acpPromptParams{
			SessionID: sessionID,
			Prompt:    []acpPromptEntry{{Type: "text", Text: message}},
		})
		done <- err
	}()

	var textParts []string
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case update := <-updates:
			if update != nil && update.SessionUpdate == "agent_message_chunk" {
				if text := extractACPChunkText(update); text != "" {
					textParts = append(textParts, text)
				}
			}
		case err := <-done:
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(strings.Join(textParts, "")), nil
		}
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
			ch <- &params.Update
		}
	}
}

func (a *acpConversationProcess) handlePermissionRequest(msg *rpcResponse) {
	if msg == nil || msg.ID == nil {
		return
	}

	var params struct {
		Options []struct {
			OptionID string `json:"optionId"`
			Kind     string `json:"kind"`
		} `json:"options"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return
	}

	optionID := ""
	for _, option := range params.Options {
		if strings.TrimSpace(option.OptionID) == "" {
			continue
		}
		optionID = option.OptionID
		if strings.EqualFold(strings.TrimSpace(option.Kind), "allow") {
			break
		}
	}
	if optionID == "" {
		return
	}

	data, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      *msg.ID,
		Params: map[string]interface{}{
			"outcome": map[string]interface{}{
				"outcome":  "selected",
				"optionId": optionID,
			},
		},
	})
	if err != nil {
		return
	}

	a.mu.Lock()
	_, _ = fmt.Fprintf(a.stdin, "%s\n", data)
	a.mu.Unlock()
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
