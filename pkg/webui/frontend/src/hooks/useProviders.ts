import { api } from '@/api/client';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { t } from '@/lib/i18n';

// ---------- Types ----------

export interface Provider {
  name: string;
  provider_kind: string;
  api_key: string;
  api_base: string;
  proxy: string;
  models: string[] | null;
  default_model: string;
  timeout: number;
}

export interface CreateProviderInput {
  name: string;
  provider_kind: string;
  api_key?: string;
  api_base?: string;
  proxy?: string;
  timeout?: number;
}

export interface UpdateProviderInput {
  provider_kind?: string;
  api_key?: string;
  api_base?: string;
  proxy?: string;
  models?: string[];
  default_model?: string;
  timeout?: number;
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

// ---------- Hooks ----------

const PROVIDERS_KEY = ['providers'] as const;

export function useProviders() {
  return useQuery<Provider[]>({
    queryKey: [...PROVIDERS_KEY],
    queryFn: () => api.get('/api/providers'),
    staleTime: 30_000,
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
      api.put<{ status: string; provider: Provider }>(`/api/providers/${encodeURIComponent(name)}`, data),
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
    mutationFn: (name: string) =>
      api.delete(`/api/providers/${encodeURIComponent(name)}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PROVIDERS_KEY] });
      toast.success(t('deleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDiscoverModels() {
  return useMutation({
    mutationFn: (input: DiscoverModelsInput) =>
      api.post<DiscoverModelsResponse>('/api/providers/discover-models', input),
    onError: (err: Error) => toast.error(t('discoveryFailed', err.message)),
  });
}
