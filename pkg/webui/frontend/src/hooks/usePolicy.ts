import { api } from '@/api/client';
import { useMutation, useQuery } from '@tanstack/react-query';

export interface PolicyRule {
  name: string;
  description?: string;
  filesystem?: Record<string, unknown>;
  network?: Record<string, unknown>;
  tools?: Record<string, unknown>;
}

export interface PolicyEvaluationInput {
  tool_name?: string;
  path?: string;
  write?: boolean;
  host?: string;
  port?: number;
  method?: string;
  url_path?: string;
}

export interface PolicyEvaluationResult {
  allowed: boolean;
  area: string;
  reason: string;
}

export function usePolicyPresets() {
  return useQuery<PolicyRule[]>({
    queryKey: ['policy-presets'],
    queryFn: () => api.get('/api/policy/presets'),
    staleTime: 30_000,
  });
}

export function useEvaluatePolicy() {
  return useMutation({
    mutationFn: ({ policy, input }: { policy: PolicyRule; input: PolicyEvaluationInput }) =>
      api.post<PolicyEvaluationResult>('/api/policy/evaluate', { policy, input }),
  });
}
