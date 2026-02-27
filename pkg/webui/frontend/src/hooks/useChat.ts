import { useState, useRef, useCallback, useEffect } from 'react';
import { getToken } from '@/api/client';

export interface ChatMessage {
  role: 'user' | 'assistant' | 'system' | 'error';
  content: string;
  timestamp: number;
}

export type ConnectionStatus = 'connected' | 'disconnected' | 'connecting';

interface SendOptions {
  provider: string;
  model: string;
  fallbackProviders: string[];
}

interface UseChatReturn {
  messages: ChatMessage[];
  sendMessage: (text: string, options: SendOptions) => void;
  clearMessages: () => void;
  connectionStatus: ConnectionStatus;
  reconnect: () => void;
}

export function useChat(): UseChatReturn {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('disconnected');
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
      wsRef.current = null;
    };

    ws.onerror = () => {
      setConnectionStatus('disconnected');
    };

    ws.onmessage = (ev: MessageEvent) => {
      let msg: { type?: string; content?: string };
      try {
        msg = JSON.parse(ev.data);
      } catch {
        return;
      }

      const now = Date.now();

      if (msg.type === 'message') {
        setMessages((prev) => [
          ...prev,
          { role: 'assistant', content: msg.content || '', timestamp: now },
        ]);
      } else if (msg.type === 'error') {
        setMessages((prev) => [
          ...prev,
          { role: 'error', content: msg.content || 'Unknown error', timestamp: now },
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
      }),
    );

    setMessages((prev) => [
      ...prev,
      { role: 'user', content: text, timestamp: Date.now() },
    ]);
  }, []);

  const clearMessages = useCallback(() => {
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'clear' }));
    }
    setMessages([]);
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
    connectionStatus,
    reconnect,
  };
}
