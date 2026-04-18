import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from '@/lib/notify';
import { t } from '@/lib/i18n';

export type PermissionRuleAction = 'allow' | 'deny' | 'ask';

export interface PermissionRule {
  id?: string;
  enabled: boolean;
  priority: number;
  tool_name: string;
  session_id?: string;
  runtime_id?: string;
  action: PermissionRuleAction;
  description?: string;
  created_at?: string;
  updated_at?: string;
}

export interface PermissionRuleInput {
  enabled: boolean;
  priority: number;
  tool_name: string;
  session_id?: string;
  runtime_id?: string;
  action: PermissionRuleAction;
  description?: string;
}

export const PERMISSION_RULES_KEY = ['permission-rules'] as const;

export function usePermissionRules() {
  return useQuery<PermissionRule[]>({
    queryKey: [...PERMISSION_RULES_KEY],
    queryFn: () => api.get('/api/permission-rules'),
    staleTime: 30_000,
  });
}

export function useCreatePermissionRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: PermissionRuleInput) =>
      api.post<{ status: string; rule: PermissionRule }>('/api/permission-rules', input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PERMISSION_RULES_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useUpdatePermissionRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: PermissionRuleInput }) =>
      api.put<{ status: string; rule: PermissionRule }>(
        `/api/permission-rules/${encodeURIComponent(id)}`,
        data,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PERMISSION_RULES_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeletePermissionRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      api.delete<{ status: string; id: string }>(`/api/permission-rules/${encodeURIComponent(id)}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...PERMISSION_RULES_KEY] });
      toast.success(t('deleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}
