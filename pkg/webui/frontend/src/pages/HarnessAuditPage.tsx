import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ShieldCheck, RefreshCw, Trash2, Clock3, CheckCircle2, CircleAlert, Link2 } from 'lucide-react';

import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useClearHarnessAudit, useHarnessAudit } from '@/hooks/useConfig';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';

function formatTimestamp(value?: string): string {
  if (!value) {
    return '-';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function formatBytes(value?: number): string {
  if (!value || value <= 0) {
    return '0 B';
  }
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

export default function HarnessAuditPage() {
  const [limitInput, setLimitInput] = useState('100');
  const limit = useMemo(() => {
    const parsed = Number(limitInput);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      return 100;
    }
    return Math.min(parsed, 500);
  }, [limitInput]);

  const { data, isLoading, refetch, isFetching } = useHarnessAudit(limit);
  const clearAudit = useClearHarnessAudit();
  const entries = data?.entries ?? [];

  const successCount = entries.filter((entry) => entry.success).length;
  const failureCount = entries.length - successCount;
  const successRate = entries.length
    ? Math.round((successCount / entries.length) * 100)
    : 0;

  const handleClear = async () => {
    if (!window.confirm(t('harnessAuditClearConfirm'))) {
      return;
    }
    await clearAudit.mutateAsync();
  };

  return (
    <div className="space-y-6">
      <Header
        title={t('harnessAuditTitle')}
        description={t('harnessAuditDescription')}
      />

      <div className="grid gap-4 md:grid-cols-3">
        <Card className="border-border/70 bg-card/92 shadow-sm">
          <CardHeader className="pb-3">
            <CardDescription>{t('harnessAuditMetricEntries')}</CardDescription>
            <CardTitle className="mono-data text-3xl">{String(data?.stats.entries ?? 0)}</CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            {t('harnessAuditMetricEntriesHint', String(limit))}
          </CardContent>
        </Card>

        <Card className="border-border/70 bg-card/92 shadow-sm">
          <CardHeader className="pb-3">
            <CardDescription>{t('harnessAuditMetricSuccessRate')}</CardDescription>
            <CardTitle className="mono-data text-3xl">{`${successRate}%`}</CardTitle>
          </CardHeader>
          <CardContent className="flex items-center gap-3 text-xs text-muted-foreground">
            <span>{t('harnessAuditSuccessCount', String(successCount))}</span>
            <span>{t('harnessAuditFailureCount', String(failureCount))}</span>
          </CardContent>
        </Card>

        <Card className="border-border/70 bg-card/92 shadow-sm">
          <CardHeader className="pb-3">
            <CardDescription>{t('harnessAuditMetricStorage')}</CardDescription>
            <CardTitle className="mono-data text-3xl">{formatBytes(data?.stats.size)}</CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            {data?.stats.modified ? t('harnessAuditModifiedAt', formatTimestamp(data.stats.modified)) : t('harnessAuditNoLogYet')}
          </CardContent>
        </Card>
      </div>

      <Card className="border-border/70 bg-card/92 shadow-sm">
        <CardHeader className="pb-4">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <div className="eyebrow-label inline-flex items-center gap-2 rounded-full bg-[hsl(var(--brand-100))] px-3 py-1 text-[hsl(var(--brand-800))] dark:text-[hsl(var(--brand-100))]">
                <ShieldCheck className="h-3.5 w-3.5" />
                {t('tabHarnessAudit')}
              </div>
              <CardTitle className="mt-3 text-xl">{t('harnessAuditConsoleTitle')}</CardTitle>
              <CardDescription className="mt-2">{t('harnessAuditConsoleDescription')}</CardDescription>
            </div>
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <div className="w-full sm:w-[160px]">
                <label className="eyebrow-label mb-2 block text-muted-foreground" htmlFor="harness-audit-limit">
                  {t('harnessAuditMetricEntries')}
                </label>
                <Input
                  id="harness-audit-limit"
                  type="number"
                  min={1}
                  max={500}
                  value={limitInput}
                  onChange={(event) => setLimitInput(event.target.value)}
                  className="mono-data h-11 rounded-xl bg-background"
                />
              </div>
              <Button type="button" variant="outline" className="h-11 rounded-xl" onClick={() => refetch()} disabled={isFetching}>
                <RefreshCw className={cn('mr-2 h-4 w-4', isFetching && 'animate-spin')} />
                {t('refresh')}
              </Button>
              <Button
                type="button"
                variant="outline"
                className="h-11 rounded-xl border-destructive/40 text-destructive hover:bg-destructive/10"
                onClick={handleClear}
                disabled={clearAudit.isPending}
              >
                <Trash2 className={cn('mr-2 h-4 w-4', clearAudit.isPending && 'animate-pulse')} />
                {t('harnessAuditClear')}
              </Button>
            </div>
          </div>
        </CardHeader>

        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-3">
            <div className="rounded-2xl border border-border/70 bg-card/80 p-4">
              <div className="eyebrow-label text-muted-foreground">{t('harnessAuditLogFile')}</div>
              <div className="mono-data mt-2 break-all text-sm text-foreground">{data?.stats.file || '-'}</div>
            </div>
            <div className="rounded-2xl border border-border/70 bg-card/80 p-4">
              <div className="eyebrow-label text-muted-foreground">{t('harnessAuditLastUpdated')}</div>
              <div className="mono-data mt-2 text-sm text-foreground">{formatTimestamp(data?.stats.modified)}</div>
            </div>
            <div className="rounded-2xl border border-border/70 bg-card/80 p-4">
              <div className="eyebrow-label text-muted-foreground">{t('harnessAuditLinks')}</div>
              <div className="mt-2 flex flex-wrap gap-2">
                <Button asChild variant="outline" className="rounded-full">
                  <Link to="/chat">
                    <Link2 className="mr-2 h-4 w-4" />
                    {t('tabChat')}
                  </Link>
                </Button>
                <Button asChild variant="outline" className="rounded-full">
                  <Link to="/config">
                    <Link2 className="mr-2 h-4 w-4" />
                    {t('tabConfig')}
                  </Link>
                </Button>
              </div>
            </div>
          </div>

          <ScrollArea className="max-h-[68vh] rounded-[1.5rem] border border-border/70 bg-card/70">
            <div className="divide-y divide-border/70">
              {isLoading ? (
                <div className="p-6 text-sm text-muted-foreground">{t('loading')}</div>
              ) : entries.length ? (
                entries.map((entry, index) => (
                  <div key={`${entry.ts}-${entry.tool}-${index}`} className="p-4 sm:p-5">
                    <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                      <div className="min-w-0 space-y-3">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-background px-3 py-1 text-xs font-medium text-foreground">
                            {entry.success ? <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-300" /> : <CircleAlert className="h-3.5 w-3.5 text-rose-600 dark:text-rose-300" />}
                            {entry.tool}
                          </span>
                          <span className="mono-data inline-flex items-center gap-1 rounded-full bg-muted px-2.5 py-1 text-xs text-muted-foreground">
                            <Clock3 className="h-3.5 w-3.5" />
                            {t('harnessAuditDuration', String(entry.duration_ms))}
                          </span>
                          {entry.session_id && (
                            <span className="rounded-full bg-[hsl(var(--brand-50))] px-2.5 py-1 text-xs text-[hsl(var(--brand-800))] dark:text-[hsl(var(--brand-100))]">
                              {t('harnessAuditSession', entry.session_id)}
                            </span>
                          )}
                        </div>

                        <div className="mono-data text-xs text-muted-foreground">
                          {formatTimestamp(entry.ts)}
                        </div>

                        {entry.result_preview && (
                          <div className="rounded-2xl border border-emerald-200/60 bg-emerald-50/70 p-3 text-sm text-emerald-950 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-100 whitespace-pre-wrap break-words">
                            {entry.result_preview}
                          </div>
                        )}

                        {entry.error && (
                          <div className="rounded-2xl border border-rose-200/70 bg-rose-50/80 p-3 text-sm text-rose-950 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-100 whitespace-pre-wrap break-words">
                            {entry.error}
                          </div>
                        )}

                        {entry.args && Object.keys(entry.args).length > 0 && (
                          <pre className="overflow-x-auto rounded-2xl border border-border/70 bg-[hsl(var(--gray-950))] px-4 py-3 text-xs leading-6 text-slate-100">
                            {JSON.stringify(entry.args, null, 2)}
                          </pre>
                        )}
                      </div>
                    </div>
                  </div>
                ))
              ) : (
                <div className="p-8 text-center">
                  <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-[hsl(var(--brand-100))] text-[hsl(var(--brand-700))]">
                    <ShieldCheck className="h-6 w-6" />
                  </div>
                  <div className="mt-4 text-lg font-semibold text-foreground">{t('harnessAuditEmptyTitle')}</div>
                  <div className="mt-2 text-sm text-muted-foreground">{t('harnessAuditEmptyDescription')}</div>
                </div>
              )}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>
    </div>
  );
}
