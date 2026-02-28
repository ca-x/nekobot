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
import { Save, Trash2 } from 'lucide-react';

function formatDateTime(value: string): string {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

export default function SessionsPage() {
  const { data: sessions = [], isLoading } = useSessions();
  const [selectedId, setSelectedId] = useState('');
  const [summaryDraft, setSummaryDraft] = useState('');

  const updateSummary = useUpdateSessionSummary();
  const deleteSession = useDeleteSession();

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
    if (!window.confirm(t('sessionDeleteConfirm'))) return;

    const targetId = detail.id;
    deleteSession.mutate(targetId, {
      onSuccess: () => {
        setSelectedId((current) => (current === targetId ? '' : current));
      },
    });
  };

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)]">
      <Header
        title={t('tabSessions')}
        description={t('sessionsPageDescription')}
      />

      <div className="flex flex-1 min-h-0 gap-4">
        <Card className="w-80 min-h-0 flex flex-col">
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
                  <div className="text-sm text-muted-foreground text-center py-8">
                    {t('sessionsEmpty')}
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
                      <div className="text-xs text-muted-foreground truncate mb-1">
                        {item.id}
                      </div>
                      <div className="text-sm font-medium truncate mb-1">
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
              <div className="h-full flex items-center justify-center text-muted-foreground text-sm">
                {t('sessionSelectHint')}
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

                <div className="flex items-end gap-2">
                  <div className="flex-1">
                    <label className="text-xs text-muted-foreground mb-1 block">
                      {t('sessionSummaryLabel')}
                    </label>
                    <Input
                      value={summaryDraft}
                      onChange={(e) => setSummaryDraft(e.target.value)}
                      placeholder={t('sessionSummaryPlaceholder')}
                    />
                  </div>
                  <Button
                    onClick={handleSaveSummary}
                    disabled={
                      updateSummary.isPending || summaryDraft === detail.summary
                    }
                  >
                    <Save className="h-4 w-4 mr-1.5" />
                    {t('save')}
                  </Button>
                  <Button
                    variant="destructive"
                    onClick={handleDeleteSession}
                    disabled={deleteSession.isPending}
                  >
                    <Trash2 className="h-4 w-4 mr-1.5" />
                    {t('delete')}
                  </Button>
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
  );
}
