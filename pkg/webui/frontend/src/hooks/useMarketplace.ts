import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from '@/lib/notify';
import { t } from '@/lib/i18n';

export interface MarketplaceSkill {
  id: string;
  name: string;
  description: string;
  version: string;
  author: string;
  enabled: boolean;
  always: boolean;
  file_path: string;
  tags: string[];
  eligible: boolean;
  ineligibility_reasons: string[];
  missing_requirements: MarketplaceMissingRequirements;
  install_specs: MarketplaceInstallSpec[];
  is_installed: boolean;
}

export interface MarketplaceInstalledResponse {
  total: number;
  records: MarketplaceSkill[];
}

export interface MarketplaceSearchResponse {
  query: string;
  success: boolean;
  proxy: string;
  output: string;
  error: string;
  has_output: boolean;
}

export interface MarketplaceInstallSkillResponse {
  source: string;
  target: string;
  proxy: string;
  installed: boolean;
  refreshed: boolean;
}

export interface MarketplaceSkillContent {
  id: string;
  name: string;
  file_path: string;
  raw: string;
  body_raw: string;
}

export interface MarketplaceMissingRequirements {
  binaries: string[];
  any_binaries: string[];
  env: string[];
  config_paths: string[];
  python_packages: string[];
  node_packages: string[];
}

export interface MarketplaceInstallSpec {
  method: string;
  package: string;
  version: string;
  post_hook: string;
  options?: Record<string, unknown>;
}

export interface MarketplaceInstallResult {
  success: boolean;
  method: string;
  package: string;
  output: string;
  error: string;
  duration_ms: number;
  installed_at: string;
}

export interface MarketplaceInstallDependenciesResponse {
  skill_id: string;
  success: boolean;
  results: MarketplaceInstallResult[];
}

interface MarketplaceToggleResponse {
  status: 'enabled' | 'disabled';
}

export interface MarketplaceSkillSource {
  path: string;
  priority: number;
  type: string;
  exists: boolean;
  builtin: boolean;
}

export interface MarketplaceInventoryResponse {
  writable_dir: string;
  proxy: string;
  source_count: number;
  enabled_count: number;
  always_count: number;
  version_history: {
    enabled: boolean;
    max_count: number;
    skill_count: number;
    version_count: number;
  };
  sources: MarketplaceSkillSource[];
}

export interface MarketplaceSnapshot {
  id: string;
  timestamp: string;
  skill_count: number;
  enabled_count: number;
  metadata: Record<string, string>;
}

export interface MarketplaceSnapshotListResponse {
  total: number;
  snapshots: MarketplaceSnapshot[];
  auto_prune: boolean;
  max_count: number;
}

export interface MarketplaceSnapshotPruneResponse {
  deleted: number;
  max_count: number;
  auto_prune: boolean;
}


export interface MarketplaceEvolutionSuggestion {
  skill_id: string;
  name: string;
  enabled: boolean;
  always: boolean;
  reasons: string[];
}

export interface MarketplaceEvolutionReview {
  active_learnings: string;
  learning_entry_count: number;
  snapshot_count: number;
  inventory: {
    source_count: number;
    enabled_count: number;
    eligible_count: number;
    version_history: {
      skill_count: number;
      version_count: number;
    };
  };
  suggestions: MarketplaceEvolutionSuggestion[];
}


export interface MarketplaceWorkspaceDraftResponse {
  status: string;
  created: boolean;
  file_path: string;
  skill: MarketplaceSkill;
}

export interface WorkspaceStatus {
  path: string;
  exists: boolean;
  bootstrapped: boolean;
  contract?: {
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
  validation_summary?: {
    on_turn_end?: Array<{ name: string; passed: boolean; detail?: string }>;
    on_source_change?: Array<{ name: string; passed: boolean; detail?: string }>;
    on_completion?: Array<{ name: string; passed: boolean; detail?: string }>;
  };
  bootstrap_files: string[];
  missing_bootstrap: string[];
  today_log_path: string;
  today_log_exists: boolean;
  heartbeat_state_path: string;
  heartbeat_state_exists: boolean;
  file_count: number;
  directory_count: number;
  updated_at: string;
}

function normalizeInstallSpec(input: Partial<MarketplaceInstallSpec> | null | undefined): MarketplaceInstallSpec {
  return {
    method: input?.method ?? '',
    package: input?.package ?? '',
    version: input?.version ?? '',
    post_hook: input?.post_hook ?? '',
    options: input?.options,
  };
}

function normalizeMissingRequirements(
  input: Partial<MarketplaceMissingRequirements> | null | undefined,
): MarketplaceMissingRequirements {
  return {
    binaries: Array.isArray(input?.binaries) ? input.binaries : [],
    any_binaries: Array.isArray(input?.any_binaries) ? input.any_binaries : [],
    env: Array.isArray(input?.env) ? input.env : [],
    config_paths: Array.isArray(input?.config_paths) ? input.config_paths : [],
    python_packages: Array.isArray(input?.python_packages) ? input.python_packages : [],
    node_packages: Array.isArray(input?.node_packages) ? input.node_packages : [],
  };
}

function normalizeMarketplaceSkill(input: Partial<MarketplaceSkill> | null | undefined): MarketplaceSkill {
  return {
    id: input?.id ?? '',
    name: input?.name ?? '',
    description: input?.description ?? '',
    version: input?.version ?? '',
    author: input?.author ?? '',
    enabled: Boolean(input?.enabled),
    always: Boolean(input?.always),
    file_path: input?.file_path ?? '',
    tags: Array.isArray(input?.tags) ? input.tags : [],
    eligible: Boolean(input?.eligible),
    ineligibility_reasons: Array.isArray(input?.ineligibility_reasons) ? input.ineligibility_reasons : [],
    missing_requirements: normalizeMissingRequirements(input?.missing_requirements),
    install_specs: Array.isArray(input?.install_specs)
      ? input.install_specs.map((spec) => normalizeInstallSpec(spec))
      : [],
    is_installed: Boolean(input?.is_installed),
  };
}

function normalizeMarketplaceInventory(
  input: Partial<MarketplaceInventoryResponse> | null | undefined,
): MarketplaceInventoryResponse {
  const rawHistory = input?.version_history;
  return {
    writable_dir: input?.writable_dir ?? '',
    proxy: input?.proxy ?? '',
    source_count: input?.source_count ?? 0,
    enabled_count: input?.enabled_count ?? 0,
    always_count: input?.always_count ?? 0,
    version_history: {
      enabled: Boolean(rawHistory?.enabled),
      max_count: rawHistory?.max_count ?? 0,
      skill_count: rawHistory?.skill_count ?? 0,
      version_count: rawHistory?.version_count ?? 0,
    },
    sources: Array.isArray(input?.sources) ? input.sources : [],
  };
}

function normalizeMarketplaceSnapshot(
  input: Partial<MarketplaceSnapshot> | null | undefined,
): MarketplaceSnapshot {
  return {
    id: input?.id ?? '',
    timestamp: input?.timestamp ?? '',
    skill_count: input?.skill_count ?? 0,
    enabled_count: input?.enabled_count ?? 0,
    metadata: input?.metadata ?? {},
  };
}

function normalizeWorkspaceStatus(input: Partial<WorkspaceStatus> | null | undefined): WorkspaceStatus {
  return {
    path: input?.path ?? '',
    exists: Boolean(input?.exists),
    bootstrapped: Boolean(input?.bootstrapped),
    bootstrap_files: Array.isArray(input?.bootstrap_files) ? input.bootstrap_files : [],
    missing_bootstrap: Array.isArray(input?.missing_bootstrap) ? input.missing_bootstrap : [],
    today_log_path: input?.today_log_path ?? '',
    today_log_exists: Boolean(input?.today_log_exists),
    heartbeat_state_path: input?.heartbeat_state_path ?? '',
    heartbeat_state_exists: Boolean(input?.heartbeat_state_exists),
    file_count: input?.file_count ?? 0,
    directory_count: input?.directory_count ?? 0,
    updated_at: input?.updated_at ?? '',
  };
}

const marketplaceKeys = {
  all: ['marketplace'] as const,
  skills: () => [...marketplaceKeys.all, 'skills'] as const,
  installed: () => [...marketplaceKeys.all, 'installed'] as const,
  item: (skillID: string) => [...marketplaceKeys.all, 'item', skillID] as const,
  content: (skillID: string) => [...marketplaceKeys.all, 'content', skillID] as const,
  inventory: () => [...marketplaceKeys.all, 'inventory'] as const,
  snapshots: () => [...marketplaceKeys.all, 'snapshots'] as const,
  workspace: () => [...marketplaceKeys.all, 'workspace'] as const,
};

export function useMarketplaceSkills() {
  return useQuery<MarketplaceSkill[]>({
    queryKey: marketplaceKeys.skills(),
    queryFn: async () => {
      const data = await api.get<MarketplaceSkill[]>('/api/marketplace/skills');
      return Array.isArray(data) ? data.map((skill) => normalizeMarketplaceSkill(skill)) : [];
    },
    staleTime: 30_000,
  });
}

export function useInstalledMarketplaceSkills() {
  return useQuery<MarketplaceInstalledResponse>({
    queryKey: marketplaceKeys.installed(),
    queryFn: () => api.get<MarketplaceInstalledResponse>('/api/marketplace/skills/installed'),
    staleTime: 30_000,
  });
}

export function useSearchMarketplaceSkills(query: string) {
  return useQuery<MarketplaceSearchResponse>({
    queryKey: [...marketplaceKeys.all, 'search', query],
    queryFn: () =>
      api.get<MarketplaceSearchResponse>(
        `/api/marketplace/skills/search?q=${encodeURIComponent(query)}`,
      ),
    enabled: query.trim().length > 0,
    staleTime: 15_000,
  });
}

export function useMarketplaceSkillItem(skillID: string | null) {
  return useQuery<MarketplaceSkill>({
    queryKey: marketplaceKeys.item(skillID ?? ''),
    queryFn: async () =>
      normalizeMarketplaceSkill(
        await api.get<MarketplaceSkill>(`/api/marketplace/skills/items/${encodeURIComponent(skillID ?? '')}`),
      ),
    enabled: Boolean(skillID),
    staleTime: 30_000,
  });
}

export function useMarketplaceSkillContent(skillID: string | null) {
  return useQuery<MarketplaceSkillContent>({
    queryKey: marketplaceKeys.content(skillID ?? ''),
    queryFn: () =>
      api.get<MarketplaceSkillContent>(
        `/api/marketplace/skills/items/${encodeURIComponent(skillID ?? '')}/content`,
      ),
    enabled: Boolean(skillID),
    staleTime: 30_000,
  });
}

export function useMarketplaceInventory() {
  return useQuery<MarketplaceInventoryResponse>({
    queryKey: marketplaceKeys.inventory(),
    queryFn: async () =>
      normalizeMarketplaceInventory(
        await api.get<MarketplaceInventoryResponse>('/api/marketplace/skills/inventory'),
      ),
    staleTime: 30_000,
  });
}

export function useMarketplaceSnapshots() {
  return useQuery<MarketplaceSnapshotListResponse>({
    queryKey: marketplaceKeys.snapshots(),
    queryFn: async () => {
      const data = await api.get<MarketplaceSnapshotListResponse>('/api/marketplace/skills/snapshots');
      return {
        total: data?.total ?? 0,
        snapshots: Array.isArray(data?.snapshots)
          ? data.snapshots.map((snapshot) => normalizeMarketplaceSnapshot(snapshot))
          : [],
        auto_prune: Boolean(data?.auto_prune),
        max_count: data?.max_count ?? 0,
      };
    },
    staleTime: 15_000,
  });
}

export function useWorkspaceStatus() {
  return useQuery<WorkspaceStatus>({
    queryKey: marketplaceKeys.workspace(),
    queryFn: async () =>
      normalizeWorkspaceStatus(await api.get<WorkspaceStatus>('/api/workspace/status')),
    staleTime: 15_000,
  });
}

export function useEnableMarketplaceSkill() {
  const qc = useQueryClient();
  return useMutation<MarketplaceToggleResponse, Error, string>({
    mutationFn: (skillID) =>
      api.post<MarketplaceToggleResponse>(
        `/api/marketplace/skills/${encodeURIComponent(skillID)}/enable`,
        {},
      ),
    onSuccess: (_data, skillID) => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.all });
      toast.success(t('marketplaceSkillEnabled'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useDisableMarketplaceSkill() {
  const qc = useQueryClient();
  return useMutation<MarketplaceToggleResponse, Error, string>({
    mutationFn: (skillID) =>
      api.post<MarketplaceToggleResponse>(
        `/api/marketplace/skills/${encodeURIComponent(skillID)}/disable`,
        {},
      ),
    onSuccess: (_data, skillID) => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.all });
      toast.success(t('marketplaceSkillDisabled'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useInstallMarketplaceSkillDependencies() {
  const qc = useQueryClient();
  return useMutation<MarketplaceInstallDependenciesResponse, Error, string>({
    mutationFn: (skillID) =>
      api.post<MarketplaceInstallDependenciesResponse>(
        `/api/marketplace/skills/${encodeURIComponent(skillID)}/install-deps`,
        {},
      ),
    onSuccess: (data, skillID) => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.all });
      if (data.success) {
        toast.success(t('marketplaceDependenciesInstalled'));
        return;
      }
      toast.error(t('marketplaceDependenciesPartialFailure'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useInstallMarketplaceSkill() {
  const qc = useQueryClient();
  return useMutation<MarketplaceInstallSkillResponse, Error, string>({
    mutationFn: (source) =>
      api.post<MarketplaceInstallSkillResponse>('/api/marketplace/skills/install', { source }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.all });
      toast.success(t('marketplaceSkillInstalled'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useCreateMarketplaceSnapshot() {
  const qc = useQueryClient();
  return useMutation<MarketplaceSnapshot, Error, { label?: string; note?: string }>({
    mutationFn: (payload) => api.post<MarketplaceSnapshot>('/api/marketplace/skills/snapshots', payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.snapshots() });
      toast.success(t('marketplaceSnapshotCreated'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useRestoreMarketplaceSnapshot() {
  const qc = useQueryClient();
  return useMutation<{ id: string; status: string }, Error, string>({
    mutationFn: (snapshotID) =>
      api.post<{ id: string; status: string }>(
        `/api/marketplace/skills/snapshots/${encodeURIComponent(snapshotID)}/restore`,
        {},
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.all });
      toast.success(t('marketplaceSnapshotRestored'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useDeleteMarketplaceSnapshot() {
  const qc = useQueryClient();
  return useMutation<{ id: string; status: string }, Error, string>({
    mutationFn: (snapshotID) =>
      api.delete<{ id: string; status: string }>(
        `/api/marketplace/skills/snapshots/${encodeURIComponent(snapshotID)}`,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.snapshots() });
      toast.success(t('marketplaceSnapshotDeleted'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function usePruneMarketplaceSnapshots() {
  const qc = useQueryClient();
  return useMutation<MarketplaceSnapshotPruneResponse, Error, void>({
    mutationFn: () =>
      api.post<MarketplaceSnapshotPruneResponse>('/api/marketplace/skills/snapshots/prune', {}),
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.snapshots() });
      toast.success(t('marketplaceSnapshotsPruned', String(data.deleted)));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useRepairWorkspace() {
  const qc = useQueryClient();
  return useMutation<WorkspaceStatus, Error, void>({
    mutationFn: () => api.post<WorkspaceStatus>('/api/workspace/repair', {}),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.workspace() });
      toast.success(t('marketplaceWorkspaceRepaired'));
    },
    onError: (err) => toast.error(err.message),
  });
}


export function useMarketplaceSkillEvolutionReview() {
  return useQuery<MarketplaceEvolutionReview>({
    queryKey: [...marketplaceKeys.all, 'evolution-review'],
    queryFn: () => api.get('/api/marketplace/skills/evolution-review'),
    staleTime: 10_000,
  });
}


export function useCreateMarketplaceWorkspaceDraft() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ skillID, content }: { skillID: string; content: string }) =>
      api.post<MarketplaceWorkspaceDraftResponse>(
        `/api/marketplace/skills/items/${encodeURIComponent(skillID)}/workspace-draft`,
        { content },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.skills() });
      qc.invalidateQueries({ queryKey: marketplaceKeys.item('') });
      toast.success('Workspace skill draft saved.');
    },
    onError: (err: Error) => toast.error(err.message),
  });
}
