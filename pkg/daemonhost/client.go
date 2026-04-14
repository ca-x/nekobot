package daemonhost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return NewAuthedClient(baseURL, "")
}

func NewAuthedClient(baseURL, token string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://" + DefaultAddr
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
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
	var resp daemonv1.HeartbeatMachineResponse
	if err := c.postProto("/api/daemon/heartbeat", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) FetchAssignedTasksRemote(req *daemonv1.FetchAssignedTasksRequest) (*daemonv1.FetchAssignedTasksResponse, error) {
	var resp daemonv1.FetchAssignedTasksResponse
	if err := c.postProto("/api/daemon/tasks/fetch", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateTaskStatusRemote(req *daemonv1.UpdateTaskStatusRequest) (*daemonv1.UpdateTaskStatusResponse, error) {
	var resp daemonv1.UpdateTaskStatusResponse
	if err := c.postProto("/api/daemon/tasks/update", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
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
	if err := protojson.Unmarshal(body, target); err != nil {
		return err
	}
	return nil
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
	if err := protojson.Unmarshal(body, target); err != nil {
		return err
	}
	return nil
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
