import { useEffect, useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useDisableMarketplaceSkill,
  useEnableMarketplaceSkill,
  useInstallMarketplaceSkillDependencies,
  useInstalledMarketplaceSkills,
  useMarketplaceSkillContent,
  useMarketplaceSkillItem,
  useMarketplaceSkills,
  type MarketplaceInstallResult,
  type MarketplaceSkill,
} from '@/hooks/useMarketplace';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import {
  AlertTriangle,
  BadgeCheck,
  FileCode2,
  FileText,
  Pin,
  Search,
  ShieldAlert,
  ShieldCheck,
  Sparkles,
  ToggleLeft,
  Wrench,
} from 'lucide-react';

export default function MarketplacePage() {
  const { data: skills, isLoading } = useMarketplaceSkills();
  const { data: installed } = useInstalledMarketplaceSkills();
  const enableSkill = useEnableMarketplaceSkill();
  const disableSkill = useDisableMarketplaceSkill();
  const installDependencies = useInstallMarketplaceSkillDependencies();

  const [query, setQuery] = useState('');
  const [selectedSkillID, setSelectedSkillID] = useState<string | null>(null);

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

  const installedCount = installed?.total ?? marketplaceSkills.filter((skill) => skill.enabled).length;
  const alwaysOnCount = marketplaceSkills.filter((skill) => skill.always).length;
  const readyCount = marketplaceSkills.filter((skill) => skill.eligible).length;
  const selectedInstallResults =
    installDependencies.data?.skill_id === selectedSkillID ? installDependencies.data.results : [];

  return (
    <div className="space-y-6">
      <Header
        title={t('tabMarketplace')}
        description="Review installed skills, inspect raw content, and toggle capabilities without leaving the dashboard."
      />

      <section className="relative overflow-hidden rounded-[28px] border border-emerald-200/70 bg-[radial-gradient(circle_at_top_left,_rgba(16,185,129,0.14),_transparent_40%),linear-gradient(135deg,_rgba(255,255,255,0.98),_rgba(236,253,245,0.72))] p-5 shadow-sm sm:p-6">
        <div className="absolute bottom-0 right-0 h-40 w-40 rounded-full bg-emerald-100/60 blur-3xl" />
        <div className="relative flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-3">
            <div className="inline-flex items-center gap-2 rounded-full border border-emerald-200/70 bg-white/90 px-3 py-1 text-xs font-medium text-emerald-700 shadow-sm">
              <Sparkles className="h-3.5 w-3.5" />
              Installed skills
            </div>
            <div className="space-y-2">
              <h2 className="max-w-2xl text-2xl font-semibold tracking-tight text-slate-900">
                Treat skills like runtime modules, not hidden markdown files.
              </h2>
              <p className="max-w-2xl text-sm leading-6 text-slate-600">
                Inspect runtime readiness, missing requirements, install plans, and parsed content
                before enabling a skill in production.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <SkillMetric label="Installed" value={String(installedCount)} />
              <SkillMetric label="Ready" value={String(readyCount)} />
              <SkillMetric label="Always on" value={String(alwaysOnCount)} />
              <SkillMetric label="Selected" value={selectedSkill?.name ?? 'None'} muted={!selectedSkill} />
            </div>
          </div>

          <div className="w-full lg:w-[340px]">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Search skills, tags, author"
                className="h-11 rounded-2xl border-emerald-200/60 bg-white/90 pl-9 shadow-sm"
              />
            </div>
          </div>
        </div>
      </section>

      {isLoading && (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
          <Skeleton className="h-[720px] rounded-[28px]" />
          <Skeleton className="h-[720px] rounded-[28px]" />
        </div>
      )}

      {!isLoading && marketplaceSkills.length === 0 && (
        <div className="rounded-[28px] border border-dashed border-slate-300 bg-white/90 px-6 py-20 text-center shadow-sm">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-[20px] bg-slate-900 text-white">
            <FileCode2 className="h-6 w-6" />
          </div>
          <h3 className="text-lg font-semibold text-slate-900">{t('marketplaceEmptyTitle')}</h3>
          <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-slate-500">
            {t('marketplaceEmptyDescription')}
          </p>
        </div>
      )}

      {!isLoading && marketplaceSkills.length > 0 && (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
          <Card className="overflow-hidden rounded-[28px] border-slate-200/80 bg-white/95 shadow-sm">
            <div className="border-b border-slate-100 px-5 py-4">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h3 className="text-sm font-semibold text-slate-900">Skills</h3>
                  <p className="mt-1 text-xs text-slate-500">
                    {filteredSkills.length} visible · {readyCount} ready
                  </p>
                </div>
                <div className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2.5 py-1 text-[11px] font-medium text-slate-600">
                  <ToggleLeft className="h-3.5 w-3.5" />
                  Local install
                </div>
              </div>
            </div>

            <ScrollArea className="h-[720px]">
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

          <Card className="overflow-hidden rounded-[28px] border-slate-200/80 bg-white/95 shadow-sm">
            {!selectedSkillID && (
              <div className="flex h-[720px] flex-col items-center justify-center px-6 text-center">
                <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-[20px] bg-slate-100 text-slate-400">
                  <FileText className="h-6 w-6" />
                </div>
                <h3 className="text-lg font-semibold text-slate-900">Select a skill</h3>
                <p className="mt-2 max-w-sm text-sm leading-6 text-slate-500">
                  Choose a skill from the left list to inspect metadata, body content, and runtime status.
                </p>
              </div>
            )}

            {selectedSkillID && (
              <div className="flex h-[720px] flex-col">
                <div className="border-b border-slate-100 px-5 py-5">
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
                        label="State"
                        value={selectedSkill?.enabled ? 'Enabled' : 'Disabled'}
                      />
                      <SkillInfoCard
                        icon={
                          selectedSkill?.eligible ? (
                            <ShieldCheck className="h-4 w-4" />
                          ) : (
                            <ShieldAlert className="h-4 w-4" />
                          )
                        }
                        label="Runtime"
                        value={selectedSkill?.eligible ? 'Ready' : 'Needs setup'}
                      />
                    </section>

                    <section className="rounded-[24px] border border-slate-200/80 bg-slate-50/70 p-4">
                      <div className="text-xs font-medium uppercase tracking-[0.18em] text-slate-400">Description</div>
                      <p className="mt-2 text-sm leading-6 text-slate-700">
                        {selectedSkill?.description || t('marketplaceNoDescription')}
                      </p>
                      <div className="mt-4 flex flex-wrap gap-2">
                        {(selectedSkill?.tags ?? []).length > 0 ? (
                          (selectedSkill?.tags ?? []).map((tag) => (
                            <span
                              key={tag}
                              className="rounded-full bg-white px-2.5 py-1 text-xs font-medium text-slate-700 shadow-sm"
                            >
                              {tag}
                            </span>
                          ))
                        ) : (
                          <span className="text-xs text-slate-500">No tags</span>
                        )}
                      </div>
                    </section>

                    <section className="rounded-[24px] border border-slate-200/80 bg-white p-4">
                      <div className="text-xs font-medium uppercase tracking-[0.18em] text-slate-400">
                        {t('marketplaceFilePath')}
                      </div>
                      <p className="mt-2 break-all text-sm leading-6 text-slate-700">
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
                      <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-slate-500">
                        {selectedSkill?.eligible ? (
                          <ShieldCheck className="h-4 w-4 text-emerald-600" />
                        ) : (
                          <AlertTriangle className="h-4 w-4 text-amber-600" />
                        )}
                        Runtime readiness
                      </div>
                      <p className="mt-2 text-sm leading-6 text-slate-700">
                        {selectedSkill?.eligible
                          ? 'This skill passes the current runtime checks and can be enabled immediately.'
                          : 'This skill is blocked by missing requirements in the current runtime environment.'}
                      </p>
                      {!selectedSkill?.eligible && (selectedSkill?.ineligibility_reasons?.length ?? 0) > 0 && (
                        <div className="mt-4 flex flex-wrap gap-2">
                          {selectedSkill?.ineligibility_reasons.map((reason) => (
                            <span
                              key={reason}
                              className="rounded-full border border-amber-200 bg-white px-2.5 py-1 text-xs text-amber-800"
                            >
                              {reason}
                            </span>
                          ))}
                        </div>
                      )}
                    </section>

                    <section className="rounded-[24px] border border-slate-200/80 bg-white p-4">
                      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                        <div>
                          <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-slate-400">
                            <Wrench className="h-4 w-4" />
                            Dependency plan
                          </div>
                          <p className="mt-2 text-sm leading-6 text-slate-600">
                            Review the install steps parsed from skill metadata and run them from the dashboard when needed.
                          </p>
                        </div>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={
                            !selectedSkill ||
                            selectedSkill.install_specs.length === 0 ||
                            (installDependencies.isPending && installDependencies.variables === selectedSkill.id)
                          }
                          onClick={() => selectedSkill && handleInstallDependencies(selectedSkill.id)}
                          className="rounded-xl"
                        >
                          {installDependencies.isPending && installDependencies.variables === selectedSkill?.id
                            ? 'Installing…'
                            : 'Install dependencies'}
                        </Button>
                      </div>

                      <div className="mt-4 space-y-3">
                        {(selectedSkill?.install_specs ?? []).length > 0 ? (
                          selectedSkill?.install_specs.map((spec, index) => (
                            <div
                              key={`${spec.method}-${spec.package}-${index}`}
                              className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4"
                            >
                              <div className="flex flex-wrap items-center gap-2">
                                <span className="rounded-full bg-slate-900 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-white">
                                  {spec.method}
                                </span>
                                <span className="text-sm font-semibold text-slate-900">
                                  {spec.package || '-'}
                                </span>
                                {spec.version ? (
                                  <span className="rounded-full bg-white px-2 py-0.5 text-[11px] text-slate-600">
                                    v{spec.version}
                                  </span>
                                ) : null}
                              </div>
                              {spec.post_hook ? (
                                <p className="mt-2 text-xs leading-6 text-slate-500">
                                  Post hook: <span className="font-mono text-slate-700">{spec.post_hook}</span>
                                </p>
                              ) : null}
                            </div>
                          ))
                        ) : (
                          <p className="text-sm text-slate-500">No install steps declared for this skill.</p>
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
                          Parsed body
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

                    <section className="rounded-[24px] border border-slate-200/80 bg-white">
                      <div className="border-b border-slate-100 px-4 py-3">
                        <div className="text-xs font-medium uppercase tracking-[0.18em] text-slate-400">
                          Raw source
                        </div>
                      </div>
                      <div className="max-h-[320px] overflow-auto px-4 py-4">
                        {isLoadingContent ? (
                          <Skeleton className="h-64 rounded-2xl" />
                        ) : (
                          <pre className="whitespace-pre-wrap break-words font-mono text-xs leading-6 text-slate-700">
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
    <div className="min-w-[120px] rounded-2xl border border-emerald-200/70 bg-white/90 px-4 py-3 shadow-sm">
      <div className="text-[11px] uppercase tracking-[0.18em] text-slate-400">{label}</div>
      <div className={cn('mt-1 text-base font-semibold text-slate-900', muted && 'text-slate-500')}>{value}</div>
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
          ? 'border-emerald-300 bg-emerald-50/70 shadow-sm'
          : 'border-slate-200/80 bg-white hover:border-slate-300 hover:bg-slate-50/70',
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="truncate text-sm font-semibold text-slate-900">{skill.name || skill.id}</h3>
            {skill.always && (
              <span className="rounded-full bg-sky-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.12em] text-sky-700">
                Always
              </span>
            )}
          </div>
          <p className="mt-1 truncate text-xs text-slate-500">{skill.id}</p>
        </div>
        <span
          className={cn(
            'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium',
            skill.enabled ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-100 text-slate-600',
          )}
        >
          <span className={cn('h-1.5 w-1.5 rounded-full', skill.enabled ? 'bg-emerald-500' : 'bg-slate-400')} />
          {skill.enabled ? t('on') : t('off')}
        </span>
      </div>

      <p className="mt-3 line-clamp-2 text-sm leading-6 text-slate-600">
        {skill.description || t('marketplaceNoDescription')}
      </p>

      <div className="mt-3 flex flex-wrap gap-2">
        <span
          className={cn(
            'rounded-full px-2 py-0.5 text-[11px] font-medium',
            skill.eligible ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700',
          )}
        >
          {skill.eligible ? 'Ready' : 'Needs setup'}
        </span>
        {skill.install_specs.length > 0 ? (
          <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] text-slate-600">
            {skill.install_specs.length} install step{skill.install_specs.length > 1 ? 's' : ''}
          </span>
        ) : null}
      </div>

      <div className="mt-3 flex flex-wrap gap-2">
        {(skill.tags ?? []).slice(0, 3).map((tag) => (
          <span key={tag} className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] text-slate-600">
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
          <h3 className="text-xl font-semibold tracking-tight text-slate-900">{skill.name || skill.id}</h3>
          {skill.always && (
            <span className="inline-flex items-center gap-1 rounded-full bg-sky-100 px-2.5 py-1 text-xs font-medium text-sky-700">
              <Pin className="h-3.5 w-3.5" />
              Always on
            </span>
          )}
        </div>
        <p className="text-sm text-slate-500">{skill.id}</p>
      </div>

      <div className="flex items-center gap-2">
        <Button
          size="sm"
          variant="outline"
          disabled={isBusy || skill.install_specs.length === 0}
          onClick={() => onInstallDependencies(skill.id)}
          className="rounded-xl"
        >
          Install deps
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
          {result.success ? 'Success' : 'Failed'}
        </span>
        <span className="text-sm font-semibold text-slate-900">
          {result.method} · {result.package}
        </span>
        <span className="text-xs text-slate-500">{result.duration_ms} ms</span>
      </div>
      {result.error ? (
        <p className="mt-2 text-sm leading-6 text-rose-700">{result.error}</p>
      ) : null}
      {result.output ? (
        <pre className="mt-3 whitespace-pre-wrap break-words rounded-xl bg-white/80 p-3 font-mono text-xs leading-6 text-slate-700">
          {result.output}
        </pre>
      ) : null}
    </div>
  );
}

function SkillInfoCard({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
      <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-slate-400">
        {icon}
        {label}
      </div>
      <div className="mt-2 text-sm font-semibold text-slate-900">{value}</div>
    </div>
  );
}
