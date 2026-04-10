import { api } from '@/api/client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { t } from '@/lib/i18n';

export interface WebhookConfig {
  enabled: boolean;
  path: string;
}

export interface WebhookTestResult {
  status: string;
  reply: string;
  session_id: string;
}

export function useSaveWebhookConfig() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (webhook: WebhookConfig) =>
      api.put<{ status: string }>('/api/config', { webhook }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['config'] });
      toast.success(t('configSaved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useTestWebhook() {
  return useMutation({
    mutationFn: (message: string) =>
      api.post<WebhookTestResult>('/api/webhooks/test', { message }),
    onError: (err: Error) => toast.error(err.message),
  });
}
