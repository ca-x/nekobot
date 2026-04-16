import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Target, Loader2, ArrowRight } from 'lucide-react';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  type CreateGoalRunInput,
  type GoalRun,
  type GoalRunRiskLevel,
  useCreateGoalRun,
  useGoalRuns,
} from '@/hooks/useGoalRuns';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { formatGoalRunScopeSummary } from './goalRunScopeCopy';

const RISK_LEVELS: GoalRunRiskLevel[] = ['conservative', 'balanced', 'aggressive'];

function formatDateTime(value?: string): string {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
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

function riskTone(riskLevel: GoalRunRiskLevel): string {
  switch (riskLevel) {
    case 'conservative':
      return 'bg-slate-500/12 text-slate-700 dark:text-slate-300';
    case 'balanced':
      return 'bg-violet-500/12 text-violet-700 dark:text-violet-300';
    case 'aggressive':
      return 'bg-rose-500/12 text-rose-700 dark:text-rose-300';
  }
}

function scopeLabel(run: GoalRun): string {
  return formatGoalRunScopeSummary(run, t);
}

function StatusBadge({ status }: { status: GoalRun['status'] }) {
  return (
    <span className={cn('inline-flex rounded-full px-2.5 py-1 text-[11px] font-medium', statusTone(status))}>
      {t(`goalRunsStatus_${status}`)}
    </span>
  );
}

function RiskBadge({ riskLevel }: { riskLevel: GoalRunRiskLevel }) {
  return (
    <span className={cn('inline-flex rounded-full px-2.5 py-1 text-[11px] font-medium', riskTone(riskLevel))}>
      {t(`goalRunsRisk_${riskLevel}`)}
    </span>
  );
}

function EmptyState({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-[24px] border border-dashed border-border/70 bg-card/70 px-6 py-14 text-center shadow-sm">
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary">
        <Target className="h-6 w-6" />
      </div>
      <h3 className="mt-5 text-lg font-semibold text-foreground">{t('goalRunsEmptyTitle')}</h3>
      <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
        {t('goalRunsEmptyDescription')}
      </p>
      <Button className="mt-6" onClick={onCreate}>
        <Plus className="mr-2 h-4 w-4" />
        {t('goalRunsCreateButton')}
      </Button>
    </div>
  );
}

function CreateGoalRunDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (id: string) => void;
}) {
  const createGoalRun = useCreateGoalRun();
  const [form, setForm] = useState<CreateGoalRunInput>({
    name: '',
    goal: '',
    natural_language_criteria: '',
    risk_level: 'balanced',
    allow_auto_scope: true,
  });

  useEffect(() => {
    if (!open) {
      setForm({
        name: '',
        goal: '',
        natural_language_criteria: '',
        risk_level: 'balanced',
        allow_auto_scope: true,
      });
    }
  }, [open]);

  const isValid =
    form.name.trim().length > 0 &&
    form.goal.trim().length > 0 &&
    form.natural_language_criteria.trim().length > 0;

  const updateField = <K extends keyof CreateGoalRunInput>(key: K, value: CreateGoalRunInput[K]) => {
    setForm((current) => ({ ...current, [key]: value }));
  };

  const handleSubmit = () => {
    if (!isValid || createGoalRun.isPending) {
      return;
    }
    createGoalRun.mutate(
      {
        name: form.name.trim(),
        goal: form.goal.trim(),
        natural_language_criteria: form.natural_language_criteria.trim(),
        risk_level: form.risk_level,
        allow_auto_scope: form.allow_auto_scope,
      },
      {
        onSuccess: (result) => {
          onCreated(result.goal_run.id);
          onOpenChange(false);
        },
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('goalRunsCreateTitle')}</DialogTitle>
          <DialogDescription>{t('goalRunsCreateDescription')}</DialogDescription>
        </DialogHeader>

        <div className="grid gap-5 py-2">
          <div className="grid gap-2">
            <Label htmlFor="goal-run-name">{t('goalRunsFieldName')}</Label>
            <Input
              id="goal-run-name"
              value={form.name}
              onChange={(event) => updateField('name', event.target.value)}
              placeholder={t('goalRunsFieldNamePlaceholder')}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="goal-run-goal">{t('goalRunsFieldGoal')}</Label>
            <Textarea
              id="goal-run-goal"
              value={form.goal}
              onChange={(event) => updateField('goal', event.target.value)}
              placeholder={t('goalRunsFieldGoalPlaceholder')}
              className="min-h-[120px]"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="goal-run-criteria">{t('goalRunsFieldNaturalCriteria')}</Label>
            <Textarea
              id="goal-run-criteria"
              value={form.natural_language_criteria}
              onChange={(event) => updateField('natural_language_criteria', event.target.value)}
              placeholder={t('goalRunsFieldNaturalCriteriaPlaceholder')}
              className="min-h-[120px]"
            />
          </div>

          <div className="grid gap-5 md:grid-cols-[1fr_auto] md:items-end">
            <div className="grid gap-2">
              <Label>{t('goalRunsFieldRiskLevel')}</Label>
              <Select
                value={form.risk_level}
                onValueChange={(value) => updateField('risk_level', value as GoalRunRiskLevel)}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('goalRunsFieldRiskLevelPlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                  {RISK_LEVELS.map((riskLevel) => (
                    <SelectItem key={riskLevel} value={riskLevel}>
                      {t(`goalRunsRisk_${riskLevel}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <label className="flex items-center justify-between gap-3 rounded-2xl border border-border/70 bg-muted/30 px-4 py-3 md:min-w-[260px]">
              <div>
                <div className="text-sm font-medium text-foreground">{t('goalRunsFieldAllowAutoScope')}</div>
                <div className="text-xs text-muted-foreground">{t('goalRunsFieldAllowAutoScopeHint')}</div>
              </div>
              <Switch
                checked={form.allow_auto_scope}
                onCheckedChange={(checked) => updateField('allow_auto_scope', checked)}
                aria-label={t('goalRunsFieldAllowAutoScope')}
              />
            </label>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={!isValid || createGoalRun.isPending}>
            {createGoalRun.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Plus className="mr-2 h-4 w-4" />}
            {t('goalRunsCreateSubmit')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default function GoalRunsPage() {
  const navigate = useNavigate();
  const { data: goalRuns = [], isLoading, isError } = useGoalRuns();
  const [createOpen, setCreateOpen] = useState(false);

  const sortedRuns = useMemo(
    () =>
      [...goalRuns].sort(
        (left, right) =>
          new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime(),
      ),
    [goalRuns],
  );

  return (
    <div className="flex min-h-full flex-col">
      <Header
        title={t('tabGoalRuns')}
        description={t('goalRunsPageDescription')}
      />

      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div className="text-sm text-muted-foreground">
          {t('goalRunsCount', sortedRuns.length)}
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t('goalRunsCreateButton')}
        </Button>
      </div>

      {isLoading ? (
        <Card className="rounded-[24px] border-border/70 bg-card/92 shadow-sm">
          <CardContent className="flex min-h-[220px] items-center justify-center text-sm text-muted-foreground">
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            {t('goalRunsLoading')}
          </CardContent>
        </Card>
      ) : isError ? (
        <Card className="rounded-[24px] border-border/70 bg-card/92 shadow-sm">
          <CardContent className="flex min-h-[220px] items-center justify-center text-sm text-destructive">
            {t('goalRunsLoadFailed')}
          </CardContent>
        </Card>
      ) : sortedRuns.length === 0 ? (
        <EmptyState onCreate={() => setCreateOpen(true)} />
      ) : (
        <ScrollArea className="flex-1 pb-6">
          <div className="grid gap-4 pb-2 xl:grid-cols-2">
            {sortedRuns.map((run) => (
              <Card
                key={run.id}
                className="rounded-[24px] border-border/70 bg-card/92 shadow-sm transition-transform hover:-translate-y-0.5"
              >
                <CardHeader className="gap-3 pb-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="space-y-1">
                      <CardTitle className="text-lg">{run.name}</CardTitle>
                      <CardDescription className="line-clamp-2 text-sm leading-6">
                        {run.goal}
                      </CardDescription>
                    </div>
                    <StatusBadge status={run.status} />
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex flex-wrap gap-2">
                    <RiskBadge riskLevel={run.risk_level} />
                    <span className="inline-flex rounded-full bg-muted px-2.5 py-1 text-[11px] font-medium text-muted-foreground">
                      {scopeLabel(run)}
                    </span>
                  </div>

                  <dl className="grid gap-3 text-sm text-muted-foreground sm:grid-cols-2">
                    <div>
                      <dt className="text-xs uppercase tracking-[0.18em]">{t('goalRunsUpdatedAt')}</dt>
                      <dd className="mt-1 text-foreground">{formatDateTime(run.updated_at)}</dd>
                    </div>
                    <div>
                      <dt className="text-xs uppercase tracking-[0.18em]">{t('goalRunsCreatedBy')}</dt>
                      <dd className="mt-1 text-foreground">{run.created_by || '-'}</dd>
                    </div>
                  </dl>

                  <div className="flex justify-end">
                    <Button variant="outline" onClick={() => navigate(`/goal-runs/${run.id}`)}>
                      {t('goalRunsOpenDetail')}
                      <ArrowRight className="ml-2 h-4 w-4" />
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </ScrollArea>
      )}

      <CreateGoalRunDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={(id) => navigate(`/goal-runs/${id}`)}
      />
    </div>
  );
}
