import { api } from '@/api/client';
import { t } from '@/lib/i18n';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

export interface RuntimeAgent {
  id: string;
  name: string;
  display_name: string;
  description: string;
  enabled: boolean;
  provider: string;
  model: string;
  prompt_id: string;
  skills: string[];
  tools: string[];
  policy: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface ChannelAccount {
  id: string;
  channel_type: string;
  account_key: string;
  display_name: string;
  description: string;
  enabled: boolean;
  config: Record<string, unknown>;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface AccountBinding {
  id: string;
  channel_account_id: string;
  agent_runtime_id: string;
  binding_mode: string;
  enabled: boolean;
  allow_public_reply: boolean;
  reply_label: string;
  priority: number;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface RuntimeTopologySummary {
  runtime_count: number;
  channel_account_count: number;
  binding_count: number;
  single_agent_accounts: number;
  multi_agent_accounts: number;
}

export interface RuntimeNode {
  runtime: RuntimeAgent;
  bound_account_count: number;
}

export interface AccountNode {
  account: ChannelAccount;
  binding_mode: string;
  bound_runtime_count: number;
}

export interface BindingEdge {
  binding: AccountBinding;
  runtime_name: string;
  account_label: string;
  channel_type: string;
}

export interface RuntimeTopologySnapshot {
  summary: RuntimeTopologySummary;
  runtimes: RuntimeNode[];
  accounts: AccountNode[];
  bindings: BindingEdge[];
}

export interface RuntimeAgentInput {
  name: string;
  display_name: string;
  description: string;
  enabled: boolean;
  provider: string;
  model: string;
  prompt_id: string;
  skills: string[];
  tools: string[];
  policy: Record<string, unknown>;
}

export interface ChannelAccountInput {
  channel_type: string;
  account_key: string;
  display_name: string;
  description: string;
  enabled: boolean;
  config: Record<string, unknown>;
  metadata: Record<string, unknown>;
}

export interface AccountBindingInput {
  channel_account_id: string;
  agent_runtime_id: string;
  binding_mode: string;
  enabled: boolean;
  allow_public_reply: boolean;
  reply_label: string;
  priority: number;
  metadata: Record<string, unknown>;
}

const TOPOLOGY_KEY = ['runtime-topology'] as const;
const RUNTIME_AGENTS_KEY = ['runtime-agents'] as const;
const CHANNEL_ACCOUNTS_KEY = ['channel-accounts'] as const;
const ACCOUNT_BINDINGS_KEY = ['account-bindings'] as const;

function invalidateTopologyQueries(queryClient: ReturnType<typeof useQueryClient>) {
  queryClient.invalidateQueries({ queryKey: [...TOPOLOGY_KEY] });
  queryClient.invalidateQueries({ queryKey: [...RUNTIME_AGENTS_KEY] });
  queryClient.invalidateQueries({ queryKey: [...CHANNEL_ACCOUNTS_KEY] });
  queryClient.invalidateQueries({ queryKey: [...ACCOUNT_BINDINGS_KEY] });
}

export function useRuntimeTopology() {
  return useQuery<RuntimeTopologySnapshot>({
    queryKey: [...TOPOLOGY_KEY],
    queryFn: () => api.get('/api/runtime-topology'),
    staleTime: 10_000,
    refetchInterval: 15_000,
  });
}

export function useRuntimeAgents() {
  return useQuery<RuntimeAgent[]>({
    queryKey: [...RUNTIME_AGENTS_KEY],
    queryFn: () => api.get('/api/runtime-agents'),
    staleTime: 10_000,
  });
}

export function useChannelAccounts() {
  return useQuery<ChannelAccount[]>({
    queryKey: [...CHANNEL_ACCOUNTS_KEY],
    queryFn: () => api.get('/api/channel-accounts'),
    staleTime: 10_000,
  });
}

export function useAccountBindings() {
  return useQuery<AccountBinding[]>({
    queryKey: [...ACCOUNT_BINDINGS_KEY],
    queryFn: () => api.get('/api/account-bindings'),
    staleTime: 10_000,
  });
}

export function useCreateRuntimeAgent() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: RuntimeAgentInput) => api.post<RuntimeAgent>('/api/runtime-agents', input),
    onSuccess: () => {
      invalidateTopologyQueries(queryClient);
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useUpdateRuntimeAgent() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: RuntimeAgentInput }) =>
      api.put<RuntimeAgent>(`/api/runtime-agents/${encodeURIComponent(id)}`, input),
    onSuccess: () => {
      invalidateTopologyQueries(queryClient);
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeleteRuntimeAgent() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/runtime-agents/${encodeURIComponent(id)}`),
    onSuccess: () => {
      invalidateTopologyQueries(queryClient);
      toast.success(t('deleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useCreateChannelAccount() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ChannelAccountInput) => api.post<ChannelAccount>('/api/channel-accounts', input),
    onSuccess: () => {
      invalidateTopologyQueries(queryClient);
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useUpdateChannelAccount() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: ChannelAccountInput }) =>
      api.put<ChannelAccount>(`/api/channel-accounts/${encodeURIComponent(id)}`, input),
    onSuccess: () => {
      invalidateTopologyQueries(queryClient);
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeleteChannelAccount() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/channel-accounts/${encodeURIComponent(id)}`),
    onSuccess: () => {
      invalidateTopologyQueries(queryClient);
      toast.success(t('deleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useCreateAccountBinding() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: AccountBindingInput) => api.post<AccountBinding>('/api/account-bindings', input),
    onSuccess: () => {
      invalidateTopologyQueries(queryClient);
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useUpdateAccountBinding() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: AccountBindingInput }) =>
      api.put<AccountBinding>(`/api/account-bindings/${encodeURIComponent(id)}`, input),
    onSuccess: () => {
      invalidateTopologyQueries(queryClient);
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeleteAccountBinding() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/account-bindings/${encodeURIComponent(id)}`),
    onSuccess: () => {
      invalidateTopologyQueries(queryClient);
      toast.success(t('deleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}
