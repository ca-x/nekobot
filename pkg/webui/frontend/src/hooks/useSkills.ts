import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from '@/lib/notify';
import { t } from '@/lib/i18n';

export interface SkillItem {
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
  missing_requirements: SkillMissingRequirements;
  install_specs: SkillInstallSpec[];
  is_installed: boolean;
}

export interface SkillContent {
  id: string;
  name: string;
  file_path: string;
  raw: string;
  body_raw: string;
}

export interface SkillMissingRequirements {
  binaries: string[];
  any_binaries: string[];
  env: string[];
  config_paths: string[];
  python_packages: string[];
  node_packages: string[];
}

export interface SkillInstallSpec {
  method: string;
  package: string;
  version: string;
  post_hook: string;
  options?: Record<string, unknown>;
}

interface SkillToggleResponse {
  status: 'enabled' | 'disabled';
}

function normalizeInstallSpec(input: Partial<SkillInstallSpec> | null | undefined): SkillInstallSpec {
  return {
    method: input?.method ?? '',
    package: input?.package ?? '',
    version: input?.version ?? '',
    post_hook: input?.post_hook ?? '',
    options: input?.options,
  };
}

function normalizeMissingRequirements(
  input: Partial<SkillMissingRequirements> | null | undefined,
): SkillMissingRequirements {
  return {
    binaries: Array.isArray(input?.binaries) ? input.binaries : [],
    any_binaries: Array.isArray(input?.any_binaries) ? input.any_binaries : [],
    env: Array.isArray(input?.env) ? input.env : [],
    config_paths: Array.isArray(input?.config_paths) ? input.config_paths : [],
    python_packages: Array.isArray(input?.python_packages) ? input.python_packages : [],
    node_packages: Array.isArray(input?.node_packages) ? input.node_packages : [],
  };
}

function normalizeSkillItem(input: Partial<SkillItem> | null | undefined): SkillItem {
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

const skillKeys = {
  all: ['skills'] as const,
  list: () => [...skillKeys.all, 'list'] as const,
  item: (skillID: string) => [...skillKeys.all, 'item', skillID] as const,
  content: (skillID: string) => [...skillKeys.all, 'content', skillID] as const,
};

export function useSkills() {
  return useQuery<SkillItem[]>({
    queryKey: skillKeys.list(),
    queryFn: async () => {
      const data = await api.get<SkillItem[]>('/api/skills');
      return Array.isArray(data) ? data.map((item) => normalizeSkillItem(item)) : [];
    },
    staleTime: 30_000,
  });
}

export function useSkillItem(skillID: string | null) {
  return useQuery<SkillItem>({
    queryKey: skillKeys.item(skillID ?? ''),
    queryFn: async () =>
      normalizeSkillItem(
        await api.get<SkillItem>(`/api/skills/${encodeURIComponent(skillID ?? '')}`),
      ),
    enabled: Boolean(skillID),
    staleTime: 30_000,
  });
}

export function useSkillContent(skillID: string | null) {
  return useQuery<SkillContent>({
    queryKey: skillKeys.content(skillID ?? ''),
    queryFn: () =>
      api.get<SkillContent>(
        `/api/skills/${encodeURIComponent(skillID ?? '')}/content`,
      ),
    enabled: Boolean(skillID),
    staleTime: 30_000,
  });
}

export function useEnableSkill() {
  const qc = useQueryClient();
  return useMutation<SkillToggleResponse, Error, string>({
    mutationFn: (skillID) =>
      api.post<SkillToggleResponse>(
        `/api/skills/${encodeURIComponent(skillID)}/enable`,
        {},
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: skillKeys.all });
      toast.success(t('marketplaceSkillEnabled'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useDisableSkill() {
  const qc = useQueryClient();
  return useMutation<SkillToggleResponse, Error, string>({
    mutationFn: (skillID) =>
      api.post<SkillToggleResponse>(
        `/api/skills/${encodeURIComponent(skillID)}/disable`,
        {},
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: skillKeys.all });
      toast.success(t('marketplaceSkillDisabled'));
    },
    onError: (err) => toast.error(err.message),
  });
}
