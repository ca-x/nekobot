import { useEffect, useMemo, useState, type ReactNode } from 'react';
import Header from '@/components/layout/Header';
import { ProviderForm } from '@/components/config/ProviderForm';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useConfig, useSaveConfig } from '@/hooks/useConfig';
import {
  useClearProviderCooldown,
  useProviderRuntime,
  useProviders,
  type Provider,
  type ProviderRuntime,
} from '@/hooks/useProviders';
import { useProviderTypes } from '@/hooks/useProviderTypes';
import { getProviderLogo } from '@/lib/provider-logos';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { toast } from '@/lib/notify';
import {
  ArrowUpRight,
  BadgeCheck,
  CircleAlert,
  GitBranch,
  Globe,
  KeyRound,
  Plus,
  Search,
  ShieldCheck,
  Sparkles,
  TimerReset,
  Waypoints,
} from 'lucide-react';

const KIND_TINTS: Record<string, string> = {
  openai: 'from-emerald-500/18 via-emerald-500/8 to-transparent',
  anthropic: 'from-amber-500/18 via-orange-500/10 to-transparent',
  gemini: 'from-sky-500/18 via-indigo-500/10 to-transparent',
  openrouter: 'from-violet-500/18 via-indigo-500/10 to-transparent',
  groq: 'from-rose-500/18 via-orange-500/8 to-transparent',
  deepseek: 'from-cyan-500/18 via-blue-500/8 to-transparent',
  moonshot: 'from-slate-500/18 via-slate-400/8 to-transparent',
  zhipu: 'from-blue-500/18 via-blue-400/8 to-transparent',
  vllm: 'from-red-500/18 via-orange-500/8 to-transparent',
  generic: 'from-stone-500/15 via-stone-400/8 to-transparent',
};

type ProviderGroupStrategy = 'round_robin' | 'least_used' | 'random';

interface ProviderGroup {
  name: string;
  strategy: ProviderGroupStrategy;
  members: string[];
}

interface ProviderGroupDraft extends ProviderGroup {
  id: string;
}

const PROVIDER_GROUP_STRATEGIES: Array<{
  value: ProviderGroupStrategy;
  labelKey: string;
  descriptionKey: string;
}> = [
  { value: 'round_robin', labelKey: 'providerGroupStrategyRoundRobin', descriptionKey: 'providerGroupStrategyRoundRobinDescription' },
  { value: 'least_used', labelKey: 'providerGroupStrategyLeastUsed', descriptionKey: 'providerGroupStrategyLeastUsedDescription' },
  { value: 'random', labelKey: 'providerGroupStrategyRandom', descriptionKey: 'providerGroupStrategyRandomDescription' },
];

function getKindTint(kind: string): string {
  return KIND_TINTS[kind.trim().toLowerCase()] ?? KIND_TINTS.generic;
}

function getConnectionState(provider: Provider, runtime?: ProviderRuntime) {
  if (!provider.enabled) {
    return { label: t('disabled'), tone: 'bg-slate-100 text-slate-700 border-slate-200/70', dot: 'bg-slate-400' };
  }
  if (!provider.api_key_set) {
    return { label: t('providerStateCredentialsMissing'), tone: 'bg-amber-50 text-amber-700 border-amber-200/70', dot: 'bg-amber-500' };
  }
  if (runtime?.in_cooldown) {
    return { label: t('providerRuntimeCooldown'), tone: 'bg-amber-50 text-amber-800 border-amber-200/70', dot: 'bg-amber-500' };
  }
  if (runtime && !runtime.available) {
    return { label: t('providerRuntimeUnavailable'), tone: 'bg-rose-50 text-rose-700 border-rose-200/70', dot: 'bg-rose-500' };
  }
  return { label: t('wsConnected'), tone: 'bg-emerald-50 text-emerald-700 border-emerald-200/70', dot: 'bg-emerald-500' };
}

export default function ProvidersPage() {
  const { data: providers, isLoading } = useProviders();
  const { data: providerRuntime = [] } = useProviderRuntime();
  const { data: providerTypes = [] } = useProviderTypes();
  const { data: runtimeConfig } = useConfig();
  const saveConfig = useSaveConfig();
  const clearProviderCooldown = useClearProviderCooldown();

  const [formOpen, setFormOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null);
  const [runtimeDetailProvider, setRuntimeDetailProvider] = useState<ProviderRuntime | null>(null);
  const [query, setQuery] = useState('');
  const [providerGroups, setProviderGroups] = useState<ProviderGroupDraft[]>([]);

  const providerTypeMap = useMemo(() => new Map(providerTypes.map((item) => [item.id, item])), [providerTypes]);
  const runtimeMap = useMemo(() => new Map(providerRuntime.map((item) => [item.name, item])), [providerRuntime]);
  const providerNames = useMemo(() => (providers ?? []).map((provider) => provider.name), [providers]);

  const filteredProviders = useMemo(() => {
    const items = providers ?? [];
    const keyword = query.trim().toLowerCase();
    if (!keyword) {
      return items;
    }
    return items.filter((provider) =>
      [provider.name, provider.provider_kind, provider.api_base, provider.proxy, provider.summary]
        .join(' ')
        .toLowerCase()
        .includes(keyword),
    );
  }, [providers, query]);

  const readyCount = (providers ?? []).filter((provider) => provider.api_key_set && provider.enabled).length;
  const routingDefault = (providers ?? []).find((provider) => provider.is_routing_default) ?? null;
  const discoveryCount = (providers ?? []).filter((provider) => provider.supports_discovery && provider.api_key_set && provider.enabled).length;

  useEffect(() => {
    const agents = runtimeConfig?.agents;
    if (!agents || typeof agents !== 'object') {
      setProviderGroups([]);
      return;
    }
    const defaults = (agents as Record<string, unknown>).defaults;
    if (!defaults || typeof defaults !== 'object') {
      setProviderGroups([]);
      return;
    }
    const rawGroups = (defaults as Record<string, unknown>).provider_groups;
    if (!Array.isArray(rawGroups)) {
      setProviderGroups([]);
      return;
    }
    setProviderGroups(rawGroups.map((group) => normalizeProviderGroup(group)).filter((group): group is ProviderGroupDraft => group !== null));
  }, [runtimeConfig]);

  const providerGroupValidationError = useMemo(() => validateProviderGroups(providerGroups), [providerGroups]);

  const openNew = () => {
    setEditingProvider(null);
    setFormOpen(true);
  };

  const openEdit = (provider: Provider) => {
    setEditingProvider(provider);
    setFormOpen(true);
  };

  const handleFormOpenChange = (open: boolean) => {
    setFormOpen(open);
    if (!open) {
      setEditingProvider(null);
    }
  };

  const handleAddGroup = () => {
    const nextIndex = providerGroups.length + 1;
    setProviderGroups((current) => [
      ...current,
      { id: createProviderGroupId(), name: `provider-pool-${nextIndex}`, strategy: 'round_robin', members: [] },
    ]);
  };

  const handleRemoveGroup = (id: string) => {
    setProviderGroups((current) => current.filter((group) => group.id !== id));
  };

  const handleGroupNameChange = (id: string, nextName: string) => {
    setProviderGroups((current) => current.map((group) => (group.id === id ? { ...group, name: nextName } : group)));
  };

  const handleGroupStrategyChange = (id: string, strategy: ProviderGroupStrategy) => {
    setProviderGroups((current) => current.map((group) => (group.id === id ? { ...group, strategy } : group)));
  };

  const handleToggleGroupMember = (id: string, providerName: string) => {
    setProviderGroups((current) =>
      current.map((group) => {
        if (group.id !== id) return group;
        const exists = group.members.includes(providerName);
        return { ...group, members: exists ? group.members.filter((member) => member !== providerName) : [...group.members, providerName] };
      }),
    );
  };

  const handleSaveGroups = () => {
    const agents = runtimeConfig?.agents;
    if (!agents || typeof agents !== 'object') {
      toast.error(t('providerGroupsConfigUnavailable'));
      return;
    }
    const defaults = (agents as Record<string, unknown>).defaults;
    if (!defaults || typeof defaults !== 'object') {
      toast.error(t('providerGroupsConfigUnavailable'));
      return;
    }
    if (providerGroupValidationError) {
      toast.error(providerGroupValidationError);
      return;
    }
    const sanitizedGroups = providerGroups.map((group) => ({
      name: group.name.trim(),
      strategy: group.strategy,
      members: group.members.filter((member) => member.trim().length > 0),
    }));
    saveConfig.mutate({
      agents: {
        ...(agents as Record<string, unknown>),
        defaults: {
          ...(defaults as Record<string, unknown>),
          provider_groups: sanitizedGroups,
        },
      },
    });
  };

  return (
    <div className="providers-page space-y-6">
      <Header title={t('tabProviders')} description={t('providersHeaderDescription')} />

      <section className="relative overflow-hidden rounded-[28px] border border-border/70 bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.14),_transparent_38%),linear-gradient(135deg,hsl(var(--card)/0.98),hsl(var(--muted)/0.72))] p-5 shadow-sm sm:p-6">
        <div className="absolute right-0 top-0 h-40 w-40 rounded-full bg-sky-100/60 blur-3xl" />
        <div className="relative flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-3">
            <div className="inline-flex items-center gap-2 rounded-full border border-sky-300/40 bg-card/90 px-3 py-1 text-xs font-medium text-sky-700 dark:text-sky-300 shadow-sm">
              <Sparkles className="h-3.5 w-3.5" />
              {t('providersHeroBadge')}
            </div>
            <div className="space-y-2">
              <h2 className="max-w-2xl text-2xl font-semibold tracking-tight text-foreground">{t('providersHeroTitle')}</h2>
              <p className="max-w-2xl text-sm leading-6 text-muted-foreground">{t('providersHeroDescription')}</p>
            </div>
            <div className="flex flex-wrap gap-3">
              <MetricCard label={t('providersMetricCount')} value={String(providers?.length ?? 0)} />
              <MetricCard label={t('providersMetricReady')} value={String(readyCount)} />
              <MetricCard label={t('providerMetricDiscoveryReady')} value={String(discoveryCount)} />
              <MetricCard label={t('providersMetricRoutingDefault')} value={routingDefault?.name ?? t('providersUnset')} muted={!routingDefault} />
            </div>
          </div>

          <div className="flex w-full flex-col gap-3 sm:w-auto sm:min-w-[320px]">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('providersSearchPlaceholder')} className="h-11 rounded-2xl border-border/70 bg-card/90 pl-9 shadow-sm" />
            </div>
            <Button size="sm" onClick={openNew} className="h-11 rounded-2xl px-4">
              <Plus className="mr-1.5 h-4 w-4" />
              {t('newProvider')}
            </Button>
          </div>
        </div>
      </section>

      <section className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <Card className="overflow-hidden rounded-[28px] border-border/70 bg-card/92 shadow-sm">
          <div className="border-b border-border/70 px-5 py-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="inline-flex items-center gap-2 rounded-full border border-violet-200/80 bg-violet-50 px-3 py-1 text-xs font-medium text-violet-700">
                  <GitBranch className="h-3.5 w-3.5" />
                  {t('providerGroupsBadge')}
                </div>
                <h3 className="mt-3 text-lg font-semibold text-foreground">{t('providerGroupsTitle')}</h3>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">{t('providerGroupsDescription')}</p>
              </div>
              <Button size="sm" onClick={handleAddGroup} className="rounded-xl">
                <Plus className="mr-1.5 h-4 w-4" />
                {t('providerGroupAdd')}
              </Button>
            </div>
          </div>

          <div className="space-y-3 p-4">
            {providerGroups.length === 0 && (
              <div className="rounded-[22px] border border-dashed border-slate-300 bg-slate-50/80 px-4 py-8 text-center">
                <p className="text-sm font-medium text-slate-700">{t('providerGroupsEmptyTitle')}</p>
                <p className="mt-2 text-sm leading-6 text-slate-500">{t('providerGroupsEmptyDescription')}</p>
              </div>
            )}

            {providerGroups.map((group) => (
              <div key={group.id} className="rounded-[24px] border border-slate-200/80 bg-white p-4 shadow-sm">
                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                  <div className="min-w-0 flex-1 space-y-3">
                    <div className="space-y-2">
                      <label className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">{t('providerGroupName')}</label>
                      <Input value={group.name} onChange={(event) => handleGroupNameChange(group.id, event.target.value)} className="h-10 rounded-xl" />
                    </div>
                    <div className="space-y-2">
                      <label className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">{t('providerGroupStrategy')}</label>
                      <Select value={group.strategy} onValueChange={(value) => handleGroupStrategyChange(group.id, value as ProviderGroupStrategy)}>
                        <SelectTrigger className="h-10 rounded-xl"><SelectValue /></SelectTrigger>
                        <SelectContent>
                          {PROVIDER_GROUP_STRATEGIES.map((strategy) => (
                            <SelectItem key={strategy.value} value={strategy.value}>{t(strategy.labelKey)}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <p className="text-xs leading-5 text-slate-500">{t(PROVIDER_GROUP_STRATEGIES.find((strategy) => strategy.value === group.strategy)?.descriptionKey ?? 'providerGroupStrategyRoundRobinDescription')}</p>
                    </div>
                  </div>
                  <Button size="sm" variant="outline" onClick={() => handleRemoveGroup(group.id)} className="rounded-xl">{t('delete')}</Button>
                </div>
                <div className="mt-4 space-y-2">
                  <div className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">{t('providerGroupMembers')}</div>
                  <div className="flex flex-wrap gap-2">
                    {providerNames.map((providerName) => {
                      const selected = group.members.includes(providerName);
                      return (
                        <button key={providerName} type="button" onClick={() => handleToggleGroupMember(group.id, providerName)} className={cn('rounded-full border px-3 py-1.5 text-xs font-medium transition-colors', selected ? 'border-violet-300 bg-violet-50 text-violet-700' : 'border-slate-200 bg-slate-50 text-slate-600 hover:border-slate-300 hover:bg-slate-100')}>
                          {providerName}
                        </button>
                      );
                    })}
                  </div>
                </div>
              </div>
            ))}
          </div>

          <div className="border-t border-slate-100 px-5 py-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <p className={cn('text-sm', providerGroupValidationError ? 'text-rose-600' : 'text-slate-500')}>
                {providerGroupValidationError ?? t('providerGroupsSaveHint')}
              </p>
              <Button onClick={handleSaveGroups} disabled={saveConfig.isPending || Boolean(providerGroupValidationError)} className="rounded-xl">
                {saveConfig.isPending ? t('saving') : t('providerGroupsSave')}
              </Button>
            </div>
          </div>
        </Card>

        <Card className="overflow-hidden rounded-[28px] border-slate-200/80 bg-white/95 shadow-sm">
          <div className="border-b border-slate-100 px-5 py-5">
            <div className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">{t('providerOverviewBadge')}</div>
            <h3 className="mt-2 text-lg font-semibold text-slate-950">{t('providerOverviewTitle')}</h3>
          </div>
          <div className="space-y-3 p-4">
            {(providers ?? []).slice(0, 5).map((provider) => {
              const runtime = runtimeMap.get(provider.name);
              const typeMeta = providerTypeMap.get(provider.provider_kind);
              return (
                <div key={`${provider.name}-overview`} className="rounded-[24px] border border-slate-200/80 bg-[linear-gradient(140deg,_rgba(248,250,252,0.95),_rgba(255,255,255,0.98))] p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <h4 className="text-sm font-semibold text-slate-900">{provider.name}</h4>
                      <p className="mt-1 text-xs uppercase tracking-[0.14em] text-slate-400">{typeMeta?.display_name || provider.provider_kind}</p>
                    </div>
                    <span className="rounded-full bg-slate-100 px-2.5 py-1 text-[11px] font-medium text-slate-600">{provider.enabled ? t('enabled') : t('disabled')}</span>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <span className="rounded-full bg-white px-2.5 py-1 text-xs font-medium text-slate-700 shadow-sm">{provider.api_key_set ? t('providerCredentialsSet') : t('providerCredentialsMissing')}</span>
                    <span className="rounded-full bg-white px-2.5 py-1 text-xs font-medium text-slate-700 shadow-sm">{provider.supports_discovery ? t('providerDiscoverLabel') : t('providerPanelManualOnly')}</span>
                    {runtime?.in_cooldown && <span className="rounded-full bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-700 shadow-sm">{t('providerRuntimeCooldown')}</span>}
                  </div>
                </div>
              );
            })}
            {(providers?.length ?? 0) === 0 && <div className="rounded-[22px] border border-dashed border-slate-300 bg-slate-50/80 px-4 py-8 text-center"><p className="text-sm leading-6 text-slate-500">{t('providerGroupsOverviewEmpty')}</p></div>}
          </div>
        </Card>
      </section>

      {isLoading && (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          {Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-[360px] rounded-[24px]" />)}
        </div>
      )}

      {!isLoading && filteredProviders.length > 0 && (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          {filteredProviders.map((provider) => (
            <ProviderPanel
              key={provider.name}
              provider={provider}
              runtime={runtimeMap.get(provider.name)}
              typeMeta={providerTypeMap.get(provider.provider_kind)}
              onClick={() => openEdit(provider)}
              onClearCooldown={(providerName) => clearProviderCooldown.mutate(providerName)}
              onShowRuntimeDetails={(runtime) => setRuntimeDetailProvider(runtime)}
              clearingCooldown={clearProviderCooldown.isPending}
            />
          ))}
        </div>
      )}

      {!isLoading && (providers?.length ?? 0) > 0 && filteredProviders.length === 0 && (
        <div className="rounded-[24px] border border-dashed border-slate-300 bg-slate-50/70 px-6 py-14 text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-white shadow-sm"><Search className="h-5 w-5 text-slate-400" /></div>
          <h3 className="text-sm font-semibold text-slate-900">{t('providersNoMatchTitle')}</h3>
          <p className="mt-1 text-sm text-slate-500">{t('providersNoMatchDescription')}</p>
        </div>
      )}

      {!isLoading && (providers?.length ?? 0) === 0 && (
        <div className="rounded-[28px] border border-dashed border-slate-300 bg-[linear-gradient(180deg,_rgba(255,255,255,0.96),_rgba(248,250,252,0.96))] px-6 py-20 text-center shadow-sm">
          <div className="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-[22px] bg-slate-900 text-white shadow-lg shadow-slate-900/10"><KeyRound className="h-7 w-7" /></div>
          <h3 className="text-lg font-semibold text-slate-900">{t('noProviders')}</h3>
          <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-slate-500">{t('providersEmptyDescription')}</p>
          <Button size="sm" onClick={openNew} className="mt-5 h-10 rounded-xl px-4"><Plus className="mr-1.5 h-4 w-4" />{t('newProvider')}</Button>
        </div>
      )}

      <Dialog open={runtimeDetailProvider !== null} onOpenChange={(open) => !open && setRuntimeDetailProvider(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('providerRuntimeDetailTitle')}</DialogTitle>
            <DialogDescription>{runtimeDetailProvider ? t('providerRuntimeDetailDescription', runtimeDetailProvider.name) : ''}</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            {runtimeDetailProvider && Object.entries(runtimeDetailProvider.failure_counts ?? {}).length > 0 ? (
              Object.entries(runtimeDetailProvider.failure_counts).map(([reason, count]) => (
                <div key={reason} className="rounded-2xl border border-border/70 bg-card/85 px-4 py-3 text-sm">
                  <div className="font-medium text-foreground">{reason}</div>
                  <div className="mt-1 text-muted-foreground">{t('providerRuntimeErrors', String(count))}</div>
                </div>
              ))
            ) : (
              <div className="rounded-2xl border border-dashed border-border/70 px-4 py-6 text-sm text-muted-foreground">{t('providerRuntimeIdle')}</div>
            )}
            {runtimeDetailProvider?.disabled_reason ? <div className="text-sm text-muted-foreground">{t('providerRuntimeReason', runtimeDetailProvider.disabled_reason)}</div> : null}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRuntimeDetailProvider(null)}>{t('close')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ProviderForm open={formOpen} onOpenChange={handleFormOpenChange} provider={editingProvider} />
    </div>
  );
}

function normalizeProviderGroup(value: unknown): ProviderGroupDraft | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null;
  }
  const record = value as Record<string, unknown>;
  const name = typeof record.name === 'string' ? record.name : '';
  const rawStrategy = typeof record.strategy === 'string' ? record.strategy : 'round_robin';
  const strategy = PROVIDER_GROUP_STRATEGIES.some((item) => item.value === rawStrategy)
    ? (rawStrategy as ProviderGroupStrategy)
    : 'round_robin';
  const members = Array.isArray(record.members)
    ? record.members.filter((member): member is string => typeof member === 'string' && member.trim().length > 0)
    : [];
  return { id: createProviderGroupId(), name, strategy, members };
}

function createProviderGroupId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID();
  return `provider-group-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function validateProviderGroups(groups: ProviderGroup[]): string | null {
  const seen = new Set<string>();
  for (const group of groups) {
    const name = group.name.trim();
    if (!name) return t('providerGroupValidationNameRequired');
    if (seen.has(name)) return t('providerGroupValidationDuplicateName', name);
    seen.add(name);
    if (group.members.length < 2) return t('providerGroupValidationMembersMin', name);
  }
  return null;
}

function MetricCard({ label, value, muted }: { label: string; value: string; muted?: boolean }) {
  return (
    <div className="min-w-[120px] rounded-2xl border border-slate-200/80 bg-white/90 px-4 py-3 shadow-sm">
      <div className="text-[11px] uppercase tracking-[0.18em] text-slate-400">{label}</div>
      <div className={cn('mt-1 text-base font-semibold text-slate-900', muted && 'text-slate-500')}>{value}</div>
    </div>
  );
}

function ProviderPanel({ provider, runtime, typeMeta, onClick, onClearCooldown, onShowRuntimeDetails, clearingCooldown }: { provider: Provider; runtime?: ProviderRuntime; typeMeta?: { display_name: string; description: string; default_api_base?: string; icon: string }; onClick: () => void; onClearCooldown: (providerName: string) => void; onShowRuntimeDetails: (runtime: ProviderRuntime) => void; clearingCooldown: boolean; }) {
  const state = getConnectionState(provider, runtime);
  const logo = getProviderLogo(typeMeta?.icon || provider.provider_kind);
  const tint = getKindTint(provider.provider_kind);
  const endpoint = provider.api_base?.trim() || typeMeta?.default_api_base?.trim() || t('providerPanelDefaultEndpoint');
  const credentialsReady = provider.api_key_set;
  const discoveryReady = provider.supports_discovery && credentialsReady;

  return (
    <Card className="group cursor-pointer overflow-hidden rounded-[28px] border-slate-200/80 bg-white/95 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-lg hover:shadow-slate-200/60" onClick={onClick}>
      <div className={cn('border-b border-slate-100 bg-gradient-to-br p-5', tint)}>
        <div className="flex items-start justify-between gap-4">
          <div className="flex min-w-0 items-center gap-4">
            <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-[18px] border border-white/70 bg-white/95 shadow-sm">
              {logo ? <img src={logo} alt={provider.provider_kind} className="h-7 w-7 object-contain" /> : <span className="text-lg font-semibold uppercase">{provider.provider_kind?.[0] ?? '?'}</span>}
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="truncate text-lg font-semibold text-slate-950">{provider.name}</h3>
                {provider.is_routing_default && <span className="inline-flex items-center gap-1 rounded-full border border-sky-200/80 bg-sky-50 px-2 py-0.5 text-[11px] font-medium text-sky-700"><ShieldCheck className="h-3 w-3" />{t('providerPanelRoutingDefault')}</span>}
              </div>
              <p className="mt-1 text-sm capitalize text-slate-600">{typeMeta?.display_name || provider.provider_kind}</p>
            </div>
          </div>
          <span className={cn('inline-flex items-center gap-2 rounded-full border px-2.5 py-1 text-xs font-medium shadow-sm', state.tone)}>
            <span className={cn('h-2 w-2 rounded-full', state.dot)} />
            {state.label}
          </span>
        </div>
      </div>

      <div className="space-y-5 p-5">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <InfoTile icon={<KeyRound className="h-4 w-4" />} label={t('providerPanelCredentials')} value={provider.api_key_set ? t('providerPanelConfigured') : t('providerPanelMissing')} detail={provider.enabled ? credentialsReady ? t('providerPanelCredentialsReadyDetail') : t('providerPanelCredentialsMissingDetail') : t('providerPanelDisabledDetail')} />
          <InfoTile icon={<Waypoints className="h-4 w-4" />} label={t('providerDiscoverLabel')} value={provider.supports_discovery ? discoveryReady ? t('providerPanelDiscoveryReady') : t('providerPanelDiscoveryBlocked') : t('providerPanelDiscoveryManual')} detail={provider.supports_discovery ? discoveryReady ? t('providerPanelDiscoveryReadyDetail') : t('providerPanelDiscoveryBlockedDetail') : t('providerPanelDiscoveryManualDetail')} />
          <InfoTile icon={<BadgeCheck className="h-4 w-4" />} label={t('providerPanelTimeout')} value={`${provider.timeout || 0}s`} detail={provider.proxy?.trim() ? t('providerPanelProxyEnabled') : t('providerPanelDirect')} />
        </div>

        <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_220px]">
          <div className="rounded-2xl border border-slate-200/80 bg-slate-50/80 p-4">
            <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.16em] text-slate-400"><Globe className="h-3.5 w-3.5" />{t('providerPanelEndpoint')}</div>
            <p className="mt-2 break-all text-sm leading-6 text-slate-700">{endpoint}</p>
            {provider.proxy?.trim() && <p className="mt-3 text-xs leading-5 text-slate-500">Proxy: {provider.proxy}</p>}
            <p className="mt-3 text-xs leading-5 text-slate-500">{t('providerWeightSummary', String(provider.default_weight), provider.enabled ? t('enabled') : t('disabled'))}</p>
          </div>

          <div className="rounded-2xl border border-slate-200/80 bg-white p-4">
            <div className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">{t('providerPanelSummary')}</div>
            <p className="mt-2 text-sm leading-6 text-slate-700">{provider.summary || typeMeta?.description || t('providerPanelNoSummary')}</p>
            <div className="mt-3 flex items-center justify-between gap-2 text-xs text-slate-500">
              <span>{provider.provider_kind}</span>
              <ArrowUpRight className="h-4 w-4 text-slate-300 transition-colors group-hover:text-slate-500" />
            </div>
          </div>
        </div>

        <div className="rounded-2xl border border-slate-200/80 bg-slate-50/80 p-4">
          <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.16em] text-slate-400"><TimerReset className="h-3.5 w-3.5" />{t('providerRuntimeTitle')}</div>
          <div className="mt-3 flex flex-wrap gap-2">
            <span className={cn('rounded-full px-3 py-1.5 text-xs font-medium', runtime?.in_cooldown ? 'bg-amber-50 text-amber-800' : runtime?.available === false ? 'bg-rose-50 text-rose-700' : 'bg-emerald-50 text-emerald-700')}>
              {runtime?.in_cooldown ? t('providerRuntimeCooldown') : runtime?.available === false ? t('providerRuntimeUnavailable') : t('providerRuntimeAvailable')}
            </span>
            <span className="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs text-slate-600">{t('providerRuntimeErrors', String(runtime?.error_count ?? 0))}</span>
            {runtime?.in_cooldown && <span className="rounded-full border border-amber-200 bg-white px-3 py-1.5 text-xs text-amber-800">{t('providerRuntimeRemaining', formatDuration(runtime.cooldown_remaining_seconds))}</span>}
            {runtime?.disabled_reason && <span className="rounded-full border border-rose-200 bg-white px-3 py-1.5 text-xs text-rose-700">{t('providerRuntimeReason', runtime.disabled_reason)}</span>}
          </div>
          <p className="mt-3 text-sm leading-6 text-slate-500">{formatFailureSummary(runtime)}</p>
          {runtime ? (
            <div className="mt-3 flex flex-wrap gap-2">
              {runtime.error_count > 0 ? <Button type="button" variant="outline" className="rounded-full" onClick={(event) => { event.stopPropagation(); onShowRuntimeDetails(runtime); }}>{t('providerRuntimeViewErrors', String(runtime.error_count))}</Button> : null}
              {runtime.in_cooldown ? <Button type="button" variant="outline" className="rounded-full" onClick={(event) => { event.stopPropagation(); onClearCooldown(provider.name); }} disabled={clearingCooldown}><TimerReset className="mr-2 h-4 w-4" />{t('providerCooldownClear')}</Button> : null}
            </div>
          ) : null}
        </div>
      </div>
    </Card>
  )
}

function formatDuration(totalSeconds: number): string {
  if (!totalSeconds || totalSeconds <= 0) return '0s';
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes <= 0) return `${seconds}s`
  if (seconds === 0) return `${minutes}m`
  return `${minutes}m ${seconds}s`
}

function formatFailureSummary(runtime?: ProviderRuntime): string {
  if (!runtime || !runtime.failure_counts || Object.keys(runtime.failure_counts).length === 0) {
    return t('providerRuntimeIdle')
  }
  return Object.entries(runtime.failure_counts)
    .map(([reason, count]) => `${reason}: ${count}`)
    .join(' · ')
}

function InfoTile({ icon, label, value, detail }: { icon: ReactNode; label: string; value: string; detail: string }) {
  return (
    <div className="rounded-2xl border border-slate-200/80 bg-white p-4">
      <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.16em] text-slate-400">{icon}{label}</div>
      <div className="mt-2 text-sm font-semibold text-slate-900">{value}</div>
      <p className="mt-2 text-xs leading-5 text-slate-500">{detail}</p>
    </div>
  )
}
