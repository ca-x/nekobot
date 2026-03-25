import { useEffect, useMemo, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Send, Sparkles, RefreshCw, Trash2, Radio, Wand2 } from 'lucide-react';

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
          <div className="rounded-[1.4rem] rounded-br-md bg-[hsl(var(--gray-900))] px-4 py-3 text-sm leading-6 text-white shadow-[0_16px_40px_-24px_rgba(20,15,10,0.75)]">
            {message.content}
          </div>
          <div className="text-right text-[11px] uppercase tracking-[0.18em] text-muted-foreground/80">
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
          <div className="rounded-[1.4rem] rounded-bl-md border border-[hsl(var(--brand-200))] bg-white/90 px-4 py-3 text-sm leading-6 text-foreground shadow-[0_18px_42px_-30px_rgba(120,55,75,0.35)] backdrop-blur">
            {message.content}
          </div>
          <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground/80">
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
    <div className="inline-flex items-center gap-2 rounded-full border border-white/70 bg-white/70 px-3 py-1.5 text-xs text-muted-foreground shadow-sm backdrop-blur">
      <span className={cn('h-2.5 w-2.5 rounded-full', colorClass)} />
      <span>{label}</span>
      {isAwaitingReply && <span className="text-[hsl(var(--brand-700))]">{t('chatWaitingReply')}</span>}
    </div>
  );
}

export default function ChatPage() {
  const { data: providers = [] } = useProviders();
  const { data: config } = useAppConfig();
  const {
    messages,
    sendMessage,
    clearMessages,
    connectionStatus,
    reconnect,
    routeSettings,
    isAwaitingReply,
  } = useChat();

  const { models, defaultProvider, defaultModel, defaultFallback, routeTargets } = buildModelList(providers, config);
  const [selectedProvider, setSelectedProvider] = useState('');
  const [selectedModel, setSelectedModel] = useState('');
  const [customModel, setCustomModel] = useState('');
  const [selectedFallbackTargets, setSelectedFallbackTargets] = useState<string[]>([]);
  const [chatInput, setChatInput] = useState('');
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
  const activeFallback = selectedFallbackTargets.filter((target) => target.trim().length > 0);
  const selectedModelEntry = findModelEntry(filteredModels, selectedProvider, selectedModel);
  const selectedModelValue = selectedModelEntry ? encodeModelValue(selectedModelEntry) : EMPTY_VALUE;
  const fallbackRouteTargets = routeTargets.filter((target) => target.name !== activeProvider);

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

  function handleSend() {
    const content = chatInput.trim();
    if (!content || connectionStatus !== 'connected') {
      return;
    }

    sendMessage(content, {
      provider: activeProvider,
      model: activeModel,
      fallbackProviders: activeFallback,
    });
    setChatInput('');
  }

  function handleInputKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      handleSend();
    }
  }

  return (
    <div className="flex h-[calc(100vh-4rem)] flex-col overflow-hidden">
      <Header title={t('tabChat')} />

      <div className="relative flex min-h-0 flex-1 flex-col gap-4 lg:grid lg:grid-cols-[320px_minmax(0,1fr)]">
        <div className="absolute inset-x-0 top-0 -z-10 h-48 rounded-[2rem] bg-[radial-gradient(circle_at_top_left,rgba(198,104,140,0.22),transparent_48%),radial-gradient(circle_at_top_right,rgba(229,183,107,0.22),transparent_42%),linear-gradient(180deg,rgba(255,255,255,0.92),rgba(255,247,243,0.55))]" />

        <Card className="overflow-hidden border-white/70 bg-white/72 shadow-[0_20px_60px_-36px_rgba(120,55,75,0.45)] backdrop-blur xl:sticky xl:top-0 xl:h-fit">
          <CardHeader className="space-y-4 border-b border-[hsl(var(--gray-200))]/80 bg-[linear-gradient(135deg,rgba(255,248,246,0.96),rgba(252,239,244,0.9))]">
            <div className="flex items-start justify-between gap-3">
              <div className="space-y-2">
                <div className="inline-flex items-center gap-2 rounded-full bg-[hsl(var(--brand-50))] px-3 py-1 text-[11px] font-medium uppercase tracking-[0.22em] text-[hsl(var(--brand-700))]">
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
              <div className="rounded-2xl border border-white/80 bg-white/75 p-3">
                <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                  {t('chatMetricMessages')}
                </div>
                <div className="mt-2 text-2xl font-semibold text-[hsl(var(--gray-900))]">
                  {messageCount}
                </div>
              </div>
              <div className="rounded-2xl border border-white/80 bg-white/75 p-3">
                <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                  {t('chatMetricModel')}
                </div>
                <div className="mt-2 truncate text-sm font-semibold text-[hsl(var(--gray-900))]">
                  {activeModel || 'auto'}
                </div>
              </div>
            </div>
          </CardHeader>

          <CardContent className="space-y-4 p-4">
            <div className="rounded-[1.5rem] border border-[hsl(var(--gray-200))] bg-[linear-gradient(180deg,rgba(255,255,255,0.85),rgba(249,244,241,0.96))] p-4">
              <div className="mb-3 flex items-center gap-2 text-xs uppercase tracking-[0.18em] text-muted-foreground">
                <Sparkles className="h-3.5 w-3.5" />
                {t('chatActiveRoute')}
              </div>
              <div className="flex flex-wrap gap-2">
                <span className="rounded-full bg-[hsl(var(--gray-900))] px-3 py-1.5 text-xs font-medium text-white">
                  {activeProvider || t('chatRouteAuto')}
                </span>
                <span className="rounded-full bg-[hsl(var(--brand-100))] px-3 py-1.5 text-xs font-medium text-[hsl(var(--brand-800))]">
                  {activeModel || t('chatModelUnset')}
                </span>
                <span className="rounded-full border border-[hsl(var(--gray-200))] bg-white px-3 py-1.5 text-xs text-muted-foreground">
                  {activeFallback.length > 0
                    ? `${t('fallbackProviders')}: ${activeFallback.join(' -> ')}`
                    : t('chatNoFallback')}
                </span>
              </div>
            </div>

            <div className="space-y-3">
              <label className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t('defaultProvider')}
              </label>
              <Select value={toSelectValue(selectedProvider)} onValueChange={handleProviderChange}>
                <SelectTrigger className="h-11 rounded-2xl border-white bg-white/80">
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
              <label className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t('defaultModel')}
              </label>
              <Select value={selectedModelValue} onValueChange={handleModelChange}>
                <SelectTrigger className="h-11 rounded-2xl border-white bg-white/80">
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
              <label className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t('customModel')}
              </label>
              <Input
                className="h-11 rounded-2xl border-white bg-white/80"
                placeholder={t('chatCustomModelHint')}
                value={customModel}
                onChange={(event) => setCustomModel(event.target.value)}
              />
            </div>

            <div className="space-y-3">
              <label className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t('fallbackProviders')}
              </label>
              <div className="space-y-3 rounded-[1.4rem] border border-white bg-white/80 p-3">
                <p className="text-sm text-muted-foreground">{t('chatFallbackSelectHint')}</p>
                {activeFallback.length > 0 ? (
                  <div className="flex flex-wrap gap-2">
                    {activeFallback.map((targetName, index) => (
                      <button
                        key={targetName}
                        type="button"
                        onClick={() => handleToggleFallbackTarget(targetName)}
                        className="inline-flex items-center gap-2 rounded-full border border-[hsl(var(--brand-200))] bg-[hsl(var(--brand-50))] px-3 py-1.5 text-xs font-medium text-[hsl(var(--brand-800))]"
                      >
                        <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-white text-[10px] text-[hsl(var(--brand-700))]">
                          {index + 1}
                        </span>
                        {targetName}
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
                          'rounded-full border px-3 py-1.5 text-xs font-medium transition-colors',
                          selected
                            ? 'border-[hsl(var(--brand-300))] bg-[hsl(var(--brand-100))] text-[hsl(var(--brand-800))]'
                            : 'border-[hsl(var(--gray-200))] bg-white text-muted-foreground hover:border-[hsl(var(--gray-300))] hover:bg-[hsl(var(--gray-50))]',
                        )}
                      >
                        {target.type === 'group'
                          ? `${target.name} (${t('chatRouteTargetGroup')})`
                          : target.name}
                      </button>
                    );
                  })}
                </div>
              </div>
            </div>

            <div className="flex flex-wrap gap-2 pt-2">
              {connectionStatus !== 'connected' && (
                <Button variant="outline" className="rounded-full" onClick={reconnect}>
                  <RefreshCw className="mr-2 h-4 w-4" />
                  {t('reconnect')}
                </Button>
              )}
              <Button variant="outline" className="rounded-full" onClick={clearMessages}>
                <Trash2 className="mr-2 h-4 w-4" />
                {t('clearSession')}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="flex min-h-0 flex-col overflow-hidden border-white/70 bg-[linear-gradient(180deg,rgba(255,255,255,0.84),rgba(255,250,248,0.96))] shadow-[0_24px_80px_-40px_rgba(80,40,45,0.45)] backdrop-blur">
          <CardHeader className="border-b border-[hsl(var(--gray-200))]/80 pb-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="text-[11px] uppercase tracking-[0.22em] text-muted-foreground">
                  {t('chatTranscriptTitle')}
                </div>
                <div className="mt-2 text-lg font-semibold text-[hsl(var(--gray-900))]">
                  {t('chatTranscriptSubtitle')}
                </div>
              </div>
              <div className="inline-flex items-center gap-2 rounded-full bg-[hsl(var(--gray-100))] px-3 py-1.5 text-xs text-muted-foreground">
                <Radio className="h-3.5 w-3.5" />
                {activeProvider || t('chatRouteAuto')}
                <span className="text-[hsl(var(--gray-300))]">/</span>
                {activeModel || t('chatModelUnset')}
              </div>
            </div>
          </CardHeader>

          <CardContent className="flex min-h-0 flex-1 flex-col p-0">
            <ScrollArea className="min-h-0 flex-1 px-4 py-5 sm:px-6">
              {messages.length === 0 ? (
                <div className="flex h-full min-h-[320px] items-center justify-center">
                  <div className="max-w-md rounded-[2rem] border border-dashed border-[hsl(var(--brand-200))] bg-[linear-gradient(180deg,rgba(255,251,250,0.95),rgba(252,241,245,0.78))] p-8 text-center shadow-[0_20px_60px_-40px_rgba(198,104,140,0.45)]">
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
                  {messages.map((message, index) => (
                    <MessageBubble key={`${message.timestamp}-${index}`} message={message} />
                  ))}
                  {isAwaitingReply && (
                    <div className="flex justify-start">
                      <div className="rounded-full border border-[hsl(var(--brand-200))] bg-white/90 px-4 py-2 text-sm text-muted-foreground shadow-sm">
                        {t('chatWaitingReply')}
                      </div>
                    </div>
                  )}
                  <div ref={scrollEndRef} />
                </div>
              )}
            </ScrollArea>

            <div className="border-t border-[hsl(var(--gray-200))]/80 bg-white/85 p-4 sm:p-5">
              <div className="rounded-[1.6rem] border border-[hsl(var(--gray-200))] bg-[linear-gradient(180deg,rgba(255,255,255,0.96),rgba(249,244,241,0.98))] p-3 shadow-[0_18px_44px_-36px_rgba(50,32,20,0.45)]">
                <textarea
                  rows={1}
                  className="min-h-[84px] w-full resize-none border-0 bg-transparent px-2 py-1 text-sm leading-6 text-foreground placeholder:text-muted-foreground focus:outline-none"
                  placeholder={t('chatPlaceholder')}
                  value={chatInput}
                  onChange={(event) => setChatInput(event.target.value)}
                  onKeyDown={handleInputKeyDown}
                  disabled={connectionStatus !== 'connected'}
                />
                <div className="mt-3 flex flex-wrap items-center justify-between gap-3 border-t border-[hsl(var(--gray-200))]/80 px-2 pt-3">
                  <div className="text-xs text-muted-foreground">
                    {t('chatComposerHint')}
                  </div>
                  <Button
                    className="rounded-full px-5"
                    onClick={handleSend}
                    disabled={connectionStatus !== 'connected' || !chatInput.trim()}
                  >
                    <Send className="mr-2 h-4 w-4" />
                    {t('send')}
                  </Button>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
