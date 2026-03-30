import { api } from '@/api/client';
import { useQuery } from '@tanstack/react-query';

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

export function useRuntimeTopology() {
  return useQuery<RuntimeTopologySnapshot>({
    queryKey: ['runtime-topology'],
    queryFn: () => api.get('/api/runtime-topology'),
    staleTime: 10_000,
    refetchInterval: 15_000,
  });
}
