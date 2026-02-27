import { useState, useCallback, useRef } from 'react';
import { t } from '@/lib/i18n';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ScrollArea } from '@/components/ui/scroll-area';
import Header from '@/components/layout/Header';
import { useConfig, useSaveConfig, useExportConfig, useImportConfig } from '@/hooks/useConfig';
import { Download, Upload, Save, RotateCcw, Code, FormInput } from 'lucide-react';

const CONFIG_SECTIONS = ['agents', 'gateway', 'tools', 'memory', 'heartbeat', 'approval', 'logger', 'webui'] as const;
type ConfigSection = (typeof CONFIG_SECTIONS)[number];

function sectionLabel(section: ConfigSection): string {
  const map: Record<ConfigSection, string> = {
    agents: t('configSectionAgents'),
    gateway: t('configSectionGateway'),
    tools: t('configSectionTools'),
    memory: t('configSectionMemory'),
    heartbeat: t('configSectionHeartbeat'),
    approval: t('configSectionApproval'),
    logger: t('configSectionLogger'),
    webui: t('configSectionWebUI'),
  };
  return map[section] || section;
}

function flattenObject(obj: Record<string, unknown>, prefix = ''): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  if (!obj || typeof obj !== 'object' || Array.isArray(obj)) return result;
  for (const k of Object.keys(obj)) {
    const fullKey = prefix ? `${prefix}.${k}` : k;
    const val = obj[k];
    if (val !== null && typeof val === 'object' && !Array.isArray(val)) {
      Object.assign(result, flattenObject(val as Record<string, unknown>, fullKey));
    } else {
      result[fullKey] = val;
    }
  }
  return result;
}

function setNestedValue(obj: Record<string, unknown>, dotPath: string, value: unknown) {
  const parts = dotPath.split('.');
  let cur: Record<string, unknown> = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    if (!cur[parts[i]] || typeof cur[parts[i]] !== 'object') {
      cur[parts[i]] = {};
    }
    cur = cur[parts[i]] as Record<string, unknown>;
  }
  cur[parts[parts.length - 1]] = value;
}

interface FieldDef {
  key: string;
  type: 'text' | 'bool' | 'number' | 'tags';
  value: unknown;
}

function inferFields(data: Record<string, unknown>): FieldDef[] {
  const flat = flattenObject(data);
  return Object.entries(flat).map(([key, value]) => {
    let type: FieldDef['type'] = 'text';
    if (typeof value === 'boolean') type = 'bool';
    else if (typeof value === 'number') type = 'number';
    else if (Array.isArray(value)) type = 'tags';
    return { key, type, value };
  });
}

function FormField({
  field,
  onChange,
}: {
  field: FieldDef;
  onChange: (key: string, value: unknown) => void;
}) {
  switch (field.type) {
    case 'bool':
      return (
        <div className="flex items-center justify-between py-2.5 px-1 border-b border-border/50">
          <Label className="text-sm font-mono text-muted-foreground">{field.key}</Label>
          <Switch checked={!!field.value} onCheckedChange={(v) => onChange(field.key, v)} />
        </div>
      );
    case 'number':
      return (
        <div className="flex items-center justify-between gap-4 py-2.5 px-1 border-b border-border/50">
          <Label className="text-sm font-mono text-muted-foreground shrink-0">{field.key}</Label>
          <Input
            type="number"
            className="w-32 text-right"
            value={field.value != null ? String(field.value) : ''}
            onChange={(e) => onChange(field.key, e.target.value === '' ? 0 : Number(e.target.value))}
          />
        </div>
      );
    case 'tags':
      return (
        <div className="py-2.5 px-1 border-b border-border/50">
          <Label className="text-sm font-mono text-muted-foreground block mb-1.5">{field.key}</Label>
          <textarea
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono resize-y min-h-[60px]"
            rows={2}
            value={Array.isArray(field.value) ? (field.value as string[]).join('\n') : String(field.value ?? '')}
            onChange={(e) =>
              onChange(field.key, e.target.value.split('\n').map((s) => s.trim()).filter(Boolean))
            }
          />
        </div>
      );
    default:
      return (
        <div className="flex items-center justify-between gap-4 py-2.5 px-1 border-b border-border/50">
          <Label className="text-sm font-mono text-muted-foreground shrink-0">{field.key}</Label>
          <Input
            className="max-w-xs"
            value={field.value != null ? String(field.value) : ''}
            onChange={(e) => onChange(field.key, e.target.value)}
          />
        </div>
      );
  }
}

export default function ConfigPage() {
  const [section, setSection] = useState<ConfigSection>('agents');
  const [mode, setMode] = useState<'form' | 'json'>('form');
  const [jsonValue, setJsonValue] = useState('{}');
  const [localData, setLocalData] = useState<Record<string, unknown> | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const { data: config, isLoading } = useConfig();
  const saveConfig = useSaveConfig();
  const exportConfig = useExportConfig();
  const importConfig = useImportConfig();

  const sectionData = localData ?? ((config?.[section] as Record<string, unknown>) ?? {});
  const fields = inferFields(sectionData);

  const handleSectionChange = useCallback((val: string) => {
    setSection(val as ConfigSection);
    setLocalData(null);
    setMode('form');
  }, []);

  const handleFieldChange = useCallback(
    (key: string, value: unknown) => {
      setLocalData((prev) => {
        const base = prev ?? { ...((config?.[section] as Record<string, unknown>) ?? {}) };
        const copy = JSON.parse(JSON.stringify(base));
        setNestedValue(copy, key, value);
        return copy;
      });
    },
    [config, section],
  );

  const handleToggleMode = useCallback(() => {
    if (mode === 'form') {
      setJsonValue(JSON.stringify(localData ?? sectionData, null, 2));
      setMode('json');
    } else {
      try {
        const parsed = JSON.parse(jsonValue);
        setLocalData(parsed);
      } catch (err) {
        toast.error(t('invalidJson', String(err)));
        return;
      }
      setMode('form');
    }
  }, [mode, localData, sectionData, jsonValue]);

  const handleSave = useCallback(() => {
    let payload: Record<string, unknown>;
    if (mode === 'json') {
      try {
        payload = JSON.parse(jsonValue);
      } catch (err) {
        toast.error(t('invalidJson', String(err)));
        return;
      }
    } else {
      payload = localData ?? sectionData;
    }
    saveConfig.mutate({ [section]: payload });
    setLocalData(null);
  }, [mode, jsonValue, localData, sectionData, section, saveConfig]);

  const handleReset = useCallback(() => {
    setLocalData(null);
    if (mode === 'json') {
      setJsonValue(JSON.stringify((config?.[section] as Record<string, unknown>) ?? {}, null, 2));
    }
  }, [config, section, mode]);

  const handleImportFile = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;
      const reader = new FileReader();
      reader.onload = () => {
        try {
          const data = JSON.parse(reader.result as string);
          importConfig.mutate(data);
        } catch {
          toast.error(t('importFailed'));
        }
      };
      reader.readAsText(file);
      e.target.value = '';
    },
    [importConfig],
  );

  return (
    <div className="flex flex-col h-full">
      <Header title={t('tabConfig')} />
      <div className="flex items-center gap-2 px-6 pb-4 flex-wrap">
        <Select value={section} onValueChange={handleSectionChange}>
          <SelectTrigger className="w-[180px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CONFIG_SECTIONS.map((s) => (
              <SelectItem key={s} value={s}>
                {sectionLabel(s)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Button variant="outline" size="sm" onClick={handleToggleMode}>
          {mode === 'form' ? (
            <><Code className="h-4 w-4 mr-1" />{t('configJsonMode')}</>
          ) : (
            <><FormInput className="h-4 w-4 mr-1" />{t('configFormMode')}</>
          )}
        </Button>

        <div className="flex-1" />

        <Button variant="outline" size="sm" onClick={handleReset}>
          <RotateCcw className="h-4 w-4 mr-1" />{t('reset')}
        </Button>
        <Button size="sm" onClick={handleSave} disabled={saveConfig.isPending}>
          <Save className="h-4 w-4 mr-1" />{t('save')}
        </Button>
        <Button variant="outline" size="sm" onClick={() => exportConfig.mutate()}>
          <Download className="h-4 w-4 mr-1" />{t('exportConfig')}
        </Button>
        <Button variant="outline" size="sm" onClick={() => fileInputRef.current?.click()}>
          <Upload className="h-4 w-4 mr-1" />{t('importConfig')}
        </Button>
        <input ref={fileInputRef} type="file" accept=".json" className="hidden" onChange={handleImportFile} />
      </div>

      <p className="px-6 pb-3 text-sm text-muted-foreground">
        {t('configSectionHint', sectionLabel(section))}
      </p>

      <ScrollArea className="flex-1 px-6 pb-6">
        {isLoading ? (
          <div className="text-muted-foreground py-8 text-center animate-pulse">Loading\u2026</div>
        ) : mode === 'form' ? (
          <div className="max-w-2xl">
            {fields.length === 0 ? (
              <div className="text-muted-foreground py-8 text-center text-sm">
                No configuration fields in this section.
              </div>
            ) : (
              fields.map((f) => <FormField key={f.key} field={f} onChange={handleFieldChange} />)
            )}
          </div>
        ) : (
          <textarea
            className="w-full h-[60vh] rounded-md border border-input bg-background px-4 py-3 text-sm font-mono resize-y"
            value={jsonValue}
            onChange={(e) => setJsonValue(e.target.value)}
            spellCheck={false}
          />
        )}
      </ScrollArea>
    </div>
  );
}
