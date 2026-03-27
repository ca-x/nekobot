import { t } from '@/lib/i18n';
import { useRestartService, useServiceStatus, useStatus } from '@/hooks/useConfig';
import { useInstallQMD, useQMDStatus, useUpdateQMD } from '@/hooks/useQMD';
import Header from '@/components/layout/Header';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { cn } from '@/lib/utils';
import { DatabaseZap, RefreshCw } from 'lucide-react';

export default function SystemPage() {
  const { data: status, isLoading, refetch, isFetching } = useStatus();
  const { data: service, isLoading: serviceLoading, refetch: refetchService, isFetching: serviceFetching } = useServiceStatus();
  const { data: qmd, isLoading: qmdLoading, refetch: refetchQMD, isFetching: qmdFetching } = useQMDStatus();
  const updateQMD = useUpdateQMD();
  const installQMD = useInstallQMD();
  const restartService = useRestartService();
  const statusRecord = status as Record<string, unknown> | undefined;
  const serviceInstalled = service?.installed ?? false;
  const serviceStatus = service?.status ?? 'unknown';

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
          onClick={() => refetchService()}
          disabled={serviceFetching}
        >
          <RefreshCw className={`h-4 w-4 mr-1 ${serviceFetching ? 'animate-spin' : ''}`} />
          {t('systemServiceButton')}
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
          {!isLoading ? (
            <Card className="rounded-[24px] border-slate-200/80 bg-white/95 p-5 shadow-sm">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-slate-400">Runtime Paths</div>
                <h3 className="mt-2 text-lg font-semibold text-slate-900">
                  Active bootstrap and storage targets.
                </h3>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  Use this to confirm which config file, runtime DB, and workspace the current process is pointing at.
                </p>
              </div>
              <div className="mt-4 grid gap-3 md:grid-cols-2">
                <StatusMetric label="Config path" value={String(statusRecord?.config_path ?? '-')} />
                <StatusMetric label="DB directory" value={String(statusRecord?.database_dir ?? '-')} />
                <StatusMetric label="Runtime DB" value={String(statusRecord?.runtime_db_path ?? '-')} />
                <StatusMetric label="Workspace" value={String(statusRecord?.workspace_path ?? '-')} />
              </div>
            </Card>
          ) : null}

          <Card className="rounded-[24px] border-slate-200/80 bg-white/95 p-5 shadow-sm">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-slate-400">{t('systemServiceTitle')}</div>
                <h3 className="mt-2 text-lg font-semibold text-slate-900">
                  {t('systemServiceHeadline')}
                </h3>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  {t('systemServiceDescription')}
                </p>
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={() => restartService.mutate()}
                disabled={restartService.isPending || !serviceInstalled}
              >
                <RefreshCw className={cn('mr-2 h-4 w-4', restartService.isPending && 'animate-spin')} />
                {restartService.isPending ? t('systemServiceRestarting') : t('systemServiceRestart')}
              </Button>
            </div>

            {serviceLoading ? (
              <div className="text-muted-foreground py-8 text-center animate-pulse">{t('systemLoading')}</div>
            ) : (
              <div className="mt-4 space-y-4">
                <div className="grid gap-3 md:grid-cols-4">
                  <StatusMetric label={t('systemServiceInstalled')} value={serviceInstalled ? t('systemYes') : t('systemNo')} />
                  <StatusMetric label={t('systemServiceStatus')} value={formatServiceStatus(serviceStatus)} />
                  <StatusMetric label={t('systemServicePlatform')} value={service?.platform || '-'} />
                  <StatusMetric label={t('systemServiceName')} value={service?.name || '-'} />
                </div>

                <div className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
                  <div className="text-xs font-medium uppercase tracking-[0.18em] text-slate-400">{t('systemServiceConfigPath')}</div>
                  <div className="mt-2 break-all font-mono text-sm text-slate-700">{service?.config_path || '-'}</div>
                  <div className="mt-3">
                    <div className="text-xs font-medium uppercase tracking-[0.18em] text-slate-400">{t('systemServiceArguments')}</div>
                    <div className="mt-2 break-all font-mono text-sm text-slate-700">
                      {service?.arguments && service.arguments.length > 0 ? service.arguments.join(' ') : '-'}
                    </div>
                  </div>
                  {!serviceInstalled ? (
                    <div className="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                      {t('systemServiceNotInstalledHint')}
                    </div>
                  ) : null}
                  {serviceInstalled && serviceStatus !== 'running' ? (
                    <div className="mt-3 rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700">
                      {t('systemServiceRestartHint')}
                    </div>
                  ) : null}
                </div>
              </div>
            )}
          </Card>

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
              <div className="text-muted-foreground py-8 text-center animate-pulse">{t('systemLoading')}</div>
            ) : (
              <div className="mt-4 space-y-4">
                <div className="grid gap-3 md:grid-cols-4">
                  <StatusMetric label="Enabled" value={qmd?.enabled ? t('systemYes') : t('systemNo')} />
                  <StatusMetric label="Available" value={qmd?.available ? t('systemYes') : t('systemNo')} />
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
            <div className="text-muted-foreground py-8 text-center animate-pulse">{t('systemLoading')}</div>
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

function formatServiceStatus(status: string) {
  switch (status) {
    case 'running':
      return t('systemServiceStatusRunning');
    case 'stopped':
      return t('systemServiceStatusStopped');
    case 'not_installed':
      return t('systemServiceStatusNotInstalled');
    default:
      return t('systemServiceStatusUnknown');
  }
}

function StatusMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
      <div className="text-[11px] uppercase tracking-[0.18em] text-slate-400">{label}</div>
      <div className="mt-2 break-all text-sm font-semibold text-slate-900">{value}</div>
    </div>
  );
}
