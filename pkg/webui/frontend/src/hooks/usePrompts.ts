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

export interface ContextSourceRecord {
  kind: string;
  title: string;
  stable: boolean;
  summary?: string;
  item_count?: number;
  metadata?: Record<string, unknown>;
}

export interface ContextFootprintRecord {
  system_chars: number;
  identity_chars: number;
  bootstrap_chars: number;
  skills_chars: number;
  memory_chars: number;
  managed_prompt_chars: number;
  user_prompt_chars: number;
  raw_user_chars: number;
  preprocessed_chars: number;
  file_reference_chars: number;
  final_user_chars: number;
  total_chars: number;
  memory_limit_chars: number;
  mention_count: number;
}

export interface ContextCompactionRecord {
  recommended: boolean;
  strategy?: string;
  reasons?: string[];
  estimated_chars_saved?: number;
}

export interface ContextPreflightRecord {
  action?: 'proceed' | 'consider_compaction' | 'compact_before_run';
  budget_status?: 'ok' | 'warning' | 'critical';
  budget_reasons?: string[];
  compaction: ContextCompactionRecord;
}

export interface ContextSourcesPreview {
  sources: ContextSourceRecord[];
  system_prompt_text?: string;
  user_prompt_text?: string;
  preprocessed_input?: string;
  footprint: ContextFootprintRecord;
  preflight: ContextPreflightRecord;
  budget_status?: 'ok' | 'warning' | 'critical';
  budget_reasons?: string[];
  compaction: ContextCompactionRecord;
  warnings?: string[];
}

export interface ContextSourcesPreviewInput {
  channel?: string;
  session_id?: string;
  user_id?: string;
  username?: string;
  requested_provider?: string;
  requested_model?: string;
  requested_fallback?: string[];
  explicit_prompt_ids?: string[];
  custom?: Record<string, unknown>;
  user_message?: string;
}

const PROMPTS_KEY = ['prompts'] as const;
const PROMPT_BINDINGS_KEY = ['prompt-bindings'] as const;
const promptSessionBindingsKey = (sessionID: string) =>
  ['prompt-session-bindings', sessionID] as const;

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
    queryKey: promptSessionBindingsKey(sessionID ?? ''),
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
    onMutate: async (variables) => {
      const queryKey = promptSessionBindingsKey(variables.sessionID);
      await qc.cancelQueries({ queryKey });

      const previous = qc.getQueryData<PromptSessionBindingSet>(queryKey);
      qc.setQueryData<PromptSessionBindingSet>(queryKey, {
        system_prompt_ids: [...variables.systemPromptIDs],
        user_prompt_ids: [...variables.userPromptIDs],
        bindings: previous?.bindings ?? [],
      });

      return { previous, queryKey };
    },
    onSuccess: (data, variables) => {
      qc.setQueryData(promptSessionBindingsKey(variables.sessionID), data);
      qc.invalidateQueries({ queryKey: [...PROMPT_BINDINGS_KEY] });
    },
    onError: (err: Error, _variables, context) => {
      if (context?.previous) {
        qc.setQueryData(context.queryKey, context.previous);
      }
      toast.error(err.message);
    },
  });
}

export function usePreviewContextSources() {
  return useMutation({
    mutationFn: (input: ContextSourcesPreviewInput) =>
      api.post<ContextSourcesPreview>('/api/prompts/context-sources', input),
    onError: (err: Error) => toast.error(err.message),
  });
}
