import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { t } from '@/lib/i18n';

export interface PromptRecord {
  id: string;
  key: string;
  name: string;
  description: string;
  mode: 'system' | 'user';
  template: string;
  enabled: boolean;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface PromptBindingRecord {
  id: string;
  scope: 'global' | 'channel' | 'session';
  target: string;
  prompt_id: string;
  enabled: boolean;
  priority: number;
  created_at: string;
  updated_at: string;
}

export interface PromptSessionBindingSet {
  system_prompt_ids: string[];
  user_prompt_ids: string[];
  bindings: PromptBindingRecord[];
}

export interface PromptInput {
  key: string;
  name: string;
  description: string;
  mode: 'system' | 'user';
  template: string;
  enabled: boolean;
  tags: string[];
}

export interface PromptBindingInput {
  scope: 'global' | 'channel' | 'session';
  target: string;
  prompt_id: string;
  enabled: boolean;
  priority: number;
}

const PROMPTS_KEY = ['prompts'] as const;
const PROMPT_BINDINGS_KEY = ['prompt-bindings'] as const;

export function usePrompts() {
  return useQuery<PromptRecord[]>({
    queryKey: [...PROMPTS_KEY],
    queryFn: () => api.get('/api/prompts'),
    staleTime: 15_000,
  });
}

export function usePromptBindings(scope?: string, target?: string) {
  return useQuery<PromptBindingRecord[]>({
    queryKey: [...PROMPT_BINDINGS_KEY, scope ?? '', target ?? ''],
    queryFn: () => {
      const params = new URLSearchParams();
      if (scope) {
        params.set('scope', scope);
      }
      if (target) {
        params.set('target', target);
      }
      const suffix = params.toString();
      return api.get(`/api/prompts/bindings${suffix ? `?${suffix}` : ''}`);
    },
    staleTime: 15_000,
  });
}

export function usePromptSessionBindings(sessionID: string | null) {
  return useQuery<PromptSessionBindingSet>({
    queryKey: ['prompt-session-bindings', sessionID ?? ''],
    queryFn: () => api.get(`/api/chat/prompts/session/${encodeURIComponent(sessionID ?? '')}`),
    enabled: Boolean(sessionID),
    staleTime: 5_000,
  });
}

export function useCreatePrompt() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: PromptInput) => api.post<PromptRecord>('/api/prompts', input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PROMPTS_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useUpdatePrompt() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: PromptInput }) =>
      api.put<PromptRecord>(`/api/prompts/${encodeURIComponent(id)}`, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PROMPTS_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeletePrompt() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/prompts/${encodeURIComponent(id)}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PROMPTS_KEY] });
      qc.invalidateQueries({ queryKey: [...PROMPT_BINDINGS_KEY] });
      toast.success(t('deleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useCreatePromptBinding() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: PromptBindingInput) => api.post<PromptBindingRecord>('/api/prompts/bindings', input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PROMPT_BINDINGS_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useUpdatePromptBinding() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: PromptBindingInput }) =>
      api.put<PromptBindingRecord>(`/api/prompts/bindings/${encodeURIComponent(id)}`, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PROMPT_BINDINGS_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeletePromptBinding() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/prompts/bindings/${encodeURIComponent(id)}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PROMPT_BINDINGS_KEY] });
      toast.success(t('deleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useReplacePromptSessionBindings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      sessionID,
      systemPromptIDs,
      userPromptIDs,
    }: {
      sessionID: string;
      systemPromptIDs: string[];
      userPromptIDs: string[];
    }) =>
      api.put<PromptSessionBindingSet>(`/api/chat/prompts/session/${encodeURIComponent(sessionID)}`, {
        system_prompt_ids: systemPromptIDs,
        user_prompt_ids: userPromptIDs,
      }),
    onSuccess: (_, variables) => {
      qc.invalidateQueries({ queryKey: ['prompt-session-bindings', variables.sessionID] });
      qc.invalidateQueries({ queryKey: [...PROMPT_BINDINGS_KEY] });
    },
    onError: (err: Error) => toast.error(err.message),
  });
}
