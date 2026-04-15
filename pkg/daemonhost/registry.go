package daemonhost

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	daemonv1 "nekobot/gen/go/nekobot/daemon/v1"
	"nekobot/pkg/state"
)

const daemonRegistryKey = "daemonhost.registry.v1"
const OfflineThreshold = 45 * time.Second

type Registry struct {
	kv state.KV
}

type Snapshot struct {
	Machines    map[string]*daemonv1.DaemonInfo       `json:"machines"`
	Inventories map[string]*daemonv1.RuntimeInventory `json:"inventories"`
}

type MachineStatus struct {
	Info                  *daemonv1.DaemonInfo `json:"info"`
	WorkspaceCount        int                  `json:"workspace_count"`
	RuntimeCount          int                  `json:"runtime_count"`
	InstalledRuntimeCount int                  `json:"installed_runtime_count"`
	HealthyRuntimeCount   int                  `json:"healthy_runtime_count"`
	GoalRunRunnable       bool                 `json:"goal_run_runnable"`
}

func NewRegistry(kv state.KV) *Registry { return &Registry{kv: kv} }

func (r *Registry) Register(ctx context.Context, req *daemonv1.RegisterMachineRequest) (*daemonv1.RegisterMachineResponse, error) {
	if r == nil || r.kv == nil {
		return nil, fmt.Errorf("daemon registry is unavailable")
	}
	if req == nil || req.Info == nil {
		return nil, fmt.Errorf("daemon info is required")
	}
	now := time.Now().Unix()
	req.Info.LastSeenUnix = now
	if strings.TrimSpace(req.Info.Status) == "" {
		req.Info.Status = "online"
	}
	if err := r.update(ctx, req.Info, req.Inventory); err != nil {
		return nil, err
	}
	return &daemonv1.RegisterMachineResponse{Accepted: true, ServerTimeUnix: now}, nil
}

func (r *Registry) Heartbeat(ctx context.Context, req *daemonv1.HeartbeatMachineRequest) (*daemonv1.HeartbeatMachineResponse, error) {
	if r == nil || r.kv == nil {
		return nil, fmt.Errorf("daemon registry is unavailable")
	}
	if req == nil || req.Info == nil {
		return nil, fmt.Errorf("daemon info is required")
	}
	now := time.Now().Unix()
	req.Info.LastSeenUnix = now
	if strings.TrimSpace(req.Info.Status) == "" {
		req.Info.Status = "online"
	}
	if err := r.update(ctx, req.Info, req.Inventory); err != nil {
		return nil, err
	}
	return &daemonv1.HeartbeatMachineResponse{Accepted: true, ServerTimeUnix: now, NextHeartbeatAfterSeconds: 15}, nil
}

func (r *Registry) Snapshot(ctx context.Context) (*Snapshot, error) {
	if r == nil || r.kv == nil {
		return &Snapshot{Machines: map[string]*daemonv1.DaemonInfo{}, Inventories: map[string]*daemonv1.RuntimeInventory{}}, nil
	}
	value, ok, err := r.kv.Get(ctx, daemonRegistryKey)
	if err != nil {
		return nil, err
	}
	if !ok || value == nil {
		return &Snapshot{Machines: map[string]*daemonv1.DaemonInfo{}, Inventories: map[string]*daemonv1.RuntimeInventory{}}, nil
	}
	raw, ok := value.(map[string]interface{})
	if !ok {
		return &Snapshot{Machines: map[string]*daemonv1.DaemonInfo{}, Inventories: map[string]*daemonv1.RuntimeInventory{}}, nil
	}
	return decodeSnapshot(raw), nil
}

func (r *Registry) update(ctx context.Context, info *daemonv1.DaemonInfo, inventory *daemonv1.RuntimeInventory) error {
	return r.kv.UpdateFunc(ctx, daemonRegistryKey, func(current interface{}) interface{} {
		snapshot := &Snapshot{Machines: map[string]*daemonv1.DaemonInfo{}, Inventories: map[string]*daemonv1.RuntimeInventory{}}
		if raw, ok := current.(map[string]interface{}); ok {
			snapshot = decodeSnapshot(raw)
		}
		machineID := strings.TrimSpace(info.MachineId)
		if machineID == "" {
			machineID = strings.TrimSpace(info.DaemonId)
		}
		copiedInfo := *info
		copiedInfo.MachineId = machineID
		snapshot.Machines[machineID] = &copiedInfo
		if inventory != nil {
			snapshot.Inventories[machineID] = inventory
		}
		return encodeSnapshot(snapshot)
	})
}

func decodeSnapshot(raw map[string]interface{}) *Snapshot {
	result := &Snapshot{Machines: map[string]*daemonv1.DaemonInfo{}, Inventories: map[string]*daemonv1.RuntimeInventory{}}
	if machinesRaw, ok := raw["machines"].(map[string]interface{}); ok {
		for key, value := range machinesRaw {
			if item, ok := value.(map[string]interface{}); ok {
				result.Machines[key] = &daemonv1.DaemonInfo{
					DaemonId:     getString(item, "daemon_id"),
					MachineId:    getString(item, "machine_id"),
					MachineName:  getString(item, "machine_name"),
					Hostname:     getString(item, "hostname"),
					Os:           getString(item, "os"),
					Arch:         getString(item, "arch"),
					Version:      getString(item, "version"),
					Status:       getString(item, "status"),
					LastSeenUnix: getInt64(item, "last_seen_unix"),
					DaemonUrl:    getString(item, "daemon_url"),
				}
			}
		}
	}
	if inventoriesRaw, ok := raw["inventories"].(map[string]interface{}); ok {
		for key, value := range inventoriesRaw {
			if item, ok := value.(map[string]interface{}); ok {
				result.Inventories[key] = decodeInventory(item)
			}
		}
	}
	return result
}

func MachineStatuses(snapshot *Snapshot) []MachineStatus {
	if snapshot == nil || len(snapshot.Machines) == 0 {
		return []MachineStatus{}
	}
	machineIDs := make([]string, 0, len(snapshot.Machines))
	for id := range snapshot.Machines {
		machineIDs = append(machineIDs, id)
	}
	sort.Strings(machineIDs)
	out := make([]MachineStatus, 0, len(machineIDs))
	now := time.Now()
	for _, id := range machineIDs {
		info := snapshot.Machines[id]
		if info == nil {
			continue
		}
		cloned := *info
		cloned.Status = DeriveMachineStatus(info, now)
		status := MachineStatus{Info: &cloned}
		if inv, ok := snapshot.Inventories[id]; ok && inv != nil {
			status.WorkspaceCount = len(inv.Workspaces)
			status.RuntimeCount = len(inv.Runtimes)
			for _, item := range inv.Runtimes {
				if item == nil {
					continue
				}
				if item.Installed {
					status.InstalledRuntimeCount++
				}
				if item.Installed && item.Healthy {
					status.HealthyRuntimeCount++
				}
			}
			_, _, status.GoalRunRunnable = SelectRunnableWorkspaceRuntime(inv)
		}
		out = append(out, status)
	}
	return out
}

// SelectRunnableWorkspaceRuntime returns the runnable runtime in the selected/default workspace.
// The chosen runtime must be installed and healthy.
func SelectRunnableWorkspaceRuntime(inventory *daemonv1.RuntimeInventory) (runtimeID, workspaceID string, ok bool) {
	if inventory == nil {
		return "", "", false
	}
	for _, ws := range inventory.Workspaces {
		if ws == nil {
			continue
		}
		if ws.IsDefault {
			workspaceID = strings.TrimSpace(ws.WorkspaceId)
			break
		}
	}
	if workspaceID == "" && len(inventory.Workspaces) > 0 && inventory.Workspaces[0] != nil {
		workspaceID = strings.TrimSpace(inventory.Workspaces[0].WorkspaceId)
	}
	if workspaceID == "" {
		return "", "", false
	}
	for _, runtimeItem := range inventory.Runtimes {
		if runtimeItem == nil {
			continue
		}
		if strings.TrimSpace(runtimeItem.WorkspaceId) != workspaceID {
			continue
		}
		if runtimeItem.Installed && runtimeItem.Healthy {
			return strings.TrimSpace(runtimeItem.RuntimeId), workspaceID, true
		}
	}
	return "", workspaceID, false
}

func DeriveMachineStatus(info *daemonv1.DaemonInfo, now time.Time) string {
	if info == nil {
		return "offline"
	}
	current := strings.TrimSpace(info.Status)
	if current == "" {
		current = "online"
	}
	if current != "online" {
		return current
	}
	lastSeenUnix := info.LastSeenUnix
	if lastSeenUnix <= 0 {
		return "offline"
	}
	lastSeenAt := time.Unix(lastSeenUnix, 0)
	if now.Sub(lastSeenAt) > OfflineThreshold {
		return "offline"
	}
	return current
}

func decodeInventory(raw map[string]interface{}) *daemonv1.RuntimeInventory {
	inventory := &daemonv1.RuntimeInventory{}
	for _, ws := range getMapSlice(raw, "workspaces") {
		inventory.Workspaces = append(inventory.Workspaces, &daemonv1.Workspace{
			WorkspaceId: getString(ws, "workspace_id"),
			MachineId:   getString(ws, "machine_id"),
			Path:        getString(ws, "path"),
			DisplayName: getString(ws, "display_name"),
			Aliases:     getStringSlice(ws, "aliases"),
			IsDefault:   getBool(ws, "is_default"),
		})
	}
	for _, rt := range getMapSlice(raw, "runtimes") {
		inventory.Runtimes = append(inventory.Runtimes, &daemonv1.Runtime{
			RuntimeId:           getString(rt, "runtime_id"),
			MachineId:           getString(rt, "machine_id"),
			WorkspaceId:         getString(rt, "workspace_id"),
			Kind:                getString(rt, "kind"),
			DisplayName:         getString(rt, "display_name"),
			Aliases:             getStringSlice(rt, "aliases"),
			Tool:                getString(rt, "tool"),
			Command:             getString(rt, "command"),
			Installed:           getBool(rt, "installed"),
			Healthy:             getBool(rt, "healthy"),
			SupportsAutoInstall: getBool(rt, "supports_auto_install"),
			InstallHint:         getStringSlice(rt, "install_hint"),
			ConfigDir:           getString(rt, "config_dir"),
			CurrentTaskCount:    uint32(getInt64(rt, "current_task_count")),
			PendingTaskCount:    uint32(getInt64(rt, "pending_task_count")),
		})
	}
	return inventory
}

func encodeSnapshot(snapshot *Snapshot) map[string]interface{} {
	result := map[string]interface{}{"machines": map[string]interface{}{}, "inventories": map[string]interface{}{}}
	machineIDs := make([]string, 0, len(snapshot.Machines))
	for id := range snapshot.Machines {
		machineIDs = append(machineIDs, id)
	}
	sort.Strings(machineIDs)
	machines := result["machines"].(map[string]interface{})
	for _, id := range machineIDs {
		item := snapshot.Machines[id]
		machines[id] = map[string]interface{}{
			"daemon_id":      item.DaemonId,
			"machine_id":     item.MachineId,
			"machine_name":   item.MachineName,
			"hostname":       item.Hostname,
			"os":             item.Os,
			"arch":           item.Arch,
			"version":        item.Version,
			"status":         item.Status,
			"last_seen_unix": item.LastSeenUnix,
			"daemon_url":     item.DaemonUrl,
		}
	}
	inventories := result["inventories"].(map[string]interface{})
	for _, id := range machineIDs {
		if inv, ok := snapshot.Inventories[id]; ok && inv != nil {
			inventories[id] = map[string]interface{}{
				"workspaces": encodeWorkspaces(inv.Workspaces),
				"runtimes":   encodeRuntimes(inv.Runtimes),
			}
		}
	}
	return result
}

func encodeWorkspaces(items []*daemonv1.Workspace) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"workspace_id": item.WorkspaceId,
			"machine_id":   item.MachineId,
			"path":         item.Path,
			"display_name": item.DisplayName,
			"aliases":      append([]string(nil), item.Aliases...),
			"is_default":   item.IsDefault,
		})
	}
	return out
}

func encodeRuntimes(items []*daemonv1.Runtime) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"runtime_id":            item.RuntimeId,
			"machine_id":            item.MachineId,
			"workspace_id":          item.WorkspaceId,
			"kind":                  item.Kind,
			"display_name":          item.DisplayName,
			"aliases":               append([]string(nil), item.Aliases...),
			"tool":                  item.Tool,
			"command":               item.Command,
			"installed":             item.Installed,
			"healthy":               item.Healthy,
			"supports_auto_install": item.SupportsAutoInstall,
			"install_hint":          append([]string(nil), item.InstallHint...),
			"config_dir":            item.ConfigDir,
			"current_task_count":    item.CurrentTaskCount,
			"pending_task_count":    item.PendingTaskCount,
		})
	}
	return out
}

func getString(m map[string]interface{}, key string) string { v, _ := m[key].(string); return v }
func getBool(m map[string]interface{}, key string) bool     { v, _ := m[key].(bool); return v }
func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	if v, ok := m[key].(int64); ok {
		return v
	}
	if v, ok := m[key].(int); ok {
		return int64(v)
	}
	return 0
}
func getStringSlice(m map[string]interface{}, key string) []string {
	if items, ok := m[key].([]interface{}); ok {
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	if items, ok := m[key].([]string); ok {
		return append([]string(nil), items...)
	}
	return nil
}

func getMapSlice(m map[string]interface{}, key string) []map[string]interface{} {
	if items, ok := m[key].([]map[string]interface{}); ok {
		return append([]map[string]interface{}(nil), items...)
	}
	if items, ok := m[key].([]interface{}); ok {
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			if mapped, ok := item.(map[string]interface{}); ok {
				out = append(out, mapped)
			}
		}
		return out
	}
	return nil
}
