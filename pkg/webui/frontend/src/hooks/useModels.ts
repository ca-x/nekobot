import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from '@/lib/notify';
import { t } from '@/lib/i18n';

export interface ModelCatalog {
  id?: string;
  model_id: string;
  display_name: string;
  developer?: string;
  family?: string;
  type?: string;
  capabilities?: string[];
  catalog_source?: string;
  enabled: boolean;
}

export interface ModelRoute {
  id?: string;
  model_id: string;
  provider_name: string;
  enabled: boolean;
  is_default: boolean;
  weight_override: number;
  aliases: string[];
  regex_rules: string[];
  metadata: Record<string, unknown>;
}

export interface CreateModelInput {
  model_id: string;
  display_name: string;
  developer?: string;
  family?: string;
  type?: string;
  capabilities?: string[];
  catalog_source?: string;
  enabled?: boolean;
}

export interface UpdateModelRouteInput {
  model_id: string;
  provider_name: string;
  enabled: boolean;
  is_default: boolean;
  weight_override?: number;
  aliases?: string[];
  regex_rules?: string[];
  metadata?: Record<string, unknown>;
}

export interface ResolvedModelOption {
  value: string;
  label: string;
  providers: string[];
}

export const MODELS_KEY = ['models'] as const;

export type ModelRoutesByModel = Record<string, ModelRoute[]>;


export function useModels() {
  return useQuery<ModelCatalog[]>({
    queryKey: [...MODELS_KEY],
    queryFn: () => api.get('/api/models'),
    staleTime: 30_000,
  });
}

export function useCreateModel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateModelInput) =>
      api.post<{ status: string; model: ModelCatalog }>('/api/models', input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...MODELS_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useModelRoutes(modelID: string) {
  return useQuery<ModelRoute[]>({
    queryKey: ['model-routes', modelID],
    queryFn: () => api.get(`/api/model-routes?model_id=${encodeURIComponent(modelID)}`),
    staleTime: 30_000,
    enabled: modelID.trim().length > 0,
  });
}

export function useModelRoutesForModels(modelIDs: string[]) {
  const trimmedModelIDs = Array.from(new Set(modelIDs.map((modelID) => modelID.trim()).filter(Boolean)));

  return useQuery<ModelRoutesByModel>({
    queryKey: ['model-routes-batch', trimmedModelIDs],
    queryFn: async () => {
      if (trimmedModelIDs.length === 0) {
        return {};
      }
      if (trimmedModelIDs.length === 1) {
        const routes = await api.get<ModelRoute[]>(`/api/model-routes?model_id=${encodeURIComponent(trimmedModelIDs[0])}`);
        return { [trimmedModelIDs[0]]: routes };
      }
      const params = trimmedModelIDs
        .map((modelID) => `model_ids=${encodeURIComponent(modelID)}`)
        .join('&');
      return api.get<ModelRoutesByModel>(`/api/model-routes?${params}`);
    },
    staleTime: 30_000,
    enabled: trimmedModelIDs.length > 0,
  });
}

export function useUpdateModelRoute() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ modelID, providerName, data }: { modelID: string; providerName: string; data: UpdateModelRouteInput }) =>
      api.put<{ status: string; route: ModelRoute }>(
        `/api/model-routes/${encodeURIComponent(modelID)}/${encodeURIComponent(providerName)}`,
        data,
      ),
    onSuccess: (_result, variables) => {
      qc.invalidateQueries({ queryKey: ['model-routes', variables.modelID] });
      qc.invalidateQueries({ queryKey: ['model-routes-batch'] });
      qc.invalidateQueries({ queryKey: [...MODELS_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function buildModelOptions(
  models: ModelCatalog[],
  routesByModel: Record<string, ModelRoute[]>,
): ResolvedModelOption[] {
  return models
    .filter((model) => model.enabled)
    .map((model) => {
      const routes = (routesByModel[model.model_id] ?? []).filter((route) => route.enabled);
      const providers = Array.from(new Set(routes.map((route) => route.provider_name.trim()).filter(Boolean)));
      return {
        value: model.model_id,
        label: model.display_name?.trim() || model.model_id,
        providers,
      };
    })
    .sort((left, right) => left.label.localeCompare(right.label));
}

export function normalizeRouteMetadataProviderModelID(route: ModelRoute): string {
  const raw = route.metadata?.provider_model_id;
  return typeof raw === 'string' ? raw.trim() : '';
}
