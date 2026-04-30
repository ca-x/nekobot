package daemonhost

import (
	"path/filepath"
	"testing"
	"time"

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

	_, err = registry.Register(t.Context(), &daemonv1.RegisterComputerRequest{
		Info:      &daemonv1.ComputerInfo{DaemonId: "daemon-a", ComputerId: "machine-a", DisplayName: "machine-a", Status: "online"},
		Inventory: &daemonv1.ComputerInventory{},
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	_, err = registry.Heartbeat(t.Context(), &daemonv1.HeartbeatComputerRequest{
		Info:      &daemonv1.ComputerInfo{DaemonId: "daemon-a", ComputerId: "machine-a", DisplayName: "machine-a", Status: "online"},
		Inventory: &daemonv1.ComputerInventory{},
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

func TestDeriveMachineStatusMarksOldHeartbeatOffline(t *testing.T) {
	info := &daemonv1.ComputerInfo{
		ComputerId:   "machine-a",
		Status:       "online",
		LastSeenUnix: time.Now().Add(-(OfflineThreshold + time.Second)).Unix(),
	}
	if got := DeriveMachineStatus(info, time.Now()); got != "offline" {
		t.Fatalf("expected offline, got %q", got)
	}
}

func TestDeriveMachineStatusKeepsRecentHeartbeatOnline(t *testing.T) {
	info := &daemonv1.ComputerInfo{
		ComputerId:   "machine-a",
		Status:       "online",
		LastSeenUnix: time.Now().Unix(),
	}
	if got := DeriveMachineStatus(info, time.Now()); got != "online" {
		t.Fatalf("expected online, got %q", got)
	}
}
