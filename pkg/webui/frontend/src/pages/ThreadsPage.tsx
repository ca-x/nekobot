import { useEffect, useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { useRuntimeAgents } from '@/hooks/useTopology';
import { useThreadDetail, useThreads, useUpdateThread } from '@/hooks/useThreads';
import { Save, MessageSquare } from 'lucide-react';
import { useNavigate } from 'react-router-dom';

function formatDateTime(value: string): string {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

export default function ThreadsPage() {
  const navigate = useNavigate();
  const { data: threads = [], isLoading } = useThreads();
  const { data: runtimes = [] } = useRuntimeAgents();
  const updateThread = useUpdateThread();
  const [selectedId, setSelectedId] = useState('');
  const [summaryDraft, setSummaryDraft] = useState('');
  const [runtimeDraft, setRuntimeDraft] = useState('');
  const [topicDraft, setTopicDraft] = useState('');

  const sortedThreads = useMemo(
    () => [...threads].sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()),
    [threads],
  );

  useEffect(() => {
    if (!selectedId && sortedThreads.length > 0) {
      setSelectedId(sortedThreads[0].id);
    }
  }, [selectedId, sortedThreads]);

  const { data: detail } = useThreadDetail(selectedId || null);

  useEffect(() => {
    if (!detail) return;
    setSummaryDraft(detail.summary || '');
    setRuntimeDraft(detail.runtime_id || '');
    setTopicDraft(detail.topic || '');
  }, [detail]);

  const handleSave = () => {
    if (!detail) return;
    updateThread.mutate({
      id: detail.id,
      summary: summaryDraft,
      runtime_id: runtimeDraft,
      topic: topicDraft,
    });
  };

  const handleOpenInChat = () => {
    if (!detail) return;
    sessionStorage.setItem(
      'nekobot_chat_thread_handoff',
      JSON.stringify({
        runtime_id: detail.runtime_id || '',
      }),
    );
    navigate('/chat');
  };

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)]">
      <Header title={t('tabThreads')} description={t('threadsPageDescription')} />
      <div className="flex flex-1 min-h-0 flex-col gap-4 lg:flex-row">
        <Card className="w-full lg:w-80 min-h-0 flex flex-col shrink-0">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t('tabThreads')}</CardTitle>
            <CardDescription>{t('threadListCount', String(sortedThreads.length))}</CardDescription>
          </CardHeader>
          <CardContent className="flex-1 min-h-0 p-0">
            <ScrollArea className="h-full">
              <div className="p-3 space-y-2">
                {isLoading && sortedThreads.length === 0 ? (
                  <div className="text-sm text-muted-foreground text-center py-8">{t('threadsLoading')}</div>
                ) : null}
                {!isLoading && sortedThreads.length === 0 ? (
                  <div className="space-y-4 py-8 text-center">
                    <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-muted">
                      <MessageSquare className="h-5 w-5 text-muted-foreground" />
                    </div>
                    <div>
                      <p className="text-sm font-medium">{t('threadsEmpty')}</p>
                      <p className="text-xs text-muted-foreground mt-1">{t('threadsEmptyHint')}</p>
                    </div>
                  </div>
                ) : null}
                {sortedThreads.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setSelectedId(item.id)}
                    className={cn(
                      'w-full rounded-md border text-left p-3 transition-colors',
                      item.id === selectedId ? 'border-primary/40 bg-primary/5' : 'hover:bg-muted/40 border-border',
                    )}
                  >
                    <div className="text-xs text-muted-foreground truncate mb-1" title={item.id}>
                      {item.id}
                    </div>
                    <div className="text-sm font-medium truncate mb-1" title={item.topic || item.summary || item.id}>
                      {item.topic || item.summary || item.id}
                    </div>
                    <div className="text-xs text-muted-foreground">{t('sessionUpdatedAt', formatDateTime(item.updated_at))}</div>
                  </button>
                ))}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>

        <Card className="flex-1 min-h-0 flex flex-col">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t('threadDetailsTitle')}</CardTitle>
          </CardHeader>
          <CardContent className="flex-1 min-h-0 pt-0">
            {!detail ? (
              <div className="h-full flex items-center justify-center text-muted-foreground text-sm">
                {t('threadSelectHint')}
              </div>
            ) : (
              <div className="h-full flex flex-col gap-4 min-h-0">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                  <div>
                    <div className="text-xs text-muted-foreground mb-1">{t('sessionIdLabel')}</div>
                    <div className="font-mono break-all">{detail.id}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground mb-1">{t('sessionMessageCountLabel')}</div>
                    <div>{detail.message_count}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground mb-1">{t('sessionCreatedAtLabel')}</div>
                    <div>{formatDateTime(detail.created_at)}</div>
                  </div>
                  <div>
                    <div className="text-xs text-muted-foreground mb-1">{t('sessionUpdatedAtLabel')}</div>
                    <div>{formatDateTime(detail.updated_at)}</div>
                  </div>
                </div>

                <div className="flex flex-col gap-2">
                  <label className="text-xs text-muted-foreground mb-1 block">{t('sessionThreadTopicLabel')}</label>
                  <Input value={topicDraft} onChange={(e) => setTopicDraft(e.target.value)} placeholder={t('sessionThreadTopicPlaceholder')} />
                </div>

                <div className="flex flex-col gap-2">
                  <label className="text-xs text-muted-foreground mb-1 block">{t('sessionSummaryLabel')}</label>
                  <Input value={summaryDraft} onChange={(e) => setSummaryDraft(e.target.value)} placeholder={t('sessionSummaryPlaceholder')} />
                </div>

                <div className="flex flex-col gap-2">
                  <label className="text-xs text-muted-foreground mb-1 block">{t('sessionRuntimeLabel')}</label>
                  <select
                    className="h-11 rounded-md border border-input bg-background px-3 text-sm"
                    value={runtimeDraft}
                    onChange={(e) => setRuntimeDraft(e.target.value)}
                  >
                    <option value="">{t('sessionRuntimeNone')}</option>
                    {runtimes.filter((runtime) => runtime.enabled).map((runtime) => (
                      <option key={runtime.id} value={runtime.id}>
                        {runtime.display_name || runtime.name} ({runtime.id})
                      </option>
                    ))}
                  </select>
                </div>

                <div className="flex gap-2">
                  <Button onClick={handleSave} disabled={updateThread.isPending}>
                    <Save className="h-4 w-4 mr-1.5" />
                    {t('save')}
                  </Button>
                  <Button variant="outline" onClick={handleOpenInChat}>
                    <MessageSquare className="h-4 w-4 mr-1.5" />
                    {t('threadsOpenInChat')}
                  </Button>
                </div>

                <div className="flex-1 min-h-0 rounded-md border">
                  <div className="px-3 py-2 border-b text-sm font-medium">{t('sessionMessagesTitle')}</div>
                  <ScrollArea className="h-[calc(100%-2.5rem)]">
                    <div className="p-3 space-y-3">
                      {detail.messages.map((msg, idx) => (
                        <div key={`${msg.tool_call_id || 'msg'}-${idx}`} className="rounded-md border p-3">
                          <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
                            <span className="text-xs uppercase tracking-wide text-muted-foreground">{msg.role || '-'}</span>
                          </div>
                          <pre className="text-sm whitespace-pre-wrap break-words font-sans">{msg.content || ''}</pre>
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
