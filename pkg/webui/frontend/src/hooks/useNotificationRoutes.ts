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

export interface NotificationBinding {
  id: string;
  scope: string;
  target: string;
  route_id: string;
  event_types_json: string;
  enabled: boolean;
  tenant_id?: string;
  owner_user_id?: string;
  visibility?: ResourceVisibility;
  created_at: string;
  updated_at: string;
}

export interface NotificationBindingTargetInput {
  scope: string;
  target: string;
  route_id: string;
  enabled?: boolean;
  event_types?: string[];
}

const NOTIFICATION_ROUTES_KEY = ['notification-routes'] as const;
const NOTIFICATION_BINDINGS_KEY = ['notification-bindings'] as const;

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

export function useNotificationBindings() {
  return useQuery<NotificationBinding[]>({
    queryKey: [...NOTIFICATION_BINDINGS_KEY],
    queryFn: () => api.get('/api/notification-bindings'),
    staleTime: 10_000,
  });
}

export function useSetNotificationBindingForTarget() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: NotificationBindingTargetInput) =>
      api.put<{ binding: NotificationBinding | null }>('/api/notification-bindings/by-target', input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...NOTIFICATION_BINDINGS_KEY] });
      qc.invalidateQueries({ queryKey: ['threads'] });
      toast.success(t('saved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}
