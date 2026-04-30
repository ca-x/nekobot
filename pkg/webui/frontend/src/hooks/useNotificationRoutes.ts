import { api } from '@/api/client';
import type { ResourceVisibility } from '@/components/common/OwnershipBadge';
import { t } from '@/lib/i18n';
import { toast } from '@/lib/notify';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

export interface NotificationRoute {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
  channel_account_id: string;
  target_config_json: string;
  tenant_id?: string;
  owner_user_id?: string;
  visibility?: ResourceVisibility;
  created_at: string;
  updated_at: string;
}

export interface NotificationRouteInput {
  name: string;
  description: string;
  enabled: boolean;
  channel_account_id: string;
  target_config_json: string;
  visibility?: ResourceVisibility;
}

const NOTIFICATION_ROUTES_KEY = ['notification-routes'] as const;

export function useNotificationRoutes() {
  return useQuery<NotificationRoute[]>({
    queryKey: [...NOTIFICATION_ROUTES_KEY],
    queryFn: () => api.get('/api/notification-routes'),
    staleTime: 10_000,
  });
}

export function useCreateNotificationRoute() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: NotificationRouteInput) => api.post<NotificationRoute>('/api/notification-routes', input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...NOTIFICATION_ROUTES_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useUpdateNotificationRoute() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: NotificationRouteInput }) =>
      api.put<NotificationRoute>(`/api/notification-routes/${encodeURIComponent(id)}`, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...NOTIFICATION_ROUTES_KEY] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeleteNotificationRoute() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/notification-routes/${encodeURIComponent(id)}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...NOTIFICATION_ROUTES_KEY] });
      toast.success(t('deleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}
