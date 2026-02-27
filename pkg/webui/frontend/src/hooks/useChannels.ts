import { api } from '@/api/client';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { t } from '@/lib/i18n';

/** Each channel is a flat object with `enabled` plus channel-specific fields. */
export type ChannelConfig = Record<string, unknown> & { enabled?: boolean };

/** GET /api/channels returns { channelName: ChannelConfig, ... } */
export type ChannelsMap = Record<string, ChannelConfig>;

export interface TestChannelResult {
  channel: string;
  id: string;
  enabled: boolean;
  reachable: boolean;
  status: string;
  error?: string;
}

export function useChannels() {
  return useQuery<ChannelsMap>({
    queryKey: ['channels'],
    queryFn: () => api.get('/api/channels'),
    staleTime: 30_000,
  });
}

export function useUpdateChannel() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ name, data }: { name: string; data: Record<string, unknown> }) =>
      api.put(`/api/channels/${encodeURIComponent(name)}`, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['channels'] });
      toast.success(t('channelSaved'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useTestChannel() {
  return useMutation({
    mutationFn: (name: string) =>
      api.post<TestChannelResult>(`/api/channels/${encodeURIComponent(name)}/test`),
    onSuccess: (result) => {
      if (result?.reachable) {
        toast.success(t('channelTestOk', result.channel));
      } else {
        toast.warning(t('channelTestFail', result?.status ?? 'unknown'));
      }
    },
    onError: (err: Error) => toast.error(err.message),
  });
}
