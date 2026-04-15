import { api } from '@/api/client';
import { t } from '@/lib/i18n';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

export type GoalRunStatus =
  | 'draft'
  | 'criteria_pending_confirmation'
  | 'ready'
  | 'running'
  | 'verifying'
  | 'needs_approval'
  | 'needs_human_confirmation'
  | 'completed'
  | 'failed'
  | 'canceled';

export type GoalRunRiskLevel = 'conservative' | 'balanced' | 'aggressive';
export type GoalRunScopeKind = 'server' | 'daemon';
export type GoalRunScopeSource = 'auto' | 'manual' | string;

export interface GoalRunScope {
  kind: GoalRunScopeKind;
  machine_id?: string;
  source: GoalRunScopeSource;
  reason?: string;
}

export interface GoalRun {
  id: string;
  name: string;
  goal: string;
  natural_language_criteria: string;
  status: GoalRunStatus;
  risk_level: GoalRunRiskLevel;
  allow_auto_scope: boolean;
  allow_parallel_workers: boolean;
  recommended_scope?: GoalRunScope;
  selected_scope?: GoalRunScope;
  current_worker_ids?: string[];
  last_evaluation_id?: string;
  last_activity_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
}

export type GoalCriterionType =
  | 'command'
  | 'file_exists'
  | 'file_contains'
  | 'manual_confirmation';

export type GoalCriterionStatus =
  | 'pending'
  | 'passed'
  | 'failed'
  | 'blocked'
  | 'needs_human_confirmation';

export interface GoalCriterion {
  id: string;
  title: string;
  type: GoalCriterionType;
  scope: GoalRunScope;
  required: boolean;
  status: GoalCriterionStatus;
  definition: Record<string, unknown>;
  evidence?: string[];
  last_error?: string;
  updated_at: string;
}

export interface GoalCriteriaSet {
  criteria: GoalCriterion[];
}

export interface GoalRunDetail {
  goal_run: GoalRun;
  criteria: GoalCriteriaSet;
  workers: GoalRunWorker[];
}

export interface GoalRunWorker {
  id: string;
  name: string;
  status: string;
  scope: GoalRunScope;
  task_id?: string;
  last_heartbeat_at?: string;
  last_progress_at?: string;
  lease_expires_at?: string;
  restart_count: number;
  last_error?: string;
}

export interface CreateGoalRunInput {
  name: string;
  goal: string;
  natural_language_criteria: string;
  risk_level: GoalRunRiskLevel;
  allow_auto_scope: boolean;
}

export interface CreateGoalRunResponse {
  goal_run: GoalRun;
  draft_criteria: GoalCriteriaSet;
}

export interface ConfirmGoalRunCriteriaInput {
  criteria: GoalCriteriaSet;
  selected_scope?: GoalRunScope;
}

export interface ConfirmGoalRunManualInput {
  criterion_id: string;
  approved: boolean;
  note?: string;
}

export const goalRunKeys = {
  all: ['goal-runs'] as const,
  list: () => [...goalRunKeys.all, 'list'] as const,
  detail: (id: string) => [...goalRunKeys.all, 'detail', id] as const,
};

export function useGoalRuns() {
  return useQuery<GoalRun[]>({
    queryKey: goalRunKeys.list(),
    queryFn: async () => {
      const data = await api.get<{ items?: GoalRun[] }>('/api/goal-runs');
      return Array.isArray(data.items) ? data.items : [];
    },
    staleTime: 5_000,
    refetchInterval: 10_000,
  });
}

export function useGoalRunDetail(id?: string | null) {
  return useQuery<GoalRunDetail>({
    queryKey: goalRunKeys.detail(id ?? ''),
    queryFn: () => api.get<GoalRunDetail>(`/api/goal-runs/${encodeURIComponent(id!)}`),
    enabled: !!id,
    staleTime: 5_000,
    refetchInterval: 10_000,
  });
}

export function useCreateGoalRun() {
  const qc = useQueryClient();
  return useMutation<CreateGoalRunResponse, Error, CreateGoalRunInput>({
    mutationFn: (input) => api.post<CreateGoalRunResponse>('/api/goal-runs', input),
    onSuccess: (result) => {
      qc.invalidateQueries({ queryKey: goalRunKeys.list() });
      qc.invalidateQueries({ queryKey: goalRunKeys.detail(result.goal_run.id) });
      toast.success(t('goalRunsCreateSuccess'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useConfirmGoalRunCriteria(id?: string | null) {
  const qc = useQueryClient();
  return useMutation<{ goal_run: GoalRun }, Error, ConfirmGoalRunCriteriaInput>({
    mutationFn: (input) =>
      api.post<{ goal_run: GoalRun }>(
        `/api/goal-runs/${encodeURIComponent(id!)}/confirm-criteria`,
        input,
      ),
    onSuccess: (_, variables) => {
      if (!id) {
        return;
      }
      qc.invalidateQueries({ queryKey: goalRunKeys.list() });
      qc.invalidateQueries({ queryKey: goalRunKeys.detail(id) });
      const selectedScope = variables.selected_scope;
      if (selectedScope?.kind === 'daemon') {
        toast.success(t('goalRunsConfirmSuccessDaemon'));
        return;
      }
      toast.success(t('goalRunsConfirmSuccess'));
    },
    onError: (err) => toast.error(err.message),
  });
}

function invalidateGoalRun(qc: ReturnType<typeof useQueryClient>, id?: string | null) {
  qc.invalidateQueries({ queryKey: goalRunKeys.list() });
  if (id) {
    qc.invalidateQueries({ queryKey: goalRunKeys.detail(id) });
  }
}

export function useStartGoalRun(id?: string | null) {
  const qc = useQueryClient();
  return useMutation<{ goal_run: GoalRun }, Error, void>({
    mutationFn: () => api.post<{ goal_run: GoalRun }>(`/api/goal-runs/${encodeURIComponent(id!)}/start`),
    onSuccess: () => {
      invalidateGoalRun(qc, id);
      toast.success(t('goalRunsStartSuccess'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useStopGoalRun(id?: string | null) {
  const qc = useQueryClient();
  return useMutation<{ goal_run: GoalRun }, Error, void>({
    mutationFn: () => api.post<{ goal_run: GoalRun }>(`/api/goal-runs/${encodeURIComponent(id!)}/stop`),
    onSuccess: () => {
      invalidateGoalRun(qc, id);
      toast.success(t('goalRunsStopSuccess'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useCancelGoalRun(id?: string | null) {
  const qc = useQueryClient();
  return useMutation<{ goal_run: GoalRun }, Error, void>({
    mutationFn: () => api.post<{ goal_run: GoalRun }>(`/api/goal-runs/${encodeURIComponent(id!)}/cancel`),
    onSuccess: () => {
      invalidateGoalRun(qc, id);
      toast.success(t('goalRunsCancelSuccess'));
    },
    onError: (err) => toast.error(err.message),
  });
}

export function useConfirmGoalRunManual(id?: string | null) {
  const qc = useQueryClient();
  return useMutation<{ goal_run: GoalRun }, Error, ConfirmGoalRunManualInput>({
    mutationFn: (input) =>
      api.post<{ goal_run: GoalRun }>(`/api/goal-runs/${encodeURIComponent(id!)}/confirm-manual`, input),
    onSuccess: (_, variables) => {
      invalidateGoalRun(qc, id);
      toast.success(
        variables.approved ? t('goalRunsManualConfirmSuccess') : t('goalRunsManualRejectSuccess'),
      );
    },
    onError: (err) => toast.error(err.message),
  });
}
