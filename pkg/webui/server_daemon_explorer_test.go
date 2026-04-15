package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/config"
	"nekobot/pkg/daemonhost"
	"nekobot/pkg/state"

	"github.com/labstack/echo/v5"
)

func TestHandleDaemonExplorerTreeProxiesToDaemon(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	log := newTestLogger(t)
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: filepath.Join(t.TempDir(), "daemon-state.json")})
	if err != nil {
		t.Fatalf("new daemon state store: %v", err)
	}
	defer func() { _ = store.Close() }()

	daemonSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspace/tree" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workspace_id": "machine-a:default",
			"path":         "",
			"entries":      []map[string]any{{"path": "README.md", "name": "README.md", "is_dir": false, "size_bytes": 12, "modified_time_unix": 1}},
		})
	}))
	defer daemonSrv.Close()

	registry := daemonhost.NewRegistry(store)
	_, err = registry.Register(t.Context(), &daemonv1.RegisterMachineRequest{Info: &daemonv1.DaemonInfo{DaemonId: "daemon-a", MachineId: "machine-a", MachineName: "machine-a", Status: "online", DaemonUrl: daemonSrv.URL}, Inventory: &daemonv1.RuntimeInventory{Workspaces: []*daemonv1.Workspace{{WorkspaceId: "machine-a:default", MachineId: "machine-a", Path: "/tmp/workspace", DisplayName: "default", IsDefault: true}}}})
	if err != nil {
		t.Fatalf("register daemon machine: %v", err)
	}

	s := &Server{config: cfg, kvStore: store}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/daemon/explorer/tree", strings.NewReader(`{"machine_id":"machine-a","workspace_id":"machine-a:default","path":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := s.handleDaemonExplorerTree(ctx); err != nil {
		t.Fatalf("handleDaemonExplorerTree failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDaemonExplorerTreeRejectsMissingDaemonURL(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	log := newTestLogger(t)
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: filepath.Join(t.TempDir(), "daemon-state.json")})
	if err != nil {
		t.Fatalf("new daemon state store: %v", err)
	}
	defer func() { _ = store.Close() }()

	registry := daemonhost.NewRegistry(store)
	_, err = registry.Register(t.Context(), &daemonv1.RegisterMachineRequest{Info: &daemonv1.DaemonInfo{DaemonId: "daemon-a", MachineId: "machine-a", MachineName: "machine-a", Status: "online"}, Inventory: &daemonv1.RuntimeInventory{}})
	if err != nil {
		t.Fatalf("register daemon machine: %v", err)
	}

	s := &Server{config: cfg, kvStore: store}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/daemon/explorer/tree", strings.NewReader(`{"machine_id":"machine-a","workspace_id":"machine-a:default","path":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := s.handleDaemonExplorerTree(ctx); err != nil {
		t.Fatalf("handleDaemonExplorerTree failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
