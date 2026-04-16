import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { toast } from 'sonner';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Skeleton } from '@/components/ui/skeleton';
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
  useCreateMarketplaceSnapshot,
  useDeleteMarketplaceSnapshot,
  useDisableMarketplaceSkill,
  useEnableMarketplaceSkill,
  useInstallMarketplaceSkill,
  useInstallMarketplaceSkillDependencies,
  useInstalledMarketplaceSkills,
  useMarketplaceInventory,
  useMarketplaceSkillContent,
  useMarketplaceSkillItem,
  useMarketplaceSnapshots,
  useMarketplaceSkills,
  usePruneMarketplaceSnapshots,
  useRepairWorkspace,
  useRestoreMarketplaceSnapshot,
  useSearchMarketplaceSkills,
  type MarketplaceInstallResult,
  type MarketplaceSnapshot,
  type MarketplaceSkill,
  type MarketplaceSkillSource,
  type WorkspaceStatus,
  useWorkspaceStatus,
} from '@/hooks/useMarketplace';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import {
  AlertTriangle,
  BadgeCheck,
  Download,
  FileCode2,
  FileText,
  FolderCog,
  FolderOpen,
  Loader2,
  Pin,
  RefreshCcw,
  RotateCcw,
  Search,
  ShieldAlert,
  ShieldCheck,
  Sparkles,
  TimerReset,
  ToggleLeft,
  Trash2,
  Wrench,
  Zap,
} from 'lucide-react';

export default function MarketplacePage() {
  const { data: skills, isLoading } = useMarketplaceSkills();
  const { data: installed } = useInstalledMarketplaceSkills();
  const installSkill = useInstallMarketplaceSkill();
  const enableSkill = useEnableMarketplaceSkill();
  const disableSkill = useDisableMarketplaceSkill();
  const installDependencies = useInstallMarketplaceSkillDependencies();
  const createSnapshot = useCreateMarketplaceSnapshot();
  const restoreSnapshot = useRestoreMarketplaceSnapshot();
  const deleteSnapshot = useDeleteMarketplaceSnapshot();
  const pruneSnapshots = usePruneMarketplaceSnapshots();
  const repairWorkspace = useRepairWorkspace();
  const { data: inventory } = useMarketplaceInventory();
  const { data: snapshots } = useMarketplaceSnapshots();
  const { data: workspaceStatus } = useWorkspaceStatus();

  const [query, setQuery] = useState('');
  const [remoteQuery, setRemoteQuery] = useState('');
  const [installSource, setInstallSource] = useState('');
  const [selectedSkillID, setSelectedSkillID] = useState<string | null>(null);
  const [snapshotLabel, setSnapshotLabel] = useState('');
  const [snapshotNote, setSnapshotNote] = useState('');
  const { data: remoteSearch, isFetching: isSearchingRemote } = useSearchMarketplaceSkills(remoteQuery);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [deleteSnapshotId, setDeleteSnapshotId] = useState<string>('');

  const marketplaceSkills = skills ?? [];
  const filteredSkills = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) {
      return marketplaceSkills;
    }
    return marketplaceSkills.filter((skill) =>
      [skill.id, skill.name, skill.description, skill.author, ...(skill.tags ?? [])]
        .join(' ')
        .toLowerCase()
        .includes(keyword),
    );
  }, [marketplaceSkills, query]);

  useEffect(() => {
    if (filteredSkills.length === 0) {
      setSelectedSkillID(null);
      return;
    }
    if (!selectedSkillID || !filteredSkills.some((skill) => skill.id === selectedSkillID)) {
      setSelectedSkillID(filteredSkills[0]?.id ?? null);
    }
  }, [filteredSkills, selectedSkillID]);

  const { data: selectedSkill, isLoading: isLoadingItem } = useMarketplaceSkillItem(selectedSkillID);
  const { data: selectedContent, isLoading: isLoadingContent } = useMarketplaceSkillContent(selectedSkillID);

  const handleEnable = (id: string) => {
    enableSkill.mutate(id);
  };

  const handleDisable = (id: string) => {
    disableSkill.mutate(id);
  };

  const handleInstallDependencies = (id: string) => {
    installDependencies.mutate(id);
  };

  const handleInstallSkill = () => {
    const source = installSource.trim();
    if (!source) {
      return;
    }
    installSkill.mutate(source, {
      onSuccess: () => setInstallSource(''),
      onError: (err) => toast.error(err instanceof Error ? err.message : t('marketplaceInstallFailed')),
    });
  };

  const installedCount = installed?.total ?? marketplaceSkills.filter((skill) => skill.enabled).length;
  const alwaysOnCount = marketplaceSkills.filter((skill) => skill.always).length;
  const readyCount = marketplaceSkills.filter((skill) => skill.eligible).length;
  const selectedInstallResults =
    installDependencies.data?.skill_id === selectedSkillID ? installDependencies.data.results : [];
  const snapshotItems = snapshots?.snapshots ?? [];
  const snapshotMaxCount = snapshots?.max_count ?? 0;
  const canPruneSnapshots = snapshotMaxCount > 0 && snapshotItems.length > snapshotMaxCount;
  const selectedMissingRequirements = selectedSkill?.missing_requirements ?? {
    binaries: [],
    any_binaries: [],
    env: [],
    config_paths: [],
    python_packages: [],
    node_packages: [],
  };
  const selectedInstallSpecs = selectedSkill?.install_specs ?? [];
  const selectedTags = selectedSkill?.tags ?? [];
  const selectedReasons = selectedSkill?.ineligibility_reasons ?? [];

  const handleCreateSnapshot = () => {
    createSnapshot.mutate(
      {
        label: snapshotLabel.trim() || undefined,
        note: snapshotNote.trim() || undefined,
      },
      {
        onSuccess: () => {
          setSnapshotLabel('');
          setSnapshotNote('');
        },
      },
    );
  };

  return (
    <div className="marketplace-page space-y-6">
      <Header
        title={t('tabMarketplace')}
        description={t('marketplaceHeaderDescription')}
      />

      <section className="relative overflow-hidden rounded-[28px] border border-border/70 bg-[radial-gradient(circle_at_top_left,_rgba(16,185,129,0.14),_transparent_40%),linear-gradient(135deg,_rgba(255,255,255,0.98),_rgba(236,253,245,0.72))] p-5 shadow-sm sm:p-6">
        <div className="absolute bottom-0 right-0 h-40 w-40 rounded-full bg-emerald-500/15 blur-3xl" />
        <div className="relative flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-3">
            <div className="inline-flex items-center gap-2 rounded-full border border-emerald-300/40 bg-card/90 px-3 py-1 text-xs font-medium text-emerald-700 dark:text-emerald-300 shadow-sm">
              <Sparkles className="h-3.5 w-3.5" />
              {t('marketplaceInstallSkill')}
            </div>
            <div className="space-y-2">
              <h2 className="max-w-2xl text-2xl font-semibold tracking-tight text-foreground">
                {t('marketplaceHeroTitle')}
              </h2>
              <p className="max-w-2xl text-sm leading-6 text-muted-foreground">
                {t('marketplaceHeroDescription')}
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <SkillMetric label={t('marketplaceInstalled')} value={String(installedCount)} />
              <SkillMetric label={t('marketplaceReady')} value={String(readyCount)} />
              <SkillMetric label={t('marketplaceAlwaysOn')} value={String(alwaysOnCount)} />
              <SkillMetric label={t('marketplaceSelected')} value={selectedSkill?.name ?? t('none')} muted={!selectedSkill} />
            </div>
          </div>

          <div className="w-full lg:w-[340px]">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t('marketplaceSearchPlaceholder')}
                className="h-11 rounded-2xl border-border/70 bg-card/90 pl-9 shadow-sm"
              />
            </div>
          </div>
        </div>
      </section>

      <section className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)]">
        <Card className="rounded-[28px] border-border/70 bg-card/92 p-5 shadow-sm">
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t('marketplaceRemoteSearch')}
              </div>
              <h3 className="mt-2 text-lg font-semibold text-foreground">
                {t('marketplaceRemoteHeadline')}
              </h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t('marketplaceRemoteDescription')}
              </p>
            </div>
            <span className="rounded-full bg-muted/60 px-3 py-1 text-xs text-muted-foreground">
              {remoteSearch?.proxy ? t('marketplaceProxyEnabled') : t('marketplaceDirect')}
            </span>
          </div>

          <div className="mt-4">
            <Input
              value={remoteQuery}
              onChange={(event) => setRemoteQuery(event.target.value)}
              placeholder={t('marketplaceSearchRemotePlaceholder')}
              className="h-11 rounded-2xl"
            />
          </div>

          <div className="mt-4 rounded-2xl border border-border/70 bg-[hsl(var(--gray-900))] p-4 text-white dark:bg-[hsl(var(--gray-950))] dark:text-white">
            <div className="flex items-center justify-between gap-3">
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-white/60 dark:text-white/60">
                {t('marketplaceRegistryOutput')}
              </div>
              {isSearchingRemote ? <span className="text-xs text-white/60 dark:text-white/60">{t('marketplaceSearching')}</span> : null}
            </div>
            <pre className="mt-3 min-h-[180px] whitespace-pre-wrap break-words font-mono text-xs leading-6 text-white/90 dark:text-white/90">
              {remoteQuery.trim().length === 0
                ? t('marketplaceSearchRemoteHint')
                : remoteSearch?.output || remoteSearch?.error || t('marketplaceNoRegistryOutput')}
            </pre>
          </div>
        </Card>

        <Card className="rounded-[28px] border-border/70 bg-card/92 p-5 shadow-sm">
          <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t('marketplaceInstallSkill')}</div>
          <h3 className="mt-2 text-lg font-semibold text-foreground">
            {t('marketplaceInstallHeadline')}
          </h3>
          <p className="mt-2 text-sm leading-6 text-muted-foreground">
            {t('marketplaceInstallDescription')}
          </p>

          <div className="mt-5 space-y-3">
            <Input
              value={installSource}
              onChange={(event) => setInstallSource(event.target.value)}
              placeholder={t('marketplaceInstallPlaceholder')}
              className="h-11 rounded-2xl"
            />
            <Button
              onClick={handleInstallSkill}
              disabled={!installSource.trim() || installSkill.isPending}
              className="rounded-xl"
            >
              <Download className="mr-2 h-4 w-4" />
              {installSkill.isPending ? t('marketplaceInstalling') : t('marketplaceInstallSkill')}
            </Button>
          </div>

          <div className="mt-5 rounded-2xl border border-border/70 bg-muted/35 p-4">
            <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t('marketplaceLastInstall')}</div>
            <p className="mt-2 text-sm leading-6 text-foreground">
              {installSkill.data
                ? t('marketplaceLastInstallResult', installSkill.data.source, installSkill.data.target)
                : t('marketplaceLastInstallNone')}
            </p>
          </div>
        </Card>
      </section>

      <section className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)_minmax(0,1fr)]">
        <Card className="rounded-[28px] border-border/70 bg-card/92 p-5 shadow-sm">
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t('marketplaceWorkspace')}
              </div>
              <h3 className="mt-2 text-lg font-semibold text-foreground">
                {t('marketplaceWorkspaceHeadline')}
              </h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t('marketplaceWorkspaceDescription')}
              </p>
            </div>
            <Button
              size="sm"
              variant="outline"
              className="rounded-xl"
              disabled={repairWorkspace.isPending}
              onClick={() => repairWorkspace.mutate()}
            >
              <RefreshCcw className="mr-2 h-4 w-4" />
              {repairWorkspace.isPending ? t('marketplaceRepairingWorkspace') : t('marketplaceRepairWorkspace')}
            </Button>
          </div>

          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <SkillInfoCard
              icon={<FolderOpen className="h-4 w-4" />}
              label={t('marketplaceWorkspacePath')}
              value={workspaceStatus?.path || '-'}
            />
            <SkillInfoCard
              icon={<FolderCog className="h-4 w-4" />}
              label={t('marketplaceBootstrap')}
              value={workspaceStatus?.bootstrapped ? t('marketplaceReady') : t('marketplaceNeedsRepair')}
            />
            <SkillInfoCard
              icon={<FileText className="h-4 w-4" />}
              label={t('marketplaceTodayLog')}
              value={workspaceStatus?.today_log_exists ? t('marketplacePresent') : t('marketplaceMissing')}
            />
            <SkillInfoCard
              icon={<Zap className="h-4 w-4" />}
              label={t('marketplaceHeartbeatState')}
              value={workspaceStatus?.heartbeat_state_exists ? t('marketplacePresent') : t('marketplaceMissing')}
            />
          </div>

          <div className="mt-4 rounded-[24px] border border-border/70 bg-muted/35 p-4">
            <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t('marketplaceBootstrapFiles')}</div>
            <div className="mt-3 flex flex-wrap gap-2">
              {((workspaceStatus?.bootstrap_files ?? []).length > 0
                ? (workspaceStatus?.bootstrap_files ?? [])
                : (workspaceStatus?.missing_bootstrap ?? [])).map((name) => {
                const missing = (workspaceStatus?.missing_bootstrap ?? []).includes(name);
                return (
                  <span
                    key={name}
                    className={cn(
                      'rounded-full px-2.5 py-1 text-xs font-medium',
                      missing
                        ? 'border border-amber-200 bg-amber-50 text-amber-800'
                        : 'bg-emerald-100 text-emerald-700',
                    )}
                  >
                    {name}
                  </span>
                );
              })}
            </div>
            {(workspaceStatus?.bootstrap_files ?? []).length === 0 && (workspaceStatus?.missing_bootstrap ?? []).length === 0 ? (
              <p className="mt-3 text-sm text-muted-foreground">{t('marketplaceNoSources')}</p>
            ) : null}
            <p className="mt-4 break-all text-xs leading-6 text-muted-foreground">
              {t('marketplaceDailyLog')}: {workspaceStatus?.today_log_path || '-'}
            </p>
            {workspaceStatus?.contract ? (
              <div className="mt-4 border-t border-border/70 pt-4">
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">Workspace contract</div>
                <div className="mt-2 text-sm font-semibold text-foreground">{workspaceStatus.contract.kind}</div>
                <div className="mt-3 flex flex-wrap gap-2">
                  {(workspaceStatus.contract.validation?.on_turn_end ?? []).map((item) => (
                    <span key={`turn-${item}`} className="rounded-full bg-background/80 px-2.5 py-1 text-[11px] text-muted-foreground">
                      turn_end: {item}
                    </span>
                  ))}
                  {(workspaceStatus.contract.validation?.on_completion ?? []).map((item) => (
                    <span key={`done-${item}`} className="rounded-full bg-background/80 px-2.5 py-1 text-[11px] text-muted-foreground">
                      completion: {item}
                    </span>
                  ))}
                </div>
              </div>
            ) : null}
          </div>
        </Card>

        <Card className="rounded-[28px] border-border/70 bg-card/92 p-5 shadow-sm">
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t('marketplaceSkillSources')}
              </div>
              <h3 className="mt-2 text-lg font-semibold text-foreground">
                {t('marketplaceSourcesHeadline')}
              </h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t('marketplaceSourcesDescription')}
              </p>
            </div>
            <div className="rounded-full bg-muted/60 px-3 py-1 text-xs text-muted-foreground">
              {t('marketplaceSourcesCount', String(inventory?.source_count ?? 0))}
            </div>
          </div>

          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-3">
            <SkillMetric label={t('marketplaceWritableDir')} value={inventory?.writable_dir || '-'} muted={!inventory?.writable_dir} />
            <SkillMetric label={t('marketplaceEnabled')} value={String(inventory?.enabled_count ?? 0)} />
            <SkillMetric label={t('marketplaceAlwaysOn')} value={String(inventory?.always_count ?? 0)} />
            <SkillMetric label={t('marketplaceVersionedSkills')} value={String(inventory?.version_history?.skill_count ?? 0)} />
            <SkillMetric label={t('marketplaceVersionRecords')} value={String(inventory?.version_history?.version_count ?? 0)} />
            <SkillMetric
              label={t('marketplaceVersionRetention')}
              value={
                inventory?.version_history
                  ? `${inventory.version_history.enabled ? t('on') : t('off')} · ${inventory.version_history.max_count}`
                  : '-'
              }
              muted={!inventory?.version_history}
            />
          </div>

          <div className="mt-4 space-y-3">
            {(inventory?.sources ?? []).map((source) => (
              <SkillSourceRow key={`${source.type}-${source.path}-${source.priority}`} source={source} />
            ))}
            {(inventory?.sources ?? []).length === 0 ? (
              <p className="rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                {t('marketplaceNoSources')}
              </p>
            ) : null}
          </div>
        </Card>

        <Card className="rounded-[28px] border-border/70 bg-card/92 p-5 shadow-sm">
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t('marketplaceSnapshots')}
              </div>
              <h3 className="mt-2 text-lg font-semibold text-foreground">
                {t('marketplaceSnapshotsHeadline')}
              </h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t('marketplaceSnapshotsDescription')}
              </p>
            </div>
            <div className="rounded-full bg-muted/60 px-3 py-1 text-xs text-muted-foreground">
              {t('marketplaceSavedCount', String(snapshotItems.length))}
            </div>
          </div>

          <div className="mt-4 space-y-3">
            <Input
              value={snapshotLabel}
              onChange={(event) => setSnapshotLabel(event.target.value)}
              placeholder={t('marketplaceSnapshotLabel')}
              className="h-11 rounded-2xl"
            />
            <Input
              value={snapshotNote}
              onChange={(event) => setSnapshotNote(event.target.value)}
              placeholder={t('marketplaceSnapshotNote')}
              className="h-11 rounded-2xl"
            />
            <Button
              onClick={handleCreateSnapshot}
              disabled={createSnapshot.isPending}
              className="rounded-xl"
            >
              <TimerReset className="mr-2 h-4 w-4" />
              {createSnapshot.isPending ? t('marketplaceCreatingSnapshot') : t('marketplaceCreateSnapshot')}
            </Button>
            <Button
              variant="outline"
              onClick={() => pruneSnapshots.mutate()}
              disabled={pruneSnapshots.isPending || snapshotMaxCount < 1 || !canPruneSnapshots}
              className="rounded-xl"
            >
              <RefreshCcw className="mr-2 h-4 w-4" />
              {pruneSnapshots.isPending ? t('marketplacePruning') : t('marketplacePruneTo', String(snapshotMaxCount || 0))}
            </Button>
            <p className="text-xs leading-5 text-muted-foreground">
              {snapshotMaxCount > 0
                ? t('marketplaceRetentionPolicy', String(snapshotMaxCount))
                : t('marketplaceRetentionUnset')}
            </p>
          </div>

          <div className="mt-4 space-y-3">
            {snapshotItems.map((snapshot) => (
              <SnapshotCard
                key={snapshot.id}
                snapshot={snapshot}
                isRestoring={restoreSnapshot.isPending && restoreSnapshot.variables === snapshot.id}
                isDeleting={deleteSnapshot.isPending && deleteSnapshot.variables === snapshot.id}
                onRestore={(id) => restoreSnapshot.mutate(id)}
                onDelete={(id) => {
                  setDeleteSnapshotId(id);
                  setShowDeleteConfirm(true);
                }}
              />
            ))}
            {snapshotItems.length === 0 ? (
              <p className="rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                {t('marketplaceNoSnapshots')}
              </p>
            ) : null}
          </div>
        </Card>
      </section>

      {isLoading && (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
          <Skeleton className="h-[min(62vh,720px)] min-h-[360px] rounded-[28px]" />
          <Skeleton className="h-[min(62vh,720px)] min-h-[360px] rounded-[28px]" />
        </div>
      )}

      {!isLoading && marketplaceSkills.length === 0 && (
        <div className="rounded-[28px] border border-dashed border-border bg-card/90 px-6 py-20 text-center shadow-sm">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-[20px] bg-foreground text-background">
            <FileCode2 className="h-6 w-6" />
          </div>
          <h3 className="text-lg font-semibold text-foreground">{t('marketplaceEmptyTitle')}</h3>
          <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-muted-foreground">
            {t('marketplaceEmptyDescription')}
          </p>
          <div className="mx-auto mt-6 flex max-w-md flex-col items-center gap-3 sm:flex-row sm:justify-center">
            <Input
              value={installSource}
              onChange={(event) => setInstallSource(event.target.value)}
              placeholder={t('marketplaceInstallPlaceholder')}
              className="h-11 rounded-2xl"
            />
            <Button
              onClick={handleInstallSkill}
              disabled={!installSource.trim() || installSkill.isPending}
              className="w-full rounded-xl sm:w-auto"
            >
              <Download className="mr-2 h-4 w-4" />
              {installSkill.isPending ? t('marketplaceInstalling') : t('marketplaceInstallSkill')}
            </Button>
          </div>
        </div>
      )}

      {!isLoading && marketplaceSkills.length > 0 && (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
          <Card className="overflow-hidden rounded-[28px] border-border/70 bg-card/92 shadow-sm">
            <div className="border-b border-border/70 px-5 py-4">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h3 className="text-sm font-semibold text-foreground">{t('marketplaceSkillsTitle')}</h3>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t('marketplaceVisibleReadyCount', String(filteredSkills.length), String(readyCount))}
                  </p>
                </div>
                <div className="inline-flex items-center gap-1 rounded-full bg-muted/60 px-2.5 py-1 text-[11px] font-medium text-muted-foreground">
                  <ToggleLeft className="h-3.5 w-3.5" />
                  {t('marketplaceLocalInstall')}
                </div>
              </div>
            </div>

            <ScrollArea className="h-[min(62vh,720px)] min-h-[360px] md:h-[720px]">
              <div className="space-y-2 p-3">
                {filteredSkills.map((skill) => (
                  <SkillListItem
                    key={skill.id}
                    skill={skill}
                    selected={selectedSkillID === skill.id}
                    onSelect={() => setSelectedSkillID(skill.id)}
                  />
                ))}
              </div>
            </ScrollArea>
          </Card>

          <Card className="overflow-hidden rounded-[28px] border-border/70 bg-card/92 shadow-sm">
            {!selectedSkillID && (
              <div className="flex h-[min(62vh,720px)] min-h-[360px] flex-col items-center justify-center px-6 text-center md:h-[720px]">
                <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-[20px] bg-muted text-muted-foreground">
                  <FileText className="h-6 w-6" />
                </div>
                <h3 className="text-lg font-semibold text-foreground">{t('marketplaceSelectSkill')}</h3>
                <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
                  {t('marketplaceSelectSkillDescription')}
                </p>
              </div>
            )}

            {selectedSkillID && (
              <div className="flex h-[min(62vh,720px)] min-h-[360px] flex-col md:h-[720px]">
                <div className="border-b border-border/70 px-5 py-5">
                  {isLoadingItem ? (
                    <Skeleton className="h-24 rounded-2xl" />
                  ) : selectedSkill ? (
                    <SkillDetailHeader
                      skill={selectedSkill}
                      isBusy={
                        (enableSkill.isPending && enableSkill.variables === selectedSkill.id) ||
                        (disableSkill.isPending && disableSkill.variables === selectedSkill.id) ||
                        (installDependencies.isPending && installDependencies.variables === selectedSkill.id)
                      }
                      onEnable={handleEnable}
                      onDisable={handleDisable}
                      onInstallDependencies={handleInstallDependencies}
                    />
                  ) : null}
                </div>

                <ScrollArea className="flex-1">
                  <div className="space-y-5 p-5">
                    <section className="grid grid-cols-1 gap-3 lg:grid-cols-3">
                      <SkillInfoCard
                        icon={<BadgeCheck className="h-4 w-4" />}
                        label={t('marketplaceVersion')}
                        value={selectedSkill?.version || '-'}
                      />
                      <SkillInfoCard
                        icon={<Pin className="h-4 w-4" />}
                        label={t('marketplaceAuthor')}
                        value={selectedSkill?.author || '-'}
                      />
                      <SkillInfoCard
                        icon={<ToggleLeft className="h-4 w-4" />}
                        label={t('marketplaceState')}
                        value={selectedSkill?.enabled ? t('marketplaceEnabled') : t('marketplaceDisabled')}
                      />
                      <SkillInfoCard
                        icon={
                          selectedSkill?.eligible ? (
                            <ShieldCheck className="h-4 w-4" />
                          ) : (
                            <ShieldAlert className="h-4 w-4" />
                          )
                        }
                        label={t('marketplaceRuntime')}
                        value={selectedSkill?.eligible ? t('marketplaceReady') : t('marketplaceNeedsSetup')}
                      />
                    </section>

                    <section className="rounded-[24px] border border-border/70 bg-muted/35 p-4">
                      <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t('marketplaceDescriptionLabel')}</div>
                      <p className="mt-2 text-sm leading-6 text-foreground">
                        {selectedSkill?.description || t('marketplaceNoDescription')}
                      </p>
                      <div className="mt-4 flex flex-wrap gap-2">
                        {selectedTags.length > 0 ? (
                          selectedTags.map((tag) => (
                            <span
                              key={tag}
                              className="rounded-full bg-background/80 px-2.5 py-1 text-xs font-medium text-foreground shadow-sm"
                            >
                              {tag}
                            </span>
                          ))
                        ) : (
                          <span className="text-xs text-muted-foreground">{t('marketplaceNoTags')}</span>
                        )}
                      </div>
                    </section>

                    <section className="rounded-[24px] border border-border/70 bg-card p-4">
                      <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                        {t('marketplaceFilePath')}
                      </div>
                      <p className="mt-2 break-all text-sm leading-6 text-foreground">
                        {selectedSkill?.file_path || '-'}
                      </p>
                    </section>

                    <section
                      className={cn(
                        'rounded-[24px] border p-4',
                        selectedSkill?.eligible
                          ? 'border-emerald-200/80 bg-emerald-50/70'
                          : 'border-amber-200/80 bg-amber-50/80',
                      )}
                    >
                      <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                        {selectedSkill?.eligible ? (
                          <ShieldCheck className="h-4 w-4 text-emerald-600" />
                        ) : (
                          <AlertTriangle className="h-4 w-4 text-amber-600" />
                        )}
                        {t('marketplaceRuntimeReadiness')}
                      </div>
                      <p className="mt-2 text-sm leading-6 text-foreground">
                        {selectedSkill?.eligible
                          ? t('marketplaceRuntimeReadyDescription')
                          : t('marketplaceRuntimeBlockedDescription')}
                      </p>
                      {!selectedSkill?.eligible && selectedReasons.length > 0 && (
                        <div className="mt-4 flex flex-wrap gap-2">
                          {selectedReasons.map((reason) => (
                            <span
                              key={reason}
                              className="rounded-full border border-amber-200 bg-white px-2.5 py-1 text-xs text-amber-800"
                            >
                              {reason}
                            </span>
                          ))}
                        </div>
                      )}
                      {selectedSkill && (
                        <div className="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2">
                          <MissingRequirementGroup
                            label={t('marketplaceConfigPaths')}
                            values={selectedMissingRequirements.config_paths}
                          />
                          <MissingRequirementGroup
                            label={t('marketplaceEnvironment')}
                            values={selectedMissingRequirements.env}
                          />
                          <MissingRequirementGroup
                            label={t('marketplaceBinaries')}
                            values={selectedMissingRequirements.binaries}
                          />
                          <MissingRequirementGroup
                            label={t('marketplaceAnyBinaries')}
                            values={selectedMissingRequirements.any_binaries}
                          />
                          <MissingRequirementGroup
                            label={t('marketplacePythonPackages')}
                            values={selectedMissingRequirements.python_packages}
                          />
                          <MissingRequirementGroup
                            label={t('marketplaceNodePackages')}
                            values={selectedMissingRequirements.node_packages}
                          />
                        </div>
                      )}
                    </section>

                    <section className="rounded-[24px] border border-border/70 bg-card p-4">
                      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                        <div>
                          <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                            <Wrench className="h-4 w-4" />
                            {t('marketplaceDependencyPlan')}
                          </div>
                          <p className="mt-2 text-sm leading-6 text-muted-foreground">
                            {t('marketplaceDependencyDescription')}
                          </p>
                        </div>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={
                            !selectedSkill ||
                            selectedInstallSpecs.length === 0 ||
                            (installDependencies.isPending && installDependencies.variables === selectedSkill.id)
                          }
                          onClick={() => selectedSkill && handleInstallDependencies(selectedSkill.id)}
                          className="rounded-xl"
                        >
                          {installDependencies.isPending && installDependencies.variables === selectedSkill?.id
                            ? t('marketplaceInstalling')
                            : t('marketplaceInstallDependencies')}
                        </Button>
                      </div>

                      <div className="mt-4 space-y-3">
                        {selectedInstallSpecs.length > 0 ? (
                          selectedInstallSpecs.map((spec, index) => (
                            <div
                              key={`${spec.method}-${spec.package}-${index}`}
                              className="rounded-2xl border border-border/70 bg-muted/35 p-4"
                            >
                              <div className="flex flex-wrap items-center gap-2">
                                <span className="rounded-full bg-foreground px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-background">
                                  {spec.method}
                                </span>
                                <span className="text-sm font-semibold text-foreground">
                                  {spec.package || '-'}
                                </span>
                                {spec.version ? (
                                  <span className="rounded-full bg-background/80 px-2 py-0.5 text-[11px] text-muted-foreground">
                                    v{spec.version}
                                  </span>
                                ) : null}
                              </div>
                              {spec.post_hook ? (
                                <p className="mt-2 text-xs leading-6 text-muted-foreground">
                                  {t('marketplacePostHook')}: <span className="font-mono text-foreground">{spec.post_hook}</span>
                                </p>
                              ) : null}
                            </div>
                          ))
                        ) : (
                          <p className="text-sm text-muted-foreground">{t('marketplaceNoInstallSteps')}</p>
                        )}
                      </div>

                      {selectedInstallResults.length > 0 && (
                        <div className="mt-4 space-y-3">
                          {selectedInstallResults.map((result, index) => (
                            <InstallResultCard
                              key={`${result.method}-${result.package}-${index}`}
                              result={result}
                            />
                          ))}
                        </div>
                      )}
                    </section>

                    <section className="rounded-[24px] border border-slate-200/80 bg-slate-950 text-slate-100 shadow-inner">
                      <div className="border-b border-white/10 px-4 py-3">
                        <div className="text-xs font-medium uppercase tracking-[0.18em] text-slate-400">
                          {t('marketplaceParsedBody')}
                        </div>
                      </div>
                      <div className="max-h-[280px] overflow-auto px-4 py-4">
                        {isLoadingContent ? (
                          <Skeleton className="h-52 rounded-2xl bg-white/10" />
                        ) : (
                          <pre className="whitespace-pre-wrap break-words font-mono text-xs leading-6 text-slate-200">
                            {selectedContent?.body_raw || ''}
                          </pre>
                        )}
                      </div>
                    </section>

                    <section className="rounded-[24px] border border-border/70 bg-card">
                      <div className="border-b border-border/70 px-4 py-3">
                        <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                          {t('marketplaceRawSource')}
                        </div>
                      </div>
                      <div className="max-h-[320px] overflow-auto px-4 py-4">
                        {isLoadingContent ? (
                          <Skeleton className="h-64 rounded-2xl" />
                        ) : (
                          <pre className="whitespace-pre-wrap break-words font-mono text-xs leading-6 text-foreground">
                            {selectedContent?.raw || ''}
                          </pre>
                        )}
                      </div>
                    </section>
                  </div>
                </ScrollArea>
              </div>
            )}
          </Card>
        </div>
      )}

      {/* Delete Snapshot Confirmation Dialog */}
      <Dialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <DialogPortal>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('snapshotDeleteConfirmTitle')}</DialogTitle>
              <DialogDescription>
                {t('snapshotDeleteConfirmDescription')}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowDeleteConfirm(false)}>
                {t('cancel')}
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  if (deleteSnapshotId) {
                    deleteSnapshot.mutate(deleteSnapshotId);
                    setShowDeleteConfirm(false);
                    setDeleteSnapshotId('');
                  }
                }}
                disabled={deleteSnapshot.isPending}
              >
                {deleteSnapshot.isPending ? (
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

function SkillMetric({
  label,
  value,
  muted,
}: {
  label: string;
  value: string;
  muted?: boolean;
}) {
  return (
    <div className="min-w-[120px] rounded-2xl border border-border/70 bg-card/90 px-4 py-3 shadow-sm">
      <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className={cn('mt-1 text-base font-semibold text-foreground', muted && 'text-muted-foreground')}>{value}</div>
    </div>
  );
}

function SkillListItem({
  skill,
  selected,
  onSelect,
}: {
  skill: MarketplaceSkill;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        'w-full rounded-[22px] border p-4 text-left transition-all',
        selected
          ? 'border-emerald-300/70 bg-emerald-500/10 shadow-sm'
          : 'border-border/70 bg-card hover:border-border hover:bg-muted/40',
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="truncate text-sm font-semibold text-foreground">{skill.name || skill.id}</h3>
            {skill.always && (
              <span className="rounded-full bg-sky-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.12em] text-sky-700">
                {t('marketplaceAlwaysOn')}
              </span>
            )}
          </div>
          <p className="mt-1 truncate text-xs text-muted-foreground">{skill.id}</p>
        </div>
        <span
          className={cn(
            'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium',
            skill.enabled ? 'bg-emerald-100 text-emerald-700' : 'bg-muted text-muted-foreground',
          )}
        >
          <span className={cn('h-1.5 w-1.5 rounded-full', skill.enabled ? 'bg-emerald-500' : 'bg-slate-400')} />
          {skill.enabled ? t('marketplaceEnabled') : t('marketplaceDisabled')}
        </span>
      </div>

      <p className="mt-3 line-clamp-2 text-sm leading-6 text-muted-foreground">
        {skill.description || t('marketplaceNoDescription')}
      </p>

      <div className="mt-3 flex flex-wrap gap-2">
        <span
          className={cn(
            'rounded-full px-2 py-0.5 text-[11px] font-medium',
            skill.eligible ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700',
          )}
        >
          {skill.eligible ? t('marketplaceReady') : t('marketplaceNeedsSetup')}
        </span>
        {skill.install_specs.length > 0 ? (
          <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
            {t(
              skill.install_specs.length > 1 ? 'marketplaceInstallStepsPlural' : 'marketplaceInstallSteps',
              String(skill.install_specs.length),
            )}
          </span>
        ) : null}
      </div>

      <div className="mt-3 flex flex-wrap gap-2">
        {(skill.tags ?? []).slice(0, 3).map((tag) => (
          <span key={tag} className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
            {tag}
          </span>
        ))}
      </div>
    </button>
  );
}

function SkillDetailHeader({
  skill,
  isBusy,
  onEnable,
  onDisable,
  onInstallDependencies,
}: {
  skill: MarketplaceSkill;
  isBusy: boolean;
  onEnable: (id: string) => void;
  onDisable: (id: string) => void;
  onInstallDependencies: (id: string) => void;
}) {
  return (
    <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
      <div className="space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="text-xl font-semibold tracking-tight text-foreground">{skill.name || skill.id}</h3>
          {skill.always && (
            <span className="inline-flex items-center gap-1 rounded-full bg-sky-100 px-2.5 py-1 text-xs font-medium text-sky-700">
              <Pin className="h-3.5 w-3.5" />
              {t('marketplaceAlwaysOn')}
            </span>
          )}
        </div>
        <p className="text-sm text-muted-foreground">{skill.id}</p>
      </div>

      <div className="flex items-center gap-2">
        <Button
          size="sm"
          variant="outline"
          disabled={isBusy || skill.install_specs.length === 0}
          onClick={() => onInstallDependencies(skill.id)}
          className="rounded-xl"
        >
          {t('marketplaceInstallDependencies')}
        </Button>
        {skill.enabled ? (
          <Button
            size="sm"
            variant="outline"
            disabled={isBusy || skill.always}
            onClick={() => onDisable(skill.id)}
            className="rounded-xl"
          >
            {isBusy ? t('marketplaceDisabling') : t('marketplaceDisable')}
          </Button>
        ) : (
          <Button
            size="sm"
            disabled={isBusy || skill.always}
            onClick={() => onEnable(skill.id)}
            className="rounded-xl"
          >
            {isBusy ? t('marketplaceEnabling') : t('marketplaceEnable')}
          </Button>
        )}
      </div>
    </div>
  );
}

function InstallResultCard({ result }: { result: MarketplaceInstallResult }) {
  return (
    <div
      className={cn(
        'rounded-2xl border p-4',
        result.success ? 'border-emerald-200/80 bg-emerald-50/70' : 'border-rose-200/80 bg-rose-50/80',
      )}
    >
      <div className="flex flex-wrap items-center gap-2">
        <span
          className={cn(
            'rounded-full px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em]',
            result.success ? 'bg-emerald-600 text-white' : 'bg-rose-600 text-white',
          )}
        >
          {result.success ? t('marketplaceInstallResultSuccess') : t('marketplaceInstallResultFailed')}
        </span>
        <span className="text-sm font-semibold text-foreground">
          {result.method} · {result.package}
        </span>
        <span className="text-xs text-muted-foreground">{result.duration_ms} ms</span>
      </div>
      {result.error ? (
        <p className="mt-2 text-sm leading-6 text-rose-700">{result.error}</p>
      ) : null}
      {result.output ? (
        <pre className="mt-3 whitespace-pre-wrap break-words rounded-xl bg-background/70 p-3 font-mono text-xs leading-6 text-foreground">
          {result.output}
        </pre>
      ) : null}
    </div>
  );
}

function MissingRequirementGroup({
  label,
  values,
}: {
  label: string;
  values: string[];
}) {
  return (
    <div className="rounded-2xl border border-border/70 bg-background/70 p-3">
      <div className="text-[11px] font-medium uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className="mt-2 flex flex-wrap gap-2">
        {values.length > 0 ? (
          values.map((value) => (
            <span
              key={`${label}-${value}`}
              className="rounded-full border border-amber-200 bg-amber-50 px-2.5 py-1 text-xs text-amber-800"
            >
              {value}
            </span>
          ))
        ) : (
          <span className="text-xs text-muted-foreground">{t('marketplaceNoneMissing')}</span>
        )}
      </div>
    </div>
  );
}

function SkillInfoCard({
  icon,
  label,
  value,
}: {
  icon: ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
      <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
        {icon}
        {label}
      </div>
      <div className="mt-2 break-all text-sm font-semibold text-foreground">{value}</div>
    </div>
  );
}

function SkillSourceRow({ source }: { source: MarketplaceSkillSource }) {
  return (
    <div className="rounded-[22px] border border-border/70 bg-muted/35 p-4">
      <div className="flex flex-wrap items-center gap-2">
        <span className="rounded-full bg-foreground px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-background">
          {source.type}
        </span>
        <span className="rounded-full bg-background/80 px-2.5 py-1 text-[11px] text-muted-foreground">
          {t('marketplacePriority', String(source.priority))}
        </span>
        <span
          className={cn(
            'rounded-full px-2.5 py-1 text-[11px] font-medium',
            source.exists ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700',
          )}
        >
          {source.exists ? t('marketplaceAvailable') : t('marketplaceMissing')}
        </span>
      </div>
      <p className="mt-3 break-all font-mono text-xs leading-6 text-foreground">{source.path}</p>
    </div>
  );
}

function SnapshotCard({
  snapshot,
  isRestoring,
  isDeleting,
  onRestore,
  onDelete,
}: {
  snapshot: MarketplaceSnapshot;
  isRestoring: boolean;
  isDeleting: boolean;
  onRestore: (id: string) => void;
  onDelete: (id: string) => void;
}) {
  return (
    <div className="rounded-[22px] border border-border/70 bg-muted/35 p-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="truncate text-sm font-semibold text-foreground">
              {snapshot.metadata.label || snapshot.id}
            </span>
            <span className="rounded-full bg-background/80 px-2 py-0.5 text-[11px] text-muted-foreground">
              {t('marketplaceSkillCount', String(snapshot.skill_count))}
            </span>
            <span className="rounded-full bg-background/80 px-2 py-0.5 text-[11px] text-muted-foreground">
              {t('marketplaceEnabledCount', String(snapshot.enabled_count))}
            </span>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">{snapshot.timestamp}</p>
          {snapshot.metadata.note ? (
            <p className="mt-2 text-sm leading-6 text-muted-foreground">{snapshot.metadata.note}</p>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            className="rounded-xl"
            disabled={isRestoring || isDeleting}
            onClick={() => onRestore(snapshot.id)}
          >
            <RotateCcw className="mr-2 h-4 w-4" />
            {isRestoring ? t('marketplaceRestoring') : t('marketplaceRestore')}
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="rounded-xl"
            disabled={isRestoring || isDeleting}
            onClick={() => onDelete(snapshot.id)}
          >
            {isDeleting ? t('marketplaceDeleting') : t('delete')}
          </Button>
        </div>
      </div>
    </div>
  );
}
