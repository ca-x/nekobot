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

	register, err := registry.Register(t.Context(), &daemonv1.RegisterComputerRequest{
		Info:      &daemonv1.ComputerInfo{DaemonId: "daemon-a", ComputerId: "machine-a", DisplayName: "machine-a", Status: "online"},
		Inventory: &daemonv1.ComputerInventory{},
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if register.GetLease().GetLeaseId() == "" || register.GetLease().GetResourceId() != "machine-a" {
		t.Fatalf("expected register lease for machine-a, got %+v", register.GetLease())
	}
	heartbeat, err := registry.Heartbeat(t.Context(), &daemonv1.HeartbeatComputerRequest{
		Info:      &daemonv1.ComputerInfo{DaemonId: "daemon-a", ComputerId: "machine-a", DisplayName: "machine-a", Status: "online"},
		Inventory: &daemonv1.ComputerInventory{},
		LeaseId:   register.GetLease().GetLeaseId(),
	})
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	if heartbeat.GetLease().GetLeaseId() != register.GetLease().GetLeaseId() || heartbeat.GetNextHeartbeatAfterSeconds() != HeartbeatAfterSeconds {
		t.Fatalf("expected renewed lease %q, got %+v", register.GetLease().GetLeaseId(), heartbeat.GetLease())
	}
	ss, err := registry.Snapshot(t.Context())
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(ss.Machines) != 1 || ss.Machines["machine-a"] == nil {
		t.Fatalf("expected stored machine state, got %+v", ss.Machines)
	}
	if ss.Machines["machine-a"].GetLeaseId() != register.GetLease().GetLeaseId() {
		t.Fatalf("expected persisted lease id %q, got %+v", register.GetLease().GetLeaseId(), ss.Machines["machine-a"])
	}
	if _, err := registry.Heartbeat(t.Context(), &daemonv1.HeartbeatComputerRequest{
		Info:    &daemonv1.ComputerInfo{DaemonId: "daemon-a", ComputerId: "machine-a", DisplayName: "machine-a", Status: "online"},
		LeaseId: "wrong-lease",
	}); err == nil {
		t.Fatalf("expected heartbeat with mismatched lease to fail")
	}
}

func TestRegistryPreservesRuntimeCapabilityManifest(t *testing.T) {
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
		Info: &daemonv1.ComputerInfo{
			DaemonId:   "daemon-a",
			ComputerId: "machine-a",
			Status:     "online",
			Capabilities: []*daemonv1.Capability{
				{Name: "daemon.heartbeat", Enabled: true},
			},
		},
		Inventory: &daemonv1.ComputerInventory{
			Workspaces: []*daemonv1.Workspace{{WorkspaceId: "machine-a:default", ComputerId: "machine-a", IsDefault: true}},
			Runtimes: []*daemonv1.Runtime{{
				RuntimeId:   "machine-a:default:codex",
				ComputerId:  "machine-a",
				WorkspaceId: "machine-a:default",
				Kind:        "codex",
				Capabilities: []*daemonv1.Capability{
					{Name: "runtime.launch", Enabled: true},
					{Name: "agent.control.restart_full_reset", Enabled: false},
				},
				RuntimeProfile: &daemonv1.RuntimeProfile{
					RuntimeProfileId: "machine-a:default:codex",
					Kind:             "codex",
					WorkspaceId:      "machine-a:default",
					Capabilities: []*daemonv1.Capability{
						{Name: "runtime.launch", Enabled: true},
					},
				},
			}},
			RuntimeProfiles: []*daemonv1.RuntimeProfile{{
				RuntimeProfileId: "machine-a:default:codex",
				Kind:             "codex",
				WorkspaceId:      "machine-a:default",
				Capabilities: []*daemonv1.Capability{
					{Name: "runtime.launch", Enabled: true},
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	ss, err := registry.Snapshot(t.Context())
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if !protoCapabilityEnabled(ss.Machines["machine-a"].GetCapabilities(), "daemon.heartbeat") {
		t.Fatalf("expected machine capability to round-trip, got %+v", ss.Machines["machine-a"].GetCapabilities())
	}
	inv := ss.Inventories["machine-a"]
	if inv == nil || len(inv.GetRuntimes()) != 1 {
		t.Fatalf("expected stored inventory runtime, got %+v", inv)
	}
	runtimeItem := inv.GetRuntimes()[0]
	if !protoCapabilityEnabled(runtimeItem.GetCapabilities(), "runtime.launch") {
		t.Fatalf("expected runtime.launch capability to round-trip, got %+v", runtimeItem.GetCapabilities())
	}
	if protoCapabilityEnabled(runtimeItem.GetCapabilities(), "agent.control.restart_full_reset") {
		t.Fatalf("expected full reset capability to remain disabled, got %+v", runtimeItem.GetCapabilities())
	}
	if runtimeItem.GetRuntimeProfile().GetRuntimeProfileId() != "machine-a:default:codex" {
		t.Fatalf("expected embedded runtime profile to round-trip, got %+v", runtimeItem.GetRuntimeProfile())
	}
	if len(inv.GetRuntimeProfiles()) != 1 || !protoCapabilityEnabled(inv.GetRuntimeProfiles()[0].GetCapabilities(), "runtime.launch") {
		t.Fatalf("expected runtime_profiles manifest to round-trip, got %+v", inv.GetRuntimeProfiles())
	}
}

func TestBuildInventoryAdvertisesAdapterCapabilityManifest(t *testing.T) {
	inventory, err := BuildInventory(t.TempDir())
	if err != nil {
		t.Fatalf("build inventory: %v", err)
	}
	if len(inventory.GetRuntimes()) == 0 {
		t.Fatalf("expected adapter runtimes in inventory")
	}
	for _, runtimeItem := range inventory.GetRuntimes() {
		if runtimeItem.GetRuntimeProfile() == nil {
			t.Fatalf("runtime %s missing embedded runtime profile", runtimeItem.GetRuntimeId())
		}
		if !protoCapabilityEnabled(runtimeItem.GetCapabilities(), "runtime.launch") {
			t.Fatalf("runtime %s missing runtime.launch capability: %+v", runtimeItem.GetRuntimeId(), runtimeItem.GetCapabilities())
		}
		if !protoCapabilityEnabled(runtimeItem.GetCapabilities(), "agent.control.terminate") {
			t.Fatalf("runtime %s missing terminate capability: %+v", runtimeItem.GetRuntimeId(), runtimeItem.GetCapabilities())
		}
		if protoCapabilityEnabled(runtimeItem.GetCapabilities(), "agent.control.restart_full_reset") {
			t.Fatalf("runtime %s must not advertise full reset as enabled until implemented", runtimeItem.GetRuntimeId())
		}
	}
	if len(inventory.GetRuntimeProfiles()) != len(inventory.GetRuntimes()) {
		t.Fatalf("expected runtime profile per runtime, got profiles=%d runtimes=%d", len(inventory.GetRuntimeProfiles()), len(inventory.GetRuntimes()))
	}
}

func protoCapabilityEnabled(items []*daemonv1.Capability, name string) bool {
	for _, item := range items {
		if item.GetName() == name {
			return item.GetEnabled()
		}
	}
	return false
}

func TestDeriveMachineStatusMarksStaleHeartbeat(t *testing.T) {
	info := &daemonv1.ComputerInfo{
		ComputerId:   "machine-a",
		Status:       "online",
		LastSeenUnix: time.Now().Add(-(StaleThreshold + time.Second)).Unix(),
	}
	if got := DeriveMachineStatus(info, time.Now()); got != "stale" {
		t.Fatalf("expected stale, got %q", got)
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
