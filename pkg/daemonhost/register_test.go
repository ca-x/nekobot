package daemonhost

import (
	"path/filepath"
	"testing"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
)

func TestRegistryRegisterAndHeartbeatPersistMachineState(t *testing.T) {
	log, err := logger.New(&logger.Config{Level: logger.LevelError})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	store, err := state.NewFileStore(log, &state.FileStoreConfig{FilePath: filepath.Join(t.TempDir(), "daemon-state.json")})
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	defer func() { _ = store.Close() }()
	registry := NewRegistry(store)

	_, err = registry.Register(t.Context(), &daemonv1.RegisterMachineRequest{
		Info:      &daemonv1.DaemonInfo{DaemonId: "daemon-a", MachineId: "machine-a", MachineName: "machine-a", Status: "online"},
		Inventory: &daemonv1.RuntimeInventory{},
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	_, err = registry.Heartbeat(t.Context(), &daemonv1.HeartbeatMachineRequest{
		Info:      &daemonv1.DaemonInfo{DaemonId: "daemon-a", MachineId: "machine-a", MachineName: "machine-a", Status: "online"},
		Inventory: &daemonv1.RuntimeInventory{},
	})
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	ss, err := registry.Snapshot(t.Context())
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(ss.Machines) != 1 || ss.Machines["machine-a"] == nil {
		t.Fatalf("expected stored machine state, got %+v", ss.Machines)
	}
}
