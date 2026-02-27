import { useEffect, useRef, useCallback } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { getToken } from '@/api/client';
import '@xterm/xterm/css/xterm.css';

interface TerminalPanelProps {
  sessionId: string;
  active: boolean;
}

interface WsMessage {
  type: string;
  data?: string;
  total?: number;
  running?: boolean;
  missing?: boolean;
  exit_code?: number;
  message?: string;
  cols?: number;
  rows?: number;
}

function buildWsUrl(sessionId: string): string {
  const token = getToken();
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
  return (
    proto +
    '://' +
    window.location.host +
    '/api/tool-sessions/ws?token=' +
    encodeURIComponent(token || '') +
    '&session_id=' +
    encodeURIComponent(sessionId)
  );
}

export default function TerminalPanel({ sessionId, active }: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const sessionRef = useRef(sessionId);
  const inputQueueRef = useRef('');
  const sendingRef = useRef(false);

  /* ---- send resize over WS ---- */
  const sendResize = useCallback((cols: number, rows: number) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    if (cols <= 0 || rows <= 0) return;
    ws.send(JSON.stringify({ type: 'resize', cols, rows }));
  }, []);

  /* ---- send input over WS (with queue) ---- */
  const sendInput = useCallback(async (data: string) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: 'input', data }));
  }, []);

  const flushQueue = useCallback(async () => {
    if (sendingRef.current) return;
    const payload = inputQueueRef.current;
    if (!payload) return;
    inputQueueRef.current = '';
    sendingRef.current = true;
    try {
      await sendInput(payload);
    } finally {
      sendingRef.current = false;
      if (inputQueueRef.current) flushQueue();
    }
  }, [sendInput]);

  const queueInput = useCallback(
    (data: string) => {
      inputQueueRef.current += data;
      flushQueue();
    },
    [flushQueue],
  );

  /* ---- main effect: create terminal + websocket ---- */
  useEffect(() => {
    const el = containerRef.current;
    if (!el || !sessionId) return;

    sessionRef.current = sessionId;

    /* Create terminal */
    const isDark = document.documentElement.classList.contains('dark');
    const term = new Terminal({
      convertEol: true,
      cursorBlink: true,
      fontFamily: "'SF Mono', 'Fira Code', 'Cascadia Code', Consolas, monospace",
      fontSize: 13,
      scrollback: 10000,
      theme: isDark
        ? {
            background: '#1a1b26',
            foreground: '#c0caf5',
            cursor: '#c0caf5',
            selectionBackground: '#33467c',
          }
        : {
            background: '#fafafa',
            foreground: '#383a42',
            cursor: '#526fff',
            selectionBackground: '#d2d5db',
          },
    });
    termRef.current = term;

    /* Fit addon */
    const fitAddon = new FitAddon();
    fitAddonRef.current = fitAddon;
    term.loadAddon(fitAddon);

    /* Web links addon */
    try {
      term.loadAddon(new WebLinksAddon());
    } catch {
      /* ignore */
    }

    /* Open terminal in DOM */
    term.open(el);
    try {
      fitAddon.fit();
    } catch {
      /* ignore */
    }

    /* Terminal data handler: user typing */
    const dataDisposable = term.onData((data) => {
      queueInput(data);
    });

    /* Terminal resize handler */
    const resizeDisposable = term.onResize((size) => {
      sendResize(size.cols, size.rows);
    });

    /* Custom key handler for Ctrl+Shift+C/V */
    term.attachCustomKeyEventHandler((ev) => {
      const isMac = navigator.platform.toUpperCase().includes('MAC');
      const ctrlOrMeta = isMac ? ev.metaKey : ev.ctrlKey;
      if (!ctrlOrMeta || !ev.shiftKey) return true;
      const key = (ev.key || '').toLowerCase();
      if (key === 'c') {
        const selected = term.getSelection();
        if (!selected) return true;
        navigator.clipboard.writeText(selected).catch(() => {});
        return false;
      }
      if (key === 'v') {
        navigator.clipboard
          .readText()
          .then((text) => {
            if (text) queueInput(text);
          })
          .catch(() => {});
        return false;
      }
      return true;
    });

    /* Connect WebSocket */
    let ws: WebSocket;
    try {
      ws = new WebSocket(buildWsUrl(sessionId));
    } catch {
      return;
    }
    wsRef.current = ws;

    ws.onopen = () => {
      /* Send initial resize */
      sendResize(term.cols, term.rows);
    };

    ws.onmessage = (ev) => {
      let msg: WsMessage;
      try {
        msg = JSON.parse(ev.data || '{}');
      } catch {
        return;
      }

      if (msg.type === 'output') {
        const data = msg.data || '';
        if (data) {
          term.write(data);
        }
        return;
      }

      if (msg.type === 'status') {
        /* We don't need to do much here; the session list polling handles state */
        return;
      }

      if (msg.type === 'error') {
        const errMsg = msg.message || 'WebSocket error';
        term.write('\r\n\x1b[31m[Error] ' + errMsg + '\x1b[0m\r\n');
        return;
      }
    };

    ws.onerror = () => {
      term.write('\r\n\x1b[31m[WebSocket connection error]\x1b[0m\r\n');
    };

    ws.onclose = () => {
      term.write('\r\n\x1b[33m[WebSocket disconnected]\x1b[0m\r\n');
    };

    /* Window resize listener */
    const handleResize = () => {
      try {
        fitAddon.fit();
      } catch {
        /* ignore */
      }
    };
    window.addEventListener('resize', handleResize);

    /* ResizeObserver for container size changes */
    let resizeObserver: ResizeObserver | null = null;
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => {
        try {
          fitAddon.fit();
        } catch {
          /* ignore */
        }
      });
      resizeObserver.observe(el);
    }

    /* Cleanup */
    return () => {
      window.removeEventListener('resize', handleResize);
      if (resizeObserver) {
        resizeObserver.disconnect();
      }
      dataDisposable.dispose();
      resizeDisposable.dispose();
      try {
        ws.close();
      } catch {
        /* ignore */
      }
      wsRef.current = null;
      term.dispose();
      termRef.current = null;
      fitAddonRef.current = null;
    };
  }, [sessionId, queueInput, sendResize]);

  /* ---- Refit when becoming active ---- */
  useEffect(() => {
    if (!active) return;
    const timer = setTimeout(() => {
      const fitAddon = fitAddonRef.current;
      if (fitAddon) {
        try {
          fitAddon.fit();
        } catch {
          /* ignore */
        }
      }
      const term = termRef.current;
      if (term) {
        term.focus();
      }
    }, 50);
    return () => clearTimeout(timer);
  }, [active]);

  /* ---- Theme sync ---- */
  useEffect(() => {
    const observer = new MutationObserver(() => {
      const term = termRef.current;
      if (!term) return;
      const isDark = document.documentElement.classList.contains('dark');
      term.options.theme = isDark
        ? {
            background: '#1a1b26',
            foreground: '#c0caf5',
            cursor: '#c0caf5',
            selectionBackground: '#33467c',
          }
        : {
            background: '#fafafa',
            foreground: '#383a42',
            cursor: '#526fff',
            selectionBackground: '#d2d5db',
          };
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    });
    return () => observer.disconnect();
  }, []);

  return (
    <div
      ref={containerRef}
      className="w-full h-full min-h-0"
      style={{ overflow: 'hidden' }}
    />
  );
}
