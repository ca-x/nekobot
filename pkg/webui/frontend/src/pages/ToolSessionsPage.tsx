import { useCallback, useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { t } from '@/lib/i18n';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import {
  useCleanupTerminated,
  useKillToolProcess,
  useToolSessions,
  type ToolSession,
} from '@/hooks/useToolSessions';
import ToolAccessDialog, { getAccessRecord } from '@/components/tools/ToolAccessDialog';
import TerminalPanel from '@/components/tools/TerminalPanel';
import ToolSessionDialog from '@/components/tools/ToolSessionDialog';
import {
  Columns2,
  Key,
  Loader2,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  Settings2,
  Skull,
  Terminal,
  Trash2,
  X,
} from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogPortal,
  DialogTitle,
} from '@/components/ui/dialog';

function isVisibleSession(item: ToolSession): boolean {
  return (item.state || '').toLowerCase() !== 'archived';
}

function getTitle(item: ToolSession): string {
  return (item.title || item.tool || item.id || '-').trim() || '-';
}

function formatUpdatedAt(value?: string): string {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

function getRuntimeTransport(session: ToolSession): string {
  const direct = (session.runtime_transport || '').trim();
  if (direct) return direct;
  const metadata = session.metadata;
  if (metadata && typeof metadata === 'object') {
    const nested = String((metadata as Record<string, unknown>).runtime_transport || '').trim();
    if (nested) return nested;
  }
  return 'tmux';
}

function SessionStatusBadge({ session }: { session: ToolSession | null }) {
  if (!session) return null;
  const running = (session.state || '').toLowerCase() === 'running';
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2 py-1 text-[10px] font-medium',
        running
          ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
          : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
      )}
    >
      {running ? t('running') : session.state || '-'}
    </span>
  );
}

export default function ToolSessionsPage() {
  const { data: rawSessions = [], isLoading } = useToolSessions();
  const cleanupMutation = useCleanupTerminated();
  const killMutation = useKillToolProcess();

  const sessions = useMemo(() => rawSessions.filter(isVisibleSession), [rawSessions]);
  const terminatedCount = useMemo(
    () => rawSessions.filter((item) => (item.state || '').toLowerCase() === 'terminated').length,
    [rawSessions],
  );

  const [openTabs, setOpenTabs] = useState<string[]>([]);
  const [activeTab, setActiveTab] = useState('');
  const [splitTab, setSplitTab] = useState('');
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  const [sessionDialogOpen, setSessionDialogOpen] = useState(false);
  const [editSession, setEditSession] = useState<ToolSession | null>(null);
  const [accessDialogOpen, setAccessDialogOpen] = useState(false);
  const [accessSession, setAccessSession] = useState<ToolSession | null>(null);
  const [accessInitialUrl, setAccessInitialUrl] = useState('');
  const [accessInitialPw, setAccessInitialPw] = useState('');
  const [showKillConfirm, setShowKillConfirm] = useState(false);
  const [killTargetId, setKillTargetId] = useState('');

  const getSessionById = useCallback(
    (id: string) => rawSessions.find((item) => item.id === id) ?? null,
    [rawSessions],
  );

  const openTab = useCallback(
    (sessionId: string, asSplit = false) => {
      setOpenTabs((current) => (current.includes(sessionId) ? current : [...current, sessionId]));

      if (asSplit) {
        if (sessionId === activeTab) {
          toast.warning(t('splitRequiresAnotherSession'));
          return;
        }
        setSplitTab(sessionId);
        if (!activeTab) {
          setActiveTab(sessionId);
          setSplitTab('');
        }
        return;
      }

      setActiveTab(sessionId);
      if (splitTab === sessionId) {
        setSplitTab('');
      }
    },
    [activeTab, splitTab],
  );

  const closeTab = useCallback(
    (sessionId: string) => {
      const remaining = openTabs.filter((id) => id !== sessionId);
      setOpenTabs(remaining);

      if (activeTab === sessionId) {
        const nextActive = remaining.find((id) => id !== splitTab) ?? remaining[0] ?? '';
        setActiveTab(nextActive);
      }

      if (splitTab === sessionId) {
        setSplitTab('');
      }
    },
    [activeTab, openTabs, splitTab],
  );

  const clearSplit = useCallback(() => {
    setSplitTab('');
  }, []);

  const handleNewSession = useCallback(() => {
    setEditSession(null);
    setSessionDialogOpen(true);
  }, []);

  const handleEditSession = useCallback(
    (id: string) => {
      const session = getSessionById(id);
      if (!session) return;
      setEditSession(session);
      setSessionDialogOpen(true);
    },
    [getSessionById],
  );

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

  const handleShowAccess = useCallback(
    (id: string) => {
      const session = getSessionById(id);
      if (!session) return;
      const mode = (session.access_mode || '').trim().toLowerCase();
      if (!mode || mode === 'none') {
        toast.warning(t('externalAccessDisabled'));
        return;
      }
      const cached = getAccessRecord(id);
      setAccessSession(session);
      setAccessInitialUrl(cached?.url || '');
      setAccessInitialPw(cached?.password || '');
      setAccessDialogOpen(true);
    },
    [getSessionById],
  );

  const handleKill = useCallback((id: string) => {
    setKillTargetId(id);
    setShowKillConfirm(true);
  }, []);

  const confirmKill = useCallback(() => {
    if (!killTargetId) return;
    killMutation.mutate(killTargetId);
    setShowKillConfirm(false);
    setKillTargetId('');
  }, [killMutation, killTargetId]);

  const handleCleanup = useCallback(() => {
    cleanupMutation.mutate();
  }, [cleanupMutation]);

  const validActiveTab = activeTab && openTabs.includes(activeTab) ? activeTab : '';
  const validSplitTab =
    splitTab && splitTab !== validActiveTab && openTabs.includes(splitTab) ? splitTab : '';
  const panels = [validActiveTab, validSplitTab].filter((id): id is string => Boolean(id));

  return (
    <>
      <div className="flex h-[calc(100vh-4rem)] flex-col">
        <Header title={t('tabTools')} />

        <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
          <aside
            className={cn(
              'flex shrink-0 flex-col border-b border-border bg-card transition-[width] duration-200 lg:border-b-0 lg:border-r',
              sidebarCollapsed ? 'w-full lg:w-12' : 'w-full lg:w-80',
            )}
          >
            <div className="flex items-center justify-between gap-2 border-b border-border p-2">
              {!sidebarCollapsed && (
                <div className="flex min-w-0 flex-1 items-center gap-2">
                  <Button size="sm" className="h-8 flex-1 sm:flex-none" onClick={handleNewSession}>
                    <Plus className="mr-1 h-3.5 w-3.5" />
                    {t('newToolSession')}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-8 shrink-0"
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
                className="h-8 w-8 shrink-0"
                onClick={() => setSidebarCollapsed((current) => !current)}
                aria-label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
              >
                {sidebarCollapsed ? <PanelLeftOpen className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
              </Button>
            </div>

            {!sidebarCollapsed && (
              <ScrollArea className="flex-1">
                <div className="space-y-2 p-2">
                  {isLoading && sessions.length === 0 && (
                    <div className="p-3 text-center text-xs text-muted-foreground">{t('loading')}</div>
                  )}

                  {!isLoading && sessions.length === 0 && (
                    <div className="space-y-1.5 rounded-xl border border-dashed border-border p-4 text-center">
                      <p className="text-sm font-medium text-foreground">{t('noToolSessions')}</p>
                      <p className="text-xs leading-5 text-muted-foreground">{t('noToolSessionsHint')}</p>
                      <Button
                        variant="outline"
                        size="sm"
                        className="mt-2 h-8 rounded-full px-4 text-xs"
                        onClick={handleNewSession}
                      >
                        <Plus className="mr-1 h-3 w-3" />
                        {t('newToolSession')}
                      </Button>
                    </div>
                  )}

                  {sessions.map((item) => {
                    const running = (item.state || '').toLowerCase() === 'running';
                    const terminated = (item.state || '').toLowerCase() === 'terminated';
                    const isOpen = item.id === validActiveTab || item.id === validSplitTab;
                    const accessEnabled = (item.access_mode || '').trim().toLowerCase() !== 'none';

                    return (
                      <div
                        key={item.id}
                        className={cn(
                          'rounded-xl border p-3 transition-colors',
                          isOpen
                            ? 'border-primary/50 bg-primary/5'
                            : 'border-transparent hover:border-border hover:bg-muted/40',
                        )}
                      >
                        <button type="button" onClick={() => openTab(item.id)} className="w-full text-left">
                          <div className="mb-2 flex items-start gap-2">
                            <div className="min-w-0 flex-1">
                              <div className="line-clamp-2 text-sm font-medium text-foreground" title={getTitle(item)}>{getTitle(item)}</div>
                              <div className="mt-1 flex flex-wrap items-center gap-1 text-[11px] text-muted-foreground">
                                <span className="shrink-0">{item.tool || '-'}</span>
                                <span className="shrink-0">·</span>
                                <span className="shrink-0 uppercase">{getRuntimeTransport(item)}</span>
                                <span className="shrink-0">·</span>
                                <span className="min-w-0 break-all font-mono" title={item.command || '-'}>{item.command || '-'}</span>
                              </div>
                            </div>
                            <SessionStatusBadge session={item} />
                          </div>
                          <div className="flex flex-wrap gap-1 text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
                            {item.source === 'agent' && <span>{t('sourceAgent')}</span>}
                            {item.source === 'channel' && <span>{t('sourceChannel')}</span>}
                            <span>{formatUpdatedAt(item.updated_at)}</span>
                          </div>
                        </button>

                        <div className="mt-3 flex flex-wrap gap-2" onClick={(event) => event.stopPropagation()}>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 px-3 text-[11px]"
                            onClick={() => openTab(item.id)}
                            disabled={terminated}
                          >
                            <Terminal className="mr-1 h-3 w-3" />
                            {t('open')}
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 px-3 text-[11px]"
                            onClick={() => openTab(item.id, true)}
                            disabled={terminated}
                          >
                            <Columns2 className="mr-1 h-3 w-3" />
                            {t('openSplit')}
                          </Button>
                          {accessEnabled && (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-8 px-3 text-[11px]"
                              onClick={() => handleShowAccess(item.id)}
                            >
                              <Key className="mr-1 h-3 w-3" />
                              {getAccessRecord(item.id) ? t('copyAccess') : t('refreshAccess')}
                            </Button>
                          )}
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 px-3 text-[11px]"
                            onClick={() => handleEditSession(item.id)}
                          >
                            <Settings2 className="mr-1 h-3 w-3" />
                            {t('modify')}
                          </Button>
                          {running && (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-8 px-3 text-[11px] text-destructive hover:text-destructive"
                              onClick={() => handleKill(item.id)}
                            >
                              <Skull className="mr-1 h-3 w-3" />
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
          </aside>

          <section className="flex min-w-0 flex-1 flex-col bg-background">
            <div className="flex min-h-[48px] flex-col gap-2 border-b border-border px-3 py-2 sm:flex-row sm:items-center sm:justify-between sm:py-0">
              <ScrollArea className="min-w-0 flex-1">
                <div className="flex items-center gap-2 py-1 sm:py-2">
                  {openTabs.map((id) => {
                    const session = getSessionById(id);
                    const isSelected = id === validActiveTab || id === validSplitTab;
                    return (
                      <div
                        key={id}
                        className={cn(
                          'inline-flex shrink-0 max-w-[160px] sm:max-w-[180px] items-center gap-2 rounded-full border px-3 py-1.5 text-xs',
                          isSelected
                            ? 'border-primary/50 bg-primary/10 text-foreground'
                            : 'border-border bg-card text-muted-foreground',
                        )}
                      >
                        <button type="button" onClick={() => openTab(id)} className="min-w-0 truncate text-left">
                          {session ? getTitle(session) : id}
                        </button>
                        <button type="button" onClick={() => closeTab(id)} className="shrink-0 rounded-full p-0.5 hover:bg-muted">
                          <X className="h-3 w-3" />
                        </button>
                      </div>
                    );
                  })}
                </div>
              </ScrollArea>

              {validSplitTab && (
                <Button variant="ghost" size="sm" className="h-8 self-start sm:self-auto" onClick={clearSplit}>
                  {t('clearSplit')}
                </Button>
              )}
            </div>

            {panels.length === 0 ? (
              <div className="flex min-h-0 flex-1 items-center justify-center p-6">
                <div className="max-w-md rounded-[1.8rem] border border-dashed border-border bg-card/80 p-8 text-center">
                  <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
                    <Terminal className="h-5 w-5" />
                  </div>
                  <div className="mt-4 text-base font-semibold text-foreground">{t('noSessionOpened')}</div>
                  <p className="mt-2 text-sm leading-6 text-muted-foreground">{t('noToolSessionsHint')}</p>
                  <div className="mt-4 flex flex-wrap justify-center gap-2">
                    <Button className="rounded-full" onClick={handleNewSession}>
                      <Plus className="mr-2 h-4 w-4" />
                      {t('newToolSession')}
                    </Button>
                    {!sidebarCollapsed && sessions.length > 0 && (
                      <Button variant="outline" className="rounded-full" onClick={() => openTab(sessions[0].id)}>
                        <Terminal className="mr-2 h-4 w-4" />
                        {t('open')}
                      </Button>
                    )}
                  </div>
                </div>
              </div>
            ) : (
              <div
                className={cn(
                  'grid min-h-0 flex-1 gap-3 p-3',
                  panels.length === 2 ? 'grid-cols-1 xl:grid-cols-2' : 'grid-cols-1',
                )}
              >
                {panels.map((id) => {
                  const session = getSessionById(id);
                  if (!session) return null;

                  return (
                    <div key={id} className="flex min-h-0 flex-col overflow-hidden rounded-xl border border-border bg-card">
                      <div className="flex flex-col gap-2 border-b border-border px-4 py-3 sm:flex-row sm:items-start sm:justify-between">
                        <div className="min-w-0 flex-1">
                          <div className="text-sm font-medium text-foreground" title={getTitle(session)}>{getTitle(session)}</div>
                          <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                            <span className="shrink-0">{session.tool || '-'}</span>
                            <SessionStatusBadge session={session} />
                          </div>
                          <div className="mt-2 space-y-1 text-xs text-muted-foreground">
                            <div className="break-all font-mono" title={session.command || '-'}>{session.command || '-'}</div>
                            <div className="uppercase">{getRuntimeTransport(session)}</div>
                            <div className="break-all" title={session.workdir || '-'}>{session.workdir || '-'}</div>
                          </div>
                        </div>
                        <div className="flex shrink-0 gap-1 self-end sm:self-auto">
                          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => handleEditSession(session.id)}>
                            <Settings2 className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => closeTab(session.id)}>
                            <X className="h-4 w-4" />
                          </Button>
                        </div>
                      </div>
                      <div className="min-h-0 flex-1 bg-background">
                        <TerminalPanel sessionId={session.id} active={session.id === validActiveTab} />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </section>
        </div>
      </div>

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

      <Dialog open={showKillConfirm} onOpenChange={setShowKillConfirm}>
        <DialogPortal>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('killConfirmTitle')}</DialogTitle>
              <DialogDescription>{t('killConfirmDescription')}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowKillConfirm(false)}>
                {t('cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmKill} disabled={killMutation.isPending}>
                {killMutation.isPending ? (
                  <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                ) : (
                  <Skull className="mr-1.5 h-4 w-4" />
                )}
                {t('kill')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </DialogPortal>
      </Dialog>
    </>
  );
}
