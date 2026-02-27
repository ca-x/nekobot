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
} from 'lucide-react';

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
      if (!window.confirm(t('killConfirm'))) return;
      killMutation.mutate(id);
    },
    [killMutation],
  );

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
                      </div>
                    </div>
                  );
                })}
              </div>
            </ScrollArea>
          )}
        </div>

        {/* ======== Main Terminal Area ======== */}
        <div className="flex-1 flex flex-col min-w-0">
          {/* Tab bar */}
          {openTabs.length > 0 && (
            <div className="flex items-center border-b border-border bg-card overflow-x-auto">
              <div className="flex items-center gap-0 flex-1 min-w-0 px-1">
                {openTabs.map((tabId) => {
                  const s = getSessionById(tabId);
                  const tabTitle = s ? getTitle(s) : tabId.slice(0, 8);
                  const isActive = tabId === validActiveTab;
                  const isSplit = tabId === validSplitTab;
                  return (
                    <div
                      key={tabId}
                      className={cn(
                        'flex items-center gap-1 px-3 py-1.5 text-xs cursor-pointer border-b-2 transition-colors whitespace-nowrap',
                        isActive
                          ? 'border-primary text-foreground font-medium'
                          : isSplit
                            ? 'border-blue-400 text-foreground/80'
                            : 'border-transparent text-muted-foreground hover:text-foreground',
                      )}
                      onClick={() => setActiveTab(tabId)}
                    >
                      <span>{tabTitle}</span>
                      {isSplit && (
                        <span className="text-[10px] text-blue-500">[split]</span>
                      )}
                      <button
                        className="ml-1 hover:bg-muted rounded p-0.5"
                        onClick={(e) => {
                          e.stopPropagation();
                          closeTab(tabId);
                        }}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </div>
                  );
                })}
              </div>
              {hasSplit && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 mr-1 text-xs"
                  onClick={clearSplit}
                >
                  {t('clearSplit')}
                </Button>
              )}
            </div>
          )}

          {/* Terminal panels */}
          <div className="flex-1 min-h-0 flex">
            {validActiveTab ? (
              <>
                {/* Primary panel */}
                <div
                  className={cn(
                    'flex flex-col min-h-0 min-w-0',
                    hasSplit ? 'flex-1' : 'flex-1',
                  )}
                >
                  <div className="flex items-center justify-between px-2 py-1 bg-muted/30 border-b border-border text-xs text-muted-foreground">
                    <span>
                      {getTitle(getSessionById(validActiveTab)!) ||
                        validActiveTab.slice(0, 8)}
                    </span>
                    <SessionStatusBadge
                      session={getSessionById(validActiveTab)}
                    />
                  </div>
                  <div className="flex-1 min-h-0">
                    <TerminalPanel
                      key={validActiveTab}
                      sessionId={validActiveTab}
                      active={true}
                    />
                  </div>
                </div>

                {/* Split panel */}
                {hasSplit && (
                  <>
                    <div className="w-px bg-border" />
                    <div className="flex flex-col flex-1 min-h-0 min-w-0">
                      <div className="flex items-center justify-between px-2 py-1 bg-muted/30 border-b border-border text-xs text-muted-foreground">
                        <span>
                          {getTitle(getSessionById(validSplitTab)!) ||
                            validSplitTab.slice(0, 8)}
                        </span>
                        <SessionStatusBadge
                          session={getSessionById(validSplitTab)}
                        />
                      </div>
                      <div className="flex-1 min-h-0">
                        <TerminalPanel
                          key={validSplitTab}
                          sessionId={validSplitTab}
                          active={true}
                        />
                      </div>
                    </div>
                  </>
                )}
              </>
            ) : (
              /* Empty state */
              <div className="flex-1 flex items-center justify-center text-muted-foreground text-sm">
                <div className="text-center space-y-3">
                  <Terminal className="h-12 w-12 mx-auto opacity-30" />
                  <p>{t('noSessionOpened')}</p>
                  <Button size="sm" onClick={handleNewSession}>
                    <Plus className="h-3.5 w-3.5 mr-1.5" />
                    {t('newToolSession')}
                  </Button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Dialogs */}
      <ToolSessionDialog
        open={sessionDialogOpen}
        onOpenChange={setSessionDialogOpen}
        editSession={editSession}
        onCreated={handleSessionCreated}
      />
      <ToolAccessDialog
        open={accessDialogOpen}
        onOpenChange={setAccessDialogOpen}
        session={accessSession}
        initialUrl={accessInitialUrl}
        initialPassword={accessInitialPw}
      />
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
