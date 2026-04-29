package daemonhost

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/tasks"
)

type GRPCService struct {
	daemonv1.UnimplementedDaemonControlServiceServer
	Registry        *Registry
	TaskService     *tasks.Service
	InventoryLoader func() (*daemonv1.RuntimeInventory, error)
	AppendSession   func(context.Context, tasks.Task, *daemonv1.UpdateTaskStatusRequest) error
	Collaboration   CollaborationService
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
	GetServerInfo(context.Context, *daemonv1.ServerInfoRequest) (*daemonv1.ServerInfoResponse, error)
	GetAgentProfile(context.Context, *daemonv1.GetAgentProfileRequest) (*daemonv1.GetAgentProfileResponse, error)
	SetAgentEnv(context.Context, *daemonv1.SetAgentEnvRequest) (*daemonv1.SetAgentEnvResponse, error)
	ListAgentProfiles(context.Context, *daemonv1.ListAgentProfilesRequest) (*daemonv1.ListAgentProfilesResponse, error)
	ListAgentDMs(context.Context, *daemonv1.ListAgentDMsRequest) (*daemonv1.ListAgentDMsResponse, error)
	ScheduleReminder(context.Context, *daemonv1.ScheduleReminderRequest) (*daemonv1.ScheduleReminderResponse, error)
	ListReminders(context.Context, *daemonv1.ListRemindersRequest) (*daemonv1.ListRemindersResponse, error)
	CancelReminder(context.Context, *daemonv1.CancelReminderRequest) (*daemonv1.CancelReminderResponse, error)
	LogActivity(context.Context, *daemonv1.LogActivityRequest) (*daemonv1.LogActivityResponse, error)
	ListActivity(context.Context, *daemonv1.ListActivityRequest) (*daemonv1.ListActivityResponse, error)
}

func NewGRPCService(registry *Registry, taskService *tasks.Service, inventoryLoader func() (*daemonv1.RuntimeInventory, error), appendSession func(context.Context, tasks.Task, *daemonv1.UpdateTaskStatusRequest) error, collaboration ...CollaborationService) *GRPCService {
	svc := &GRPCService{Registry: registry, TaskService: taskService, InventoryLoader: inventoryLoader, AppendSession: appendSession}
	if len(collaboration) > 0 {
		svc.Collaboration = collaboration[0]
	}
	return svc
}

func (s *GRPCService) RegisterMachine(ctx context.Context, req *daemonv1.RegisterMachineRequest) (*daemonv1.RegisterMachineResponse, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("daemon registry unavailable")
	}
	return s.Registry.Register(ctx, req)
}
func (s *GRPCService) HeartbeatMachine(ctx context.Context, req *daemonv1.HeartbeatMachineRequest) (*daemonv1.HeartbeatMachineResponse, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("daemon registry unavailable")
	}
	return s.Registry.Heartbeat(ctx, req)
}
func (s *GRPCService) FetchAssignedTasks(ctx context.Context, req *daemonv1.FetchAssignedTasksRequest) (*daemonv1.FetchAssignedTasksResponse, error) {
	if s == nil || s.TaskService == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	return BuildAssignedTasks(s.TaskService, strings.TrimSpace(req.GetMachineId()), req.GetRuntimeIds(), int(req.GetLimit())), nil
}
func (s *GRPCService) UpdateTaskStatus(ctx context.Context, req *daemonv1.UpdateTaskStatusRequest) (*daemonv1.UpdateTaskStatusResponse, error) {
	if s == nil || s.TaskService == nil {
		return nil, fmt.Errorf("task service unavailable")
	}
	resp, task, err := ApplyTaskStatusUpdate(s.TaskService, req)
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

func (s *GRPCService) GetServerInfo(ctx context.Context, req *daemonv1.ServerInfoRequest) (*daemonv1.ServerInfoResponse, error) {
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

func (s *GRPCService) loadInventory() (*daemonv1.RuntimeInventory, error) {
	if s == nil || s.InventoryLoader == nil {
		return nil, status.Error(codes.Unimplemented, "workspace RPCs are not implemented on this daemon control surface")
	}
	return s.InventoryLoader()
}
