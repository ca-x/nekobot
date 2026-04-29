package daemonhost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
)

type Client struct {
	baseURL    string
	token      string
	http       *http.Client
	grpcConn   *grpc.ClientConn
	grpcClient daemonv1.DaemonControlServiceClient
}

func NewClient(baseURL string) *Client { return NewAuthedClient(baseURL, "") }
func NewAuthedClient(baseURL, token string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://" + DefaultAddr
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: strings.TrimSpace(token), http: &http.Client{Timeout: 5 * time.Second}}
}
func NewGRPCClient(target, token string) (*Client, error) {
	if strings.TrimSpace(target) == "" {
		target = DefaultAddr
	}
	conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial grpc target %s: %w", target, err)
	}
	return &Client{token: strings.TrimSpace(token), http: &http.Client{Timeout: 5 * time.Second}, grpcConn: conn, grpcClient: daemonv1.NewDaemonControlServiceClient(conn)}, nil
}
func (c *Client) Close() error {
	if c == nil || c.grpcConn == nil {
		return nil
	}
	return c.grpcConn.Close()
}
func (c *Client) GetInfo() (*daemonv1.DaemonInfo, error) {
	var info daemonv1.DaemonInfo
	if err := c.getProto("/v1/info", &info); err != nil {
		return nil, err
	}
	return &info, nil
}
func (c *Client) GetInventory() (*daemonv1.RuntimeInventory, error) {
	var inventory daemonv1.RuntimeInventory
	if err := c.getProto("/v1/runtimes", &inventory); err != nil {
		return nil, err
	}
	return &inventory, nil
}
func (c *Client) Register(req *daemonv1.RegisterMachineRequest) (*daemonv1.RegisterMachineResponse, error) {
	var resp daemonv1.RegisterMachineResponse
	if err := c.postProto("/v1/register", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
func (c *Client) RegisterRemote(req *daemonv1.RegisterMachineRequest) (*daemonv1.RegisterMachineResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.RegisterMachine(ctx, req)
	}
	var resp daemonv1.RegisterMachineResponse
	if err := c.postProto("/api/daemon/register", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
func (c *Client) Heartbeat(req *daemonv1.HeartbeatMachineRequest) (*daemonv1.HeartbeatMachineResponse, error) {
	var resp daemonv1.HeartbeatMachineResponse
	if err := c.postProto("/v1/heartbeat", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
func (c *Client) HeartbeatRemote(req *daemonv1.HeartbeatMachineRequest) (*daemonv1.HeartbeatMachineResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.HeartbeatMachine(ctx, req)
	}
	var resp daemonv1.HeartbeatMachineResponse
	if err := c.postProto("/api/daemon/heartbeat", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
func (c *Client) FetchAssignedTasksRemote(req *daemonv1.FetchAssignedTasksRequest) (*daemonv1.FetchAssignedTasksResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.FetchAssignedTasks(ctx, req)
	}
	var resp daemonv1.FetchAssignedTasksResponse
	if err := c.postProto("/api/daemon/tasks/fetch", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
func (c *Client) UpdateTaskStatusRemote(req *daemonv1.UpdateTaskStatusRequest) (*daemonv1.UpdateTaskStatusResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.UpdateTaskStatus(ctx, req)
	}
	var resp daemonv1.UpdateTaskStatusResponse
	if err := c.postProto("/api/daemon/tasks/update", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
func (c *Client) ListWorkspaceTree(req *daemonv1.ListWorkspaceTreeRequest) (*daemonv1.ListWorkspaceTreeResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ListWorkspaceTree(ctx, req)
	}
	var resp daemonv1.ListWorkspaceTreeResponse
	if err := c.postProto("/v1/workspace/tree", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
func (c *Client) ReadWorkspaceFile(req *daemonv1.ReadWorkspaceFileRequest) (*daemonv1.ReadWorkspaceFileResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ReadWorkspaceFile(ctx, req)
	}
	var resp daemonv1.ReadWorkspaceFileResponse
	if err := c.postProto("/v1/workspace/file", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
func (c *Client) ListChannelsRemote(req *daemonv1.ListChannelsRequest) (*daemonv1.ListChannelsResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ListChannels(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) ListThreadsRemote(req *daemonv1.ListThreadsRequest) (*daemonv1.ListThreadsResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ListThreads(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) GetThreadRemote(req *daemonv1.GetThreadRequest) (*daemonv1.GetThreadResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.GetThread(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) ReadMessagesRemote(req *daemonv1.ReadMessagesRequest) (*daemonv1.ReadMessagesResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ReadMessages(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) SendMessageRemote(req *daemonv1.SendMessageRequest) (*daemonv1.SendMessageResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.SendMessage(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) FollowThreadRemote(req *daemonv1.FollowThreadRequest) (*daemonv1.FollowThreadResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.FollowThread(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) UnfollowThreadRemote(req *daemonv1.UnfollowThreadRequest) (*daemonv1.UnfollowThreadResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.UnfollowThread(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) CreateCollaborationTaskRemote(req *daemonv1.CreateCollaborationTaskRequest) (*daemonv1.CreateCollaborationTaskResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.CreateCollaborationTask(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) ListCollaborationTasksRemote(req *daemonv1.ListCollaborationTasksRequest) (*daemonv1.ListCollaborationTasksResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ListCollaborationTasks(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) ClaimCollaborationTaskRemote(req *daemonv1.ClaimCollaborationTaskRequest) (*daemonv1.ClaimCollaborationTaskResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ClaimCollaborationTask(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) GetServerInfoRemote(req *daemonv1.ServerInfoRequest) (*daemonv1.ServerInfoResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.GetServerInfo(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) GetAgentProfileRemote(req *daemonv1.GetAgentProfileRequest) (*daemonv1.GetAgentProfileResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.GetAgentProfile(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) SetAgentEnvRemote(req *daemonv1.SetAgentEnvRequest) (*daemonv1.SetAgentEnvResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.SetAgentEnv(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) ListAgentProfilesRemote(req *daemonv1.ListAgentProfilesRequest) (*daemonv1.ListAgentProfilesResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ListAgentProfiles(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) ListAgentDMsRemote(req *daemonv1.ListAgentDMsRequest) (*daemonv1.ListAgentDMsResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ListAgentDMs(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) ScheduleReminderRemote(req *daemonv1.ScheduleReminderRequest) (*daemonv1.ScheduleReminderResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ScheduleReminder(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) ListRemindersRemote(req *daemonv1.ListRemindersRequest) (*daemonv1.ListRemindersResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ListReminders(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) CancelReminderRemote(req *daemonv1.CancelReminderRequest) (*daemonv1.CancelReminderResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.CancelReminder(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) LogActivityRemote(req *daemonv1.LogActivityRequest) (*daemonv1.LogActivityResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.LogActivity(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) ListActivityRemote(req *daemonv1.ListActivityRequest) (*daemonv1.ListActivityResponse, error) {
	if c.grpcClient != nil {
		ctx, cancel := c.rpcContext()
		defer cancel()
		return c.grpcClient.ListActivity(ctx, req)
	}
	return nil, fmt.Errorf("collaboration RPCs require grpc transport")
}
func (c *Client) rpcContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if token := strings.TrimSpace(c.token); token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	return ctx, cancel
}
func (c *Client) getProto(path string, target proto.Message) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	c.applyAuth(req)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon host returned %d: %s", res.StatusCode, string(body))
	}
	return protojson.Unmarshal(body, target)
}
func (c *Client) postProto(path string, req proto.Message, target proto.Message) error {
	payload, err := protojson.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.applyAuth(httpReq)
	res, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon host returned %d: %s", res.StatusCode, string(body))
	}
	return protojson.Unmarshal(body, target)
}
func (c *Client) RawJSON(path string) (map[string]interface{}, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.applyAuth(req)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon host returned %d: %s", res.StatusCode, string(body))
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
func (c *Client) applyAuth(req *http.Request) {
	if c == nil || req == nil {
		return
	}
	if token := strings.TrimSpace(c.token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
