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
}

interface MarketplaceToggleResponse {
  status: 'enabled' | 'disabled';
}

const marketplaceKeys = {
  all: ['marketplace'] as const,
  skills: () => [...marketplaceKeys.all, 'skills'] as const,
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

export function useEnableMarketplaceSkill() {
  const qc = useQueryClient();
  return useMutation<MarketplaceToggleResponse, Error, string>({
    mutationFn: (skillID) =>
      api.post<MarketplaceToggleResponse>(
        `/api/marketplace/skills/${encodeURIComponent(skillID)}/enable`,
        {},
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.skills() });
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
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: marketplaceKeys.skills() });
      toast.success(t('marketplaceSkillDisabled'));
    },
    onError: (err) => toast.error(err.message),
  });
}
