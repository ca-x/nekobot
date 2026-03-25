import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
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

export interface WorkspaceStatus {
  path: string;
  exists: boolean;
  bootstrapped: boolean;
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
      return Array.isArray(data) ? data : [];
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
    queryFn: () => api.get<MarketplaceSkill>(`/api/marketplace/skills/items/${encodeURIComponent(skillID ?? '')}`),
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
    queryFn: () => api.get<MarketplaceInventoryResponse>('/api/marketplace/skills/inventory'),
    staleTime: 30_000,
  });
}

export function useMarketplaceSnapshots() {
  return useQuery<MarketplaceSnapshotListResponse>({
    queryKey: marketplaceKeys.snapshots(),
    queryFn: () => api.get<MarketplaceSnapshotListResponse>('/api/marketplace/skills/snapshots'),
    staleTime: 15_000,
  });
}

export function useWorkspaceStatus() {
  return useQuery<WorkspaceStatus>({
    queryKey: marketplaceKeys.workspace(),
    queryFn: () => api.get<WorkspaceStatus>('/api/workspace/status'),
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
        toast.success('Skill dependencies installed');
        return;
      }
      toast.error('Some dependencies failed to install');
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
      toast.success('Skill installed');
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
      toast.success('Snapshot created');
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
      toast.success('Snapshot restored');
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
      toast.success('Snapshot deleted');
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
      toast.success(`Pruned ${data.deleted} old snapshots`);
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
      toast.success('Workspace repaired');
    },
    onError: (err) => toast.error(err.message),
  });
}
