import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  ArrowLeft,
  CheckCircle2,
  Loader2,
  PauseCircle,
  PlayCircle,
  ShieldAlert,
  StopCircle,
  Target,
} from 'lucide-react';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Textarea } from '@/components/ui/textarea';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  type GoalCriterion,
  type GoalCriteriaSet,
  type GoalRun,
  type GoalRunWorker,
  type GoalRunScope,
  type GoalRunScopeKind,
  useCancelGoalRun,
  useConfirmGoalRunCriteria,
  useConfirmGoalRunManual,
  useGoalRunDetail,
  useStartGoalRun,
  useStopGoalRun,
} from '@/hooks/useGoalRuns';
import { useStatus } from '@/hooks/useConfig';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import {
  autofillDaemonMachineID,
  initialMachineID,
  initialScopeKind,
  isRunnableDaemonMachineSelected,
} from './goalRunScopeSelection';
import { formatGoalRunScope, formatGoalRunSelectedScope } from './goalRunScopeCopy';

function formatDateTime(value?: string): string {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

function safeStringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function statusTone(status: GoalRun['status']): string {
  switch (status) {
    case 'completed':
    case 'ready':
      return 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-300';
    case 'running':
    case 'verifying':
      return 'bg-blue-500/12 text-blue-700 dark:text-blue-300';
    case 'criteria_pending_confirmation':
    case 'needs_approval':
    case 'needs_human_confirmation':
      return 'bg-amber-500/12 text-amber-700 dark:text-amber-300';
    case 'failed':
    case 'canceled':
      return 'bg-destructive/12 text-destructive';
    default:
      return 'bg-muted text-muted-foreground';
  }
}

function criterionTone(status: GoalCriterion['status']): string {
  switch (status) {
    case 'passed':
      return 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-300';
    case 'failed':
    case 'blocked':
      return 'bg-destructive/12 text-destructive';
    case 'needs_human_confirmation':
      return 'bg-amber-500/12 text-amber-700 dark:text-amber-300';
    default:
      return 'bg-muted text-muted-foreground';
  }
}

function describeScope(scope?: GoalRunScope | null): string {
  return formatGoalRunScope(scope, t);
}

function describeSelectedScope(run: GoalRun): string {
  return formatGoalRunSelectedScope(run, t);
}

function buildScopeSelection(
  run: GoalRun,
  selectedKind: GoalRunScopeKind,
  selectedMachineID: string,
): GoalRunScope {
  const base = run.selected_scope ?? run.recommended_scope;
  if (selectedKind === 'daemon') {
    return {
      kind: 'daemon',
      machine_id: selectedMachineID || (base?.kind === 'daemon' ? base.machine_id : undefined),
      source: 'manual',
      reason: t('goalRunsConfirmManualScopeReason'),
    };
  }
  return {
    kind: 'server',
    source: 'manual',
    reason: t('goalRunsConfirmManualScopeReason'),
  };
}

function GoalRunMeta({ run }: { run: GoalRun }) {
  return (
    <Card className="rounded-[24px] border-border/70 bg-card/92 shadow-sm">
      <CardHeader className="gap-3 pb-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle className="text-xl">{run.name}</CardTitle>
            <CardDescription className="mt-2 max-w-3xl text-sm leading-6">
              {run.goal}
            </CardDescription>
          </div>
          <span className={cn('inline-flex rounded-full px-3 py-1 text-xs font-medium', statusTone(run.status))}>
            {t(`goalRunsStatus_${run.status}`)}
          </span>
        </div>
      </CardHeader>
      <CardContent className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetaItem label={t('goalRunsFieldNaturalCriteria')} value={run.natural_language_criteria} />
        <MetaItem label={t('goalRunsFieldRiskLevel')} value={t(`goalRunsRisk_${run.risk_level}`)} />
        <MetaItem label={t('goalRunsSelectedScope')} value={describeSelectedScope(run)} />
        <MetaItem label={t('goalRunsRecommendedScope')} value={describeScope(run.recommended_scope)} />
        <MetaItem label={t('goalRunsCreatedBy')} value={run.created_by || '-'} />
        <MetaItem label={t('goalRunsCreatedAt')} value={formatDateTime(run.created_at)} />
        <MetaItem label={t('goalRunsUpdatedAt')} value={formatDateTime(run.updated_at)} />
        <MetaItem label={t('goalRunsAllowAutoScopeLabel')} value={run.allow_auto_scope ? t('yes') : t('no')} />
      </CardContent>
    </Card>
  );
}

function MetaItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-border/70 bg-muted/25 p-4">
      <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className="mt-2 whitespace-pre-wrap text-sm leading-6 text-foreground">{value}</div>
    </div>
  );
}

function CriteriaCard({ criteria }: { criteria: GoalCriteriaSet }) {
  return (
    <Card className="rounded-[24px] border-border/70 bg-card/92 shadow-sm">
      <CardHeader className="pb-4">
        <CardTitle className="text-lg">{t('goalRunsCriteriaTitle')}</CardTitle>
        <CardDescription>{t('goalRunsCriteriaDescription', criteria.criteria.length)}</CardDescription>
      </CardHeader>
      <CardContent>
        {criteria.criteria.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-border/70 px-4 py-8 text-center text-sm text-muted-foreground">
            {t('goalRunsCriteriaEmpty')}
          </div>
        ) : (
          <div className="space-y-3">
            {criteria.criteria.map((item) => (
              <div key={item.id} className="rounded-2xl border border-border/70 bg-muted/20 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <div className="text-sm font-medium text-foreground">{item.title}</div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {t('goalRunsCriterionTypeLabel', t(`goalRunsCriterionType_${item.type}`))}
                    </div>
                  </div>
                  <span className={cn('inline-flex rounded-full px-2.5 py-1 text-[11px] font-medium', criterionTone(item.status))}>
                    {t(`goalRunsCriterionStatus_${item.status}`)}
                  </span>
                </div>

                <div className="mt-3 grid gap-3 lg:grid-cols-2">
                  <div>
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{t('goalRunsCriterionScope')}</div>
                    <div className="mt-1 text-sm text-foreground">{describeScope(item.scope)}</div>
                  </div>
                  <div>
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{t('goalRunsCriterionRequired')}</div>
                    <div className="mt-1 text-sm text-foreground">{item.required ? t('yes') : t('no')}</div>
                  </div>
                </div>

                <div className="mt-3 grid gap-3 xl:grid-cols-2">
                  <div>
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{t('goalRunsCriterionDefinition')}</div>
                    <pre className="mt-1 overflow-x-auto rounded-xl bg-background/70 p-3 text-xs leading-6 text-foreground">{safeStringify(item.definition)}</pre>
                  </div>
                  <div>
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{t('goalRunsCriterionEvidence')}</div>
                    {item.evidence && item.evidence.length > 0 ? (
                      <ul className="mt-1 list-disc space-y-1 pl-5 text-sm text-foreground">
                        {item.evidence.map((entry) => (
                          <li key={entry} className="break-words">{entry}</li>
                        ))}
                      </ul>
                    ) : (
                      <div className="mt-1 text-sm text-muted-foreground">{t('goalRunsCriterionNoEvidence')}</div>
                    )}
                    {item.last_error ? (
                      <div className="mt-3 rounded-xl border border-destructive/20 bg-destructive/5 p-3 text-sm text-destructive">
                        {item.last_error}
                      </div>
                    ) : null}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function WorkerCard({ worker }: { worker: GoalRunWorker }) {
  return (
    <div className="rounded-2xl border border-border/70 bg-muted/20 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-sm font-medium text-foreground">{worker.name || worker.id}</div>
          <div className="mt-1 text-xs text-muted-foreground">{describeScope(worker.scope)}</div>
        </div>
        <span className="inline-flex rounded-full bg-muted px-2.5 py-1 text-[11px] font-medium text-foreground">
          {worker.status}
        </span>
      </div>
      <div className="mt-3 grid gap-3 text-sm sm:grid-cols-2">
        <MetaItem label={t('goalRunsWorkerTaskID')} value={worker.task_id || '-'} />
        <MetaItem label={t('goalRunsWorkerRestartCount')} value={String(worker.restart_count ?? 0)} />
        <MetaItem label={t('goalRunsWorkerHeartbeat')} value={formatDateTime(worker.last_heartbeat_at)} />
        <MetaItem label={t('goalRunsWorkerProgress')} value={formatDateTime(worker.last_progress_at)} />
      </div>
      {worker.last_error ? (
        <div className="mt-3 rounded-xl border border-destructive/20 bg-destructive/5 p-3 text-sm text-destructive">
          {worker.last_error}
        </div>
      ) : null}
    </div>
  );
}

function WorkersCard({ workers }: { workers: GoalRunWorker[] }) {
  return (
    <Card className="rounded-[24px] border-border/70 bg-card/92 shadow-sm">
      <CardHeader className="pb-4">
        <CardTitle className="text-lg">{t('goalRunsWorkersTitle')}</CardTitle>
        <CardDescription>{t('goalRunsWorkersDescription', workers.length)}</CardDescription>
      </CardHeader>
      <CardContent>
        {workers.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-border/70 px-4 py-8 text-center text-sm text-muted-foreground">
            {t('goalRunsWorkersEmpty')}
          </div>
        ) : (
          <div className="space-y-3">
            {workers.map((worker) => (
              <WorkerCard key={worker.id} worker={worker} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function ConfirmCriteriaCard({
  goalRunId,
  run,
  criteria,
}: {
  goalRunId: string;
  run: GoalRun;
  criteria: GoalCriteriaSet;
}) {
  const confirmCriteria = useConfirmGoalRunCriteria(goalRunId);
  const [selectedScopeKind, setSelectedScopeKind] = useState<GoalRunScopeKind>('server');
  const [selectedMachineID, setSelectedMachineID] = useState('');
  const { data: status } = useStatus();
  const daemonOptions = useMemo(
    () =>
      (status?.daemon_machines ?? [])
        .filter(
          (machine) =>
            machine.info.daemon_url &&
            machine.info.status === 'online' &&
            machine.goal_run_runnable,
        )
        .map((machine) => ({
          value: machine.info.machine_id,
          label: machine.info.machine_name || machine.info.machine_id,
        })),
    [status?.daemon_machines],
  );

  useEffect(() => {
    if (!run) {
      return;
    }
    setSelectedScopeKind(initialScopeKind(run));
    setSelectedMachineID(initialMachineID(run, daemonOptions));
  }, [run]);

  useEffect(() => {
    setSelectedMachineID((current) =>
      autofillDaemonMachineID({
        currentMachineID: current,
        selectedScopeKind,
        run,
        daemonOptions,
      }),
    );
  }, [daemonOptions, run, selectedScopeKind]);

  const selectedScope = useMemo(() => {
    if (!run) {
      return undefined;
    }
    return buildScopeSelection(run, selectedScopeKind, selectedMachineID);
  }, [run, selectedMachineID, selectedScopeKind]);

  if (!run || !criteria || run.status !== 'criteria_pending_confirmation') {
    return null;
  }

  const daemonSelectionRequired = selectedScopeKind === 'daemon';
  const daemonScopeUnavailable = daemonSelectionRequired && daemonOptions.length === 0;
  const daemonSelectionAvailable = isRunnableDaemonMachineSelected(selectedMachineID, daemonOptions);
  const daemonSelectionValid = !daemonSelectionRequired || daemonSelectionAvailable;
  const daemonSelectionStale = daemonSelectionRequired && selectedMachineID.trim().length > 0 && !daemonSelectionAvailable;

  const handleConfirm = () => {
    if (!daemonSelectionValid) {
      return;
    }
    confirmCriteria.mutate({
      criteria,
      selected_scope: selectedScope,
    });
  };

  return (
    <Card className="rounded-[24px] border-amber-500/30 bg-card/92 shadow-sm">
      <CardHeader className="pb-4">
        <div className="flex items-start gap-3">
          <div className="mt-0.5 rounded-2xl bg-amber-500/12 p-2 text-amber-700 dark:text-amber-300">
            <ShieldAlert className="h-5 w-5" />
          </div>
          <div>
            <CardTitle className="text-lg">{t('goalRunsConfirmTitle')}</CardTitle>
            <CardDescription className="mt-2 leading-6">
              {t('goalRunsConfirmDescription')}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-2 md:max-w-sm">
          <div className="text-sm font-medium text-foreground">{t('goalRunsConfirmScopeLabel')}</div>
          <Select value={selectedScopeKind} onValueChange={(value) => setSelectedScopeKind(value as GoalRunScopeKind)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="server">{t('goalRunsConfirmScopeServer')}</SelectItem>
              <SelectItem value="daemon">{t('goalRunsConfirmScopeDaemon')}</SelectItem>
            </SelectContent>
          </Select>
          <div className="text-xs text-muted-foreground">{describeScope(selectedScope)}</div>
        </div>

        {selectedScopeKind === 'daemon' ? (
          <div className="grid gap-2 md:max-w-sm">
            <div className="text-sm font-medium text-foreground">{t('goalRunsConfirmMachineLabel')}</div>
            <Select value={selectedMachineID} onValueChange={setSelectedMachineID} disabled={daemonScopeUnavailable}>
              <SelectTrigger>
                <SelectValue placeholder={t('goalRunsConfirmMachinePlaceholder')} />
              </SelectTrigger>
              <SelectContent>
                {daemonOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {daemonScopeUnavailable ? (
              <div className="text-xs text-muted-foreground">{t('goalRunsConfirmMachineUnavailable')}</div>
            ) : daemonSelectionStale ? (
              <div className="text-xs text-destructive">{t('goalRunsConfirmMachineStale')}</div>
            ) : !daemonSelectionValid ? (
              <div className="text-xs text-destructive">{t('goalRunsConfirmMachineRequired')}</div>
            ) : null}
          </div>
        ) : null}

        <div className="flex flex-wrap gap-3">
          <Button onClick={handleConfirm} disabled={confirmCriteria.isPending || !daemonSelectionValid || daemonScopeUnavailable}>
            {confirmCriteria.isPending ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <CheckCircle2 className="mr-2 h-4 w-4" />
            )}
            {t('goalRunsConfirmSubmit')}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function ManualConfirmationCard({
  goalRunId,
  criteria,
}: {
  goalRunId: string;
  criteria: GoalCriteriaSet;
}) {
  const confirmManual = useConfirmGoalRunManual(goalRunId);
  const [note, setNote] = useState('');

  const pendingManual = criteria.criteria.find((item) => item.status === 'needs_human_confirmation');
  if (!pendingManual) {
    return null;
  }

  return (
    <Card className="rounded-[24px] border-amber-500/30 bg-card/92 shadow-sm">
      <CardHeader className="pb-4">
        <CardTitle className="text-lg">{t('goalRunsManualConfirmTitle')}</CardTitle>
        <CardDescription>{t('goalRunsManualConfirmDescription')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="rounded-2xl border border-border/70 bg-muted/20 p-4">
          <div className="text-sm font-medium text-foreground">{pendingManual.title}</div>
          <div className="mt-1 text-sm text-muted-foreground">
            {String(pendingManual.definition?.prompt ?? '')}
          </div>
        </div>

        <div className="grid gap-2">
          <div className="text-sm font-medium text-foreground">{t('goalRunsManualConfirmNote')}</div>
          <Textarea
            value={note}
            onChange={(event) => setNote(event.target.value)}
            placeholder={t('goalRunsManualConfirmNotePlaceholder')}
            className="min-h-[100px]"
          />
        </div>

        <div className="flex flex-wrap gap-3">
          <Button
            onClick={() =>
              confirmManual.mutate({
                criterion_id: pendingManual.id,
                approved: true,
                note: note.trim(),
              })
            }
            disabled={confirmManual.isPending}
          >
            {confirmManual.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            {t('goalRunsManualConfirmApprove')}
          </Button>
          <Button
            variant="outline"
            onClick={() =>
              confirmManual.mutate({
                criterion_id: pendingManual.id,
                approved: false,
                note: note.trim(),
              })
            }
            disabled={confirmManual.isPending}
          >
            {t('goalRunsManualConfirmReject')}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function ActionBar({ run }: { run: GoalRun }) {
  const startGoalRun = useStartGoalRun(run.id);
  const stopGoalRun = useStopGoalRun(run.id);
  const cancelGoalRun = useCancelGoalRun(run.id);

  return (
    <Card className="rounded-[24px] border-border/70 bg-card/92 shadow-sm">
      <CardHeader className="pb-4">
        <CardTitle className="text-lg">{t('goalRunsActionsTitle')}</CardTitle>
        <CardDescription>{t('goalRunsActionsDescription')}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-3">
        <Button
          onClick={() => startGoalRun.mutate()}
          disabled={run.status !== 'ready' || startGoalRun.isPending}
        >
          {startGoalRun.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <PlayCircle className="mr-2 h-4 w-4" />}
          {t('goalRunsActionStart')}
        </Button>
        <Button
          variant="outline"
          onClick={() => stopGoalRun.mutate()}
          disabled={
            !['running', 'verifying', 'needs_human_confirmation'].includes(run.status) || stopGoalRun.isPending
          }
        >
          {stopGoalRun.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <PauseCircle className="mr-2 h-4 w-4" />}
          {t('goalRunsActionStop')}
        </Button>
        <Button
          variant="outline"
          onClick={() => cancelGoalRun.mutate()}
          disabled={['completed', 'failed', 'canceled'].includes(run.status) || cancelGoalRun.isPending}
        >
          {cancelGoalRun.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <StopCircle className="mr-2 h-4 w-4" />}
          {t('goalRunsActionCancel')}
        </Button>
      </CardContent>
    </Card>
  );
}

export default function GoalRunDetailPage() {
  const navigate = useNavigate();
  const { id = '' } = useParams();
  const { data: detail, isLoading, isError } = useGoalRunDetail(id || null);

  return (
    <div className="flex min-h-full flex-col">
      <div className="mb-4 flex items-center justify-between gap-3">
        <Button variant="ghost" onClick={() => navigate('/goal-runs')}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          {t('goalRunsBackToList')}
        </Button>
      </div>

      <Header
        title={detail?.goal_run.name || t('goalRunsDetailTitle')}
        description={t('goalRunsDetailDescription')}
      />

      {isLoading ? (
        <Card className="rounded-[24px] border-border/70 bg-card/92 shadow-sm">
          <CardContent className="flex min-h-[260px] items-center justify-center text-sm text-muted-foreground">
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            {t('goalRunsDetailLoading')}
          </CardContent>
        </Card>
      ) : isError || !detail ? (
        <Card className="rounded-[24px] border-border/70 bg-card/92 shadow-sm">
          <CardContent className="flex min-h-[260px] flex-col items-center justify-center gap-3 text-center">
            <Target className="h-6 w-6 text-muted-foreground" />
            <div>
              <div className="text-sm font-medium text-foreground">{t('goalRunsDetailLoadFailed')}</div>
              <div className="mt-1 text-sm text-muted-foreground">{t('goalRunsDetailLoadFailedHint')}</div>
            </div>
          </CardContent>
        </Card>
      ) : (
        <ScrollArea className="flex-1 pb-6">
          <div className="space-y-4 pb-2">
            <GoalRunMeta run={detail.goal_run} />
            <ActionBar run={detail.goal_run} />
            <ConfirmCriteriaCard goalRunId={detail.goal_run.id} run={detail.goal_run} criteria={detail.criteria} />
            <ManualConfirmationCard goalRunId={detail.goal_run.id} criteria={detail.criteria} />
            <CriteriaCard criteria={detail.criteria} />
            <WorkersCard workers={detail.workers ?? []} />
          </div>
        </ScrollArea>
      )}
    </div>
  );
}
