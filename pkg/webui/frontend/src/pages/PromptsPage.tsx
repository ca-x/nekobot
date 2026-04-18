import { useMemo, useState } from 'react';
import { Bot, Plus, Sparkles, Trash2, Wand2, Loader2 } from 'lucide-react';
import { toast } from '@/lib/notify';
import { useNavigate } from 'react-router-dom';

import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogPortal,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import {
  useCreatePrompt,
  useCreatePromptBinding,
  useDeletePrompt,
  useDeletePromptBinding,
  usePromptBindings,
  usePreviewContextSources,
  usePrompts,
  type ContextSourceRecord,
  useUpdatePrompt,
  type PromptBindingInput,
  type PromptBindingRecord,
  type PromptInput,
  type PromptRecord,
} from '@/hooks/usePrompts';

type PromptMode = 'system' | 'user';
type PromptScope = 'global' | 'channel' | 'session';

const EMPTY_SCOPE_TARGET = '__global__';

function parseTags(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function formatScope(scope: PromptScope): string {
  return t(`promptScope_${scope}`);
}

function emptyPromptDraft(): PromptInput {
  return {
    key: '',
    name: '',
    description: '',
    mode: 'system',
    template: '',
    enabled: true,
    tags: [],
  };
}

function emptyBindingDraft(): PromptBindingInput {
  return {
    scope: 'global',
    target: '',
    prompt_id: '',
    enabled: true,
    priority: 100,
  };
}

export default function PromptsPage() {
  const navigate = useNavigate();
  const { data: prompts = [] } = usePrompts();
  const { data: bindings = [] } = usePromptBindings();
  const createPrompt = useCreatePrompt();
  const updatePrompt = useUpdatePrompt();
  const deletePrompt = useDeletePrompt();
  const createBinding = useCreatePromptBinding();
  const deleteBinding = useDeletePromptBinding();
  const previewContextSources = usePreviewContextSources();

  const [selectedPromptID, setSelectedPromptID] = useState<string>('');
  const [draft, setDraft] = useState<PromptInput>(emptyPromptDraft());
  const [tagsInput, setTagsInput] = useState('');
  const [bindingDraft, setBindingDraft] = useState<PromptBindingInput>(emptyBindingDraft());
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ type: 'prompt' | 'binding'; id: string } | null>(null);
  const [previewSessionID, setPreviewSessionID] = useState('webui-preview');
  const [previewChannel, setPreviewChannel] = useState('webui');
  const [previewProvider, setPreviewProvider] = useState('');
  const [previewModel, setPreviewModel] = useState('');
  const [previewMessage, setPreviewMessage] = useState('hello');

  const selectedPrompt = useMemo(
    () => prompts.find((item) => item.id === selectedPromptID) ?? null,
    [prompts, selectedPromptID],
  );

  const bindingsWithPrompt = useMemo(() => {
    const promptMap = new Map(prompts.map((item) => [item.id, item]));
    return bindings.map((binding) => ({
      binding,
      prompt: promptMap.get(binding.prompt_id) ?? null,
    }));
  }, [bindings, prompts]);

  function resetPromptForm(next?: PromptRecord | null) {
    if (!next) {
      setDraft(emptyPromptDraft());
      setTagsInput('');
      setSelectedPromptID('');
      return;
    }
    setDraft({
      key: next.key,
      name: next.name,
      description: next.description,
      mode: next.mode,
      template: next.template,
      enabled: next.enabled,
      tags: next.tags,
    });
    setTagsInput(next.tags.join(', '));
    setSelectedPromptID(next.id);
  }

  function handleSelectPrompt(id: string) {
    const next = prompts.find((item) => item.id === id) ?? null;
    resetPromptForm(next);
  }

  async function handleSavePrompt() {
    const payload: PromptInput = {
      ...draft,
      tags: parseTags(tagsInput),
    };
    try {
      if (selectedPromptID) {
        await updatePrompt.mutateAsync({ id: selectedPromptID, input: payload });
        return;
      }
      const created = await createPrompt.mutateAsync(payload);
      resetPromptForm(created);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('saveFailed'));
    }
  }

  function handleDeletePromptRecord(id: string) {
    setDeleteTarget({ type: 'prompt', id });
    setShowDeleteConfirm(true);
  }

  async function handleCreateBindingRecord() {
    await createBinding.mutateAsync({
      ...bindingDraft,
      target: bindingDraft.scope === 'global' ? '' : bindingDraft.target.trim(),
    });
    setBindingDraft(emptyBindingDraft());
  }

  function handleDeleteBindingRecord(id: string) {
    setDeleteTarget({ type: 'binding', id });
    setShowDeleteConfirm(true);
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    if (deleteTarget.type === 'prompt') {
      await deletePrompt.mutateAsync(deleteTarget.id);
      if (selectedPromptID === deleteTarget.id) {
        resetPromptForm(null);
      }
    } else {
      await deleteBinding.mutateAsync(deleteTarget.id);
    }
    setShowDeleteConfirm(false);
    setDeleteTarget(null);
  }

  const deleteDialogTitle =
    deleteTarget?.type === 'binding' ? t('promptBindingDeleteTitle') : t('promptDeleteTitle');
  const deleteDialogDescription =
    deleteTarget?.type === 'binding'
      ? t('promptBindingDeleteDescription')
      : t('promptDeleteDescription');

  async function handlePreviewContextSources() {
    await previewContextSources.mutateAsync({
      channel: previewChannel.trim(),
      session_id: previewSessionID.trim(),
      requested_provider: previewProvider.trim(),
      requested_model: previewModel.trim(),
      user_message: previewMessage,
    });
  }

  const previewData = previewContextSources.data;
  const previewBudgetStatus = previewData?.preflight.budget_status ?? previewData?.budget_status ?? 'ok';
  const previewBudgetReasons = previewData?.preflight.budget_reasons ?? previewData?.budget_reasons ?? [];
  const previewCompaction = previewData?.preflight.compaction ?? previewData?.compaction;
  const savePromptDisabled =
    createPrompt.isPending ||
    updatePrompt.isPending ||
    (!selectedPromptID &&
      (!draft.key.trim() || !draft.name.trim() || !draft.template.trim()));
  const saveBindingDisabled =
    createBinding.isPending ||
    !bindingDraft.prompt_id.trim() ||
    (bindingDraft.scope !== 'global' && !bindingDraft.target.trim());

  return (
    <>
      <div className="prompts-page flex h-[calc(100dvh-4rem)] flex-col overflow-hidden">
        <Header title={t('tabPrompts')} />

        <div className="grid min-h-0 flex-1 gap-4 2xl:grid-cols-[minmax(0,1.1fr)_380px]">
          <div className="grid min-h-0 gap-4 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
            <Card className="overflow-hidden border-border/70 bg-card/88 shadow-[0_20px_60px_-42px_rgba(120,55,75,0.28)]">
              <div className="border-b border-border/70 bg-[linear-gradient(135deg,hsl(var(--card)/0.98),hsl(var(--muted)/0.72))] p-5">
                <div className="inline-flex items-center gap-2 rounded-full bg-primary/10 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.2em] text-primary">
                  <Sparkles className="h-3.5 w-3.5" />
                  {t('promptLibraryBadge')}
                </div>
                <h2 className="mt-3 text-xl font-semibold text-foreground">{t('promptLibraryTitle')}</h2>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">{t('promptLibraryDescription')}</p>
              </div>

              <div className="space-y-3 p-4">
                <div className="rounded-[1.4rem] border border-[hsl(var(--brand-200))] bg-[linear-gradient(135deg,rgba(255,250,247,0.98),rgba(252,240,244,0.86))] p-4">
                  <div className="text-xs font-medium uppercase tracking-[0.18em] text-[hsl(var(--brand-700))]">
                    {t('chatSetupGuide')}
                  </div>
                  <p className="mt-2 text-sm leading-6 text-muted-foreground">
                    {t('promptWorkflowHint')}
                  </p>
                  <Button
                    variant="outline"
                    size="sm"
                    className="mt-3 rounded-full"
                    onClick={() => navigate('/chat')}
                  >
                    {t('promptGoToChat')}
                  </Button>
                </div>

                <Button className="h-11 w-full rounded-2xl" onClick={() => resetPromptForm(null)}>
                  <Plus className="mr-2 h-4 w-4" />
                  {t('promptNew')}
                </Button>

                <div className="space-y-3">
                  {prompts.map((item) => (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => handleSelectPrompt(item.id)}
                      className={cn(
                        'w-full rounded-[1.4rem] border p-4 text-left transition-colors',
                        selectedPromptID === item.id
                          ? 'border-primary/30 bg-primary/10'
                          : 'border-border/70 bg-card hover:border-border hover:bg-muted/35',
                      )}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <div className="break-words text-sm font-semibold text-foreground">{item.name}</div>
                          <div className="mt-1 break-all text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
                            {item.key}
                          </div>
                        </div>
                        <span
                          className={cn(
                            'shrink-0 rounded-full px-2.5 py-1 text-[11px] font-medium',
                            item.mode === 'system'
                              ? 'bg-[hsl(var(--brand-100))] text-[hsl(var(--brand-800))]'
                              : 'bg-[hsl(var(--gray-100))] text-[hsl(var(--gray-700))]',
                          )}
                        >
                          {t(item.mode === 'system' ? 'promptModeSystem' : 'promptModeUser')}
                        </span>
                      </div>
                      <p className="mt-3 line-clamp-2 text-sm leading-6 text-muted-foreground">
                        {item.description || item.template}
                      </p>
                    </button>
                  ))}

                  {prompts.length === 0 && (
                    <div className="rounded-[1.4rem] border border-dashed border-border px-4 py-8 text-center text-sm text-muted-foreground">
                      <p>{t('promptEmpty')}</p>
                      <p className="mt-2 text-xs">{t('promptEmptyGuide')}</p>
                      <div className="mt-4 flex flex-wrap justify-center gap-2">
                        <Button size="sm" className="rounded-full" onClick={() => resetPromptForm(null)}>
                          <Plus className="mr-2 h-3.5 w-3.5" />
                          {t('promptNew')}
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="rounded-full"
                          onClick={() => navigate('/chat')}
                        >
                          {t('promptGoToChat')}
                        </Button>
                      </div>
                    </div>
                  )}
                </div>

                <div className="rounded-[1.4rem] border border-border/70 bg-card/80 p-4">
                  <div className="inline-flex items-center gap-2 rounded-full bg-primary/10 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.2em] text-primary">
                    <Wand2 className="h-3.5 w-3.5" />
                    {t('promptContextSourcesBadge')}
                  </div>
                  <h3 className="mt-3 text-base font-semibold text-foreground">{t('promptContextSourcesTitle')}</h3>
                  <p className="mt-2 text-sm leading-6 text-muted-foreground">{t('promptContextSourcesDescription')}</p>

                  <div className="mt-4 grid gap-3 sm:grid-cols-2">
                    <div className="space-y-2">
                      <Label>{t('promptPreviewChannel')}</Label>
                      <Input value={previewChannel} onChange={(event) => setPreviewChannel(event.target.value)} />
                    </div>
                    <div className="space-y-2">
                      <Label>{t('promptPreviewSession')}</Label>
                      <Input value={previewSessionID} onChange={(event) => setPreviewSessionID(event.target.value)} />
                    </div>
                    <div className="space-y-2">
                      <Label>{t('defaultProvider')}</Label>
                      <Input value={previewProvider} onChange={(event) => setPreviewProvider(event.target.value)} placeholder="openai" />
                    </div>
                    <div className="space-y-2">
                      <Label>{t('defaultModel')}</Label>
                      <Input value={previewModel} onChange={(event) => setPreviewModel(event.target.value)} placeholder="gpt-5.4" />
                    </div>
                  </div>

                  <div className="mt-3 space-y-2">
                    <Label>{t('promptPreviewMessage')}</Label>
                    <Textarea value={previewMessage} onChange={(event) => setPreviewMessage(event.target.value)} rows={3} />
                  </div>

                  <Button
                    type="button"
                    variant="outline"
                    className="mt-4 rounded-2xl"
                    onClick={handlePreviewContextSources}
                    disabled={previewContextSources.isPending}
                  >
                    {previewContextSources.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Sparkles className="mr-2 h-4 w-4" />}
                    {t('promptPreviewContextSources')}
                  </Button>

                  {previewData ? (
                    <div className="mt-4 space-y-3">
                      {previewData.preflight.action ? (
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="rounded-full border border-border/70 bg-card px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
                            {previewData.preflight.action}
                          </span>
                        </div>
                      ) : null}
                      <div className="flex flex-wrap items-center gap-2">
                        <span
                          className={cn(
                            'rounded-full px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em]',
                            contextBudgetTone(previewBudgetStatus),
                          )}
                        >
                          {t(`promptContextBudget_${previewBudgetStatus}`)}
                        </span>
                      </div>
                      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                        <ContextMetricCard
                          label={t('promptContextMetricTotal')}
                          value={String(previewData.footprint.total_chars)}
                        />
                        <ContextMetricCard
                          label={t('promptContextMetricSystem')}
                          value={String(previewData.footprint.system_chars)}
                        />
                        <ContextMetricCard
                          label={t('promptContextMetricUser')}
                          value={String(previewData.footprint.final_user_chars)}
                        />
                        <ContextMetricCard
                          label={t('promptContextMetricMentions')}
                          value={String(previewData.footprint.mention_count)}
                        />
                      </div>
                      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                        <ContextMetricCard
                          label={t('promptContextMetricMemory')}
                          value={String(previewData.footprint.memory_chars)}
                          hint={
                            previewData.footprint.memory_limit_chars > 0
                              ? t(
                                  'promptContextMetricMemoryLimit',
                                  String(previewData.footprint.memory_limit_chars),
                                )
                              : undefined
                          }
                        />
                        <ContextMetricCard
                          label={t('promptContextMetricManaged')}
                          value={String(previewData.footprint.managed_prompt_chars)}
                        />
                        <ContextMetricCard
                          label={t('promptContextMetricReferenced')}
                          value={String(previewData.footprint.file_reference_chars)}
                        />
                      </div>
                      {previewData.warnings && previewData.warnings.length > 0 ? (
                        <div className="rounded-[1.2rem] border border-amber-300/40 bg-amber-500/10 p-3 text-sm text-amber-800 dark:text-amber-200">
                          <div className="text-xs font-medium uppercase tracking-[0.18em]">
                            {t('promptContextWarnings')}
                          </div>
                          <div className="mt-2 space-y-1">
                            {previewData.warnings.map((warning, index) => (
                              <div key={`${warning}-${index}`}>{warning}</div>
                            ))}
                          </div>
                        </div>
                      ) : null}
                      {previewBudgetReasons.length > 0 ? (
                        <div className="rounded-[1.2rem] border border-border/70 bg-muted/35 p-3 text-sm text-muted-foreground">
                          <div className="text-xs font-medium uppercase tracking-[0.18em]">
                            {t('promptContextBudgetReasons')}
                          </div>
                          <div className="mt-2 space-y-1">
                            {previewBudgetReasons.map((reason, index) => (
                              <div key={`${reason}-${index}`}>{reason}</div>
                            ))}
                          </div>
                        </div>
                      ) : null}
                      {previewCompaction?.recommended ? (
                        <div className="rounded-[1.2rem] border border-sky-300/40 bg-sky-500/10 p-3 text-sm text-sky-900 dark:text-sky-100">
                          <div className="text-xs font-medium uppercase tracking-[0.18em]">
                            {t('promptContextCompactionTitle')}
                          </div>
                          <div className="mt-2">
                            {t(
                              'promptContextCompactionStrategy',
                              t(`promptContextCompaction_${previewCompaction.strategy ?? 'drop_oldest_history'}`),
                            )}
                          </div>
                          {previewCompaction.estimated_chars_saved ? (
                            <div className="mt-1 text-xs text-sky-800/80 dark:text-sky-100/80">
                              {t(
                                'promptContextCompactionSaved',
                                String(previewCompaction.estimated_chars_saved),
                              )}
                            </div>
                          ) : null}
                          {previewCompaction.reasons && previewCompaction.reasons.length > 0 ? (
                            <div className="mt-2 space-y-1">
                              {previewCompaction.reasons.map((reason, index) => (
                                <div key={`${reason}-${index}`}>{reason}</div>
                              ))}
                            </div>
                          ) : null}
                        </div>
                      ) : null}
                      <div className="rounded-[1.2rem] border border-border/70 bg-muted/35 p-3 text-sm text-muted-foreground">
                        <div className="text-xs font-medium uppercase tracking-[0.18em]">{t('promptPreviewProcessedInput')}</div>
                        <div className="mt-2 break-words text-foreground">{previewData.preprocessed_input || t('none')}</div>
                      </div>
                      {previewData.sources.map((source, index) => (
                        <ContextSourceCard key={`${source.kind}-${index}`} source={source} />
                      ))}
                    </div>
                  ) : (
                    <div className="mt-4 rounded-[1.2rem] border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                      {t('promptContextSourcesEmpty')}
                    </div>
                  )}
                </div>
              </div>
            </Card>

            <Card className="overflow-hidden border-border/70 bg-card/90 shadow-[0_22px_60px_-42px_rgba(82,42,59,0.3)]">
              <div className="border-b border-border/70 bg-[linear-gradient(160deg,hsl(var(--card)/0.98),hsl(var(--muted)/0.65))] p-5">
                <div className="inline-flex items-center gap-2 rounded-full bg-muted px-3 py-1 text-[11px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
                  <Wand2 className="h-3.5 w-3.5" />
                  {selectedPrompt ? t('promptEditorEdit') : t('promptEditorCreate')}
                </div>
                <h2 className="mt-3 text-xl font-semibold text-foreground">{t('promptEditorTitle')}</h2>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">{t('promptEditorDescription')}</p>
              </div>

              <div className="space-y-5 p-5">
                <div className="grid gap-4 xl:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="prompt-key">{t('promptKey')}</Label>
                    <Input
                      id="prompt-key"
                      className="h-11 font-mono text-xs sm:text-sm"
                      value={draft.key}
                      onChange={(event) => setDraft((current) => ({ ...current, key: event.target.value }))}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="prompt-name">{t('promptName')}</Label>
                    <Input
                      id="prompt-name"
                      className="h-11"
                      value={draft.name}
                      onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="prompt-description">{t('promptDescription')}</Label>
                  <Input
                    id="prompt-description"
                    className="h-11"
                    value={draft.description}
                    onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))}
                  />
                </div>

                <div className="grid gap-4 xl:grid-cols-2">
                  <div className="space-y-2">
                    <Label>{t('promptMode')}</Label>
                    <Select
                      value={draft.mode}
                      onValueChange={(value: PromptMode) => setDraft((current) => ({ ...current, mode: value }))}
                    >
                      <SelectTrigger className="h-11 w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="system">{t('promptModeSystem')}</SelectItem>
                        <SelectItem value="user">{t('promptModeUser')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="flex flex-col gap-3 rounded-[1.2rem] border border-border/70 bg-muted/20 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-foreground">{t('enabled')}</div>
                      <div className="text-xs text-muted-foreground">{t('promptEnabledHint')}</div>
                    </div>
                    <Switch
                      checked={draft.enabled}
                      onCheckedChange={(checked) => setDraft((current) => ({ ...current, enabled: checked }))}
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="prompt-tags">{t('promptTags')}</Label>
                  <Input
                    id="prompt-tags"
                    className="h-11"
                    value={tagsInput}
                    onChange={(event) => setTagsInput(event.target.value)}
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="prompt-template">{t('promptTemplate')}</Label>
                  <Textarea
                    id="prompt-template"
                    rows={14}
                    className="min-h-[320px] resize-y font-mono text-xs leading-6 sm:text-sm"
                    value={draft.template}
                    onChange={(event) => setDraft((current) => ({ ...current, template: event.target.value }))}
                  />
                </div>

                <div className="flex flex-col gap-2 pt-1 sm:flex-row sm:flex-wrap">
                  <Button
                    className="h-11 rounded-full px-5"
                    onClick={() => void handleSavePrompt()}
                    disabled={savePromptDisabled}
                  >
                    {selectedPrompt ? t('save') : t('promptCreateAction')}
                  </Button>
                  {selectedPrompt && (
                    <Button
                      variant="outline"
                      className="h-11 rounded-full text-destructive hover:text-destructive sm:min-w-[120px]"
                      onClick={() => handleDeletePromptRecord(selectedPrompt.id)}
                      disabled={deletePrompt.isPending}
                    >
                      <Trash2 className="mr-2 h-4 w-4" />
                      {t('delete')}
                    </Button>
                  )}
                </div>
              </div>
            </Card>
          </div>

          <Card className="overflow-hidden border-border/70 bg-card/88 shadow-[0_24px_60px_-42px_rgba(93,51,68,0.28)]">
            <div className="border-b border-border/70 bg-[linear-gradient(150deg,hsl(var(--card)/0.98),hsl(var(--muted)/0.72))] p-5">
              <div className="inline-flex items-center gap-2 rounded-full bg-primary/10 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.2em] text-primary">
                <Bot className="h-3.5 w-3.5" />
                {t('promptBindingsBadge')}
              </div>
              <h2 className="mt-3 text-xl font-semibold text-foreground">{t('promptBindingsTitle')}</h2>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">{t('promptBindingsDescription')}</p>
            </div>

            <div className="space-y-5 p-5">
              <div className="space-y-4 rounded-[1.6rem] border border-border/70 bg-muted/30 p-4">
                <div className="grid gap-4 xl:grid-cols-2">
                  <div className="space-y-2">
                    <Label>{t('promptBindingScope')}</Label>
                    <Select
                      value={bindingDraft.scope}
                      onValueChange={(value: PromptScope) =>
                        setBindingDraft((current) => ({
                          ...current,
                          scope: value,
                          target: value === 'global' ? '' : current.target,
                        }))
                      }
                    >
                      <SelectTrigger className="h-11 w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="global">{formatScope('global')}</SelectItem>
                        <SelectItem value="channel">{formatScope('channel')}</SelectItem>
                        <SelectItem value="session">{formatScope('session')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="space-y-2">
                    <Label>{t('promptBindingPrompt')}</Label>
                    <Select
                      value={bindingDraft.prompt_id || EMPTY_SCOPE_TARGET}
                      onValueChange={(value) =>
                        setBindingDraft((current) => ({
                          ...current,
                          prompt_id: value === EMPTY_SCOPE_TARGET ? '' : value,
                        }))
                      }
                    >
                      <SelectTrigger className="h-11 w-full">
                        <SelectValue placeholder={t('promptBindingPrompt')} />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={EMPTY_SCOPE_TARGET}>{t('promptBindingSelectPrompt')}</SelectItem>
                        {prompts.map((item) => (
                          <SelectItem key={item.id} value={item.id}>
                            {item.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                {bindingDraft.scope !== 'global' && (
                  <div className="space-y-2">
                    <Label htmlFor="binding-target">{t('promptBindingTarget')}</Label>
                    <Input
                      id="binding-target"
                      className="h-11"
                      value={bindingDraft.target}
                      onChange={(event) => setBindingDraft((current) => ({ ...current, target: event.target.value }))}
                      placeholder={bindingDraft.scope === 'channel' ? 'wechat' : 'webui-chat:...'}
                    />
                  </div>
                )}

                <div className="grid gap-4 xl:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="binding-priority">{t('promptBindingPriority')}</Label>
                    <Input
                      id="binding-priority"
                      type="number"
                      className="h-11"
                      value={String(bindingDraft.priority)}
                      onChange={(event) =>
                        setBindingDraft((current) => ({
                          ...current,
                          priority: Number(event.target.value || 0),
                        }))
                      }
                    />
                  </div>

                  <div className="flex flex-col gap-3 rounded-[1.2rem] border border-[hsl(var(--gray-200))] px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-[hsl(var(--gray-900))]">{t('enabled')}</div>
                      <div className="text-xs text-muted-foreground">{t('promptBindingEnabledHint')}</div>
                    </div>
                    <Switch
                      checked={bindingDraft.enabled}
                      onCheckedChange={(checked) => setBindingDraft((current) => ({ ...current, enabled: checked }))}
                    />
                  </div>
                </div>

                <Button className="h-11 w-full rounded-full" onClick={() => void handleCreateBindingRecord()} disabled={saveBindingDisabled}>
                  <Plus className="mr-2 h-4 w-4" />
                  {t('promptBindingAdd')}
                </Button>
              </div>

              <div className="space-y-3">
                {bindingsWithPrompt.map(({ binding, prompt }) => (
                  <BindingCard
                    key={binding.id}
                    binding={binding}
                    prompt={prompt}
                    onDelete={handleDeleteBindingRecord}
                    deleting={deleteBinding.isPending}
                  />
                ))}

                {bindingsWithPrompt.length === 0 && (
                  <div className="rounded-[1.4rem] border border-dashed border-border px-4 py-8 text-center text-sm text-muted-foreground">
                    <p>{t('promptBindingsEmpty')}</p>
                    <p className="mt-2 text-xs">{t('promptBindingsEmptyGuide')}</p>
                    <Button
                      size="sm"
                      variant="outline"
                      className="mt-4 rounded-full"
                      onClick={() => resetPromptForm(null)}
                    >
                      <Plus className="mr-2 h-3.5 w-3.5" />
                      {t('promptNew')}
                    </Button>
                  </div>
                )}

                {bindingsWithPrompt.length > 0 && (
                  <Button
                    variant="outline"
                    className="h-11 w-full rounded-full"
                    onClick={() => navigate('/chat')}
                  >
                    {t('promptGoToChat')}
                  </Button>
                )}
              </div>
            </div>
          </Card>
        </div>
      </div>

      <Dialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <DialogPortal>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{deleteDialogTitle}</DialogTitle>
              <DialogDescription>{deleteDialogDescription}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowDeleteConfirm(false)}>
                {t('cancel')}
              </Button>
              <Button
                variant="destructive"
                onClick={() => void confirmDelete()}
                disabled={deleteTarget?.type === 'prompt' ? deletePrompt.isPending : deleteBinding.isPending}
              >
                {(deleteTarget?.type === 'prompt' ? deletePrompt.isPending : deleteBinding.isPending) ? (
                  <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                ) : (
                  <Trash2 className="mr-1.5 h-4 w-4" />
                )}
                {t('delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </DialogPortal>
      </Dialog>
    </>
  );
}

function BindingCard({
  binding,
  prompt,
  onDelete,
  deleting,
}: {
  binding: PromptBindingRecord;
  prompt: PromptRecord | null;
  onDelete: (id: string) => void;
  deleting: boolean;
}) {
  return (
    <div className="rounded-[1.4rem] border border-border/70 bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="break-words text-sm font-semibold text-foreground">
            {prompt?.name ?? t('promptBindingMissingPrompt')}
          </div>
          {prompt?.key && (
            <div className="mt-1 break-all font-mono text-[11px] text-muted-foreground">{prompt.key}</div>
          )}
          <div className="mt-2 flex flex-wrap gap-2 text-xs text-muted-foreground">
            <span className="rounded-full bg-muted px-2.5 py-1">{formatScope(binding.scope)}</span>
            {binding.target && (
              <span className="min-w-0 break-all rounded-full bg-muted px-2.5 py-1">{binding.target}</span>
            )}
            <span className="rounded-full bg-muted px-2.5 py-1">
              {t('promptBindingPriority')}: {binding.priority}
            </span>
          </div>
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="h-9 w-9 shrink-0 rounded-full text-muted-foreground hover:text-destructive"
          onClick={() => onDelete(binding.id)}
          disabled={deleting}
          aria-label={t('promptBindingDeleteTitle')}
          title={t('promptBindingDeleteTitle')}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

function ContextSourceCard({ source }: { source: ContextSourceRecord }) {
  const metadataEntries = Object.entries(source.metadata ?? {});

  return (
    <div className="rounded-[1.2rem] border border-border/70 bg-card/90 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="text-sm font-semibold text-foreground">{source.title}</div>
          <div className="mt-2 flex flex-wrap gap-2 text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
            <span className="rounded-full bg-muted px-2.5 py-1">{source.kind}</span>
            <span
              className={cn(
                'rounded-full px-2.5 py-1 font-medium',
                source.stable
                  ? 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-300'
                  : 'bg-amber-500/12 text-amber-700 dark:text-amber-300',
              )}
            >
              {t(source.stable ? 'promptContextStable' : 'promptContextDynamic')}
            </span>
            {source.item_count ? (
              <span className="rounded-full bg-muted px-2.5 py-1">
                {t('promptContextItemCount', String(source.item_count))}
              </span>
            ) : null}
          </div>
        </div>
      </div>

      {source.summary ? (
        <p className="mt-3 text-sm leading-6 text-muted-foreground">{source.summary}</p>
      ) : null}

      {metadataEntries.length > 0 ? (
        <div className="mt-3 grid gap-2 md:grid-cols-2">
          {metadataEntries.map(([key, value]) => (
            <div key={key} className="rounded-xl border border-border/60 bg-muted/30 px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{key}</div>
              <div className="mt-1 break-all text-sm text-foreground">{formatContextMetadataValue(value)}</div>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function formatContextMetadataValue(value: unknown): string {
  if (value == null) {
    return t('none');
  }
  if (Array.isArray(value)) {
    return value.map((item) => String(item)).join(', ');
  }
  if (typeof value === 'object') {
    return JSON.stringify(value);
  }
  return String(value);
}

function ContextMetricCard({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint?: string;
}) {
  return (
    <div className="rounded-[1.2rem] border border-border/70 bg-muted/30 px-4 py-3">
      <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className="mt-2 text-lg font-semibold text-foreground">{value}</div>
      {hint ? <div className="mt-1 text-xs text-muted-foreground">{hint}</div> : null}
    </div>
  );
}

function contextBudgetTone(status: 'ok' | 'warning' | 'critical') {
  switch (status) {
    case 'critical':
      return 'bg-rose-500/15 text-rose-700 dark:text-rose-300';
    case 'warning':
      return 'bg-amber-500/15 text-amber-700 dark:text-amber-300';
    default:
      return 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300';
  }
}
