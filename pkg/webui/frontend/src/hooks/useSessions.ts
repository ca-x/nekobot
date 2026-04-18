import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from '@/lib/notify';
import { t } from '@/lib/i18n';

export interface SessionSummary {
  id: string;
  created_at: string;
  updated_at: string;
  summary: string;
  message_count: number;
  runtime_id: string;
  topic: string;
}

export interface SessionMessage {
  role: string;
  content: string;
  tool_call_id?: string;
}

export interface SessionDetail extends SessionSummary {
  messages: SessionMessage[];
}

export const sessionKeys = {
  all: ['sessions'] as const,
  list: () => [...sessionKeys.all, 'list'] as const,
  detail: (id: string) => [...sessionKeys.all, 'detail', id] as const,
};

export function useSessions() {
  return useQuery<SessionSummary[]>({
    queryKey: sessionKeys.list(),
    queryFn: async () => {
      const data = await api.get<SessionSummary[]>('/api/sessions');
      return Array.isArray(data) ? data : [];
    },
    staleTime: 10_000,
  });
}

export function useSessionDetail(id?: string | null) {
  return useSessionDetailWithOptions(id, {});
}

export function useSessionDetailWithOptions(
  id?: string | null,
  options?: {
    refetchInterval?: number;
    enabled?: boolean;
  },
) {
  return useQuery<SessionDetail>({
    queryKey: sessionKeys.detail(id ?? ''),
    queryFn: () =>
      api.get<SessionDetail>(`/api/sessions/${encodeURIComponent(id!)}`),
    enabled: (options?.enabled ?? true) && !!id,
    staleTime: 5_000,
    refetchInterval: options?.refetchInterval,
  });
}

export function useUpdateSessionSummary() {
  const qc = useQueryClient();

  return useMutation<unknown, Error, { id: string; summary: string }>({
    mutationFn: ({ id, summary }) =>
      api.put(`/api/sessions/${encodeURIComponent(id)}/summary`, { summary }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: sessionKeys.list() });
      qc.invalidateQueries({ queryKey: sessionKeys.detail(vars.id) });
      toast.success(t('sessionSummarySaved'));
    },
    onError: (err) => toast.error(err.message || t('sessionSummarySaveFailed')),
  });
}

export function useDeleteSession() {
  const qc = useQueryClient();

  return useMutation<void, Error, string>({
    mutationFn: (id) => api.delete(`/api/sessions/${encodeURIComponent(id)}`),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: sessionKeys.list() });
      qc.invalidateQueries({ queryKey: sessionKeys.detail(id) });
      toast.success(t('sessionDeleted'));
    },
    onError: (err) => toast.error(err.message || t('sessionDeleteFailed')),
  });
}

export function useUpdateSessionRuntime() {
  const qc = useQueryClient();

  return useMutation<unknown, Error, { id: string; runtime_id: string }>({
    mutationFn: ({ id, runtime_id }) =>
      api.put(`/api/sessions/${encodeURIComponent(id)}/runtime`, { runtime_id }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: sessionKeys.list() });
      qc.invalidateQueries({ queryKey: sessionKeys.detail(vars.id) });
      toast.success(t('sessionRuntimeSaved'));
    },
    onError: (err) => toast.error(err.message || t('sessionRuntimeSaveFailed')),
  });
}

export function useUpdateSessionThread() {
  const qc = useQueryClient();

  return useMutation<unknown, Error, { id: string; topic: string }>({
    mutationFn: ({ id, topic }) =>
      api.put(`/api/sessions/${encodeURIComponent(id)}/thread`, { topic }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: sessionKeys.list() });
      qc.invalidateQueries({ queryKey: sessionKeys.detail(vars.id) });
      toast.success(t('sessionThreadSaved'));
    },
    onError: (err) => toast.error(err.message || t('sessionThreadSaveFailed')),
  });
}
