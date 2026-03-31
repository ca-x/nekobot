import { useEffect, useMemo, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { Send, Sparkles, RefreshCw, Trash2, Radio, Wand2, AlertCircle, ArrowRight, RotateCcw, Eye, EyeOff, ShieldCheck, Settings2 } from 'lucide-react';
import { toast } from 'sonner';

import { api } from '@/api/client';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useChat, type ChatMessage } from '@/hooks/useChat';
import { useWatchStatus } from '@/hooks/useConfig';
import { usePrompts, usePromptSessionBindings } from '@/hooks/usePrompts';
import { useRuntimeAgents } from '@/hooks/useTopology';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';

interface ProviderInfo {
  name: string;
  default_model?: string;
  models?: string[];
}

interface ProviderGroupInfo {
  name: string;
  strategy?: string;
  members?: string[];
}

interface ConfigData {
  agents?: {
    defaults?: {
      provider?: string;
      model?: string;
      fallback?: string[];
      provider_groups?: ProviderGroupInfo[];
    };
  };
}

interface ModelEntry {
  provider: string;
  model: string;
}

interface RouteTarget {
  name: string;
  type: 'provider' | 'group';
  strategy?: string;
}

const EMPTY_VALUE = '__default__';
const MODEL_VALUE_SEPARATOR = '::';

function toSelectValue(value: string): string {
  return value.trim() === '' ? EMPTY_VALUE : value;
}

function fromSelectValue(value: string): string {
  return value === EMPTY_VALUE ? '' : value;
}

function encodeModelValue(entry: ModelEntry): string {
  return `${entry.provider}${MODEL_VALUE_SEPARATOR}${entry.model}`;
}

function decodeModelValue(value: string): ModelEntry {
  const separatorIndex = value.indexOf(MODEL_VALUE_SEPARATOR);
  if (separatorIndex === -1) {
    return { provider: '', model: value };
  }
  return {
    provider: value.slice(0, separatorIndex),
    model: value.slice(separatorIndex + MODEL_VALUE_SEPARATOR.length),
  };
}

function findModelEntry(models: ModelEntry[], provider: string, model: string): ModelEntry | undefined {
  const normalizedModel = model.trim();
  if (!normalizedModel) {
    return undefined;
  }

  const normalizedProvider = provider.trim();
  if (normalizedProvider) {
    return models.find((entry) => entry.provider === normalizedProvider && entry.model === normalizedModel);
  }

  return models.find((entry) => entry.provider === 'default' && entry.model === normalizedModel)
    ?? models.find((entry) => entry.model === normalizedModel);
}

function formatTime(timestamp: number): string {
  return new Date(timestamp).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
  });
}

function useProviders() {
  return useQuery<ProviderInfo[]>({
    queryKey: ['providers'],
    queryFn: () => api.get('/api/providers'),
    staleTime: 30_000,
  });
}

function useAppConfig() {
  return useQuery<ConfigData>({
    queryKey: ['config'],
    queryFn: () => api.get('/api/config'),
    staleTime: 30_000,
  });
}

function buildModelList(
  providers: ProviderInfo[],
  config: ConfigData | undefined,
): {
  models: ModelEntry[];
  defaultProvider: string;
  defaultModel: string;
  defaultFallback: string[];
  routeTargets: RouteTarget[];
} {
  const models: ModelEntry[] = [];
  const seen = new Set<string>();
  const defaults = config?.agents?.defaults;
  const defaultProvider = defaults?.provider?.trim() || '';
  const defaultModel = defaults?.model?.trim() || '';
  const defaultFallback = defaults?.fallback || [];
  const routeTargets: RouteTarget[] = [];
  const targetSeen = new Set<string>();

  const add = (provider: string, model: string) => {
    const normalizedProvider = provider.trim() || 'default';
    const normalizedModel = model.trim();
    if (!normalizedModel) {
      return;
    }
    const key = `${normalizedProvider}::${normalizedModel}`;
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    models.push({ provider: normalizedProvider, model: normalizedModel });
  };

  const addRouteTarget = (target: RouteTarget) => {
    const name = target.name.trim();
    if (!name || targetSeen.has(name)) {
      return;
    }
    targetSeen.add(name);
    routeTargets.push({ ...target, name });
  };

  add(defaultProvider, defaultModel);
  for (const provider of providers) {
    addRouteTarget({ name: provider.name, type: 'provider' });
    if (provider.default_model) {
      add(provider.name, provider.default_model);
    }
    for (const model of provider.models || []) {
      add(provider.name, model);
    }
  }

  for (const group of defaults?.provider_groups || []) {
    if (!group?.name?.trim()) {
      continue;
    }
    addRouteTarget({
      name: group.name,
      type: 'group',
      strategy: group.strategy?.trim() || '',
    });
  }

  return { models, defaultProvider, defaultModel, defaultFallback, routeTargets };
}

function MessageBubble({ message }: { message: ChatMessage }) {
  if (message.role === 'user') {
    return (
      <div className="flex justify-end">
        <div className="max-w-[82%] space-y-2">
          <div className="rounded-[1.4rem] rounded-br-md bg-[hsl(var(--gray-900))] px-4 py-3 text-sm leading-6 text-white shadow-[0_16px_40px_-24px_rgba(20,15,10,0.75)] whitespace-pre-wrap break-words">
            {message.content}
          </div>
          <div className="eyebrow-label mono-data text-right text-muted-foreground/80">
            {formatTime(message.timestamp)}
          </div>
        </div>
      </div>
    );
  }

  if (message.role === 'assistant') {
    return (
      <div className="flex justify-start">
        <div className="max-w-[88%] space-y-2">
          <div className="rounded-[1.4rem] rounded-bl-md border border-[hsl(var(--brand-200))] bg-white/90 px-4 py-3 text-sm leading-6 text-foreground shadow-[0_18px_42px_-30px_rgba(120,55,75,0.35)] backdrop-blur whitespace-pre-wrap break-words">
            {message.content}
          </div>
          <div className="eyebrow-label mono-data text-muted-foreground/80">
            {formatTime(message.timestamp)}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex justify-center">
      <div
        className={cn(
          'max-w-[90%] rounded-full px-3 py-1.5 text-xs',
          message.role === 'error'
            ? 'bg-destructive/10 text-destructive'
            : 'bg-[hsl(var(--gray-100))] text-muted-foreground',
        )}
      >
        {message.content}
      </div>
    </div>
  );
}

function StatusPill({
  status,
  isAwaitingReply,
}: {
  status: string;
  isAwaitingReply: boolean;
}) {
  const colorClass =
    status === 'connected'
      ? 'bg-emerald-500'
      : status === 'connecting'
        ? 'bg-amber-500 animate-pulse'
        : 'bg-rose-500';

  const label =
    status === 'connected'
      ? t('wsConnected')
      : status === 'connecting'
        ? t('chatConnecting')
        : t('wsDisconnected');

  return (
    <div className="flex max-w-full flex-col items-start gap-1.5 sm:items-end">
      <div className="inline-flex max-w-full items-center gap-2 rounded-full border border-border/70 bg-card/92 px-3.5 py-2 text-xs text-muted-foreground shadow-sm backdrop-blur">
        <span className={cn('h-2.5 w-2.5 shrink-0 rounded-full', colorClass)} />
        <span className="min-w-0 break-words font-medium">{label}</span>
      </div>
      {isAwaitingReply && (
        <span className="inline-flex h-7 items-center rounded-full bg-accent px-2.5 text-[11px] font-medium text-accent-foreground whitespace-nowrap">
          {t('chatWaitingReply')}
        </span>
      )}
    </div>
  );
}

export default function ChatPage() {
  const { data: providers = [] } = useProviders();
  const { data: config } = useAppConfig();
  const { data: prompts = [] } = usePrompts();
  const { data: runtimes = [] } = useRuntimeAgents();
  const {
    messages,
    sendMessage,
    clearMessages,
    replaceMessages,
    connectionStatus,
    reconnect,
    routeSettings,
    routeResult,
    isAwaitingReply,
    fileMentionFeedback,
    clearFileMentionFeedback,
  } = useChat();
  const { data: watchStatus } = useWatchStatus();

  const { models, defaultProvider, defaultModel, defaultFallback, routeTargets } = buildModelList(providers, config);
  const [selectedProvider, setSelectedProvider] = useState('');
  const [selectedModel, setSelectedModel] = useState('');
  const [customModel, setCustomModel] = useState('');
  const [selectedRuntimeID, setSelectedRuntimeID] = useState('');
  const [selectedFallbackTargets, setSelectedFallbackTargets] = useState<string[]>([]);
  const [selectedSystemPromptIDs, setSelectedSystemPromptIDs] = useState<string[]>([]);
  const [selectedUserPromptIDs, setSelectedUserPromptIDs] = useState<string[]>([]);
  const [chatInput, setChatInput] = useState('');
  const [showFileMentionDetails, setShowFileMentionDetails] = useState(false);
  const scrollEndRef = useRef<HTMLDivElement>(null);
  const routeTargetMap = useMemo(
    () => new Map(routeTargets.map((target) => [target.name, target])),
    [routeTargets],
  );

  const filteredModels = useMemo(() => {
    if (!selectedProvider) {
      return models;
    }
    if (routeTargetMap.get(selectedProvider)?.type === 'group') {
      return models;
    }
    return models.filter((entry) => entry.provider === selectedProvider);
  }, [models, routeTargetMap, selectedProvider]);

  useEffect(() => {
    if (routeSettings.provider || routeSettings.model || routeSettings.fallback.length > 0) {
      return;
    }
    if (selectedProvider || selectedModel || customModel || selectedFallbackTargets.length > 0) {
      return;
    }
    setSelectedProvider(defaultProvider);
    setSelectedModel(defaultModel);
    setCustomModel(defaultModel);
    setSelectedFallbackTargets(defaultFallback);
  }, [
    customModel,
    defaultFallback,
    defaultModel,
    defaultProvider,
    routeSettings.fallback,
    routeSettings.model,
    routeSettings.provider,
    selectedFallbackTargets.length,
    selectedModel,
    selectedProvider,
  ]);

  useEffect(() => {
    if (!routeSettings.provider && !routeSettings.model && routeSettings.fallback.length === 0) {
      return;
    }
    setSelectedProvider(routeSettings.provider);
    setSelectedModel(routeSettings.model);
    setCustomModel(routeSettings.model);
    setSelectedFallbackTargets(routeSettings.fallback);
  }, [routeSettings]);

  useEffect(() => {
    if (!selectedProvider) {
      return;
    }
    setSelectedFallbackTargets((current) =>
      current.filter((target) => target !== selectedProvider),
    );
  }, [selectedProvider]);

  useEffect(() => {
    scrollEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, isAwaitingReply]);

  const messageCount = messages.filter((message) => message.role === 'user' || message.role === 'assistant').length;
  const activeModel = customModel.trim() || selectedModel.trim();
  const activeProvider = selectedProvider.trim();
  const activeRuntimeID = selectedRuntimeID.trim();
  const activeSessionBindingID = activeRuntimeID ? `route:${activeRuntimeID}:webui-chat` : 'webui-chat';
  const { data: sessionPromptBindings } = usePromptSessionBindings(activeSessionBindingID);
  const activeFallback = selectedFallbackTargets.filter((target) => target.trim().length > 0);
  const actualProvider = routeResult?.actual_provider?.trim() || '';
  const actualModel = routeResult?.actual_model?.trim() || '';
  const resolvedOrder = routeResult?.resolved_order ?? [];
  const enabledRuntimes = useMemo(
    () => runtimes.filter((runtime) => runtime.enabled),
    [runtimes],
  );
  const activeRuntime = useMemo(
    () => enabledRuntimes.find((runtime) => runtime.id === activeRuntimeID) ?? null,
    [activeRuntimeID, enabledRuntimes],
  );
  const watchEnabled = !!watchStatus?.enabled;
  const watchRunning = !!watchStatus?.running;
  const watchLabel = watchEnabled ? t('chatWatchOn') : t('chatWatchOff');
  const selectedModelEntry = findModelEntry(filteredModels, selectedProvider, selectedModel);
  const selectedModelValue = selectedModelEntry ? encodeModelValue(selectedModelEntry) : EMPTY_VALUE;
  const fallbackRouteTargets = routeTargets.filter((target) => target.name !== activeProvider);
  const systemPrompts = useMemo(
    () => prompts.filter((item) => item.enabled && item.mode === 'system'),
    [prompts],
  );
  const userPrompts = useMemo(
    () => prompts.filter((item) => item.enabled && item.mode === 'user'),
    [prompts],
  );
  const hasProviders = providers.length > 0;
  const hasEnabledPrompts = systemPrompts.length + userPrompts.length > 0;

  useEffect(() => {
    if (!sessionPromptBindings) {
      return;
    }
    setSelectedSystemPromptIDs(sessionPromptBindings.system_prompt_ids ?? []);
    setSelectedUserPromptIDs(sessionPromptBindings.user_prompt_ids ?? []);
  }, [sessionPromptBindings]);

  function handleProviderChange(value: string) {
    const provider = fromSelectValue(value);
    setSelectedProvider(provider);
    if (!provider) {
      return;
    }
    const candidate = models.find((entry) => entry.provider === provider && entry.model === selectedModel)
      ? selectedModel
      : models.find((entry) => entry.provider === provider)?.model || '';
    setSelectedModel(candidate);
    if (!customModel.trim()) {
      setCustomModel(candidate);
    }
  }

  function handleModelChange(value: string) {
    if (value === EMPTY_VALUE) {
      setSelectedModel('');
      setCustomModel('');
      return;
    }

    const entry = decodeModelValue(value);
    setSelectedModel(entry.model);
    setCustomModel(entry.model);
    if (!selectedProvider) {
      setSelectedProvider(entry.provider === 'default' ? '' : entry.provider);
    }
  }

  function handleToggleFallbackTarget(targetName: string) {
    setSelectedFallbackTargets((current) =>
      current.includes(targetName)
        ? current.filter((item) => item !== targetName)
        : [...current, targetName],
    );
  }

  function handleTogglePrompt(promptID: string, mode: 'system' | 'user') {
    const setter = mode === 'system' ? setSelectedSystemPromptIDs : setSelectedUserPromptIDs;
    setter((current) =>
      current.includes(promptID)
        ? current.filter((item) => item !== promptID)
        : [...current, promptID],
    );
  }

  function handleSend() {
    const content = chatInput.trim();
    if (!content || connectionStatus !== 'connected') {
      return;
    }

    sendMessage(content, {
      provider: activeProvider,
      model: activeModel,
      fallbackProviders: activeFallback,
      systemPromptIDs: selectedSystemPromptIDs,
      userPromptIDs: selectedUserPromptIDs,
      runtimeID: activeRuntimeID,
    });
    setChatInput('');
  }

  function handleInputKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      handleSend();
    }
  }

  async function handleUndo() {
    try {
      const undoSessionID = activeRuntimeID ? `route:${activeRuntimeID}:webui-chat` : 'webui-chat';
      const result = await api.post<{
        undone_steps: number;
        remaining_turns: number;
        message_count: number;
        messages: { role: string; content: string }[];
      }>(`/api/chat/session/${encodeURIComponent(undoSessionID)}/undo`, { steps: 1 });
      if ((result.undone_steps ?? 0) <= 0) {
        toast.info(t('chatUndoNothing'));
        return;
      }
      replaceMessages((result.messages ?? []).map((message, index) => ({
        role: message.role as ChatMessage['role'],
        content: message.content,
        timestamp: Date.now() + index,
      })));
      clearFileMentionFeedback();
      toast.success(t('chatUndoSuccess', String(result.undone_steps ?? 0)));
    } catch (error) {
      const message = error instanceof Error ? error.message : t('chatUndoFailed');
      toast.error(message);
    }
  }

  return (
    <div className="chat-page flex min-h-0 flex-1 flex-col overflow-hidden">
      <Header title={t('tabChat')} className="mb-3 md:mb-4" />

      <div className="relative flex min-h-0 flex-1 flex-col gap-4 lg:grid lg:grid-cols-[320px_minmax(0,1fr)] lg:gap-5">
        <div className="absolute inset-x-0 top-0 -z-10 h-48 rounded-[2rem] bg-[radial-gradient(circle_at_top_left,rgba(198,104,140,0.22),transparent_48%),radial-gradient(circle_at_top_right,rgba(229,183,107,0.22),transparent_42%),linear-gradient(180deg,rgba(255,255,255,0.92),rgba(255,247,243,0.55))]" />

        <Card className="overflow-hidden border-border/70 bg-card/88 shadow-[0_20px_60px_-36px_rgba(120,55,75,0.45)] backdrop-blur lg:sticky lg:top-3 lg:h-fit">
          <CardHeader className="space-y-4 border-b border-[hsl(var(--gray-200))]/80 bg-[linear-gradient(135deg,rgba(255,248,246,0.96),rgba(252,239,244,0.9))]">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="space-y-2">
                <div className="eyebrow-label inline-flex items-center gap-2 rounded-full bg-[hsl(var(--brand-50))] px-3 py-1 text-[hsl(var(--brand-700))]">
                  <Wand2 className="h-3.5 w-3.5" />
                  {t('chatRouteCardTitle')}
                </div>
                <CardTitle className="text-xl font-semibold text-[hsl(var(--gray-900))]">
                  {t('chatRouteHeadline')}
                </CardTitle>
              </div>
              <StatusPill status={connectionStatus} isAwaitingReply={isAwaitingReply} />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="rounded-2xl border border-border/70 bg-card/90 p-3">
                <div className="eyebrow-label text-muted-foreground">
                  {t('chatMetricMessages')}
                </div>
                <div className="mono-data mt-2 text-2xl font-semibold text-[hsl(var(--gray-900))]">
                  {messageCount}
                </div>
              </div>
              <div className="rounded-2xl border border-border/70 bg-card/90 p-3">
                <div className="eyebrow-label text-muted-foreground">
                  {t('chatMetricModel')}
                </div>
                <div className="mt-2 truncate text-sm font-semibold text-[hsl(var(--gray-900))]">
                  {activeModel || 'auto'}
                </div>
              </div>
            </div>
          </CardHeader>

          <CardContent className="space-y-4 p-4">
            {(!hasProviders || !hasEnabledPrompts) && (
              <div className="rounded-[1.5rem] border border-[hsl(var(--brand-200))] bg-[linear-gradient(180deg,rgba(255,252,250,0.92),rgba(252,241,245,0.8))] p-4 dark:bg-card/90">
                <div className="eyebrow-label mb-3 flex items-center gap-2 text-[hsl(var(--brand-700))]">
                  <Sparkles className="h-3.5 w-3.5" />
                  {t('chatSetupGuide')}
                </div>
                <div className="space-y-3">
                  {!hasProviders && (
                    <div className="rounded-2xl border border-amber-300/60 bg-amber-50/80 p-4 dark:border-amber-700/50 dark:bg-amber-950/30">
                      <div className="flex items-start gap-3">
                        <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300">
                          <AlertCircle className="h-4 w-4" />
                        </div>
                        <div className="space-y-2">
                          <div className="text-sm font-semibold text-amber-900 dark:text-amber-200">{t('chatNoProvidersTitle')}</div>
                          <p className="text-xs leading-5 text-amber-800/80 dark:text-amber-300/80">{t('chatNoProvidersDescription')}</p>
                          <Link
                            to="/providers"
                            className="inline-flex items-center gap-1.5 rounded-full bg-amber-900 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-amber-800 dark:bg-amber-200 dark:text-amber-900 dark:hover:bg-amber-100"
                          >
                            {t('chatGoToProviders')}
                            <ArrowRight className="h-3 w-3" />
                          </Link>
                        </div>
                      </div>
                    </div>
                  )}
                  {!hasEnabledPrompts && (
                    <div className="rounded-[1.4rem] border border-[hsl(var(--brand-200))] bg-[hsl(var(--brand-50))]/60 p-4 dark:border-[hsl(var(--brand-800))] dark:bg-[hsl(var(--brand-950))]/20">
                      <div className="flex items-start gap-3">
                        <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-[hsl(var(--brand-100))] text-[hsl(var(--brand-700))] dark:bg-[hsl(var(--brand-900))]/50 dark:text-[hsl(var(--brand-300))]">
                          <Sparkles className="h-4 w-4" />
                        </div>
                        <div className="space-y-2">
                          <div className="text-sm font-semibold text-[hsl(var(--brand-900))] dark:text-[hsl(var(--brand-200))]">{t('chatNoPromptsTitle')}</div>
                          <p className="text-xs leading-5 text-[hsl(var(--brand-800))]/70 dark:text-[hsl(var(--brand-300))]/70">{t('chatNoPromptsDescription')}</p>
                          <Link
                            to="/prompts"
                            className="inline-flex items-center gap-1.5 rounded-full bg-[hsl(var(--brand-700))] px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-[hsl(var(--brand-800))] dark:bg-[hsl(var(--brand-300))] dark:text-[hsl(var(--brand-900))] dark:hover:bg-[hsl(var(--brand-200))]"
                          >
                            {t('chatGoToPrompts')}
                            <ArrowRight className="h-3 w-3" />
                          </Link>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}

            <div className="rounded-[1.5rem] border border-[hsl(var(--gray-200))] bg-[linear-gradient(180deg,rgba(255,255,255,0.78),rgba(249,244,241,0.92))] p-4 dark:bg-card/90">
              <div className="eyebrow-label mb-3 flex items-center gap-2 text-muted-foreground">
                <Sparkles className="h-3.5 w-3.5" />
                {t('chatActiveRoute')}
              </div>
              <div className="flex flex-wrap gap-2">
                <span className="max-w-full break-all rounded-full border border-[hsl(var(--brand-200))] bg-card px-3 py-1.5 text-xs text-muted-foreground">
                  {activeRuntime
                    ? t('chatRuntimeSelected', activeRuntime.display_name || activeRuntime.name)
                    : t('chatRuntimeAuto')}
                </span>
                <span className="max-w-full break-all rounded-full bg-[hsl(var(--gray-900))] px-3 py-1.5 text-xs font-medium text-white dark:bg-[hsl(var(--gray-100))] dark:text-[hsl(var(--gray-800))]">
                  {activeProvider || t('chatRouteAuto')}
                </span>
                <span className="max-w-full break-all rounded-full bg-[hsl(var(--brand-100))] px-3 py-1.5 text-xs font-medium text-[hsl(var(--brand-800))]">
                  {activeModel || t('chatModelUnset')}
                </span>
                <span className="max-w-full break-all rounded-full border border-border/70 bg-card px-3 py-1.5 text-xs text-muted-foreground">
                  {activeFallback.length > 0
                    ? `${t('fallbackProviders')}: ${activeFallback.join(' -> ')}`
                    : t('chatNoFallback')}
                </span>
              </div>
            </div>

            <div className="rounded-[1.5rem] border border-[hsl(var(--brand-200))] bg-[linear-gradient(180deg,rgba(255,252,250,0.92),rgba(252,241,245,0.8))] p-4 dark:bg-card/90">
              <div className="eyebrow-label mb-3 flex items-center gap-2 text-[hsl(var(--brand-700))]">
                <Radio className="h-3.5 w-3.5" />
                {t('chatActualRoute')}
              </div>
              <div className="flex flex-wrap gap-2">
                {actualProvider || actualModel ? (
                  <>
                    {actualProvider && (
                      <span className="max-w-full break-all rounded-full bg-[hsl(var(--gray-900))] px-3 py-1.5 text-xs font-medium text-white dark:bg-[hsl(var(--gray-100))] dark:text-[hsl(var(--gray-800))]">
                        {actualProvider}
                      </span>
                    )}
                    {actualModel && (
                      <span className="max-w-full break-all rounded-full bg-card px-3 py-1.5 text-xs font-medium text-[hsl(var(--brand-800))] dark:text-foreground">
                        {actualModel}
                      </span>
                    )}
                    {resolvedOrder.length > 0 && (
                      <span className="max-w-full break-all rounded-full border border-[hsl(var(--brand-200))] bg-card px-3 py-1.5 text-xs text-[hsl(var(--brand-800))] dark:text-foreground">
                        {t('chatResolvedOrder')}: {resolvedOrder.join(' -> ')}
                      </span>
                    )}
                  </>
                ) : (
                  <span className="rounded-full bg-[hsl(var(--gray-100))] px-3 py-1.5 text-xs font-medium text-muted-foreground dark:bg-[hsl(var(--gray-200))]">
                    {t('chatActualRoutePending')}
                  </span>
                )}
              </div>
              <p className="mt-3 text-sm leading-6 text-muted-foreground">
                {t('chatActualRouteHint')}
              </p>
            </div>

            <div className="rounded-[1.5rem] border border-[hsl(var(--gray-200))] bg-[linear-gradient(180deg,rgba(245,249,255,0.9),rgba(241,245,255,0.82))] p-4 dark:bg-card/90">
              <div className="eyebrow-label mb-3 flex items-center gap-2 text-muted-foreground">
                <ShieldCheck className="h-3.5 w-3.5" />
                {t('chatHarnessConsoleTitle')}
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="rounded-2xl border border-border/70 bg-card/85 p-4">
                  <div className="text-sm font-semibold text-foreground">{t('chatHarnessAuditTitle')}</div>
                  <div className="mt-2 text-xs leading-5 text-muted-foreground">{t('chatHarnessAuditDescription')}</div>
                  <Button asChild variant="outline" className="mt-3 rounded-full">
                    <Link to="/harness/audit">
                      <ShieldCheck className="mr-2 h-4 w-4" />
                      {t('chatOpenAudit')}
                    </Link>
                  </Button>
                </div>
                <div className="rounded-2xl border border-border/70 bg-card/85 p-4">
                  <div className="text-sm font-semibold text-foreground">{t('chatHarnessWatchTitle')}</div>
                  <div className="mt-2 text-xs leading-5 text-muted-foreground">{t('chatHarnessWatchDescription')}</div>
                  <Button asChild variant="outline" className="mt-3 rounded-full">
                    <Link to="/config">
                      <Settings2 className="mr-2 h-4 w-4" />
                      {t('chatOpenWatchConfig')}
                    </Link>
                  </Button>
                </div>
              </div>
            </div>

            <div className="space-y-3">
              <label className="eyebrow-label text-muted-foreground">
                {t('chatRuntimeTarget')}
              </label>
              <Select value={toSelectValue(selectedRuntimeID)} onValueChange={(value) => setSelectedRuntimeID(fromSelectValue(value))}>
                <SelectTrigger className="h-11 rounded-2xl border-border/70 bg-card/90">
                  <SelectValue placeholder={t('chatRuntimeTarget')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={EMPTY_VALUE}>{t('chatRuntimeAuto')}</SelectItem>
                  {enabledRuntimes.map((runtime) => (
                    <SelectItem key={runtime.id} value={runtime.id}>
                      {runtime.display_name || runtime.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-3">
              <label className="eyebrow-label text-muted-foreground">
                {t('defaultProvider')}
              </label>
              <Select value={toSelectValue(selectedProvider)} onValueChange={handleProviderChange}>
                <SelectTrigger className="h-11 rounded-2xl border-border/70 bg-card/90">
                  <SelectValue placeholder={t('defaultProvider')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={EMPTY_VALUE}>{t('chatRouteAuto')}</SelectItem>
                  {routeTargets.map((target) => (
                    <SelectItem key={target.name} value={target.name}>
                      {target.type === 'group'
                        ? `${target.name} (${t('chatRouteTargetGroup')})`
                        : target.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-3">
              <label className="eyebrow-label text-muted-foreground">
                {t('defaultModel')}
              </label>
              <Select value={selectedModelValue} onValueChange={handleModelChange}>
                <SelectTrigger className="h-11 rounded-2xl border-border/70 bg-card/90">
                  <SelectValue placeholder={t('defaultModel')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={EMPTY_VALUE}>{t('chatModelUnset')}</SelectItem>
                  {filteredModels.map((entry) => (
                    <SelectItem key={encodeModelValue(entry)} value={encodeModelValue(entry)}>
                      {selectedProvider ? entry.model : `${entry.model} (${entry.provider})`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-3">
              <label className="eyebrow-label text-muted-foreground">
                {t('customModel')}
              </label>
              <Input
                className="h-11 rounded-2xl border-border/70 bg-card/90"
                placeholder={t('chatCustomModelHint')}
                value={customModel}
                onChange={(event) => setCustomModel(event.target.value)}
              />
            </div>

            <div className="space-y-3">
              <label className="eyebrow-label text-muted-foreground">
                {t('chatSystemPrompts')}
              </label>
              <div className="space-y-3 rounded-[1.4rem] border border-border/70 bg-card/90 p-3">
                <p className="text-sm text-muted-foreground">{t('chatSystemPromptsHint')}</p>
                {systemPrompts.length === 0 ? (
                  <div className="rounded-2xl border border-dashed border-[hsl(var(--gray-200))] px-3 py-4 text-sm text-muted-foreground">
                    {t('chatPromptEmpty')}
                  </div>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {systemPrompts.map((prompt) => {
                      const selected = selectedSystemPromptIDs.includes(prompt.id);
                      return (
                        <button
                          key={prompt.id}
                          type="button"
                          onClick={() => handleTogglePrompt(prompt.id, 'system')}
                          className={cn(
                            'rounded-full border px-3 py-1.5 text-xs font-medium transition-colors',
                            selected
                              ? 'border-[hsl(var(--brand-300))] bg-[hsl(var(--brand-100))] text-[hsl(var(--brand-800))]'
                              : 'border-[hsl(var(--gray-200))] bg-white text-muted-foreground hover:border-[hsl(var(--gray-300))] hover:bg-[hsl(var(--gray-50))]',
                          )}
                        >
                          {prompt.name}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>

            <div className="space-y-3">
              <label className="eyebrow-label text-muted-foreground">
                {t('chatUserPrompts')}
              </label>
              <div className="space-y-3 rounded-[1.4rem] border border-border/70 bg-card/90 p-3">
                <p className="text-sm text-muted-foreground">{t('chatUserPromptsHint')}</p>
                {userPrompts.length === 0 ? (
                  <div className="rounded-2xl border border-dashed border-[hsl(var(--gray-200))] px-3 py-4 text-sm text-muted-foreground">
                    {t('chatPromptEmpty')}
                  </div>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {userPrompts.map((prompt) => {
                      const selected = selectedUserPromptIDs.includes(prompt.id);
                      return (
                        <button
                          key={prompt.id}
                          type="button"
                          onClick={() => handleTogglePrompt(prompt.id, 'user')}
                          className={cn(
                            'rounded-full border px-3 py-1.5 text-xs font-medium transition-colors',
                            selected
                              ? 'border-[hsl(var(--brand-300))] bg-[hsl(var(--brand-100))] text-[hsl(var(--brand-800))]'
                              : 'border-[hsl(var(--gray-200))] bg-card text-muted-foreground hover:border-[hsl(var(--gray-300))] hover:bg-[hsl(var(--gray-50))]',
                          )}
                        >
                          {prompt.name}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>

            <div className="space-y-3">
              <label className="eyebrow-label text-muted-foreground">
                {t('fallbackProviders')}
              </label>
              <div className="space-y-3 rounded-[1.4rem] border border-border/70 bg-card/90 p-3">
                <p className="text-sm text-muted-foreground">{t('chatFallbackSelectHint')}</p>
                {activeFallback.length > 0 ? (
                  <div className="flex flex-wrap gap-2">
                    {activeFallback.map((targetName, index) => (
                      <button
                        key={targetName}
                        type="button"
                        onClick={() => handleToggleFallbackTarget(targetName)}
                        className="inline-flex max-w-full items-center gap-2 rounded-full border border-[hsl(var(--brand-200))] bg-[hsl(var(--brand-50))] px-3 py-1.5 text-xs font-medium text-[hsl(var(--brand-800))]"
                      >
                        <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-card text-[10px] text-[hsl(var(--brand-700))]">
                          {index + 1}
                        </span>
                        <span className="break-all text-left">{targetName}</span>
                      </button>
                    ))}
                  </div>
                ) : (
                  <div className="rounded-2xl border border-dashed border-[hsl(var(--gray-200))] px-3 py-4 text-sm text-muted-foreground">
                    {t('chatFallbackEmpty')}
                  </div>
                )}
                <div className="flex flex-wrap gap-2">
                  {fallbackRouteTargets.map((target) => {
                    const selected = activeFallback.includes(target.name);
                    return (
                      <button
                        key={target.name}
                        type="button"
                        onClick={() => handleToggleFallbackTarget(target.name)}
                        className={cn(
                          'max-w-full rounded-full border px-3 py-1.5 text-left text-xs font-medium transition-colors',
                          selected
                            ? 'border-[hsl(var(--brand-300))] bg-[hsl(var(--brand-100))] text-[hsl(var(--brand-800))]'
                            : 'border-[hsl(var(--gray-200))] bg-card text-muted-foreground hover:border-[hsl(var(--gray-300))] hover:bg-[hsl(var(--gray-50))]',
                        )}
                      >
                        <span className="break-all">
                          {target.type === 'group'
                            ? `${target.name} (${t('chatRouteTargetGroup')})`
                            : target.name}
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>
            </div>

            <div className="flex flex-col gap-2 pt-2 sm:flex-row sm:flex-wrap">
              {connectionStatus !== 'connected' && (
                <Button variant="outline" className="h-11 rounded-full sm:min-w-[140px]" onClick={reconnect}>
                  <RefreshCw className="mr-2 h-4 w-4" />
                  {t('reconnect')}
                </Button>
              )}
              <Button variant="outline" className="h-11 rounded-full sm:min-w-[140px]" onClick={() => clearMessages(activeRuntimeID)}>
                <Trash2 className="mr-2 h-4 w-4" />
                {t('clearSession')}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="flex min-h-0 flex-col overflow-hidden border-border/70 bg-[linear-gradient(180deg,rgba(255,255,255,0.84),rgba(255,250,248,0.96))] shadow-[0_24px_80px_-40px_rgba(80,40,45,0.45)] backdrop-blur dark:bg-card/92">
          <CardHeader className="border-b border-[hsl(var(--gray-200))]/80 pb-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <div className="eyebrow-label text-muted-foreground">
                  {t('chatTranscriptTitle')}
                </div>
                <div className="mt-2 text-lg font-semibold text-[hsl(var(--gray-900))]">
                  {t('chatTranscriptSubtitle')}
                </div>
              </div>
              <div className="inline-flex max-w-full flex-wrap items-center gap-2 rounded-full bg-muted/70 px-3 py-1.5 text-xs text-muted-foreground sm:flex-nowrap">
                <Radio className="h-3.5 w-3.5 shrink-0" />
                <span className="min-w-0 max-w-full break-all sm:max-w-[12rem] sm:truncate">
                  {actualProvider || activeProvider || t('chatRouteAuto')}
                </span>
                <span className="shrink-0 text-[hsl(var(--gray-300))]">/</span>
                <span className="min-w-0 max-w-full break-all sm:max-w-[14rem] sm:truncate">
                  {actualModel || activeModel || t('chatModelUnset')}
                </span>
              </div>
              <div className={cn(
                'inline-flex max-w-full items-center gap-2 rounded-full px-3 py-1.5 text-xs font-medium',
                watchEnabled
                  ? 'bg-[hsl(var(--brand-100))] text-[hsl(var(--brand-800))]'
                  : 'bg-muted/70 text-muted-foreground',
              )}>
                <span className={cn('h-2.5 w-2.5 rounded-full', watchEnabled && watchRunning ? 'bg-emerald-500' : 'bg-slate-400')} />
                <span>{watchLabel}</span>
              </div>
            </div>
          </CardHeader>

          <CardContent className="flex min-h-0 flex-1 flex-col p-0">
            <ScrollArea className="min-h-0 flex-1 px-4 py-5 sm:px-6">
              {messages.length === 0 ? (
                <div className="flex h-full min-h-[320px] items-center justify-center">
                  <div className="max-w-md rounded-[2rem] border border-dashed border-[hsl(var(--brand-200))] bg-[linear-gradient(180deg,rgba(255,251,250,0.95),rgba(252,241,245,0.78))] p-8 text-center shadow-[0_20px_60px_-40px_rgba(198,104,140,0.45)] dark:bg-card/92">
                    <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-[hsl(var(--brand-100))] text-[hsl(var(--brand-700))]">
                      <Sparkles className="h-6 w-6" />
                    </div>
                    <div className="mt-4 text-lg font-semibold text-[hsl(var(--gray-900))]">
                      {t('chatEmptyHint')}
                    </div>
                    <div className="mt-2 text-sm leading-6 text-muted-foreground">
                      {t('chatEmptyDescription')}
                    </div>
                  </div>
                </div>
              ) : (
                <div className="space-y-4 pb-2">
                  {fileMentionFeedback && (
                    <div className="rounded-[1.4rem] border border-[hsl(var(--brand-200))] bg-[linear-gradient(180deg,rgba(255,252,250,0.92),rgba(252,241,245,0.8))] p-4 text-sm">
                      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                          <div className="eyebrow-label text-[hsl(var(--brand-700))]">
                            {t('chatFileMentionsTitle')}
                          </div>
                          <div className="mt-2 text-sm font-medium text-[hsl(var(--gray-900))]">
                            {t('chatFileMentionsSummary', String(fileMentionFeedback.count))}
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          <Button
                            type="button"
                            variant="ghost"
                            className="h-9 rounded-full px-3"
                            onClick={() => setShowFileMentionDetails((value) => !value)}
                          >
                            {showFileMentionDetails ? <EyeOff className="mr-2 h-4 w-4" /> : <Eye className="mr-2 h-4 w-4" />}
                            {showFileMentionDetails ? t('chatHideDetails') : t('chatShowDetails')}
                          </Button>
                          <Button
                            type="button"
                            variant="ghost"
                            className="h-9 rounded-full px-3"
                            onClick={clearFileMentionFeedback}
                          >
                            {t('dismiss')}
                          </Button>
                        </div>
                      </div>
                      {showFileMentionDetails && (
                        <div className="mt-3 space-y-3">
                          {fileMentionFeedback.paths.length > 0 && (
                            <div className="rounded-2xl border border-border/70 bg-card/80 p-3">
                              <div className="eyebrow-label mb-2 text-muted-foreground">
                                {t('chatFileMentionPaths')}
                              </div>
                              <div className="flex flex-wrap gap-2">
                                {fileMentionFeedback.paths.map((path) => (
                                  <span key={path} className="rounded-full border border-border/70 bg-card px-3 py-1.5 text-xs text-foreground">
                                    {path}
                                  </span>
                                ))}
                              </div>
                            </div>
                          )}
                          {fileMentionFeedback.warnings.length > 0 && (
                            <div className="rounded-2xl border border-amber-300/60 bg-amber-50/80 p-3 text-amber-900">
                              <div className="eyebrow-label mb-2">
                                {t('chatFileMentionWarnings')}
                              </div>
                              <div className="space-y-1 text-xs">
                                {fileMentionFeedback.warnings.map((warning) => (
                                  <div key={warning}>{warning}</div>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  )}
                  {messages.map((message, index) => (
                    <MessageBubble key={`${message.timestamp}-${index}`} message={message} />
                  ))}
                  {isAwaitingReply && (
                    <div className="flex justify-start">
                      <div className="rounded-full border border-[hsl(var(--brand-200))] bg-card/92 px-4 py-2 text-sm text-muted-foreground shadow-sm">
                        {t('chatWaitingReply')}
                      </div>
                    </div>
                  )}
                  <div ref={scrollEndRef} />
                </div>
              )}
            </ScrollArea>

            <div className="border-t border-[hsl(var(--gray-200))]/80 bg-card/88 p-4 sm:p-5">
              <div className="rounded-[1.6rem] border border-[hsl(var(--gray-200))] bg-[linear-gradient(180deg,rgba(255,255,255,0.96),rgba(249,244,241,0.98))] p-3 shadow-[0_18px_44px_-36px_rgba(50,32,20,0.45)] dark:bg-card/92">
                <textarea
                  rows={1}
                  className="min-h-[84px] w-full resize-none border-0 bg-transparent px-2 py-1 text-sm leading-6 text-foreground placeholder:text-muted-foreground focus:outline-none"
                  placeholder={t('chatPlaceholder')}
                  aria-label={t('chatPlaceholder')}
                  value={chatInput}
                  onChange={(event) => setChatInput(event.target.value)}
                  onKeyDown={handleInputKeyDown}
                  disabled={connectionStatus !== 'connected'}
                />
                <div className="mt-3 flex flex-col gap-3 border-t border-[hsl(var(--gray-200))]/80 px-2 pt-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="space-y-1 text-xs text-muted-foreground">
                    <div>{t('chatComposerHint')}</div>
                    {watchStatus?.last_command && (
                      <div>
                        {t('chatWatchHint', watchStatus.last_command)}
                      </div>
                    )}
                  </div>
                  <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:self-end">
                    <Button
                      type="button"
                      variant="outline"
                      className="h-11 rounded-full px-5"
                      onClick={handleUndo}
                    >
                      <RotateCcw className="mr-2 h-4 w-4" />
                      {t('chatUndo')}
                    </Button>
                    <Button
                      className="h-11 rounded-full px-5"
                      onClick={handleSend}
                      disabled={connectionStatus !== 'connected' || !chatInput.trim()}
                    >
                      <Send className="mr-2 h-4 w-4" />
                      {t('send')}
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
