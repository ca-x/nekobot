import { useEffect, useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { getToken } from '@/api/client';
import { t } from '@/lib/i18n';
import { toast } from '@/lib/notify';
import { cn } from '@/lib/utils';
import { useRuntimeAgents } from '@/hooks/useTopology';
import { describeCollaborationTarget, useCollaborationTargets } from '@/hooks/useCollaborationTargets';
import { useSaveMessage, useSavedMessages, useThreadDetail, useThreads, useUnsaveMessage, useUpdateThread } from '@/hooks/useThreads';
import { Bookmark, Save, MessageSquare, Paperclip, Route } from 'lucide-react';
import { useNavigate } from 'react-router-dom';

function formatDateTime(value: string): string {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

export default function ThreadsPage() {
  const navigate = useNavigate();
  const { data: threads = [], isLoading } = useThreads();
  const { data: runtimes = [] } = useRuntimeAgents();
  const { optionMap } = useCollaborationTargets();
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
  const detailTarget = detail?.target || (detail?.id ? `#websocket:${detail.id}` : '');
  const detailTargetInfo = describeCollaborationTarget(detailTarget, optionMap);
  const { data: savedMessages } = useSavedMessages(detailTarget || null);
  const saveMessage = useSaveMessage();
  const unsaveMessage = useUnsaveMessage();
  const savedMessageIds = useMemo(() => new Set((savedMessages?.saved_messages ?? []).map((item) => item.message_id)), [savedMessages]);

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
    navigate(`/chat?thread=${encodeURIComponent(detail.id)}`);
  };

  const handleOpenAttachment = async (attachmentId: string, filename: string, target: string) => {
    const token = getToken();
    const resp = await fetch(`/api/daemon/attachments/${encodeURIComponent(attachmentId)}?target=${encodeURIComponent(target)}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    });
    if (!resp.ok) {
      throw new Error(await resp.text() || resp.statusText);
    }
    const blob = await resp.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    link.download = filename || attachmentId;
    link.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
  };

  const handleToggleSavedMessage = (messageId: string, saved: boolean) => {
    if (!detailTarget || !messageId) return;
    const request_id = `webui-save-${messageId}-${Date.now()}`;
    if (saved) {
      unsaveMessage.mutate({ target: detailTarget, message_id: messageId, request_id });
    } else {
      saveMessage.mutate({ target: detailTarget, message_id: messageId, request_id });
    }
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
                {sortedThreads.map((item) => {
                  const targetInfo = describeCollaborationTarget(item.target || `#websocket:${item.id}`, optionMap);
                  return (
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
                    <div className="mb-1 flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
                      <Route className="h-3 w-3 shrink-0" />
                      <span className="truncate" title={`${targetInfo.label} · ${targetInfo.target}`}>
                        {targetInfo.label}
                      </span>
                    </div>
                    <div className="text-xs text-muted-foreground">{t('sessionUpdatedAt', formatDateTime(item.updated_at))}</div>
                  </button>
                  );
                })}
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
                  <div className="sm:col-span-2 rounded-md border border-dashed border-border/70 bg-muted/25 px-3 py-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="inline-flex h-7 w-7 items-center justify-center rounded-md bg-background">
                        <Route className="h-3.5 w-3.5 text-muted-foreground" />
                      </div>
                      <div className="min-w-0">
                        <div className="text-xs text-muted-foreground">{t('collaborationTargetBound')}</div>
                        <div className="truncate text-sm font-medium" title={`${detailTargetInfo.label} · ${detailTargetInfo.target}`}>
                          {detailTargetInfo.label}
                        </div>
                      </div>
                      <div className="ml-auto max-w-full rounded-full bg-background px-2.5 py-1 font-mono text-[11px] text-muted-foreground">
                        {detailTargetInfo.target}
                      </div>
                    </div>
                    <div className="mt-2 text-xs leading-5 text-muted-foreground">
                      {t('collaborationThreadTargetHint')}
                    </div>
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
                            {msg.id ? (
                              <Button
                                type="button"
                                variant={savedMessageIds.has(msg.id) ? 'default' : 'outline'}
                                size="sm"
                                className="h-8 gap-1.5"
                                disabled={saveMessage.isPending || unsaveMessage.isPending}
                                onClick={() => handleToggleSavedMessage(msg.id!, savedMessageIds.has(msg.id!))}
                              >
                                <Bookmark className="h-3.5 w-3.5" />
                                {savedMessageIds.has(msg.id) ? t('messageSaved') : t('saveMessage')}
                              </Button>
                            ) : null}
                          </div>
                          <pre className="text-sm whitespace-pre-wrap break-words font-sans">{msg.content || ''}</pre>
                          {msg.attachments && msg.attachments.length > 0 ? (
                            <div className="mt-3 flex flex-wrap gap-2">
                              {msg.attachments.map((attachment) => (
                                <Button
                                  key={attachment.attachment_id}
                                  type="button"
                                  variant="outline"
                                  size="sm"
                                  className="h-auto min-h-8 gap-2 whitespace-normal px-2 py-1 text-xs"
                                  onClick={() => {
                                    void handleOpenAttachment(attachment.attachment_id, attachment.filename, detailTarget).catch((error) => {
                                      toast.error(error instanceof Error ? error.message : 'Failed to open attachment');
                                    });
                                  }}
                                >
                                  <Paperclip className="h-3.5 w-3.5 shrink-0" />
                                  <span className="max-w-[14rem] truncate">{attachment.filename || attachment.attachment_id}</span>
                                  <span className="shrink-0 text-muted-foreground">{formatBytes(attachment.size_bytes)}</span>
                                </Button>
                              ))}
                            </div>
                          ) : null}
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
