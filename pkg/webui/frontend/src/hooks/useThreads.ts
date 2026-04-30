import { api, apiFetch } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from '@/lib/notify';
import { t } from '@/lib/i18n';

export interface ThreadSummary {
  id: string;
  target?: string;
  created_at: string;
  updated_at: string;
  summary: string;
  message_count: number;
  runtime_id: string;
  topic: string;
}

export interface ThreadDetail extends ThreadSummary {
  messages: Array<{
    id?: string;
    role: string;
    content: string;
    tool_call_id?: string;
    attachments?: Array<{
      attachment_id: string;
      filename: string;
      mime_type?: string;
      size_bytes: number;
      target?: string;
      owner_id?: string;
    }>;
  }>;
}

export interface SavedMessageRecord {
  saved_message_id: string;
  target: string;
  thread_id: string;
  message_id: string;
  saved_by_user_id?: string;
  saved_by_agent_id?: string;
  saved_time_unix: number;
  message?: {
    message_id: string;
    target: string;
    thread_id: string;
    role: string;
    content: string;
  };
}

export const threadKeys = {
  all: ['threads'] as const,
  list: () => [...threadKeys.all, 'list'] as const,
  detail: (id: string) => [...threadKeys.all, 'detail', id] as const,
  saved: (target?: string) => [...threadKeys.all, 'saved', target ?? ''] as const,
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

export function useSavedMessages(target?: string | null) {
  return useQuery<{ saved_messages?: SavedMessageRecord[] }>({
    queryKey: threadKeys.saved(target ?? ''),
    queryFn: () => api.get(`/api/daemon/saved-messages${target ? `?target=${encodeURIComponent(target)}` : ''}`),
    staleTime: 5_000,
  });
}

export function useSaveMessage() {
  const qc = useQueryClient();
  return useMutation<unknown, Error, { target: string; message_id: string; request_id: string }>({
    mutationFn: ({ target, message_id, request_id }) =>
      api.post(`/api/daemon/messages/${encodeURIComponent(message_id)}/save`, { target, request_id }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: threadKeys.saved(vars.target) });
      toast.success(t('messageSaved'));
    },
    onError: (err) => toast.error(err.message || t('messageSaveFailed')),
  });
}

export function useUnsaveMessage() {
  const qc = useQueryClient();
  return useMutation<unknown, Error, { target: string; message_id: string; request_id: string }>({
    mutationFn: ({ target, message_id, request_id }) =>
      apiFetch(`/api/daemon/messages/${encodeURIComponent(message_id)}/save`, {
        method: 'DELETE',
        body: JSON.stringify({ target, request_id }),
      }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: threadKeys.saved(vars.target) });
      toast.success(t('messageUnsaved'));
    },
    onError: (err) => toast.error(err.message || t('messageUnsaveFailed')),
  });
}
