import { useState, type ReactNode } from 'react';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  type AccountBinding,
  type AccountBindingInput,
  type ChannelAccount,
  type ChannelAccountInput,
  type RuntimeAgent,
  type RuntimeAgentInput,
  useAccountBindings,
  useChannelAccounts,
  useCreateAccountBinding,
  useCreateChannelAccount,
  useCreateRuntimeAgent,
  useDeleteAccountBinding,
  useDeleteChannelAccount,
  useDeleteRuntimeAgent,
  useRuntimeAgents,
  useRuntimeTopology,
  useUpdateAccountBinding,
  useUpdateChannelAccount,
  useUpdateRuntimeAgent,
} from '@/hooks/useTopology';
import { t } from '@/lib/i18n';
import { ApiError } from '@/api/client';
import { Bot, Link2, Pencil, Plus, RadioTower, Sparkles, Trash2 } from 'lucide-react';
import { toast } from 'sonner';

type RuntimeDialogState = {
  id?: string;
  name: string;
  display_name: string;
  description: string;
  enabled: boolean;
  provider: string;
  model: string;
  prompt_id: string;
  skills_text: string;
  tools_text: string;
  policy_text: string;
};

type ChannelAccountDialogState = {
  id?: string;
  channel_type: string;
  account_key: string;
  display_name: string;
  description: string;
  enabled: boolean;
  config_text: string;
  metadata_text: string;
};

type BindingDialogState = {
  id?: string;
  channel_account_id: string;
  agent_runtime_id: string;
  binding_mode: string;
  enabled: boolean;
  allow_public_reply: boolean;
  reply_label: string;
  priority: string;
  metadata_text: string;
};

type DeleteTarget =
  | { kind: 'runtime'; id: string; label: string }
  | { kind: 'account'; id: string; label: string }
  | { kind: 'binding'; id: string; label: string };

const COMMON_CHANNEL_TYPES = ['wechat', 'telegram', 'discord', 'slack', 'gotify', 'websocket', 'serverchan'];

function defaultRuntimeState(): RuntimeDialogState {
  return {
    name: '',
    display_name: '',
    description: '',
    enabled: true,
    provider: '',
    model: '',
    prompt_id: '',
    skills_text: '',
    tools_text: '',
    policy_text: '{}',
  };
}

function defaultChannelAccountState(): ChannelAccountDialogState {
  return {
    channel_type: 'wechat',
    account_key: '',
    display_name: '',
    description: '',
    enabled: true,
    config_text: '{}',
    metadata_text: '{}',
  };
}

function defaultBindingState(): BindingDialogState {
  return {
    channel_account_id: '',
    agent_runtime_id: '',
    binding_mode: 'single_agent',
    enabled: true,
    allow_public_reply: true,
    reply_label: '',
    priority: '100',
    metadata_text: '{}',
  };
}

export default function RuntimeTopologyPage() {
  const topologyQuery = useRuntimeTopology();
  const runtimeQuery = useRuntimeAgents();
  const accountQuery = useChannelAccounts();
  const bindingQuery = useAccountBindings();

  const createRuntime = useCreateRuntimeAgent();
  const updateRuntime = useUpdateRuntimeAgent();
  const deleteRuntime = useDeleteRuntimeAgent();
  const createAccount = useCreateChannelAccount();
  const updateAccount = useUpdateChannelAccount();
  const deleteAccount = useDeleteChannelAccount();
  const createBinding = useCreateAccountBinding();
  const updateBinding = useUpdateAccountBinding();
  const deleteBinding = useDeleteAccountBinding();

  const [runtimeDialogOpen, setRuntimeDialogOpen] = useState(false);
  const [runtimeState, setRuntimeState] = useState<RuntimeDialogState>(defaultRuntimeState());
  const [accountDialogOpen, setAccountDialogOpen] = useState(false);
  const [accountState, setAccountState] = useState<ChannelAccountDialogState>(defaultChannelAccountState());
  const [bindingDialogOpen, setBindingDialogOpen] = useState(false);
  const [bindingState, setBindingState] = useState<BindingDialogState>(defaultBindingState());
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);

  const runtimes = runtimeQuery.data ?? [];
  const accounts = accountQuery.data ?? [];
  const bindings = bindingQuery.data ?? [];
  const snapshot = topologyQuery.data;
  const enabledRuntimes = runtimes.filter((runtime) => runtime.enabled);
  const enabledAccounts = accounts.filter((account) => account.enabled);
  const selectableRuntimes = bindingState.enabled ? enabledRuntimes : runtimes;
  const selectableAccounts = bindingState.enabled ? enabledAccounts : accounts;
  const selectedRuntime = runtimes.find((runtime) => runtime.id === bindingState.agent_runtime_id) ?? null;
  const selectedAccount = accounts.find((account) => account.id === bindingState.channel_account_id) ?? null;
  const bindingTargetsValid =
    !bindingState.enabled ||
    (selectedRuntime !== null && selectedRuntime.enabled && selectedAccount !== null && selectedAccount.enabled);
  const bindingCountByRuntimeID = new Map<string, number>();
  const bindingCountByAccountID = new Map<string, number>();
  for (const binding of bindings) {
    bindingCountByRuntimeID.set(
      binding.agent_runtime_id,
      (bindingCountByRuntimeID.get(binding.agent_runtime_id) ?? 0) + 1,
    );
    bindingCountByAccountID.set(
      binding.channel_account_id,
      (bindingCountByAccountID.get(binding.channel_account_id) ?? 0) + 1,
    );
  }

  const isLoading =
    topologyQuery.isLoading ||
    runtimeQuery.isLoading ||
    accountQuery.isLoading ||
    bindingQuery.isLoading;

  const isMutating =
    createRuntime.isPending ||
    updateRuntime.isPending ||
    deleteRuntime.isPending ||
    createAccount.isPending ||
    updateAccount.isPending ||
    deleteAccount.isPending ||
    createBinding.isPending ||
    updateBinding.isPending ||
    deleteBinding.isPending;

  async function submitRuntimeForm() {
    const input: RuntimeAgentInput = {
      name: runtimeState.name.trim(),
      display_name: runtimeState.display_name.trim(),
      description: runtimeState.description.trim(),
      enabled: runtimeState.enabled,
      provider: runtimeState.provider.trim(),
      model: runtimeState.model.trim(),
      prompt_id: runtimeState.prompt_id.trim(),
      skills: parseLineList(runtimeState.skills_text),
      tools: parseLineList(runtimeState.tools_text),
      policy: parseJSONObject(runtimeState.policy_text),
    };
    if (runtimeState.id) {
      await updateRuntime.mutateAsync({ id: runtimeState.id, input });
    } else {
      await createRuntime.mutateAsync(input);
    }
    setRuntimeDialogOpen(false);
    setRuntimeState(defaultRuntimeState());
  }

  async function submitChannelAccountForm() {
    const input: ChannelAccountInput = {
      channel_type: accountState.channel_type.trim(),
      account_key: accountState.account_key.trim(),
      display_name: accountState.display_name.trim(),
      description: accountState.description.trim(),
      enabled: accountState.enabled,
      config: parseJSONObject(accountState.config_text),
      metadata: parseJSONObject(accountState.metadata_text),
    };
    if (accountState.id) {
      await updateAccount.mutateAsync({ id: accountState.id, input });
    } else {
      await createAccount.mutateAsync(input);
    }
    setAccountDialogOpen(false);
    setAccountState(defaultChannelAccountState());
  }

  async function submitBindingForm() {
    if (!bindingTargetsValid) {
      throw new Error(t('runtimeTopologyBindingTargetEnabledHint'));
    }
    const input: AccountBindingInput = {
      channel_account_id: bindingState.channel_account_id,
      agent_runtime_id: bindingState.agent_runtime_id,
      binding_mode: bindingState.binding_mode,
      enabled: bindingState.enabled,
      allow_public_reply: bindingState.allow_public_reply,
      reply_label: bindingState.reply_label.trim(),
      priority: parsePositiveInteger(bindingState.priority),
      metadata: parseJSONObject(bindingState.metadata_text),
    };
    if (bindingState.id) {
      await updateBinding.mutateAsync({ id: bindingState.id, input });
    } else {
      await createBinding.mutateAsync(input);
    }
    setBindingDialogOpen(false);
    setBindingState(defaultBindingState());
  }

  async function confirmDelete() {
    if (!deleteTarget) {
      return;
    }
    if (deleteTarget.kind === 'runtime') {
      await deleteRuntime.mutateAsync(deleteTarget.id);
    } else if (deleteTarget.kind === 'account') {
      await deleteAccount.mutateAsync(deleteTarget.id);
    } else {
      await deleteBinding.mutateAsync(deleteTarget.id);
    }
    setDeleteTarget(null);
  }

  function buildDeleteDescription(target: DeleteTarget | null): string {
    if (!target) {
      return t('runtimeTopologyDeleteDescription', '');
    }
    if (target.kind === 'runtime') {
      return t(
        'runtimeTopologyDeleteRuntimeDescription',
        target.label,
        String(bindingCountByRuntimeID.get(target.id) ?? 0),
      );
    }
    if (target.kind === 'account') {
      return t(
        'runtimeTopologyDeleteAccountDescription',
        target.label,
        String(bindingCountByAccountID.get(target.id) ?? 0),
      );
    }
    return t('runtimeTopologyDeleteBindingDescription', target.label);
  }

  function openNewRuntimeDialog() {
    setRuntimeState(defaultRuntimeState());
    setRuntimeDialogOpen(true);
  }

  function openEditRuntimeDialog(runtime: RuntimeAgent) {
    setRuntimeState({
      id: runtime.id,
      name: runtime.name,
      display_name: runtime.display_name,
      description: runtime.description,
      enabled: runtime.enabled,
      provider: runtime.provider,
      model: runtime.model,
      prompt_id: runtime.prompt_id,
      skills_text: runtime.skills.join('\n'),
      tools_text: runtime.tools.join('\n'),
      policy_text: JSON.stringify(runtime.policy ?? {}, null, 2),
    });
    setRuntimeDialogOpen(true);
  }

  function openNewAccountDialog() {
    setAccountState(defaultChannelAccountState());
    setAccountDialogOpen(true);
  }

  function openEditAccountDialog(account: ChannelAccount) {
    setAccountState({
      id: account.id,
      channel_type: account.channel_type,
      account_key: account.account_key,
      display_name: account.display_name,
      description: account.description,
      enabled: account.enabled,
      config_text: JSON.stringify(account.config ?? {}, null, 2),
      metadata_text: JSON.stringify(account.metadata ?? {}, null, 2),
    });
    setAccountDialogOpen(true);
  }

  function openNewBindingDialog() {
    const next = defaultBindingState();
    const preferredAccounts = enabledAccounts.length > 0 ? enabledAccounts : accounts;
    const preferredRuntimes = enabledRuntimes.length > 0 ? enabledRuntimes : runtimes;
    if (preferredAccounts.length > 0) {
      next.channel_account_id = preferredAccounts[0].id;
    }
    if (preferredRuntimes.length > 0) {
      next.agent_runtime_id = preferredRuntimes[0].id;
    }
    if (enabledAccounts.length === 0 || enabledRuntimes.length === 0) {
      next.enabled = false;
    }
    setBindingState(next);
    setBindingDialogOpen(true);
  }

  function openEditBindingDialog(binding: AccountBinding) {
    setBindingState({
      id: binding.id,
      channel_account_id: binding.channel_account_id,
      agent_runtime_id: binding.agent_runtime_id,
      binding_mode: binding.binding_mode,
      enabled: binding.enabled,
      allow_public_reply: binding.allow_public_reply,
      reply_label: binding.reply_label,
      priority: String(binding.priority),
      metadata_text: JSON.stringify(binding.metadata ?? {}, null, 2),
    });
    setBindingDialogOpen(true);
  }

  function handleBindingEnabledChange(checked: boolean) {
    setBindingState((prev) => {
      const next = { ...prev, enabled: checked };
      if (!checked) {
        return next;
      }

      const preferredAccount = enabledAccounts.find((account) => account.id === prev.channel_account_id) ?? enabledAccounts[0];
      const preferredRuntime = enabledRuntimes.find((runtime) => runtime.id === prev.agent_runtime_id) ?? enabledRuntimes[0];

      if (preferredAccount) {
        next.channel_account_id = preferredAccount.id;
      }
      if (preferredRuntime) {
        next.agent_runtime_id = preferredRuntime.id;
      }

      return next;
    });
  }

  function askDelete(target: DeleteTarget) {
    setDeleteTarget(target);
  }

  async function guardedSubmit(fn: () => Promise<void>) {
    try {
      await fn();
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(extractApiError(err.message));
        return;
      }
      if (err instanceof Error) {
        toast.error(err.message);
      }
    }
  }

  return (
    <div>
      <Header title={t('tabRuntimeTopology')} description={t('runtimeTopologyDescription')} />

      {isLoading ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-32 rounded-3xl" />
          ))}
        </div>
      ) : null}

      {!isLoading ? (
        <div className="space-y-5">
          <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <MetricCard
              icon={<Bot className="h-4 w-4" />}
              label={t('runtimeTopologyRuntimes')}
              value={String(snapshot?.summary.runtime_count ?? runtimes.length)}
              accent="from-sky-500/18 via-sky-500/8 to-transparent"
            />
            <MetricCard
              icon={<RadioTower className="h-4 w-4" />}
              label={t('runtimeTopologyAccounts')}
              value={String(snapshot?.summary.channel_account_count ?? accounts.length)}
              accent="from-emerald-500/18 via-emerald-500/8 to-transparent"
            />
            <MetricCard
              icon={<Link2 className="h-4 w-4" />}
              label={t('runtimeTopologyBindings')}
              value={String(snapshot?.summary.binding_count ?? bindings.length)}
              accent="from-amber-500/18 via-amber-500/8 to-transparent"
            />
            <MetricCard
              icon={<Sparkles className="h-4 w-4" />}
              label={t('runtimeTopologyMultiAgent')}
              value={`${snapshot?.summary.multi_agent_accounts ?? 0}/${snapshot?.summary.channel_account_count ?? accounts.length}`}
              accent="from-violet-500/18 via-violet-500/8 to-transparent"
            />
          </section>

          <Card className="overflow-hidden rounded-[28px] border-border/70 bg-card/92 shadow-sm">
            <CardContent className="p-5">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
                <div className="max-w-3xl">
                  <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                    {t('runtimeTopologyControlTitle')}
                  </div>
                  <p className="mt-2 text-sm leading-6 text-muted-foreground">
                    {t('runtimeTopologyControlDescription')}
                  </p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button onClick={openNewRuntimeDialog}>
                    <Plus className="mr-2 h-4 w-4" />
                    {t('runtimeTopologyCreateRuntime')}
                  </Button>
                  <Button variant="outline" onClick={openNewAccountDialog}>
                    <Plus className="mr-2 h-4 w-4" />
                    {t('runtimeTopologyCreateAccount')}
                  </Button>
                  <Button
                    variant="outline"
                    onClick={openNewBindingDialog}
                    disabled={accounts.length === 0 || runtimes.length === 0}
                  >
                    <Plus className="mr-2 h-4 w-4" />
                    {t('runtimeTopologyCreateBinding')}
                  </Button>
                </div>
              </div>
              <div className="mt-4 rounded-3xl border border-border/70 bg-muted/30 p-4 text-sm text-muted-foreground">
                {accounts.length === 0 || runtimes.length === 0
                  ? t('runtimeTopologyBindingGuard')
                  : t('runtimeTopologyBindingReady')}
              </div>
            </CardContent>
          </Card>

          <section className="grid gap-5 xl:grid-cols-[1.08fr_1fr]">
            <Card className="rounded-[28px] border-border/70 bg-card/92 shadow-sm">
              <CardContent className="p-5">
                <SectionHeading
                  title={t('runtimeTopologyRuntimeSection')}
                  description={t('runtimeTopologyRuntimeSectionDescription')}
                />
                <div className="mt-4 grid gap-3">
                  {runtimes.length === 0 ? (
                    <EmptyCard text={t('runtimeTopologyNoRuntimes')} actionLabel={t('runtimeTopologyCreateRuntime')} onAction={openNewRuntimeDialog} />
                  ) : (
                    runtimes.map((runtime) => {
                      const node = snapshot?.runtimes.find((item) => item.runtime.id === runtime.id);
                      return (
                        <EntityCard
                          key={runtime.id}
                          title={runtime.display_name || runtime.name}
                          subtitle={runtime.name}
                          enabled={runtime.enabled}
                          chips={[
                            t('runtimeTopologyProvider', runtime.provider || '-'),
                            t('runtimeTopologyModel', runtime.model || '-'),
                            t('runtimeTopologyBoundAccounts', String(node?.bound_account_count ?? 0)),
                            runtime.prompt_id
                              ? t('runtimeTopologyPrompt', runtime.prompt_id)
                              : t('runtimeTopologyPromptUnset'),
                          ]}
                          description={runtime.description}
                          onEdit={() => openEditRuntimeDialog(runtime)}
                          onDelete={() =>
                            askDelete({
                              kind: 'runtime',
                              id: runtime.id,
                              label: runtime.display_name || runtime.name,
                            })
                          }
                        />
                      );
                    })
                  )}
                </div>
              </CardContent>
            </Card>

            <Card className="rounded-[28px] border-border/70 bg-card/92 shadow-sm">
              <CardContent className="p-5">
                <SectionHeading
                  title={t('runtimeTopologyAccountSection')}
                  description={t('runtimeTopologyAccountSectionDescription')}
                />
                <div className="mt-4 grid gap-3">
                  {accounts.length === 0 ? (
                    <EmptyCard text={t('runtimeTopologyNoAccounts')} actionLabel={t('runtimeTopologyCreateAccount')} onAction={openNewAccountDialog} />
                  ) : (
                    accounts.map((account) => {
                      const node = snapshot?.accounts.find((item) => item.account.id === account.id);
                      return (
                        <EntityCard
                          key={account.id}
                          title={account.display_name || account.account_key}
                          subtitle={`${account.channel_type} / ${account.account_key}`}
                          enabled={account.enabled}
                          chips={[
                            t('runtimeTopologyMode', node?.binding_mode || 'single_agent'),
                            t('runtimeTopologyBoundRuntimes', String(node?.bound_runtime_count ?? 0)),
                          ]}
                          description={account.description}
                          onEdit={() => openEditAccountDialog(account)}
                          onDelete={() =>
                            askDelete({
                              kind: 'account',
                              id: account.id,
                              label: account.display_name || account.account_key,
                            })
                          }
                        />
                      );
                    })
                  )}
                </div>
              </CardContent>
            </Card>
          </section>

          <Card className="rounded-[28px] border-border/70 bg-card/92 shadow-sm">
            <CardContent className="p-5">
              <SectionHeading
                title={t('runtimeTopologyBindingSection')}
                description={t('runtimeTopologyBindingSectionDescription')}
              />
              <div className="mt-4 grid gap-3">
                {bindings.length === 0 ? (
                  <EmptyCard text={t('runtimeTopologyNoBindings')} actionLabel={t('runtimeTopologyCreateBinding')} onAction={openNewBindingDialog} disabled={accounts.length === 0 || runtimes.length === 0} />
                ) : (
                  bindings.map((binding) => {
                    const edge = snapshot?.bindings.find((item) => item.binding.id === binding.id);
                    const account = accounts.find((item) => item.id === binding.channel_account_id);
                    const runtime = runtimes.find((item) => item.id === binding.agent_runtime_id);
                    const accountLabel = edge?.account_label || account?.display_name || account?.account_key || binding.channel_account_id;
                    const runtimeLabel = edge?.runtime_name || runtime?.display_name || runtime?.name || binding.agent_runtime_id;
                    const bindingActive = edge?.effective_enabled ?? binding.enabled;
                    return (
                      <EntityCard
                        key={binding.id}
                        title={`${accountLabel} -> ${runtimeLabel}`}
                        subtitle={`${edge?.channel_type || account?.channel_type || '-'} / ${binding.binding_mode}`}
                        enabled={bindingActive}
                        chips={[
                          t('runtimeTopologyPriority', String(binding.priority)),
                          binding.allow_public_reply
                            ? t('runtimeTopologyPublicReplyEnabled')
                            : t('runtimeTopologyPublicReplyDisabled'),
                          binding.reply_label
                            ? t('runtimeTopologyReplyLabel', binding.reply_label)
                            : t('runtimeTopologyReplyLabelUnset'),
                          edge?.disabled_reason
                            ? t(`runtimeTopologyBindingDisabledReason_${edge.disabled_reason}`)
                            : '',
                        ]}
                        description=""
                        onEdit={() => openEditBindingDialog(binding)}
                        onDelete={() =>
                          askDelete({
                            kind: 'binding',
                            id: binding.id,
                            label: `${accountLabel} -> ${runtimeLabel}`,
                          })
                        }
                      />
                    );
                  })
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      ) : null}

      <Dialog open={runtimeDialogOpen} onOpenChange={setRuntimeDialogOpen}>
        <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {runtimeState.id ? t('runtimeTopologyEditRuntime') : t('runtimeTopologyCreateRuntime')}
            </DialogTitle>
            <DialogDescription>{t('runtimeTopologyRuntimeDialogDescription')}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-1">
            <div className="grid gap-2 md:grid-cols-2 md:gap-4">
              <Field>
                <Label htmlFor="runtime-name">{t('runtimeTopologyFieldName')}</Label>
                <Input
                  id="runtime-name"
                  value={runtimeState.name}
                  onChange={(event) => setRuntimeState((prev) => ({ ...prev, name: event.target.value }))}
                />
              </Field>
              <Field>
                <Label htmlFor="runtime-display-name">{t('runtimeTopologyFieldDisplayName')}</Label>
                <Input
                  id="runtime-display-name"
                  value={runtimeState.display_name}
                  onChange={(event) =>
                    setRuntimeState((prev) => ({ ...prev, display_name: event.target.value }))
                  }
                />
              </Field>
            </div>
            <Field>
              <Label htmlFor="runtime-description">{t('runtimeTopologyFieldDescription')}</Label>
              <Textarea
                id="runtime-description"
                className="min-h-[88px]"
                value={runtimeState.description}
                onChange={(event) =>
                  setRuntimeState((prev) => ({ ...prev, description: event.target.value }))
                }
              />
            </Field>
            <div className="grid gap-2 md:grid-cols-3 md:gap-4">
              <Field>
                <Label htmlFor="runtime-provider">{t('runtimeTopologyFieldProvider')}</Label>
                <Input
                  id="runtime-provider"
                  value={runtimeState.provider}
                  onChange={(event) =>
                    setRuntimeState((prev) => ({ ...prev, provider: event.target.value }))
                  }
                />
              </Field>
              <Field>
                <Label htmlFor="runtime-model">{t('runtimeTopologyFieldModel')}</Label>
                <Input
                  id="runtime-model"
                  value={runtimeState.model}
                  onChange={(event) => setRuntimeState((prev) => ({ ...prev, model: event.target.value }))}
                />
              </Field>
              <Field>
                <Label htmlFor="runtime-prompt">{t('runtimeTopologyFieldPrompt')}</Label>
                <Input
                  id="runtime-prompt"
                  value={runtimeState.prompt_id}
                  onChange={(event) =>
                    setRuntimeState((prev) => ({ ...prev, prompt_id: event.target.value }))
                  }
                />
              </Field>
            </div>
            <div className="grid gap-2 md:grid-cols-2 md:gap-4">
              <Field>
                <Label htmlFor="runtime-skills">{t('runtimeTopologyFieldSkills')}</Label>
                <Textarea
                  id="runtime-skills"
                  className="min-h-[120px]"
                  placeholder={t('runtimeTopologyListFieldHint')}
                  value={runtimeState.skills_text}
                  onChange={(event) =>
                    setRuntimeState((prev) => ({ ...prev, skills_text: event.target.value }))
                  }
                />
              </Field>
              <Field>
                <Label htmlFor="runtime-tools">{t('runtimeTopologyFieldTools')}</Label>
                <Textarea
                  id="runtime-tools"
                  className="min-h-[120px]"
                  placeholder={t('runtimeTopologyListFieldHint')}
                  value={runtimeState.tools_text}
                  onChange={(event) =>
                    setRuntimeState((prev) => ({ ...prev, tools_text: event.target.value }))
                  }
                />
              </Field>
            </div>
            <Field>
              <Label htmlFor="runtime-policy">{t('runtimeTopologyFieldPolicy')}</Label>
              <Textarea
                id="runtime-policy"
                className="min-h-[132px] font-mono text-xs"
                value={runtimeState.policy_text}
                onChange={(event) =>
                  setRuntimeState((prev) => ({ ...prev, policy_text: event.target.value }))
                }
              />
              <p className="text-xs text-muted-foreground">{t('runtimeTopologyJsonFieldHint')}</p>
            </Field>
            <SwitchField
              label={t('enabled')}
              description={t('runtimeTopologyEnabledHint')}
              checked={runtimeState.enabled}
              onCheckedChange={(checked) => setRuntimeState((prev) => ({ ...prev, enabled: checked }))}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRuntimeDialogOpen(false)} disabled={isMutating}>
              {t('cancel')}
            </Button>
            <Button onClick={() => guardedSubmit(submitRuntimeForm)} disabled={isMutating}>
              {t('save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={accountDialogOpen} onOpenChange={setAccountDialogOpen}>
        <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {accountState.id ? t('runtimeTopologyEditAccount') : t('runtimeTopologyCreateAccount')}
            </DialogTitle>
            <DialogDescription>{t('runtimeTopologyAccountDialogDescription')}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-1">
            <div className="grid gap-2 md:grid-cols-2 md:gap-4">
              <Field>
                <Label htmlFor="account-channel-type">{t('runtimeTopologyFieldChannelType')}</Label>
                <Input
                  id="account-channel-type"
                  list="runtime-topology-channel-types"
                  value={accountState.channel_type}
                  onChange={(event) =>
                    setAccountState((prev) => ({ ...prev, channel_type: event.target.value }))
                  }
                />
                <datalist id="runtime-topology-channel-types">
                  {COMMON_CHANNEL_TYPES.map((channelType) => (
                    <option key={channelType} value={channelType} />
                  ))}
                </datalist>
              </Field>
              <Field>
                <Label htmlFor="account-key">{t('runtimeTopologyFieldAccountKey')}</Label>
                <Input
                  id="account-key"
                  value={accountState.account_key}
                  onChange={(event) =>
                    setAccountState((prev) => ({ ...prev, account_key: event.target.value }))
                  }
                />
              </Field>
            </div>
            <div className="grid gap-2 md:grid-cols-2 md:gap-4">
              <Field>
                <Label htmlFor="account-display-name">{t('runtimeTopologyFieldDisplayName')}</Label>
                <Input
                  id="account-display-name"
                  value={accountState.display_name}
                  onChange={(event) =>
                    setAccountState((prev) => ({ ...prev, display_name: event.target.value }))
                  }
                />
              </Field>
              <Field>
                <Label htmlFor="account-description">{t('runtimeTopologyFieldDescription')}</Label>
                <Input
                  id="account-description"
                  value={accountState.description}
                  onChange={(event) =>
                    setAccountState((prev) => ({ ...prev, description: event.target.value }))
                  }
                />
              </Field>
            </div>
            <Field>
              <Label htmlFor="account-config">{t('runtimeTopologyFieldConfig')}</Label>
              <Textarea
                id="account-config"
                className="min-h-[160px] font-mono text-xs"
                value={accountState.config_text}
                onChange={(event) =>
                  setAccountState((prev) => ({ ...prev, config_text: event.target.value }))
                }
              />
              <p className="text-xs text-muted-foreground">{t('runtimeTopologyJsonFieldHint')}</p>
            </Field>
            <Field>
              <Label htmlFor="account-metadata">{t('runtimeTopologyFieldMetadata')}</Label>
              <Textarea
                id="account-metadata"
                className="min-h-[132px] font-mono text-xs"
                value={accountState.metadata_text}
                onChange={(event) =>
                  setAccountState((prev) => ({ ...prev, metadata_text: event.target.value }))
                }
              />
              <p className="text-xs text-muted-foreground">{t('runtimeTopologyJsonFieldHint')}</p>
            </Field>
            <SwitchField
              label={t('enabled')}
              description={t('runtimeTopologyEnabledHint')}
              checked={accountState.enabled}
              onCheckedChange={(checked) => setAccountState((prev) => ({ ...prev, enabled: checked }))}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setAccountDialogOpen(false)} disabled={isMutating}>
              {t('cancel')}
            </Button>
            <Button onClick={() => guardedSubmit(submitChannelAccountForm)} disabled={isMutating}>
              {t('save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={bindingDialogOpen} onOpenChange={setBindingDialogOpen}>
        <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {bindingState.id ? t('runtimeTopologyEditBinding') : t('runtimeTopologyCreateBinding')}
            </DialogTitle>
            <DialogDescription>{t('runtimeTopologyBindingDialogDescription')}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-1">
            <div className="grid gap-2 md:grid-cols-2 md:gap-4">
              <Field>
                <Label>{t('runtimeTopologyFieldChannelAccount')}</Label>
                <Select
                  value={bindingState.channel_account_id}
                  onValueChange={(value) =>
                    setBindingState((prev) => ({ ...prev, channel_account_id: value }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t('runtimeTopologySelectAccount')} />
                  </SelectTrigger>
                  <SelectContent>
                    {selectableAccounts.map((account) => (
                      <SelectItem key={account.id} value={account.id}>
                        {formatBindingTargetLabel(
                          account.display_name || account.account_key,
                          account.channel_type,
                          account.enabled,
                        )}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {bindingState.enabled && selectableAccounts.length === 0 ? (
                  <p className="text-xs text-amber-600">{t('runtimeTopologyNoEnabledAccountsHint')}</p>
                ) : null}
              </Field>
              <Field>
                <Label>{t('runtimeTopologyFieldRuntime')}</Label>
                <Select
                  value={bindingState.agent_runtime_id}
                  onValueChange={(value) =>
                    setBindingState((prev) => ({ ...prev, agent_runtime_id: value }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t('runtimeTopologySelectRuntime')} />
                  </SelectTrigger>
                  <SelectContent>
                    {selectableRuntimes.map((runtime) => (
                      <SelectItem key={runtime.id} value={runtime.id}>
                        {formatBindingTargetLabel(
                          runtime.display_name || runtime.name,
                          'runtime',
                          runtime.enabled,
                        )}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {bindingState.enabled && selectableRuntimes.length === 0 ? (
                  <p className="text-xs text-amber-600">{t('runtimeTopologyNoEnabledRuntimesHint')}</p>
                ) : null}
              </Field>
            </div>
            <div className="grid gap-2 md:grid-cols-2 md:gap-4">
              <Field>
                <Label>{t('runtimeTopologyFieldBindingMode')}</Label>
                <Select
                  value={bindingState.binding_mode}
                  onValueChange={(value) => setBindingState((prev) => ({ ...prev, binding_mode: value }))}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="single_agent">{t('runtimeTopologyBindingModeSingle')}</SelectItem>
                    <SelectItem value="multi_agent">{t('runtimeTopologyBindingModeMulti')}</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <Label htmlFor="binding-priority">{t('runtimeTopologyFieldPriority')}</Label>
                <Input
                  id="binding-priority"
                  type="number"
                  min="1"
                  value={bindingState.priority}
                  onChange={(event) =>
                    setBindingState((prev) => ({ ...prev, priority: event.target.value }))
                  }
                />
              </Field>
            </div>
            <Field>
              <Label htmlFor="binding-reply-label">{t('runtimeTopologyFieldReplyLabel')}</Label>
              <Input
                id="binding-reply-label"
                value={bindingState.reply_label}
                onChange={(event) =>
                  setBindingState((prev) => ({ ...prev, reply_label: event.target.value }))
                }
              />
            </Field>
            <Field>
              <Label htmlFor="binding-metadata">{t('runtimeTopologyFieldMetadata')}</Label>
              <Textarea
                id="binding-metadata"
                className="min-h-[132px] font-mono text-xs"
                value={bindingState.metadata_text}
                onChange={(event) =>
                  setBindingState((prev) => ({ ...prev, metadata_text: event.target.value }))
                }
              />
              <p className="text-xs text-muted-foreground">{t('runtimeTopologyJsonFieldHint')}</p>
            </Field>
            <div className="grid gap-3 md:grid-cols-2">
              <SwitchField
                label={t('enabled')}
                description={t('runtimeTopologyEnabledHint')}
                checked={bindingState.enabled}
                onCheckedChange={handleBindingEnabledChange}
              />
              <SwitchField
                label={t('runtimeTopologyFieldAllowPublicReply')}
                description={t('runtimeTopologyAllowPublicReplyHint')}
                checked={bindingState.allow_public_reply}
                onCheckedChange={(checked) =>
                  setBindingState((prev) => ({ ...prev, allow_public_reply: checked }))
                }
              />
            </div>
            {bindingState.enabled && !bindingTargetsValid ? (
              <p className="text-sm text-destructive">{t('runtimeTopologyBindingTargetEnabledHint')}</p>
            ) : null}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setBindingDialogOpen(false)} disabled={isMutating}>
              {t('cancel')}
            </Button>
            <Button
              onClick={() => guardedSubmit(submitBindingForm)}
              disabled={isMutating || selectableAccounts.length === 0 || selectableRuntimes.length === 0 || !bindingTargetsValid}
            >
              {t('save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('runtimeTopologyDeleteTitle')}</DialogTitle>
            <DialogDescription>
              {buildDeleteDescription(deleteTarget)}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)} disabled={isMutating}>
              {t('cancel')}
            </Button>
            <Button variant="destructive" onClick={() => guardedSubmit(confirmDelete)} disabled={isMutating}>
              {t('delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function MetricCard({
  icon,
  label,
  value,
  accent,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  accent: string;
}) {
  return (
    <Card className="overflow-hidden rounded-[28px] border-border/70 bg-card/92 shadow-sm">
      <CardContent className="relative p-5">
        <div className={`pointer-events-none absolute inset-0 bg-gradient-to-br ${accent}`} />
        <div className="relative">
          <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
            {icon}
            <span>{label}</span>
          </div>
          <div className="mt-4 text-3xl font-semibold tracking-tight text-foreground">{value}</div>
        </div>
      </CardContent>
    </Card>
  );
}

function formatBindingTargetLabel(name: string, kind: string, enabled: boolean): string {
  if (enabled) {
    return `${name} (${kind})`;
  }
  return `${name} (${kind} · ${t('disabled')})`;
}

function SectionHeading({ title, description }: { title: string; description: string }) {
  return (
    <div>
      <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{title}</div>
      <p className="mt-2 text-sm leading-6 text-muted-foreground">{description}</p>
    </div>
  );
}

function EntityCard({
  title,
  subtitle,
  enabled,
  chips,
  description,
  onEdit,
  onDelete,
}: {
  title: string;
  subtitle: string;
  enabled: boolean;
  chips: string[];
  description: string;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="rounded-3xl border border-border/70 bg-muted/35 p-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <div className="text-sm font-semibold text-foreground">{title}</div>
          <div className="mt-1 text-xs text-muted-foreground">{subtitle}</div>
          {description ? <p className="mt-3 text-sm leading-6 text-muted-foreground">{description}</p> : null}
        </div>
        <div className="flex items-center gap-2 self-start">
          <StatusPill enabled={enabled} />
          <Button variant="ghost" size="icon" onClick={onEdit} aria-label={t('edit')}>
            <Pencil className="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" onClick={onDelete} aria-label={t('delete')}>
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </div>
      <div className="mt-3 flex flex-wrap gap-2 text-xs text-muted-foreground">
        {chips.filter(Boolean).map((chip) => (
          <span key={chip} className="rounded-full bg-background/80 px-2.5 py-1">
            {chip}
          </span>
        ))}
      </div>
    </div>
  );
}

function StatusPill({ enabled }: { enabled: boolean }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${
        enabled
          ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
          : 'bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300'
      }`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${enabled ? 'bg-emerald-500' : 'bg-slate-400'}`} />
      {enabled ? t('on') : t('off')}
    </span>
  );
}

function EmptyCard({
  text,
  actionLabel,
  onAction,
  disabled,
}: {
  text: string;
  actionLabel?: string;
  onAction?: () => void;
  disabled?: boolean;
}) {
  return (
    <div className="rounded-3xl border border-dashed border-border px-4 py-8 text-sm text-muted-foreground">
      <p>{text}</p>
      {actionLabel && onAction ? (
        <Button className="mt-4" variant="outline" onClick={onAction} disabled={disabled}>
          {actionLabel}
        </Button>
      ) : null}
    </div>
  );
}

function Field({ children }: { children: ReactNode }) {
  return <div className="grid gap-2">{children}</div>;
}

function SwitchField({
  label,
  description,
  checked,
  onCheckedChange,
}: {
  label: string;
  description: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <div className="flex items-start justify-between gap-4 rounded-2xl border border-border/70 bg-muted/25 px-4 py-3">
      <div>
        <div className="text-sm font-medium text-foreground">{label}</div>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
      </div>
      <Switch checked={checked} onCheckedChange={onCheckedChange} />
    </div>
  );
}

function parseLineList(value: string): string[] {
  return value
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseJSONObject(value: string): Record<string, unknown> {
  const trimmed = value.trim();
  if (trimmed === '') {
    return {};
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    throw new Error(t('invalidJson', message));
  }
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error(t('runtimeTopologyJsonObjectRequired'));
  }
  return parsed as Record<string, unknown>;
}

function parsePositiveInteger(value: string): number {
  const parsed = Number.parseInt(value.trim(), 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new Error(t('runtimeTopologyPriorityInvalid'));
  }
  return parsed;
}

function extractApiError(message: string): string {
  const trimmed = message.trim();
  try {
    const parsed = JSON.parse(trimmed) as { error?: string };
    if (parsed?.error) {
      return parsed.error;
    }
  } catch {
    return trimmed;
  }
  return trimmed;
}
