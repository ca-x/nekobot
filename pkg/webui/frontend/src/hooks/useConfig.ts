import { api } from '@/api/client';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { t } from '@/lib/i18n';

export interface ConfigData {
  [section: string]: Record<string, unknown>;
}

export function useConfig() {
  return useQuery<ConfigData>({
    queryKey: ['config'],
    queryFn: () => api.get('/api/config'),
    staleTime: 30_000,
  });
}

export function useSaveConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: ConfigData) => api.put('/api/config', data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['config'] });
      toast.success(t('configSaved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useExportConfig() {
  return useMutation({
    mutationFn: async () => {
      const data = await api.get<ConfigData>('/api/config/export');
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `nekobot-config-${new Date().toISOString().slice(0, 10)}.json`;
      a.click();
      URL.revokeObjectURL(url);
    },
    onSuccess: () => toast.success(t('exported')),
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useImportConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: ConfigData) => api.post<{ sections: number; providers: number }>('/api/config/import', data),
    onSuccess: (result) => {
      qc.invalidateQueries({ queryKey: ['config'] });
      if (result) {
        toast.success(t('imported', String(result.sections ?? 0), String(result.providers ?? 0)));
      }
    },
    onError: () => toast.error(t('importFailed')),
  });
}

export function useStatus() {
  return useQuery<Record<string, unknown>>({
    queryKey: ['status'],
    queryFn: () => api.get('/api/status'),
    staleTime: 10_000,
  });
}
