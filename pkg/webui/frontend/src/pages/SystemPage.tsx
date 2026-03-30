import { t } from '@/lib/i18n';
import { useReloadService, useRestartService, useServiceStatus, useStatus } from '@/hooks/useConfig';
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
  const reloadService = useReloadService();
  const statusRecord = status as Record<string, unknown> | undefined;
  const serviceInstalled = service?.installed ?? false;
  const serviceStatus = service?.status ?? 'unknown';

  return (
    <div className="system-page flex h-full flex-col">
      <Header title={t('tabStatus')} />
      <div className="flex flex-wrap items-center gap-2 pb-4">
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
          {t('systemQMDButton')}
        </Button>
      </div>

      <ScrollArea className="flex-1 pb-6">
        <div className="space-y-4">
          {!isLoading ? (
            <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t('systemRuntimePathsTitle')}</div>
                <h3 className="mt-2 text-lg font-semibold text-foreground">
                  {t('systemRuntimePathsHeadline')}
                </h3>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  {t('systemRuntimePathsDescription')}
                </p>
              </div>
              <div className="mt-4 grid gap-3 md:grid-cols-2">
                <StatusMetric label={t('systemConfigPath')} value={String(statusRecord?.config_path ?? '-')} />
                <StatusMetric label={t('systemDatabaseDir')} value={String(statusRecord?.database_dir ?? '-')} />
                <StatusMetric label={t('systemRuntimeDatabase')} value={String(statusRecord?.runtime_db_path ?? '-')} />
                <StatusMetric label={t('agentsWorkspace')} value={String(statusRecord?.workspace_path ?? '-')} />
              </div>
            </Card>
          ) : null}

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t('systemServiceTitle')}</div>
                <h3 className="mt-2 text-lg font-semibold text-foreground">
                  {t('systemServiceHeadline')}
                </h3>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  {t('systemServiceDescription')}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => reloadService.mutate()}
                  disabled={reloadService.isPending}
                >
                  <RefreshCw className={cn('mr-2 h-4 w-4', reloadService.isPending && 'animate-spin')} />
                  {reloadService.isPending ? t('systemServiceReloading') : t('systemServiceReload')}
                </Button>
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

                <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
                  <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t('systemServiceConfigPath')}</div>
                  <div className="mt-2 break-all font-mono text-sm text-foreground">{service?.config_path || '-'}</div>
                  <div className="mt-3">
                    <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t('systemServiceArguments')}</div>
                    <div className="mt-2 break-all font-mono text-sm text-foreground">
                      {service?.arguments && service.arguments.length > 0 ? service.arguments.join(' ') : '-'}
                    </div>
                  </div>
                  {!serviceInstalled ? (
                    <div className="mt-3 rounded-xl border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
                      {t('systemServiceNotInstalledHint')}
                    </div>
                  ) : null}
                  {serviceInstalled ? (
                    <div className="mt-3 rounded-xl border border-border/70 bg-background/70 px-3 py-2 text-sm text-foreground/80">
                      {t('systemServiceReloadHint')}
                    </div>
                  ) : null}
                  {serviceInstalled && serviceStatus !== 'running' ? (
                    <div className="mt-3 rounded-xl border border-border/70 bg-background/70 px-3 py-2 text-sm text-foreground/80">
                      {t('systemServiceRestartHint')}
                    </div>
                  ) : null}
                </div>
              </div>
            )}
          </Card>

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t('systemQMDTitle')}</div>
                <h3 className="mt-2 text-lg font-semibold text-foreground">
                  {t('systemQMDHeadline')}
                </h3>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  {t('systemQMDDescription')}
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
                  {installQMD.isPending ? t('marketplaceInstalling') : t('systemQMDInstall')}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => updateQMD.mutate()}
                  disabled={updateQMD.isPending || !qmd?.available}
                >
                  <RefreshCw className={cn('mr-2 h-4 w-4', updateQMD.isPending && 'animate-spin')} />
                  {updateQMD.isPending ? t('systemUpdating') : t('systemQMDUpdate')}
                </Button>
              </div>
            </div>

            {qmdLoading ? (
              <div className="text-muted-foreground py-8 text-center animate-pulse">{t('systemLoading')}</div>
            ) : (
              <div className="mt-4 space-y-4">
                <div className="grid gap-3 md:grid-cols-4">
                  <StatusMetric label={t('enabled')} value={qmd?.enabled ? t('systemYes') : t('systemNo')} />
                  <StatusMetric label={t('systemAvailable')} value={qmd?.available ? t('systemYes') : t('systemNo')} />
                  <StatusMetric label={t('marketplaceVersion')} value={qmd?.version || '-'} />
                  <StatusMetric label={t('systemCollections')} value={String(qmd?.collections.length ?? 0)} />
                </div>

                <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
                  <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t('systemCommand')}</div>
                  <div className="mt-2 font-mono text-sm text-foreground">{qmd?.command || '-'}</div>
                  <div className="mt-3 grid gap-3 md:grid-cols-3">
                    <StatusMetric label={t('systemResolvedCommand')} value={qmd?.resolved_command || '-'} />
                    <StatusMetric label={t('systemCommandSource')} value={qmd?.command_source || '-'} />
                    <StatusMetric label={t('systemPersistentCommand')} value={qmd?.persistent_command || '-'} />
                  </div>
                  {qmd?.error ? (
                    <div className="mt-3 rounded-xl border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
                      {qmd.error}
                    </div>
                  ) : null}
                </div>

                <div className="space-y-3">
                  {(qmd?.collections ?? []).map((collection) => (
                    <div key={`${collection.Name}-${collection.Path}`} className="rounded-2xl border border-border/70 bg-muted/35 p-4">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="rounded-full bg-primary px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-primary-foreground">
                          {collection.Name}
                        </span>
                        <span className="rounded-full bg-background/80 px-2.5 py-1 text-[11px] text-muted-foreground">
                          {collection.Pattern}
                        </span>
                      </div>
                      <div className="mt-3 break-all font-mono text-xs leading-6 text-foreground">
                        {collection.Path}
                      </div>
                    </div>
                  ))}
                  {(qmd?.collections ?? []).length === 0 ? (
                    <div className="rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                      {t('systemQMDNoCollections')}
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
    <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
      <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className="mt-2 break-all text-sm font-semibold text-foreground">{value}</div>
    </div>
  );
}
