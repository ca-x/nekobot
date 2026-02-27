import { useState, useRef, useEffect, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/client';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Send, Trash2, RefreshCw } from 'lucide-react';
import { useChat, type ChatMessage } from '@/hooks/useChat';

/* ---------- Types ---------- */

interface ProviderInfo {
  name: string;
  default_model?: string;
  models?: string[];
}

interface ConfigData {
  agents?: {
    defaults?: {
      provider?: string;
      model?: string;
      fallback?: string[];
    };
  };
}

interface ModelEntry {
  provider: string;
  model: string;
}

/* ---------- Helpers ---------- */

/** The radix Select uses "" as the value for the "no selection" placeholder,
 *  but it does not allow empty-string items. We use a sentinel instead. */
const EMPTY_VALUE = '__default__';

function toSelectValue(v: string): string {
  return v || EMPTY_VALUE;
}
function fromSelectValue(v: string): string {
  return v === EMPTY_VALUE ? '' : v;
}

function formatTime(ts: number): string {
  const d = new Date(ts);
  return d.toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

/* ---------- Data hooks ---------- */

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
): { models: ModelEntry[]; defaultProvider: string; defaultFallback: string[] } {
  const models: ModelEntry[] = [];
  const seen = new Set<string>();
  const defaults = config?.agents?.defaults;
  const defaultProvider = defaults?.provider || '';
  const defaultFallback = defaults?.fallback || [];

  const add = (provider: string, model: string) => {
    const m = model.trim();
    if (!m) return;
    const p = (provider || 'default').trim() || 'default';
    const key = `${p}::${m}`;
    if (seen.has(key)) return;
    seen.add(key);
    models.push({ provider: p, model: m });
  };

  // Add the config default model first
  add(defaultProvider, defaults?.model || '');

  // Then add models from each provider
  for (const p of providers) {
    if (p.default_model) add(p.name, p.default_model);
    if (Array.isArray(p.models)) {
      for (const m of p.models) add(p.name, m);
    }
  }

  return { models, defaultProvider, defaultFallback };
}

/* ---------- Message bubble component ---------- */

function MessageBubble({ message }: { message: ChatMessage }) {
  if (message.role === 'user') {
    return (
      <div className="flex justify-end mb-3">
        <div className="max-w-[75%]">
          <div className="rounded-2xl rounded-br-md bg-primary text-primary-foreground px-4 py-2.5 text-sm whitespace-pre-wrap break-words">
            {message.content}
          </div>
          <div className="text-[10px] text-muted-foreground mt-1 text-right">
            {formatTime(message.timestamp)}
          </div>
        </div>
      </div>
    );
  }

  if (message.role === 'assistant') {
    return (
      <div className="flex justify-start mb-3">
        <div className="max-w-[75%]">
          <Card className="px-4 py-2.5 text-sm whitespace-pre-wrap break-words">
            {message.content}
          </Card>
          <div className="text-[10px] text-muted-foreground mt-1">
            {formatTime(message.timestamp)}
          </div>
        </div>
      </div>
    );
  }

  // system or error
  return (
    <div className="flex justify-center mb-3">
      <div
        className={cn(
          'max-w-[85%] rounded-lg px-3 py-1.5 text-xs text-center',
          message.role === 'error'
            ? 'bg-destructive/10 text-destructive'
            : 'bg-muted text-muted-foreground',
        )}
      >
        {message.content}
        <span className="ml-2 opacity-60">{formatTime(message.timestamp)}</span>
      </div>
    </div>
  );
}

/* ---------- Connection status indicator ---------- */

function StatusDot({ status }: { status: string }) {
  const dotClass = cn(
    'h-2.5 w-2.5 rounded-full inline-block',
    status === 'connected' && 'bg-green-500',
    status === 'connecting' && 'bg-yellow-500 animate-pulse',
    status === 'disconnected' && 'bg-red-500',
  );

  const label =
    status === 'connected'
      ? t('wsConnected')
      : status === 'connecting'
        ? 'Connecting...'
        : t('wsDisconnected');

  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
      <span className={dotClass} />
      {label}
    </span>
  );
}

/* ---------- ChatPage ---------- */

export default function ChatPage() {
  const { data: providers = [] } = useProviders();
  const { data: config } = useAppConfig();
  const { messages, sendMessage, clearMessages, connectionStatus, reconnect } = useChat();

  const { models, defaultProvider, defaultFallback } = buildModelList(providers, config);

  // Chat settings state
  const [selectedProvider, setSelectedProvider] = useState<string>('');
  const [selectedModel, setSelectedModel] = useState<string>('');
  const [customModel, setCustomModel] = useState<string>('');
  const [fallbackInput, setFallbackInput] = useState<string>('');
  const [chatInput, setChatInput] = useState('');

  // Scroll ref
  const scrollEndRef = useRef<HTMLDivElement>(null);

  // Apply defaults from config once loaded
  const defaultsApplied = useRef(false);
  useEffect(() => {
    if (defaultsApplied.current) return;
    if (!config) return;
    defaultsApplied.current = true;
    if (defaultProvider) setSelectedProvider(defaultProvider);
    if (defaultFallback.length > 0) setFallbackInput(defaultFallback.join(', '));
    const defaultModel = config?.agents?.defaults?.model || '';
    if (defaultModel) {
      setSelectedModel(defaultModel);
      setCustomModel(defaultModel);
    }
  }, [config, defaultProvider, defaultFallback]);

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    scrollEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // Filtered models based on selected provider
  const filteredModels = selectedProvider
    ? models.filter((m) => m.provider === selectedProvider)
    : models;

  const handleProviderChange = useCallback(
    (value: string) => {
      const provider = fromSelectValue(value);
      setSelectedProvider(provider);
      // Re-filter models; if current selection not in new list, reset
      const available = provider
        ? models.filter((m) => m.provider === provider)
        : models;
      if (selectedModel && !available.some((m) => m.model === selectedModel)) {
        const first = available.length > 0 ? available[0].model : '';
        setSelectedModel(first);
        setCustomModel(first);
      }
    },
    [models, selectedModel],
  );

  const handleModelChange = useCallback(
    (value: string) => {
      const model = fromSelectValue(value);
      setSelectedModel(model);
      setCustomModel(model);
      // Auto-set provider from model entry if not manually chosen
      if (model) {
        const entry = models.find((m) => m.model === model);
        if (entry && entry.provider !== 'default' && !selectedProvider) {
          setSelectedProvider(entry.provider);
        }
      }
    },
    [models, selectedProvider],
  );

  const handleSend = useCallback(() => {
    const text = chatInput.trim();
    if (!text) return;
    if (connectionStatus !== 'connected') return;

    const model = customModel.trim() || selectedModel;
    const provider = selectedProvider;
    const fallback = fallbackInput
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);

    sendMessage(text, { provider, model, fallbackProviders: fallback });
    setChatInput('');
  }, [chatInput, connectionStatus, customModel, selectedModel, selectedProvider, fallbackInput, sendMessage]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend],
  );

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)]">
      <Header title={t('tabChat')} />

      {/* Top bar: provider, model, fallback, status */}
      <div className="flex flex-wrap items-end gap-3 mb-3">
        {/* Provider selector */}
        <div className="flex flex-col gap-1 min-w-[160px]">
          <label className="text-xs font-medium text-muted-foreground">
            {t('defaultProvider')}
          </label>
          <Select
            value={toSelectValue(selectedProvider)}
            onValueChange={handleProviderChange}
          >
            <SelectTrigger className="h-9 text-sm">
              <SelectValue placeholder={t('defaultProvider')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={EMPTY_VALUE}>{t('defaultProvider')}</SelectItem>
              {providers.map((p) => (
                <SelectItem key={p.name} value={p.name}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {/* Model selector */}
        <div className="flex flex-col gap-1 min-w-[200px]">
          <label className="text-xs font-medium text-muted-foreground">
            {t('defaultModel')}
          </label>
          <Select
            value={toSelectValue(selectedModel)}
            onValueChange={handleModelChange}
          >
            <SelectTrigger className="h-9 text-sm">
              <SelectValue placeholder={t('defaultModel')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={EMPTY_VALUE}>{t('defaultModel')}</SelectItem>
              {filteredModels.map((m) => (
                <SelectItem
                  key={`${m.provider}::${m.model}`}
                  value={m.model}
                >
                  {selectedProvider ? m.model : `${m.model} (${m.provider})`}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {/* Custom model override */}
        <div className="flex flex-col gap-1 min-w-[180px]">
          <label className="text-xs font-medium text-muted-foreground">
            {t('customModel')}
          </label>
          <Input
            className="h-9 text-sm"
            placeholder={t('customModel')}
            value={customModel}
            onChange={(e) => setCustomModel(e.target.value)}
          />
        </div>

        {/* Fallback providers */}
        <div className="flex flex-col gap-1 min-w-[200px] flex-1">
          <label className="text-xs font-medium text-muted-foreground">
            {t('fallbackProviders')}
          </label>
          <Input
            className="h-9 text-sm"
            placeholder={t('fallbackProviders')}
            value={fallbackInput}
            onChange={(e) => setFallbackInput(e.target.value)}
          />
        </div>

        {/* Status + action buttons */}
        <div className="flex items-center gap-2 pb-0.5">
          <StatusDot status={connectionStatus} />
          {connectionStatus === 'disconnected' && (
            <Button
              variant="outline"
              size="sm"
              className="h-9 gap-1.5"
              onClick={reconnect}
            >
              <RefreshCw className="h-3.5 w-3.5" />
              {t('reconnect')}
            </Button>
          )}
          <Button
            variant="outline"
            size="sm"
            className="h-9 gap-1.5"
            onClick={clearMessages}
          >
            <Trash2 className="h-3.5 w-3.5" />
            {t('clearSession')}
          </Button>
        </div>
      </div>

      {/* Chat log */}
      <Card className="flex-1 flex flex-col min-h-0 overflow-hidden">
        <ScrollArea className="flex-1 p-4">
          {messages.length === 0 ? (
            <div className="flex items-center justify-center h-full min-h-[200px] text-sm text-muted-foreground">
              {t('chatEmptyHint')}
            </div>
          ) : (
            <div className="flex flex-col">
              {messages.map((msg, idx) => (
                <MessageBubble key={idx} message={msg} />
              ))}
              <div ref={scrollEndRef} />
            </div>
          )}
        </ScrollArea>

        {/* Input area */}
        <div className="border-t p-3 flex gap-2 items-end">
          <textarea
            className="flex-1 resize-none rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 min-h-[40px] max-h-[160px]"
            rows={1}
            placeholder={t('chatPlaceholder')}
            value={chatInput}
            onChange={(e) => setChatInput(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={connectionStatus !== 'connected'}
          />
          <Button
            size="icon"
            className="h-10 w-10 shrink-0"
            onClick={handleSend}
            disabled={connectionStatus !== 'connected' || !chatInput.trim()}
          >
            <Send className="h-4 w-4" />
          </Button>
        </div>
      </Card>
    </div>
  );
}
