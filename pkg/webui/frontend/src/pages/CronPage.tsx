import { useMemo, useState } from 'react';
import { toast } from 'sonner';
import Header from '@/components/layout/Header';
import { useConfig } from '@/hooks/useConfig';
import { useProviders } from '@/hooks/useProviders';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogPortal,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { t } from '@/lib/i18n';
import {
  useCreateCronJob,
  useCronJobs,
  useDeleteCronJob,
  useDisableCronJob,
  useEnableCronJob,
  useRunCronJob,
  type CronScheduleKind,
  type CronJob,
} from '@/hooks/useCron';
import { Plus, Play, RefreshCw, Trash2, Loader2 } from 'lucide-react';

interface ProviderGroupInfo {
  name: string;
}

interface RouteTarget {
  name: string;
  type: 'provider' | 'group';
}

interface ModelEntry {
  provider: string;
  model: string;
}

interface CronFormState {
  name: string;
  schedule_kind: CronScheduleKind;
  schedule: string;
  at_time: string;
  every_duration: string;
  prompt: string;
  provider: string;
  model: string;
  fallback: string[];
  delete_after_run: boolean;
}

function toRFC3339FromLocal(localValue: string): string {
  const trimmed = localValue.trim();
  if (!trimmed) {
    return '';
  }
  const date = new Date(trimmed);
  if (Number.isNaN(date.getTime())) {
    return '';
  }
  return date.toISOString();
}

const DEFAULT_FORM: CronFormState = {
  name: '',
  schedule_kind: 'cron',
  schedule: '0 * * * *',
  at_time: '',
  every_duration: '5m',
  prompt: '',
  provider: '',
  model: '',
  fallback: [],
  delete_after_run: true,
};

function renderSchedule(job: CronJob): string {
  if (job.schedule_kind === 'at') {
    return job.at_time || '-';
  }
  if (job.schedule_kind === 'every') {
    return job.every_duration || '-';
  }
  return job.schedule || '-';
}

function renderLastResult(job: CronJob): string {
  if (!job.last_run) {
    return t('cronNeverRun');
  }
  return job.last_success ? t('cronLastRunOk') : (job.last_error || t('cronLastRunFailed'));
}

export default function CronPage() {
  const [form, setForm] = useState<CronFormState>(DEFAULT_FORM);
  const { data: providers = [] } = useProviders();
  const { data: config } = useConfig();

  const { data: jobs = [], isLoading, isFetching, refetch } = useCronJobs();
  const createJob = useCreateCronJob();
  const deleteJob = useDeleteCronJob();
  const enableJob = useEnableCronJob();
  const disableJob = useDisableCronJob();
  const runJob = useRunCronJob();

  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [deleteJobId, setDeleteJobId] = useState<string>('');

  const isBusy = createJob.isPending || deleteJob.isPending || enableJob.isPending || disableJob.isPending || runJob.isPending;

  const sortedJobs = useMemo(
    () => [...jobs].sort((a, b) => b.created_at.localeCompare(a.created_at)),
    [jobs],
  );

  const routeTargets = useMemo(() => {
    const targets: RouteTarget[] = [];
    const seen = new Set<string>();
    for (const provider of providers) {
      const name = provider.name.trim();
      if (!name || seen.has(name)) {
        continue;
      }
      seen.add(name);
      targets.push({ name, type: 'provider' });
    }
    const groups = ((config?.agents as { defaults?: { provider_groups?: ProviderGroupInfo[] } } | undefined)?.defaults?.provider_groups) ?? [];
    for (const group of groups) {
      const name = group?.name?.trim();
      if (!name || seen.has(name)) {
        continue;
      }
      seen.add(name);
      targets.push({ name, type: 'group' });
    }
    return targets;
  }, [config, providers]);

  const routeTargetMap = useMemo(
    () => new Map(routeTargets.map((target) => [target.name, target])),
    [routeTargets],
  );

  const modelOptions = useMemo(() => {
    const result: ModelEntry[] = [];
    const seen = new Set<string>();
    for (const provider of providers) {
      const providerName = provider.name.trim();
      if (!providerName) {
        continue;
      }
      const addModel = (model: string) => {
        const normalized = model.trim();
        if (!normalized) {
          return;
        }
        const key = `${providerName}::${normalized}`;
        if (seen.has(key)) {
          return;
        }
        seen.add(key);
        result.push({ provider: providerName, model: normalized });
      };
      if (provider.default_model) {
        addModel(provider.default_model);
      }
      for (const model of provider.models ?? []) {
        addModel(model);
      }
    }
    return result;
  }, [providers]);

  const filteredModels = useMemo(() => {
    if (!form.provider || routeTargetMap.get(form.provider)?.type === 'group') {
      return modelOptions;
    }
    return modelOptions.filter((entry) => entry.provider === form.provider);
  }, [form.provider, modelOptions, routeTargetMap]);

  const fallbackTargets = useMemo(
    () => routeTargets.filter((target) => target.name !== form.provider),
    [form.provider, routeTargets],
  );

  const setField = <K extends keyof CronFormState>(key: K, value: CronFormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const resetForm = () => {
    setForm(DEFAULT_FORM);
  };

  const handleCreate = () => {
    const payload: Record<string, unknown> = {
      name: form.name,
      schedule_kind: form.schedule_kind,
      prompt: form.prompt,
      provider: form.provider,
      model: form.model,
      fallback: form.fallback,
      delete_after_run: form.delete_after_run,
    };

    if (form.schedule_kind === 'cron') {
      payload.schedule = form.schedule;
    } else if (form.schedule_kind === 'at') {
      const atTimeRFC3339 = toRFC3339FromLocal(form.at_time);
      if (!atTimeRFC3339) {
        toast.error(t('cronInvalidAtTime'));
        return;
      }
      payload.at_time = atTimeRFC3339;
    } else {
      payload.every_duration = form.every_duration;
    }

    createJob.mutate(payload as never, {
      onSuccess: () => {
        resetForm();
      },
    });
  };

  const handleProviderChange = (value: string) => {
    const provider = value === '__default__' ? '' : value;
    setForm((prev) => ({
      ...prev,
      provider,
      fallback: prev.fallback.filter((item) => item !== provider),
    }));
  };

  const handleToggleFallback = (target: string) => {
    setForm((prev) => ({
      ...prev,
      fallback: prev.fallback.includes(target)
        ? prev.fallback.filter((item) => item !== target)
        : [...prev.fallback, target],
    }));
  };

  return (
    <div className="flex flex-col h-full gap-4">
      <Header title={t('tabCron')} description={t('cronPageDescription')} />

      <Card>
        <CardContent className="p-4 space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>{t('cronName')}</Label>
              <Input value={form.name} onChange={(e) => setField('name', e.target.value)} />
            </div>
            <div className="space-y-1.5">
              <Label>{t('cronKind')}</Label>
              <Select
                value={form.schedule_kind}
                onValueChange={(value) => setField('schedule_kind', value as CronScheduleKind)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="cron">cron</SelectItem>
                  <SelectItem value="at">at</SelectItem>
                  <SelectItem value="every">every</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {form.schedule_kind === 'cron' && (
              <div className="space-y-1.5 md:col-span-2">
                <Label>{t('cronExpression')}</Label>
                <Input
                  value={form.schedule}
                  onChange={(e) => setField('schedule', e.target.value)}
                  placeholder="0 * * * *"
                />
              </div>
            )}

            {form.schedule_kind === 'at' && (
              <div className="space-y-1.5 md:col-span-2">
                <Label>{t('cronAtTime')}</Label>
                <Input
                  type="datetime-local"
                  value={form.at_time}
                  onChange={(e) => setField('at_time', e.target.value)}
                />
              </div>
            )}

            {form.schedule_kind === 'every' && (
              <div className="space-y-1.5 md:col-span-2">
                <Label>{t('cronEveryDuration')}</Label>
                <Input
                  value={form.every_duration}
                  onChange={(e) => setField('every_duration', e.target.value)}
                  placeholder="5m"
                />
              </div>
            )}

            <div className="space-y-1.5 md:col-span-2">
              <Label>{t('cronPrompt')}</Label>
              <textarea
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm min-h-[84px]"
                value={form.prompt}
                onChange={(e) => setField('prompt', e.target.value)}
              />
            </div>

            <div className="space-y-1.5">
              <Label>{t('cronProvider')}</Label>
              <Select value={form.provider || '__default__'} onValueChange={handleProviderChange}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__default__">{t('cronRouteAuto')}</SelectItem>
                  {routeTargets.map((target) => (
                    <SelectItem key={target.name} value={target.name}>
                      {target.type === 'group'
                        ? `${target.name} (${t('cronRouteTargetGroup')})`
                        : target.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>{t('cronModel')}</Label>
              <Select
                value={form.model || '__default__'}
                onValueChange={(value) => setField('model', value === '__default__' ? '' : value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__default__">{t('cronModelDefault')}</SelectItem>
                  {filteredModels.map((entry) => (
                    <SelectItem key={`${entry.provider}::${entry.model}`} value={entry.model}>
                      {form.provider && routeTargetMap.get(form.provider)?.type !== 'group'
                        ? entry.model
                        : `${entry.model} (${entry.provider})`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2 md:col-span-2">
              <Label>{t('cronFallback')}</Label>
              <div className="rounded-md border border-input bg-background px-3 py-3">
                <div className="mb-3 text-xs text-muted-foreground">{t('cronFallbackHint')}</div>
                {form.fallback.length > 0 ? (
                  <div className="mb-3 flex flex-wrap gap-2">
                    {form.fallback.map((target, index) => (
                      <button
                        key={target}
                        type="button"
                        onClick={() => handleToggleFallback(target)}
                        className="inline-flex items-center gap-2 rounded-full border border-[hsl(var(--brand-200))] bg-[hsl(var(--brand-50))] px-3 py-1.5 text-xs font-medium text-[hsl(var(--brand-800))]"
                      >
                        <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-white text-[10px] text-[hsl(var(--brand-700))]">
                          {index + 1}
                        </span>
                        {target}
                      </button>
                    ))}
                  </div>
                ) : (
                  <div className="mb-3 rounded-md border border-dashed border-input px-3 py-3 text-sm text-muted-foreground">
                    {t('cronFallbackEmpty')}
                  </div>
                )}
                <div className="flex flex-wrap gap-2">
                  {fallbackTargets.map((target) => {
                    const selected = form.fallback.includes(target.name);
                    return (
                      <button
                        key={target.name}
                        type="button"
                        onClick={() => handleToggleFallback(target.name)}
                        className={
                          selected
                            ? 'rounded-full border border-[hsl(var(--brand-300))] bg-[hsl(var(--brand-100))] px-3 py-1.5 text-xs font-medium text-[hsl(var(--brand-800))]'
                            : 'rounded-full border border-input bg-white px-3 py-1.5 text-xs font-medium text-muted-foreground'
                        }
                      >
                        {target.type === 'group'
                          ? `${target.name} (${t('cronRouteTargetGroup')})`
                          : target.name}
                      </button>
                    );
                  })}
                </div>
              </div>
            </div>

            {form.schedule_kind === 'at' && (
              <div className="flex items-center justify-between border rounded-md px-3 py-2 md:col-span-2">
                <Label>{t('cronDeleteAfterRun')}</Label>
                <Switch
                  checked={form.delete_after_run}
                  onCheckedChange={(checked) => setField('delete_after_run', checked)}
                />
              </div>
            )}
          </div>

          <div className="flex items-center gap-2">
            <Button onClick={handleCreate} disabled={createJob.isPending}>
              <Plus className="h-4 w-4 mr-1" />
              {t('cronCreate')}
            </Button>
            <Button variant="outline" onClick={resetForm}>
              {t('reset')}
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw className={`h-4 w-4 mr-1 ${isFetching ? 'animate-spin' : ''}`} />
          {t('refresh')}
        </Button>
        <span className="text-xs text-muted-foreground">{t('cronTotalJobs', String(sortedJobs.length))}</span>
      </div>

      <ScrollArea className="flex-1">
        <div className="space-y-3 pr-2 pb-4">
          {isLoading ? (
            <div className="py-8 text-center text-sm text-muted-foreground">{t('loading')}</div>
          ) : sortedJobs.length === 0 ? (
            <div className="text-sm text-muted-foreground py-8 text-center">{t('cronNoJobs')}</div>
          ) : (
            sortedJobs.map((job) => (
              <Card key={job.id}>
                <CardContent className="p-4 space-y-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="font-medium text-sm truncate">{job.name}</div>
                      <div className="text-xs text-muted-foreground break-all">{job.id}</div>
                    </div>
                    <div className="text-xs px-2 py-1 rounded bg-muted">{job.enabled ? t('on') : t('off')}</div>
                  </div>

                  <div className="text-xs text-muted-foreground space-y-1">
                    <div>{t('cronKind')}: {job.schedule_kind}</div>
                    <div>{t('cronSchedule')}: {renderSchedule(job)}</div>
                    <div>{t('cronProvider')}: {job.provider || t('cronRouteAuto')}</div>
                    <div>{t('cronModel')}: {job.model || t('cronModelDefault')}</div>
                    <div>{t('cronFallback')}: {job.fallback && job.fallback.length > 0 ? job.fallback.join(' -> ') : t('cronFallbackEmpty')}</div>
                    <div>{t('cronNextRun')}: {job.next_run || '-'}</div>
                    <div>{t('cronLastRun')}: {job.last_run || '-'}</div>
                    <div>{t('cronRunCount')}: {job.run_count}</div>
                    <div>{t('cronLastResult')}: {renderLastResult(job)}</div>
                  </div>

                  <div className="flex flex-wrap gap-2">
                    {job.enabled ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => disableJob.mutate(job.id)}
                        disabled={isBusy}
                      >
                        {t('cronDisable')}
                      </Button>
                    ) : (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => enableJob.mutate(job.id)}
                        disabled={isBusy}
                      >
                        {t('cronEnable')}
                      </Button>
                    )}
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => runJob.mutate(job.id)}
                      disabled={isBusy}
                    >
                      <Play className="h-3.5 w-3.5 mr-1" />
                      {t('cronRunNow')}
                    </Button>
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={() => {
                        setDeleteJobId(job.id);
                        setShowDeleteConfirm(true);
                      }}
                      disabled={isBusy}
                    >
                      <Trash2 className="h-3.5 w-3.5 mr-1" />
                      {t('delete')}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))
          )}
        </div>
      </ScrollArea>

      {/* Delete Job Confirmation Dialog */}
      <Dialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <DialogPortal>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('cronDeleteConfirmTitle')}</DialogTitle>
              <DialogDescription>
                {t('cronDeleteConfirmDescription')}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowDeleteConfirm(false)}>
                {t('cancel')}
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  if (deleteJobId) {
                    deleteJob.mutate(deleteJobId);
                    setShowDeleteConfirm(false);
                    setDeleteJobId('');
                  }
                }}
                disabled={deleteJob.isPending}
              >
                {deleteJob.isPending ? (
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
    </div>
  );
}
