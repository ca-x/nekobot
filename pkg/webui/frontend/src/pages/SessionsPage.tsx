import { useEffect, useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import {
  useDeleteSession,
  useSessionDetail,
  useSessions,
  useUpdateSessionSummary,
} from '@/hooks/useSessions';
import { Save, Trash2, Loader2, MessageSquare } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogPortal,
} from '@/components/ui/dialog';

function formatDateTime(value: string): string {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

export default function SessionsPage() {
  const navigate = useNavigate();
  const { data: sessions = [], isLoading } = useSessions();
  const [selectedId, setSelectedId] = useState('');
  const [summaryDraft, setSummaryDraft] = useState('');

  const updateSummary = useUpdateSessionSummary();
  const deleteSession = useDeleteSession();
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  const sortedSessions = useMemo(
    () =>
      [...sessions].sort(
        (a, b) =>
          new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime(),
      ),
    [sessions],
  );

  const selectedExists = sortedSessions.some((item) => item.id === selectedId);

  useEffect(() => {
    if (selectedId && !selectedExists) {
      setSelectedId('');
    }
  }, [selectedId, selectedExists]);

  const {
    data: detail,
    isLoading: isDetailLoading,
    isError: isDetailError,
  } = useSessionDetail(selectedId || null);

  useEffect(() => {
    if (detail) {
      setSummaryDraft(detail.summary || '');
    }
  }, [detail]);

  const handleSaveSummary = () => {
    if (!detail) return;
    updateSummary.mutate({ id: detail.id, summary: summaryDraft });
  };

  const handleDeleteSession = () => {
    if (!detail) return;
    setShowDeleteConfirm(true);
  };

  const confirmDeleteSession = () => {
    if (!detail) return;
    const targetId = detail.id;
    deleteSession.mutate(targetId, {
      onSuccess: () => {
        setSelectedId((current) => (current === targetId ? '' : current));
        setShowDeleteConfirm(false);
      },
    });
  };

  return (
    <>
      <div className="flex flex-col h-[calc(100vh-4rem)]">
      <Header
        title={t('tabSessions')}
        description={t('sessionsPageDescription')}
      />

      <div className="flex flex-1 min-h-0 flex-col gap-4 lg:flex-row">
        <Card className="w-full lg:w-80 min-h-0 flex flex-col shrink-0">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t('tabSessions')}</CardTitle>
            <CardDescription>
              {t('sessionListCount', String(sortedSessions.length))}
            </CardDescription>
          </CardHeader>
          <CardContent className="flex-1 min-h-0 p-0">
            <ScrollArea className="h-full">
              <div className="p-3 space-y-2">
                {isLoading && sortedSessions.length === 0 && (
                  <div className="text-sm text-muted-foreground text-center py-8">
                    {t('sessionsLoading')}
                  </div>
                )}

                {!isLoading && sortedSessions.length === 0 && (
                  <div className="space-y-4 py-8 text-center">
                    <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-muted">
                      <MessageSquare className="h-5 w-5 text-muted-foreground" />
                    </div>
                    <div>
                      <p className="text-sm font-medium">{t('sessionsEmpty')}</p>
                      <p className="text-xs text-muted-foreground mt-1">{t('sessionsEmptyHint')}</p>
                    </div>
                    <Button variant="outline" size="sm" onClick={() => navigate('/chat')} className="mt-2">
                      <MessageSquare className="mr-1.5 h-3.5 w-3.5" />
                      {t('sessionsStartChat')}
                    </Button>
                  </div>
                )}

                {sortedSessions.map((item) => {
                  const displaySummary = item.summary?.trim() || item.id;
                  const isActive = item.id === selectedId;

                  return (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => setSelectedId(item.id)}
                      className={cn(
                        'w-full rounded-md border text-left p-3 transition-colors',
                        isActive
                          ? 'border-primary/40 bg-primary/5'
                          : 'hover:bg-muted/40 border-border',
                      )}
                    >
                      <div className="text-xs text-muted-foreground truncate mb-1" title={item.id}>
                        {item.id}
                      </div>
                      <div className="text-sm font-medium truncate mb-1" title={displaySummary}>
                        {displaySummary}
                      </div>
                      <div className="text-xs text-muted-foreground">
                        {t('sessionUpdatedAt', formatDateTime(item.updated_at))}
                      </div>
                      <div className="text-xs text-muted-foreground mt-1">
                        {t('sessionMessageCount', String(item.message_count))}
                      </div>
                    </button>
                  );
                })}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>

        <Card className="flex-1 min-h-0 flex flex-col">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t('sessionDetailsTitle')}</CardTitle>
          </CardHeader>
          <CardContent className="flex-1 min-h-0 pt-0">
            {!selectedId && (
              <div className="flex h-full flex-col items-center justify-center gap-4 text-muted-foreground">
                <div className="text-center space-y-2">
                  <p className="text-sm font-medium">{t('sessionSelectHint')}</p>
                  {sortedSessions.length === 0 && (
                    <Button variant="outline" size="sm" onClick={() => navigate('/chat')} className="mt-2">
                      <MessageSquare className="mr-1.5 h-3.5 w-3.5" />
                      {t('sessionsStartChat')}
                    </Button>
                  )}
                </div>
              </div>
            )}

            {selectedId && isDetailLoading && !detail && (
              <div className="h-full flex items-center justify-center text-muted-foreground text-sm">
                {t('sessionDetailLoading')}
              </div>
            )}

            {selectedId && isDetailError && !detail && (
              <div className="h-full flex items-center justify-center text-destructive text-sm">
                {t('sessionDetailLoadFailed')}
              </div>
            )}

            {detail && (
              <div className="h-full flex flex-col gap-4 min-h-0">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                  <div>
                    <div className="text-xs text-muted-foreground mb-1">
                      {t('sessionIdLabel')}
                    </div>
                    <div className="font-mono break-all">{detail.id}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground mb-1">
                      {t('sessionMessageCountLabel')}
                    </div>
                    <div>{detail.message_count}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground mb-1">
                      {t('sessionCreatedAtLabel')}
                    </div>
                    <div>{formatDateTime(detail.created_at)}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground mb-1">
                      {t('sessionUpdatedAtLabel')}
                    </div>
                    <div>{formatDateTime(detail.updated_at)}</div>
                  </div>
                </div>

                <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
                  <div className="flex-1">
                    <label className="text-xs text-muted-foreground mb-1 block">
                      {t('sessionSummaryLabel')}
                    </label>
                    <Input
                      className="h-11"
                      value={summaryDraft}
                      onChange={(e) => setSummaryDraft(e.target.value)}
                      placeholder={t('sessionSummaryPlaceholder')}
                    />
                  </div>
                  <div className="flex gap-2 sm:shrink-0">
                    <Button
                      onClick={handleSaveSummary}
                      disabled={
                        updateSummary.isPending || summaryDraft === detail.summary
                      }
                      className="h-11 flex-1 sm:flex-initial"
                    >
                      <Save className="h-4 w-4 mr-1.5" />
                      {t('save')}
                    </Button>
                    <Button
                      variant="destructive"
                      onClick={handleDeleteSession}
                      disabled={deleteSession.isPending}
                      className="h-11 flex-1 sm:flex-initial"
                    >
                      <Trash2 className="h-4 w-4 mr-1.5" />
                      {t('delete')}
                    </Button>
                  </div>
                </div>

                <div className="flex-1 min-h-0 rounded-md border">
                  <div className="px-3 py-2 border-b text-sm font-medium">
                    {t('sessionMessagesTitle')}
                  </div>
                  <ScrollArea className="h-[calc(100%-2.5rem)]">
                    <div className="p-3 space-y-3">
                      {detail.messages.length === 0 && (
                        <div className="text-sm text-muted-foreground text-center py-6">
                          {t('sessionNoMessages')}
                        </div>
                      )}

                      {detail.messages.map((msg, idx) => (
                        <div key={`${msg.tool_call_id || 'msg'}-${idx}`} className="rounded-md border p-3">
                          <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
                            <span className="text-xs uppercase tracking-wide text-muted-foreground">
                              {msg.role || '-'}
                            </span>
                            {msg.tool_call_id && (
                              <span className="text-xs text-muted-foreground font-mono">
                                tool_call_id: {msg.tool_call_id}
                              </span>
                            )}
                          </div>
                          <pre className="text-sm whitespace-pre-wrap break-words font-sans">
                            {msg.content || ''}
                          </pre>
                        </div>
                      ))}
                    </div>
                  </ScrollArea>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
      </div>

      {/* Delete Confirmation Dialog */}
      <Dialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <DialogPortal>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('deleteConfirmTitle')}</DialogTitle>
              <DialogDescription>
                {t('deleteConfirmDescription')}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowDeleteConfirm(false)}>
                {t('cancel')}
              </Button>
              <Button
                variant="destructive"
                onClick={confirmDeleteSession}
                disabled={deleteSession.isPending}
              >
                {deleteSession.isPending ? (
                  <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
                ) : (
                  <Trash2 className="h-4 w-4 mr-1.5" />
                )}
                {t('delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </DialogPortal>
      </Dialog>
    </>
  );
}
