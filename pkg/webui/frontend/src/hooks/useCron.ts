import { api } from '@/api/client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { t } from '@/lib/i18n';

export type CronScheduleKind = 'cron' | 'at' | 'every';

export interface CronJob {
  id: string;
  name: string;
  schedule_kind: CronScheduleKind;
  schedule?: string;
  at_time?: string;
  every_duration?: string;
  prompt: string;
  enabled: boolean;
  delete_after_run?: boolean;
  created_at: string;
  last_run?: string;
  next_run?: string;
  run_count: number;
  last_error: string;
  last_success: boolean;
}

export interface CreateCronJobInput {
  name: string;
  schedule_kind: CronScheduleKind;
  schedule?: string;
  at_time?: string;
  every_duration?: string;
  prompt: string;
  delete_after_run?: boolean;
}

const CRON_KEY = ['cron', 'jobs'] as const;

export function useCronJobs() {
  return useQuery<CronJob[]>({
    queryKey: [...CRON_KEY],
    queryFn: () => api.get('/api/cron/jobs'),
    staleTime: 5_000,
    refetchInterval: 10_000,
  });
}

export function useCreateCronJob() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateCronJobInput) =>
      api.post<{ status: string; job: CronJob }>('/api/cron/jobs', input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...CRON_KEY] });
      toast.success(t('cronCreated'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeleteCronJob() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/cron/jobs/${encodeURIComponent(id)}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...CRON_KEY] });
      toast.success(t('deleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useEnableCronJob() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/api/cron/jobs/${encodeURIComponent(id)}/enable`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...CRON_KEY] });
      toast.success(t('cronEnabled'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDisableCronJob() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/api/cron/jobs/${encodeURIComponent(id)}/disable`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...CRON_KEY] });
      toast.success(t('cronDisabled'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useRunCronJob() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/api/cron/jobs/${encodeURIComponent(id)}/run`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...CRON_KEY] });
      toast.success(t('cronRunStarted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}
