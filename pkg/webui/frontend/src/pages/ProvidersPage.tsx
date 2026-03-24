import { useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { useProviders, type Provider } from '@/hooks/useProviders';
import { ProviderForm } from '@/components/config/ProviderForm';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { getProviderLogo } from '@/lib/provider-logos';
import {
  ArrowUpRight,
  BadgeCheck,
  CircleAlert,
  Cpu,
  Globe,
  KeyRound,
  Plus,
  Search,
  ShieldCheck,
  Sparkles,
} from 'lucide-react';

const KIND_TINTS: Record<string, string> = {
  openai: 'from-emerald-500/15 via-emerald-500/8 to-transparent text-emerald-700',
  anthropic: 'from-amber-500/15 via-orange-500/8 to-transparent text-orange-700',
  gemini: 'from-sky-500/15 via-indigo-500/8 to-transparent text-sky-700',
  openrouter: 'from-violet-500/15 via-indigo-500/8 to-transparent text-violet-700',
  groq: 'from-rose-500/15 via-orange-500/8 to-transparent text-rose-700',
  deepseek: 'from-cyan-500/15 via-blue-500/8 to-transparent text-cyan-700',
  moonshot: 'from-slate-500/15 via-slate-400/8 to-transparent text-slate-700',
  zhipu: 'from-blue-500/15 via-blue-400/8 to-transparent text-blue-700',
  vllm: 'from-red-500/15 via-orange-500/8 to-transparent text-red-700',
  generic: 'from-stone-500/15 via-stone-400/8 to-transparent text-stone-700',
};

function getKindTint(kind: string): string {
  return KIND_TINTS[kind.trim().toLowerCase()] ?? KIND_TINTS.generic;
}

function getProviderState(provider: Provider): {
  label: string;
  tone: string;
  dot: string;
} {
  if (!provider.api_key_set) {
    return {
      label: 'Credentials missing',
      tone: 'bg-amber-50 text-amber-700 border-amber-200/70',
      dot: 'bg-amber-500',
    };
  }
  if ((provider.model_count ?? 0) === 0) {
    return {
      label: 'Needs models',
      tone: 'bg-slate-100 text-slate-700 border-slate-200/70',
      dot: 'bg-slate-400',
    };
  }
  return {
    label: 'Ready',
    tone: 'bg-emerald-50 text-emerald-700 border-emerald-200/70',
    dot: 'bg-emerald-500',
  };
}

export default function ProvidersPage() {
  const { data: providers, isLoading } = useProviders();
  const [formOpen, setFormOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null);
  const [query, setQuery] = useState('');

  const filteredProviders = useMemo(() => {
    const items = providers ?? [];
    const keyword = query.trim().toLowerCase();
    if (!keyword) {
      return items;
    }
    return items.filter((provider) =>
      [
        provider.name,
        provider.provider_kind,
        provider.api_base,
        provider.default_model,
        ...(provider.models ?? []),
      ]
        .join(' ')
        .toLowerCase()
        .includes(keyword),
    );
  }, [providers, query]);

  const readyCount = (providers ?? []).filter((provider) => provider.api_key_set).length;
  const routingDefault = (providers ?? []).find((provider) => provider.is_routing_default) ?? null;

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

  return (
    <div className="space-y-6">
      <Header
        title={t('tabProviders')}
        description="Add endpoints, check readiness, and keep routing defaults visible at a glance."
      />

      <section className="relative overflow-hidden rounded-[28px] border border-slate-200/80 bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.12),_transparent_38%),linear-gradient(135deg,_rgba(255,255,255,0.98),_rgba(248,250,252,0.96))] p-5 shadow-sm sm:p-6">
        <div className="absolute right-0 top-0 h-40 w-40 rounded-full bg-sky-100/60 blur-3xl" />
        <div className="relative flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-3">
            <div className="inline-flex items-center gap-2 rounded-full border border-sky-200/70 bg-white/85 px-3 py-1 text-xs font-medium text-sky-700 shadow-sm">
              <Sparkles className="h-3.5 w-3.5" />
              Provider workspace
            </div>
            <div className="space-y-2">
              <h2 className="max-w-2xl text-2xl font-semibold tracking-tight text-slate-900">
                Build a clean provider graph before you touch routing.
              </h2>
              <p className="max-w-2xl text-sm leading-6 text-slate-600">
                Configure each provider once, keep model discovery close to the record, and make the
                active default obvious.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <MetricCard label="Providers" value={String(providers?.length ?? 0)} />
              <MetricCard label="Key ready" value={String(readyCount)} />
              <MetricCard
                label="Routing default"
                value={routingDefault?.name ?? 'Unset'}
                muted={!routingDefault}
              />
            </div>
          </div>

          <div className="flex w-full flex-col gap-3 sm:w-auto sm:min-w-[320px]">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Search provider, model, endpoint"
                className="h-11 rounded-2xl border-slate-200 bg-white/90 pl-9 shadow-sm"
              />
            </div>
            <Button size="sm" onClick={openNew} className="h-11 rounded-2xl px-4">
              <Plus className="mr-1.5 h-4 w-4" />
              {t('newProvider')}
            </Button>
          </div>
        </div>
      </section>

      {isLoading && (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-64 rounded-[24px]" />
          ))}
        </div>
      )}

      {!isLoading && filteredProviders.length > 0 && (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          {filteredProviders.map((provider) => (
            <ProviderPanel key={provider.name} provider={provider} onClick={() => openEdit(provider)} />
          ))}
        </div>
      )}

      {!isLoading && (providers?.length ?? 0) > 0 && filteredProviders.length === 0 && (
        <div className="rounded-[24px] border border-dashed border-slate-300 bg-slate-50/70 px-6 py-14 text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-white shadow-sm">
            <Search className="h-5 w-5 text-slate-400" />
          </div>
          <h3 className="text-sm font-semibold text-slate-900">No matching providers</h3>
          <p className="mt-1 text-sm text-slate-500">Try a broader keyword or clear the search box.</p>
        </div>
      )}

      {!isLoading && (providers?.length ?? 0) === 0 && (
        <div className="rounded-[28px] border border-dashed border-slate-300 bg-[linear-gradient(180deg,_rgba(255,255,255,0.96),_rgba(248,250,252,0.96))] px-6 py-20 text-center shadow-sm">
          <div className="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-[22px] bg-slate-900 text-white shadow-lg shadow-slate-900/10">
            <KeyRound className="h-7 w-7" />
          </div>
          <h3 className="text-lg font-semibold text-slate-900">{t('noProviders')}</h3>
          <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-slate-500">
            Start with one provider that can successfully list models. Once that works, route defaults
            and model switching become much easier to reason about.
          </p>
          <Button size="sm" onClick={openNew} className="mt-5 h-10 rounded-xl px-4">
            <Plus className="mr-1.5 h-4 w-4" />
            {t('newProvider')}
          </Button>
        </div>
      )}

      <ProviderForm
        open={formOpen}
        onOpenChange={handleFormOpenChange}
        provider={editingProvider}
      />
    </div>
  );
}

function MetricCard({
  label,
  value,
  muted,
}: {
  label: string;
  value: string;
  muted?: boolean;
}) {
  return (
    <div className="min-w-[120px] rounded-2xl border border-slate-200/80 bg-white/90 px-4 py-3 shadow-sm">
      <div className="text-[11px] uppercase tracking-[0.18em] text-slate-400">{label}</div>
      <div className={cn('mt-1 text-base font-semibold text-slate-900', muted && 'text-slate-500')}>
        {value}
      </div>
    </div>
  );
}

function ProviderPanel({
  provider,
  onClick,
}: {
  provider: Provider;
  onClick: () => void;
}) {
  const state = getProviderState(provider);
  const logo = getProviderLogo(provider.provider_kind);
  const tint = getKindTint(provider.provider_kind);
  const modelCount = provider.model_count ?? provider.models?.length ?? 0;
  const endpoint = provider.api_base?.trim() || 'Using provider default endpoint';
  const topModels = (provider.models ?? []).slice(0, 3);

  return (
    <Card
      className="group cursor-pointer overflow-hidden rounded-[28px] border-slate-200/80 bg-white/95 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-lg hover:shadow-slate-200/60"
      onClick={onClick}
    >
      <div className={cn('border-b border-slate-100 bg-gradient-to-br p-5', tint)}>
        <div className="flex items-start justify-between gap-4">
          <div className="flex min-w-0 items-center gap-4">
            <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-[18px] border border-white/70 bg-white/95 shadow-sm">
              {logo ? (
                <img src={logo} alt={provider.provider_kind} className="h-7 w-7 object-contain" />
              ) : (
                <span className="text-lg font-semibold uppercase">{provider.provider_kind?.[0] ?? '?'}</span>
              )}
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="truncate text-lg font-semibold text-slate-950">{provider.name}</h3>
                {provider.is_routing_default && (
                  <span className="inline-flex items-center gap-1 rounded-full border border-sky-200/80 bg-sky-50 px-2 py-0.5 text-[11px] font-medium text-sky-700">
                    <ShieldCheck className="h-3 w-3" />
                    Routing default
                  </span>
                )}
              </div>
              <p className="mt-1 text-sm capitalize text-slate-600">{provider.provider_kind}</p>
            </div>
          </div>

          <span
            className={cn(
              'inline-flex items-center gap-2 rounded-full border px-2.5 py-1 text-xs font-medium shadow-sm',
              state.tone,
            )}
          >
            <span className={cn('h-2 w-2 rounded-full', state.dot)} />
            {state.label}
          </span>
        </div>
      </div>

      <div className="space-y-5 p-5">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <InfoTile
            icon={<Cpu className="h-4 w-4" />}
            label="Models"
            value={modelCount > 0 ? String(modelCount) : '0'}
            detail={provider.has_default_model ? provider.default_model : 'No default'}
          />
          <InfoTile
            icon={<KeyRound className="h-4 w-4" />}
            label="Credentials"
            value={provider.api_key_set ? 'Configured' : 'Missing'}
            detail={provider.supports_discovery ? 'Discovery ready' : 'Manual only'}
          />
          <InfoTile
            icon={<BadgeCheck className="h-4 w-4" />}
            label="Timeout"
            value={`${provider.timeout || 0}s`}
            detail={provider.proxy?.trim() ? 'Proxy enabled' : 'Direct'}
          />
        </div>

        <div className="space-y-3">
          <div className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
            <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.16em] text-slate-400">
              <Globe className="h-3.5 w-3.5" />
              Endpoint
            </div>
            <p className="mt-2 break-all text-sm leading-6 text-slate-700">{endpoint}</p>
          </div>

          <div className="rounded-2xl border border-slate-200/80 bg-white p-4">
            <div className="flex items-center justify-between gap-2">
              <div className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">
                Summary
              </div>
              <ArrowUpRight className="h-4 w-4 text-slate-300 transition-colors group-hover:text-slate-500" />
            </div>
            <p className="mt-2 text-sm leading-6 text-slate-700">{provider.summary || 'No summary available.'}</p>
            {topModels.length > 0 && (
              <div className="mt-3 flex flex-wrap gap-2">
                {topModels.map((model) => (
                  <span
                    key={model}
                    className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700"
                  >
                    {model}
                  </span>
                ))}
              </div>
            )}
            {topModels.length === 0 && (
              <div className="mt-3 inline-flex items-center gap-1.5 rounded-full bg-amber-50 px-2.5 py-1 text-xs text-amber-700">
                <CircleAlert className="h-3.5 w-3.5" />
                Discover or add models before using this provider in chat.
              </div>
            )}
          </div>
        </div>
      </div>
    </Card>
  );
}

function InfoTile({
  icon,
  label,
  value,
  detail,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  detail: string;
}) {
  return (
    <div className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
      <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.16em] text-slate-400">
        {icon}
        {label}
      </div>
      <div className="mt-2 text-sm font-semibold text-slate-900">{value}</div>
      <div className="mt-1 line-clamp-1 text-xs text-slate-500">{detail}</div>
    </div>
  );
}
