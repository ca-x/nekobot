import type { ReactNode } from 'react';
import Header from '@/components/layout/Header';
import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useRuntimeTopology } from '@/hooks/useTopology';
import { t } from '@/lib/i18n';
import { Bot, Link2, RadioTower, Sparkles } from 'lucide-react';

export default function RuntimeTopologyPage() {
  const { data, isLoading } = useRuntimeTopology();

  return (
    <div>
      <Header title={t('tabRuntimeTopology')} description={t('runtimeTopologyDescription')} />

      {isLoading ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-32 rounded-3xl" />
          ))}
        </div>
      ) : null}

      {!isLoading && data ? (
        <div className="space-y-5">
          <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <MetricCard
              icon={<Bot className="h-4 w-4" />}
              label={t('runtimeTopologyRuntimes')}
              value={String(data.summary.runtime_count)}
              accent="from-sky-500/18 via-sky-500/8 to-transparent"
            />
            <MetricCard
              icon={<RadioTower className="h-4 w-4" />}
              label={t('runtimeTopologyAccounts')}
              value={String(data.summary.channel_account_count)}
              accent="from-emerald-500/18 via-emerald-500/8 to-transparent"
            />
            <MetricCard
              icon={<Link2 className="h-4 w-4" />}
              label={t('runtimeTopologyBindings')}
              value={String(data.summary.binding_count)}
              accent="from-amber-500/18 via-amber-500/8 to-transparent"
            />
            <MetricCard
              icon={<Sparkles className="h-4 w-4" />}
              label={t('runtimeTopologyMultiAgent')}
              value={`${data.summary.multi_agent_accounts}/${data.summary.channel_account_count}`}
              accent="from-violet-500/18 via-violet-500/8 to-transparent"
            />
          </section>

          <section className="grid gap-5 xl:grid-cols-[1.1fr_1fr]">
            <Card className="rounded-[28px] border-border/70 bg-card/92 shadow-sm">
              <CardContent className="p-5">
                <SectionHeading
                  title={t('runtimeTopologyRuntimeSection')}
                  description={t('runtimeTopologyRuntimeSectionDescription')}
                />
                <div className="mt-4 grid gap-3">
                  {data.runtimes.length === 0 ? (
                    <EmptyCard text={t('runtimeTopologyNoRuntimes')} />
                  ) : (
                    data.runtimes.map((node) => (
                      <div
                        key={node.runtime.id}
                        className="rounded-3xl border border-border/70 bg-muted/35 p-4"
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <div className="text-sm font-semibold text-foreground">
                              {node.runtime.display_name || node.runtime.name}
                            </div>
                            <div className="mt-1 text-xs text-muted-foreground">
                              {node.runtime.name}
                            </div>
                          </div>
                          <StatusPill enabled={node.runtime.enabled} />
                        </div>
                        <div className="mt-3 flex flex-wrap gap-2 text-xs text-muted-foreground">
                          <span className="rounded-full bg-background/80 px-2.5 py-1">
                            {t('runtimeTopologyProvider', node.runtime.provider || '-')}
                          </span>
                          <span className="rounded-full bg-background/80 px-2.5 py-1">
                            {t('runtimeTopologyModel', node.runtime.model || '-')}
                          </span>
                          <span className="rounded-full bg-background/80 px-2.5 py-1">
                            {t('runtimeTopologyBoundAccounts', String(node.bound_account_count))}
                          </span>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </CardContent>
            </Card>

            <Card className="rounded-[28px] border-border/70 bg-card/92 shadow-sm">
              <CardContent className="p-5">
                <SectionHeading
                  title={t('runtimeTopologyAccountSection')}
                  description={t('runtimeTopologyAccountSectionDescription')}
                />
                <div className="mt-4 grid gap-3">
                  {data.accounts.length === 0 ? (
                    <EmptyCard text={t('runtimeTopologyNoAccounts')} />
                  ) : (
                    data.accounts.map((node) => (
                      <div
                        key={node.account.id}
                        className="rounded-3xl border border-border/70 bg-muted/35 p-4"
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <div className="text-sm font-semibold text-foreground">
                              {node.account.display_name || node.account.account_key}
                            </div>
                            <div className="mt-1 text-xs text-muted-foreground">
                              {node.account.channel_type} / {node.account.account_key}
                            </div>
                          </div>
                          <StatusPill enabled={node.account.enabled} />
                        </div>
                        <div className="mt-3 flex flex-wrap gap-2 text-xs text-muted-foreground">
                          <span className="rounded-full bg-background/80 px-2.5 py-1">
                            {t('runtimeTopologyMode', node.binding_mode)}
                          </span>
                          <span className="rounded-full bg-background/80 px-2.5 py-1">
                            {t('runtimeTopologyBoundRuntimes', String(node.bound_runtime_count))}
                          </span>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </CardContent>
            </Card>
          </section>

          <Card className="rounded-[28px] border-border/70 bg-card/92 shadow-sm">
            <CardContent className="p-5">
              <SectionHeading
                title={t('runtimeTopologyBindingSection')}
                description={t('runtimeTopologyBindingSectionDescription')}
              />
              <div className="mt-4 grid gap-3">
                {data.bindings.length === 0 ? (
                  <EmptyCard text={t('runtimeTopologyNoBindings')} />
                ) : (
                  data.bindings.map((edge) => (
                    <div
                      key={edge.binding.id}
                      className="rounded-3xl border border-border/70 bg-muted/35 p-4"
                    >
                      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                        <div className="min-w-0">
                          <div className="text-sm font-semibold text-foreground">
                            {edge.account_label} <span className="text-muted-foreground">→</span> {edge.runtime_name || edge.binding.agent_runtime_id}
                          </div>
                          <div className="mt-1 text-xs text-muted-foreground">
                            {edge.channel_type} · {t('runtimeTopologyMode', edge.binding.binding_mode)}
                          </div>
                        </div>
                        <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                          <span className="rounded-full bg-background/80 px-2.5 py-1">
                            {t('runtimeTopologyPriority', String(edge.binding.priority))}
                          </span>
                          <span className="rounded-full bg-background/80 px-2.5 py-1">
                            {edge.binding.allow_public_reply
                              ? t('runtimeTopologyPublicReplyEnabled')
                              : t('runtimeTopologyPublicReplyDisabled')}
                          </span>
                          {edge.binding.reply_label ? (
                            <span className="rounded-full bg-background/80 px-2.5 py-1">
                              {t('runtimeTopologyReplyLabel', edge.binding.reply_label)}
                            </span>
                          ) : null}
                        </div>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      ) : null}
    </div>
  );
}

function MetricCard({
  icon,
  label,
  value,
  accent,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  accent: string;
}) {
  return (
    <Card className="overflow-hidden rounded-[28px] border-border/70 bg-card/92 shadow-sm">
      <CardContent className="relative p-5">
        <div className={`pointer-events-none absolute inset-0 bg-gradient-to-br ${accent}`} />
        <div className="relative">
          <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
            {icon}
            <span>{label}</span>
          </div>
          <div className="mt-4 text-3xl font-semibold tracking-tight text-foreground">{value}</div>
        </div>
      </CardContent>
    </Card>
  );
}

function SectionHeading({ title, description }: { title: string; description: string }) {
  return (
    <div>
      <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{title}</div>
      <p className="mt-2 text-sm leading-6 text-muted-foreground">{description}</p>
    </div>
  );
}

function StatusPill({ enabled }: { enabled: boolean }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${
        enabled
          ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
          : 'bg-slate-100 text-slate-600 dark:bg-slate-900 dark:text-slate-300'
      }`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${enabled ? 'bg-emerald-500' : 'bg-slate-400'}`} />
      {enabled ? t('on') : t('off')}
    </span>
  );
}

function EmptyCard({ text }: { text: string }) {
  return (
    <div className="rounded-3xl border border-dashed border-border px-4 py-8 text-sm text-muted-foreground">
      {text}
    </div>
  );
}
