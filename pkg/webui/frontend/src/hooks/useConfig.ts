import { api } from "@/api/client";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from '@/lib/notify';
import { t } from "@/lib/i18n";
import type { RuntimeAgent } from "@/hooks/useTopology";
import type { CronJob } from "@/hooks/useCron";

export interface ConfigData {
  [section: string]: Record<string, unknown>;
}

export interface ConfigMutationResult {
  status?: string;
  sections_saved?: number;
  providers_imported?: number;
  restart_required?: boolean;
  restart_sections?: string[];
}

export interface ServiceStatusData {
  name: string;
  platform: string;
  config_path: string;
  arguments: string[];
  installed: boolean;
  status: string;
}

export interface WatchPattern {
  file_glob: string;
  command: string;
  fail_command?: string;
}

export interface WatchStatusData {
  enabled: boolean;
  running: boolean;
  debounce_ms: number;
  patterns: WatchPattern[];
  last_run_at?: string;
  last_command?: string;
  last_file?: string;
  last_success: boolean;
  last_error?: string;
  last_result_preview?: string;
}

export interface AuditEntry {
  ts: string;
  tool: string;
  args?: Record<string, unknown>;
  duration_ms: number;
  success: boolean;
  result_preview?: string;
  error?: string;
  session_id?: string;
  workspace?: string;
}

export interface AuditStatsData {
  exists: boolean;
  entries: number;
  size?: number;
  file?: string;
  modified?: string;
}

export interface HarnessAuditData {
  entries: AuditEntry[];
  stats: AuditStatsData;
  limit: number;
}

export interface StatusTask {
  id: string;
  type: string;
  state: string;
  summary?: string;
  session_id?: string;
  runtime_id?: string;
  actual_provider?: string;
  actual_model?: string;
  pending_action?: string;
  last_error?: string;
  permission_mode?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  metadata?: Record<string, unknown>;
}

export interface SessionRuntimeState {
  session_id: string;
  lifecycle_state?: string;
  lifecycle_detail?: string;
  max_tool_rounds?: number;
  tool_rounds?: number;
  tool_calls?: Record<string, number>;
  per_tool_limits?: Record<string, number>;
  permission_mode?: string;
  pending_action?: string;
  pending_request_id?: string;
  updated_at: string;
}

export interface AgentDefinitionStatus {
  id: string;
  orchestrator: string;
  permissionMode: string;
  maxToolIterations: number;
  route?: {
    provider: string;
    model: string;
    fallback: string[];
  };
  toolPolicy?: {
    allowlist: string[];
    denylist: string[];
  };
  promptSections?: {
    static: string[];
    dynamic: string[];
  };
}

export interface DaemonWorkspaceInfo {
  workspace_id: string;
  machine_id: string;
  path: string;
  display_name: string;
  aliases: string[];
  is_default: boolean;
}

export interface DaemonWorkspaceTreeEntry {
  path: string;
  name: string;
  is_dir: boolean;
  size_bytes: number;
  modified_time_unix: number;
}

export interface DaemonWorkspaceTreeData {
  workspace_id: string;
  path: string;
  entries: DaemonWorkspaceTreeEntry[];
}

export interface DaemonWorkspaceFileData {
  workspace_id: string;
  path: string;
  content: string;
  truncated: boolean;
  size_bytes: number;
}

export interface DaemonMachineStatus {
  info: {
    daemon_id: string;
    machine_id: string;
    machine_name: string;
    hostname: string;
    os: string;
    arch: string;
    version: string;
    status: string;
    last_seen_unix: number;
    daemon_url?: string;
  };
  workspace_count: number;
  runtime_count: number;
  installed_runtime_count: number;
  healthy_runtime_count: number;
  goal_run_runnable: boolean;
}

export interface DaemonBootstrapData {
  server_url: string;
  daemon_token: string;
  command: string;
}

export interface NekoClientdBootstrapData {
  server_url: string;
  daemon_token: string;
  grpc_target: string;
  target_os: string;
  target_arch: string;
  binary_name: string;
  version: string;
  download_base_url: string;
  download_url: string;
  archive_name: string;
  install_command: string;
  service_install_command: string;
  start_command: string;
}

export interface SystemServiceBundleData {
  gateway: ServiceStatusData;
  nekoclientd: ServiceStatusData;
}

export interface StatusData {
  version: string;
  commit: string;
  build_time: string;
  os: string;
  arch: string;
  go_version: string;
  pid: number;
  uptime: string;
  uptime_seconds: number;
  memory_alloc_bytes: number;
  memory_sys_bytes: number;
  provider_count: number;
  config_path: string;
  database_dir: string;
  runtime_db_path: string;
  workspace_path: string;
  workspace_contract?: {
    kind: string;
    validation?: {
      on_turn_end?: string[];
      on_source_change?: string[];
      on_completion?: string[];
    };
    artifacts?: Record<string, string>;
    spawn_tasks?: Record<string, {
      artifacts?: string[];
      on_verify?: string[];
      on_failure?: string[];
    }>;
  };
  workspace_validation_summary?: {
    on_turn_end?: Array<{ name: string; passed: boolean; detail?: string }>;
    on_source_change?: Array<{ name: string; passed: boolean; detail?: string }>;
    on_completion?: Array<{ name: string; passed: boolean; detail?: string }>;
  };
  task_count: number;
  task_state_counts: Record<string, number>;
  recent_tasks: StatusTask[];
  recent_cron_jobs: CronJob[];
  runtime_states: RuntimeAgent[];
  daemon_machines: DaemonMachineStatus[];
  session_runtime_states: SessionRuntimeState[];
  agent_definition?: AgentDefinitionStatus | null;
  gateway_host: string;
  gateway_port: number;
  gateway: {
    host: string;
    port: number;
  };
}

function formatRestartNotice(result: ConfigMutationResult): string | null {
  if (
    !result.restart_required ||
    !result.restart_sections ||
    result.restart_sections.length === 0
  ) {
    return null;
  }
  return t("configRestartRequired", result.restart_sections.join(", "));
}

export function useConfig() {
  return useQuery<ConfigData>({
    queryKey: ["config"],
    queryFn: () => api.get("/api/config"),
    staleTime: 30_000,
  });
}

export function useSaveConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: ConfigData) =>
      api.put<ConfigMutationResult>("/api/config", data),
    onSuccess: (result) => {
      qc.invalidateQueries({ queryKey: ["config"] });
      qc.invalidateQueries({ queryKey: ["watch-status"] });
      toast.success(t("configSaved"));
      const restartNotice = formatRestartNotice(result ?? {});
      if (restartNotice) {
        toast.info(restartNotice);
      }
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useExportConfig() {
  return useMutation({
    mutationFn: async () => {
      const data = await api.get<ConfigData>("/api/config/export");
      const blob = new Blob([JSON.stringify(data, null, 2)], {
        type: "application/json",
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `nekobot-config-${new Date().toISOString().slice(0, 10)}.json`;
      a.click();
      URL.revokeObjectURL(url);
    },
    onSuccess: () => toast.success(t("exported")),
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useImportConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: ConfigData) =>
      api.post<ConfigMutationResult>("/api/config/import", data),
    onSuccess: (result) => {
      qc.invalidateQueries({ queryKey: ["config"] });
      qc.invalidateQueries({ queryKey: ["watch-status"] });
      if (result) {
        toast.success(
          t(
            "imported",
            String(result.sections_saved ?? 0),
            String(result.providers_imported ?? 0),
          ),
        );
        const restartNotice = formatRestartNotice(result);
        if (restartNotice) {
          toast.info(restartNotice);
        }
      }
    },
    onError: () => toast.error(t("importFailed")),
  });
}

export function useStatus() {
  return useQuery<StatusData>({
    queryKey: ["status"],
    queryFn: () => api.get("/api/status"),
    staleTime: 10_000,
  });
}

export function useServiceStatus() {
  return useQuery<SystemServiceBundleData>({
    queryKey: ["service-status"],
    queryFn: () => api.get("/api/service"),
    staleTime: 10_000,
  });
}

export function useDaemonBootstrap() {
  return useQuery<DaemonBootstrapData>({
    queryKey: ["daemon-bootstrap"],
    queryFn: () => api.get("/api/daemon/bootstrap"),
    staleTime: 30_000,
  });
}

export function useNekoClientdBootstrap(targetOS = "linux", targetArch = "amd64") {
  return useQuery<NekoClientdBootstrapData>({
    queryKey: ["nekoclientd-bootstrap", targetOS, targetArch],
    queryFn: () => api.get(`/api/nekoclientd/bootstrap?os=${encodeURIComponent(targetOS)}&arch=${encodeURIComponent(targetArch)}`),
    staleTime: 30_000,
  });
}

export function useWatchStatus() {
  return useQuery<WatchStatusData>({
    queryKey: ["watch-status"],
    queryFn: () => api.get("/api/harness/watch"),
    staleTime: 10_000,
  });
}

export function useUpdateWatchStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: Partial<WatchStatusData>) =>
      api.post("/api/harness/watch", payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["watch-status"] });
      qc.invalidateQueries({ queryKey: ["config"] });
      toast.success(t("configSaved"));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useHarnessAudit(limit = 100) {
  return useQuery<HarnessAuditData>({
    queryKey: ["harness-audit", limit],
    queryFn: () => api.get(`/api/harness/audit?limit=${limit}`),
    staleTime: 5_000,
    refetchInterval: 10_000,
  });
}

export function useClearHarnessAudit() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      api.post<{ status: string; stats: AuditStatsData }>(
        "/api/harness/audit/clear",
        {},
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["harness-audit"] });
      toast.success(t("harnessAuditCleared"));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useRestartService() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<{ status: string }>("/api/service/restart", {}),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["service-status"] });
      qc.invalidateQueries({ queryKey: ["status"] });
      toast.success(t("systemServiceRestartQueued"));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useReloadService() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<{ status: string }>("/api/service/reload", {}),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["service-status"] });
      qc.invalidateQueries({ queryKey: ["status"] });
      qc.invalidateQueries({ queryKey: ["config"] });
      toast.success(t("systemServiceReloaded"));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useCleanupSessions() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<{ status: string }>("/api/sessions/cleanup"),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["config"] });
      qc.invalidateQueries({ queryKey: ["sessions"] });
      toast.success(t("sessionsCleanupRan"));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useCleanupToolSessionEvents() {
  return useMutation({
    mutationFn: () =>
      api.post<{ deleted: number }>("/api/tool-sessions/events/cleanup"),
    onSuccess: (result) => {
      toast.success(
        t("webuiToolSessionEventsCleanupDone", String(result.deleted ?? 0)),
      );
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useCleanupSkillVersions() {
  return useMutation({
    mutationFn: () =>
      api.post<{
        deleted: number;
        max_count: number;
        enabled: boolean;
        mode: string;
      }>("/api/marketplace/skills/versions/cleanup", {}),
    onSuccess: (result) => {
      if (result.mode === "clear_all") {
        toast.success(
          t("webuiSkillVersionsCleanupDone", String(result.deleted ?? 0)),
        );
        return;
      }
      toast.success(
        t("webuiSkillVersionsPruned", String(result.max_count ?? 0)),
      );
    },
    onError: (err: Error) => toast.error(err.message),
  });
}


export function useDaemonWorkspaceTree(machineID: string | null, workspaceID: string | null, path: string) {
  return useQuery<DaemonWorkspaceTreeData>({
    queryKey: ["daemon-workspace-tree", machineID, workspaceID, path],
    queryFn: () =>
      api.post("/api/daemon/explorer/tree", {
        machine_id: machineID,
        workspace_id: workspaceID,
        path,
      }),
    enabled: Boolean(machineID && workspaceID),
    staleTime: 10_000,
  });
}

export function useDaemonWorkspaceFile() {
  return useMutation({
    mutationFn: (payload: { machine_id: string; workspace_id: string; path: string }) =>
      api.post<DaemonWorkspaceFileData>("/api/daemon/explorer/file", payload),
  });
}
export function useDaemonExplorerWorkspaces(machineID: string | null) {
  return useQuery<{ workspaces: DaemonWorkspaceInfo[] }>({
    queryKey: ["daemon-explorer-workspaces", machineID],
    queryFn: () => api.get(`/api/daemon/explorer/workspaces?machine_id=${encodeURIComponent(machineID ?? "")}`),
    enabled: Boolean(machineID),
    staleTime: 10_000,
  });
}
