import { t } from '@/lib/i18n';
import { useStatus } from '@/hooks/useConfig';
import { useInstallQMD, useQMDStatus, useUpdateQMD } from '@/hooks/useQMD';
import Header from '@/components/layout/Header';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { cn } from '@/lib/utils';
import { DatabaseZap, RefreshCw } from 'lucide-react';

export default function SystemPage() {
  const { data: status, isLoading, refetch, isFetching } = useStatus();
  const { data: qmd, isLoading: qmdLoading, refetch: refetchQMD, isFetching: qmdFetching } = useQMDStatus();
  const updateQMD = useUpdateQMD();
  const installQMD = useInstallQMD();

  return (
    <div className="flex flex-col h-full">
      <Header title={t('tabStatus')} />
      <div className="flex items-center gap-2 px-6 pb-4">
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetch()}
          disabled={isFetching}
        >
          <RefreshCw className={`h-4 w-4 mr-1 ${isFetching ? 'animate-spin' : ''}`} />
          {t('refresh')}
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetchQMD()}
          disabled={qmdFetching}
        >
          <DatabaseZap className={`h-4 w-4 mr-1 ${qmdFetching ? 'animate-spin' : ''}`} />
          QMD
        </Button>
      </div>

      <ScrollArea className="flex-1 px-6 pb-6">
        <div className="space-y-4">
          <Card className="rounded-[24px] border-slate-200/80 bg-white/95 p-5 shadow-sm">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-slate-400">QMD</div>
                <h3 className="mt-2 text-lg font-semibold text-slate-900">
                  Optional semantic search runtime.
                </h3>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  This reflects actual runtime availability, not just saved config.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => installQMD.mutate()}
                  disabled={installQMD.isPending}
                >
                  <RefreshCw className={cn('mr-2 h-4 w-4', installQMD.isPending && 'animate-spin')} />
                  {installQMD.isPending ? 'Installing…' : 'Install persisted QMD'}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => updateQMD.mutate()}
                  disabled={updateQMD.isPending || !qmd?.available}
                >
                  <RefreshCw className={cn('mr-2 h-4 w-4', updateQMD.isPending && 'animate-spin')} />
                  {updateQMD.isPending ? 'Updating…' : 'Update QMD'}
                </Button>
              </div>
            </div>

            {qmdLoading ? (
              <div className="text-muted-foreground py-8 text-center animate-pulse">Loading…</div>
            ) : (
              <div className="mt-4 space-y-4">
                <div className="grid gap-3 md:grid-cols-4">
                  <StatusMetric label="Enabled" value={qmd?.enabled ? 'Yes' : 'No'} />
                  <StatusMetric label="Available" value={qmd?.available ? 'Yes' : 'No'} />
                  <StatusMetric label="Version" value={qmd?.version || '-'} />
                  <StatusMetric label="Collections" value={String(qmd?.collections.length ?? 0)} />
                </div>

                <div className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
                  <div className="text-xs font-medium uppercase tracking-[0.18em] text-slate-400">Command</div>
                  <div className="mt-2 font-mono text-sm text-slate-700">{qmd?.command || '-'}</div>
                  <div className="mt-3 grid gap-3 md:grid-cols-3">
                    <StatusMetric label="Resolved command" value={qmd?.resolved_command || '-'} />
                    <StatusMetric label="Command source" value={qmd?.command_source || '-'} />
                    <StatusMetric label="Persistent command" value={qmd?.persistent_command || '-'} />
                  </div>
                  {qmd?.error ? (
                    <div className="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                      {qmd.error}
                    </div>
                  ) : null}
                </div>

                <div className="space-y-3">
                  {(qmd?.collections ?? []).map((collection) => (
                    <div key={`${collection.Name}-${collection.Path}`} className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="rounded-full bg-slate-900 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-white">
                          {collection.Name}
                        </span>
                        <span className="rounded-full bg-white px-2.5 py-1 text-[11px] text-slate-600">
                          {collection.Pattern}
                        </span>
                      </div>
                      <div className="mt-3 break-all font-mono text-xs leading-6 text-slate-700">
                        {collection.Path}
                      </div>
                    </div>
                  ))}
                  {(qmd?.collections ?? []).length === 0 ? (
                    <div className="rounded-2xl border border-dashed border-slate-200 px-4 py-6 text-sm text-slate-500">
                      No QMD collections initialized.
                    </div>
                  ) : null}
                </div>
              </div>
            )}
          </Card>

          {isLoading ? (
            <div className="text-muted-foreground py-8 text-center animate-pulse">Loading…</div>
          ) : (
            <pre className="rounded-lg border border-border bg-card p-4 text-sm font-mono overflow-auto whitespace-pre-wrap break-words">
              {JSON.stringify(status, null, 2)}
            </pre>
          )}
        </div>
      </ScrollArea>
    </div>
  );
}

function StatusMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
      <div className="text-[11px] uppercase tracking-[0.18em] text-slate-400">{label}</div>
      <div className="mt-2 break-all text-sm font-semibold text-slate-900">{value}</div>
    </div>
  );
}
