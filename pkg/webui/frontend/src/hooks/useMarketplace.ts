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

const marketplaceKeys = {
  all: ['marketplace'] as const,
  skills: () => [...marketplaceKeys.all, 'skills'] as const,
  installed: () => [...marketplaceKeys.all, 'installed'] as const,
  item: (skillID: string) => [...marketplaceKeys.all, 'item', skillID] as const,
  content: (skillID: string) => [...marketplaceKeys.all, 'content', skillID] as const,
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

export function useEnableMarketplaceSkill() {
  const qc = useQueryClient();
  return useMutation<MarketplaceToggleResponse, Error, string>({
    mutationFn: (skillID) =>
      api.post<MarketplaceToggleResponse>(
        `/api/marketplace/skills/${encodeURIComponent(skillID)}/enable`,
        {},
      ),
    onSuccess: (_data, skillID) => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.skills() });
      qc.invalidateQueries({ queryKey: marketplaceKeys.installed() });
      qc.invalidateQueries({ queryKey: marketplaceKeys.item(skillID) });
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
      qc.invalidateQueries({ queryKey: marketplaceKeys.skills() });
      qc.invalidateQueries({ queryKey: marketplaceKeys.installed() });
      qc.invalidateQueries({ queryKey: marketplaceKeys.item(skillID) });
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
      qc.invalidateQueries({ queryKey: marketplaceKeys.skills() });
      qc.invalidateQueries({ queryKey: marketplaceKeys.installed() });
      qc.invalidateQueries({ queryKey: marketplaceKeys.item(skillID) });
      if (data.success) {
        toast.success('Skill dependencies installed');
        return;
      }
      toast.error('Some dependencies failed to install');
    },
    onError: (err) => toast.error(err.message),
  });
}
