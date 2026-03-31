import { useState, useRef, useCallback, useEffect } from 'react';
import { getToken } from '@/api/client';

export interface ChatMessage {
  role: 'user' | 'assistant' | 'system' | 'error';
  content: string;
  timestamp: number;
}

export interface FileMentionFeedback {
  count: number;
  paths: string[];
  warnings: string[];
}

export type ConnectionStatus = 'connected' | 'disconnected' | 'connecting';

export interface ChatRouteSettings {
  provider: string;
  model: string;
  fallback: string[];
}

export interface ChatRouteResult {
  requested_provider: string;
  requested_model: string;
  requested_fallback: string[];
  resolved_order: string[];
  actual_provider: string;
  actual_model: string;
  runtime_id?: string;
}

interface SendOptions {
  provider: string;
  model: string;
  fallbackProviders: string[];
  systemPromptIDs?: string[];
  userPromptIDs?: string[];
  runtimeID?: string;
}

interface UseChatReturn {
  messages: ChatMessage[];
  sendMessage: (text: string, options: SendOptions) => void;
  clearMessages: (runtimeID?: string) => void;
  replaceMessages: (messages: ChatMessage[]) => void;
  connectionStatus: ConnectionStatus;
  reconnect: () => void;
  routeSettings: ChatRouteSettings;
  routeResult: ChatRouteResult | null;
  isAwaitingReply: boolean;
  fileMentionFeedback: FileMentionFeedback | null;
  clearFileMentionFeedback: () => void;
}

export function useChat(): UseChatReturn {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('disconnected');
  const [routeSettings, setRouteSettings] = useState<ChatRouteSettings>({
    provider: '',
    model: '',
    fallback: [],
  });
  const [routeResult, setRouteResult] = useState<ChatRouteResult | null>(null);
  const [isAwaitingReply, setIsAwaitingReply] = useState(false);
  const [fileMentionFeedback, setFileMentionFeedback] = useState<FileMentionFeedback | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const cleanup = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    if (wsRef.current) {
      try {
        wsRef.current.close();
      } catch {
        // ignore close errors
      }
      wsRef.current = null;
    }
  }, []);

  const connect = useCallback(() => {
    const token = getToken();
    if (!token) {
      setConnectionStatus('disconnected');
      return;
    }

    cleanup();
    setConnectionStatus('connecting');

    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${window.location.host}/api/chat/ws?token=${encodeURIComponent(token)}`;

    let ws: WebSocket;
    try {
      ws = new WebSocket(url);
    } catch {
      setConnectionStatus('disconnected');
      return;
    }
    wsRef.current = ws;

    ws.onopen = () => {
      setConnectionStatus('connected');
    };

    ws.onclose = () => {
      setConnectionStatus('disconnected');
      setIsAwaitingReply(false);
      wsRef.current = null;
    };

    ws.onerror = () => {
      setConnectionStatus('disconnected');
      setIsAwaitingReply(false);
    };

    ws.onmessage = (ev: MessageEvent) => {
      let msg: {
        type?: string;
        content?: string;
        route?: ChatRouteResult;
        meta?: { kind?: string; data?: FileMentionFeedback };
      };
      try {
        msg = JSON.parse(ev.data);
      } catch {
        return;
      }

      const now = Date.now();

      if (msg.type === 'routing') {
        try {
          const parsed = JSON.parse(msg.content || '{}') as Partial<ChatRouteSettings>;
          setRouteSettings({
            provider: parsed.provider?.trim() || '',
            model: parsed.model?.trim() || '',
            fallback: Array.isArray(parsed.fallback)
              ? parsed.fallback.map((item) => String(item).trim()).filter(Boolean)
              : [],
          });
        } catch {
          // ignore malformed routing snapshots
        }
        return;
      }

      if (msg.type === 'message') {
        setIsAwaitingReply(false);
        setMessages((prev) => [
          ...prev,
          { role: 'assistant', content: msg.content || '', timestamp: now },
        ]);
      } else if (msg.type === 'error') {
        setIsAwaitingReply(false);
        setMessages((prev) => [
          ...prev,
          { role: 'error', content: msg.content || 'Unknown error', timestamp: now },
        ]);
      } else if (msg.type === 'route_result' && msg.route) {
        setRouteResult(msg.route);
      } else if (msg.type === 'system' && msg.meta?.kind === 'file_mentions' && msg.meta.data) {
        setFileMentionFeedback({
          count: Number(msg.meta.data.count || 0),
          paths: Array.isArray(msg.meta.data.paths) ? msg.meta.data.paths : [],
          warnings: Array.isArray(msg.meta.data.warnings) ? msg.meta.data.warnings : [],
        });
        setMessages((prev) => [
          ...prev,
          {
            role: 'system',
            content: msg.content || 'file mention feedback',
            timestamp: now,
          },
        ]);
      } else {
        setMessages((prev) => [
          ...prev,
          {
            role: 'system',
            content: msg.content || msg.type || 'event',
            timestamp: now,
          },
        ]);
      }
    };
  }, [cleanup]);

  const reconnect = useCallback(() => {
    connect();
  }, [connect]);

  const sendMessage = useCallback((text: string, options: SendOptions) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    if (!text.trim()) return;

    ws.send(
      JSON.stringify({
        type: 'message',
        content: text,
        model: options.model,
        provider: options.provider,
        fallback: options.fallbackProviders,
        system_prompt_ids: options.systemPromptIDs ?? [],
        user_prompt_ids: options.userPromptIDs ?? [],
        runtime_id: options.runtimeID ?? '',
      }),
    );
    setRouteSettings({
      provider: options.provider,
      model: options.model,
      fallback: options.fallbackProviders,
    });
    setRouteResult(null);
    setFileMentionFeedback(null);
    setIsAwaitingReply(true);

    setMessages((prev) => [
      ...prev,
      { role: 'user', content: text, timestamp: Date.now() },
    ]);
  }, []);

  const clearMessages = useCallback((runtimeID?: string) => {
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'clear', runtime_id: runtimeID ?? '' }));
    }
    setIsAwaitingReply(false);
    setRouteResult(null);
    setFileMentionFeedback(null);
    setMessages([]);
  }, []);

  const replaceMessages = useCallback((nextMessages: ChatMessage[]) => {
    setIsAwaitingReply(false);
    setMessages(nextMessages);
  }, []);

  const clearFileMentionFeedback = useCallback(() => {
    setFileMentionFeedback(null);
  }, []);

  // Connect on mount, cleanup on unmount
  useEffect(() => {
    connect();
    return cleanup;
  }, [connect, cleanup]);

  return {
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
  };
}
