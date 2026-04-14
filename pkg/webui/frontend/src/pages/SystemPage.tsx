import { t } from "@/lib/i18n";
import {
  SessionRuntimeState,
  StatusTask,
  useDaemonBootstrap,
  useReloadService,
  useRestartService,
  useServiceStatus,
  useStatus,
} from "@/hooks/useConfig";
import { useInstallQMD, useQMDStatus, useUpdateQMD } from "@/hooks/useQMD";
import type { CronJob } from "@/hooks/useCron";
import Header from "@/components/layout/Header";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import {
  AlertCircle,
  Clock3,
  Copy,
  DatabaseZap,
  RefreshCw,
} from "lucide-react";
import { toast } from "sonner";

export default function SystemPage() {
  const { data: status, isLoading, refetch, isFetching } = useStatus();
  const {
    data: service,
    isLoading: serviceLoading,
    refetch: refetchService,
    isFetching: serviceFetching,
  } = useServiceStatus();
  const {
    data: daemonBootstrap,
    isLoading: daemonBootstrapLoading,
    refetch: refetchDaemonBootstrap,
    isFetching: daemonBootstrapFetching,
  } = useDaemonBootstrap();
  const {
    data: qmd,
    isLoading: qmdLoading,
    refetch: refetchQMD,
    isFetching: qmdFetching,
  } = useQMDStatus();
  const updateQMD = useUpdateQMD();
  const installQMD = useInstallQMD();
  const restartService = useRestartService();
  const reloadService = useReloadService();
  const serviceInstalled = service?.installed ?? false;
  const serviceStatus = service?.status ?? "unknown";
  const taskCounts = status?.task_state_counts ?? {};
  const recentTasks = status?.recent_tasks ?? [];
  const recentCronJobs = status?.recent_cron_jobs ?? [];
  const runtimeStates = status?.runtime_states ?? [];
  const daemonMachines = status?.daemon_machines ?? [];
  const sessionStates = status?.session_runtime_states ?? [];
  const agentDefinition = status?.agent_definition ?? null;
  const agentRoute = agentDefinition?.route ?? null;
  const agentToolPolicy = agentDefinition?.toolPolicy ?? null;
  const agentPromptSections = agentDefinition?.promptSections ?? null;

  async function copyText(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(t("copied"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("copyFailed"));
    }
  }

  return (
    <div className="system-page flex h-full flex-col">
      <Header title={t("tabStatus")} />
      <div className="flex flex-wrap items-center gap-2 pb-4">
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetch()}
          disabled={isFetching}
        >
          <RefreshCw
            className={`h-4 w-4 mr-1 ${isFetching ? "animate-spin" : ""}`}
          />
          {t("refresh")}
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetchService()}
          disabled={serviceFetching}
        >
          <RefreshCw
            className={`h-4 w-4 mr-1 ${serviceFetching ? "animate-spin" : ""}`}
          />
          {t("systemServiceButton")}
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetchQMD()}
          disabled={qmdFetching}
        >
          <DatabaseZap
            className={`h-4 w-4 mr-1 ${qmdFetching ? "animate-spin" : ""}`}
          />
          {t("systemQMDButton")}
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetchDaemonBootstrap()}
          disabled={daemonBootstrapFetching}
        >
          <RefreshCw
            className={`h-4 w-4 mr-1 ${daemonBootstrapFetching ? "animate-spin" : ""}`}
          />
          {t("systemDaemonBootstrapRefresh")}
        </Button>
      </div>

      <ScrollArea className="flex-1 pb-6">
        <div className="space-y-4">
          {!isLoading ? (
            <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                  {t("systemRuntimePathsTitle")}
                </div>
                <h3 className="mt-2 text-lg font-semibold text-foreground">
                  {t("systemRuntimePathsHeadline")}
                </h3>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  {t("systemRuntimePathsDescription")}
                </p>
              </div>
              <div className="mt-4 grid gap-3 md:grid-cols-2">
                <StatusMetric
                  label={t("systemConfigPath")}
                  value={status?.config_path || "-"}
                />
                <StatusMetric
                  label={t("systemDatabaseDir")}
                  value={status?.database_dir || "-"}
                />
                <StatusMetric
                  label={t("systemRuntimeDatabase")}
                  value={status?.runtime_db_path || "-"}
                />
                <StatusMetric
                  label={t("agentsWorkspace")}
                  value={status?.workspace_path || "-"}
                />
              </div>
            </Card>
          ) : null}

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div>
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t("systemTasksTitle")}
              </div>
              <h3 className="mt-2 text-lg font-semibold text-foreground">
                {t("systemTasksHeadline")}
              </h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t("systemTasksDescription")}
              </p>
            </div>

            {isLoading ? (
              <div className="text-muted-foreground py-8 text-center animate-pulse">
                {t("systemLoading")}
              </div>
            ) : (
              <div className="mt-4 space-y-4">
                <div className="grid gap-3 md:grid-cols-4">
                  <StatusMetric
                    label={t("systemTasksTotal")}
                    value={String(status?.task_count ?? 0)}
                  />
                  <StatusMetric
                    label={t("systemTasksRunning")}
                    value={String(taskCounts.running ?? 0)}
                  />
                  <StatusMetric
                    label={t("systemTasksPending")}
                    value={String(taskCounts.pending ?? 0)}
                  />
                  <StatusMetric
                    label={t("systemTasksFailed")}
                    value={String(taskCounts.failed ?? 0)}
                  />
                </div>

                {recentTasks.length > 0 ? (
                  <div className="space-y-3">
                    {recentTasks.map((task) => (
                      <TaskCard key={task.id} task={task} />
                    ))}
                  </div>
                ) : recentCronJobs.length > 0 ? (
                  <div className="space-y-3">
                    <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                      {t("systemRecentCronJobs")}
                    </div>
                    {recentCronJobs.map((job) => (
                      <CronJobCard key={job.id} job={job} />
                    ))}
                  </div>
                ) : (
                  <div className="rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                    {t("systemTasksEmpty")}
                  </div>
                )}
              </div>
            )}
          </Card>

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div>
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t("runtimeTopologyRuntimes")}
              </div>
              <h3 className="mt-2 text-lg font-semibold text-foreground">
                {t("runtimeTopologyRuntimeSection")}
              </h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t("runtimeTopologyRuntimeSectionDescription")}
              </p>
            </div>

            {isLoading ? (
              <div className="text-muted-foreground py-8 text-center animate-pulse">
                {t("systemLoading")}
              </div>
            ) : runtimeStates.length > 0 ? (
              <div className="mt-4 space-y-3">
                {runtimeStates.map((runtime) => (
                  <div
                    key={runtime.id}
                    className="rounded-2xl border border-border/70 bg-muted/35 p-4"
                  >
                    <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span
                            className={cn(
                              "rounded-full px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em]",
                              runtime.status?.effective_available
                                ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                                : "bg-amber-500/15 text-amber-700 dark:text-amber-300",
                            )}
                          >
                            {formatRuntimeAvailabilityLabel(
                              runtime.status?.effective_available ?? false,
                              runtime.status?.availability_reason,
                            )}
                          </span>
                          <span className="rounded-full bg-background/80 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-foreground/80">
                            {runtime.provider || "-"} / {runtime.model || "-"}
                          </span>
                        </div>
                        <div className="mt-3 text-sm font-semibold text-foreground">
                          {runtime.display_name || runtime.name}
                        </div>
                        <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                          <span>
                            {t(
                              "runtimeTopologyBoundAccounts",
                              String(runtime.status?.bound_account_count ?? 0),
                            )}
                          </span>
                          <span>
                            {t("systemTasksRunning")}:{" "}
                            {String(runtime.status?.current_task_count ?? 0)}
                          </span>
                          {runtime.status?.last_seen_at ? (
                            <span>
                              {formatTaskTimestamp(runtime.status.last_seen_at)}
                            </span>
                          ) : null}
                        </div>
                      </div>
                      <div className="break-all text-xs text-muted-foreground md:max-w-[14rem] md:text-right">
                        {runtime.id}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="mt-4 rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                {t("runtimeTopologyNoRuntimes")}
              </div>
            )}
          </Card>

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div>
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t("systemDaemonTitle")}
              </div>
              <h3 className="mt-2 text-lg font-semibold text-foreground">
                {t("systemDaemonHeadline")}
              </h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t("systemDaemonDescription")}
              </p>
            </div>

            {isLoading ? (
              <div className="text-muted-foreground py-8 text-center animate-pulse">
                {t("systemLoading")}
              </div>
            ) : daemonMachines.length > 0 ? (
              <div className="mt-4 space-y-3">
                {daemonMachines.map((machine) => (
                  <div
                    key={machine.info.machine_id}
                    className="rounded-2xl border border-border/70 bg-muted/35 p-4"
                  >
                    <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span
                            className={cn(
                              "rounded-full px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em]",
                              machine.info.status === "online"
                                ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                                : "bg-amber-500/15 text-amber-700 dark:text-amber-300",
                            )}
                          >
                            {machine.info.status || "-"}
                          </span>
                          <span className="rounded-full bg-background/80 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-foreground/80">
                            {machine.info.os}/{machine.info.arch}
                          </span>
                        </div>
                        <div className="mt-3 text-sm font-semibold text-foreground">
                          {machine.info.machine_name || machine.info.hostname}
                        </div>
                        <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                          <span>
                            {t(
                              "systemDaemonWorkspaces",
                              String(machine.workspace_count),
                            )}
                          </span>
                          <span>
                            {t(
                              "systemDaemonRuntimes",
                              String(machine.runtime_count),
                            )}
                          </span>
                          <span>
                            {t(
                              "systemDaemonInstalledRuntimes",
                              String(machine.installed_runtime_count),
                            )}
                          </span>
                        </div>
                      </div>
                      <div className="break-all text-xs text-muted-foreground md:max-w-[14rem] md:text-right">
                        {machine.info.machine_id}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="mt-4 rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                {t("systemDaemonEmpty")}
              </div>
            )}

            <div className="mt-4 rounded-2xl border border-border/70 bg-background/70 p-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                    {t("systemDaemonBootstrapTitle")}
                  </div>
                  <div className="mt-2 text-sm text-muted-foreground">
                    {t("systemDaemonBootstrapDescription")}
                  </div>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => refetchDaemonBootstrap()}
                  disabled={daemonBootstrapFetching}
                >
                  <RefreshCw
                    className={cn(
                      "mr-2 h-4 w-4",
                      daemonBootstrapFetching && "animate-spin",
                    )}
                  />
                  {t("refresh")}
                </Button>
              </div>

              {daemonBootstrapLoading ? (
                <div className="text-muted-foreground py-6 text-center animate-pulse">
                  {t("systemLoading")}
                </div>
              ) : daemonBootstrap ? (
                <div className="mt-4 space-y-3">
                  <StatusMetric
                    label={t("systemDaemonServerUrl")}
                    value={daemonBootstrap.server_url || "-"}
                  />
                  <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
                    <div className="flex items-center justify-between gap-3">
                      <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                        {t("systemDaemonCommand")}
                      </div>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => copyText(daemonBootstrap.command || "")}
                      >
                        <Copy className="mr-2 h-4 w-4" />
                        {t("copyAccess")}
                      </Button>
                    </div>
                    <div className="mt-2 break-all font-mono text-sm text-foreground">
                      {daemonBootstrap.command || "-"}
                    </div>
                  </div>
                  <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
                    <div className="flex items-center justify-between gap-3">
                      <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                        {t("systemDaemonToken")}
                      </div>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() =>
                          copyText(daemonBootstrap.daemon_token || "")
                        }
                      >
                        <Copy className="mr-2 h-4 w-4" />
                        {t("copyAccess")}
                      </Button>
                    </div>
                    <div className="mt-2 break-all font-mono text-sm text-foreground">
                      {daemonBootstrap.daemon_token || "-"}
                    </div>
                  </div>
                </div>
              ) : (
                <div className="mt-4 rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                  {t("systemDaemonBootstrapEmpty")}
                </div>
              )}
            </div>
          </Card>

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div>
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t("systemSessionRuntimeTitle")}
              </div>
              <h3 className="mt-2 text-lg font-semibold text-foreground">
                {t("systemSessionRuntimeHeadline")}
              </h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t("systemSessionRuntimeDescription")}
              </p>
            </div>

            {isLoading ? (
              <div className="text-muted-foreground py-8 text-center animate-pulse">
                {t("systemLoading")}
              </div>
            ) : sessionStates.length > 0 ? (
              <div className="mt-4 space-y-3">
                {sessionStates.map((state) => (
                  <SessionStateCard key={state.session_id} state={state} />
                ))}
              </div>
            ) : (
              <div className="mt-4 rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                {t("systemSessionRuntimeEmpty")}
              </div>
            )}
          </Card>

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div>
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t("systemAgentDefinitionTitle")}
              </div>
              <h3 className="mt-2 text-lg font-semibold text-foreground">
                {t("systemAgentDefinitionHeadline")}
              </h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t("systemAgentDefinitionDescription")}
              </p>
            </div>

            {isLoading ? (
              <div className="text-muted-foreground py-8 text-center animate-pulse">
                {t("systemLoading")}
              </div>
            ) : agentDefinition ? (
              <div className="mt-4 space-y-4">
                <div className="grid gap-3 md:grid-cols-4">
                  <StatusMetric
                    label={t("systemAgentDefinitionID")}
                    value={agentDefinition.id || "-"}
                  />
                  <StatusMetric
                    label={t("systemAgentDefinitionOrchestrator")}
                    value={agentDefinition.orchestrator || "-"}
                  />
                  <StatusMetric
                    label={t("systemAgentDefinitionPermissionMode")}
                    value={agentDefinition.permissionMode || "-"}
                  />
                  <StatusMetric
                    label={t("systemAgentDefinitionIterations")}
                    value={String(agentDefinition.maxToolIterations ?? 0)}
                  />
                </div>

                <div className="grid gap-3 md:grid-cols-2">
                  <StatusMetric
                    label={t("defaultProvider")}
                    value={agentRoute?.provider || "-"}
                  />
                  <StatusMetric
                    label={t("defaultModel")}
                    value={agentRoute?.model || "-"}
                  />
                </div>

                <div className="grid gap-3 md:grid-cols-3">
                  <StatusMetric
                    label={t("fallbackProviders")}
                    value={
                      agentRoute?.fallback?.length
                        ? agentRoute.fallback.join(", ")
                        : t("none")
                    }
                  />
                  <StatusMetric
                    label={t("systemAgentDefinitionStaticSections")}
                    value={
                      agentPromptSections?.static?.length
                        ? agentPromptSections.static.join(", ")
                        : t("none")
                    }
                  />
                  <StatusMetric
                    label={t("systemAgentDefinitionDynamicSections")}
                    value={
                      agentPromptSections?.dynamic?.length
                        ? agentPromptSections.dynamic.join(", ")
                        : t("none")
                    }
                  />
                </div>

                <div className="grid gap-3 md:grid-cols-2">
                  <StatusMetric
                    label={t("systemAgentDefinitionAllowlist")}
                    value={
                      agentToolPolicy?.allowlist?.length
                        ? agentToolPolicy.allowlist.join(", ")
                        : t("none")
                    }
                  />
                  <StatusMetric
                    label={t("systemAgentDefinitionDenylist")}
                    value={
                      agentToolPolicy?.denylist?.length
                        ? agentToolPolicy.denylist.join(", ")
                        : t("none")
                    }
                  />
                </div>
              </div>
            ) : (
              <div className="mt-4 rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                {t("systemAgentDefinitionEmpty")}
              </div>
            )}
          </Card>

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                  {t("systemServiceTitle")}
                </div>
                <h3 className="mt-2 text-lg font-semibold text-foreground">
                  {t("systemServiceHeadline")}
                </h3>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  {t("systemServiceDescription")}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => reloadService.mutate()}
                  disabled={reloadService.isPending}
                >
                  <RefreshCw
                    className={cn(
                      "mr-2 h-4 w-4",
                      reloadService.isPending && "animate-spin",
                    )}
                  />
                  {reloadService.isPending
                    ? t("systemServiceReloading")
                    : t("systemServiceReload")}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => restartService.mutate()}
                  disabled={restartService.isPending || !serviceInstalled}
                >
                  <RefreshCw
                    className={cn(
                      "mr-2 h-4 w-4",
                      restartService.isPending && "animate-spin",
                    )}
                  />
                  {restartService.isPending
                    ? t("systemServiceRestarting")
                    : t("systemServiceRestart")}
                </Button>
              </div>
            </div>

            {serviceLoading ? (
              <div className="text-muted-foreground py-8 text-center animate-pulse">
                {t("systemLoading")}
              </div>
            ) : (
              <div className="mt-4 space-y-4">
                <div className="grid gap-3 md:grid-cols-4">
                  <StatusMetric
                    label={t("systemServiceInstalled")}
                    value={serviceInstalled ? t("systemYes") : t("systemNo")}
                  />
                  <StatusMetric
                    label={t("systemServiceStatus")}
                    value={formatServiceStatus(serviceStatus)}
                  />
                  <StatusMetric
                    label={t("systemServicePlatform")}
                    value={service?.platform || "-"}
                  />
                  <StatusMetric
                    label={t("systemServiceName")}
                    value={service?.name || "-"}
                  />
                </div>

                <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
                  <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                    {t("systemServiceConfigPath")}
                  </div>
                  <div className="mt-2 break-all font-mono text-sm text-foreground">
                    {service?.config_path || "-"}
                  </div>
                  <div className="mt-3">
                    <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                      {t("systemServiceArguments")}
                    </div>
                    <div className="mt-2 break-all font-mono text-sm text-foreground">
                      {service?.arguments && service.arguments.length > 0
                        ? service.arguments.join(" ")
                        : "-"}
                    </div>
                  </div>
                  {!serviceInstalled ? (
                    <div className="mt-3 rounded-xl border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
                      {t("systemServiceNotInstalledHint")}
                    </div>
                  ) : null}
                  {serviceInstalled ? (
                    <div className="mt-3 rounded-xl border border-border/70 bg-background/70 px-3 py-2 text-sm text-foreground/80">
                      {t("systemServiceReloadHint")}
                    </div>
                  ) : null}
                  {serviceInstalled && serviceStatus !== "running" ? (
                    <div className="mt-3 rounded-xl border border-border/70 bg-background/70 px-3 py-2 text-sm text-foreground/80">
                      {t("systemServiceRestartHint")}
                    </div>
                  ) : null}
                </div>
              </div>
            )}
          </Card>

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                  {t("systemQMDTitle")}
                </div>
                <h3 className="mt-2 text-lg font-semibold text-foreground">
                  {t("systemQMDHeadline")}
                </h3>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  {t("systemQMDDescription")}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => installQMD.mutate()}
                  disabled={installQMD.isPending}
                >
                  <RefreshCw
                    className={cn(
                      "mr-2 h-4 w-4",
                      installQMD.isPending && "animate-spin",
                    )}
                  />
                  {installQMD.isPending
                    ? t("marketplaceInstalling")
                    : t("systemQMDInstall")}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => updateQMD.mutate()}
                  disabled={updateQMD.isPending || !qmd?.available}
                >
                  <RefreshCw
                    className={cn(
                      "mr-2 h-4 w-4",
                      updateQMD.isPending && "animate-spin",
                    )}
                  />
                  {updateQMD.isPending
                    ? t("systemUpdating")
                    : t("systemQMDUpdate")}
                </Button>
              </div>
            </div>

            {qmdLoading ? (
              <div className="text-muted-foreground py-8 text-center animate-pulse">
                {t("systemLoading")}
              </div>
            ) : (
              <div className="mt-4 space-y-4">
                <div className="grid gap-3 md:grid-cols-4">
                  <StatusMetric
                    label={t("enabled")}
                    value={qmd?.enabled ? t("systemYes") : t("systemNo")}
                  />
                  <StatusMetric
                    label={t("systemAvailable")}
                    value={qmd?.available ? t("systemYes") : t("systemNo")}
                  />
                  <StatusMetric
                    label={t("marketplaceVersion")}
                    value={qmd?.version || "-"}
                  />
                  <StatusMetric
                    label={t("systemCollections")}
                    value={String(qmd?.collections.length ?? 0)}
                  />
                </div>

                <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
                  <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                    {t("systemCommand")}
                  </div>
                  <div className="mt-2 font-mono text-sm text-foreground">
                    {qmd?.command || "-"}
                  </div>
                  <div className="mt-3 grid gap-3 md:grid-cols-3">
                    <StatusMetric
                      label={t("systemResolvedCommand")}
                      value={qmd?.resolved_command || "-"}
                    />
                    <StatusMetric
                      label={t("systemCommandSource")}
                      value={qmd?.command_source || "-"}
                    />
                    <StatusMetric
                      label={t("systemPersistentCommand")}
                      value={qmd?.persistent_command || "-"}
                    />
                  </div>
                  {qmd?.error ? (
                    <div className="mt-3 rounded-xl border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
                      {qmd.error}
                    </div>
                  ) : null}
                </div>

                <div className="space-y-3">
                  {(qmd?.collections ?? []).map((collection) => (
                    <div
                      key={`${collection.Name}-${collection.Path}`}
                      className="rounded-2xl border border-border/70 bg-muted/35 p-4"
                    >
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
                      {t("systemQMDNoCollections")}
                    </div>
                  ) : null}
                </div>
              </div>
            )}
          </Card>

          {isLoading ? (
            <div className="text-muted-foreground py-8 text-center animate-pulse">
              {t("systemLoading")}
            </div>
          ) : (
            <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                  {t("systemRawStatusTitle")}
                </div>
                <h3 className="mt-2 text-lg font-semibold text-foreground">
                  {t("systemRawStatusHeadline")}
                </h3>
              </div>
              <pre className="mt-4 overflow-auto whitespace-pre-wrap break-words rounded-lg border border-border bg-card p-4 text-sm font-mono">
                {JSON.stringify(status, null, 2)}
              </pre>
            </Card>
          )}
        </div>
      </ScrollArea>
    </div>
  );
}

function formatServiceStatus(status: string) {
  switch (status) {
    case "running":
      return t("systemServiceStatusRunning");
    case "stopped":
      return t("systemServiceStatusStopped");
    case "not_installed":
      return t("systemServiceStatusNotInstalled");
    default:
      return t("systemServiceStatusUnknown");
  }
}

function StatusMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
      <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
        {label}
      </div>
      <div className="mt-2 break-all text-sm font-semibold text-foreground">
        {value}
      </div>
    </div>
  );
}

function SessionStateCard({ state }: { state: SessionRuntimeState }) {
  return (
    <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            {state.permission_mode ? (
              <span className="rounded-full bg-sky-500/15 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-sky-700 dark:text-sky-300">
                {t("systemSessionPermissionMode", state.permission_mode)}
              </span>
            ) : null}
            {state.pending_action ? (
              <span className="rounded-full bg-amber-500/15 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-amber-700 dark:text-amber-300">
                {t("systemSessionPendingAction", state.pending_action)}
              </span>
            ) : null}
          </div>
          <div className="mt-3 text-sm font-semibold text-foreground">
            {state.session_id}
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1">
              <Clock3 className="h-3.5 w-3.5" />
              {t(
                "systemSessionUpdatedAt",
                formatTaskTimestamp(state.updated_at),
              )}
            </span>
            {state.pending_request_id ? (
              <span>
                {t("systemSessionPendingRequest", state.pending_request_id)}
              </span>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}

function TaskCard({ task }: { task: StatusTask }) {
  const stateLabel = formatTaskState(task.state);
  const detailTime = task.completed_at || task.started_at || task.created_at;
  const metadata = task.metadata ?? {};
  const label = typeof metadata.label === "string" ? metadata.label : "";
  const channel = typeof metadata.channel === "string" ? metadata.channel : "";

  return (
    <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-full bg-background/80 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-foreground/80">
              {task.type}
            </span>
            <span
              className={cn(
                "rounded-full px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em]",
                taskStateClassName(task.state),
              )}
            >
              {stateLabel}
            </span>
            {label ? (
              <span className="rounded-full bg-primary px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-primary-foreground">
                {label}
              </span>
            ) : null}
          </div>
          <div className="mt-3 text-sm font-semibold text-foreground">
            {task.summary || task.id}
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1">
              <Clock3 className="h-3.5 w-3.5" />
              {formatTaskTimestamp(detailTime)}
            </span>
            {task.session_id ? (
              <span>{t("systemTasksSession", task.session_id)}</span>
            ) : null}
            {channel ? <span>{t("systemTasksChannel", channel)}</span> : null}
          </div>
          {task.last_error ? (
            <div className="mt-3 rounded-xl border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
              <div className="flex items-start gap-2">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{task.last_error}</span>
              </div>
            </div>
          ) : null}
        </div>
        <div className="break-all text-xs text-muted-foreground md:max-w-[14rem] md:text-right">
          {task.id}
        </div>
      </div>
    </div>
  );
}

function CronJobCard({ job }: { job: CronJob }) {
  const state = job.last_success ? "completed" : "failed";
  const detailTime = job.last_run || job.next_run || job.created_at;
  return (
    <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-full bg-background/80 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-foreground/80">
              cron
            </span>
            <span
              className={cn(
                "rounded-full px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em]",
                taskStateClassName(state),
              )}
            >
              {formatTaskState(state)}
            </span>
            {job.enabled ? (
              <span className="rounded-full bg-emerald-500/15 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-emerald-700 dark:text-emerald-300">
                {t("enabled")}
              </span>
            ) : null}
          </div>
          <div className="mt-3 text-sm font-semibold text-foreground">
            {job.name || job.id}
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1">
              <Clock3 className="h-3.5 w-3.5" />
              {formatTaskTimestamp(detailTime)}
            </span>
            <span>
              {t("cronRunCount")}: {String(job.run_count ?? 0)}
            </span>
          </div>
          {job.last_error ? (
            <div className="mt-3 rounded-xl border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
              <div className="flex items-start gap-2">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{job.last_error}</span>
              </div>
            </div>
          ) : null}
        </div>
        <div className="break-all text-xs text-muted-foreground md:max-w-[14rem] md:text-right">
          {job.id}
        </div>
      </div>
    </div>
  );
}

function formatTaskState(state: string) {
  switch (state) {
    case "running":
      return t("systemTasksStateRunning");
    case "pending":
      return t("systemTasksStatePending");
    case "completed":
      return t("systemTasksStateCompleted");
    case "failed":
      return t("systemTasksStateFailed");
    case "requires_action":
      return t("systemTasksStateRequiresAction");
    default:
      return state || t("none");
  }
}

function taskStateClassName(state: string) {
  switch (state) {
    case "running":
      return "bg-sky-500/15 text-sky-700 dark:text-sky-300";
    case "pending":
      return "bg-amber-500/15 text-amber-700 dark:text-amber-300";
    case "completed":
      return "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300";
    case "failed":
      return "bg-rose-500/15 text-rose-700 dark:text-rose-300";
    case "requires_action":
      return "bg-violet-500/15 text-violet-700 dark:text-violet-300";
    default:
      return "bg-background/80 text-foreground/80";
  }
}

function formatTaskTimestamp(value?: string) {
  if (!value) {
    return "-";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  if (parsed.getUTCFullYear() <= 1) {
    return "-";
  }
  return parsed.toLocaleString();
}

function formatRuntimeAvailabilityLabel(
  effectiveAvailable: boolean,
  reason?: string,
): string {
  if (effectiveAvailable) {
    return t("systemAvailable");
  }
  if (!reason) {
    return t("none");
  }
  return t(`runtimeTopologyAvailabilityReason_${reason}`);
}
