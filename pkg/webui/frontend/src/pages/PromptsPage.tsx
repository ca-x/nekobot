import { useMemo, useState } from 'react';
import { Bot, Plus, Sparkles, Trash2, Wand2, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
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
  usePrompts,
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

  const [selectedPromptID, setSelectedPromptID] = useState<string>('');
  const [draft, setDraft] = useState<PromptInput>(emptyPromptDraft());
  const [tagsInput, setTagsInput] = useState('');
  const [bindingDraft, setBindingDraft] = useState<PromptBindingInput>(emptyBindingDraft());
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ type: 'prompt' | 'binding'; id: string } | null>(null);

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

  return (
    <>
      <div className="prompts-page flex h-[calc(100dvh-4rem)] flex-col overflow-hidden">
        <Header title={t('tabPrompts')} />

        <div className="grid min-h-0 flex-1 gap-4 xl:grid-cols-[minmax(0,1.1fr)_380px]">
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

                <Button className="w-full rounded-2xl" onClick={() => resetPromptForm(null)}>
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
                        <div>
                          <div className="text-sm font-semibold text-foreground">{item.name}</div>
                          <div className="mt-1 text-xs uppercase tracking-[0.16em] text-muted-foreground">{item.key}</div>
                        </div>
                        <span
                          className={cn(
                            'rounded-full px-2.5 py-1 text-[11px] font-medium',
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

              <div className="space-y-4 p-5">
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="prompt-key">{t('promptKey')}</Label>
                    <Input id="prompt-key" value={draft.key} onChange={(event) => setDraft((current) => ({ ...current, key: event.target.value }))} />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="prompt-name">{t('promptName')}</Label>
                    <Input id="prompt-name" value={draft.name} onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))} />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="prompt-description">{t('promptDescription')}</Label>
                  <Input
                    id="prompt-description"
                    value={draft.description}
                    onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))}
                  />
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label>{t('promptMode')}</Label>
                    <Select
                      value={draft.mode}
                      onValueChange={(value: PromptMode) => setDraft((current) => ({ ...current, mode: value }))}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="system">{t('promptModeSystem')}</SelectItem>
                        <SelectItem value="user">{t('promptModeUser')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="flex items-center justify-between rounded-[1.2rem] border border-border/70 bg-muted/20 px-4 py-3">
                    <div>
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
                  <Input id="prompt-tags" value={tagsInput} onChange={(event) => setTagsInput(event.target.value)} />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="prompt-template">{t('promptTemplate')}</Label>
                  <Textarea
                    id="prompt-template"
                    rows={14}
                    className="min-h-[280px]"
                    value={draft.template}
                    onChange={(event) => setDraft((current) => ({ ...current, template: event.target.value }))}
                  />
                </div>

                <div className="flex flex-wrap gap-2">
                  <Button
                    className="rounded-full"
                    onClick={() => void handleSavePrompt()}
                    disabled={createPrompt.isPending || updatePrompt.isPending}
                  >
                    {selectedPrompt ? t('save') : t('promptCreateAction')}
                  </Button>
                  {selectedPrompt && (
                    <Button
                      variant="outline"
                      className="rounded-full text-destructive hover:text-destructive"
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
                <div className="grid gap-4 md:grid-cols-2">
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
                      <SelectTrigger>
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
                      <SelectTrigger>
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
                      value={bindingDraft.target}
                      onChange={(event) => setBindingDraft((current) => ({ ...current, target: event.target.value }))}
                      placeholder={bindingDraft.scope === 'channel' ? 'wechat' : 'webui-chat:...'}
                    />
                  </div>
                )}

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="binding-priority">{t('promptBindingPriority')}</Label>
                    <Input
                      id="binding-priority"
                      type="number"
                      value={String(bindingDraft.priority)}
                      onChange={(event) =>
                        setBindingDraft((current) => ({
                          ...current,
                          priority: Number(event.target.value || 0),
                        }))
                      }
                    />
                  </div>

                  <div className="flex items-center justify-between rounded-[1.2rem] border border-[hsl(var(--gray-200))] px-4 py-3">
                    <div>
                      <div className="text-sm font-medium text-[hsl(var(--gray-900))]">{t('enabled')}</div>
                      <div className="text-xs text-muted-foreground">{t('promptBindingEnabledHint')}</div>
                    </div>
                    <Switch
                      checked={bindingDraft.enabled}
                      onCheckedChange={(checked) => setBindingDraft((current) => ({ ...current, enabled: checked }))}
                    />
                  </div>
                </div>

                <Button className="w-full rounded-full" onClick={() => void handleCreateBindingRecord()} disabled={createBinding.isPending}>
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
                  </div>
                )}

                {bindingsWithPrompt.length > 0 && (
                  <Button
                    variant="outline"
                    className="w-full rounded-full"
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
              <DialogTitle>{t('deleteConfirmTitle')}</DialogTitle>
              <DialogDescription>{t('deleteConfirmDescription')}</DialogDescription>
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
        <div>
          <div className="text-sm font-semibold text-foreground">
            {prompt?.name ?? t('promptBindingMissingPrompt')}
          </div>
          <div className="mt-1 flex flex-wrap gap-2 text-xs text-muted-foreground">
            <span>{formatScope(binding.scope)}</span>
            {binding.target && <span>{binding.target}</span>}
            <span>{t('promptBindingPriority')}: {binding.priority}</span>
          </div>
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="rounded-full text-muted-foreground hover:text-destructive"
          onClick={() => onDelete(binding.id)}
          disabled={deleting}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
