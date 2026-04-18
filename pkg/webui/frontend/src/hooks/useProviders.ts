import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from '@/lib/notify';
import { t } from '@/lib/i18n';

export interface Provider {
  name: string;
  provider_kind: string;
  api_key_set: boolean;
  api_base: string;
  proxy: string;
  default_weight: number;
  enabled: boolean;
  is_routing_default: boolean;
  supports_discovery: boolean;
  summary: string;
  timeout: number;
}

export interface ProviderRuntime {
  name: string;
  available: boolean;
  in_cooldown: boolean;
  error_count: number;
  cooldown_remaining_seconds: number;
  failure_counts: Record<string, number>;
  disabled_reason: string;
  last_failure_unix: number;
  cooldown_end_unix: number;
  disabled_until_unix: number;
}

export interface CreateProviderInput {
  name: string;
  provider_kind: string;
  api_key?: string;
  api_base?: string;
  proxy?: string;
  timeout?: number;
  default_weight?: number;
  enabled?: boolean;
}

export interface UpdateProviderInput {
  provider_kind?: string;
  api_key?: string;
  api_base?: string;
  proxy?: string;
  timeout?: number;
  default_weight?: number;
  enabled?: boolean;
}

export interface DiscoverModelsInput {
  name?: string;
  provider_kind: string;
  api_key?: string;
  api_base?: string;
  proxy?: string;
  timeout?: number;
}

interface DiscoverModelsResponse {
  provider_kind: string;
  models: string[];
}

export const PROVIDERS_KEY = ['providers'] as const;
export const PROVIDER_RUNTIME_KEY = ['providers', 'runtime'] as const;

export function useProviders() {
  return useQuery<Provider[]>({
    queryKey: [...PROVIDERS_KEY],
    queryFn: () => api.get('/api/providers'),
    staleTime: 30_000,
  });
}

export function useProviderRuntime() {
  return useQuery<ProviderRuntime[]>({
    queryKey: [...PROVIDER_RUNTIME_KEY],
    queryFn: () => api.get('/api/providers/runtime'),
    staleTime: 5_000,
    refetchInterval: 5_000,
  });
}

export function useCreateProvider() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateProviderInput) =>
      api.post<{ status: string; provider: Provider }>('/api/providers', input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PROVIDERS_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useUpdateProvider() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ name, data }: { name: string; data: UpdateProviderInput }) =>
      api.put<{ status: string; provider: Provider }>(
        `/api/providers/${encodeURIComponent(name)}`,
        data,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PROVIDERS_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeleteProvider() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => api.delete(`/api/providers/${encodeURIComponent(name)}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PROVIDERS_KEY] });
      toast.success(t('deleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDiscoverModels() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: DiscoverModelsInput) =>
      api.post<DiscoverModelsResponse>('/api/providers/discover-models', input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['models'] });
      qc.invalidateQueries({ queryKey: ['model-routes'] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(t('discoveryFailed', err.message)),
  });
}
