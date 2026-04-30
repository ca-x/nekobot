package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/agent"
	"nekobot/pkg/audit"
	"nekobot/pkg/bus"
	"nekobot/pkg/channels"
	"nekobot/pkg/cron"
	"nekobot/pkg/daemonhost"
	eventlog "nekobot/pkg/events"
	"nekobot/pkg/idempotency"
	"nekobot/pkg/runtimeagents"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/tasks"
	"nekobot/pkg/version"
)

const (
	daemonFollowStoreKey   = "daemon.collaboration.followed_threads.v1"
	daemonAgentEnvStoreKey = "daemon.collaboration.agent_env.v1"
	daemonReminderMetaKey  = "daemon.collaboration.reminder_meta.v1"
	daemonActivityStoreKey = "daemon.collaboration.activity.v1"
	daemonAttachmentKey    = "daemon.collaboration.attachments.v1"

	maxDaemonAttachmentBytes = 32 << 20
)

type storedAttachment struct {
	Record        *daemonv1.AttachmentRecord `json:"record"`
	ContentBase64 string                     `json:"content_base64"`
}

func (s *Server) ListChannels(ctx context.Context, req *daemonv1.ListChannelsRequest) (*daemonv1.ListChannelsResponse, error) {
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	items := []*daemonv1.ChannelRecord{
		{
			Target:      "#websocket",
			ChannelId:   "websocket",
			DisplayName: "WebSocket",
			ChannelType: "websocket",
			Enabled:     true,
		},
	}
	for _, account := range s.listChannelAccountRecords(ctx) {
		if len(items) >= limit {
			break
		}
		items = append(items, account)
	}
	return &daemonv1.ListChannelsResponse{Channels: items}, nil
}

func (s *Server) ListThreads(ctx context.Context, req *daemonv1.ListThreadsRequest) (*daemonv1.ListThreadsResponse, error) {
	if s == nil || s.sessionMgr == nil {
		return nil, fmt.Errorf("session manager not available")
	}
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	targetPrefix := strings.TrimSpace(req.GetTargetPrefix())
	ids, err := s.sessionMgr.List()
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	followed := s.followedThreadSet(ctx)
	records := make([]*daemonv1.ThreadRecord, 0, len(ids))
	for _, id := range ids {
		target := threadTarget(id)
		if targetPrefix != "" && !strings.HasPrefix(target, targetPrefix) {
			continue
		}
		record, err := s.threadRecord(ctx, id, followed)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].UpdatedTimeUnix == records[j].UpdatedTimeUnix {
			return records[i].Target < records[j].Target
		}
		return records[i].UpdatedTimeUnix > records[j].UpdatedTimeUnix
	})
	if len(records) > limit {
		records = records[:limit]
	}
	return &daemonv1.ListThreadsResponse{Threads: records}, nil
}

func (s *Server) GetThread(ctx context.Context, req *daemonv1.GetThreadRequest) (*daemonv1.GetThreadResponse, error) {
	sessionID, err := collaborationSessionID(req.GetTarget())
	if err != nil {
		return nil, err
	}
	record, err := s.threadRecord(ctx, sessionID, s.followedThreadSet(ctx))
	if err != nil {
		return nil, err
	}
	return &daemonv1.GetThreadResponse{Thread: record}, nil
}

func (s *Server) ReadMessages(ctx context.Context, req *daemonv1.ReadMessagesRequest) (*daemonv1.ReadMessagesResponse, error) {
	if s == nil || s.sessionMgr == nil {
		return nil, fmt.Errorf("session manager not available")
	}
	sessionID, err := collaborationSessionID(req.GetTarget())
	if err != nil {
		return nil, err
	}
	sess, err := s.sessionMgr.GetExisting(sessionID)
	if err != nil {
		return nil, fmt.Errorf("load thread %s: %w", sessionID, err)
	}
	messages := sess.GetMessages()
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	out := make([]*daemonv1.CollaborationMessage, 0, len(messages))
	target := threadTarget(sessionID)
	for i, msg := range messages {
		out = append(out, sessionMessageToProto(target, sessionID, i, msg, sess.GetUpdatedAt()))
	}
	return &daemonv1.ReadMessagesResponse{Messages: out}, nil
}

func (s *Server) SendMessage(ctx context.Context, req *daemonv1.SendMessageRequest) (*daemonv1.SendMessageResponse, error) {
	if s == nil || s.sessionMgr == nil {
		return nil, fmt.Errorf("session manager not available")
	}
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.GetContent())
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	// Idempotency guard.
	reqID := strings.TrimSpace(req.GetRequestId())
	idemKey := idempotency.Key{
		CallerKind: "agent",
		CallerID:   strings.TrimSpace(req.GetSenderAgentId()),
		Method:     "SendMessage",
		RequestID:  reqID,
	}
	if reqID != "" && s.idempotencyStore != nil {
		hash := idempotentRequestHash(
			"target", req.GetTarget(),
			"content", content,
			"role", req.GetRole(),
			"sender_agent_id", req.GetSenderAgentId(),
			"sender_display_name", req.GetSenderDisplayName(),
			"reply_to_message_id", req.GetReplyToMessageId(),
		)
		result, err := s.idempotencyStore.Reserve(ctx, idemKey, hash, 7*24*time.Hour)
		if err != nil {
			return nil, err
		}
		switch result.Outcome {
		case idempotency.OutcomeReplay:
			return replaySendMessage(result.Record)
		case idempotency.OutcomeConflict:
			return nil, fmt.Errorf("idempotency conflict: request_id %s already used with different body", reqID)
		case idempotency.OutcomeInProgress:
			return nil, fmt.Errorf("request %s is already being processed", reqID)
		}
		// OutcomeReserved — continue with mutation.
	}

	idemReserved := reqID != "" && s.idempotencyStore != nil

	sessionID, err := collaborationSessionID(target)
	if err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, err
	}
	role := normalizeMessageRole(req.GetRole())
	sess, err := s.sessionMgr.GetWithSource(sessionID, session.SourceGateway)
	if err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, fmt.Errorf("load thread %s: %w", sessionID, err)
	}
	msg := agent.Message{Role: role, Content: content}
	sess.AddMessage(msg)
	if err := s.sessionMgr.Save(sess); err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, fmt.Errorf("save thread %s: %w", sessionID, err)
	}
	protoMsg := &daemonv1.CollaborationMessage{
		MessageId:         uuid.NewString(),
		Target:            target,
		ThreadId:          sessionID,
		Role:              role,
		Content:           content,
		SenderAgentId:     strings.TrimSpace(req.GetSenderAgentId()),
		SenderDisplayName: strings.TrimSpace(req.GetSenderDisplayName()),
		ReplyToMessageId:  strings.TrimSpace(req.GetReplyToMessageId()),
		CreatedTimeUnix:   time.Now().Unix(),
	}
	if req.GetEmitOutbound() && s.bus != nil {
		_ = s.bus.SendOutbound(&bus.Message{
			ID:        protoMsg.MessageId,
			ChannelID: targetChannelID(target),
			SessionID: sessionID,
			UserID:    protoMsg.SenderAgentId,
			Username:  protoMsg.SenderDisplayName,
			Type:      bus.MessageTypeText,
			Content:   content,
			Timestamp: time.Now(),
			ReplyTo:   protoMsg.ReplyToMessageId,
		})
	}

	resp := &daemonv1.SendMessageResponse{Accepted: true, Message: protoMsg}
	if idemReserved {
		if err := completeIdempotency(ctx, s.idempotencyStore, idemKey, idempotency.CompleteRequest{
			ResponseType: "json:SendMessageResponse",
			ResponseJSON: mustMarshalJSON(resp),
			ResourceKind: "message",
			ResourceID:   protoMsg.MessageId,
		}); err != nil {
			// Mutation succeeded but idempotency completion failed.
			// Return the successful response; the record stays pending
			// which means future retries get "in_progress" (retriable).
		}
	}
	return resp, nil
}

func (s *Server) FollowThread(ctx context.Context, req *daemonv1.FollowThreadRequest) (*daemonv1.FollowThreadResponse, error) {
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	if _, err := collaborationSessionID(target); err != nil {
		return nil, err
	}
	return &daemonv1.FollowThreadResponse{Accepted: true}, s.updateFollowedThread(ctx, target, true)
}

func (s *Server) UnfollowThread(ctx context.Context, req *daemonv1.UnfollowThreadRequest) (*daemonv1.UnfollowThreadResponse, error) {
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	return &daemonv1.UnfollowThreadResponse{Accepted: true}, s.updateFollowedThread(ctx, target, false)
}

func (s *Server) CreateCollaborationTask(ctx context.Context, req *daemonv1.CreateCollaborationTaskRequest) (*daemonv1.CreateCollaborationTaskResponse, error) {
	if s == nil || s.agent == nil || s.agent.TaskService() == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	summary := strings.TrimSpace(req.GetSummary())
	if summary == "" {
		return nil, fmt.Errorf("summary is required")
	}

	// Idempotency guard.
	reqID := strings.TrimSpace(req.GetRequestId())
	idemKey := idempotency.Key{
		CallerKind: "agent",
		CallerID:   strings.TrimSpace(req.GetAgentId()),
		Method:     "CreateCollaborationTask",
		RequestID:  reqID,
	}
	if reqID != "" && s.idempotencyStore != nil {
		hash := idempotentRequestHash(
			"target", req.GetTarget(),
			"summary", summary,
			"agent_id", req.GetAgentId(),
			"created_by_user_id", req.GetCreatedByUserId(),
		)
		result, err := s.idempotencyStore.Reserve(ctx, idemKey, hash, 7*24*time.Hour)
		if err != nil {
			return nil, err
		}
		switch result.Outcome {
		case idempotency.OutcomeReplay:
			return replayCreateTask(result.Record)
		case idempotency.OutcomeConflict:
			return nil, fmt.Errorf("idempotency conflict: request_id %s already used with different body", reqID)
		case idempotency.OutcomeInProgress:
			return nil, fmt.Errorf("request %s is already being processed", reqID)
		}
	}

	idemReserved := reqID != "" && s.idempotencyStore != nil

	sessionID, err := collaborationSessionID(target)
	if err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, err
	}
	task, err := s.agent.TaskService().Enqueue(tasks.Task{
		ID:        "collab-task-" + uuid.NewString(),
		Type:      tasks.TypeRemoteAgent,
		Summary:   summary,
		SessionID: sessionID,
		RuntimeID: strings.TrimSpace(req.GetAgentId()),
		Metadata: map[string]any{
			"target":             target,
			"created_by_user_id": strings.TrimSpace(req.GetCreatedByUserId()),
			"delivery":           "daemon-collaboration",
		},
	})
	if err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, err
	}

	resp := &daemonv1.CreateCollaborationTaskResponse{Task: daemonhost.CollaborationTaskToProto(task)}
	if idemReserved {
		if err := completeIdempotency(ctx, s.idempotencyStore, idemKey, idempotency.CompleteRequest{
			ResponseType: "json:CreateCollaborationTaskResponse",
			ResponseJSON: mustMarshalJSON(resp),
			ResourceKind: "task",
			ResourceID:   task.ID,
		}); err != nil {
			// Mutation succeeded but idempotency completion failed.
		}
	}
	return resp, nil
}

func (s *Server) ListCollaborationTasks(ctx context.Context, req *daemonv1.ListCollaborationTasksRequest) (*daemonv1.ListCollaborationTasksResponse, error) {
	if s == nil || s.agent == nil || s.agent.TaskService() == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	target := strings.TrimSpace(req.GetTarget())
	runtimeID := strings.TrimSpace(req.GetAgentId())
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	out := make([]*daemonv1.Task, 0)
	for _, item := range s.agent.TaskService().List() {
		if target != "" && metadataString(item.Metadata, "target") != target && threadTarget(item.SessionID) != target {
			continue
		}
		if runtimeID != "" && strings.TrimSpace(item.RuntimeID) != runtimeID {
			continue
		}
		out = append(out, daemonhost.CollaborationTaskToProto(item))
		if len(out) >= limit {
			break
		}
	}
	return &daemonv1.ListCollaborationTasksResponse{Tasks: out}, nil
}

func (s *Server) ClaimCollaborationTask(ctx context.Context, req *daemonv1.ClaimCollaborationTaskRequest) (*daemonv1.ClaimCollaborationTaskResponse, error) {
	if s == nil || s.agent == nil || s.agent.TaskService() == nil {
		return nil, fmt.Errorf("task service unavailable")
	}

	// Idempotency guard.
	reqID := strings.TrimSpace(req.GetRequestId())
	idemKey := idempotency.Key{
		CallerKind: "agent",
		CallerID:   strings.TrimSpace(req.GetAgentId()),
		Method:     "ClaimCollaborationTask",
		RequestID:  reqID,
	}
	if reqID != "" && s.idempotencyStore != nil {
		hash := idempotentRequestHash(
			"task_id", req.GetTaskId(),
			"agent_id", req.GetAgentId(),
		)
		result, err := s.idempotencyStore.Reserve(ctx, idemKey, hash, 7*24*time.Hour)
		if err != nil {
			return nil, err
		}
		switch result.Outcome {
		case idempotency.OutcomeReplay:
			return replayClaimTask(result.Record)
		case idempotency.OutcomeConflict:
			return nil, fmt.Errorf("idempotency conflict: request_id %s already used with different body", reqID)
		case idempotency.OutcomeInProgress:
			return nil, fmt.Errorf("request %s is already being processed", reqID)
		}
	}

	idemReserved := reqID != "" && s.idempotencyStore != nil

	task, err := s.agent.TaskService().Claim(req.GetTaskId(), req.GetAgentId())
	if err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, err
	}

	resp := &daemonv1.ClaimCollaborationTaskResponse{Task: daemonhost.CollaborationTaskToProto(task)}
	if idemReserved {
		if err := completeIdempotency(ctx, s.idempotencyStore, idemKey, idempotency.CompleteRequest{
			ResponseType: "json:ClaimCollaborationTaskResponse",
			ResponseJSON: mustMarshalJSON(resp),
			ResourceKind: "task",
			ResourceID:   req.GetTaskId(),
		}); err != nil {
			// Mutation succeeded but idempotency completion failed.
		}
	}
	return resp, nil
}

// --- Task Graph RPCs ---

type splitProposal struct {
	ProposalID    string
	ParentTaskID  string
	ProposedTasks []*daemonv1.Task
}

func (s *Server) ProposeTaskSplit(_ context.Context, req *daemonv1.ProposeTaskSplitRequest) (*daemonv1.ProposeTaskSplitResponse, error) {
	if s == nil || s.agent == nil || s.agent.TaskService() == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	parentID := strings.TrimSpace(req.GetParentTaskId())
	if parentID == "" {
		return nil, fmt.Errorf("parent_task_id is required")
	}
	parentTask, err := s.agent.TaskService().Get(parentID)
	if err != nil {
		return nil, fmt.Errorf("parent task not found: %s", parentID)
	}

	// Map client_proposed_id → server-assigned proposed ID.
	proposedIDMap := map[string]string{}
	for _, p := range req.GetProposedTasks() {
		cid := strings.TrimSpace(p.GetClientProposedId())
		if cid == "" {
			return nil, fmt.Errorf("client_proposed_id is required for each proposed subtask")
		}
		if _, dup := proposedIDMap[cid]; dup {
			return nil, fmt.Errorf("duplicate client_proposed_id: %s", cid)
		}
		proposedIDMap[cid] = "proposed-" + uuid.NewString()
	}

	proposedTasks := make([]*daemonv1.Task, 0, len(req.GetProposedTasks()))
	for _, p := range req.GetProposedTasks() {
		cid := p.GetClientProposedId()
		proposedTasks = append(proposedTasks, &daemonv1.Task{
			TaskId:               proposedIDMap[cid],
			Summary:              p.GetSummary(),
			AssigneeId:           p.GetAssigneeId(),
			RequiredCapabilities: p.GetRequiredCapabilities(),
			RootTaskId:           metadataString(parentTask.Metadata, "root_task_id"),
			ParentTaskId:         parentID,
			Source:               "agent",
		})
	}

	proposalID := "split-" + uuid.NewString()
	if s.splitProposals == nil {
		s.splitProposals = map[string]*splitProposal{}
	}
	s.splitProposals[proposalID] = &splitProposal{
		ProposalID:    proposalID,
		ParentTaskID:  parentID,
		ProposedTasks: proposedTasks,
	}

	return &daemonv1.ProposeTaskSplitResponse{
		ProposalId:    proposalID,
		ParentTask:    daemonhost.CollaborationTaskToProto(parentTask),
		ProposedTasks: proposedTasks,
		Accepted:      true,
	}, nil
}

func (s *Server) ApplyTaskSplit(ctx context.Context, req *daemonv1.ApplyTaskSplitRequest) (*daemonv1.ApplyTaskSplitResponse, error) {
	if s == nil || s.agent == nil || s.agent.TaskService() == nil {
		return nil, fmt.Errorf("task service unavailable")
	}

	// Idempotency guard.
	reqID := strings.TrimSpace(req.GetRequestId())
	var idemKey idempotency.Key
	idemReserved := false
	if reqID != "" && s.idempotencyStore != nil {
		hash := idempotentRequestHash(
			"parent_task_id", req.GetParentTaskId(),
			"proposal_id", req.GetProposalId(),
		)
		idemKey = idempotency.Key{
			CallerKind: "agent",
			CallerID:   strings.TrimSpace(req.GetParentTaskId()),
			Method:     "ApplyTaskSplit",
			RequestID:  reqID,
		}
		result, err := s.idempotencyStore.Reserve(ctx, idemKey, hash, 30*24*time.Hour)
		if err != nil {
			return nil, err
		}
		switch result.Outcome {
		case idempotency.OutcomeReplay:
			return replayApplyTaskSplit(result.Record)
		case idempotency.OutcomeConflict:
			return nil, fmt.Errorf("idempotency conflict: request_id %s already used with different body", reqID)
		case idempotency.OutcomeInProgress:
			return nil, fmt.Errorf("request %s is already being processed", reqID)
		}
		idemReserved = true
	}

	proposalID := strings.TrimSpace(req.GetProposalId())
	if s.splitProposals == nil || s.splitProposals[proposalID] == nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, fmt.Errorf("split proposal not found: %s", proposalID))
		}
		return nil, fmt.Errorf("split proposal not found: %s", proposalID)
	}
	proposal := s.splitProposals[proposalID]

	// Build proposed task lookup.
	proposedByID := map[string]*daemonv1.Task{}
	for _, pt := range proposal.ProposedTasks {
		proposedByID[pt.GetTaskId()] = pt
	}

	// Determine which subtasks to create.
	selectedIDs := req.GetSelectedTaskIds()
	if len(selectedIDs) == 0 {
		selectedIDs = make([]string, 0, len(proposal.ProposedTasks))
		for _, pt := range proposal.ProposedTasks {
			selectedIDs = append(selectedIDs, pt.GetTaskId())
		}
	}
	for _, sid := range selectedIDs {
		if proposedByID[sid] == nil {
			if idemReserved {
				failIdempotency(ctx, s.idempotencyStore, idemKey, fmt.Errorf("selected task %s not in proposal %s", sid, proposalID))
			}
			return nil, fmt.Errorf("selected task %s not in proposal %s", sid, proposalID)
		}
	}

	parentID := proposal.ParentTaskID
	parentTask, err := s.agent.TaskService().Get(parentID)
	if err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, fmt.Errorf("parent task not found: %s", parentID))
		}
		return nil, fmt.Errorf("parent task not found: %s", parentID)
	}

	target := metadataString(parentTask.Metadata, "target")
	sessionID, err := collaborationSessionID(target)
	if err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, err
	}

	currentVersion := metadataInt64(parentTask.Metadata, "graph_version")
	newVersion := currentVersion + int64(len(selectedIDs)) + 1

	createdSubtasks := make([]*daemonv1.Task, 0, len(selectedIDs))
	for _, sid := range selectedIDs {
		pt := proposedByID[sid]
		newID := "collab-task-" + uuid.NewString()
		subTask, err := s.agent.TaskService().Enqueue(tasks.Task{
			ID:        newID,
			Type:      tasks.TypeRemoteAgent,
			Summary:   pt.GetSummary(),
			SessionID: sessionID,
			RuntimeID: strings.TrimSpace(pt.GetRuntimeId()),
			Metadata: map[string]any{
				"target":                target,
				"source":                "agent",
				"root_task_id":          metadataString(parentTask.Metadata, "root_task_id"),
				"parent_task_id":        parentID,
				"split_proposal_id":     proposalID,
				"assignee_id":           pt.GetAssigneeId(),
				"required_capabilities": pt.GetRequiredCapabilities(),
				"graph_version":         newVersion,
				"delivery":              "daemon-collaboration",
			},
		})
		if err != nil {
			// Best-effort cleanup: cancel already-created subtasks in this batch.
			for _, prev := range createdSubtasks {
				_, _ = s.agent.TaskService().Cancel(prev.TaskId)
			}
			if idemReserved {
				failIdempotency(ctx, s.idempotencyStore, idemKey, err)
			}
			return nil, err
		}
		createdSubtasks = append(createdSubtasks, daemonhost.CollaborationTaskToProto(subTask))
	}

	// Clean up the used proposal.
	delete(s.splitProposals, proposalID)

	resp := &daemonv1.ApplyTaskSplitResponse{
		ParentTask:      daemonhost.CollaborationTaskToProto(parentTask),
		CreatedSubtasks: createdSubtasks,
		NewGraphVersion: newVersion,
	}
	if idemReserved {
		if err := completeIdempotency(ctx, s.idempotencyStore, idemKey, idempotency.CompleteRequest{
			ResponseType: "json:ApplyTaskSplitResponse",
			ResponseJSON: mustMarshalJSON(resp),
			ResourceKind: "task_graph",
			ResourceID:   parentID,
		}); err != nil {
			// Mutation succeeded but idempotency completion failed.
		}
	}
	return resp, nil
}

func (s *Server) CreateTaskGraph(ctx context.Context, req *daemonv1.CreateTaskGraphRequest) (*daemonv1.CreateTaskGraphResponse, error) {
	if s == nil || s.agent == nil || s.agent.TaskService() == nil {
		return nil, fmt.Errorf("task service unavailable")
	}

	root := req.GetRootTask()
	if root == nil {
		return nil, fmt.Errorf("root_task is required")
	}
	target := strings.TrimSpace(root.GetTarget())
	if _, err := daemonhost.ValidateCollaborationTarget(target); err != nil {
		return nil, err
	}

	// Idempotency guard.
	reqID := strings.TrimSpace(req.GetRequestId())
	var idemKey idempotency.Key
	idemReserved := false
	if reqID != "" && s.idempotencyStore != nil {
		hash := idempotentRequestHash(
			"root_task", protoHashString(root),
			"subtasks", protoHashStringSlice(req.GetSubtasks()),
			"dependencies", protoHashEdgeSlice(req.GetDependencies()),
		)
		idemKey = idempotency.Key{
			CallerKind: "agent",
			CallerID:   strings.TrimSpace(root.GetCreatedByAgentId()),
			Method:     "CreateTaskGraph",
			RequestID:  reqID,
		}
		result, err := s.idempotencyStore.Reserve(ctx, idemKey, hash, 30*24*time.Hour)
		if err != nil {
			return nil, err
		}
		switch result.Outcome {
		case idempotency.OutcomeReplay:
			return replayCreateTaskGraph(result.Record)
		case idempotency.OutcomeConflict:
			return nil, fmt.Errorf("idempotency conflict: request_id %s already used with different body", reqID)
		case idempotency.OutcomeInProgress:
			return nil, fmt.Errorf("request %s is already being processed", reqID)
		}
		idemReserved = true
	}

	sessionID, err := collaborationSessionID(target)
	if err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, err
	}

	graphVersion := int64(1)
	rootID := "collab-task-" + uuid.NewString()

	// Pre-allocate subtask IDs.
	subtaskIDs := make([]string, len(req.GetSubtasks()))
	for i := range req.GetSubtasks() {
		subtaskIDs[i] = "collab-task-" + uuid.NewString()
	}

	// Build dependency map: index → depends_on subtask IDs.
	// Dependencies reference subtasks by position index ("0", "1", ...).
	depsBySubtask := map[int][]string{}
	for _, edge := range req.GetDependencies() {
		fromIdx, err1 := parseSubtaskIndex(edge.GetFromTaskId(), len(subtaskIDs))
		toIdx, err2 := parseSubtaskIndex(edge.GetToTaskId(), len(subtaskIDs))
		if err1 != nil || err2 != nil {
			continue // skip edges with invalid indexes
		}
		if fromIdx == toIdx {
			continue
		}
		depsBySubtask[toIdx] = append(depsBySubtask[toIdx], subtaskIDs[fromIdx])
	}

	// Create root task first (atomic ordering: root exists before children).
	rootTask, err := s.agent.TaskService().Enqueue(tasks.Task{
		ID:        rootID,
		Type:      tasks.TypeRemoteAgent,
		Summary:   root.GetSummary(),
		SessionID: sessionID,
		RuntimeID: strings.TrimSpace(root.GetRuntimeId()),
		Metadata: map[string]any{
			"target":              target,
			"created_by_user_id":  root.GetCreatedByUserId(),
			"source":              "agent",
			"created_by_agent_id": root.GetCreatedByAgentId(),
			"root_task_id":        rootID,
			"subtask_ids":         subtaskIDs,
			"graph_version":       graphVersion,
			"delivery":            "daemon-collaboration",
		},
	})
	if err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, err
	}

	// Create subtasks. On failure, cancel already-created subtasks (root stays).
	createdSubtasks := make([]*daemonv1.Task, 0, len(req.GetSubtasks()))
	for i, st := range req.GetSubtasks() {
		graphVersion++
		meta := map[string]any{
			"target":                target,
			"created_by_user_id":    st.GetCreatedByUserId(),
			"source":                "agent",
			"root_task_id":          rootID,
			"parent_task_id":        rootID,
			"graph_version":         graphVersion,
			"delivery":              "daemon-collaboration",
			"required_capabilities": st.GetRequiredCapabilities(),
		}
		if deps := depsBySubtask[i]; len(deps) > 0 {
			meta["depends_on_task_ids"] = toAnySlice(deps)
		}
		subTask, err := s.agent.TaskService().Enqueue(tasks.Task{
			ID:        subtaskIDs[i],
			Type:      tasks.TypeRemoteAgent,
			Summary:   st.GetSummary(),
			SessionID: sessionID,
			RuntimeID: strings.TrimSpace(st.GetRuntimeId()),
			Metadata:  meta,
		})
		if err != nil {
			// Best-effort cleanup: cancel already-created subtasks.
			for j := 0; j < i; j++ {
				_, _ = s.agent.TaskService().Cancel(subtaskIDs[j])
			}
			if idemReserved {
				failIdempotency(ctx, s.idempotencyStore, idemKey, err)
			}
			return nil, err
		}
		createdSubtasks = append(createdSubtasks, daemonhost.CollaborationTaskToProto(subTask))
	}

	// Build dependency edges for the response.
	var edges []*daemonv1.TaskEdge
	for i, deps := range depsBySubtask {
		for _, depID := range deps {
			edges = append(edges, &daemonv1.TaskEdge{
				FromTaskId: depID,
				ToTaskId:   subtaskIDs[i],
				Kind:       "depends_on",
			})
		}
	}
	// Parent-child edges.
	for _, sid := range subtaskIDs {
		edges = append(edges, &daemonv1.TaskEdge{
			FromTaskId: rootID,
			ToTaskId:   sid,
			Kind:       "parent_child",
		})
	}

	snapshot := &daemonv1.TaskGraphSnapshot{
		RootTaskId:   rootID,
		GraphVersion: graphVersion,
		Nodes:        append([]*daemonv1.Task{daemonhost.CollaborationTaskToProto(rootTask)}, createdSubtasks...),
		Edges:        edges,
	}

	resp := &daemonv1.CreateTaskGraphResponse{Graph: snapshot}
	if idemReserved {
		if err := completeIdempotency(ctx, s.idempotencyStore, idemKey, idempotency.CompleteRequest{
			ResponseType: "json:CreateTaskGraphResponse",
			ResponseJSON: mustMarshalJSON(resp),
			ResourceKind: "task_graph",
			ResourceID:   rootID,
		}); err != nil {
			// Mutation succeeded but idempotency completion failed.
		}
	}
	return resp, nil
}

func (s *Server) ListTaskGraph(_ context.Context, req *daemonv1.ListTaskGraphRequest) (*daemonv1.ListTaskGraphResponse, error) {
	if s == nil || s.agent == nil || s.agent.TaskService() == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	rootID := strings.TrimSpace(req.GetRootTaskId())
	if rootID == "" {
		return nil, fmt.Errorf("root_task_id is required")
	}

	allTasks := s.agent.TaskService().List()
	var nodes []*daemonv1.Task
	var maxVersion int64
	for _, t := range allTasks {
		if metadataString(t.Metadata, "root_task_id") == rootID || t.ID == rootID {
			proto := daemonhost.CollaborationTaskToProto(t)
			nodes = append(nodes, proto)
			if proto.GraphVersion > maxVersion {
				maxVersion = proto.GraphVersion
			}
		}
	}
	if nodes == nil {
		return nil, fmt.Errorf("task graph not found: %s", rootID)
	}

	// Build edges from parent/child relationships.
	var edges []*daemonv1.TaskEdge
	for _, n := range nodes {
		if n.ParentTaskId != "" && n.ParentTaskId != n.TaskId {
			edges = append(edges, &daemonv1.TaskEdge{
				FromTaskId: n.ParentTaskId,
				ToTaskId:   n.TaskId,
				Kind:       "parent_child",
			})
		}
		for _, depID := range n.DependsOnTaskIds {
			edges = append(edges, &daemonv1.TaskEdge{
				FromTaskId: depID,
				ToTaskId:   n.TaskId,
				Kind:       "depends_on",
			})
		}
	}

	return &daemonv1.ListTaskGraphResponse{
		Graph: &daemonv1.TaskGraphSnapshot{
			RootTaskId:   rootID,
			GraphVersion: maxVersion,
			Nodes:        nodes,
			Edges:        edges,
		},
	}, nil
}

func (s *Server) UpdateTaskGraph(_ context.Context, _ *daemonv1.UpdateTaskGraphRequest) (*daemonv1.UpdateTaskGraphResponse, error) {
	return nil, status.Error(codes.Unimplemented, "UpdateTaskGraph not yet implemented")
}

func (s *Server) GetServerInfo(ctx context.Context, req *daemonv1.GetServerInfoRequest) (*daemonv1.GetServerInfoResponse, error) {
	channels, err := s.ListChannels(ctx, &daemonv1.ListChannelsRequest{Limit: 200})
	if err != nil {
		return nil, err
	}
	agents, err := s.ListAgentProfiles(ctx, &daemonv1.ListAgentProfilesRequest{Limit: 200})
	if err != nil {
		return nil, err
	}
	return &daemonv1.GetServerInfoResponse{
		ServerName: "nekobot",
		Version:    version.GetVersion(),
		Channels:   channels.Channels,
		Agents:     agents.Profiles,
	}, nil
}

func (s *Server) GetAgentProfile(ctx context.Context, req *daemonv1.GetAgentProfileRequest) (*daemonv1.GetAgentProfileResponse, error) {
	runtimeID := strings.TrimSpace(req.GetAgentId())
	profiles, err := s.agentProfiles(ctx, 1000)
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if runtimeID == "" || profile.AgentId == runtimeID || profile.Name == runtimeID {
			return &daemonv1.GetAgentProfileResponse{Profile: profile}, nil
		}
	}
	return nil, fmt.Errorf("agent profile not found: %s", runtimeID)
}

func (s *Server) SetAgentEnv(ctx context.Context, req *daemonv1.SetAgentEnvRequest) (*daemonv1.SetAgentEnvResponse, error) {
	runtimeID := strings.TrimSpace(req.GetAgentId())
	if runtimeID == "" {
		return nil, fmt.Errorf("runtime_id is required")
	}
	env := normalizeEnvVars(req.GetEnv())
	if err := s.storeAgentEnv(ctx, runtimeID, env); err != nil {
		return nil, err
	}
	profile, err := s.GetAgentProfile(ctx, &daemonv1.GetAgentProfileRequest{AgentId: runtimeID})
	if err != nil {
		return nil, err
	}
	return &daemonv1.SetAgentEnvResponse{Profile: profile.Profile}, nil
}

func (s *Server) ListAgentProfiles(ctx context.Context, req *daemonv1.ListAgentProfilesRequest) (*daemonv1.ListAgentProfilesResponse, error) {
	profiles, err := s.agentProfiles(ctx, normalizedCollaborationLimit(req.GetLimit(), 200))
	if err != nil {
		return nil, err
	}
	return &daemonv1.ListAgentProfilesResponse{Profiles: profiles}, nil
}

func (s *Server) ListAgentDMs(ctx context.Context, req *daemonv1.ListAgentDMsRequest) (*daemonv1.ListAgentDMsResponse, error) {
	profiles, err := s.agentProfiles(ctx, normalizedCollaborationLimit(req.GetLimit(), 200))
	if err != nil {
		return nil, err
	}
	self := strings.TrimSpace(req.GetAgentId())
	dms := make([]*daemonv1.ChannelRecord, 0, len(profiles))
	for _, profile := range profiles {
		if profile == nil || profile.AgentId == "" || profile.AgentId == self {
			continue
		}
		dms = append(dms, &daemonv1.ChannelRecord{
			Target:      agentDMTarget(profile.AgentId),
			ChannelId:   "dm:" + profile.AgentId,
			DisplayName: profile.DisplayName,
			ChannelType: "dm",
			Enabled:     profile.Enabled,
		})
	}
	return &daemonv1.ListAgentDMsResponse{Dms: dms}, nil
}

func (s *Server) ControlAgent(ctx context.Context, req *daemonv1.ControlAgentRequest) (*daemonv1.ControlAgentResponse, error) {
	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	if req.GetAction() == daemonv1.AgentControlAction_AGENT_CONTROL_ACTION_UNSPECIFIED {
		return nil, fmt.Errorf("action is required")
	}
	profile, err := s.findAgentProfile(ctx, agentID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	operation := &daemonv1.AgentControlOperation{
		OperationId:        "agent-control-" + uuid.NewString(),
		AgentId:            profile.GetAgentId(),
		ComputerId:         firstNonEmpty(strings.TrimSpace(req.GetComputerId()), profile.GetComputerId()),
		RuntimeProfileId:   firstNonEmpty(strings.TrimSpace(req.GetRuntimeProfileId()), profile.GetRuntimeProfileId()),
		Action:             req.GetAction(),
		State:              "unsupported",
		Reason:             strings.TrimSpace(req.GetReason()),
		RequestedByAgentId: strings.TrimSpace(req.GetRequestedByAgentId()),
		CreatedTimeUnix:    now,
		UpdatedTimeUnix:    now,
	}
	_, err = s.appendCollaborationEvent(ctx, eventlog.EventRecord{
		EventType:      "agent.control_requested",
		ActorKind:      "agent",
		ActorID:        operation.RequestedByAgentId,
		SubjectKind:    "agent",
		SubjectID:      operation.AgentId,
		IdempotencyKey: strings.TrimSpace(req.GetRequestId()),
		PayloadJSON:    mustMarshalJSON(operation),
	})
	if err != nil {
		return nil, err
	}
	return &daemonv1.ControlAgentResponse{
		Accepted:  false,
		Operation: operation,
		Profile:   profile,
	}, nil
}

func (s *Server) SendAgentDirectMessage(ctx context.Context, req *daemonv1.SendAgentDirectMessageRequest) (*daemonv1.SendAgentDirectMessageResponse, error) {
	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	resp, err := s.SendMessage(ctx, &daemonv1.SendMessageRequest{
		Target:            agentDMTarget(agentID),
		Role:              "user",
		Content:           req.GetContent(),
		SenderAgentId:     strings.TrimSpace(req.GetSenderAgentId()),
		SenderDisplayName: strings.TrimSpace(req.GetSenderDisplayName()),
		ReplyToMessageId:  strings.TrimSpace(req.GetReplyToMessageId()),
		RequestId:         strings.TrimSpace(req.GetRequestId()),
		AttachmentIds:     req.GetAttachmentIds(),
	})
	if err != nil {
		return nil, err
	}
	return &daemonv1.SendAgentDirectMessageResponse{Accepted: resp.GetAccepted(), Message: resp.GetMessage()}, nil
}

func (s *Server) ScheduleReminder(ctx context.Context, req *daemonv1.ScheduleReminderRequest) (*daemonv1.ScheduleReminderResponse, error) {
	if s == nil || s.cronMgr == nil {
		return nil, fmt.Errorf("reminder scheduler unavailable")
	}
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		name = "daemon reminder"
	}
	prompt := strings.TrimSpace(req.GetPrompt())
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	route := cron.RouteOptions{Skills: req.GetSkills()}
	var job *cron.Job
	switch cron.ScheduleKind(strings.ToLower(strings.TrimSpace(req.GetScheduleKind()))) {
	case "", cron.ScheduleCron:
		schedule := strings.TrimSpace(req.GetSchedule())
		if schedule == "" {
			return nil, fmt.Errorf("schedule is required")
		}
		job, err = s.cronMgr.AddCronJobWithRoute(name, schedule, prompt, route)
	case cron.ScheduleAt:
		at, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(req.GetSchedule()))
		if parseErr != nil {
			return nil, fmt.Errorf("invalid at schedule, use RFC3339: %w", parseErr)
		}
		job, err = s.cronMgr.AddAtJobWithRoute(name, at, prompt, true, route)
	case cron.ScheduleEvery:
		schedule := strings.TrimSpace(req.GetSchedule())
		if schedule == "" {
			return nil, fmt.Errorf("schedule is required")
		}
		job, err = s.cronMgr.AddEveryJobWithRoute(name, schedule, prompt, route)
	default:
		return nil, fmt.Errorf("invalid schedule_kind")
	}
	if err != nil {
		return nil, err
	}
	if err := s.storeReminderTarget(ctx, job.ID, target); err != nil {
		return nil, err
	}
	return &daemonv1.ScheduleReminderResponse{Reminder: reminderToProto(job, target)}, nil
}

func (s *Server) ListReminders(ctx context.Context, req *daemonv1.ListRemindersRequest) (*daemonv1.ListRemindersResponse, error) {
	if s == nil || s.cronMgr == nil {
		return nil, fmt.Errorf("reminder scheduler unavailable")
	}
	target := strings.TrimSpace(req.GetTarget())
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	targets := s.reminderTargets(ctx)
	jobs := s.cronMgr.ListJobs()
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i] == nil || jobs[j] == nil {
			return jobs[j] == nil
		}
		return jobs[i].NextRun.Before(jobs[j].NextRun)
	})
	out := make([]*daemonv1.ReminderRecord, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		jobTarget := targets[job.ID]
		if target != "" && jobTarget != target {
			continue
		}
		out = append(out, reminderToProto(job, jobTarget))
		if len(out) >= limit {
			break
		}
	}
	return &daemonv1.ListRemindersResponse{Reminders: out}, nil
}

func (s *Server) CancelReminder(ctx context.Context, req *daemonv1.CancelReminderRequest) (*daemonv1.CancelReminderResponse, error) {
	if s == nil || s.cronMgr == nil {
		return nil, fmt.Errorf("reminder scheduler unavailable")
	}
	reminderID := strings.TrimSpace(req.GetReminderId())
	if reminderID == "" {
		return nil, fmt.Errorf("reminder_id is required")
	}
	if err := s.cronMgr.RemoveJob(reminderID); err != nil {
		return nil, err
	}
	_ = s.deleteReminderTarget(ctx, reminderID)
	return &daemonv1.CancelReminderResponse{Accepted: true}, nil
}

func (s *Server) LogActivity(ctx context.Context, req *daemonv1.LogActivityRequest) (*daemonv1.LogActivityResponse, error) {
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	reqID := strings.TrimSpace(req.GetRequestId())
	idemKey := idempotency.Key{
		CallerKind: "agent",
		CallerID:   strings.TrimSpace(req.GetAgentId()),
		Method:     "LogActivity",
		RequestID:  reqID,
	}
	if reqID != "" && s.idempotencyStore != nil {
		hash := idempotentRequestHash(
			"target", req.GetTarget(),
			"agent_id", req.GetAgentId(),
			"kind", req.GetKind(),
			"summary", req.GetSummary(),
			"detail", req.GetDetail(),
			"run_id", req.GetRunId(),
			"step_id", req.GetStepId(),
		)
		result, err := s.idempotencyStore.Reserve(ctx, idemKey, hash, 7*24*time.Hour)
		if err != nil {
			return nil, err
		}
		switch result.Outcome {
		case idempotency.OutcomeReplay:
			return replayLogActivity(result.Record)
		case idempotency.OutcomeConflict:
			return nil, fmt.Errorf("idempotency conflict: request_id %s already used with different body", reqID)
		case idempotency.OutcomeInProgress:
			return nil, fmt.Errorf("request %s is already being processed", reqID)
		}
	}
	idemReserved := reqID != "" && s.idempotencyStore != nil

	activity := &daemonv1.ActivityRecord{
		ActivityId:      "activity-" + uuid.NewString(),
		Target:          target,
		AgentId:         strings.TrimSpace(req.GetAgentId()),
		Kind:            strings.TrimSpace(req.GetKind()),
		Summary:         strings.TrimSpace(req.GetSummary()),
		Detail:          strings.TrimSpace(req.GetDetail()),
		CreatedTimeUnix: time.Now().Unix(),
		RunId:           strings.TrimSpace(req.GetRunId()),
		StepId:          strings.TrimSpace(req.GetStepId()),
	}
	if activity.Kind == "" {
		activity.Kind = "event"
	}
	if activity.Summary == "" {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, fmt.Errorf("summary is required"))
		}
		return nil, fmt.Errorf("summary is required")
	}
	if err := s.appendActivity(ctx, activity); err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, err
	}
	eventID := ""
	if event, err := s.appendCollaborationEvent(ctx, eventlog.EventRecord{
		EventType:         "activity.logged",
		Target:            target,
		ActorKind:         "agent",
		ActorID:           activity.AgentId,
		SubjectKind:       "activity",
		SubjectID:         activity.ActivityId,
		ParentSubjectKind: "run",
		ParentSubjectID:   activity.RunId,
		IdempotencyKey:    reqID,
		PayloadJSON:       mustMarshalJSON(activity),
	}); err != nil {
		if idemReserved {
			failIdempotency(ctx, s.idempotencyStore, idemKey, err)
		}
		return nil, err
	} else if event != nil {
		eventID = event.EventID
	}
	if s.auditLogger != nil {
		s.auditLogger.Log(&audit.Entry{
			Timestamp:     time.Unix(activity.CreatedTimeUnix, 0),
			ToolName:      "daemon.activity." + activity.Kind,
			Arguments:     map[string]interface{}{"target": target, "runtime_id": activity.AgentId},
			Success:       true,
			ResultPreview: activity.Summary,
			SessionID:     target,
		})
	}
	resp := &daemonv1.LogActivityResponse{Activity: activity}
	if idemReserved {
		_ = completeIdempotency(ctx, s.idempotencyStore, idemKey, idempotency.CompleteRequest{
			ResponseType: "json:LogActivityResponse",
			ResponseJSON: mustMarshalJSON(resp),
			ResourceKind: "activity",
			ResourceID:   activity.ActivityId,
			EventID:      eventID,
		})
	}
	return resp, nil
}

func (s *Server) ListActivity(ctx context.Context, req *daemonv1.ListActivityRequest) (*daemonv1.ListActivityResponse, error) {
	target := strings.TrimSpace(req.GetTarget())
	runtimeID := strings.TrimSpace(req.GetAgentId())
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	activities := s.activityLog(ctx)
	out := make([]*daemonv1.ActivityRecord, 0, len(activities))
	for i := len(activities) - 1; i >= 0 && len(out) < limit; i-- {
		item := activities[i]
		if item == nil {
			continue
		}
		if target != "" && item.Target != target {
			continue
		}
		if runtimeID != "" && item.AgentId != runtimeID {
			continue
		}
		out = append(out, item)
	}
	return &daemonv1.ListActivityResponse{Activities: out}, nil
}

func (s *Server) UploadAttachment(ctx context.Context, req *daemonv1.UploadAttachmentRequest) (*daemonv1.UploadAttachmentResponse, error) {
	if s == nil || s.kvStore == nil {
		return nil, fmt.Errorf("attachment store unavailable")
	}
	target, err := daemonhost.ValidateCollaborationTarget(req.GetTarget())
	if err != nil {
		return nil, err
	}
	filename := strings.TrimSpace(req.GetFilename())
	if filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	content := req.GetContent()
	if len(content) == 0 {
		return nil, fmt.Errorf("content is required")
	}
	if len(content) > maxDaemonAttachmentBytes {
		return nil, fmt.Errorf("attachment exceeds %d bytes", maxDaemonAttachmentBytes)
	}
	record := &daemonv1.AttachmentRecord{
		AttachmentId:    "attachment-" + uuid.NewString(),
		Target:          target,
		OwnerId:         strings.TrimSpace(req.GetOwnerId()),
		Filename:        filename,
		MimeType:        strings.TrimSpace(req.GetMimeType()),
		SizeBytes:       int64(len(content)),
		StorageRef:      "kv:" + daemonAttachmentKey,
		CreatedTimeUnix: time.Now().Unix(),
	}
	if err := s.storeAttachment(ctx, record, content); err != nil {
		return nil, err
	}
	return &daemonv1.UploadAttachmentResponse{Attachment: record}, nil
}

func (s *Server) GetAttachment(ctx context.Context, req *daemonv1.GetAttachmentRequest) (*daemonv1.GetAttachmentResponse, error) {
	attachmentID := strings.TrimSpace(req.GetAttachmentId())
	if attachmentID == "" {
		return nil, fmt.Errorf("attachment_id is required")
	}
	record, content, err := s.loadAttachment(ctx, attachmentID)
	if err != nil {
		return nil, err
	}
	return &daemonv1.GetAttachmentResponse{Attachment: record, Content: content}, nil
}

func (s *Server) ListEventsSince(ctx context.Context, req *daemonv1.ListEventsSinceRequest) (*daemonv1.ListEventsSinceResponse, error) {
	limit := normalizedCollaborationLimit(req.GetLimit(), 200)
	cursor := req.GetCursor()
	if s != nil && s.eventMgr != nil {
		target := ""
		opaque := ""
		if cursor != nil {
			target = strings.TrimSpace(cursor.GetTarget())
			opaque = strings.TrimSpace(cursor.GetCursor())
		}
		records, nextCursor, err := s.eventMgr.ListSince(ctx, opaque, eventlog.ListFilter{Target: target}, limit)
		if err != nil {
			return nil, err
		}
		events := make([]*daemonv1.CollaborationEvent, 0, len(records))
		for i := range records {
			events = append(events, eventRecordToProto(&records[i]))
		}
		next := &daemonv1.EventCursor{Cursor: nextCursor, Target: target}
		if len(events) > 0 {
			last := events[len(events)-1]
			next.LastEventId = last.GetEventId()
			next.LastMessageId = last.GetMessageId()
			next.LastActivityId = last.GetActivityId()
		} else if cursor != nil {
			next.LastEventId = cursor.GetLastEventId()
			next.LastMessageId = cursor.GetLastMessageId()
			next.LastActivityId = cursor.GetLastActivityId()
		}
		return &daemonv1.ListEventsSinceResponse{Events: events, NextCursor: next}, nil
	}
	afterActivityID := ""
	target := ""
	if cursor != nil {
		afterActivityID = strings.TrimSpace(cursor.GetLastActivityId())
		target = strings.TrimSpace(cursor.GetTarget())
	}
	activities := s.activityLog(ctx)
	events := make([]*daemonv1.CollaborationEvent, 0, len(activities))
	seenCursor := afterActivityID == ""
	for _, activity := range activities {
		if activity == nil {
			continue
		}
		if !seenCursor {
			if activity.GetActivityId() == afterActivityID {
				seenCursor = true
			}
			continue
		}
		if target != "" && activity.GetTarget() != target {
			continue
		}
		if len(events) >= limit {
			break
		}
		events = append(events, activityToEvent(activity))
	}
	if afterActivityID != "" && !seenCursor {
		return nil, fmt.Errorf("event cursor expired or unknown: %s", afterActivityID)
	}
	next := &daemonv1.EventCursor{}
	if cursor != nil {
		next.Target = cursor.GetTarget()
	}
	if len(events) > 0 {
		last := events[len(events)-1]
		next.LastEventId = last.GetEventId()
		next.LastActivityId = last.GetActivityId()
		next.Cursor = last.GetEventId()
	} else if cursor != nil {
		next.Cursor = cursor.GetCursor()
		next.LastEventId = cursor.GetLastEventId()
		next.LastActivityId = cursor.GetLastActivityId()
	}
	return &daemonv1.ListEventsSinceResponse{Events: events, NextCursor: next}, nil
}

func (s *Server) listChannelAccountRecords(ctx context.Context) []*daemonv1.ChannelRecord {
	if s == nil || s.channels == nil {
		return nil
	}
	registered := s.channels.ListChannels()
	items := make([]*daemonv1.ChannelRecord, 0, len(registered))
	for _, ch := range registered {
		if ch == nil {
			continue
		}
		channelType := ch.ID()
		if typed, ok := ch.(channels.TypedChannel); ok {
			channelType = typed.ChannelType()
		}
		items = append(items, &daemonv1.ChannelRecord{
			Target:      "#" + strings.TrimPrefix(ch.ID(), "#"),
			ChannelId:   ch.ID(),
			DisplayName: ch.Name(),
			ChannelType: channelType,
			Enabled:     ch.IsEnabled(),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Target < items[j].Target
	})
	return items
}

func (s *Server) agentProfiles(ctx context.Context, limit int) ([]*daemonv1.AgentProfile, error) {
	runtimes, err := s.runtimeDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	envByRuntime := s.agentEnv(ctx)
	allSkills := s.skillRecords(nil)
	out := make([]*daemonv1.AgentProfile, 0, len(runtimes))
	for _, item := range runtimes {
		skillFilter := map[string]struct{}{}
		for _, id := range item.Skills {
			skillFilter[id] = struct{}{}
		}
		out = append(out, &daemonv1.AgentProfile{
			AgentId:     item.ID,
			Name:        item.Name,
			DisplayName: item.DisplayName,
			Description: item.Description,
			Enabled:     item.Enabled,
			Provider:    item.Provider,
			Model:       item.Model,
			Env:         profileEnvVars(envByRuntime[item.ID]),
			Skills:      s.skillRecords(skillFilter),
			DmTargets:   []string{agentDMTarget(item.ID)},
		})
		if len(out) >= limit {
			return out, nil
		}
	}
	if len(out) == 0 {
		out = append(out, &daemonv1.AgentProfile{
			AgentId:     "default",
			Name:        "default",
			DisplayName: "Default Agent",
			Enabled:     true,
			Env:         profileEnvVars(envByRuntime["default"]),
			Skills:      allSkills,
			DmTargets:   []string{agentDMTarget("default")},
		})
	}
	return out, nil
}

func (s *Server) findAgentProfile(ctx context.Context, agentID string) (*daemonv1.AgentProfile, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	profiles, err := s.agentProfiles(ctx, 1000)
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		if profile.GetAgentId() == agentID || profile.GetName() == agentID {
			return profile, nil
		}
	}
	return nil, fmt.Errorf("agent profile not found: %s", agentID)
}

func (s *Server) runtimeDefinitions(ctx context.Context) ([]runtimeagents.AgentRuntime, error) {
	if s == nil || s.runtimeMgr == nil {
		return nil, nil
	}
	items, err := s.runtimeMgr.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (s *Server) skillRecords(filter map[string]struct{}) []*daemonv1.SkillRecord {
	if s == nil || s.skillsMgr == nil {
		return nil
	}
	eligible := map[string]struct{}{}
	for _, skill := range s.skillsMgr.ListEligibleEnabled() {
		if skill != nil {
			eligible[skill.ID] = struct{}{}
		}
	}
	all := s.skillsMgr.List()
	records := make([]*daemonv1.SkillRecord, 0, len(all))
	for _, skill := range all {
		if skill == nil {
			continue
		}
		if len(filter) > 0 {
			if _, ok := filter[skill.ID]; !ok {
				continue
			}
		}
		_, isEligible := eligible[skill.ID]
		records = append(records, skillToProto(skill, isEligible))
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Id < records[j].Id
	})
	return records
}

func skillToProto(skill *skills.Skill, eligible bool) *daemonv1.SkillRecord {
	if skill == nil {
		return nil
	}
	return &daemonv1.SkillRecord{
		Id:          skill.ID,
		Name:        skill.Name,
		Description: skill.Description,
		Version:     skill.Version,
		Enabled:     skill.Enabled,
		Always:      skill.Always,
		Eligible:    eligible,
		FilePath:    skill.FilePath,
	}
}

func agentDMTarget(runtimeID string) string {
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		runtimeID = "default"
	}
	return "dm:@" + runtimeID
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeEnvVars(items []*daemonv1.EnvVar) []*daemonv1.EnvVar {
	out := make([]*daemonv1.EnvVar, 0, len(items))
	seen := map[string]int{}
	for _, item := range items {
		if item == nil {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" || strings.ContainsAny(name, "\x00\r\n=") {
			continue
		}
		env := &daemonv1.EnvVar{Name: name, Value: item.Value, Secret: item.Secret}
		if idx, ok := seen[name]; ok {
			out[idx] = env
			continue
		}
		seen[name] = len(out)
		out = append(out, env)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func cloneEnvVars(items []*daemonv1.EnvVar) []*daemonv1.EnvVar {
	out := make([]*daemonv1.EnvVar, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, &daemonv1.EnvVar{Name: item.Name, Value: item.Value, Secret: item.Secret})
	}
	return out
}

func profileEnvVars(items []*daemonv1.EnvVar) []*daemonv1.EnvVar {
	out := make([]*daemonv1.EnvVar, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		value := item.Value
		if item.Secret {
			value = "********"
		}
		out = append(out, &daemonv1.EnvVar{Name: item.Name, Value: value, Secret: item.Secret})
	}
	return out
}

func (s *Server) agentEnv(ctx context.Context) map[string][]*daemonv1.EnvVar {
	result := map[string][]*daemonv1.EnvVar{}
	if s == nil || s.kvStore == nil {
		return result
	}
	value, ok, err := s.kvStore.Get(ctx, daemonAgentEnvStoreKey)
	if err != nil || !ok {
		return result
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return result
	}
	_ = json.Unmarshal(payload, &result)
	if result == nil {
		return map[string][]*daemonv1.EnvVar{}
	}
	return result
}

func (s *Server) storeAgentEnv(ctx context.Context, runtimeID string, env []*daemonv1.EnvVar) error {
	if s == nil || s.kvStore == nil {
		return nil
	}
	return s.kvStore.UpdateFunc(ctx, daemonAgentEnvStoreKey, func(current interface{}) interface{} {
		result := map[string][]*daemonv1.EnvVar{}
		payload, err := json.Marshal(current)
		if err == nil {
			_ = json.Unmarshal(payload, &result)
		}
		if result == nil {
			result = map[string][]*daemonv1.EnvVar{}
		}
		result[runtimeID] = cloneEnvVars(env)
		return result
	})
}

func reminderToProto(job *cron.Job, target string) *daemonv1.ReminderRecord {
	if job == nil {
		return nil
	}
	return &daemonv1.ReminderRecord{
		ReminderId:   job.ID,
		Target:       target,
		ScheduleKind: string(job.ScheduleKind),
		Schedule:     reminderSchedule(job),
		Prompt:       job.Prompt,
		Enabled:      job.Enabled,
		NextRunUnix:  unixOrZero(job.NextRun),
		LastRunUnix:  unixOrZero(job.LastRun),
		RunCount:     uint32(job.RunCount),
		LastError:    job.LastError,
	}
}

func reminderSchedule(job *cron.Job) string {
	switch job.ScheduleKind {
	case cron.ScheduleAt:
		if job.AtTime == nil {
			return ""
		}
		return job.AtTime.Format(time.RFC3339)
	case cron.ScheduleEvery:
		return job.EveryDuration
	default:
		return job.Schedule
	}
}

func unixOrZero(ts time.Time) int64 {
	if ts.IsZero() {
		return 0
	}
	return ts.Unix()
}

func (s *Server) reminderTargets(ctx context.Context) map[string]string {
	result := map[string]string{}
	if s == nil || s.kvStore == nil {
		return result
	}
	value, ok, err := s.kvStore.Get(ctx, daemonReminderMetaKey)
	if err != nil || !ok {
		return result
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return result
	}
	_ = json.Unmarshal(payload, &result)
	if result == nil {
		return map[string]string{}
	}
	return result
}

func (s *Server) storeReminderTarget(ctx context.Context, reminderID, target string) error {
	if s == nil || s.kvStore == nil {
		return nil
	}
	return s.kvStore.UpdateFunc(ctx, daemonReminderMetaKey, func(current interface{}) interface{} {
		result := map[string]string{}
		payload, err := json.Marshal(current)
		if err == nil {
			_ = json.Unmarshal(payload, &result)
		}
		if result == nil {
			result = map[string]string{}
		}
		result[reminderID] = target
		return result
	})
}

func (s *Server) deleteReminderTarget(ctx context.Context, reminderID string) error {
	if s == nil || s.kvStore == nil {
		return nil
	}
	return s.kvStore.UpdateFunc(ctx, daemonReminderMetaKey, func(current interface{}) interface{} {
		result := map[string]string{}
		payload, err := json.Marshal(current)
		if err == nil {
			_ = json.Unmarshal(payload, &result)
		}
		if result == nil {
			result = map[string]string{}
		}
		delete(result, reminderID)
		return result
	})
}

func (s *Server) appendActivity(ctx context.Context, activity *daemonv1.ActivityRecord) error {
	if s == nil || s.kvStore == nil || activity == nil {
		return nil
	}
	return s.kvStore.UpdateFunc(ctx, daemonActivityStoreKey, func(current interface{}) interface{} {
		activities := []*daemonv1.ActivityRecord{}
		payload, err := json.Marshal(current)
		if err == nil {
			_ = json.Unmarshal(payload, &activities)
		}
		if activities == nil {
			activities = []*daemonv1.ActivityRecord{}
		}
		activities = append(activities, activity)
		if len(activities) > 1000 {
			activities = activities[len(activities)-1000:]
		}
		return activities
	})
}

func (s *Server) activityLog(ctx context.Context) []*daemonv1.ActivityRecord {
	if s == nil || s.kvStore == nil {
		return nil
	}
	value, ok, err := s.kvStore.Get(ctx, daemonActivityStoreKey)
	if err != nil || !ok {
		return nil
	}
	activities := []*daemonv1.ActivityRecord{}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	_ = json.Unmarshal(payload, &activities)
	if activities == nil {
		return nil
	}
	return activities
}

func activityToEvent(activity *daemonv1.ActivityRecord) *daemonv1.CollaborationEvent {
	if activity == nil {
		return nil
	}
	eventID := "event-" + activity.GetActivityId()
	return &daemonv1.CollaborationEvent{
		EventId:         eventID,
		Target:          activity.GetTarget(),
		Kind:            "activity." + activity.GetKind(),
		ActivityId:      activity.GetActivityId(),
		RunId:           activity.GetRunId(),
		CreatedTimeUnix: activity.GetCreatedTimeUnix(),
	}
}

func (s *Server) appendCollaborationEvent(ctx context.Context, item eventlog.EventRecord) (*eventlog.EventRecord, error) {
	if s == nil || s.eventMgr == nil {
		return nil, nil
	}
	rec, err := s.eventMgr.Append(ctx, item)
	if err != nil {
		return nil, fmt.Errorf("append collaboration event: %w", err)
	}
	return rec, nil
}

func eventRecordToProto(record *eventlog.EventRecord) *daemonv1.CollaborationEvent {
	if record == nil {
		return nil
	}
	out := &daemonv1.CollaborationEvent{
		EventId:         record.EventID,
		Target:          record.Target,
		Kind:            record.EventType,
		CreatedTimeUnix: record.CreatedAt.Unix(),
	}
	switch record.SubjectKind {
	case "message":
		out.MessageId = record.SubjectID
	case "activity":
		out.ActivityId = record.SubjectID
	case "task":
		out.TaskId = record.SubjectID
	case "run":
		out.RunId = record.SubjectID
	case "run_step":
		out.RunId = record.ParentSubjectID
	}
	if out.RunId == "" && record.ParentSubjectKind == "run" {
		out.RunId = record.ParentSubjectID
	}
	return out
}

func (s *Server) storeAttachment(ctx context.Context, record *daemonv1.AttachmentRecord, content []byte) error {
	if s == nil || s.kvStore == nil || record == nil {
		return fmt.Errorf("attachment store unavailable")
	}
	return s.kvStore.UpdateFunc(ctx, daemonAttachmentKey, func(current interface{}) interface{} {
		attachments := map[string]storedAttachment{}
		payload, err := json.Marshal(current)
		if err == nil {
			_ = json.Unmarshal(payload, &attachments)
		}
		if attachments == nil {
			attachments = map[string]storedAttachment{}
		}
		attachments[record.GetAttachmentId()] = storedAttachment{
			Record:        record,
			ContentBase64: base64.StdEncoding.EncodeToString(content),
		}
		return attachments
	})
}

func (s *Server) loadAttachment(ctx context.Context, attachmentID string) (*daemonv1.AttachmentRecord, []byte, error) {
	if s == nil || s.kvStore == nil {
		return nil, nil, fmt.Errorf("attachment store unavailable")
	}
	value, ok, err := s.kvStore.Get(ctx, daemonAttachmentKey)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, fmt.Errorf("attachment not found: %s", attachmentID)
	}
	attachments := map[string]storedAttachment{}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(payload, &attachments); err != nil {
		return nil, nil, err
	}
	item, ok := attachments[attachmentID]
	if !ok || item.Record == nil {
		return nil, nil, fmt.Errorf("attachment not found: %s", attachmentID)
	}
	content, err := base64.StdEncoding.DecodeString(item.ContentBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("decode attachment %s: %w", attachmentID, err)
	}
	return item.Record, content, nil
}

func (s *Server) threadRecord(ctx context.Context, sessionID string, followed map[string]struct{}) (*daemonv1.ThreadRecord, error) {
	sess, err := s.sessionMgr.GetExisting(sessionID)
	if err != nil {
		return nil, err
	}
	messages := sess.GetMessages()
	target := threadTarget(sessionID)
	_, isFollowed := followed[target]
	return &daemonv1.ThreadRecord{
		Target:          target,
		ThreadId:        sessionID,
		Summary:         sess.GetSummary(),
		RuntimeId:       "",
		MessageCount:    uint32(len(messages)),
		CreatedTimeUnix: sess.GetCreatedAt().Unix(),
		UpdatedTimeUnix: sess.GetUpdatedAt().Unix(),
		Followed:        isFollowed,
	}, nil
}

func (s *Server) followedThreadSet(ctx context.Context) map[string]struct{} {
	result := map[string]struct{}{}
	if s == nil || s.kvStore == nil {
		return result
	}
	value, ok, err := s.kvStore.Get(ctx, daemonFollowStoreKey)
	if err != nil || !ok {
		return result
	}
	raw, ok := value.(map[string]interface{})
	if !ok {
		return result
	}
	for target, enabled := range raw {
		if b, _ := enabled.(bool); b {
			result[strings.TrimSpace(target)] = struct{}{}
		}
	}
	return result
}

func (s *Server) updateFollowedThread(ctx context.Context, target string, followed bool) error {
	if s == nil || s.kvStore == nil {
		return nil
	}
	return s.kvStore.UpdateFunc(ctx, daemonFollowStoreKey, func(current interface{}) interface{} {
		raw, _ := current.(map[string]interface{})
		next := map[string]interface{}{}
		for key, value := range raw {
			next[key] = value
		}
		if followed {
			next[target] = true
		} else {
			delete(next, target)
		}
		return next
	})
}

func collaborationSessionID(target string) (string, error) {
	target, err := daemonhost.ValidateCollaborationTarget(target)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(target, "dm:@") {
		return "dm_" + strings.TrimPrefix(target, "dm:"), nil
	}
	if strings.Contains(target, ":") {
		_, threadID, ok := strings.Cut(target, ":")
		if !ok || strings.TrimSpace(threadID) == "" {
			return "", fmt.Errorf("thread target must be #channel:thread")
		}
		return strings.TrimSpace(threadID), nil
	}
	if strings.HasPrefix(target, "#") {
		return strings.TrimPrefix(target, "#"), nil
	}
	return target, nil
}

func threadTarget(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	if strings.HasPrefix(sessionID, "dm_@") {
		return "dm:" + strings.TrimPrefix(sessionID, "dm_")
	}
	return "#websocket:" + sessionID
}

func targetChannelID(target string) string {
	if strings.HasPrefix(strings.TrimSpace(target), "dm:@") {
		return "dm"
	}
	target = strings.TrimSpace(strings.TrimPrefix(target, "#"))
	if channel, _, ok := strings.Cut(target, ":"); ok {
		return channel
	}
	return target
}

func normalizedCollaborationLimit(limit uint32, fallback int) int {
	if limit == 0 {
		return fallback
	}
	if limit > 1000 {
		return 1000
	}
	return int(limit)
}

func normalizeMessageRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system", "user", "assistant", "tool":
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return "assistant"
	}
}

func sessionMessageToProto(target, sessionID string, index int, msg session.Message, updatedAt time.Time) *daemonv1.CollaborationMessage {
	return &daemonv1.CollaborationMessage{
		MessageId:       fmt.Sprintf("%s:%d", sessionID, index),
		Target:          target,
		ThreadId:        sessionID,
		Role:            msg.Role,
		Content:         msg.Content,
		CreatedTimeUnix: updatedAt.Unix(),
	}
}

func metadataString(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func metadataInt64(values map[string]any, key string) int64 {
	if len(values) == 0 {
		return 0
	}
	switch v := values[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	}
	return 0
}

func metadataStringSlice(values map[string]any, key string) []string {
	if len(values) == 0 {
		return nil
	}
	raw, ok := values[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Idempotency helpers
// ---------------------------------------------------------------------------

// idempotentRequestHash computes a deterministic hash from alternating key-value pairs.
func idempotentRequestHash(pairs ...string) string {
	// Build a canonical JSON object from key-value pairs.
	m := make(map[string]string, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	b, _ := json.Marshal(m)
	return idempotency.Hash(b)
}

// failIdempotency marks a pending record as failed after a mutation error.
// Errors from Fail itself are swallowed because the original mutation error is more important.
func failIdempotency(ctx context.Context, store *idempotency.Store, key idempotency.Key, mutationErr error) {
	if store == nil {
		return
	}
	_, _ = store.Fail(ctx, key, idempotency.FailRequest{
		ErrorCode:    "MUTATION_FAILED",
		ErrorMessage: mutationErr.Error(),
	})
}

// completeIdempotency marks a pending record as succeeded.
// Returns an error so callers can log it, but callers should still return
// the successful mutation result to the client (the mutation already happened).
func completeIdempotency(ctx context.Context, store *idempotency.Store, key idempotency.Key, req idempotency.CompleteRequest) error {
	if store == nil {
		return nil
	}
	_, err := store.Complete(ctx, key, req)
	return err
}

func mustMarshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func replaySendMessage(rec *idempotency.Record) (*daemonv1.SendMessageResponse, error) {
	if rec != nil && rec.Status == idempotency.StatusFailed {
		return nil, fmt.Errorf("previous request failed: %s", rec.ErrorMessage)
	}
	if rec != nil && rec.ResponseJSON != "" {
		var resp daemonv1.SendMessageResponse
		if json.Unmarshal([]byte(rec.ResponseJSON), &resp) == nil {
			return &resp, nil
		}
	}
	return nil, fmt.Errorf("idempotency record missing response data")
}

func replayCreateTask(rec *idempotency.Record) (*daemonv1.CreateCollaborationTaskResponse, error) {
	if rec != nil && rec.Status == idempotency.StatusFailed {
		return nil, fmt.Errorf("previous request failed: %s", rec.ErrorMessage)
	}
	if rec != nil && rec.ResponseJSON != "" {
		var resp daemonv1.CreateCollaborationTaskResponse
		if json.Unmarshal([]byte(rec.ResponseJSON), &resp) == nil {
			return &resp, nil
		}
	}
	return nil, fmt.Errorf("idempotency record missing response data")
}

func replayClaimTask(rec *idempotency.Record) (*daemonv1.ClaimCollaborationTaskResponse, error) {
	if rec != nil && rec.Status == idempotency.StatusFailed {
		return nil, fmt.Errorf("previous request failed: %s", rec.ErrorMessage)
	}
	if rec != nil && rec.ResponseJSON != "" {
		var resp daemonv1.ClaimCollaborationTaskResponse
		if json.Unmarshal([]byte(rec.ResponseJSON), &resp) == nil {
			return &resp, nil
		}
	}
	return nil, fmt.Errorf("idempotency record missing response data")
}

func replayLogActivity(rec *idempotency.Record) (*daemonv1.LogActivityResponse, error) {
	if rec != nil && rec.Status == idempotency.StatusFailed {
		return nil, fmt.Errorf("previous request failed: %s", rec.ErrorMessage)
	}
	if rec != nil && rec.ResponseJSON != "" {
		var resp daemonv1.LogActivityResponse
		if json.Unmarshal([]byte(rec.ResponseJSON), &resp) == nil {
			return &resp, nil
		}
	}
	return nil, fmt.Errorf("idempotency record missing response data")
}

func replayCreateTaskGraph(rec *idempotency.Record) (*daemonv1.CreateTaskGraphResponse, error) {
	if rec != nil && rec.Status == idempotency.StatusFailed {
		return nil, fmt.Errorf("previous request failed: %s", rec.ErrorMessage)
	}
	if rec != nil && rec.ResponseJSON != "" {
		var resp daemonv1.CreateTaskGraphResponse
		if json.Unmarshal([]byte(rec.ResponseJSON), &resp) == nil {
			return &resp, nil
		}
	}
	return nil, fmt.Errorf("idempotency record missing response data")
}

func replayApplyTaskSplit(rec *idempotency.Record) (*daemonv1.ApplyTaskSplitResponse, error) {
	if rec != nil && rec.Status == idempotency.StatusFailed {
		return nil, fmt.Errorf("previous request failed: %s", rec.ErrorMessage)
	}
	if rec != nil && rec.ResponseJSON != "" {
		var resp daemonv1.ApplyTaskSplitResponse
		if json.Unmarshal([]byte(rec.ResponseJSON), &resp) == nil {
			return &resp, nil
		}
	}
	return nil, fmt.Errorf("idempotency record missing response data")
}

func protoHashString(msg *daemonv1.Task) string {
	b, _ := json.Marshal(msg)
	return idempotency.Hash(b)
}

func extractProtoIDs(tasks []*daemonv1.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.GetTaskId()
	}
	return ids
}

// protoHashStringSlice computes a deterministic hash over a slice of Task protos.
func protoHashStringSlice(tasks []*daemonv1.Task) string {
	b, _ := json.Marshal(tasks)
	return idempotency.Hash(b)
}

// protoHashEdgeSlice computes a deterministic hash over a slice of TaskEdge protos.
func protoHashEdgeSlice(edges []*daemonv1.TaskEdge) string {
	b, _ := json.Marshal(edges)
	return idempotency.Hash(b)
}

// parseSubtaskIndex parses a string subtask index (e.g. "0", "1") and bounds-checks it.
func parseSubtaskIndex(s string, count int) (int, error) {
	var idx int
	_, err := fmt.Sscanf(s, "%d", &idx)
	if err != nil {
		return 0, fmt.Errorf("invalid subtask index: %s", s)
	}
	if idx < 0 || idx >= count {
		return 0, fmt.Errorf("subtask index out of range: %d (count=%d)", idx, count)
	}
	return idx, nil
}

// toAnySlice converts a string slice to []any for metadata storage.
func toAnySlice(ss []string) []any {
	result := make([]any, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}
