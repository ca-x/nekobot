import { useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { t } from '@/lib/i18n';
import {
  buildModelOptions,
  normalizeRouteMetadataProviderModelID,
  useCreateModel,
  useModelRoutesForModels,
  useModels,
  useUpdateModelRoute,
  type ModelRoute,
} from '@/hooks/useModels';
import { useProviders } from '@/hooks/useProviders';
import {
  ChevronDown,
  ChevronUp,
  Route,
  Search,
  SlidersHorizontal,
  Sparkles,
} from 'lucide-react';

function parseListInput(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function defaultRouteDraft(modelID: string, providerName: string): ModelRoute {
  return {
    model_id: modelID,
    provider_name: providerName,
    enabled: true,
    is_default: true,
    weight_override: 0,
    aliases: [],
    regex_rules: [],
    metadata: {},
  };
}

function normalizeRoute(route: ModelRoute): ModelRoute {
  return {
    ...route,
    aliases: Array.isArray(route.aliases) ? route.aliases : [],
    regex_rules: Array.isArray(route.regex_rules) ? route.regex_rules : [],
    metadata: route.metadata ?? {},
  };
}

export default function ModelsPage() {
  const { data: modelCatalog = [], isLoading } = useModels();
  const { data: providers = [] } = useProviders();
  const createModel = useCreateModel();
  const updateRoute = useUpdateModelRoute();
  const [query, setQuery] = useState('');
  const [expandedModelID, setExpandedModelID] = useState('');
  const [newModelID, setNewModelID] = useState('');
  const [newDisplayName, setNewDisplayName] = useState('');
  const [newRouteProviderByModel, setNewRouteProviderByModel] = useState<Record<string, string>>({});

  const routesQuery = useModelRoutesForModels(modelCatalog.map((item) => item.model_id));
  const routesByModel = useMemo(
    () => routesQuery.data ?? Object.fromEntries(modelCatalog.map((item) => [item.model_id, []])),
    [modelCatalog, routesQuery.data],
  );
  const modelOptions = useMemo(
    () => buildModelOptions(modelCatalog, routesByModel),
    [modelCatalog, routesByModel],
  );

  const filteredModels = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) {
      return modelCatalog;
    }
    return modelCatalog.filter((model) =>
      [
        model.model_id,
        model.display_name,
        model.developer,
        model.family,
        model.type,
        model.catalog_source,
      ]
        .join(' ')
        .toLowerCase()
        .includes(keyword),
    );
  }, [modelCatalog, query]);

  const providerNames = useMemo(
    () => new Set(providers.map((provider) => provider.name.trim()).filter(Boolean)),
    [providers],
  );
  const createDisabled = createModel.isPending || !newModelID.trim() || !newDisplayName.trim();

  const handleCreateModel = () => {
    createModel.mutate(
      {
        model_id: newModelID.trim(),
        display_name: newDisplayName.trim(),
        catalog_source: 'manual',
        enabled: true,
      },
      {
        onSuccess: () => {
          setNewModelID('');
          setNewDisplayName('');
        },
      },
    );
  };

  return (
    <div className="space-y-6">
      <Header title={t('tabModels')} description={t('modelsPageDescription')} />

      <section className="relative overflow-hidden rounded-[28px] border border-border/70 bg-[radial-gradient(circle_at_top_left,_rgba(244,114,182,0.18),_transparent_38%),linear-gradient(135deg,hsl(var(--card)/0.98),hsl(var(--muted)/0.72))] p-5 shadow-sm sm:p-6">
        <div className="absolute right-0 top-0 h-40 w-40 rounded-full bg-rose-100/60 blur-3xl" />
        <div className="relative flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-3">
            <div className="inline-flex items-center gap-2 rounded-full border border-rose-300/40 bg-card/90 px-3 py-1 text-xs font-medium text-rose-700 shadow-sm">
              <Sparkles className="h-3.5 w-3.5" />
              {t('modelsHeroBadge')}
            </div>
            <div className="space-y-2">
              <h2 className="max-w-2xl text-2xl font-semibold tracking-tight text-foreground">
                {t('modelsHeroTitle')}
              </h2>
              <p className="max-w-2xl text-sm leading-6 text-muted-foreground">
                {t('modelsHeroDescription')}
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <MetricCard label={t('modelsMetricCatalog')} value={String(modelCatalog.length)} />
              <MetricCard label={t('modelsMetricEnabled')} value={String(modelOptions.length)} />
              <MetricCard
                label={t('modelsMetricProvidersWired')}
                value={String(
                  Array.from(
                    new Set(
                      Object.values(routesByModel)
                        .flat()
                        .map((route) => route.provider_name),
                    ),
                  ).length,
                )}
              />
            </div>
          </div>

          <div className="w-full sm:max-w-[320px]">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t('modelsSearchPlaceholder')}
                className="h-11 rounded-2xl border-border/70 bg-card/90 pl-9 shadow-sm"
              />
            </div>
          </div>
        </div>
      </section>

      <Card className="rounded-[28px] border-border/70 bg-card/92 shadow-sm">
        <CardContent className="grid gap-4 p-5 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
          <div className="space-y-2">
            <Label htmlFor="new-model-id">{t('modelsFieldModelId')}</Label>
            <Input
              id="new-model-id"
              value={newModelID}
              onChange={(event) => setNewModelID(event.target.value)}
              placeholder="gpt-4.1"
              className="h-11 rounded-2xl bg-card/90"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-model-display-name">{t('modelsFieldDisplayName')}</Label>
            <Input
              id="new-model-display-name"
              value={newDisplayName}
              onChange={(event) => setNewDisplayName(event.target.value)}
              placeholder="GPT-4.1"
              className="h-11 rounded-2xl bg-card/90"
            />
          </div>
          <div className="flex items-end">
            <Button type="button" className="h-11 rounded-full px-5" onClick={handleCreateModel} disabled={createDisabled}>
              {t('modelsCreate')}
            </Button>
          </div>
        </CardContent>
      </Card>

      {isLoading && (
        <div className="grid grid-cols-1 gap-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-48 rounded-[24px]" />
          ))}
        </div>
      )}

      {!isLoading && filteredModels.length === 0 && (
        <div className="rounded-[28px] border border-dashed border-border/70 bg-card/70 px-6 py-16 text-center">
          <div className="text-lg font-semibold text-foreground">{t('modelsEmptyTitle')}</div>
          <p className="mt-2 text-sm leading-6 text-muted-foreground">
            {t('modelsEmptyDescription')}
          </p>
        </div>
      )}

      {!isLoading && filteredModels.length > 0 && (
        <div className="grid grid-cols-1 gap-4">
          {filteredModels.map((model) => {
            const routes = routesByModel[model.model_id] ?? [];
            const expanded = expandedModelID === model.model_id;
            return (
              <Card key={model.model_id} className="overflow-hidden rounded-[28px] border-border/70 bg-card/92 shadow-sm">
                <div className="border-b border-border/70 bg-[linear-gradient(135deg,rgba(255,248,250,0.95),rgba(249,245,255,0.92))] px-5 py-5">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div className="space-y-3">
                      <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-card/90 px-3 py-1 text-xs font-medium text-muted-foreground">
                        <Route className="h-3.5 w-3.5" />
                        {model.catalog_source || t('providerPanelManualOnly')}
                      </div>
                      <div>
                        <h3 className="text-lg font-semibold text-foreground">{model.display_name || model.model_id}</h3>
                        <p className="mt-1 break-all font-mono text-sm text-muted-foreground">{model.model_id}</p>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <span className="rounded-full bg-background px-2.5 py-1 text-xs text-muted-foreground">
                          {model.enabled ? t('enabled') : t('disabled')}
                        </span>
                        {model.developer && (
                          <span className="rounded-full bg-background px-2.5 py-1 text-xs text-muted-foreground">
                            {model.developer}
                          </span>
                        )}
                        {model.family && (
                          <span className="rounded-full bg-background px-2.5 py-1 text-xs text-muted-foreground">
                            {model.family}
                          </span>
                        )}
                        {model.type && (
                          <span className="rounded-full bg-background px-2.5 py-1 text-xs text-muted-foreground">
                            {model.type}
                          </span>
                        )}
                      </div>
                    </div>

                    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                      <MetricCard label={t('modelsMetricRoutes')} value={String(routes.length)} compact />
                      <MetricCard
                        label={t('modelsMetricProviders')}
                        value={String(new Set(routes.map((route) => route.provider_name)).size)}
                        compact
                      />
                      <Button
                        type="button"
                        variant="outline"
                        className="h-full rounded-2xl px-4"
                        onClick={() => setExpandedModelID(expanded ? '' : model.model_id)}
                      >
                        {expanded ? <ChevronUp className="mr-2 h-4 w-4" /> : <ChevronDown className="mr-2 h-4 w-4" />}
                        {t('modelsRoutesButton')}
                      </Button>
                    </div>
                  </div>
                </div>

                {expanded && (
                  <CardContent className="space-y-4 p-5">
                    {routes.length === 0 ? (
                      <div className="rounded-[22px] border border-dashed border-border/70 bg-card/70 px-4 py-8">
                        <div className="space-y-4 text-center">
                          <p className="text-sm text-muted-foreground">{t('modelsNoRoutes')}</p>
                          <div className="mx-auto grid max-w-xl gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
                            <Input
                              value={newRouteProviderByModel[model.model_id] ?? ''}
                              onChange={(event) =>
                                setNewRouteProviderByModel((prev) => ({
                                  ...prev,
                                  [model.model_id]: event.target.value,
                                }))
                              }
                              placeholder={t('modelsProviderNamePlaceholder')}
                              className="h-11 rounded-2xl bg-card/90"
                              list={`providers-${model.model_id}`}
                            />
                            <datalist id={`providers-${model.model_id}`}>
                              {providers.map((provider) => (
                                <option key={provider.name} value={provider.name} />
                              ))}
                            </datalist>
                            <Button
                              type="button"
                              className="h-11 rounded-full px-5"
                              disabled={
                                updateRoute.isPending ||
                                !(newRouteProviderByModel[model.model_id] ?? '').trim()
                              }
                              onClick={() =>
                                updateRoute.mutate({
                                  modelID: model.model_id,
                                  providerName: (newRouteProviderByModel[model.model_id] ?? '').trim(),
                                  data: defaultRouteDraft(model.model_id, (newRouteProviderByModel[model.model_id] ?? '').trim()),
                                })
                              }
                            >
                              {t('modelsAddRoute')}
                            </Button>
                          </div>
                        </div>
                      </div>
                    ) : (
                      <ScrollArea className="max-h-[520px] pr-3">
                        <div className="space-y-4">
                          {routes.map((route) => (
                            <RouteEditor
                              key={`${route.model_id}-${route.provider_name}`}
                              route={normalizeRoute(route)}
                              providerExists={providerNames.has(route.provider_name)}
                              onSave={(next) =>
                                updateRoute.mutate({
                                  modelID: route.model_id,
                                  providerName: route.provider_name,
                                  data: next,
                                })
                              }
                              saving={updateRoute.isPending}
                            />
                          ))}
                        </div>
                      </ScrollArea>
                    )}
                  </CardContent>
                )}
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}

function RouteEditor({
  route,
  providerExists,
  onSave,
  saving,
}: {
  route: ModelRoute;
  providerExists: boolean;
  onSave: (value: ModelRoute) => void;
  saving: boolean;
}) {
  const [enabled, setEnabled] = useState(route.enabled);
  const [isDefault, setIsDefault] = useState(route.is_default);
  const [weight, setWeight] = useState(String(route.weight_override || ''));
  const [aliases, setAliases] = useState(route.aliases.join(', '));
  const [regexRules, setRegexRules] = useState(route.regex_rules.join(', '));
  const [providerModelID, setProviderModelID] = useState(normalizeRouteMetadataProviderModelID(route));

  const handleSave = () => {
    onSave({
      ...route,
      enabled,
      is_default: isDefault,
      weight_override: weight.trim() ? Number(weight) : 0,
      aliases: parseListInput(aliases),
      regex_rules: parseListInput(regexRules),
      metadata: {
        ...(route.metadata ?? {}),
        provider_model_id: providerModelID.trim(),
      },
    });
  };

  return (
    <div className="rounded-[24px] border border-border/70 bg-background/80 p-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <div className="text-sm font-semibold text-foreground">{route.provider_name}</div>
          <p className="mt-1 text-sm text-muted-foreground">
            {providerExists ? t('modelsRouteConnectedProvider') : t('modelsRouteMissingProvider')}
          </p>
        </div>
        <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-card px-3 py-1.5 text-xs text-muted-foreground">
          <SlidersHorizontal className="h-3.5 w-3.5" />
          {route.is_default ? t('modelsRouteDefault') : t('modelsRouteSecondary')}
        </div>
      </div>

      <div className="mt-4 grid gap-4 xl:grid-cols-2">
        <Field label={t('enabled')}>
          <div className="flex h-11 items-center rounded-2xl border border-border/70 bg-card/90 px-3">
            <Switch checked={enabled} onCheckedChange={setEnabled} />
          </div>
        </Field>
        <Field label={t('modelsRouteDefaultToggle')}>
          <div className="flex h-11 items-center rounded-2xl border border-border/70 bg-card/90 px-3">
            <Switch checked={isDefault} onCheckedChange={setIsDefault} />
          </div>
        </Field>
        <Field label={t('modelsFieldWeightOverride')}>
          <Input
            type="number"
            min={0}
            value={weight}
            onChange={(event) => setWeight(event.target.value)}
            className="h-11 rounded-2xl bg-card/90"
          />
        </Field>
        <Field label={t('modelsFieldProviderModelId')}>
          <Input
            value={providerModelID}
            onChange={(event) => setProviderModelID(event.target.value)}
            className="h-11 rounded-2xl bg-card/90"
          />
        </Field>
        <Field label={t('modelsFieldAliases')}>
          <Input
            value={aliases}
            onChange={(event) => setAliases(event.target.value)}
            placeholder={t('modelsAliasesPlaceholder')}
            className="h-11 rounded-2xl bg-card/90"
          />
        </Field>
        <Field label={t('modelsFieldRegexRules')}>
          <Input
            value={regexRules}
            onChange={(event) => setRegexRules(event.target.value)}
            placeholder={t('modelsRegexPlaceholder')}
            className="h-11 rounded-2xl bg-card/90"
          />
        </Field>
      </div>

      <div className="mt-4 flex justify-end">
        <Button type="button" className="rounded-full" onClick={handleSave} disabled={saving}>
          {t('modelsSaveRoute')}
        </Button>
      </div>
    </div>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
    </div>
  );
}

function MetricCard({
  label,
  value,
  compact,
}: {
  label: string;
  value: string;
  compact?: boolean;
}) {
  return (
    <div className={`rounded-2xl border border-border/70 bg-card/90 px-4 py-3 shadow-sm ${compact ? 'min-w-[104px]' : 'min-w-[120px]'}`}>
      <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className="mt-1 text-base font-semibold text-foreground">{value}</div>
    </div>
  );
}
