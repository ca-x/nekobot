package daemonhost

import (
	"context"
	"fmt"
	"strings"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/tasks"
)

type GRPCService struct {
	daemonv1.UnimplementedDaemonControlServiceServer
	Registry        *Registry
	TaskService     *tasks.Service
	InventoryLoader func() (*daemonv1.RuntimeInventory, error)
	AppendSession   func(context.Context, tasks.Task, *daemonv1.UpdateTaskStatusRequest) error
}

func NewGRPCService(registry *Registry, taskService *tasks.Service, inventoryLoader func() (*daemonv1.RuntimeInventory, error), appendSession func(context.Context, tasks.Task, *daemonv1.UpdateTaskStatusRequest) error) *GRPCService {
	return &GRPCService{Registry: registry, TaskService: taskService, InventoryLoader: inventoryLoader, AppendSession: appendSession}
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
func (s *GRPCService) loadInventory() (*daemonv1.RuntimeInventory, error) {
	if s == nil || s.InventoryLoader == nil {
		return nil, fmt.Errorf("inventory loader unavailable")
	}
	return s.InventoryLoader()
}
