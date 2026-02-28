import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { t } from '@/lib/i18n';

export interface SessionSummary {
  id: string;
  created_at: string;
  updated_at: string;
  summary: string;
  message_count: number;
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
  return useQuery<SessionDetail>({
    queryKey: sessionKeys.detail(id ?? ''),
    queryFn: () =>
      api.get<SessionDetail>(`/api/sessions/${encodeURIComponent(id!)}`),
    enabled: !!id,
    staleTime: 5_000,
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
