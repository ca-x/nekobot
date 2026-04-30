package daemonhost

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/idempotency"
	"nekobot/pkg/runs"
	"nekobot/pkg/tasks"
)

type GRPCService struct {
	daemonv1.UnimplementedDaemonControlServiceServer
	Registry         *Registry
	TaskService      *tasks.Service
	RunManager       *runs.Manager
	IdempotencyStore *idempotency.Store
	InventoryLoader  func() (*daemonv1.ComputerInventory, error)
	AppendSession    func(context.Context, tasks.Task, *daemonv1.UpdateRunStatusRequest) error
	Collaboration    CollaborationService
}

type CollaborationService interface {
	ListChannels(context.Context, *daemonv1.ListChannelsRequest) (*daemonv1.ListChannelsResponse, error)
	ListThreads(context.Context, *daemonv1.ListThreadsRequest) (*daemonv1.ListThreadsResponse, error)
	GetThread(context.Context, *daemonv1.GetThreadRequest) (*daemonv1.GetThreadResponse, error)
	ReadMessages(context.Context, *daemonv1.ReadMessagesRequest) (*daemonv1.ReadMessagesResponse, error)
	SendMessage(context.Context, *daemonv1.SendMessageRequest) (*daemonv1.SendMessageResponse, error)
	FollowThread(context.Context, *daemonv1.FollowThreadRequest) (*daemonv1.FollowThreadResponse, error)
	UnfollowThread(context.Context, *daemonv1.UnfollowThreadRequest) (*daemonv1.UnfollowThreadResponse, error)
	CreateCollaborationTask(context.Context, *daemonv1.CreateCollaborationTaskRequest) (*daemonv1.CreateCollaborationTaskResponse, error)
	ListCollaborationTasks(context.Context, *daemonv1.ListCollaborationTasksRequest) (*daemonv1.ListCollaborationTasksResponse, error)
	ClaimCollaborationTask(context.Context, *daemonv1.ClaimCollaborationTaskRequest) (*daemonv1.ClaimCollaborationTaskResponse, error)
	ProposeTaskSplit(context.Context, *daemonv1.ProposeTaskSplitRequest) (*daemonv1.ProposeTaskSplitResponse, error)
	ApplyTaskSplit(context.Context, *daemonv1.ApplyTaskSplitRequest) (*daemonv1.ApplyTaskSplitResponse, error)
	CreateTaskGraph(context.Context, *daemonv1.CreateTaskGraphRequest) (*daemonv1.CreateTaskGraphResponse, error)
	ListTaskGraph(context.Context, *daemonv1.ListTaskGraphRequest) (*daemonv1.ListTaskGraphResponse, error)
	UpdateTaskGraph(context.Context, *daemonv1.UpdateTaskGraphRequest) (*daemonv1.UpdateTaskGraphResponse, error)
	GetServerInfo(context.Context, *daemonv1.GetServerInfoRequest) (*daemonv1.GetServerInfoResponse, error)
	GetAgentProfile(context.Context, *daemonv1.GetAgentProfileRequest) (*daemonv1.GetAgentProfileResponse, error)
	SetAgentEnv(context.Context, *daemonv1.SetAgentEnvRequest) (*daemonv1.SetAgentEnvResponse, error)
	ListAgentProfiles(context.Context, *daemonv1.ListAgentProfilesRequest) (*daemonv1.ListAgentProfilesResponse, error)
	ListAgentDMs(context.Context, *daemonv1.ListAgentDMsRequest) (*daemonv1.ListAgentDMsResponse, error)
	ControlAgent(context.Context, *daemonv1.ControlAgentRequest) (*daemonv1.ControlAgentResponse, error)
	SendAgentDirectMessage(context.Context, *daemonv1.SendAgentDirectMessageRequest) (*daemonv1.SendAgentDirectMessageResponse, error)
	ScheduleReminder(context.Context, *daemonv1.ScheduleReminderRequest) (*daemonv1.ScheduleReminderResponse, error)
	ListReminders(context.Context, *daemonv1.ListRemindersRequest) (*daemonv1.ListRemindersResponse, error)
	CancelReminder(context.Context, *daemonv1.CancelReminderRequest) (*daemonv1.CancelReminderResponse, error)
	LogActivity(context.Context, *daemonv1.LogActivityRequest) (*daemonv1.LogActivityResponse, error)
	ListActivity(context.Context, *daemonv1.ListActivityRequest) (*daemonv1.ListActivityResponse, error)
	UploadAttachment(context.Context, *daemonv1.UploadAttachmentRequest) (*daemonv1.UploadAttachmentResponse, error)
	GetAttachment(context.Context, *daemonv1.GetAttachmentRequest) (*daemonv1.GetAttachmentResponse, error)
	ListEventsSince(context.Context, *daemonv1.ListEventsSinceRequest) (*daemonv1.ListEventsSinceResponse, error)
}

func NewGRPCService(registry *Registry, taskService *tasks.Service, inventoryLoader func() (*daemonv1.ComputerInventory, error), appendSession func(context.Context, tasks.Task, *daemonv1.UpdateRunStatusRequest) error, collaboration ...CollaborationService) *GRPCService {
	svc := &GRPCService{Registry: registry, TaskService: taskService, InventoryLoader: inventoryLoader, AppendSession: appendSession}
	if len(collaboration) > 0 {
		svc.Collaboration = collaboration[0]
	}
	return svc
}

// WithRunManager injects a run/step manager for AppendRunStep/ListRuns/GetRun.
func (s *GRPCService) WithRunManager(mgr *runs.Manager) *GRPCService {
	s.RunManager = mgr
	return s
}

// WithIdempotencyStore injects an idempotency store for mutating RPCs.
func (s *GRPCService) WithIdempotencyStore(store *idempotency.Store) *GRPCService {
	s.IdempotencyStore = store
	return s
}

func (s *GRPCService) RegisterComputer(ctx context.Context, req *daemonv1.RegisterComputerRequest) (*daemonv1.RegisterComputerResponse, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("daemon registry unavailable")
	}
	return s.Registry.Register(ctx, req)
}
func (s *GRPCService) HeartbeatComputer(ctx context.Context, req *daemonv1.HeartbeatComputerRequest) (*daemonv1.HeartbeatComputerResponse, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("daemon registry unavailable")
	}
	return s.Registry.Heartbeat(ctx, req)
}
func (s *GRPCService) FetchAssignedRuns(ctx context.Context, req *daemonv1.FetchAssignedRunsRequest) (*daemonv1.FetchAssignedRunsResponse, error) {
	if s == nil || s.TaskService == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	return BuildAssignedRuns(s.TaskService, strings.TrimSpace(req.GetComputerId()), req.GetAgentIds(), int(req.GetLimit())), nil
}
func (s *GRPCService) UpdateRunStatus(ctx context.Context, req *daemonv1.UpdateRunStatusRequest) (*daemonv1.UpdateRunStatusResponse, error) {
	if s == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	if s.RunManager != nil && strings.TrimSpace(req.GetRunId()) != "" {
		record, err := s.RunManager.UpdateRunStatus(ctx, req.GetRunId(), req.GetState(), req.GetError(), req.GetSummary(), req.GetState())
		if err != nil {
			return nil, err
		}
		if record != nil {
			return &daemonv1.UpdateRunStatusResponse{Accepted: true}, nil
		}
	}
	if s.TaskService == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	resp, task, err := ApplyRunStatusUpdate(s.TaskService, req)
	if err != nil {
		return nil, err
	}
	if s.AppendSession != nil {
		if err := s.AppendSession(ctx, task, req); err != nil {
			return nil, err
		}
	}
	return resp, nil
}
func (s *GRPCService) ListWorkspaceTree(ctx context.Context, req *daemonv1.ListWorkspaceTreeRequest) (*daemonv1.ListWorkspaceTreeResponse, error) {
	inventory, err := s.loadInventory()
	if err != nil {
		return nil, err
	}
	return ListWorkspaceTree(inventory, req)
}

func (s *GRPCService) ReadWorkspaceFile(ctx context.Context, req *daemonv1.ReadWorkspaceFileRequest) (*daemonv1.ReadWorkspaceFileResponse, error) {
	inventory, err := s.loadInventory()
	if err != nil {
		return nil, err
	}
	return ReadWorkspaceFile(inventory, req)
}

func (s *GRPCService) ListChannels(ctx context.Context, req *daemonv1.ListChannelsRequest) (*daemonv1.ListChannelsResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ListChannels(ctx, req)
}

func (s *GRPCService) ListThreads(ctx context.Context, req *daemonv1.ListThreadsRequest) (*daemonv1.ListThreadsResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ListThreads(ctx, req)
}

func (s *GRPCService) GetThread(ctx context.Context, req *daemonv1.GetThreadRequest) (*daemonv1.GetThreadResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.GetThread(ctx, req)
}

func (s *GRPCService) ReadMessages(ctx context.Context, req *daemonv1.ReadMessagesRequest) (*daemonv1.ReadMessagesResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ReadMessages(ctx, req)
}

func (s *GRPCService) SendMessage(ctx context.Context, req *daemonv1.SendMessageRequest) (*daemonv1.SendMessageResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.SendMessage(ctx, req)
}

func (s *GRPCService) FollowThread(ctx context.Context, req *daemonv1.FollowThreadRequest) (*daemonv1.FollowThreadResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.FollowThread(ctx, req)
}

func (s *GRPCService) UnfollowThread(ctx context.Context, req *daemonv1.UnfollowThreadRequest) (*daemonv1.UnfollowThreadResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.UnfollowThread(ctx, req)
}

func (s *GRPCService) CreateCollaborationTask(ctx context.Context, req *daemonv1.CreateCollaborationTaskRequest) (*daemonv1.CreateCollaborationTaskResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.CreateCollaborationTask(ctx, req)
}

func (s *GRPCService) ListCollaborationTasks(ctx context.Context, req *daemonv1.ListCollaborationTasksRequest) (*daemonv1.ListCollaborationTasksResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ListCollaborationTasks(ctx, req)
}

func (s *GRPCService) ClaimCollaborationTask(ctx context.Context, req *daemonv1.ClaimCollaborationTaskRequest) (*daemonv1.ClaimCollaborationTaskResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ClaimCollaborationTask(ctx, req)
}

func (s *GRPCService) ProposeTaskSplit(ctx context.Context, req *daemonv1.ProposeTaskSplitRequest) (*daemonv1.ProposeTaskSplitResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ProposeTaskSplit(ctx, req)
}

func (s *GRPCService) ApplyTaskSplit(ctx context.Context, req *daemonv1.ApplyTaskSplitRequest) (*daemonv1.ApplyTaskSplitResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ApplyTaskSplit(ctx, req)
}

func (s *GRPCService) CreateTaskGraph(ctx context.Context, req *daemonv1.CreateTaskGraphRequest) (*daemonv1.CreateTaskGraphResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.CreateTaskGraph(ctx, req)
}

func (s *GRPCService) ListTaskGraph(ctx context.Context, req *daemonv1.ListTaskGraphRequest) (*daemonv1.ListTaskGraphResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ListTaskGraph(ctx, req)
}

func (s *GRPCService) UpdateTaskGraph(ctx context.Context, req *daemonv1.UpdateTaskGraphRequest) (*daemonv1.UpdateTaskGraphResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.UpdateTaskGraph(ctx, req)
}

func (s *GRPCService) GetServerInfo(ctx context.Context, req *daemonv1.GetServerInfoRequest) (*daemonv1.GetServerInfoResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.GetServerInfo(ctx, req)
}

func (s *GRPCService) GetAgentProfile(ctx context.Context, req *daemonv1.GetAgentProfileRequest) (*daemonv1.GetAgentProfileResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.GetAgentProfile(ctx, req)
}

func (s *GRPCService) SetAgentEnv(ctx context.Context, req *daemonv1.SetAgentEnvRequest) (*daemonv1.SetAgentEnvResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.SetAgentEnv(ctx, req)
}

func (s *GRPCService) ListAgentProfiles(ctx context.Context, req *daemonv1.ListAgentProfilesRequest) (*daemonv1.ListAgentProfilesResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ListAgentProfiles(ctx, req)
}

func (s *GRPCService) ListAgentDMs(ctx context.Context, req *daemonv1.ListAgentDMsRequest) (*daemonv1.ListAgentDMsResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ListAgentDMs(ctx, req)
}

func (s *GRPCService) ControlAgent(ctx context.Context, req *daemonv1.ControlAgentRequest) (*daemonv1.ControlAgentResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ControlAgent(ctx, req)
}

func (s *GRPCService) SendAgentDirectMessage(ctx context.Context, req *daemonv1.SendAgentDirectMessageRequest) (*daemonv1.SendAgentDirectMessageResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.SendAgentDirectMessage(ctx, req)
}

func (s *GRPCService) ScheduleReminder(ctx context.Context, req *daemonv1.ScheduleReminderRequest) (*daemonv1.ScheduleReminderResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ScheduleReminder(ctx, req)
}

func (s *GRPCService) ListReminders(ctx context.Context, req *daemonv1.ListRemindersRequest) (*daemonv1.ListRemindersResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ListReminders(ctx, req)
}

func (s *GRPCService) CancelReminder(ctx context.Context, req *daemonv1.CancelReminderRequest) (*daemonv1.CancelReminderResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.CancelReminder(ctx, req)
}

func (s *GRPCService) LogActivity(ctx context.Context, req *daemonv1.LogActivityRequest) (*daemonv1.LogActivityResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.LogActivity(ctx, req)
}

func (s *GRPCService) ListActivity(ctx context.Context, req *daemonv1.ListActivityRequest) (*daemonv1.ListActivityResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ListActivity(ctx, req)
}

func (s *GRPCService) UploadAttachment(ctx context.Context, req *daemonv1.UploadAttachmentRequest) (*daemonv1.UploadAttachmentResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.UploadAttachment(ctx, req)
}

func (s *GRPCService) GetAttachment(ctx context.Context, req *daemonv1.GetAttachmentRequest) (*daemonv1.GetAttachmentResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.GetAttachment(ctx, req)
}

func (s *GRPCService) ListEventsSince(ctx context.Context, req *daemonv1.ListEventsSinceRequest) (*daemonv1.ListEventsSinceResponse, error) {
	if s == nil || s.Collaboration == nil {
		return nil, status.Error(codes.Unimplemented, "collaboration RPCs are not implemented on this daemon control surface")
	}
	return s.Collaboration.ListEventsSince(ctx, req)
}

// ---------------------------------------------------------------------------
// Run/RunStep RPCs
// ---------------------------------------------------------------------------

func (s *GRPCService) AppendRunStep(ctx context.Context, req *daemonv1.AppendRunStepRequest) (*daemonv1.AppendRunStepResponse, error) {
	if s == nil || s.RunManager == nil {
		return nil, status.Error(codes.FailedPrecondition, "run manager not available")
	}
	step := req.GetStep()
	if step == nil {
		return nil, status.Error(codes.InvalidArgument, "step is required")
	}

	// Idempotency guard.
	reqID := strings.TrimSpace(req.GetRequestId())
	idemKey := idempotency.Key{
		CallerKind: "agent",
		CallerID:   step.GetRunId(), // run_id serves as the caller scope
		Method:     "AppendRunStep",
		RequestID:  reqID,
	}
	if reqID != "" && s.IdempotencyStore != nil {
		hash, _ := idempotency.HashJSON(map[string]any{
			"run_id":       step.GetRunId(),
			"step_id":      step.GetStepId(),
			"sequence":     step.GetSequence(),
			"kind":         step.GetKind(),
			"status":       step.GetStatus(),
			"summary":      step.GetSummary(),
			"detail":       step.GetDetail(),
			"artifact_ids": step.GetArtifactIds(),
		})
		result, err := s.IdempotencyStore.Reserve(ctx, idemKey, hash, 30*24*time.Hour)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "idempotency reserve: %v", err)
		}
		switch result.Outcome {
		case idempotency.OutcomeReplay:
			return replayAppendRunStep(result.Record)
		case idempotency.OutcomeConflict:
			return nil, status.Errorf(codes.AlreadyExists, "idempotency conflict: request_id %s already used with different body", reqID)
		case idempotency.OutcomeInProgress:
			return nil, status.Errorf(codes.Unavailable, "request %s is already being processed", reqID)
		}
	}

	record, err := s.RunManager.AppendRunStep(ctx, runs.StepRecord{
		ID:          step.GetStepId(),
		RunID:       step.GetRunId(),
		Sequence:    step.GetSequence(),
		Kind:        step.GetKind(),
		Status:      step.GetStatus(),
		Summary:     step.GetSummary(),
		Detail:      step.GetDetail(),
		ArtifactIDs: step.GetArtifactIds(),
		RequestID:   reqID,
	})
	if err != nil {
		if reqID != "" && s.IdempotencyStore != nil {
			_, _ = s.IdempotencyStore.Fail(ctx, idemKey, idempotency.FailRequest{
				ErrorCode:    "MUTATION_FAILED",
				ErrorMessage: err.Error(),
			})
		}
		return nil, status.Errorf(codes.Internal, "append run step: %v", err)
	}
	resp := &daemonv1.AppendRunStepResponse{
		Accepted: true,
		Step:     stepRecordToProto(record),
	}
	if reqID != "" && s.IdempotencyStore != nil {
		_, _ = s.IdempotencyStore.Complete(ctx, idemKey, idempotency.CompleteRequest{
			ResponseType: "json:AppendRunStepResponse",
			ResponseJSON: marshalAppendRunStepResponse(resp),
			ResourceKind: "run_step",
			ResourceID:   record.ID,
		})
	}
	return resp, nil
}

func (s *GRPCService) ListRuns(ctx context.Context, req *daemonv1.ListRunsRequest) (*daemonv1.ListRunsResponse, error) {
	if s == nil || s.RunManager == nil {
		return nil, status.Error(codes.FailedPrecondition, "run manager not available")
	}
	records, err := s.RunManager.ListRuns(ctx,
		req.GetTarget(),
		req.GetTaskId(),
		req.GetAgentId(),
		int(req.GetLimit()),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list runs: %v", err)
	}
	out := make([]*daemonv1.Run, 0, len(records))
	for i := range records {
		out = append(out, runRecordToProto(&records[i]))
	}
	return &daemonv1.ListRunsResponse{Runs: out}, nil
}

func (s *GRPCService) GetRun(ctx context.Context, req *daemonv1.GetRunRequest) (*daemonv1.GetRunResponse, error) {
	if s == nil || s.RunManager == nil {
		return nil, status.Error(codes.FailedPrecondition, "run manager not available")
	}
	runID := strings.TrimSpace(req.GetRunId())
	if runID == "" {
		return nil, status.Error(codes.InvalidArgument, "run_id is required")
	}
	rec, err := s.RunManager.GetRun(ctx, runID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get run: %v", err)
	}
	if rec == nil {
		return nil, status.Error(codes.NotFound, "run not found")
	}
	steps, err := s.RunManager.ListRunSteps(ctx, runID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list run steps: %v", err)
	}
	protoSteps := make([]*daemonv1.RunStep, 0, len(steps))
	for i := range steps {
		protoSteps = append(protoSteps, stepRecordToProto(&steps[i]))
	}
	return &daemonv1.GetRunResponse{
		Run:   runRecordToProto(rec),
		Steps: protoSteps,
	}, nil
}

// ---------------------------------------------------------------------------
// Proto ↔ domain conversions
// ---------------------------------------------------------------------------

func runRecordToProto(rec *runs.RunRecord) *daemonv1.Run {
	if rec == nil {
		return nil
	}
	r := &daemonv1.Run{
		RunId:            rec.ID,
		TaskId:           rec.TaskID,
		Target:           rec.Target,
		AgentId:          rec.AgentID,
		ComputerId:       rec.ComputerID,
		RuntimeProfileId: rec.RuntimeProfileID,
		Status:           rec.Status,
		LeaseId:          rec.LeaseID,
		RequestId:        rec.RequestID,
		InputMessageId:   rec.InputMessageID,
		LastSeenEventId:  rec.LastSeenEventID,
		StartedTimeUnix:  rec.StartedAt.Unix(),
		UpdatedTimeUnix:  rec.UpdatedAt.Unix(),
		Error:            rec.Error,
		Summary:          rec.Summary,
		State:            rec.State,
	}
	if rec.CompletedAt != nil {
		r.CompletedTimeUnix = rec.CompletedAt.Unix()
	}
	return r
}

func stepRecordToProto(rec *runs.StepRecord) *daemonv1.RunStep {
	if rec == nil {
		return nil
	}
	s := &daemonv1.RunStep{
		StepId:          rec.ID,
		RunId:           rec.RunID,
		Sequence:        rec.Sequence,
		Kind:            rec.Kind,
		Status:          rec.Status,
		Summary:         rec.Summary,
		Detail:          rec.Detail,
		ArtifactIds:     rec.ArtifactIDs,
		StartedTimeUnix: rec.StartedAt.Unix(),
		RequestId:       rec.RequestID,
	}
	if rec.CompletedAt != nil {
		s.CompletedTimeUnix = rec.CompletedAt.Unix()
	}
	return s
}

func (s *GRPCService) loadInventory() (*daemonv1.ComputerInventory, error) {
	if s == nil || s.InventoryLoader == nil {
		return nil, status.Error(codes.Unimplemented, "workspace RPCs are not implemented on this daemon control surface")
	}
	return s.InventoryLoader()
}

// ---------------------------------------------------------------------------
// Idempotency helpers
// ---------------------------------------------------------------------------

func marshalAppendRunStepResponse(resp *daemonv1.AppendRunStepResponse) string {
	b, err := json.Marshal(resp)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func replayAppendRunStep(rec *idempotency.Record) (*daemonv1.AppendRunStepResponse, error) {
	if rec != nil && rec.Status == idempotency.StatusFailed {
		return nil, status.Errorf(codes.Internal, "previous request failed: %s", rec.ErrorMessage)
	}
	if rec != nil && rec.ResponseJSON != "" {
		var resp daemonv1.AppendRunStepResponse
		if json.Unmarshal([]byte(rec.ResponseJSON), &resp) == nil {
			return &resp, nil
		}
	}
	return nil, status.Errorf(codes.Internal, "idempotency record missing response data")
}
