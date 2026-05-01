import { useEffect, useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';
import {
  type ChannelConfig,
  useActivateWechatBinding,
  useChannels,
  useDeleteWechatBindingAccount,
  useDeleteWechatBinding,
  usePollWechatBinding,
  useStartWechatBinding,
  useTestChannel,
  useWechatBindingStatus,
} from '@/hooks/useChannels';
import { useNotificationBindings, useNotificationRoutes, useSetNotificationBindingForTarget } from '@/hooks/useNotificationRoutes';
import { ChannelForm } from '@/components/config/ChannelForm';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';

const channelLogos: Record<string, string> = {
  telegram: '/logos/telegram.svg',
  discord: '/logos/discord.svg',
  slack: '/logos/slack.svg',
  whatsapp: '/logos/whatsapp.svg',
  feishu: '/logos/feishu.svg',
  dingtalk: '/logos/dingtalk.svg',
  qq: '/logos/qq.svg',
  wework: '/logos/wecom.svg',
  wechat: '/logos/weixin.svg',
};

/** Background color per channel (for the badge). */
const channelColors: Record<string, string> = {
  telegram: 'bg-sky-500',
  gotify: 'bg-orange-600',
  discord: 'bg-indigo-500',
  slack: 'bg-emerald-600',
  whatsapp: 'bg-green-500',
  feishu: 'bg-blue-500',
  dingtalk: 'bg-blue-600',
  qq: 'bg-cyan-500',
  wework: 'bg-teal-600',
  serverchan: 'bg-amber-500',
  googlechat: 'bg-green-600',
  maixcam: 'bg-orange-500',
  teams: 'bg-violet-600',
  infoflow: 'bg-rose-500',
  wechat: 'bg-emerald-700',
  email: 'bg-gray-500',
};

export default function ChannelsPage() {
  const { data: channels, isLoading } = useChannels();
  const testChannel = useTestChannel();
  const wechatBinding = useWechatBindingStatus();
  const startWechatBinding = useStartWechatBinding();
  const pollWechatBinding = usePollWechatBinding();
  const deleteWechatBinding = useDeleteWechatBinding();
  const activateWechatBinding = useActivateWechatBinding();
  const deleteWechatBindingAccount = useDeleteWechatBindingAccount();
  const { data: notificationRoutes = [] } = useNotificationRoutes();
  const { data: notificationBindings = [] } = useNotificationBindings();
  const setNotificationBinding = useSetNotificationBindingForTarget();

  const [activeTab, setActiveTab] = useState<string>('all');
  const [editingChannel, setEditingChannel] = useState<string | null>(null);

  useEffect(() => {
    const status = wechatBinding.data?.binding?.status;
    if (!status || status === 'confirmed' || status === 'expired' || status === 'failed') {
      return;
    }

    const timer = window.setTimeout(() => {
      if (!pollWechatBinding.isPending) {
        pollWechatBinding.mutate();
      }
    }, 2500);

    return () => window.clearTimeout(timer);
  }, [wechatBinding.data, pollWechatBinding]);

  // Derive channel list from the map.
  const runtimeInstances = channels?._instances ?? [];
  const channelNotificationTargets = useMemo(() => {
    const targets = [
      {
        target: '#websocket',
        label: t('collaborationTargetWebsocket'),
        description: t('channelNotificationWebsocketHint'),
        enabled: true,
      },
    ];
    for (const instance of runtimeInstances) {
      const target = instance.id.startsWith('#') || instance.id.startsWith('dm:')
        ? instance.id
        : `#${instance.id}`;
      targets.push({
        target,
        label: instance.name || target,
        description: `${instance.type} · ${target}`,
        enabled: instance.enabled,
      });
    }
    return targets;
  }, [runtimeInstances]);
  const notificationBindingMap = useMemo(() => {
    return new Map(
      notificationBindings
        .filter((binding) => binding.scope === 'channel' && binding.enabled)
        .map((binding) => [binding.target, binding.route_id]),
    );
  }, [notificationBindings]);
  const channelConfigs: Record<string, ChannelConfig> = Object.fromEntries(
    Object.entries(channels ?? {}).filter(
      ([name, value]) => name !== '_instances' && value && !Array.isArray(value),
    ),
  ) as Record<string, ChannelConfig>;
  const allChannels = channels
    ? Object.entries(channelConfigs)
        .sort(([a], [b]) => a.localeCompare(b))
    : [];

  const enabledCount = allChannels.filter(([, cfg]) => cfg.enabled).length;

  const filteredChannels =
    activeTab === 'enabled'
      ? allChannels.filter(([, cfg]) => cfg.enabled)
      : allChannels;

  const handleTest = (name: string, e: React.MouseEvent) => {
    e.stopPropagation();
    testChannel.mutate(name);
  };

  const handleChannelNotificationRouteChange = (target: string, value: string) => {
    setNotificationBinding.mutate({
      scope: 'channel',
      target,
      route_id: value === '__none__' ? '' : value,
      event_types: ['web.message'],
    });
  };

  return (
    <div>
      <Header title={t('tabChannels')} description={t('channelsPageDescription')} />

      {/* Tab filter */}
      <Tabs value={activeTab} onValueChange={setActiveTab} className="mb-6">
        <TabsList>
          <TabsTrigger value="all">
            {t('channelsTabAll')} ({allChannels.length})
          </TabsTrigger>
          <TabsTrigger value="enabled">
            {t('channelsTabEnabled')} ({enabledCount})
          </TabsTrigger>
        </TabsList>
      </Tabs>

      {!isLoading && channelNotificationTargets.length > 0 && (
        <Card className="mb-6">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t('channelNotificationBindingsTitle')}</CardTitle>
            <CardDescription>{t('channelNotificationBindingsDescription')}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
              {channelNotificationTargets.map((target) => (
                <div key={target.target} className="rounded-md border border-border/70 bg-background px-3 py-3">
                  <div className="mb-2 flex flex-wrap items-start justify-between gap-2">
                    <div className="min-w-0">
                      <div className="text-sm font-medium">{target.label}</div>
                      <div className="truncate text-xs text-muted-foreground" title={target.description}>
                        {target.description}
                      </div>
                    </div>
                    <span className="rounded-full bg-muted px-2 py-0.5 font-mono text-[11px] text-muted-foreground">
                      {target.target}
                    </span>
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs text-muted-foreground">{t('channelNotificationRouteLabel')}</Label>
                    <Select
                      value={notificationBindingMap.get(target.target) || '__none__'}
                      onValueChange={(value) => handleChannelNotificationRouteChange(target.target, value)}
                      disabled={!target.enabled || setNotificationBinding.isPending}
                    >
                      <SelectTrigger className="h-10 w-full rounded-md">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="__none__">{t('notificationBindingRouteNone')}</SelectItem>
                        {notificationRoutes.filter((route) => route.enabled).map((route) => (
                          <SelectItem key={route.id} value={route.id}>
                            {route.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Loading state */}
      {isLoading && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-36 rounded-xl" />
          ))}
        </div>
      )}

      {/* Channel card grid */}
      {!isLoading && filteredChannels.length > 0 && (
        <div className="space-y-4">
          {runtimeInstances.length > 0 && (
            <Card className="border-primary/15 bg-gradient-to-br from-background to-muted/40">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">{t('channelInstancesTitle')}</CardTitle>
                <CardDescription>{t('channelInstancesDescription')}</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                  {runtimeInstances.map((instance) => (
                    <div
                      key={instance.id}
                      className="rounded-xl border border-border/60 bg-background/80 px-4 py-3"
                    >
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <div className="text-sm font-semibold text-foreground">{instance.name}</div>
                          <div className="text-xs text-muted-foreground">{instance.id}</div>
                        </div>
                        <span
                          className={cn(
                            'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium',
                            instance.enabled
                              ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                              : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400',
                          )}
                        >
                          <span
                            className={cn(
                              'h-1.5 w-1.5 rounded-full',
                              instance.enabled ? 'bg-emerald-500' : 'bg-gray-400',
                            )}
                          />
                          {instance.enabled ? t('on') : t('off')}
                        </span>
                      </div>
                      <div className="mt-3 text-xs text-muted-foreground">
                        {t('channelInstancesTypeLabel')}: <span className="font-medium text-foreground">{instance.type}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {channelConfigs.wechat && (
            <WechatBindingCard
              enabled={Boolean(channelConfigs.wechat.enabled)}
              binding={wechatBinding.data}
              starting={startWechatBinding.isPending}
              polling={pollWechatBinding.isPending}
              deleting={deleteWechatBinding.isPending}
              activating={activateWechatBinding.isPending}
              deletingAccount={deleteWechatBindingAccount.isPending}
              onStart={() => startWechatBinding.mutate()}
              onPoll={() => pollWechatBinding.mutate()}
              onDelete={() => deleteWechatBinding.mutate()}
              onActivateAccount={(accountId) => activateWechatBinding.mutate(accountId)}
              onDeleteAccount={(accountId) => deleteWechatBindingAccount.mutate(accountId)}
              onEdit={() => setEditingChannel('wechat')}
            />
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {filteredChannels.map(([name, config]) => {
            if (name === 'wechat') return null;

            const enabled = config.enabled ?? false;
            const badge = name.slice(0, 2).toUpperCase();
            const badgeColor = channelColors[name] ?? 'bg-gray-400';
            const logo = channelLogos[name];

            return (
              <Card
                key={name}
                className={cn(
                  'cursor-pointer transition-[shadow,border-color] hover:shadow-md hover:border-primary/30',
                  enabled && 'border-primary/20',
                )}
                onClick={() => setEditingChannel(name)}
              >
                <CardContent className="p-5">
                  <div className="flex items-start justify-between mb-3">
                    {/* Channel badge */}
                    {logo ? (
                      <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-slate-200 bg-white p-2 shadow-sm">
                        <img src={logo} alt={name} className="h-full w-full object-contain" />
                      </div>
                    ) : (
                      <div
                        className={cn(
                          'flex h-10 w-10 items-center justify-center rounded-lg text-white text-xs font-bold',
                          badgeColor,
                        )}
                      >
                        {badge}
                      </div>
                    )}

                    {/* Enabled / Disabled indicator */}
                    <span
                      className={cn(
                        'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium',
                        enabled
                          ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                          : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400',
                      )}
                    >
                      <span
                        className={cn(
                          'h-1.5 w-1.5 rounded-full',
                          enabled ? 'bg-emerald-500' : 'bg-gray-400',
                        )}
                      />
                      {enabled ? t('on') : t('off')}
                    </span>
                  </div>

                  {/* Channel name */}
                  <h3 className="text-sm font-semibold capitalize mb-3 text-foreground">
                    {name}
                  </h3>

                  {/* Actions */}
                  <div className="flex items-center gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 text-xs"
                      onClick={(e) => {
                        e.stopPropagation();
                        setEditingChannel(name);
                      }}
                    >
                      {t('edit')}
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="h-7 text-xs"
                      disabled={!enabled || testChannel.isPending}
                      onClick={(e) => handleTest(name, e)}
                    >
                      {testChannel.isPending && testChannel.variables === name
                        ? t('testing')
                        : t('test')}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            );
          })}
          </div>
        </div>
      )}

      {/* Empty state */}
      {!isLoading && filteredChannels.length === 0 && (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <div className="h-14 w-14 flex items-center justify-center rounded-xl bg-muted mb-4">
            <span className="text-2xl text-muted-foreground">#</span>
          </div>
          <h3 className="text-sm font-semibold text-foreground mb-1.5">
            {t('channelsEmptyTitle')}
          </h3>
          <p className="text-sm text-muted-foreground max-w-sm">
            {t('channelsEmptyDescription')}
          </p>
        </div>
      )}

      {/* Channel edit dialog */}
      <ChannelForm
        open={editingChannel !== null}
        channelName={editingChannel}
        channelConfig={editingChannel ? channelConfigs[editingChannel] ?? null : null}
        onClose={() => setEditingChannel(null)}
      />
    </div>
  );
}

interface WechatBindingCardProps {
  enabled: boolean;
  binding?: {
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
  };
  starting: boolean;
  polling: boolean;
  deleting: boolean;
  activating: boolean;
  deletingAccount: boolean;
  onStart: () => void;
  onPoll: () => void;
  onDelete: () => void;
  onActivateAccount: (accountId: string) => void;
  onDeleteAccount: (accountId: string) => void;
  onEdit: () => void;
}

function WechatBindingCard({
  enabled,
  binding,
  starting,
  polling,
  deleting,
  activating,
  deletingAccount,
  onStart,
  onPoll,
  onDelete,
  onActivateAccount,
  onDeleteAccount,
  onEdit,
}: WechatBindingCardProps) {
  const status = binding?.binding?.status ?? 'idle';
  const qrImage = binding?.binding?.qr_png_data_url;
  const canPoll = status === 'pending' || status === 'scanned';
  const canDeleteBinding = Boolean(binding?.bound || binding?.active_account_id);
  const accounts = binding?.accounts ?? [];

  return (
    <Card className="border-emerald-500/20 bg-gradient-to-br from-emerald-50/80 via-background to-background dark:from-emerald-950/20">
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle className="text-base">{t('wechatBindingTitle')}</CardTitle>
            <CardDescription>{t('wechatBindingDescription')}</CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <span
              className={cn(
                'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium',
                enabled
                  ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                  : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400',
              )}
            >
              <span
                className={cn(
                  'h-1.5 w-1.5 rounded-full',
                  enabled ? 'bg-emerald-500' : 'bg-gray-400',
                )}
              />
              {enabled ? t('on') : t('off')}
            </span>
            <Button size="sm" variant="outline" onClick={onEdit}>
              {t('edit')}
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="grid gap-4 lg:grid-cols-[220px_1fr]">
        <div className="rounded-xl border bg-card p-4 flex items-center justify-center min-h-[220px]">
          {qrImage ? (
            <img src={qrImage} alt={t('wechatQrAlt')} className="w-full max-w-[180px] rounded-lg" />
          ) : (
            <div className="text-center text-sm text-muted-foreground">{t('wechatNoQr')}</div>
          )}
        </div>

        <div className="space-y-4">
          <div className="grid gap-2 text-sm">
            <div>
              <span className="text-muted-foreground">{t('wechatBindStatusLabel')}</span>{' '}
              <span className="font-medium">{t(`wechatBindStatus_${status}`)}</span>
            </div>
            <div>
              <span className="text-muted-foreground">{t('wechatBoundAccountLabel')}</span>{' '}
              <span className="font-medium">{binding?.account?.bot_id ?? t('wechatNoBoundAccount')}</span>
            </div>
            {binding?.account?.user_id && (
              <div>
                <span className="text-muted-foreground">{t('wechatBoundUserLabel')}</span>{' '}
                <span className="font-medium">{binding.account.user_id}</span>
              </div>
            )}
            {binding?.binding?.error && (
              <div className="text-sm text-destructive">{binding.binding.error}</div>
            )}
          </div>

          {accounts.length > 0 && (
            <div className="space-y-2">
              <div className="text-sm font-medium">{t('wechatAccountsTitle')}</div>
              <div className="space-y-2">
                {accounts.map((account) => {
                  const accountId = account.account_id ?? '';
                  const active = Boolean(account.active);
                  return (
                    <div
                      key={accountId}
                      className="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-card/60 px-3 py-2 text-sm"
                    >
                      <div className="space-y-1">
                        <div className="font-medium">{account.bot_id ?? accountId}</div>
                        {account.user_id ? (
                          <div className="text-muted-foreground">{account.user_id}</div>
                        ) : null}
                      </div>
                      <div className="flex items-center gap-2">
                        {active ? (
                          <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">
                            {t('wechatAccountActive')}
                          </span>
                        ) : (
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={activating || !accountId}
                            onClick={() => onActivateAccount(accountId)}
                          >
                            {t('wechatActivateAccount')}
                          </Button>
                        )}
                        <Button
                          size="sm"
                          variant="ghost"
                          disabled={deletingAccount || !accountId}
                          onClick={() => onDeleteAccount(accountId)}
                        >
                          {t('wechatDeleteAccount')}
                        </Button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          <div className="flex flex-wrap gap-2">
            <Button onClick={onStart} disabled={starting}>
              {starting ? t('wechatStartingBind') : t('wechatStartBind')}
            </Button>
            <Button variant="outline" onClick={onPoll} disabled={!canPoll || polling}>
              {polling ? t('wechatPollingBind') : t('wechatRefreshBind')}
            </Button>
            <Button variant="ghost" onClick={onDelete} disabled={!canDeleteBinding || deleting}>
              {deleting ? t('wechatDeletingBind') : t('wechatDeleteBind')}
            </Button>
          </div>

          <p className="text-xs text-muted-foreground">{t('wechatMultiAccountHint')}</p>
        </div>
      </CardContent>
    </Card>
  );
}
