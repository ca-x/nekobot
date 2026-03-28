import { useState, useCallback, useMemo } from 'react';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { t } from '@/lib/i18n';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import {
  useToolSessions,
  useCleanupTerminated,
  useKillToolProcess,
  type ToolSession,
} from '@/hooks/useToolSessions';
import { getAccessRecord } from '@/components/tools/ToolAccessDialog';
import TerminalPanel from '@/components/tools/TerminalPanel';
import ToolSessionDialog from '@/components/tools/ToolSessionDialog';
import ToolAccessDialog from '@/components/tools/ToolAccessDialog';
import {
  Plus,
  Trash2,
  Terminal,
  Columns2,
  Key,
  Settings2,
  Skull,
  X,
  PanelLeftClose,
  PanelLeftOpen,
  Loader2,
} from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogPortal,
} from '@/components/ui/dialog';

/* ---------- helpers ---------- */

function isVisibleSession(item: ToolSession): boolean {
  const st = (item.state || '').toLowerCase();
  return st !== 'archived';
}

function getTitle(item: ToolSession): string {
  return (item.title || item.tool || item.id || '-').trim() || '-';
}

/* ---------- component ---------- */

export default function ToolSessionsPage() {
  const { data: rawSessions = [], isLoading } = useToolSessions();
  const cleanupMutation = useCleanupTerminated();
  const killMutation = useKillToolProcess();

  /* ---- visible sessions ---- */
  const sessions = useMemo(
    () => rawSessions.filter(isVisibleSession),
    [rawSessions],
  );
  const terminatedCount = useMemo(
    () =>
      rawSessions.filter((s) => (s.state || '').toLowerCase() === 'terminated')
        .length,
    [rawSessions],
  );

  /* ---- tab state ---- */
  const [openTabs, setOpenTabs] = useState<string[]>([]);
  const [activeTab, setActiveTab] = useState('');
  const [splitTab, setSplitTab] = useState('');

  /* ---- dialog state ---- */
  const [sessionDialogOpen, setSessionDialogOpen] = useState(false);
  const [editSession, setEditSession] = useState<ToolSession | null>(null);
  const [accessDialogOpen, setAccessDialogOpen] = useState(false);
  const [accessSession, setAccessSession] = useState<ToolSession | null>(null);
  const [accessInitialUrl, setAccessInitialUrl] = useState('');
  const [accessInitialPw, setAccessInitialPw] = useState('');
  const [showKillConfirm, setShowKillConfirm] = useState(false);
  const [killTargetId, setKillTargetId] = useState<string>('');

  /* ---- sidebar toggle ---- */
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  /* ---- helper: find session by ID ---- */
  const getSessionById = useCallback(
    (id: string) => rawSessions.find((s) => s.id === id) || null,
    [rawSessions],
  );

  /* ---- open a tab ---- */
  const openTab = useCallback(
    (sessionId: string, asSplit?: boolean) => {
      setOpenTabs((prev) => {
        const next = prev.includes(sessionId)
          ? prev
          : [...prev, sessionId];
        return next;
      });

      if (asSplit) {
        setSplitTab((prev) => {
          /* If trying to split with the same session as active, swap */
          if (sessionId === activeTab) {
            toast.warning(t('splitRequiresAnotherSession'));
            return prev;
          }
          return sessionId;
        });
      } else {
        setActiveTab(sessionId);
      }
    },
    [activeTab],
  );

  /* ---- close a tab ---- */
  const closeTab = useCallback(
    (sessionId: string) => {
      setOpenTabs((prev) => prev.filter((id) => id !== sessionId));
      if (activeTab === sessionId) {
        setActiveTab((prev) => {
          const remaining = openTabs.filter((id) => id !== sessionId);
          return remaining.length > 0
            ? remaining[remaining.length - 1]
            : '';
        });
      }
      if (splitTab === sessionId) {
        setSplitTab('');
      }
    },
    [activeTab, splitTab, openTabs],
  );

  /* ---- clear split ---- */
  const clearSplit = useCallback(() => {
    setSplitTab('');
  }, []);

  /* ---- open "new session" dialog ---- */
  const handleNewSession = useCallback(() => {
    setEditSession(null);
    setSessionDialogOpen(true);
  }, []);

  /* ---- open "edit session" dialog ---- */
  const handleEditSession = useCallback(
    (id: string) => {
      const s = getSessionById(id);
      if (!s) return;
      setEditSession(s);
      setSessionDialogOpen(true);
    },
    [getSessionById],
  );

  /* ---- session created/updated callback ---- */
  const handleSessionCreated = useCallback(
    (session: ToolSession, accessUrl?: string, accessPassword?: string) => {
      openTab(session.id);
      if (accessUrl && accessPassword) {
        setAccessSession(session);
        setAccessInitialUrl(accessUrl);
        setAccessInitialPw(accessPassword);
        setAccessDialogOpen(true);
      }
    },
    [openTab],
  );

  /* ---- access dialog ---- */
  const handleShowAccess = useCallback(
    (id: string) => {
      const s = getSessionById(id);
      if (!s) return;
      const mode = (s.access_mode || '').trim().toLowerCase();
      if (!mode || mode === 'none') {
        toast.warning(t('externalAccessDisabled'));
        return;
      }
      const cached = getAccessRecord(id);
      setAccessSession(s);
      setAccessInitialUrl(cached?.url || '');
      setAccessInitialPw(cached?.password || '');
      setAccessDialogOpen(true);
    },
    [getSessionById],
  );

  /* ---- kill process ---- */
  const handleKill = useCallback(
    (id: string) => {
      setKillTargetId(id);
      setShowKillConfirm(true);
    },
    [],
  );

  const confirmKill = useCallback(() => {
    if (!killTargetId) return;
    killMutation.mutate(killTargetId);
    setShowKillConfirm(false);
    setKillTargetId('');
  }, [killMutation, killTargetId]);

  /* ---- cleanup terminated ---- */
  const handleCleanup = useCallback(() => {
    cleanupMutation.mutate();
  }, [cleanupMutation]);

  /* ---- ensure active tab valid ---- */
  const validActiveTab =
    activeTab && openTabs.includes(activeTab) ? activeTab : '';
  const validSplitTab =
    splitTab &&
    splitTab !== validActiveTab &&
    openTabs.includes(splitTab)
      ? splitTab
      : '';
  const hasSplit = !!validSplitTab;

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)]">
      <Header title={t('tabTools')} />

      <div className="flex flex-1 min-h-0 gap-0">
        {/* ======== Left Sidebar ======== */}
        <div
          className={cn(
            'flex flex-col border-r border-border bg-card transition-[width] duration-200',
            sidebarCollapsed ? 'w-10' : 'w-72',
          )}
        >
          {/* Sidebar header */}
          <div className="flex items-center justify-between p-2 border-b border-border">
            {!sidebarCollapsed && (
              <div className="flex gap-1 flex-1 min-w-0">
                <Button size="sm" onClick={handleNewSession}>
                  <Plus className="h-3.5 w-3.5 mr-1" />
                  {t('newToolSession')}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleCleanup}
                  disabled={terminatedCount === 0 || cleanupMutation.isPending}
                  title={t('cleanupTerminatedCount', String(terminatedCount))}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            )}
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 shrink-0"
              onClick={() => setSidebarCollapsed((p) => !p)}
              aria-label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            >
              {sidebarCollapsed ? (
                <PanelLeftOpen className="h-4 w-4" />
              ) : (
                <PanelLeftClose className="h-4 w-4" />
              )}
            </Button>
          </div>

          {/* Session list */}
          {!sidebarCollapsed && (
            <ScrollArea className="flex-1">
              <div className="p-2 space-y-1">
                {isLoading && sessions.length === 0 && (
                  <div className="text-xs text-muted-foreground p-3 text-center">
                    Loading\u2026
                  </div>
                )}
                {!isLoading && sessions.length === 0 && (
                  <div className="text-xs text-muted-foreground p-3 text-center">
                    {t('noToolSessions')}
                  </div>
                )}
                {sessions.map((item) => {
                  const running =
                    (item.state || '').toLowerCase() === 'running';
                  const isActive = item.id === validActiveTab;
                  const accessEnabled =
                    (item.access_mode || '').trim().toLowerCase() !== 'none';

                  return (
                    <div
                      key={item.id}
                      className={cn(
                        'rounded-md border p-2 text-xs transition-colors cursor-pointer',
                        isActive
                          ? 'border-primary/50 bg-primary/5'
                          : 'border-transparent hover:bg-muted/50',
                      )}
                      onClick={() => openTab(item.id)}
                    >
                      {/* Title + badges row */}
                      <div className="flex items-center gap-1.5 mb-1">
                        <span className="font-medium truncate flex-1">
                          {getTitle(item)}
                        </span>
                        {item.source === 'agent' && (
                          <span className="inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                            {t('sourceAgent')}
                          </span>
                        )}
                        {item.source === 'channel' && (
                          <span className="inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400">
                            {t('sourceChannel')}
                          </span>
                        )}
                        <span
                          className={cn(
                            'inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium',
                            running
                              ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                              : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
                          )}
                        >
                          {item.state || '-'}
                        </span>
                      </div>

                      {/* Meta */}
                      <div className="text-muted-foreground truncate mb-1.5">
                        {item.tool || '-'} &middot;{' '}
                        {item.command || '-'}
                      </div>

                      {/* Action buttons */}
                      <div
                        className="flex gap-1 flex-wrap"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 px-1.5 text-[11px]"
                          onClick={() => openTab(item.id)}
                          title={t('open')}
                        >
                          <Terminal className="h-3 w-3 mr-0.5" />
                          {t('open')}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 px-1.5 text-[11px]"
                          onClick={() => openTab(item.id, true)}
                          title={t('openSplit')}
                        >
                          <Columns2 className="h-3 w-3 mr-0.5" />
                          {t('openSplit')}
                        </Button>
                        {accessEnabled && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-6 px-1.5 text-[11px]"
                            onClick={() => handleShowAccess(item.id)}
                            title={t('refreshAccess')}
                          >
                            <Key className="h-3 w-3 mr-0.5" />
                            {getAccessRecord(item.id)
                              ? t('copyAccess')
                              : t('refreshAccess')}
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 px-1.5 text-[11px]"
                          onClick={() => handleEditSession(item.id)}
                          title={t('modify')}
                        >
                          <Settings2 className="h-3 w-3 mr-0.5" />
                          {t('modify')}
                        </Button>
                        {running && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-6 px-1.5 text-[11px] text-destructive hover:text-destructive"
                            onClick={() => handleKill(item.id)}
                            title={t('kill')}
                          >
                            <Skull className="h-3 w-3 mr-0.5" />
                            {t('kill')}
                          </Button>
)}
      />

    {/* Kill Confirmation Dialog */}
    <Dialog open={showKillConfirm} onOpenChange={setShowKillConfirm}>
      <DialogPortal>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('killConfirmTitle')}</DialogTitle>
            <DialogDescription>
              {t('killConfirmDescription')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowKillConfirm(false)}>
              {t('cancel')}
            </Button>
            <Button
              variant="destructive"
              onClick={confirmKill}
              disabled={killMutation.isPending}
            >
              {killMutation.isPending ? (
                <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
              ) : (
                <Skull className="h-4 w-4 mr-1.5" />
              )}
              {t('kill')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </DialogPortal>
    </Dialog>
    </div>
  );
}

/* ---------- small status badge helper ---------- */

function SessionStatusBadge({ session }: { session: ToolSession | null }) {
  if (!session) return null;
  const running = (session.state || '').toLowerCase() === 'running';
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium',
        running
          ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
          : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
      )}
    >
      {running ? t('running') : session.state || '-'}
    </span>
  );
}
