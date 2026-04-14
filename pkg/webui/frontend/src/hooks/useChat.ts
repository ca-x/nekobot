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
  preflight?: {
    action?: 'proceed' | 'consider_compaction' | 'compact_before_run';
    applied?: boolean;
    budget_status?: 'ok' | 'warning' | 'critical';
    budget_reasons?: string[];
    compaction?: {
      recommended?: boolean;
      strategy?: string;
      reasons?: string[];
      estimated_chars_saved?: number;
    };
  };
  context_budget_status?: 'ok' | 'warning' | 'critical';
  context_budget_reasons?: string[];
  compaction_recommended?: boolean;
  compaction_strategy?: string;
  runtime_id?: string;
}

interface SendOptions {
  sessionKey: string;
  provider: string;
  model: string;
  fallbackProviders: string[];
  systemPromptIDs?: string[];
  userPromptIDs?: string[];
  runtimeID?: string;
}

interface UseChatReturn {
  messages: ChatMessage[];
  activeSessionKey: string;
  setActiveSessionKey: (sessionKey: string) => void;
  sendMessage: (text: string, options: SendOptions) => void;
  clearMessages: (sessionKey: string, runtimeID?: string) => void;
  replaceMessages: (sessionKey: string, messages: ChatMessage[]) => void;
  connectionStatus: ConnectionStatus;
  reconnect: () => void;
  routeSettings: ChatRouteSettings;
  routeResult: ChatRouteResult | null;
  isAwaitingReply: boolean;
  fileMentionFeedback: FileMentionFeedback | null;
  clearFileMentionFeedback: () => void;
}

export function useChat(): UseChatReturn {
  const [activeSessionKey, setActiveSessionKey] = useState('webui-chat');
  const [messagesBySession, setMessagesBySession] = useState<Record<string, ChatMessage[]>>({});
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('disconnected');
  const [routeSettings, setRouteSettings] = useState<ChatRouteSettings>({
    provider: '',
    model: '',
    fallback: [],
  });
  const [routeResultsBySession, setRouteResultsBySession] = useState<Record<string, ChatRouteResult | null>>({});
  const [awaitingReplyBySession, setAwaitingReplyBySession] = useState<Record<string, boolean>>({});
  const [fileMentionFeedbackBySession, setFileMentionFeedbackBySession] = useState<Record<string, FileMentionFeedback | null>>({});
  const wsRef = useRef<WebSocket | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingSessionKeyRef = useRef<string | null>(null);
  const activeSessionKeyRef = useRef(activeSessionKey);

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
    if (eventSourceRef.current) {
      try {
        eventSourceRef.current.close();
      } catch {
        // ignore close errors
      }
      eventSourceRef.current = null;
    }
  }, []);

  useEffect(() => {
    activeSessionKeyRef.current = activeSessionKey;
  }, [activeSessionKey]);

  useEffect(() => {
    const token = getToken();
    const runtimeSessionKey = activeSessionKey.trim();
    if (!token || !runtimeSessionKey || runtimeSessionKey === 'webui-chat') {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      return;
    }

    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }

    const url = `/api/chat/events?token=${encodeURIComponent(token)}&session_id=${encodeURIComponent(runtimeSessionKey)}`;
    const source = new EventSource(url);
    eventSourceRef.current = source;

    source.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data) as {
          session_id?: string;
          role?: ChatMessage['role'];
          content?: string;
          timestamp?: number;
        };
        const targetSessionKey = msg.session_id?.trim() || runtimeSessionKey;
        setMessagesBySession((prev) => ({
          ...prev,
          [targetSessionKey]: [
            ...(prev[targetSessionKey] ?? []),
            {
              role: (msg.role as ChatMessage['role']) || 'system',
              content: msg.content || '',
              timestamp: msg.timestamp ? msg.timestamp * 1000 : Date.now(),
            },
          ],
        }));
        setAwaitingReplyBySession((prev) => ({ ...prev, [targetSessionKey]: false }));
      } catch {
        // ignore malformed event payloads
      }
    };

    source.onerror = () => {
      source.close();
      if (eventSourceRef.current === source) {
        eventSourceRef.current = null;
      }
    };

    return () => {
      source.close();
      if (eventSourceRef.current === source) {
        eventSourceRef.current = null;
      }
    };
  }, [activeSessionKey]);

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
      setAwaitingReplyBySession((prev) => ({
        ...prev,
        [pendingSessionKeyRef.current ?? activeSessionKeyRef.current]: false,
      }));
      wsRef.current = null;
    };

    ws.onerror = () => {
      setConnectionStatus('disconnected');
      setAwaitingReplyBySession((prev) => ({
        ...prev,
        [pendingSessionKeyRef.current ?? activeSessionKeyRef.current]: false,
      }));
    };

    ws.onmessage = (ev: MessageEvent) => {
      let msg: {
        type?: string;
        content?: string;
        session_id?: string;
        route?: ChatRouteResult;
        meta?: { kind?: string; data?: FileMentionFeedback };
      };
      try {
        msg = JSON.parse(ev.data);
      } catch {
        return;
      }

      const now = Date.now();
      const explicitSessionKey = msg.session_id?.trim() || '';
      const targetSessionKey = explicitSessionKey || pendingSessionKeyRef.current || activeSessionKeyRef.current;

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
        if (pendingSessionKeyRef.current === targetSessionKey) {
          pendingSessionKeyRef.current = null;
        }
        setAwaitingReplyBySession((prev) => ({ ...prev, [targetSessionKey]: false }));
        setMessagesBySession((prev) => ({
          ...prev,
          [targetSessionKey]: [
            ...(prev[targetSessionKey] ?? []),
            { role: 'assistant', content: msg.content || '', timestamp: now },
          ],
        }));
      } else if (msg.type === 'error') {
        if (pendingSessionKeyRef.current === targetSessionKey) {
          pendingSessionKeyRef.current = null;
        }
        setAwaitingReplyBySession((prev) => ({ ...prev, [targetSessionKey]: false }));
        setMessagesBySession((prev) => ({
          ...prev,
          [targetSessionKey]: [
            ...(prev[targetSessionKey] ?? []),
            { role: 'error', content: msg.content || 'Unknown error', timestamp: now },
          ],
        }));
      } else if (msg.type === 'route_result' && msg.route) {
        if (pendingSessionKeyRef.current === targetSessionKey) {
          pendingSessionKeyRef.current = null;
        }
        setRouteResultsBySession((prev) => ({ ...prev, [targetSessionKey]: msg.route ?? null }));
      } else if (msg.type === 'system' && msg.meta?.kind === 'file_mentions' && msg.meta.data) {
        const feedback = msg.meta.data;
        setFileMentionFeedbackBySession((prev) => ({
          ...prev,
          [targetSessionKey]: {
            count: Number(feedback.count || 0),
            paths: Array.isArray(feedback.paths) ? feedback.paths : [],
            warnings: Array.isArray(feedback.warnings) ? feedback.warnings : [],
          },
        }));
        setMessagesBySession((prev) => ({
          ...prev,
          [targetSessionKey]: [
            ...(prev[targetSessionKey] ?? []),
            {
              role: 'system',
              content: msg.content || 'file mention feedback',
              timestamp: now,
            },
          ],
        }));
      } else {
        if (msg.type === 'system' && msg.content === 'Session cleared' && pendingSessionKeyRef.current === targetSessionKey) {
          pendingSessionKeyRef.current = null;
        }
        setMessagesBySession((prev) => ({
          ...prev,
          [targetSessionKey]: [
            ...(prev[targetSessionKey] ?? []),
            {
              role: 'system',
              content: msg.content || msg.type || 'event',
              timestamp: now,
            },
          ],
        }));
      }
    };
  }, [cleanup]);

  const reconnect = useCallback(() => {
    connect();
  }, [connect]);

  const setMessagesForSession = useCallback((sessionKey: string, messages: ChatMessage[]) => {
    setMessagesBySession((prev) => {
      const existing = prev[sessionKey];
      if (
        existing &&
        existing.length === messages.length &&
        existing.every((message, index) => {
          const next = messages[index];
          return (
            message.role === next.role &&
            message.content === next.content &&
            message.timestamp === next.timestamp
          );
        })
      ) {
        return prev;
      }

      return {
        ...prev,
        [sessionKey]: messages,
      };
    });
  }, []);

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
    setRouteResultsBySession((prev) => ({ ...prev, [options.sessionKey]: null }));
    setFileMentionFeedbackBySession((prev) => ({ ...prev, [options.sessionKey]: null }));
    setAwaitingReplyBySession((prev) => ({ ...prev, [options.sessionKey]: true }));
    setActiveSessionKey(options.sessionKey);
    pendingSessionKeyRef.current = options.sessionKey;

    setMessagesBySession((prev) => ({
      ...prev,
      [options.sessionKey]: [
        ...(prev[options.sessionKey] ?? []),
        { role: 'user', content: text, timestamp: Date.now() },
      ],
    }));
  }, []);

  const clearMessages = useCallback((sessionKey: string, runtimeID?: string) => {
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'clear', runtime_id: runtimeID ?? '' }));
    }
    setAwaitingReplyBySession((prev) => ({ ...prev, [sessionKey]: false }));
    setRouteResultsBySession((prev) => ({ ...prev, [sessionKey]: null }));
    setFileMentionFeedbackBySession((prev) => ({ ...prev, [sessionKey]: null }));
    setActiveSessionKey(sessionKey);
    pendingSessionKeyRef.current = null;
    setMessagesForSession(sessionKey, []);
  }, [setMessagesForSession]);

  const replaceMessages = useCallback((sessionKey: string, nextMessages: ChatMessage[]) => {
    setActiveSessionKey(sessionKey);
    setAwaitingReplyBySession((prev) => ({ ...prev, [sessionKey]: false }));
    setMessagesForSession(sessionKey, nextMessages);
  }, []);

  const clearFileMentionFeedback = useCallback(() => {
    setFileMentionFeedbackBySession((prev) => ({ ...prev, [activeSessionKey]: null }));
  }, [activeSessionKey]);

  // Connect on mount, cleanup on unmount
  useEffect(() => {
    connect();
    return cleanup;
  }, [connect, cleanup]);

  const messages = messagesBySession[activeSessionKey] ?? [];
  const routeResult = routeResultsBySession[activeSessionKey] ?? null;
  const isAwaitingReply = awaitingReplyBySession[activeSessionKey] ?? false;
  const fileMentionFeedback = fileMentionFeedbackBySession[activeSessionKey] ?? null;

  return {
    messages,
    activeSessionKey,
    setActiveSessionKey,
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
