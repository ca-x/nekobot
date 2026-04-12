import { useEffect, useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import {
  useCreatePermissionRule,
  useDeletePermissionRule,
  usePermissionRules,
  useUpdatePermissionRule,
  type PermissionRule,
  type PermissionRuleAction,
  type PermissionRuleInput,
} from '@/hooks/usePermissionRules';
import { Plus, Search, ShieldCheck, Slash, Sparkles, Trash2 } from 'lucide-react';

const EMPTY_RULE: PermissionRuleInput = {
  enabled: true,
  priority: 100,
  tool_name: '',
  session_id: '',
  runtime_id: '',
  action: 'ask',
  description: '',
};

function toFormValue(rule: PermissionRule | null): PermissionRuleInput {
  if (!rule) {
    return { ...EMPTY_RULE };
  }
  return {
    enabled: rule.enabled,
    priority: rule.priority,
    tool_name: rule.tool_name,
    session_id: rule.session_id ?? '',
    runtime_id: rule.runtime_id ?? '',
    action: rule.action,
    description: rule.description ?? '',
  };
}

function formatScope(rule: PermissionRule) {
  if (rule.session_id && rule.runtime_id) {
    return `${t('permissionRulesScopeSession')} + ${t('permissionRulesScopeRuntime')}`;
  }
  if (rule.session_id) {
    return t('permissionRulesScopeSession');
  }
  if (rule.runtime_id) {
    return t('permissionRulesScopeRuntime');
  }
  return t('permissionRulesScopeGlobal');
}

function actionTone(action: PermissionRuleAction) {
  switch (action) {
    case 'allow':
      return 'bg-emerald-50 text-emerald-700 border-emerald-200/70';
    case 'deny':
      return 'bg-rose-50 text-rose-700 border-rose-200/70';
    default:
      return 'bg-amber-50 text-amber-700 border-amber-200/70';
  }
}

export default function PermissionRulesPage() {
  const { data: rules = [], isLoading } = usePermissionRules();
  const createRule = useCreatePermissionRule();
  const updateRule = useUpdatePermissionRule();
  const deleteRule = useDeletePermissionRule();

  const [query, setQuery] = useState('');
  const [selectedRuleID, setSelectedRuleID] = useState<string>('new');
  const [form, setForm] = useState<PermissionRuleInput>({ ...EMPTY_RULE });

  const selectedRule = useMemo(
    () => rules.find((item) => item.id === selectedRuleID) ?? null,
    [rules, selectedRuleID],
  );

  useEffect(() => {
    if (selectedRuleID === 'new') {
      setForm({ ...EMPTY_RULE });
      return;
    }
    setForm(toFormValue(selectedRule));
  }, [selectedRule, selectedRuleID]);

  useEffect(() => {
    if (selectedRuleID !== 'new' && !selectedRule && rules.length > 0) {
      setSelectedRuleID(rules[0]?.id ?? 'new');
    }
  }, [rules, selectedRule, selectedRuleID]);

  const filteredRules = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) {
      return rules;
    }
    return rules.filter((rule) =>
      [
        rule.tool_name,
        rule.action,
        rule.description ?? '',
        rule.session_id ?? '',
        rule.runtime_id ?? '',
      ]
        .join(' ')
        .toLowerCase()
        .includes(keyword),
    );
  }, [query, rules]);

  const enabledCount = rules.filter((rule) => rule.enabled).length;
  const askCount = rules.filter((rule) => rule.action === 'ask').length;

  const saving = createRule.isPending || updateRule.isPending;
  const saveDisabled = saving || !form.tool_name.trim();

  const handleCreateNew = () => {
    setSelectedRuleID('new');
    setForm({ ...EMPTY_RULE });
  };

  const handleSave = () => {
    const payload: PermissionRuleInput = {
      enabled: form.enabled,
      priority: Number(form.priority) || 0,
      tool_name: form.tool_name.trim(),
      session_id: form.session_id?.trim() || '',
      runtime_id: form.runtime_id?.trim() || '',
      action: form.action,
      description: form.description?.trim() || '',
    };

    if (selectedRule?.id) {
      updateRule.mutate({ id: selectedRule.id, data: payload });
      return;
    }

    createRule.mutate(payload, {
      onSuccess: (result) => {
        if (result.rule.id) {
          setSelectedRuleID(result.rule.id);
        }
      },
    });
  };

  const handleDelete = () => {
    if (!selectedRule?.id) {
      return;
    }
    deleteRule.mutate(selectedRule.id, {
      onSuccess: () => {
        setSelectedRuleID('new');
        setForm({ ...EMPTY_RULE });
      },
    });
  };

  const handleQuickToggle = (rule: PermissionRule, enabled: boolean) => {
    if (!rule.id) {
      return;
    }
    updateRule.mutate({
      id: rule.id,
      data: {
        enabled,
        priority: rule.priority,
        tool_name: rule.tool_name,
        session_id: rule.session_id ?? '',
        runtime_id: rule.runtime_id ?? '',
        action: rule.action,
        description: rule.description ?? '',
      },
    });
  };

  return (
    <div className="space-y-6">
      <Header
        title={t('tabPermissionRules')}
        description={t('permissionRulesPageDescription')}
      />

      <section className="relative overflow-hidden rounded-[28px] border border-border/70 bg-[radial-gradient(circle_at_top_left,_rgba(34,197,94,0.16),_transparent_34%),linear-gradient(135deg,hsl(var(--card)/0.98),hsl(var(--muted)/0.72))] p-5 shadow-sm sm:p-6">
        <div className="absolute right-0 top-0 h-40 w-40 rounded-full bg-emerald-100/60 blur-3xl" />
        <div className="relative flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-3">
            <div className="inline-flex items-center gap-2 rounded-full border border-emerald-300/40 bg-card/90 px-3 py-1 text-xs font-medium text-emerald-700 shadow-sm">
              <ShieldCheck className="h-3.5 w-3.5" />
              {t('permissionRulesHeroBadge')}
            </div>
            <div className="space-y-2">
              <h2 className="max-w-2xl text-2xl font-semibold tracking-tight text-foreground">
                {t('permissionRulesHeroTitle')}
              </h2>
              <p className="max-w-2xl text-sm leading-6 text-muted-foreground">
                {t('permissionRulesHeroDescription')}
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <MetricCard label={t('permissionRulesMetricTotal')} value={String(rules.length)} />
              <MetricCard label={t('permissionRulesMetricEnabled')} value={String(enabledCount)} />
              <MetricCard label={t('permissionRulesMetricAsk')} value={String(askCount)} />
            </div>
          </div>

          <div className="flex w-full flex-col gap-3 sm:max-w-[360px]">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t('permissionRulesSearchPlaceholder')}
                className="h-11 rounded-2xl border-border/70 bg-card/90 pl-9 shadow-sm"
              />
            </div>
            <Button type="button" className="rounded-2xl" onClick={handleCreateNew}>
              <Plus className="mr-2 h-4 w-4" />
              {t('permissionRulesNew')}
            </Button>
          </div>
        </div>
      </section>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
        <Card className="rounded-[28px] border-border/70 bg-card/92 shadow-sm">
          <CardHeader className="pb-3">
            <CardTitle className="text-lg">{t('permissionRulesListTitle')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {isLoading && Array.from({ length: 5 }).map((_, index) => (
              <Skeleton key={index} className="h-24 rounded-[22px]" />
            ))}

            {!isLoading && filteredRules.length === 0 && (
              <div className="rounded-[22px] border border-dashed border-border/70 bg-card/70 px-4 py-8 text-center">
                <p className="text-sm text-muted-foreground">{t('permissionRulesEmpty')}</p>
              </div>
            )}

            {!isLoading && filteredRules.map((rule) => {
              const selected = rule.id === selectedRuleID;
              return (
                <button
                  key={rule.id}
                  type="button"
                  onClick={() => setSelectedRuleID(rule.id ?? 'new')}
                  className={cn(
                    'w-full rounded-[22px] border px-4 py-4 text-left transition-colors',
                    selected
                      ? 'border-emerald-300 bg-emerald-50/70 shadow-sm'
                      : 'border-border/70 bg-background/70 hover:bg-muted/70',
                  )}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0 space-y-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-sm font-semibold text-foreground">{rule.tool_name}</span>
                        <span className={cn('rounded-full border px-2 py-0.5 text-xs font-medium', actionTone(rule.action))}>
                          {rule.action}
                        </span>
                        {!rule.enabled && (
                          <span className="rounded-full border border-slate-200/70 bg-slate-100 px-2 py-0.5 text-xs text-slate-700">
                            {t('permissionRulesDisabled')}
                          </span>
                        )}
                      </div>
                      <p className="text-xs text-muted-foreground">
                        {t('permissionRulesPriorityLabel', rule.priority)} · {formatScope(rule)}
                      </p>
                      {rule.description && (
                        <p className="line-clamp-2 text-sm leading-5 text-muted-foreground">{rule.description}</p>
                      )}
                    </div>

                    <Switch
                      checked={rule.enabled}
                      onCheckedChange={(checked) => handleQuickToggle(rule, checked)}
                      aria-label={t('permissionRulesEnabled')}
                    />
                  </div>
                </button>
              );
            })}
          </CardContent>
        </Card>

        <Card className="rounded-[28px] border-border/70 bg-card/92 shadow-sm">
          <CardHeader className="border-b border-border/70 bg-[linear-gradient(135deg,rgba(240,253,244,0.95),rgba(248,250,252,0.92))]">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="space-y-2">
                <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-card/90 px-3 py-1 text-xs font-medium text-muted-foreground">
                  <Sparkles className="h-3.5 w-3.5" />
                  {selectedRule ? t('permissionRulesEditTitle') : t('permissionRulesCreateTitle')}
                </div>
                <CardTitle className="text-xl">
                  {selectedRule ? selectedRule.tool_name : t('permissionRulesCreateTitle')}
                </CardTitle>
                <p className="text-sm leading-6 text-muted-foreground">
                  {t('permissionRulesEditorDescription')}
                </p>
              </div>
              {selectedRule?.id && (
                <Button
                  type="button"
                  variant="outline"
                  className="rounded-2xl border-rose-200 text-rose-700 hover:bg-rose-50 hover:text-rose-800"
                  onClick={handleDelete}
                  disabled={deleteRule.isPending}
                >
                  <Trash2 className="mr-2 h-4 w-4" />
                  {t('permissionRulesDelete')}
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent className="space-y-6 p-6">
            <div className="grid gap-5 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="permission-rule-tool">{t('permissionRulesToolName')}</Label>
                <Input
                  id="permission-rule-tool"
                  value={form.tool_name}
                  onChange={(event) => setForm((current) => ({ ...current, tool_name: event.target.value }))}
                  placeholder="exec"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="permission-rule-action">{t('permissionRulesAction')}</Label>
                <Select
                  value={form.action}
                  onValueChange={(value) => setForm((current) => ({ ...current, action: value as PermissionRuleAction }))}
                >
                  <SelectTrigger id="permission-rule-action">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="allow">{t('permissionRulesActionAllow')}</SelectItem>
                    <SelectItem value="deny">{t('permissionRulesActionDeny')}</SelectItem>
                    <SelectItem value="ask">{t('permissionRulesActionAsk')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="permission-rule-session">{t('permissionRulesSessionID')}</Label>
                <Input
                  id="permission-rule-session"
                  value={form.session_id}
                  onChange={(event) => setForm((current) => ({ ...current, session_id: event.target.value }))}
                  placeholder="session-123"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="permission-rule-runtime">{t('permissionRulesRuntimeID')}</Label>
                <Input
                  id="permission-rule-runtime"
                  value={form.runtime_id}
                  onChange={(event) => setForm((current) => ({ ...current, runtime_id: event.target.value }))}
                  placeholder="runtime-a"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="permission-rule-priority">{t('permissionRulesPriority')}</Label>
                <Input
                  id="permission-rule-priority"
                  type="number"
                  value={String(form.priority)}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      priority: Number(event.target.value),
                    }))
                  }
                />
              </div>
              <div className="flex items-center justify-between rounded-[22px] border border-border/70 bg-muted/40 px-4 py-3">
                <div className="space-y-1">
                  <div className="text-sm font-medium text-foreground">{t('permissionRulesEnabled')}</div>
                  <div className="text-xs text-muted-foreground">{t('permissionRulesEnabledHint')}</div>
                </div>
                <Switch
                  checked={form.enabled}
                  onCheckedChange={(checked) => setForm((current) => ({ ...current, enabled: checked }))}
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="permission-rule-description">{t('permissionRulesDescription')}</Label>
              <Input
                id="permission-rule-description"
                value={form.description}
                onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))}
                placeholder={t('permissionRulesDescriptionPlaceholder')}
              />
            </div>

            <div className="rounded-[24px] border border-border/70 bg-muted/30 p-4">
              <div className="flex items-start gap-3">
                {form.action === 'deny' ? (
                  <Slash className="mt-0.5 h-4 w-4 text-rose-600" />
                ) : (
                  <ShieldCheck className="mt-0.5 h-4 w-4 text-emerald-600" />
                )}
                <div className="space-y-1">
                  <div className="text-sm font-medium text-foreground">{t('permissionRulesPreviewTitle')}</div>
                  <p className="text-sm leading-6 text-muted-foreground">
                    {t(
                      form.action === 'allow'
                        ? 'permissionRulesPreviewAllow'
                        : form.action === 'deny'
                          ? 'permissionRulesPreviewDeny'
                          : 'permissionRulesPreviewAsk',
                    )}
                  </p>
                </div>
              </div>
            </div>

            <div className="flex flex-col gap-3 border-t border-border/70 pt-5 sm:flex-row sm:justify-between">
              <Button type="button" variant="outline" className="rounded-2xl" onClick={handleCreateNew}>
                {t('permissionRulesReset')}
              </Button>
              <Button type="button" className="rounded-2xl" onClick={handleSave} disabled={saveDisabled}>
                {selectedRule ? t('permissionRulesSaveChanges') : t('permissionRulesCreate')}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-[104px] rounded-[22px] border border-border/70 bg-card/90 px-4 py-3 shadow-sm">
      <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className="mt-1 text-2xl font-semibold tracking-tight text-foreground">{value}</div>
    </div>
  );
}
