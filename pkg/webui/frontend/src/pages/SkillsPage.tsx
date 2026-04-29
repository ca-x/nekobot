import { useMemo, useState } from 'react';
import Header from '@/components/layout/Header';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useDisableMarketplaceSkill,
  useEnableMarketplaceSkill,
  useMarketplaceSkillContent,
  useMarketplaceSkillItem,
  useMarketplaceSkills,
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
  ToggleLeft,
} from 'lucide-react';
import type { ReactNode } from 'react';

export default function SkillsPage() {
  const { data: skills, isLoading } = useMarketplaceSkills();
  const enableSkill = useEnableMarketplaceSkill();
  const disableSkill = useDisableMarketplaceSkill();

  const [query, setQuery] = useState('');
  const [selectedSkillID, setSelectedSkillID] = useState<string | null>(null);

  const { data: selectedSkill, isLoading: isLoadingItem } = useMarketplaceSkillItem(selectedSkillID);
  const { data: selectedContent, isLoading: isLoadingContent } = useMarketplaceSkillContent(selectedSkillID);

  const filteredSkills = useMemo(() => {
    if (!skills) return [];
    const q = query.trim().toLowerCase();
    if (!q) return skills;
    return skills.filter(
      (s) =>
        s.id.toLowerCase().includes(q) ||
        (s.name ?? '').toLowerCase().includes(q) ||
        (s.description ?? '').toLowerCase().includes(q) ||
        (s.author ?? '').toLowerCase().includes(q) ||
        (s.tags ?? []).some((t) => t.toLowerCase().includes(q))
    );
  }, [skills, query]);

  const selectedTags = useMemo(
    () => (selectedSkill?.tags ?? []).filter(Boolean),
    [selectedSkill]
  );

  const selectedMissingRequirements = useMemo(
    () =>
      selectedSkill?.missing_requirements ?? {
        binaries: [],
        any_binaries: [],
        env: [],
        config_paths: [],
        python_packages: [],
        node_packages: [],
      },
    [selectedSkill]
  );

  const selectedReasons = useMemo(
    () => (selectedSkill?.ineligibility_reasons ?? []).filter(Boolean),
    [selectedSkill]
  );

  const handleEnable = (id: string) => enableSkill.mutate(id);
  const handleDisable = (id: string) => disableSkill.mutate(id);

  const marketplaceSkills = skills ?? [];

  return (
    <div className="space-y-6">
      <Header
        title={t('tabSkills')}
        description={t('skillsHeaderDescription')}
      />

      {isLoading && (
        <div className="space-y-4">
          <Skeleton className="h-12 rounded-2xl" />
          <Skeleton className="h-[60vh] rounded-[28px]" />
        </div>
      )}

      {!isLoading && marketplaceSkills.length === 0 && (
        <div className="rounded-[28px] border border-dashed border-border bg-card/90 px-6 py-20 text-center shadow-sm">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-[20px] bg-foreground text-background">
            <FileCode2 className="h-6 w-6" />
          </div>
          <h3 className="text-lg font-semibold text-foreground">{t('marketplaceEmptyTitle')}</h3>
          <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-muted-foreground">
            {t('skillsEmptyDescription')}
          </p>
        </div>
      )}

      {!isLoading && marketplaceSkills.length > 0 && (
        <>
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t('marketplaceSearchPlaceholder')}
              className="h-11 rounded-2xl border-border/70 bg-card/90 pl-9 shadow-sm"
            />
          </div>

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
            <Card className="overflow-hidden rounded-[28px] border-border/70 bg-card/92 shadow-sm">
              <div className="border-b border-border/70 px-5 py-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h3 className="text-sm font-semibold text-foreground">{t('marketplaceSkillsTitle')}</h3>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {t('marketplaceVisibleReadyCount', String(filteredSkills.length), String(readyCount(skills ?? [])))}
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
                          (disableSkill.isPending && disableSkill.variables === selectedSkill.id)
                        }
                        onEnable={handleEnable}
                        onDisable={handleDisable}
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
                            : 'border-amber-200/80 bg-amber-50/80'
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
        </>
      )}
    </div>
  );
}

function readyCount(skills: MarketplaceSkill[]): number {
  return skills.filter((s) => s.eligible && s.enabled).length;
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
          : 'border-border/70 bg-card hover:border-border hover:bg-muted/40'
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
            skill.enabled ? 'bg-emerald-100 text-emerald-700' : 'bg-muted text-muted-foreground'
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
            skill.eligible ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700'
          )}
        >
          {skill.eligible ? t('marketplaceReady') : t('marketplaceNeedsSetup')}
        </span>
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
}: {
  skill: MarketplaceSkill;
  isBusy: boolean;
  onEnable: (id: string) => void;
  onDisable: (id: string) => void;
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
