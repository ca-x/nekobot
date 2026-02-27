import { useState } from 'react';
import Header from '@/components/layout/Header';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { useProviders, type Provider } from '@/hooks/useProviders';
import { ProviderForm } from '@/components/config/ProviderForm';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Plus, KeyRound, Globe, Cpu } from 'lucide-react';

// ---------- Provider type badge colors ----------

const KIND_COLORS: Record<string, string> = {
  openai: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-400',
  anthropic: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-400',
  gemini: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400',
  ollama: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-400',
  groq: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-400',
  lmstudio: 'bg-pink-100 text-pink-700 dark:bg-pink-900/40 dark:text-pink-400',
  vllm: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-400',
  deepseek: 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-400',
  moonshot: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-400',
  zhipu: 'bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-400',
  openrouter: 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-400',
  nvidia: 'bg-lime-100 text-lime-700 dark:bg-lime-900/40 dark:text-lime-400',
  generic: 'bg-stone-100 text-stone-600 dark:bg-stone-800 dark:text-stone-400',
};

function getKindBadgeClass(kind: string): string {
  return KIND_COLORS[kind.toLowerCase()] ?? KIND_COLORS.generic;
}

// ---------- Component ----------

export default function ProvidersPage() {
  const { data: providers, isLoading } = useProviders();
  const [formOpen, setFormOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null);

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
    if (!open) setEditingProvider(null);
  };

  return (
    <div>
      <Header title={t('tabProviders')} description="Configure AI model providers" />

      {/* Toolbar */}
      <div className="flex items-center justify-between mb-4">
        <p className="text-sm text-muted-foreground">
          {providers
            ? `${providers.length} ${providers.length === 1 ? 'provider' : 'providers'}`
            : ''}
        </p>
        <Button size="sm" onClick={openNew}>
          <Plus className="h-4 w-4 mr-1.5" />
          {t('newProvider')}
        </Button>
      </div>

      {/* Loading skeleton */}
      {isLoading && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-40 rounded-xl" />
          ))}
        </div>
      )}

      {/* Provider Cards Grid */}
      {!isLoading && providers && providers.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {providers.map((p) => (
            <ProviderCard key={p.name} provider={p} onClick={() => openEdit(p)} />
          ))}
        </div>
      )}

      {/* Empty State */}
      {!isLoading && providers && providers.length === 0 && (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="h-14 w-14 flex items-center justify-center rounded-xl bg-muted mb-4">
            <KeyRound className="h-6 w-6 text-muted-foreground" />
          </div>
          <h3 className="text-sm font-semibold text-foreground mb-1.5">
            {t('noProviders')}
          </h3>
          <p className="text-sm text-muted-foreground max-w-sm mb-4">
            {t('noProvidersHint')}
          </p>
          <Button size="sm" onClick={openNew}>
            <Plus className="h-4 w-4 mr-1.5" />
            {t('newProvider')}
          </Button>
        </div>
      )}

      {/* Provider Form Dialog */}
      <ProviderForm
        open={formOpen}
        onOpenChange={handleFormOpenChange}
        provider={editingProvider}
      />
    </div>
  );
}

// ---------- Sub-component: ProviderCard ----------

interface ProviderCardProps {
  provider: Provider;
  onClick: () => void;
}

function ProviderCard({ provider, onClick }: ProviderCardProps) {
  const hasKey = !!provider.api_key;
  const modelCount = provider.models?.length ?? 0;
  const truncatedEndpoint = provider.api_base
    ? provider.api_base.length > 35
      ? provider.api_base.slice(0, 35) + '\u2026'
      : provider.api_base
    : '--';

  return (
    <Card
      className="group relative cursor-pointer hover:shadow-md transition-[shadow,border-color] duration-200 hover:border-primary/30"
      onClick={onClick}
    >
      <div className="p-4 space-y-3">
        {/* Header row: logo + status */}
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            {/* Type badge / logo */}
            <div
              className={cn(
                'h-10 w-10 rounded-lg flex items-center justify-center text-sm font-bold uppercase shrink-0',
                getKindBadgeClass(provider.provider_kind),
              )}
            >
              {provider.provider_kind?.[0] ?? '?'}
            </div>
            <div className="min-w-0">
              <h3 className="text-sm font-semibold text-foreground truncate">
                {provider.name}
              </h3>
              <span
                className={cn(
                  'inline-block text-[11px] font-medium px-1.5 py-0.5 rounded mt-0.5',
                  getKindBadgeClass(provider.provider_kind),
                )}
              >
                {provider.provider_kind || 'unknown'}
              </span>
            </div>
          </div>
          {/* Status dot */}
          <div className="flex items-center gap-1.5 shrink-0">
            <div
              className={cn(
                'h-2.5 w-2.5 rounded-full',
                hasKey ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600',
              )}
            />
          </div>
        </div>

        {/* Info rows */}
        <div className="space-y-1.5 text-xs text-muted-foreground">
          <div className="flex items-center gap-1.5">
            <Globe className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">{truncatedEndpoint}</span>
          </div>
          <div className="flex items-center gap-1.5">
            <Cpu className="h-3.5 w-3.5 shrink-0" />
            <span>
              {modelCount > 0
                ? `${modelCount} model${modelCount !== 1 ? 's' : ''}`
                : 'No models'}
            </span>
          </div>
        </div>
      </div>
    </Card>
  );
}
