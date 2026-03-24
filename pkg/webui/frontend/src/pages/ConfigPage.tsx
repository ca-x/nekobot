import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { t } from '@/lib/i18n';
import { toast } from 'sonner';
import Header from '@/components/layout/Header';
import { useConfig, useExportConfig, useImportConfig, useSaveConfig } from '@/hooks/useConfig';
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
  RotateCcw,
  Save,
  Search,
  Upload,
} from 'lucide-react';

const CONFIG_SECTIONS = [
  'agents',
  'gateway',
  'tools',
  'transcription',
  'memory',
  'heartbeat',
  'approval',
  'logger',
  'webui',
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

const SECTION_DESCRIPTIONS: Record<ConfigSection, string> = {
  agents: 'Agent defaults, model routing, workspace and MCP integration.',
  gateway: 'Gateway listen host and service ports.',
  tools: 'Web tool behavior, exec timeout and sandbox settings.',
  transcription: 'Speech-to-text provider, API base, model and timeout.',
  memory: 'Long-term, semantic and episodic memory controls.',
  heartbeat: 'Autonomous heartbeat interval and task cadence.',
  approval: 'Allow/deny policy and approval mode.',
  logger: 'Logging level, output target and rotation strategy.',
  webui: 'Dashboard port and interactive session settings.',
};

function sectionLabel(section: ConfigSection): string {
  const labels: Record<ConfigSection, string> = {
    agents: t('configSectionAgents'),
    gateway: t('configSectionGateway'),
    tools: t('configSectionTools'),
    transcription: t('configSectionTranscription'),
    memory: t('configSectionMemory'),
    heartbeat: t('configSectionHeartbeat'),
    approval: t('configSectionApproval'),
    logger: t('configSectionLogger'),
    webui: t('configSectionWebUI'),
  };
  return labels[section];
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
          <div className="mt-2 text-xs text-muted-foreground">One item per line.</div>
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
              JSON field. Edit this section in JSON mode to preserve the structure.
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

export default function ConfigPage() {
  const [section, setSection] = useState<ConfigSection>('agents');
  const [mode, setMode] = useState<'form' | 'json'>('form');
  const [search, setSearch] = useState('');
  const [drafts, setDrafts] = useState<Partial<Record<ConfigSection, Record<string, unknown>>>>({});
  const [jsonDrafts, setJSONDrafts] = useState<Partial<Record<ConfigSection, string>>>({});
  const [visibleSecrets, setVisibleSecrets] = useState<Record<string, boolean>>({});
  const fileInputRef = useRef<HTMLInputElement>(null);

  const { data: config, isLoading } = useConfig();
  const saveConfig = useSaveConfig();
  const exportConfig = useExportConfig();
  const importConfig = useImportConfig();

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
  const fields = useMemo(() => inferFields(currentData), [currentData]);
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
    <div className="flex h-full flex-col">
      <Header title={t('tabConfig')} />
      <div className="grid min-h-0 flex-1 gap-5 px-6 pb-6 lg:grid-cols-[280px_minmax(0,1fr)]">
        <Card className="overflow-hidden border-white/70 bg-[linear-gradient(180deg,rgba(255,250,247,0.96),rgba(252,242,246,0.9))] shadow-[0_24px_60px_-42px_rgba(120,55,75,0.5)]">
          <CardHeader className="border-b border-white/60 pb-5">
            <CardTitle className="text-xl text-[hsl(var(--gray-900))]">Config Control</CardTitle>
            <CardDescription>Database-backed runtime settings. Drafts stay local until saved.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 p-4">
            {CONFIG_SECTIONS.map((item) => (
              <button
                key={item}
                type="button"
                onClick={() => handleSectionChange(item)}
                className={
                  item === section
                    ? 'w-full rounded-2xl border border-[hsl(var(--brand-300))] bg-white/90 px-4 py-3 text-left shadow-[0_18px_36px_-28px_rgba(120,55,75,0.4)]'
                    : 'w-full rounded-2xl border border-transparent bg-white/55 px-4 py-3 text-left transition hover:border-white hover:bg-white/80'
                }
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="text-sm font-semibold text-[hsl(var(--gray-900))]">{sectionLabel(item)}</div>
                  <DirtyDot dirty={sectionDirty[item]} />
                </div>
                <div className="mt-1 text-xs leading-5 text-muted-foreground">{SECTION_DESCRIPTIONS[item]}</div>
              </button>
            ))}
          </CardContent>
        </Card>

        <Card className="min-h-0 overflow-hidden border-white/70 bg-white/78 shadow-[0_26px_80px_-48px_rgba(120,55,75,0.5)] backdrop-blur">
          <CardHeader className="border-b border-[hsl(var(--gray-200))]/80 bg-[linear-gradient(135deg,rgba(255,248,245,0.95),rgba(255,241,246,0.92))]">
            <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
              <div className="space-y-2">
                <div className="inline-flex items-center gap-2 rounded-full bg-[hsl(var(--brand-50))] px-3 py-1 text-[11px] font-medium uppercase tracking-[0.22em] text-[hsl(var(--brand-700))]">
                  <DirtyDot dirty={sectionDirty[section]} />
                  {sectionLabel(section)}
                </div>
                <CardTitle className="text-2xl text-[hsl(var(--gray-900))]">{sectionLabel(section)}</CardTitle>
                <CardDescription>{SECTION_DESCRIPTIONS[section]}</CardDescription>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <div className="relative min-w-[220px] flex-1">
                  <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    className="h-10 rounded-full border-white bg-white/85 pl-9"
                    placeholder="Search fields"
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                  />
                </div>
                <Select value={mode} onValueChange={(value) => value !== mode && handleToggleMode()}>
                  <SelectTrigger className="h-10 w-[120px] rounded-full border-white bg-white/85">
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
                <Button variant="outline" size="sm" className="rounded-full" onClick={handleReset} disabled={!sectionDirty[section]}>
                  <RotateCcw className="mr-1.5 h-4 w-4" />
                  {t('reset')}
                </Button>
                <Button size="sm" className="rounded-full" onClick={handleSave} disabled={saveConfig.isPending || !sectionDirty[section]}>
                  <Save className="mr-1.5 h-4 w-4" />
                  {t('save')}
                </Button>
                <Button variant="outline" size="sm" className="rounded-full" onClick={() => exportConfig.mutate()} disabled={exportConfig.isPending}>
                  <Download className="mr-1.5 h-4 w-4" />
                  {t('exportConfig')}
                </Button>
                <Button variant="outline" size="sm" className="rounded-full" onClick={() => fileInputRef.current?.click()} disabled={importConfig.isPending}>
                  <Upload className="mr-1.5 h-4 w-4" />
                  {t('importConfig')}
                </Button>
                <input ref={fileInputRef} type="file" accept=".json" className="hidden" onChange={handleImport} />
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
              <span>{t('configSectionHint', sectionLabel(section))}</span>
              <span className="rounded-full bg-[hsl(var(--gray-100))] px-2.5 py-1">
                {dirtyCount > 0 ? `${dirtyCount} draft section(s)` : 'No unsaved drafts'}
              </span>
            </div>
          </CardHeader>

          <CardContent className="min-h-0 p-0">
            <ScrollArea className="h-[calc(100vh-15rem)] px-6 py-5">
              {isLoading ? (
                <div className="py-12 text-center text-sm text-muted-foreground">Loading…</div>
              ) : mode === 'json' ? (
                <div className="space-y-3">
                  <div className="rounded-2xl border border-amber-200 bg-[rgba(255,248,239,0.92)] px-4 py-3 text-sm text-amber-800">
                    JSON mode edits the entire section exactly as stored. Use this for arrays of objects and advanced nested structures.
                  </div>
                  <textarea
                    className="min-h-[62vh] w-full rounded-[1.5rem] border border-[hsl(var(--gray-200))] bg-[hsl(var(--gray-950))] px-5 py-4 font-mono text-sm leading-6 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]"
                    value={currentJSON}
                    onChange={(event) => handleJSONChange(event.target.value)}
                    spellCheck={false}
                  />
                </div>
              ) : filteredFields.length === 0 ? (
                <div className="py-16 text-center">
                  <div className="text-sm font-medium text-[hsl(var(--gray-900))]">No matching fields.</div>
                  <div className="mt-1 text-sm text-muted-foreground">
                    {search.trim() ? 'Try a broader keyword.' : 'This section has no editable fields.'}
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
