import { api } from '@/api/client';
import type { ResourceVisibility } from '@/components/common/OwnershipBadge';
import { useQuery } from '@tanstack/react-query';

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

const NOTIFICATION_ROUTES_KEY = ['notification-routes'] as const;

export function useNotificationRoutes() {
  return useQuery<NotificationRoute[]>({
    queryKey: [...NOTIFICATION_ROUTES_KEY],
    queryFn: () => api.get('/api/notification-routes'),
    staleTime: 10_000,
  });
}
