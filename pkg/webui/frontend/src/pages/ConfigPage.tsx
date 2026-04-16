import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';
import Header from '@/components/layout/Header';
import {
  useWatchStatus,
  useCleanupSessions,
  useCleanupSkillVersions,
  useCleanupToolSessionEvents,
  useConfig,
  useExportConfig,
  useImportConfig,
  useSaveConfig,
} from '@/hooks/useConfig';
import { useToolSessionRuntimeTransports } from '@/hooks/useToolSessions';
import { useModels, useModelRoutesForModels, buildModelOptions } from '@/hooks/useModels';
import { useProviders } from '@/hooks/useProviders';
import { useCleanupQMDExports, useInstallQMD, useQMDStatus, useUpdateQMD } from '@/hooks/useQMD';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  AlertTriangle,
  Code,
  Download,
  Eye,
  EyeOff,
  FormInput,
  FolderKanban,
  Layers3,
  LibraryBig,
  RefreshCw,
  Route,
  RotateCcw,
  Save,
  Search,
  ShieldCheck,
  Upload,
} from 'lucide-react';

const CONFIG_SECTIONS = [
  'storage',
  'agents',
  'gateway',
  'tools',
  'transcription',
  'memory',
  'sessions',
  'heartbeat',
  'redis',
  'state',
  'bus',
  'approval',
  'logger',
  'webui',
  'audit',
  'undo',
  'preprocess',
  'learnings',
  'watch',
] as const;

type ConfigSection = (typeof CONFIG_SECTIONS)[number];

type ConfigShape = { [section: string]: Record<string, unknown> };

type Primitive = string | number | boolean | null;

interface FieldDef {
  key: string;
  label: string;
  type: 'text' | 'secret' | 'bool' | 'number' | 'tags' | 'json';
  value: unknown;
}

interface RouteTargetOption {
  name: string;
  type: 'provider' | 'group';
}

interface SectionMeta {
  labelKey: string;
  descriptionKey: string;
}

const MANAGED_AGENT_FIELDS = new Set([
  'defaults.provider',
  'defaults.model',
  'defaults.fallback',
  'defaults.provider_groups',
]);

const SECTION_META: Record<ConfigSection, SectionMeta> = {
  storage: { labelKey: 'configSectionStorage', descriptionKey: 'configSectionDescStorage' },
  agents: { labelKey: 'configSectionAgents', descriptionKey: 'configSectionDescAgents' },
  gateway: { labelKey: 'configSectionGateway', descriptionKey: 'configSectionDescGateway' },
  tools: { labelKey: 'configSectionTools', descriptionKey: 'configSectionDescTools' },
  transcription: { labelKey: 'configSectionTranscription', descriptionKey: 'configSectionDescTranscription' },
  memory: { labelKey: 'configSectionMemory', descriptionKey: 'configSectionDescMemory' },
  sessions: { labelKey: 'configSectionSessions', descriptionKey: 'configSectionDescSessions' },
  heartbeat: { labelKey: 'configSectionHeartbeat', descriptionKey: 'configSectionDescHeartbeat' },
  redis: { labelKey: 'configSectionRedis', descriptionKey: 'configSectionDescRedis' },
  state: { labelKey: 'configSectionState', descriptionKey: 'configSectionDescState' },
  bus: { labelKey: 'configSectionBus', descriptionKey: 'configSectionDescBus' },
  approval: { labelKey: 'configSectionApproval', descriptionKey: 'configSectionDescApproval' },
  logger: { labelKey: 'configSectionLogger', descriptionKey: 'configSectionDescLogger' },
  webui: { labelKey: 'configSectionWebUI', descriptionKey: 'configSectionDescWebUI' },
  audit: { labelKey: 'configSectionAudit', descriptionKey: 'configSectionDescAudit' },
  undo: { labelKey: 'configSectionUndo', descriptionKey: 'configSectionDescUndo' },
  preprocess: { labelKey: 'configSectionPreprocess', descriptionKey: 'configSectionDescPreprocess' },
  learnings: { labelKey: 'configSectionLearnings', descriptionKey: 'configSectionDescLearnings' },
  watch: { labelKey: 'configSectionWatch', descriptionKey: 'configSectionDescWatch' },
};

function sectionLabel(section: ConfigSection): string {
  return t(SECTION_META[section].labelKey);
}

function sectionDescription(section: ConfigSection): string {
  return t(SECTION_META[section].descriptionKey);
}

function sectionPersistenceHint(section: ConfigSection): string {
  switch (section) {
    case 'storage':
      return t('configSectionHintStorage');
    case 'gateway':
    case 'logger':
    case 'webui':
      return t('configSectionHintBootstrap', sectionLabel(section));
    default:
      return t('configSectionHint', sectionLabel(section));
  }
}

function cloneSection(value: Record<string, unknown>): Record<string, unknown> {
  return JSON.parse(JSON.stringify(value ?? {})) as Record<string, unknown>;
}

function stableStringify(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

function flattenObject(value: Record<string, unknown>, prefix = ''): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return result;
  }

  for (const key of Object.keys(value)) {
    const currentPath = prefix ? `${prefix}.${key}` : key;
    const currentValue = value[key];
    if (currentValue !== null && typeof currentValue === 'object' && !Array.isArray(currentValue)) {
      Object.assign(result, flattenObject(currentValue as Record<string, unknown>, currentPath));
      continue;
    }
    result[currentPath] = currentValue;
  }

  return result;
}

function setNestedValue(target: Record<string, unknown>, path: string, value: unknown) {
  const parts = path.split('.');
  let cursor: Record<string, unknown> = target;
  for (let i = 0; i < parts.length - 1; i++) {
    const part = parts[i];
    if (!cursor[part] || typeof cursor[part] !== 'object' || Array.isArray(cursor[part])) {
      cursor[part] = {};
    }
    cursor = cursor[part] as Record<string, unknown>;
  }
  cursor[parts[parts.length - 1]] = value;
}

function getNestedValue(target: Record<string, unknown>, path: string): unknown {
  const parts = path.split('.');
  let cursor: unknown = target;
  for (const part of parts) {
    if (!cursor || typeof cursor !== 'object' || Array.isArray(cursor)) {
      return undefined;
    }
    cursor = (cursor as Record<string, unknown>)[part];
  }
  return cursor;
}

function lastSegment(path: string): string {
  const parts = path.split('.');
  return parts.length > 0 ? parts[parts.length - 1] : path;
}

function humanizeLabel(path: string): string {
  return lastSegment(path)
    .split('_')
    .map((part: string) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');
}

function isSecretKey(path: string): boolean {
  return /(api[_-]?key|token|secret|password|jwt)/i.test(path);
}

function isPrimitiveArray(value: unknown): value is Primitive[] {
  return Array.isArray(value) &&
    value.every((item) => item == null || ['string', 'number', 'boolean'].includes(typeof item));
}

function inferFields(sectionData: Record<string, unknown>): FieldDef[] {
  return Object.entries(flattenObject(sectionData))
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => {
      let type: FieldDef['type'] = 'text';
      if (typeof value === 'boolean') {
        type = 'bool';
      } else if (typeof value === 'number') {
        type = 'number';
      } else if (isPrimitiveArray(value)) {
        type = 'tags';
      } else if (Array.isArray(value)) {
        type = 'json';
      } else if (isSecretKey(key)) {
        type = 'secret';
      }

      return {
        key,
        label: humanizeLabel(key),
        type,
        value,
      };
    });
}

function filterFields(fields: FieldDef[], query: string): FieldDef[] {
  const normalized = query.trim().toLowerCase();
  if (!normalized) {
    return fields;
  }
  return fields.filter((field) =>
    field.key.toLowerCase().includes(normalized) ||
    field.label.toLowerCase().includes(normalized),
  );
}

function DirtyDot({ dirty }: { dirty: boolean }) {
  return (
    <span
      className={
        dirty
          ? 'inline-flex h-2.5 w-2.5 rounded-full bg-[hsl(var(--brand-600))] shadow-[0_0_0_4px_rgba(198,104,140,0.12)]'
          : 'inline-flex h-2.5 w-2.5 rounded-full bg-[hsl(var(--gray-300))]'
      }
    />
  );
}

function formatQueryErrorMessage(error: unknown): string {
  return error instanceof Error && error.message.trim() ? error.message : t('configLoadFailedDetailFallback');
}

function ConfigLoadErrorState({
  message,
  onRetry,
  retrying,
}: {
  message: string;
  onRetry: () => void;
  retrying: boolean;
}) {
  return (
    <div className="flex min-h-[360px] items-center justify-center px-4 pb-4 md:px-5 md:pb-5">
      <Card className="w-full max-w-2xl border-rose-200/80 bg-rose-50/60 shadow-[0_24px_60px_-42px_rgba(160,60,70,0.35)]">
        <CardHeader className="pb-3">
          <div className="inline-flex w-fit items-center gap-2 rounded-full bg-rose-100 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-rose-700">
            <AlertTriangle className="h-3.5 w-3.5" />
            {t('configLoadFailedTitle')}
          </div>
          <CardTitle className="text-xl text-rose-950">{t('configLoadFailedTitle')}</CardTitle>
          <CardDescription className="text-rose-900/80">
            {t('configLoadFailedDescription')}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-2xl border border-rose-200/80 bg-white/85 px-4 py-3 text-sm text-rose-900">
            {message}
          </div>
          <div className="flex justify-end">
            <Button type="button" variant="outline" className="rounded-full" onClick={onRetry} disabled={retrying}>
              <RefreshCw className={`mr-2 h-4 w-4 ${retrying ? 'animate-spin' : ''}`} />
              {t('refresh')}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function FormField({
  field,
  secretVisible,
  onChange,
  onToggleSecret,
  onOpenJSONMode,
}: {
  field: FieldDef;
  secretVisible: boolean;
  onChange: (key: string, value: unknown) => void;
  onToggleSecret: (key: string) => void;
  onOpenJSONMode: () => void;
}) {
  const containerClassName =
    'rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4 shadow-[0_18px_40px_-34px_rgba(120,55,75,0.35)]';

  const header = (
    <div className="mb-3 flex items-start justify-between gap-3">
      <div className="space-y-1">
        <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{field.label}</Label>
        <div className="text-xs font-mono text-muted-foreground">{field.key}</div>
      </div>
      {field.type === 'secret' ? (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-8 rounded-full px-2"
          onClick={() => onToggleSecret(field.key)}
        >
          {secretVisible ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
        </Button>
      ) : null}
    </div>
  );

  switch (field.type) {
    case 'bool':
      return (
        <div className={containerClassName}>
          <div className="flex items-center justify-between gap-4">
            <div>
              <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{field.label}</Label>
              <div className="mt-1 text-xs font-mono text-muted-foreground">{field.key}</div>
            </div>
            <Switch checked={Boolean(field.value)} onCheckedChange={(next) => onChange(field.key, next)} />
          </div>
        </div>
      );
    case 'number':
      return (
        <div className={containerClassName}>
          {header}
          <Input
            type="number"
            className="h-11 rounded-xl bg-white"
            value={field.value != null ? String(field.value) : ''}
            onChange={(event) => onChange(field.key, event.target.value === '' ? 0 : Number(event.target.value))}
          />
        </div>
      );
    case 'tags':
      return (
        <div className={containerClassName}>
          {header}
          <textarea
            className="min-h-[96px] w-full rounded-xl border border-input bg-white px-3 py-2 text-sm leading-6"
            rows={4}
            value={Array.isArray(field.value) ? field.value.map((item) => String(item)).join('\n') : ''}
            onChange={(event) =>
              onChange(
                field.key,
                event.target.value.split('\n').map((item) => item.trim()).filter(Boolean),
              )
            }
          />
          <div className="mt-2 text-xs text-muted-foreground">{t('configListHint')}</div>
        </div>
      );
    case 'json':
      return (
        <div className={containerClassName}>
          {header}
          <pre className="max-h-[220px] overflow-auto rounded-xl border border-amber-200 bg-[rgba(255,248,239,0.9)] px-3 py-3 font-mono text-xs leading-6 text-amber-950">
            {stableStringify(field.value)}
          </pre>
          <div className="mt-3 flex items-center justify-between gap-3">
            <div className="flex items-center gap-2 text-xs text-amber-700">
              <AlertTriangle className="h-3.5 w-3.5" />
              {t('configJsonFieldHint')}
            </div>
            <Button type="button" variant="outline" size="sm" className="rounded-full" onClick={onOpenJSONMode}>
              <Code className="mr-1.5 h-4 w-4" />
              {t('configJsonMode')}
            </Button>
          </div>
        </div>
      );
    case 'secret':
    case 'text':
    default:
      return (
        <div className={containerClassName}>
          {header}
          <Input
            type={field.type === 'secret' && !secretVisible ? 'password' : 'text'}
            className="h-11 rounded-xl bg-white"
            value={field.value != null ? String(field.value) : ''}
            onChange={(event) => onChange(field.key, event.target.value)}
            spellCheck={false}
          />
        </div>
      );
  }
}

function MemoryField({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4 shadow-[0_18px_40px_-34px_rgba(120,55,75,0.35)]">
      <div className="mb-3">
        <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{label}</Label>
        {hint ? <div className="mt-1 text-xs leading-5 text-muted-foreground">{hint}</div> : null}
      </div>
      {children}
    </div>
  );
}

function QMDMetric({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
      <div className="eyebrow-label text-slate-400">{label}</div>
      <div className="mono-data mt-2 break-all text-sm font-semibold text-slate-900">{value}</div>
    </div>
  );
}

function AgentsSectionForm({
  data,
  onChange,
}: {
  data: Record<string, unknown>;
  onChange: (key: string, value: unknown) => void;
}) {
  const { data: providers = [] } = useProviders();
  const { data: modelCatalog = [] } = useModels();
  const modelRoutesQueries = useModelRoutesForModels(modelCatalog.map((item) => item.model_id));

  const readString = (path: string) => {
    const value = getNestedValue(data, path);
    return typeof value === 'string' ? value : '';
  };
  const readNumber = (path: string) => {
    const value = getNestedValue(data, path);
    return typeof value === 'number' ? value : 0;
  };
  const readBool = (path: string) => Boolean(getNestedValue(data, path));
  const readStringArray = (path: string) => {
    const value = getNestedValue(data, path);
    return Array.isArray(value) ? value.map((item) => String(item).trim()).filter(Boolean) : [];
  };
  const readProviderGroups = (): Array<Record<string, unknown>> => {
    const value = getNestedValue(data, 'defaults.provider_groups');
    return Array.isArray(value)
      ? value.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === 'object' && !Array.isArray(item))
      : [];
  };

  const routeTargets = useMemo(() => {
    const options: RouteTargetOption[] = [];
    const seen = new Set<string>();

    for (const provider of providers) {
      if (!provider.api_key_set) {
        continue;
      }
      const name = provider.name.trim();
      if (!name || seen.has(name)) {
        continue;
      }
      seen.add(name);
      options.push({ name, type: 'provider' });
    }

    for (const group of readProviderGroups()) {
      const name = typeof group.name === 'string' ? group.name.trim() : '';
      if (!name || seen.has(name)) {
        continue;
      }
      seen.add(name);
      options.push({ name, type: 'group' });
    }

    return options;
  }, [providers, data]);

  const routeTargetMap = useMemo(
    () => new Map(routeTargets.map((target) => [target.name, target])),
    [routeTargets],
  );

  const routesByModel = useMemo(
    () =>
      Object.fromEntries(
        modelCatalog.map((item, index) => [item.model_id, modelRoutesQueries[index]?.data ?? []]),
      ),
    [modelCatalog, modelRoutesQueries],
  );
  const models = useMemo(
    () => buildModelOptions(modelCatalog, routesByModel),
    [modelCatalog, routesByModel],
  );

  const selectedProvider = readString('defaults.provider');
  const selectedFallback = readStringArray('defaults.fallback');
  const selectedModel = readString('defaults.model');
  const selectedProviderKind = routeTargetMap.get(selectedProvider)?.type ?? null;
  const filteredModels = selectedProvider && selectedProviderKind !== 'group'
    ? models.filter((entry) => entry.providers.includes(selectedProvider))
    : models;
  const fallbackOptions = routeTargets.filter((target) => target.name !== selectedProvider);

  const handleToggleFallback = (name: string) => {
    if (selectedFallback.includes(name)) {
      onChange('defaults.fallback', selectedFallback.filter((item) => item !== name));
      return;
    }
    onChange('defaults.fallback', [...selectedFallback, name]);
  };

  const handleProviderChange = (value: string) => {
    const nextProvider = value === '__default__' ? '' : value;
    onChange('defaults.provider', nextProvider);
    onChange(
      'defaults.fallback',
      selectedFallback.filter((item) => item !== nextProvider),
    );
  };

  return (
    <div className="space-y-5">
      <Card className="border-white/70 bg-[linear-gradient(180deg,rgba(255,250,247,0.95),rgba(255,244,248,0.9))] shadow-[0_24px_60px_-42px_rgba(120,55,75,0.45)]">
        <CardHeader className="pb-4">
          <div className="inline-flex w-fit items-center gap-2 rounded-full bg-[hsl(var(--brand-50))] px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-[hsl(var(--brand-700))]">
            <Route className="h-3.5 w-3.5" />
            {t('agentsRoutingTitle')}
          </div>
          <CardTitle className="text-xl text-[hsl(var(--gray-900))]">{t('agentsRoutingHeadline')}</CardTitle>
          <CardDescription>{t('agentsRoutingDescription')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 xl:grid-cols-2">
            <MemoryField label={t('agentsDefaultProvider')} hint={t('agentsDefaultProviderHint')}>
              <Select value={selectedProvider || '__default__'} onValueChange={handleProviderChange}>
                <SelectTrigger className="h-11 rounded-xl bg-white">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__default__">{t('agentsRouteAuto')}</SelectItem>
                  {routeTargets.map((target) => (
                    <SelectItem key={target.name} value={target.name}>
                      {target.type === 'group'
                        ? `${target.name} (${t('agentsRouteTargetGroup')})`
                        : target.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </MemoryField>

            <MemoryField label={t('agentsDefaultModel')} hint={t('agentsDefaultModelHint')}>
              <Select
                value={selectedModel || '__default__'}
                onValueChange={(value) => onChange('defaults.model', value === '__default__' ? '' : value)}
              >
                <SelectTrigger className="h-11 rounded-xl bg-white">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__default__">{t('agentsModelDefault')}</SelectItem>
                  {filteredModels.map((entry) => (
                    <SelectItem key={entry.value} value={entry.value}>
                      {entry.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </MemoryField>
          </div>

          <MemoryField label={t('agentsFallbackProviders')} hint={t('agentsFallbackProvidersHint')}>
            <div className="space-y-3">
              {selectedFallback.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {selectedFallback.map((name, index) => (
                    <button
                      key={name}
                      type="button"
                      onClick={() => handleToggleFallback(name)}
                      className="inline-flex items-center gap-2 rounded-full border border-[hsl(var(--brand-200))] bg-[hsl(var(--brand-50))] px-3 py-1.5 text-xs font-medium text-[hsl(var(--brand-800))]"
                    >
                      <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-white text-[10px] text-[hsl(var(--brand-700))]">
                        {index + 1}
                      </span>
                      {name}
                    </button>
                  ))}
                </div>
              ) : (
                <div className="rounded-2xl border border-dashed border-[hsl(var(--gray-200))] px-3 py-4 text-sm text-muted-foreground">
                  {t('agentsFallbackEmpty')}
                </div>
              )}
              <div className="flex flex-wrap gap-2">
                {fallbackOptions.map((target) => {
                  const selected = selectedFallback.includes(target.name);
                  return (
                    <button
                      key={target.name}
                      type="button"
                      onClick={() => handleToggleFallback(target.name)}
                      className={
                        selected
                          ? 'rounded-full border border-[hsl(var(--brand-300))] bg-[hsl(var(--brand-100))] px-3 py-1.5 text-xs font-medium text-[hsl(var(--brand-800))]'
                          : 'rounded-full border border-[hsl(var(--gray-200))] bg-white px-3 py-1.5 text-xs font-medium text-muted-foreground hover:border-[hsl(var(--gray-300))] hover:bg-[hsl(var(--gray-50))]'
                      }
                    >
                      {target.type === 'group'
                        ? `${target.name} (${t('agentsRouteTargetGroup')})`
                        : target.name}
                    </button>
                  );
                })}
              </div>
            </div>
          </MemoryField>
        </CardContent>
      </Card>

      <Card className="border-white/70 bg-white/80 shadow-none">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t('agentsRuntimeTitle')}</CardTitle>
          <CardDescription>{t('agentsRuntimeDescription')}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 xl:grid-cols-2">
          <MemoryField label={t('agentsWorkspace')}>
            <Input
              className="h-11 rounded-xl bg-white"
              value={readString('defaults.workspace')}
              onChange={(event) => onChange('defaults.workspace', event.target.value)}
            />
          </MemoryField>
          <MemoryField label={t('agentsOrchestrator')}>
            <Select value={readString('defaults.orchestrator') || 'blades'} onValueChange={(value) => onChange('defaults.orchestrator', value)}>
              <SelectTrigger className="h-11 rounded-xl bg-white">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="blades">blades</SelectItem>
                <SelectItem value="legacy">legacy</SelectItem>
              </SelectContent>
            </Select>
          </MemoryField>
          <MemoryField label={t('agentsMaxTokens')}>
            <Input
              type="number"
              className="h-11 rounded-xl bg-white"
              value={String(readNumber('defaults.max_tokens'))}
              onChange={(event) => onChange('defaults.max_tokens', Number(event.target.value || 0))}
            />
          </MemoryField>
          <MemoryField label={t('agentsTemperature')}>
            <Input
              type="number"
              step="0.1"
              className="h-11 rounded-xl bg-white"
              value={String(readNumber('defaults.temperature'))}
              onChange={(event) => onChange('defaults.temperature', Number(event.target.value || 0))}
            />
          </MemoryField>
          <MemoryField label={t('agentsMaxToolIterations')}>
            <Input
              type="number"
              className="h-11 rounded-xl bg-white"
              value={String(readNumber('defaults.max_tool_iterations'))}
              onChange={(event) => onChange('defaults.max_tool_iterations', Number(event.target.value || 0))}
            />
          </MemoryField>
          <MemoryField label={t('agentsSkillsProxy')}>
            <Input
              className="h-11 rounded-xl bg-white"
              value={readString('defaults.skills_proxy')}
              onChange={(event) => onChange('defaults.skills_proxy', event.target.value)}
            />
          </MemoryField>
          <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
            <div>
              <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('agentsRestrictWorkspace')}</Label>
            </div>
            <Switch checked={readBool('defaults.restrict_to_workspace')} onCheckedChange={(next) => onChange('defaults.restrict_to_workspace', next)} />
          </div>
          <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
            <div>
              <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('agentsSkillsAutoReload')}</Label>
            </div>
            <Switch checked={readBool('defaults.skills_auto_reload')} onCheckedChange={(next) => onChange('defaults.skills_auto_reload', next)} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function MemorySectionForm({
  data,
  onChange,
}: {
  data: Record<string, unknown>;
  onChange: (key: string, value: unknown) => void;
}) {
  const { data: qmdStatus, isLoading: qmdLoading, refetch: refetchQMD, isFetching: qmdFetching } = useQMDStatus();
  const updateQMD = useUpdateQMD();
  const installQMD = useInstallQMD();
  const cleanupQMDExports = useCleanupQMDExports();
  const readBool = (path: string) => Boolean(getNestedValue(data, path));
  const readNumber = (path: string) => {
    const value = getNestedValue(data, path);
    return typeof value === 'number' ? value : 0;
  };
  const readString = (path: string) => {
    const value = getNestedValue(data, path);
    return typeof value === 'string' ? value : '';
  };
  const readPaths = () => {
    const value = getNestedValue(data, 'qmd.paths');
    if (!Array.isArray(value)) {
      return '';
    }
    return value
      .map((item) => {
        if (!item || typeof item !== 'object' || Array.isArray(item)) {
          return '';
        }
        const record = item as Record<string, unknown>;
        const name = typeof record.name === 'string' ? record.name : '';
        const path = typeof record.path === 'string' ? record.path : '';
        const pattern = typeof record.pattern === 'string' ? record.pattern : '';
        return [name, path, pattern].join('|');
      })
      .filter(Boolean)
      .join('\n');
  };

  const writePaths = (input: string) => {
    const paths = input
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        const [name = '', path = '', pattern = '**/*.md'] = line.split('|').map((part) => part.trim());
        return { name, path, pattern: pattern || '**/*.md' };
      });
    onChange('qmd.paths', paths);
  };

  return (
    <div className="space-y-5">
      <div className="grid gap-4 xl:grid-cols-2">
        <Card className="border-white/70 bg-[linear-gradient(180deg,rgba(255,250,247,0.95),rgba(255,244,248,0.9))] shadow-[0_24px_60px_-42px_rgba(120,55,75,0.45)]">
          <CardHeader className="pb-4">
            <div className="inline-flex w-fit items-center gap-2 rounded-full bg-[hsl(var(--brand-50))] px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-[hsl(var(--brand-700))]">
              <LibraryBig className="h-3.5 w-3.5" />
              {t('memoryBasicTitle')}
            </div>
            <CardTitle className="text-xl text-[hsl(var(--gray-900))]">{t('memoryBasicHeadline')}</CardTitle>
            <CardDescription>{t('memoryBasicDescription')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
              <div>
                <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('memoryEnabled')}</Label>
                <div className="mt-1 text-xs text-muted-foreground">{t('memoryEnabledHint')}</div>
              </div>
              <Switch checked={readBool('enabled')} onCheckedChange={(next) => onChange('enabled', next)} />
            </div>

            <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
              <div>
                <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('memoryContextEnabled')}</Label>
                <div className="mt-1 text-xs text-muted-foreground">{t('memoryContextEnabledHint')}</div>
              </div>
              <Switch
                checked={readBool('context.enabled')}
                onCheckedChange={(next) => onChange('context.enabled', next)}
              />
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">
                    {t('memoryContextIncludeWorkspace')}
                  </Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('memoryContextIncludeWorkspaceHint')}</div>
                </div>
                <Switch
                  checked={readBool('context.include_workspace_memory')}
                  onCheckedChange={(next) => onChange('context.include_workspace_memory', next)}
                />
              </div>
              <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">
                    {t('memoryContextIncludeLongTerm')}
                  </Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('memoryContextIncludeLongTermHint')}</div>
                </div>
                <Switch
                  checked={readBool('context.include_long_term')}
                  onCheckedChange={(next) => onChange('context.include_long_term', next)}
                />
              </div>
            </div>

            <MemoryField label={t('memoryBackend')} hint={t('memoryBackendHint')}>
              <Select value={readString('backend') || 'file'} onValueChange={(value) => onChange('backend', value)}>
                <SelectTrigger className="h-11 rounded-xl bg-white">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="file">file</SelectItem>
                  <SelectItem value="db">db</SelectItem>
                  <SelectItem value="kv">kv</SelectItem>
                </SelectContent>
              </Select>
            </MemoryField>

            <MemoryField label={t('memoryFilePath')} hint={t('memoryFilePathHint')}>
              <Input
                className="h-11 rounded-xl bg-white"
                value={readString('file_path')}
                onChange={(event) => onChange('file_path', event.target.value)}
                placeholder="workspace/memory"
              />
            </MemoryField>

            <div className="grid gap-4 sm:grid-cols-2">
              <MemoryField label={t('memoryDBPrefix')} hint={t('memoryDBPrefixHint')}>
                <Input
                  className="h-11 rounded-xl bg-white"
                  value={readString('db_prefix')}
                  onChange={(event) => onChange('db_prefix', event.target.value)}
                />
              </MemoryField>
              <MemoryField label={t('memoryKVPrefix')} hint={t('memoryKVPrefixHint')}>
                <Input
                  className="h-11 rounded-xl bg-white"
                  value={readString('kv_prefix')}
                  onChange={(event) => onChange('kv_prefix', event.target.value)}
                />
              </MemoryField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <MemoryField label={t('memoryContextRecentDays')} hint={t('memoryContextRecentDaysHint')}>
                <Input
                  type="number"
                  className="h-11 rounded-xl bg-white"
                  value={String(readNumber('context.recent_daily_note_days'))}
                  onChange={(event) => onChange('context.recent_daily_note_days', Number(event.target.value || 0))}
                />
              </MemoryField>
              <MemoryField label={t('memoryContextMaxChars')} hint={t('memoryContextMaxCharsHint')}>
                <Input
                  type="number"
                  className="h-11 rounded-xl bg-white"
                  value={String(readNumber('context.max_chars'))}
                  onChange={(event) => onChange('context.max_chars', Number(event.target.value || 0))}
                />
              </MemoryField>
            </div>

            <div className="rounded-2xl border border-dashed border-[hsl(var(--gray-200))] bg-white/70 px-4 py-3 text-xs leading-6 text-muted-foreground">
              {t('memoryPromptHint')}
            </div>
            <div className="rounded-2xl border border-amber-200 bg-[rgba(255,248,239,0.9)] px-4 py-3 text-xs leading-6 text-amber-800">
              {t('memoryNoMigrationHint')}
            </div>
          </CardContent>
        </Card>

        <Card className="border-white/70 bg-[linear-gradient(180deg,rgba(243,250,255,0.95),rgba(239,246,255,0.92))] shadow-[0_24px_60px_-42px_rgba(54,92,140,0.35)]">
          <CardHeader className="pb-4">
            <div className="inline-flex w-fit items-center gap-2 rounded-full bg-sky-50 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-sky-700">
              <Layers3 className="h-3.5 w-3.5" />
              {t('memoryRetrievalTitle')}
            </div>
            <CardTitle className="text-xl text-[hsl(var(--gray-900))]">{t('memoryRetrievalHeadline')}</CardTitle>
            <CardDescription>{t('memoryRetrievalDescription')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="flex items-center justify-between rounded-2xl border border-sky-100 bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('memorySemanticEnabled')}</Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('memorySemanticEnabledHint')}</div>
                </div>
                <Switch checked={readBool('semantic.enabled')} onCheckedChange={(next) => onChange('semantic.enabled', next)} />
              </div>
              <div className="flex items-center justify-between rounded-2xl border border-sky-100 bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('memoryEpisodicEnabled')}</Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('memoryEpisodicEnabledHint')}</div>
                </div>
                <Switch checked={readBool('episodic.enabled')} onCheckedChange={(next) => onChange('episodic.enabled', next)} />
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <MemoryField label={t('memorySearchPolicy')} hint={t('memorySearchPolicyHint')}>
                <Select
                  value={readString('semantic.search_policy') || 'vector'}
                  onValueChange={(value) => onChange('semantic.search_policy', value)}
                >
                  <SelectTrigger className="h-11 rounded-xl bg-white">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="vector">vector</SelectItem>
                    <SelectItem value="hybrid">hybrid</SelectItem>
                  </SelectContent>
                </Select>
              </MemoryField>
              <MemoryField label={t('memoryIncludeScores')} hint={t('memoryIncludeScoresHint')}>
                <div className="flex items-center justify-end">
                  <Switch
                    checked={readBool('semantic.include_scores')}
                    onCheckedChange={(next) => onChange('semantic.include_scores', next)}
                  />
                </div>
              </MemoryField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <MemoryField label={t('memoryDefaultTopK')}>
                <Input
                  type="number"
                  className="h-11 rounded-xl bg-white"
                  value={String(readNumber('semantic.default_top_k'))}
                  onChange={(event) => onChange('semantic.default_top_k', Number(event.target.value || 0))}
                />
              </MemoryField>
              <MemoryField label={t('memoryMaxTopK')}>
                <Input
                  type="number"
                  className="h-11 rounded-xl bg-white"
                  value={String(readNumber('semantic.max_top_k'))}
                  onChange={(event) => onChange('semantic.max_top_k', Number(event.target.value || 0))}
                />
              </MemoryField>
              <MemoryField label={t('memorySummaryWindow')}>
                <Input
                  type="number"
                  className="h-11 rounded-xl bg-white"
                  value={String(readNumber('episodic.summary_window_messages'))}
                  onChange={(event) => onChange('episodic.summary_window_messages', Number(event.target.value || 0))}
                />
              </MemoryField>
              <MemoryField label={t('memoryMaxSummaries')}>
                <Input
                  type="number"
                  className="h-11 rounded-xl bg-white"
                  value={String(readNumber('episodic.max_summaries'))}
                  onChange={(event) => onChange('episodic.max_summaries', Number(event.target.value || 0))}
                />
              </MemoryField>
            </div>

            <MemoryField label={t('memoryShortTermLimit')} hint={t('memoryShortTermLimitHint')}>
              <Input
                type="number"
                className="h-11 rounded-xl bg-white"
                value={String(readNumber('short_term.raw_history_limit'))}
                onChange={(event) => onChange('short_term.raw_history_limit', Number(event.target.value || 0))}
              />
            </MemoryField>
          </CardContent>
        </Card>
      </div>

      <Card className="border-white/70 bg-[linear-gradient(180deg,rgba(246,255,248,0.96),rgba(239,253,245,0.92))] shadow-[0_24px_60px_-42px_rgba(52,114,84,0.3)]">
        <CardHeader className="pb-4">
          <div className="inline-flex w-fit items-center gap-2 rounded-full bg-emerald-50 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-emerald-700">
            <FolderKanban className="h-3.5 w-3.5" />
            {t('memoryQMDTitle')}
          </div>
          <CardTitle className="text-xl text-[hsl(var(--gray-900))]">{t('memoryQMDHeadline')}</CardTitle>
          <CardDescription>{t('memoryQMDDescription')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-[24px] border border-emerald-200/80 bg-white/86 p-4">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-emerald-700">
                  {t('memoryQMDRuntimeTitle')}
                </div>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  {t('memoryQMDRuntimeDescription')}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="rounded-xl"
                  onClick={() => refetchQMD()}
                  disabled={qmdFetching}
                >
                  <RefreshCw className={`mr-2 h-4 w-4 ${qmdFetching ? 'animate-spin' : ''}`} />
                  {t('refresh')}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="rounded-xl"
                  onClick={() => installQMD.mutate()}
                  disabled={installQMD.isPending}
                >
                  <RefreshCw className={`mr-2 h-4 w-4 ${installQMD.isPending ? 'animate-spin' : ''}`} />
                  {installQMD.isPending ? t('memoryQMDInstalling') : t('memoryQMDInstallPersisted')}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  className="rounded-xl"
                  onClick={() => updateQMD.mutate()}
                  disabled={updateQMD.isPending || !qmdStatus?.available}
                >
                  <RefreshCw className={`mr-2 h-4 w-4 ${updateQMD.isPending ? 'animate-spin' : ''}`} />
                  {updateQMD.isPending ? t('memoryQMDUpdating') : t('memoryQMDUpdateNow')}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="rounded-xl"
                  onClick={() => cleanupQMDExports.mutate()}
                  disabled={
                    cleanupQMDExports.isPending ||
                    !qmdStatus?.sessions_enabled ||
                    (qmdStatus?.session_retention_days ?? 0) < 1
                  }
                >
                  <RefreshCw className={`mr-2 h-4 w-4 ${cleanupQMDExports.isPending ? 'animate-spin' : ''}`} />
                  {cleanupQMDExports.isPending ? t('memoryQMDCleanupRunning') : t('memoryQMDCleanupNow')}
                </Button>
              </div>
            </div>

            {qmdLoading ? (
              <div className="mt-4 text-sm text-muted-foreground">{t('memoryQMDRuntimeLoading')}</div>
            ) : (
              <div className="mt-4 space-y-4">
                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                  <QMDMetric label={t('memoryQMDStatusEnabled')} value={qmdStatus?.enabled ? t('on') : t('off')} />
                  <QMDMetric label={t('memoryQMDStatusAvailable')} value={qmdStatus?.available ? t('memoryQMDAvailable') : t('memoryQMDUnavailable')} />
                  <QMDMetric label={t('memoryQMDStatusVersion')} value={qmdStatus?.version || '-'} />
                  <QMDMetric label={t('memoryQMDStatusCollections')} value={String(qmdStatus?.collections.length ?? 0)} />
                  <QMDMetric label={t('memoryQMDStatusExports')} value={String(qmdStatus?.session_export_file_count ?? 0)} />
                  <QMDMetric label={t('memoryQMDStatusRetention')} value={String(qmdStatus?.session_retention_days ?? 0)} />
                </div>

                <div className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
                  <div className="text-[11px] font-medium uppercase tracking-[0.18em] text-slate-400">
                    {t('memoryQMDCommand')}
                  </div>
                  <div className="mt-2 break-all font-mono text-sm text-slate-700">
                    {qmdStatus?.command || readString('qmd.command') || 'qmd'}
                  </div>
                  <div className="mt-3 grid gap-3 sm:grid-cols-2">
                    <QMDMetric label={t('memoryQMDResolvedCommand')} value={qmdStatus?.resolved_command || '-'} />
                    <QMDMetric label={t('memoryQMDCommandSource')} value={qmdStatus?.command_source || '-'} />
                  </div>
                  <div className="mt-3">
                    <div className="text-[11px] font-medium uppercase tracking-[0.18em] text-slate-400">
                      {t('memoryQMDPersistentCommand')}
                    </div>
                    <div className="mt-2 break-all font-mono text-xs text-slate-600">
                      {qmdStatus?.persistent_command || '-'}
                    </div>
                  </div>
                  <div className="mt-3 grid gap-3 sm:grid-cols-2">
                    <QMDMetric label={t('memoryQMDSessionExportDir')} value={qmdStatus?.session_export_dir || '-'} />
                    <QMDMetric
                      label={t('memoryQMDSessionsEnabled')}
                      value={qmdStatus?.sessions_enabled ? t('on') : t('off')}
                    />
                  </div>
                  {qmdStatus?.error ? (
                    <div className="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                      {qmdStatus.error}
                    </div>
                  ) : null}
                  <div className="mt-3 rounded-xl border border-dashed border-slate-200 bg-white/60 px-3 py-2 text-xs leading-6 text-slate-500">
                    {t('memoryQMDSessionCleanupHint')}
                  </div>
                </div>

                <div className="space-y-3">
                  {(qmdStatus?.collections ?? []).map((collection) => (
                    <div
                      key={`${collection.Name}-${collection.Path}`}
                      className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4"
                    >
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="rounded-full bg-slate-900 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-white">
                          {collection.Name}
                        </span>
                        <span className="rounded-full bg-white px-2.5 py-1 text-[11px] text-slate-600">
                          {collection.Pattern}
                        </span>
                      </div>
                      <div className="mt-3 break-all font-mono text-xs leading-6 text-slate-700">
                        {collection.Path}
                      </div>
                    </div>
                  ))}
                  {(qmdStatus?.collections ?? []).length === 0 ? (
                    <div className="rounded-2xl border border-dashed border-slate-200 px-4 py-6 text-sm text-slate-500">
                      {t('memoryQMDCollectionsEmpty')}
                    </div>
                  ) : null}
                </div>
              </div>
            )}
          </div>

          <div className="grid gap-4 xl:grid-cols-2">
            <div className="space-y-4">
              <div className="flex items-center justify-between rounded-2xl border border-emerald-100 bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('memoryQMDEnabled')}</Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('memoryQMDEnabledHint')}</div>
                </div>
                <Switch checked={readBool('qmd.enabled')} onCheckedChange={(next) => onChange('qmd.enabled', next)} />
              </div>
              <div className="flex items-center justify-between rounded-2xl border border-emerald-100 bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('memoryQMDIncludeDefault')}</Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('memoryQMDIncludeDefaultHint')}</div>
                </div>
                <Switch checked={readBool('qmd.include_default')} onCheckedChange={(next) => onChange('qmd.include_default', next)} />
              </div>
              <div className="flex items-center justify-between rounded-2xl border border-emerald-100 bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('memoryQMDSessionsEnabled')}</Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('memoryQMDSessionsEnabledHint')}</div>
                </div>
                <Switch checked={readBool('qmd.sessions.enabled')} onCheckedChange={(next) => onChange('qmd.sessions.enabled', next)} />
              </div>
              <div className="flex items-center justify-between rounded-2xl border border-emerald-100 bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('memoryQMDOnBoot')}</Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('memoryQMDOnBootHint')}</div>
                </div>
                <Switch checked={readBool('qmd.update.on_boot')} onCheckedChange={(next) => onChange('qmd.update.on_boot', next)} />
              </div>
            </div>

            <div className="space-y-4">
              <MemoryField label={t('memoryQMDCommand')} hint={t('memoryQMDCommandHint')}>
                <Input
                  className="h-11 rounded-xl bg-white"
                  value={readString('qmd.command')}
                  onChange={(event) => onChange('qmd.command', event.target.value)}
                  placeholder="qmd"
                />
              </MemoryField>
              <div className="grid gap-4 sm:grid-cols-2">
                <MemoryField label={t('memoryQMDSessionExportDir')}>
                  <Input
                    className="h-11 rounded-xl bg-white"
                    value={readString('qmd.sessions.export_dir')}
                    onChange={(event) => onChange('qmd.sessions.export_dir', event.target.value)}
                  />
                </MemoryField>
                <MemoryField label={t('memoryQMDSessionRetentionDays')}>
                  <Input
                    type="number"
                    className="h-11 rounded-xl bg-white"
                    value={String(readNumber('qmd.sessions.retention_days'))}
                    onChange={(event) => onChange('qmd.sessions.retention_days', Number(event.target.value || 0))}
                  />
                </MemoryField>
              </div>
              <div className="grid gap-4 sm:grid-cols-3">
                <MemoryField label={t('memoryQMDInterval')}>
                  <Input
                    className="h-11 rounded-xl bg-white"
                    value={readString('qmd.update.interval')}
                    onChange={(event) => onChange('qmd.update.interval', event.target.value)}
                  />
                </MemoryField>
                <MemoryField label={t('memoryQMDCommandTimeout')}>
                  <Input
                    className="h-11 rounded-xl bg-white"
                    value={readString('qmd.update.command_timeout')}
                    onChange={(event) => onChange('qmd.update.command_timeout', event.target.value)}
                  />
                </MemoryField>
                <MemoryField label={t('memoryQMDUpdateTimeout')}>
                  <Input
                    className="h-11 rounded-xl bg-white"
                    value={readString('qmd.update.update_timeout')}
                    onChange={(event) => onChange('qmd.update.update_timeout', event.target.value)}
                  />
                </MemoryField>
              </div>
            </div>
          </div>

          <MemoryField label={t('memoryQMDPaths')} hint={t('memoryQMDPathsHint')}>
            <textarea
              className="min-h-[140px] w-full rounded-xl border border-input bg-white px-3 py-2 font-mono text-sm leading-6"
              value={readPaths()}
              onChange={(event) => writePaths(event.target.value)}
              spellCheck={false}
              placeholder="docs|/workspace/docs|**/*.md"
            />
          </MemoryField>
        </CardContent>
      </Card>
    </div>
  );
}

function SessionsSectionForm({
  data,
  onChange,
  onRunCleanup,
  cleanupPending,
}: {
  data: Record<string, unknown>;
  onChange: (key: string, value: unknown) => void;
  onRunCleanup: () => void;
  cleanupPending: boolean;
}) {
  const readBool = (path: string) => Boolean(getNestedValue(data, path));
  const readNumber = (path: string) => Number(getNestedValue(data, path) ?? 0);

  return (
    <div className="space-y-5">
      <Card className="border-white/70 bg-[linear-gradient(180deg,rgba(255,250,247,0.95),rgba(255,244,248,0.9))] shadow-[0_24px_60px_-42px_rgba(120,55,75,0.45)]">
        <CardHeader className="pb-4">
          <div className="inline-flex w-fit items-center gap-2 rounded-full bg-[hsl(var(--brand-50))] px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-[hsl(var(--brand-700))]">
            <LibraryBig className="h-3.5 w-3.5" />
            {t('sessionsPersistenceTitle')}
          </div>
          <CardTitle className="text-xl text-[hsl(var(--gray-900))]">{t('sessionsPersistenceHeadline')}</CardTitle>
          <CardDescription>{t('sessionsPersistenceDescription')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
            <div>
              <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('sessionsEnabled')}</Label>
              <div className="mt-1 text-xs text-muted-foreground">{t('sessionsEnabledHint')}</div>
            </div>
            <Switch checked={readBool('enabled')} onCheckedChange={(next) => onChange('enabled', next)} />
          </div>

          <div className="grid gap-4 xl:grid-cols-2">
            <Card className="border-white/70 bg-white/80 shadow-none">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">{t('sessionsSourcesTitle')}</CardTitle>
                <CardDescription>{t('sessionsSourcesHint')}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {[
                  ['sources.cli', t('sessionsSourceCLI'), t('sessionsSourceCLIDesc')],
                  ['sources.tui', t('sessionsSourceTUI'), t('sessionsSourceTUIDesc')],
                  ['sources.webui', t('sessionsSourceWebUI'), t('sessionsSourceWebUIDesc')],
                  ['sources.channels', t('sessionsSourceChannels'), t('sessionsSourceChannelsDesc')],
                  ['sources.heartbeat', t('sessionsSourceHeartbeat'), t('sessionsSourceHeartbeatDesc')],
                  ['sources.cron', t('sessionsSourceCron'), t('sessionsSourceCronDesc')],
                  ['sources.gateway', t('sessionsSourceGateway'), t('sessionsSourceGatewayDesc')],
                ].map(([path, label, hint]) => (
                  <div key={path} className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                    <div>
                      <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{label}</Label>
                      <div className="mt-1 text-xs text-muted-foreground">{hint}</div>
                    </div>
                    <Switch checked={readBool(path)} onCheckedChange={(next) => onChange(path, next)} />
                  </div>
                ))}
              </CardContent>
            </Card>

            <Card className="border-white/70 bg-white/80 shadow-none">
              <CardHeader className="pb-3">
                <CardTitle className="text-base">{t('sessionsContentTitle')}</CardTitle>
                <CardDescription>{t('sessionsContentHint')}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {[
                  ['content.user_messages', t('sessionsContentUserMessages'), t('sessionsContentUserMessagesDesc')],
                  ['content.assistant_messages', t('sessionsContentAssistantMessages'), t('sessionsContentAssistantMessagesDesc')],
                  ['content.system_messages', t('sessionsContentSystemMessages'), t('sessionsContentSystemMessagesDesc')],
                  ['content.tool_calls', t('sessionsContentToolCalls'), t('sessionsContentToolCallsDesc')],
                  ['content.tool_results', t('sessionsContentToolResults'), t('sessionsContentToolResultsDesc')],
                ].map(([path, label, hint]) => (
                  <div key={path} className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                    <div>
                      <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{label}</Label>
                      <div className="mt-1 text-xs text-muted-foreground">{hint}</div>
                    </div>
                    <Switch checked={readBool(path)} onCheckedChange={(next) => onChange(path, next)} />
                  </div>
                ))}
              </CardContent>
            </Card>
          </div>

          <Card className="border-white/70 bg-white/80 shadow-none">
            <CardHeader className="pb-3">
              <CardTitle className="text-base">{t('sessionsCleanupTitle')}</CardTitle>
              <CardDescription>{t('sessionsCleanupHint')}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('sessionsCleanupEnabled')}</Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('sessionsCleanupEnabledDesc')}</div>
                </div>
                <Switch checked={readBool('cleanup.enabled')} onCheckedChange={(next) => onChange('cleanup.enabled', next)} />
              </div>

              <div className="grid gap-3 md:grid-cols-2">
                <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('sessionsCleanupIntervalMinutes')}</Label>
                  <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('sessionsCleanupIntervalMinutesDesc')}</div>
                  <Input
                    type="number"
                    min={1}
                    value={String(readNumber('cleanup.interval_minutes'))}
                    onChange={(event) => onChange('cleanup.interval_minutes', Number(event.target.value || 0))}
                  />
                </div>

                <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('sessionsCleanupMaxAgeDays')}</Label>
                  <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('sessionsCleanupMaxAgeDaysDesc')}</div>
                  <Input
                    type="number"
                    min={1}
                    value={String(readNumber('cleanup.max_age_days'))}
                    onChange={(event) => onChange('cleanup.max_age_days', Number(event.target.value || 0))}
                  />
                </div>
              </div>
            </CardContent>
          </Card>

          <div className="rounded-2xl border border-dashed border-[hsl(var(--gray-200))] bg-white/70 px-4 py-3 text-xs leading-6 text-muted-foreground">
            {t('sessionsDiskHint')}
          </div>
          <div className="flex justify-end">
            <Button type="button" variant="outline" className="rounded-xl" onClick={onRunCleanup} disabled={cleanupPending}>
              <RefreshCw className={`mr-2 h-4 w-4 ${cleanupPending ? 'animate-spin' : ''}`} />
              {cleanupPending ? t('sessionsCleanupRunning') : t('sessionsCleanupNow')}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function StorageSectionForm({
  data,
  onChange,
}: {
  data: Record<string, unknown>;
  onChange: (key: string, value: unknown) => void;
}) {
  const readString = (path: string) => String(getNestedValue(data, path) ?? '');

  return (
    <div className="space-y-5">
      <Card className="border-white/70 bg-[linear-gradient(180deg,rgba(246,249,255,0.95),rgba(242,247,255,0.9))] shadow-[0_24px_60px_-42px_rgba(71,85,132,0.32)]">
        <CardHeader className="pb-4">
          <div className="inline-flex w-fit items-center gap-2 rounded-full bg-sky-50 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-sky-700">
            <FolderKanban className="h-3.5 w-3.5" />
            {t('storageRuntimeTitle')}
          </div>
          <CardTitle className="text-xl text-[hsl(var(--gray-900))]">{t('storageRuntimeHeadline')}</CardTitle>
          <CardDescription>{t('storageRuntimeDescription')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
            <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('storageDatabaseDir')}</Label>
            <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('storageDatabaseDirDesc')}</div>
            <Input
              value={readString('db_dir')}
              onChange={(event) => onChange('db_dir', event.target.value)}
              placeholder="/var/lib/nekobot"
              spellCheck={false}
            />
          </div>

          <div className="rounded-2xl border border-amber-300/80 bg-amber-50 px-4 py-3 text-xs leading-6 text-amber-900">
            {t('storageMigrationHint')}
          </div>

          <div className="rounded-2xl border border-dashed border-[hsl(var(--gray-200))] bg-white/70 px-4 py-3 text-xs leading-6 text-muted-foreground">
            {t('storageMigrationDetail')}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function WatchSectionForm({
  data,
  onChange,
}: {
  data: Record<string, unknown>;
  onChange: (key: string, value: unknown) => void;
}) {
  const { data: watchStatus } = useWatchStatus();
  const readBool = (path: string) => Boolean(getNestedValue(data, path));
  const readNumber = (path: string) => Number(getNestedValue(data, path) ?? 0);
  const readPatterns = (): Array<{ file_glob: string; command: string; fail_command: string }> => {
    const value = getNestedValue(data, 'patterns');
    if (!Array.isArray(value)) {
      return [];
    }
    return value.map((item) => {
      if (!item || typeof item !== 'object' || Array.isArray(item)) {
        return { file_glob: '', command: '', fail_command: '' };
      }
      const record = item as Record<string, unknown>;
      return {
        file_glob: typeof record.file_glob === 'string' ? record.file_glob : '',
        command: typeof record.command === 'string' ? record.command : '',
        fail_command: typeof record.fail_command === 'string' ? record.fail_command : '',
      };
    });
  };

  const patterns = readPatterns();

  const updatePattern = (index: number, patch: Partial<{ file_glob: string; command: string; fail_command: string }>) => {
    const next = patterns.map((item, itemIndex) => (itemIndex === index ? { ...item, ...patch } : item));
    onChange('patterns', next);
  };

  const addPattern = () => {
    onChange('patterns', [...patterns, { file_glob: '', command: '', fail_command: '' }]);
  };

  const removePattern = (index: number) => {
    onChange('patterns', patterns.filter((_, itemIndex) => itemIndex !== index));
  };

  return (
    <div className="space-y-5">
      <Card className="border-white/70 bg-[linear-gradient(180deg,rgba(247,250,255,0.95),rgba(241,246,255,0.9))] shadow-[0_24px_60px_-42px_rgba(71,85,132,0.35)]">
        <CardHeader className="pb-4">
          <div className="inline-flex w-fit items-center gap-2 rounded-full bg-sky-50 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-sky-700">
            <RefreshCw className="h-3.5 w-3.5" />
            {t('configSectionWatch')}
          </div>
          <CardTitle className="text-xl text-[hsl(var(--gray-900))]">{t('configSectionWatch')}</CardTitle>
          <CardDescription>{t('configSectionDescWatch')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
              <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{t('watchRuntimeState')}</div>
              <div className="mt-2 flex items-center gap-2 text-sm font-semibold text-[hsl(var(--gray-900))]">
                <span className={cn('h-2.5 w-2.5 rounded-full', watchStatus?.enabled && watchStatus?.running ? 'bg-emerald-500' : watchStatus?.enabled ? 'bg-amber-500' : 'bg-slate-400')} />
                <span>
                  {watchStatus?.enabled
                    ? watchStatus?.running
                      ? t('watchRuntimeRunning')
                      : t('watchRuntimeConfigured')
                    : t('watchRuntimeDisabled')}
                </span>
              </div>
            </div>
            <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
              <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{t('watchRuntimeLastFile')}</div>
              <div className="mt-2 break-all text-sm font-semibold text-[hsl(var(--gray-900))]">
                {watchStatus?.last_file || '-'}
              </div>
            </div>
            <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
              <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{t('watchRuntimeLastCommand')}</div>
              <div className="mt-2 break-all text-sm font-semibold text-[hsl(var(--gray-900))]">
                {watchStatus?.last_command || '-'}
              </div>
            </div>
            <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
              <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{t('watchRuntimeLastResult')}</div>
              <div className="mt-2 text-sm font-semibold text-[hsl(var(--gray-900))]">
                {watchStatus?.last_error
                  ? t('watchRuntimeResultFailed')
                  : watchStatus?.last_result_preview
                    ? t('watchRuntimeResultSucceeded')
                    : '-'}
              </div>
            </div>
          </div>

          {(watchStatus?.last_run_at || watchStatus?.last_error || watchStatus?.last_result_preview) && (
            <Card className="border-white/70 bg-white/80 shadow-none">
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <CardTitle className="text-base">{t('watchRuntimeTitle')}</CardTitle>
                    <CardDescription>{t('watchRuntimeDescription')}</CardDescription>
                  </div>
                  <Button asChild type="button" variant="outline" className="rounded-full">
                    <a href="/harness/audit">
                      <ShieldCheck className="mr-2 h-4 w-4" />
                      {t('watchOpenAudit')}
                    </a>
                  </Button>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                {watchStatus?.last_run_at && (
                  <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                    <div className="text-xs uppercase tracking-[0.16em] text-muted-foreground">{t('watchRuntimeLastRunAt')}</div>
                    <div className="mt-2 text-sm text-[hsl(var(--gray-900))]">{new Date(watchStatus.last_run_at).toLocaleString()}</div>
                  </div>
                )}
                {watchStatus?.last_result_preview && (
                  <div className="rounded-2xl border border-emerald-200/70 bg-emerald-50/80 p-4 text-sm text-emerald-950">
                    {watchStatus.last_result_preview}
                  </div>
                )}
                {watchStatus?.last_error && (
                  <div className="rounded-2xl border border-rose-200/70 bg-rose-50/80 p-4 text-sm text-rose-950">
                    {watchStatus.last_error}
                  </div>
                )}
              </CardContent>
            </Card>
          )}

          <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_220px]">
            <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
              <div>
                <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('watchEnabledTitle')}</Label>
                <div className="mt-1 text-xs text-muted-foreground">{t('watchEnabledHint')}</div>
              </div>
              <Switch checked={readBool('enabled')} onCheckedChange={(next) => onChange('enabled', next)} />
            </div>
            <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
              <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('watchDebounceMs')}</Label>
              <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('watchDebounceMsHint')}</div>
              <Input
                type="number"
                min={0}
                className="h-11 rounded-xl bg-white"
                value={String(readNumber('debounce_ms'))}
                onChange={(event) => onChange('debounce_ms', Number(event.target.value || 0))}
              />
            </div>
          </div>

          <Card className="border-white/70 bg-white/80 shadow-none">
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <CardTitle className="text-base">{t('watchPatternsTitle')}</CardTitle>
                  <CardDescription>{t('watchPatternsHint')}</CardDescription>
                </div>
                <Button type="button" variant="outline" className="rounded-full" onClick={addPattern}>
                  {t('add')}
                </Button>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {patterns.length === 0 ? (
                <div className="rounded-2xl border border-dashed border-[hsl(var(--gray-200))] px-4 py-6 text-sm text-muted-foreground">
                  {t('watchPatternsEmpty')}
                </div>
              ) : (
                patterns.map((pattern, index) => (
                  <div key={index} className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                    <div className="mb-4 flex items-center justify-between gap-3">
                      <div className="text-sm font-semibold text-[hsl(var(--gray-900))]">
                        {t('watchPatternLabel', String(index + 1))}
                      </div>
                      <Button type="button" variant="outline" size="sm" className="rounded-full" onClick={() => removePattern(index)}>
                        {t('remove')}
                      </Button>
                    </div>
                    <div className="grid gap-4">
                      <div>
                        <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('watchPatternGlob')}</Label>
                        <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('watchPatternGlobHint')}</div>
                        <Input
                          className="h-11 rounded-xl bg-white"
                          value={pattern.file_glob}
                          onChange={(event) => updatePattern(index, { file_glob: event.target.value })}
                          placeholder="pkg/**/*.go"
                        />
                      </div>
                      <div>
                        <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('watchPatternCommand')}</Label>
                        <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('watchPatternCommandHint')}</div>
                        <Input
                          className="h-11 rounded-xl bg-white"
                          value={pattern.command}
                          onChange={(event) => updatePattern(index, { command: event.target.value })}
                          placeholder="go test ./..."
                        />
                      </div>
                      <div>
                        <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('watchPatternFailCommand')}</Label>
                        <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('watchPatternFailCommandHint')}</div>
                        <Input
                          className="h-11 rounded-xl bg-white"
                          value={pattern.fail_command}
                          onChange={(event) => updatePattern(index, { fail_command: event.target.value })}
                          placeholder="notify-send 'watch failed'"
                        />
                      </div>
                    </div>
                  </div>
                ))
              )}
            </CardContent>
          </Card>
        </CardContent>
      </Card>
    </div>
  );
}

function WebUISectionForm({
  data,
  runtimeTransports,
  onChange,
  onCleanupToolSessionEvents,
  cleanupEventsPending,
  onCleanupSkillVersions,
  cleanupSkillVersionsPending,
}: {
  data: Record<string, unknown>;
  runtimeTransports: Array<{ name: string; available: boolean; is_default: boolean }>;
  onChange: (key: string, value: unknown) => void;
  onCleanupToolSessionEvents: () => void;
  cleanupEventsPending: boolean;
  onCleanupSkillVersions: () => void;
  cleanupSkillVersionsPending: boolean;
}) {
  const readBool = (path: string) => Boolean(getNestedValue(data, path));
  const readNumber = (path: string) => Number(getNestedValue(data, path) ?? 0);
  const readString = (path: string) => String(getNestedValue(data, path) ?? '');

  return (
    <div className="space-y-5">
      <Card className="border-white/70 bg-[linear-gradient(180deg,rgba(247,250,255,0.95),rgba(241,246,255,0.9))] shadow-[0_24px_60px_-42px_rgba(71,85,132,0.35)]">
        <CardHeader className="pb-4">
          <div className="inline-flex w-fit items-center gap-2 rounded-full bg-sky-50 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-sky-700">
            <Layers3 className="h-3.5 w-3.5" />
            {t('webuiRuntimeTitle')}
          </div>
          <CardTitle className="text-xl text-[hsl(var(--gray-900))]">{t('webuiRuntimeHeadline')}</CardTitle>
          <CardDescription>{t('webuiRuntimeDescription')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
              <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('webuiPublicBaseUrl')}</Label>
              <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('webuiPublicBaseUrlDesc')}</div>
              <Input
                value={readString('public_base_url')}
                onChange={(event) => onChange('public_base_url', event.target.value)}
                placeholder="https://example.com"
              />
            </div>
            <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
              <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('webuiToolSessionOTPSeconds')}</Label>
              <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('webuiToolSessionOTPSecondsDesc')}</div>
              <Input
                type="number"
                min={0}
                value={String(readNumber('tool_session_otp_ttl_seconds'))}
                onChange={(event) => onChange('tool_session_otp_ttl_seconds', Number(event.target.value || 0))}
              />
            </div>
          </div>

          <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
            <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('webuiToolSessionRuntimeTransport')}</Label>
            <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('webuiToolSessionRuntimeTransportDesc')}</div>
            <Select
              value={readString('tool_session_runtime_transport') || 'tmux'}
              onValueChange={(value) => onChange('tool_session_runtime_transport', value)}
            >
              <SelectTrigger className="h-11 rounded-xl bg-white">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="tmux">{t('runtimeTransportTmux')}</SelectItem>
                <SelectItem value="zellij">{t('runtimeTransportZellij')}</SelectItem>
              </SelectContent>
            </Select>
            {runtimeTransports.length > 0 ? (
              <div className="mt-3 flex flex-wrap gap-2">
                {runtimeTransports.map((item) => (
                  <span
                    key={item.name}
                    className={`inline-flex items-center rounded-full px-2.5 py-1 text-[11px] font-medium ${
                      item.available
                        ? 'bg-emerald-500/10 text-emerald-700'
                        : 'bg-amber-500/12 text-amber-700'
                    }`}
                  >
                    {item.name} · {item.available ? t('runtimeTransportAvailable') : t('runtimeTransportUnavailable')}
                    {item.is_default ? ' · default' : ''}
                  </span>
                ))}
              </div>
            ) : null}
            {(() => {
              const selected = runtimeTransports.find(
                (item) => item.name === (readString('tool_session_runtime_transport') || 'tmux'),
              );
              return selected && !selected.available ? (
                <div className="mt-3 text-xs text-amber-700">
                  {t('runtimeTransportUnavailableWarning', selected.name)}
                </div>
              ) : null;
            })()}
          </div>

          <Card className="border-white/70 bg-white/80 shadow-none">
            <CardHeader className="pb-3">
              <CardTitle className="text-base">{t('webuiToolSessionEventsTitle')}</CardTitle>
              <CardDescription>{t('webuiToolSessionEventsDesc')}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('webuiToolSessionEventsEnabled')}</Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('webuiToolSessionEventsEnabledDesc')}</div>
                </div>
                <Switch checked={readBool('tool_session_events.enabled')} onCheckedChange={(next) => onChange('tool_session_events.enabled', next)} />
              </div>

              <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('webuiToolSessionEventsRetentionDays')}</Label>
                <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('webuiToolSessionEventsRetentionDaysDesc')}</div>
                <Input
                  type="number"
                  min={1}
                  value={String(readNumber('tool_session_events.retention_days'))}
                  onChange={(event) => onChange('tool_session_events.retention_days', Number(event.target.value || 0))}
                />
              </div>
              <div className="flex justify-end">
                <Button
                  type="button"
                  variant="outline"
                  className="rounded-xl"
                  onClick={onCleanupToolSessionEvents}
                  disabled={cleanupEventsPending}
                >
                  <RefreshCw className={`mr-2 h-4 w-4 ${cleanupEventsPending ? 'animate-spin' : ''}`} />
                  {cleanupEventsPending ? t('webuiToolSessionEventsCleanupRunning') : t('webuiToolSessionEventsCleanupNow')}
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card className="border-white/70 bg-white/80 shadow-none">
            <CardHeader className="pb-3">
              <CardTitle className="text-base">{t('webuiSkillSnapshotsTitle')}</CardTitle>
              <CardDescription>{t('webuiSkillSnapshotsDesc')}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('webuiSkillSnapshotsAutoPrune')}</Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('webuiSkillSnapshotsAutoPruneDesc')}</div>
                </div>
                <Switch checked={readBool('skill_snapshots.auto_prune')} onCheckedChange={(next) => onChange('skill_snapshots.auto_prune', next)} />
              </div>

              <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('webuiSkillSnapshotsMaxCount')}</Label>
                <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('webuiSkillSnapshotsMaxCountDesc')}</div>
                <Input
                  type="number"
                  min={1}
                  value={String(readNumber('skill_snapshots.max_count'))}
                  onChange={(event) => onChange('skill_snapshots.max_count', Number(event.target.value || 0))}
                />
              </div>
            </CardContent>
          </Card>

          <Card className="border-white/70 bg-white/80 shadow-none">
            <CardHeader className="pb-3">
              <CardTitle className="text-base">{t('webuiSkillVersionsTitle')}</CardTitle>
              <CardDescription>{t('webuiSkillVersionsDesc')}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center justify-between rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                <div>
                  <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('webuiSkillVersionsEnabled')}</Label>
                  <div className="mt-1 text-xs text-muted-foreground">{t('webuiSkillVersionsEnabledDesc')}</div>
                </div>
                <Switch checked={readBool('skill_versions.enabled')} onCheckedChange={(next) => onChange('skill_versions.enabled', next)} />
              </div>

              <div className="rounded-2xl border border-[hsl(var(--gray-200))] bg-white/82 p-4">
                <Label className="text-sm font-semibold text-[hsl(var(--gray-900))]">{t('webuiSkillVersionsMaxCount')}</Label>
                <div className="mt-1 mb-3 text-xs text-muted-foreground">{t('webuiSkillVersionsMaxCountDesc')}</div>
                <Input
                  type="number"
                  min={1}
                  value={String(readNumber('skill_versions.max_count'))}
                  onChange={(event) => onChange('skill_versions.max_count', Number(event.target.value || 0))}
                />
              </div>

              <div className="flex justify-end">
                <Button
                  type="button"
                  variant="outline"
                  className="rounded-xl"
                  onClick={onCleanupSkillVersions}
                  disabled={cleanupSkillVersionsPending}
                >
                  <RefreshCw className={`mr-2 h-4 w-4 ${cleanupSkillVersionsPending ? 'animate-spin' : ''}`} />
                  {cleanupSkillVersionsPending ? t('webuiSkillVersionsCleanupRunning') : t('webuiSkillVersionsCleanupNow')}
                </Button>
              </div>
            </CardContent>
          </Card>

          <div className="rounded-2xl border border-dashed border-[hsl(var(--gray-200))] bg-white/70 px-4 py-3 text-xs leading-6 text-muted-foreground">
            {t('webuiDiskHint')}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export default function ConfigPage() {
  const [section, setSection] = useState<ConfigSection>('agents');
  const [mode, setMode] = useState<'form' | 'json'>('form');
  const [search, setSearch] = useState('');
  const [drafts, setDrafts] = useState<Partial<Record<ConfigSection, Record<string, unknown>>>>({});
  const [jsonDrafts, setJSONDrafts] = useState<Partial<Record<ConfigSection, string>>>({});
  const [visibleSecrets, setVisibleSecrets] = useState<Record<string, boolean>>({});
  const fileInputRef = useRef<HTMLInputElement>(null);

  const configQuery = useConfig();
  const runtimeTransportsQuery = useToolSessionRuntimeTransports();
  const { data: config, isLoading } = configQuery;
  const saveConfig = useSaveConfig();
  const exportConfig = useExportConfig();
  const importConfig = useImportConfig();
  const cleanupSessions = useCleanupSessions();
  const cleanupToolSessionEvents = useCleanupToolSessionEvents();
  const cleanupSkillVersions = useCleanupSkillVersions();
  const hasBlockingConfigError = configQuery.isError && configQuery.data == null;

  if (hasBlockingConfigError) {
    return (
      <div className="config-page flex h-full flex-col">
        <Header title={t('tabConfig')} />
        <ConfigLoadErrorState
          message={formatQueryErrorMessage(configQuery.error)}
          onRetry={() => {
            void configQuery.refetch();
          }}
          retrying={configQuery.isFetching}
        />
      </div>
    );
  }

  const persistedSections = useMemo(() => {
    const next = {} as Record<ConfigSection, Record<string, unknown>>;
    for (const item of CONFIG_SECTIONS) {
      next[item] = (config?.[item] as Record<string, unknown>) ?? {};
    }
    return next;
  }, [config]);

  const currentDraft = drafts[section];
  const currentData = currentDraft ?? persistedSections[section];
  const currentJSON = jsonDrafts[section] ?? stableStringify(currentData);
  const fields = useMemo(() => {
    const inferred = inferFields(currentData);
    if (section !== 'agents') {
      return inferred;
    }
    return inferred.filter((field) => !MANAGED_AGENT_FIELDS.has(field.key));
  }, [currentData, section]);
  const filteredFields = useMemo(() => filterFields(fields, search), [fields, search]);

  const sectionDirty = useMemo(() => {
    const result = {} as Record<ConfigSection, boolean>;
    for (const item of CONFIG_SECTIONS) {
      const draft = drafts[item];
      const draftDirty = draft ? stableStringify(draft) !== stableStringify(persistedSections[item]) : false;
      const jsonDraft = jsonDrafts[item];
      const jsonBase = stableStringify(draft ?? persistedSections[item]);
      const jsonDirty = typeof jsonDraft === 'string' ? jsonDraft !== jsonBase : false;
      result[item] = draftDirty || jsonDirty;
    }
    return result;
  }, [drafts, jsonDrafts, persistedSections]);

  const dirtyCount = CONFIG_SECTIONS.filter((item) => sectionDirty[item]).length;

  useEffect(() => {
    function handleBeforeUnload(event: BeforeUnloadEvent) {
      if (dirtyCount === 0) {
        return;
      }
      event.preventDefault();
      event.returnValue = '';
    }

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [dirtyCount]);

  const updateDraft = useCallback((targetSection: ConfigSection, updater: (base: Record<string, unknown>) => Record<string, unknown>) => {
    setDrafts((prev) => {
      const base = cloneSection(prev[targetSection] ?? persistedSections[targetSection]);
      return { ...prev, [targetSection]: updater(base) };
    });
  }, [persistedSections]);

  const handleFieldChange = useCallback((path: string, value: unknown) => {
    setJSONDrafts((prev) => {
      if (!(section in prev)) {
        return prev;
      }
      const next = { ...prev };
      delete next[section];
      return next;
    });
    updateDraft(section, (base) => {
      const next = cloneSection(base);
      setNestedValue(next, path, value);
      return next;
    });
  }, [section, updateDraft]);

  const handleJSONChange = useCallback((value: string) => {
    setJSONDrafts((prev) => ({ ...prev, [section]: value }));
  }, [section]);

  const handleSectionChange = useCallback((nextSection: ConfigSection) => {
    setSection(nextSection);
    setSearch('');
    setMode('form');
  }, []);

  const handleToggleMode = useCallback(() => {
    if (mode === 'form') {
      setJSONDrafts((prev) => {
        if (typeof prev[section] === 'string') {
          return prev;
        }
        return { ...prev, [section]: stableStringify(currentData) };
      });
      setMode('json');
      return;
    }

    try {
      const parsed = JSON.parse(currentJSON) as Record<string, unknown>;
      setDrafts((prev) => ({ ...prev, [section]: parsed }));
      setMode('form');
    } catch (error) {
      toast.error(t('invalidJson', String(error)));
    }
  }, [currentData, currentJSON, mode, section]);

  const handleSave = useCallback(async () => {
    let payload: Record<string, unknown>;
    if (mode === 'json') {
      try {
        payload = JSON.parse(currentJSON) as Record<string, unknown>;
      } catch (error) {
        toast.error(t('invalidJson', String(error)));
        return;
      }
    } else {
      payload = cloneSection(currentData);
    }

    try {
      await saveConfig.mutateAsync({ [section]: payload });
      setDrafts((prev) => {
        const next = { ...prev };
        delete next[section];
        return next;
      });
      setJSONDrafts((prev) => {
        const next = { ...prev };
        delete next[section];
        return next;
      });
    } catch {
      // Hook handles the error toast.
    }
  }, [currentData, currentJSON, mode, saveConfig, section]);

  const handleReset = useCallback(() => {
    setDrafts((prev) => {
      const next = { ...prev };
      delete next[section];
      return next;
    });
    setJSONDrafts((prev) => {
      const next = { ...prev };
      delete next[section];
      return next;
    });
    setSearch('');
    setMode('form');
  }, [section]);

  const handleImport = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }

    const reader = new FileReader();
    reader.onload = () => {
      try {
        const payload = JSON.parse(reader.result as string) as ConfigShape;
        importConfig.mutate(payload, {
          onSuccess: () => {
            setDrafts({});
            setJSONDrafts({});
          },
        });
      } catch {
        toast.error(t('importFailed'));
      }
    };
    reader.readAsText(file);
    event.target.value = '';
  }, [importConfig]);

  const toggleSecretVisibility = useCallback((path: string) => {
    setVisibleSecrets((prev) => ({ ...prev, [path]: !prev[path] }));
  }, []);

  return (
    <div className="config-page flex h-full flex-col">
      <Header title={t('tabConfig')} />
      <div className="grid min-h-0 flex-1 gap-4 px-4 pb-4 md:px-5 md:pb-5 xl:grid-cols-[252px_minmax(0,1fr)]">
        <Card className="overflow-hidden border-border/70 bg-card/92 shadow-[0_24px_60px_-42px_rgba(120,55,75,0.28)] xl:block">
          <CardHeader className="border-b border-border/70 pb-4 sm:pb-5">
            <CardTitle className="text-lg text-foreground sm:text-xl">{t('configControlTitle')}</CardTitle>
            <CardDescription className="text-sm leading-6">{t('configPageDescription')}</CardDescription>
          </CardHeader>
          <CardContent className="grid auto-cols-[minmax(180px,1fr)] grid-flow-col gap-3 overflow-x-auto p-4 md:grid-cols-2 md:grid-flow-row xl:grid-cols-1 xl:grid-flow-row">
            {CONFIG_SECTIONS.map((item) => (
              <button
                key={item}
                type="button"
                onClick={() => handleSectionChange(item)}
                className={
                  item === section
                    ? 'min-h-[80px] w-full rounded-2xl border border-primary/30 bg-primary/10 px-4 py-3 text-left shadow-[0_18px_36px_-28px_rgba(120,55,75,0.24)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
                    : 'min-h-[80px] w-full rounded-2xl border border-transparent bg-muted/35 px-4 py-3 text-left transition hover:border-border/70 hover:bg-muted/55 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
                }
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="text-sm font-semibold text-foreground">{sectionLabel(item)}</div>
                  <DirtyDot dirty={sectionDirty[item]} />
                </div>
                <div className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{sectionDescription(item)}</div>
              </button>
            ))}
          </CardContent>
        </Card>

        <Card className="min-h-0 overflow-hidden border-border/70 bg-card/88 shadow-[0_26px_80px_-48px_rgba(120,55,75,0.28)] backdrop-blur">
          <CardHeader className="border-b border-border/70 bg-[linear-gradient(135deg,hsl(var(--card)/0.98),hsl(var(--muted)/0.7))]">
            <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
              <div className="space-y-2">
                <div className="eyebrow-label inline-flex items-center gap-2 rounded-full bg-primary/10 px-3 py-1 text-primary">
                  <DirtyDot dirty={sectionDirty[section]} />
                  {sectionLabel(section)}
                </div>
                <CardTitle className="text-2xl text-foreground">{sectionLabel(section)}</CardTitle>
                <CardDescription>{sectionDescription(section)}</CardDescription>
              </div>

              <div className="grid w-full gap-2 sm:flex sm:flex-wrap sm:items-center xl:w-auto">
                <div className="relative min-w-0 w-full sm:min-w-[220px] sm:flex-1">
                  <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    aria-label={t('configSearchPlaceholder')}
                    className="h-10 rounded-full border-border/70 bg-background/80 pl-9"
                    placeholder={t('configSearchPlaceholder')}
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                  />
                </div>
                <Select value={mode} onValueChange={(value) => value !== mode && handleToggleMode()}>
                  <SelectTrigger className="h-10 w-full sm:w-[120px] rounded-full border-border/70 bg-background/80">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="form">
                      <div className="flex items-center gap-2">
                        <FormInput className="h-4 w-4" />
                        {t('configFormMode')}
                      </div>
                    </SelectItem>
                    <SelectItem value="json">
                      <div className="flex items-center gap-2">
                        <Code className="h-4 w-4" />
                        {t('configJsonMode')}
                      </div>
                    </SelectItem>
                  </SelectContent>
                </Select>
                <Button variant="outline" size="sm" className="h-10 rounded-full justify-center" onClick={handleReset} disabled={!sectionDirty[section]}>
                  <RotateCcw className="mr-1.5 h-4 w-4" />
                  {t('reset')}
                </Button>
                <Button size="sm" className="h-10 rounded-full justify-center" onClick={handleSave} disabled={saveConfig.isPending || !sectionDirty[section]}>
                  <Save className="mr-1.5 h-4 w-4" />
                  {t('save')}
                </Button>
                <Button variant="outline" size="sm" className="h-10 rounded-full justify-center" onClick={() => exportConfig.mutate()} disabled={exportConfig.isPending}>
                  <Download className="mr-1.5 h-4 w-4" />
                  {t('exportConfig')}
                </Button>
                <Button variant="outline" size="sm" className="h-10 rounded-full justify-center" onClick={() => fileInputRef.current?.click()} disabled={importConfig.isPending}>
                  <Upload className="mr-1.5 h-4 w-4" />
                  {t('importConfig')}
                </Button>
                <input ref={fileInputRef} type="file" accept=".json" className="hidden" onChange={handleImport} />
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
              <span>{sectionPersistenceHint(section)}</span>
              <span className="rounded-full bg-muted px-2.5 py-1">
                {dirtyCount > 0 ? t('configDraftSections', String(dirtyCount)) : t('configNoUnsavedDrafts')}
              </span>
            </div>
          </CardHeader>

          <CardContent className="min-h-0 p-0">
            <ScrollArea className="h-[calc(100dvh-16rem)] px-4 py-4 md:px-6 md:py-5 xl:h-[calc(100dvh-14.5rem)]">
              {isLoading ? (
                <div className="py-12 text-center text-sm text-muted-foreground">{t('loading')}</div>
              ) : mode === 'json' ? (
                <div className="space-y-3">
                  <div className="rounded-2xl border border-amber-200 bg-[rgba(255,248,239,0.92)] px-4 py-3 text-sm text-amber-800">
                    {t('configJsonModeHint')}
                  </div>
                  <textarea
                    className="mono-data min-h-[62vh] w-full rounded-[1.5rem] border border-[hsl(var(--gray-200))] bg-[hsl(var(--gray-950))] px-5 py-4 text-sm leading-6 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]"
                    value={currentJSON}
                    onChange={(event) => handleJSONChange(event.target.value)}
                    spellCheck={false}
                  />
                </div>
              ) : section === 'storage' ? (
                <StorageSectionForm data={currentData} onChange={handleFieldChange} />
              ) : section === 'agents' ? (
                <AgentsSectionForm data={currentData} onChange={handleFieldChange} />
              ) : section === 'memory' ? (
                <MemorySectionForm data={currentData} onChange={handleFieldChange} />
              ) : section === 'sessions' ? (
                <SessionsSectionForm
                  data={currentData}
                  onChange={handleFieldChange}
                  onRunCleanup={() => cleanupSessions.mutate()}
                  cleanupPending={cleanupSessions.isPending}
                />
              ) : section === 'webui' ? (
                <WebUISectionForm
                  data={currentData}
                  runtimeTransports={runtimeTransportsQuery.data ?? []}
                  onChange={handleFieldChange}
                  onCleanupToolSessionEvents={() => cleanupToolSessionEvents.mutate()}
                  cleanupEventsPending={cleanupToolSessionEvents.isPending}
                  onCleanupSkillVersions={() => cleanupSkillVersions.mutate()}
                  cleanupSkillVersionsPending={cleanupSkillVersions.isPending}
                />
              ) : section === 'watch' ? (
                <WatchSectionForm data={currentData} onChange={handleFieldChange} />
              ) : filteredFields.length === 0 ? (
                <div className="py-16 text-center">
                  <div className="text-sm font-medium text-foreground">{t('configNoMatchingFields')}</div>
                  <div className="mt-1 text-sm text-muted-foreground">
                    {search.trim() ? t('configNoMatchingFieldsHint') : t('configNoEditableFields')}
                  </div>
                </div>
              ) : (
                <div className="grid gap-4 2xl:grid-cols-2">
                  {filteredFields.map((field) => (
                    <FormField
                      key={field.key}
                      field={field}
                      secretVisible={Boolean(visibleSecrets[field.key])}
                      onChange={handleFieldChange}
                      onToggleSecret={toggleSecretVisibility}
                      onOpenJSONMode={() => {
                        setJSONDrafts((prev) => {
                          if (typeof prev[section] === 'string') {
                            return prev;
                          }
                          return { ...prev, [section]: stableStringify(currentData) };
                        });
                        setMode('json');
                      }}
                    />
                  ))}
                </div>
              )}
            </ScrollArea>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
