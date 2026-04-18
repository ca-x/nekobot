import { api } from '@/api/client';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from '@/lib/notify';
import { t } from '@/lib/i18n';

/** Each channel is a flat object with `enabled` plus channel-specific fields. */
export type ChannelConfig = Record<string, unknown> & { enabled?: boolean };

/** GET /api/channels returns { channelName: ChannelConfig, ... } */
export type ChannelsMap = Record<string, ChannelConfig>;

export interface ChannelRuntimeInstance {
  id: string;
  type: string;
  name: string;
  enabled: boolean;
}

export interface ChannelsResponse {
  _instances?: ChannelRuntimeInstance[];
  [key: string]: ChannelConfig | ChannelRuntimeInstance[] | undefined;
}

export interface WechatBindingStatus {
  bound: boolean;
  active_account_id?: string;
  account?: {
    bot_id?: string;
    user_id?: string;
  };
  accounts?: Array<{
    account_id?: string;
    bot_id?: string;
    user_id?: string;
    active?: boolean;
  }>;
  binding?: {
    status?: string;
    qrcode_content?: string;
    qr_png_data_url?: string;
    updated_at?: string;
    bot_id?: string;
    user_id?: string;
    error?: string;
  };
}

export interface TestChannelResult {
  channel: string;
  id: string;
  enabled: boolean;
  reachable: boolean;
  status: string;
  error?: string;
}

export function useChannels() {
  return useQuery<ChannelsResponse>({
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
      if (result?.status === 'ok' && result?.reachable) {
        toast.success(t('channelTestOk', result.channel));
        return;
      }
      if (result?.status === 'configured') {
        toast.info(t('channelTestConfigured', result.channel));
        return;
      }
      if (result?.status === 'disabled') {
        toast.warning(t('channelTestDisabled', result.channel));
        return;
      }
      toast.warning(t('channelTestFail', result?.error ?? result?.status ?? 'unknown'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useWechatBindingStatus() {
  return useQuery<WechatBindingStatus>({
    queryKey: ['channels', 'wechat', 'binding'],
    queryFn: () => api.get('/api/channels/wechat/binding'),
    staleTime: 5_000,
  });
}

export function useStartWechatBinding() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<WechatBindingStatus>('/api/channels/wechat/binding/start'),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['channels', 'wechat', 'binding'] });
      qc.invalidateQueries({ queryKey: ['channel-accounts'] });
      qc.invalidateQueries({ queryKey: ['runtime-topology'] });
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function usePollWechatBinding() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<WechatBindingStatus>('/api/channels/wechat/binding/poll'),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['channels', 'wechat', 'binding'] });
      qc.invalidateQueries({ queryKey: ['channels'] });
      qc.invalidateQueries({ queryKey: ['channel-accounts'] });
      qc.invalidateQueries({ queryKey: ['runtime-topology'] });
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeleteWechatBinding() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.delete('/api/channels/wechat/binding'),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['channels', 'wechat', 'binding'] });
      qc.invalidateQueries({ queryKey: ['channels'] });
      qc.invalidateQueries({ queryKey: ['channel-accounts'] });
      qc.invalidateQueries({ queryKey: ['runtime-topology'] });
      toast.success(t('wechatBindingDeleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useActivateWechatBinding() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (accountId: string) =>
      api.post<WechatBindingStatus>('/api/channels/wechat/binding/activate', {
        account_id: accountId,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['channels', 'wechat', 'binding'] });
      qc.invalidateQueries({ queryKey: ['channels'] });
      qc.invalidateQueries({ queryKey: ['channel-accounts'] });
      qc.invalidateQueries({ queryKey: ['runtime-topology'] });
      toast.success(t('wechatAccountActivated'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}

export function useDeleteWechatBindingAccount() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (accountId: string) =>
      api.delete(`/api/channels/wechat/binding/accounts/${encodeURIComponent(accountId)}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['channels', 'wechat', 'binding'] });
      qc.invalidateQueries({ queryKey: ['channels'] });
      qc.invalidateQueries({ queryKey: ['channel-accounts'] });
      qc.invalidateQueries({ queryKey: ['runtime-topology'] });
      toast.success(t('wechatAccountDeleted'));
    },
    onError: (err: Error) => toast.error(err.message),
  });
}
