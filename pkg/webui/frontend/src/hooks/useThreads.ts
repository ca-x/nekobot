import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { t } from '@/lib/i18n';

export interface ThreadSummary {
  id: string;
  created_at: string;
  updated_at: string;
  summary: string;
  message_count: number;
  runtime_id: string;
  topic: string;
}

export interface ThreadDetail extends ThreadSummary {
  messages: Array<{
    role: string;
    content: string;
    tool_call_id?: string;
  }>;
}

export const threadKeys = {
  all: ['threads'] as const,
  list: () => [...threadKeys.all, 'list'] as const,
  detail: (id: string) => [...threadKeys.all, 'detail', id] as const,
};

export function useThreads() {
  return useQuery<ThreadSummary[]>({
    queryKey: threadKeys.list(),
    queryFn: async () => {
      const data = await api.get<ThreadSummary[]>('/api/threads');
      return Array.isArray(data) ? data : [];
    },
    staleTime: 10_000,
  });
}

export function useThreadDetail(id?: string | null) {
  return useQuery<ThreadDetail>({
    queryKey: threadKeys.detail(id ?? ''),
    queryFn: () => api.get<ThreadDetail>(`/api/threads/${encodeURIComponent(id!)}`),
    enabled: !!id,
    staleTime: 5_000,
  });
}

export function useUpdateThread() {
  const qc = useQueryClient();
  return useMutation<unknown, Error, { id: string; summary: string; runtime_id: string; topic: string }>({
    mutationFn: ({ id, summary, runtime_id, topic }) =>
      api.put(`/api/threads/${encodeURIComponent(id)}`, { summary, runtime_id, topic }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: threadKeys.list() });
      qc.invalidateQueries({ queryKey: threadKeys.detail(vars.id) });
      qc.invalidateQueries({ queryKey: ['sessions'] });
      toast.success(t('threadSaved'));
    },
    onError: (err) => toast.error(err.message || t('threadSaveFailed')),
  });
}
