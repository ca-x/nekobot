package daemonhost

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/externalagent"
	"nekobot/pkg/state"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const DefaultAddr = "127.0.0.1:7777"

func BuildInfo(machineName string) (*daemonv1.DaemonInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("resolve hostname: %w", err)
	}
	name := strings.TrimSpace(machineName)
	if name == "" {
		name = hostname
	}
	machineID := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	if machineID == "" {
		machineID = hostname
	}
	return &daemonv1.DaemonInfo{
		DaemonId:     hostname,
		MachineId:    machineID,
		MachineName:  name,
		Hostname:     hostname,
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Version:      "v1alpha1",
		Status:       "online",
		LastSeenUnix: time.Now().Unix(),
	}, nil
}

func BuildInventory(homeDir string) (*daemonv1.RuntimeInventory, error) {
	if strings.TrimSpace(homeDir) == "" {
		resolved, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		homeDir = resolved
	}
	info, err := BuildInfo("")
	if err != nil {
		return nil, err
	}
	installed, err := externalagent.DetectInstalled(homeDir)
	if err != nil {
		return nil, err
	}
	installedMap := map[string]string{}
	for _, item := range installed {
		installedMap[item.Kind] = item.ConfigDir
	}
	workspaceRoot := filepath.Join(homeDir, "code")
	workspace := &daemonv1.Workspace{
		WorkspaceId: info.MachineId + ":default",
		MachineId:   info.MachineId,
		Path:        workspaceRoot,
		DisplayName: "default",
		Aliases:     []string{"default"},
		IsDefault:   true,
	}
	registry := externalagent.NewRegistry()
	result := &daemonv1.RuntimeInventory{Workspaces: []*daemonv1.Workspace{workspace}}
	for _, adapter := range registry.List() {
		runtimeItem := &daemonv1.Runtime{
			RuntimeId:           workspace.WorkspaceId + ":" + adapter.Kind(),
			MachineId:           info.MachineId,
			WorkspaceId:         workspace.WorkspaceId,
			Kind:                adapter.Kind(),
			DisplayName:         strings.Title(adapter.Kind()),
			Aliases:             []string{adapter.Kind()},
			Tool:                adapter.Tool(),
			Command:             adapter.Command(),
			Installed:           false,
			Healthy:             false,
			SupportsAutoInstall: adapter.SupportsAutoInstall(),
		}
		if adapter.SupportsAutoInstall() {
			runtimeItem.InstallHint = append([]string(nil), adapter.InstallCommand(runtime.GOOS)...)
		}
		if configDir, ok := installedMap[adapter.Kind()]; ok {
			runtimeItem.Installed = true
			runtimeItem.Healthy = true
			runtimeItem.ConfigDir = configDir
		}
		result.Runtimes = append(result.Runtimes, runtimeItem)
	}
	return result, nil
}

type Server struct {
	addr        string
	machineName string
	registry    *Registry
	httpServer  *http.Server
}

func NewServer(addr, machineName string, kv state.KV) *Server {
	if strings.TrimSpace(addr) == "" {
		addr = DefaultAddr
	}
	return &Server{addr: addr, machineName: machineName, registry: NewRegistry(kv)}
}

func (s *Server) Serve(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/info", s.handleInfo)
	mux.HandleFunc("/v1/runtimes", s.handleRuntimes)
	mux.HandleFunc("/v1/register", s.handleRegister)
	mux.HandleFunc("/v1/heartbeat", s.handleHeartbeat)
	mux.HandleFunc("/v1/registry", s.handleRegistry)
	s.httpServer = &http.Server{Addr: s.addr, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
	}()
	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	info, err := BuildInfo(s.machineName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeProtoJSON(w, info)
}

func (s *Server) handleRuntimes(w http.ResponseWriter, r *http.Request) {
	inventory, err := BuildInventory("")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeProtoJSON(w, inventory)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		http.Error(w, "daemon registry unavailable", http.StatusServiceUnavailable)
		return
	}
	var req daemonv1.RegisterMachineRequest
	if err := DecodeProtoJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.registry.Register(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeProtoJSON(w, resp)
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		http.Error(w, "daemon registry unavailable", http.StatusServiceUnavailable)
		return
	}
	var req daemonv1.HeartbeatMachineRequest
	if err := DecodeProtoJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.registry.Heartbeat(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeProtoJSON(w, resp)
}

func (s *Server) handleRegistry(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		http.Error(w, "daemon registry unavailable", http.StatusServiceUnavailable)
		return
	}
	snapshot, err := s.registry.Snapshot(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	data, _ := protojson.MarshalOptions{Multiline: true}.Marshal(snapshotToMessage(snapshot))
	_, _ = w.Write(data)
}

func writeProtoJSON(w http.ResponseWriter, msg proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	data, _ := protojson.Marshal(msg)
	_, _ = w.Write(data)
}

func DecodeProtoJSON(r *http.Request, target proto.Message) error {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(body, target)
}

func snapshotToMessage(snapshot *Snapshot) *daemonv1.RuntimeInventory {
	result := &daemonv1.RuntimeInventory{}
	for _, inventory := range snapshot.Inventories {
		if inventory == nil {
			continue
		}
		result.Workspaces = append(result.Workspaces, inventory.Workspaces...)
		result.Runtimes = append(result.Runtimes, inventory.Runtimes...)
	}
	return result
}
