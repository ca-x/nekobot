import { useMemo, useState } from 'react';
import { BellRing, Edit3, Plus, Route, Trash2 } from 'lucide-react';

import { OwnershipBadge, normalizeVisibility, type ResourceVisibility } from '@/components/common/OwnershipBadge';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  type NotificationRoute,
  type NotificationRouteInput,
  useCreateNotificationRoute,
  useDeleteNotificationRoute,
  useNotificationRoutes,
  useUpdateNotificationRoute,
} from '@/hooks/useNotificationRoutes';
import { useChannelAccounts } from '@/hooks/useTopology';
import { t } from '@/lib/i18n';
import { toast } from '@/lib/notify';
import { cn } from '@/lib/utils';

type RouteDialogState = {
  id?: string;
  name: string;
  description: string;
  enabled: boolean;
  channel_account_id: string;
  target_config_json: string;
  visibility: ResourceVisibility;
};

function emptyRouteState(): RouteDialogState {
  return {
    name: '',
    description: '',
    enabled: true,
    channel_account_id: '',
    target_config_json: '{\n  "target": ""\n}',
    visibility: 'shared',
  };
}

function routeStateFromRecord(route: NotificationRoute): RouteDialogState {
  return {
    id: route.id,
    name: route.name,
    description: route.description,
    enabled: route.enabled,
    channel_account_id: route.channel_account_id,
    target_config_json: formatJSON(route.target_config_json),
    visibility: normalizeVisibility(route.visibility),
  };
}

function formatJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw || '{}'), null, 2);
  } catch {
    return raw || '{}';
  }
}

function accountLabel(accountID: string, accountMap: Map<string, { label: string }>): string {
  return accountMap.get(accountID)?.label ?? (accountID || '-');
}

function parseRouteInput(state: RouteDialogState): NotificationRouteInput | null {
  const name = state.name.trim();
  const channelAccountID = state.channel_account_id.trim();
  if (!name) {
    toast.error(t('notificationRoutesNameRequired'));
    return null;
  }
  if (!channelAccountID) {
    toast.error(t('notificationRoutesAccountRequired'));
    return null;
  }
  let targetConfig = '{}';
  try {
    targetConfig = JSON.stringify(JSON.parse(state.target_config_json || '{}'));
  } catch {
    toast.error(t('notificationRoutesInvalidTargetConfig'));
    return null;
  }
  return {
    name,
    description: state.description.trim(),
    enabled: state.enabled,
    channel_account_id: channelAccountID,
    target_config_json: targetConfig,
    visibility: state.visibility,
  };
}

export default function NotificationRoutesPage() {
  const routesQuery = useNotificationRoutes();
  const accountsQuery = useChannelAccounts();
  const createRoute = useCreateNotificationRoute();
  const updateRoute = useUpdateNotificationRoute();
  const deleteRoute = useDeleteNotificationRoute();

  const routes = routesQuery.data ?? [];
  const accounts = accountsQuery.data ?? [];
  const enabledAccounts = accounts.filter((account) => account.enabled);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<NotificationRoute | null>(null);
  const [state, setState] = useState<RouteDialogState>(emptyRouteState());

  const accountMap = useMemo(() => {
    return new Map(
      accounts.map((account) => [
        account.id,
        {
          label:
            account.display_name.trim() ||
            `${account.channel_type}:${account.account_key}`,
        },
      ]),
    );
  }, [accounts]);

  const routeStats = useMemo(() => {
    const enabled = routes.filter((route) => route.enabled).length;
    return {
      total: routes.length,
      enabled,
      disabled: Math.max(routes.length - enabled, 0),
    };
  }, [routes]);

  const isMutating = createRoute.isPending || updateRoute.isPending || deleteRoute.isPending;
  const saveDisabled =
    isMutating ||
    !state.name.trim() ||
    !state.channel_account_id.trim();

  function openCreateDialog() {
    setState(emptyRouteState());
    setDialogOpen(true);
  }

  function openEditDialog(route: NotificationRoute) {
    setState(routeStateFromRecord(route));
    setDialogOpen(true);
  }

  async function saveRoute() {
    const input = parseRouteInput(state);
    if (!input) {
      return;
    }
    if (state.id) {
      await updateRoute.mutateAsync({ id: state.id, input });
    } else {
      await createRoute.mutateAsync(input);
    }
    setDialogOpen(false);
    setState(emptyRouteState());
  }

  async function confirmDelete() {
    if (!deleteTarget) {
      return;
    }
    await deleteRoute.mutateAsync(deleteTarget.id);
    setDeleteTarget(null);
  }

  return (
    <>
      <div className="flex h-[calc(100dvh-4rem)] flex-col overflow-hidden">
        <Header title={t('tabNotifications')} />

        <div className="grid min-h-0 flex-1 gap-4 xl:grid-cols-[minmax(0,1fr)_340px]">
          <Card className="flex min-h-0 flex-col overflow-hidden border-border/70 bg-card/88 shadow-[0_20px_60px_-42px_rgba(60,90,120,0.28)]">
            <div className="flex flex-col gap-4 border-b border-border/70 bg-[linear-gradient(135deg,hsl(var(--card)/0.98),hsl(var(--muted)/0.74))] p-5 md:flex-row md:items-center md:justify-between">
              <div>
                <div className="inline-flex items-center gap-2 rounded-full bg-primary/10 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.2em] text-primary">
                  <BellRing className="h-3.5 w-3.5" />
                  {t('notificationRoutesBadge')}
                </div>
                <h2 className="mt-3 text-xl font-semibold text-foreground">{t('notificationRoutesTitle')}</h2>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">
                  {t('notificationRoutesDescription')}
                </p>
              </div>
              <Button className="rounded-full" onClick={openCreateDialog}>
                <Plus className="mr-2 h-4 w-4" />
                {t('notificationRoutesCreate')}
              </Button>
            </div>

            <div className="custom-scrollbar min-h-0 flex-1 space-y-3 overflow-y-auto p-4">
              {routesQuery.isLoading ? (
                <div className="rounded-2xl border border-border/70 bg-muted/40 p-5 text-sm text-muted-foreground">
                  {t('loading')}
                </div>
              ) : routes.length === 0 ? (
                <div className="flex min-h-[260px] flex-col items-center justify-center rounded-2xl border border-dashed border-border bg-muted/35 p-8 text-center">
                  <Route className="h-10 w-10 text-muted-foreground" />
                  <div className="mt-4 text-lg font-semibold text-foreground">{t('notificationRoutesEmptyTitle')}</div>
                  <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
                    {t('notificationRoutesEmptyDescription')}
                  </p>
                  <Button className="mt-5 rounded-full" onClick={openCreateDialog}>
                    <Plus className="mr-2 h-4 w-4" />
                    {t('notificationRoutesCreate')}
                  </Button>
                </div>
              ) : (
                routes.map((route) => (
                  <div
                    key={route.id}
                    className="rounded-2xl border border-border/70 bg-card/95 p-4 shadow-[0_16px_40px_-34px_rgba(15,23,42,0.48)]"
                  >
                    <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <div className="truncate text-base font-semibold text-foreground">{route.name}</div>
                          <span
                            className={cn(
                              'rounded-full px-2.5 py-1 text-[11px] font-medium',
                              route.enabled
                                ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200'
                                : 'bg-muted text-muted-foreground',
                            )}
                          >
                            {route.enabled ? t('enabled') : t('disabled')}
                          </span>
                          <OwnershipBadge resource={route} />
                        </div>
                        {route.description ? (
                          <p className="mt-2 text-sm leading-6 text-muted-foreground">{route.description}</p>
                        ) : null}
                      </div>
                      <div className="flex shrink-0 gap-2">
                        <Button variant="outline" size="sm" className="rounded-full" onClick={() => openEditDialog(route)}>
                          <Edit3 className="mr-2 h-4 w-4" />
                          {t('edit')}
                        </Button>
                        <Button variant="destructive" size="sm" className="rounded-full" onClick={() => setDeleteTarget(route)}>
                          <Trash2 className="mr-2 h-4 w-4" />
                          {t('delete')}
                        </Button>
                      </div>
                    </div>

                    <div className="mt-4 grid gap-3 md:grid-cols-2">
                      <RouteMeta label={t('notificationRoutesChannelAccount')} value={accountLabel(route.channel_account_id, accountMap)} />
                      <RouteMeta label={t('notificationRoutesTargetConfig')} value={formatJSON(route.target_config_json)} mono />
                    </div>
                  </div>
                ))
              )}
            </div>
          </Card>

          <div className="space-y-4">
            <Card className="border-border/70 bg-card/88">
              <CardContent className="space-y-4 p-5">
                <div className="inline-flex items-center gap-2 rounded-full bg-primary/10 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-primary">
                  <Route className="h-3.5 w-3.5" />
                  {t('notificationRoutesOverview')}
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <Metric label={t('notificationRoutesTotal')} value={String(routeStats.total)} />
                  <Metric label={t('enabled')} value={String(routeStats.enabled)} />
                  <Metric label={t('disabled')} value={String(routeStats.disabled)} />
                </div>
                <p className="text-sm leading-6 text-muted-foreground">{t('notificationRoutesOverviewDescription')}</p>
              </CardContent>
            </Card>

            <Card className="border-border/70 bg-card/88">
              <CardContent className="space-y-3 p-5">
                <div className="text-sm font-semibold text-foreground">{t('notificationRoutesTargetHelpTitle')}</div>
                <p className="text-sm leading-6 text-muted-foreground">{t('notificationRoutesTargetHelpDescription')}</p>
                <pre className="overflow-x-auto rounded-xl bg-muted/70 p-3 text-xs text-muted-foreground">
{`{
  "target": "123456",
  "title": "Ops alerts"
}`}
                </pre>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{state.id ? t('notificationRoutesEditTitle') : t('notificationRoutesCreateTitle')}</DialogTitle>
            <DialogDescription>{t('notificationRoutesDialogDescription')}</DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label>{t('notificationRoutesName')}</Label>
              <Input value={state.name} onChange={(event) => setState((prev) => ({ ...prev, name: event.target.value }))} />
            </div>
            <div className="space-y-2">
              <Label>{t('notificationRoutesChannelAccount')}</Label>
              <Select
                value={state.channel_account_id || '__none__'}
                onValueChange={(value) => setState((prev) => ({ ...prev, channel_account_id: value === '__none__' ? '' : value }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">{t('notificationRoutesSelectAccount')}</SelectItem>
                  {(state.enabled ? enabledAccounts : accounts).map((account) => (
                    <SelectItem key={account.id} value={account.id}>
                      {account.display_name || `${account.channel_type}:${account.account_key}`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2 md:col-span-2">
              <Label>{t('notificationRoutesDescriptionField')}</Label>
              <Input
                value={state.description}
                onChange={(event) => setState((prev) => ({ ...prev, description: event.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <Label>{t('notificationRoutesVisibility')}</Label>
              <Select
                value={state.visibility}
                onValueChange={(value) => setState((prev) => ({ ...prev, visibility: normalizeVisibility(value) }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="private">{t('visibilityPrivate')}</SelectItem>
                  <SelectItem value="shared">{t('visibilityShared')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <label className="flex items-center justify-between rounded-xl border border-border/70 px-4 py-3">
              <span>
                <span className="block text-sm font-medium text-foreground">{t('enabled')}</span>
                <span className="block text-xs text-muted-foreground">{t('notificationRoutesEnabledHint')}</span>
              </span>
              <Switch checked={state.enabled} onCheckedChange={(enabled) => setState((prev) => ({ ...prev, enabled }))} />
            </label>
            <div className="space-y-2 md:col-span-2">
              <Label>{t('notificationRoutesTargetConfig')}</Label>
              <Textarea
                className="min-h-[150px] font-mono text-xs"
                value={state.target_config_json}
                onChange={(event) => setState((prev) => ({ ...prev, target_config_json: event.target.value }))}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              {t('cancel')}
            </Button>
            <Button onClick={saveRoute} disabled={saveDisabled}>
              {t('save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={deleteTarget !== null} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('notificationRoutesDeleteTitle')}</DialogTitle>
            <DialogDescription>{t('notificationRoutesDeleteDescription')}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              {t('cancel')}
            </Button>
            <Button variant="destructive" onClick={confirmDelete} disabled={deleteRoute.isPending}>
              {t('delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

function RouteMeta({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-xl bg-muted/45 px-3 py-2">
      <div className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">{label}</div>
      <div className={cn('mt-1 break-all text-sm text-foreground', mono && 'whitespace-pre-wrap font-mono text-xs')}>
        {value || '-'}
      </div>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl bg-muted/45 px-3 py-2">
      <div className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">{label}</div>
      <div className="mt-1 text-lg font-semibold text-foreground">{value}</div>
    </div>
  );
}
