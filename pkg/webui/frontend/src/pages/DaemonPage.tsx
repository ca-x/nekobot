import { useEffect, useMemo, useState } from "react";
import Header from "@/components/layout/Header";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  useDaemonBootstrap,
  useDaemonExplorerWorkspaces,
  useDaemonInventory,
  useDaemonWorkspaceFile,
  useDaemonWorkspaceTree,
  useNekoClientdBootstrap,
  useStatus,
  type DaemonMachineInventory,
  type DaemonRuntimeDetail,
} from "@/hooks/useConfig";
import { t } from "@/lib/i18n";
import { toast } from "@/lib/notify";
import { cn } from "@/lib/utils";
import {
  Bot,
  ChevronDown,
  ChevronRight,
  Copy,
  FileText,
  Folder,
  RefreshCw,
  Server,
  Zap,
} from "lucide-react";

export default function DaemonPage() {
  const { data: status, isLoading: statusLoading, refetch: refetchStatus, isFetching: statusFetching } = useStatus();
  const { data: inventory, isLoading: inventoryLoading, refetch: refetchInventory, isFetching: inventoryFetching } = useDaemonInventory();
  const {
    data: daemonBootstrap,
    isLoading: daemonBootstrapLoading,
    refetch: refetchDaemonBootstrap,
    isFetching: daemonBootstrapFetching,
  } = useDaemonBootstrap();
  const [nekoclientdTargetOS, setNekoclientdTargetOS] = useState("linux");
  const [nekoclientdTargetArch, setNekoclientdTargetArch] = useState("amd64");
  const { data: nekoclientdBootstrap } = useNekoClientdBootstrap(
    nekoclientdTargetOS,
    nekoclientdTargetArch,
  );
  const [selectedDaemonMachine, setSelectedDaemonMachine] = useState("");
  const [selectedDaemonWorkspace, setSelectedDaemonWorkspace] = useState("");
  const [daemonPath, setDaemonPath] = useState("");
  const [selectedPreviewPath, setSelectedPreviewPath] = useState("");
  const [expandedMachines, setExpandedMachines] = useState<Set<string>>(new Set());

  const daemonMachines = status?.daemon_machines ?? [];
  const inventoryMachines = inventory?.machines ?? [];
  const daemonMachinesWithURL = daemonMachines.filter((machine) => machine.info.daemon_url);
  const selectedMachine = useMemo(
    () =>
      daemonMachinesWithURL.find((machine) => machine.info.computer_id === selectedDaemonMachine) ??
      daemonMachinesWithURL[0] ??
      null,
    [daemonMachinesWithURL, selectedDaemonMachine],
  );
  const daemonWorkspaces = useDaemonExplorerWorkspaces(selectedMachine?.info.computer_id ?? null);
  const selectedWorkspace = useMemo(() => {
    const items = daemonWorkspaces.data?.workspaces ?? [];
    return (
      items.find((workspace) => workspace.workspace_id === selectedDaemonWorkspace) ??
      items.find((workspace) => workspace.is_default) ??
      items[0] ??
      null
    );
  }, [daemonWorkspaces.data?.workspaces, selectedDaemonWorkspace]);
  const selectedWorkspaceId = selectedWorkspace?.workspace_id ?? null;
  const daemonTree = useDaemonWorkspaceTree(
    selectedMachine?.info.computer_id ?? null,
    selectedWorkspaceId,
    daemonPath,
  );
  const {
    mutate: loadDaemonFile,
    data: daemonFileData,
    error: daemonFileError,
    isPending: daemonFilePending,
    reset: resetDaemonFile,
  } = useDaemonWorkspaceFile();

  useEffect(() => {
    if (!selectedMachine) {
      setSelectedDaemonMachine("");
      return;
    }
    if (
      !selectedDaemonMachine ||
      !daemonMachinesWithURL.some((machine) => machine.info.computer_id === selectedDaemonMachine)
    ) {
      setSelectedDaemonMachine(selectedMachine.info.computer_id);
    }
  }, [daemonMachinesWithURL, selectedDaemonMachine, selectedMachine]);

  useEffect(() => {
    if (!selectedWorkspace) {
      setSelectedDaemonWorkspace("");
      return;
    }
    const items = daemonWorkspaces.data?.workspaces ?? [];
    if (
      !selectedDaemonWorkspace ||
      !items.some((workspace) => workspace.workspace_id === selectedDaemonWorkspace)
    ) {
      setSelectedDaemonWorkspace(selectedWorkspace.workspace_id);
    }
  }, [daemonWorkspaces.data?.workspaces, selectedDaemonWorkspace, selectedWorkspace]);

  useEffect(() => {
    setSelectedPreviewPath("");
    resetDaemonFile();
  }, [selectedMachine?.info.computer_id, selectedWorkspaceId, resetDaemonFile]);

  function toggleMachine(machineId: string) {
    setExpandedMachines((prev) => {
      const next = new Set(prev);
      if (next.has(machineId)) {
        next.delete(machineId);
      } else {
        next.add(machineId);
      }
      return next;
    });
  }

  function handleRefresh() {
    refetchStatus();
    refetchInventory();
  }

  async function copyText(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(t("copied"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("copyFailed"));
    }
  }

  const isLoading = statusLoading || inventoryLoading;
  const isFetching = statusFetching || inventoryFetching;

  return (
    <div className="daemon-page flex h-full flex-col">
      <div className="mb-5 flex flex-col gap-3 sm:mb-6 lg:flex-row lg:items-start lg:justify-between">
        <Header
          title={t("tabDaemon")}
          description={t("daemonPageDescription")}
          className="mb-0"
        />
        <div className="lg:pt-4">
          <Button
            variant="outline"
            onClick={handleRefresh}
            disabled={isFetching}
          >
            <RefreshCw className={cn("mr-2 h-4 w-4", isFetching && "animate-spin")} />
            {t("refresh")}
          </Button>
        </div>
      </div>

      <ScrollArea className="flex-1">
        <div className="space-y-6 p-6">
          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                  {t("systemDaemonTitle")}
                </div>
                <h3 className="mt-2 text-lg font-semibold text-foreground">
                  {t("systemDaemonHeadline")}
                </h3>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">
                  {t("systemDaemonDescription")}
                </p>
              </div>
              <div className="grid grid-cols-3 gap-2 text-center">
                <DaemonStat label={t("daemonComputers")} value={String(inventoryMachines.length)} />
                <DaemonStat
                  label={t("daemonAgents")}
                  value={String(inventoryMachines.reduce((sum, m) => sum + m.runtime_count, 0))}
                />
                <DaemonStat
                  label={t("daemonOnline")}
                  value={String(inventoryMachines.filter((m) => m.status === "online").length)}
                />
              </div>
            </div>

            {isLoading ? (
              <div className="animate-pulse py-8 text-center text-muted-foreground">
                {t("systemLoading")}
              </div>
            ) : inventoryMachines.length > 0 ? (
              <div className="mt-4 space-y-3">
                {inventoryMachines.map((machine) => (
                  <MachineInventoryCard
                    key={machine.info.computer_id}
                    machine={machine}
                    expanded={expandedMachines.has(machine.info.computer_id)}
                    onToggle={() => toggleMachine(machine.info.computer_id)}
                  />
                ))}
              </div>
            ) : (
              <div className="mt-4 rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                {t("systemDaemonEmpty")}
              </div>
            )}
          </Card>

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div>
              <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                {t("systemDaemonExplorerTitle")}
              </div>
              <h3 className="mt-2 text-lg font-semibold text-foreground">
                {t("daemonExplorerHeadline")}
              </h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {t("systemDaemonExplorerDescription")}
              </p>
            </div>

            {daemonMachinesWithURL.length === 0 ? (
              <div className="mt-4 rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                {t("systemDaemonExplorerNoMachines")}
              </div>
            ) : (
              <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(15rem,20rem)_minmax(0,1fr)]">
                <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
                  <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                    {t("systemDaemonExplorerSelectMachine")}
                  </div>
                  <div className="mt-3 space-y-2">
                    {daemonMachinesWithURL.map((machine) => (
                      <button
                        key={machine.info.computer_id}
                        type="button"
                        onClick={() => {
                          setSelectedDaemonMachine(machine.info.computer_id);
                          setSelectedDaemonWorkspace("");
                          setDaemonPath("");
                          setSelectedPreviewPath("");
                        }}
                        className={cn(
                          "w-full rounded-xl border px-3 py-2 text-left text-sm transition",
                          selectedMachine?.info.computer_id === machine.info.computer_id
                            ? "border-primary/60 bg-primary/10 text-foreground"
                            : "border-border/70 bg-background/80 text-muted-foreground hover:border-primary/30 hover:text-foreground",
                        )}
                      >
                        <div className="font-medium text-foreground">
                          {machine.info.display_name || machine.info.hostname}
                        </div>
                        <div className="mt-1 break-all text-xs">{machine.info.computer_id}</div>
                      </button>
                    ))}
                  </div>
                </div>

                <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
                  <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
                    <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                      {t("systemDaemonExplorerSelectWorkspace")}
                    </div>
                    <div className="mt-3 space-y-2">
                      {daemonWorkspaces.isLoading ? (
                        <div className="animate-pulse text-sm text-muted-foreground">{t("systemLoading")}</div>
                      ) : daemonWorkspaces.error ? (
                        <div className="text-sm text-destructive">{t("systemDaemonExplorerLoadFailed")}</div>
                      ) : daemonWorkspaces.data?.workspaces?.length ? (
                        daemonWorkspaces.data.workspaces.map((workspace) => (
                          <button
                            key={workspace.workspace_id}
                            type="button"
                            onClick={() => {
                              setSelectedDaemonWorkspace(workspace.workspace_id);
                              setDaemonPath("");
                            }}
                            className={cn(
                              "w-full rounded-xl border px-3 py-2 text-left text-sm transition",
                              selectedWorkspace?.workspace_id === workspace.workspace_id
                                ? "border-primary/60 bg-primary/10 text-foreground"
                                : "border-border/70 bg-background/80 text-muted-foreground hover:border-primary/30 hover:text-foreground",
                            )}
                          >
                            <div className="font-medium text-foreground">
                              {workspace.display_name || workspace.workspace_id}
                            </div>
                            <div className="mt-1 break-all text-xs">{workspace.path}</div>
                          </button>
                        ))
                      ) : (
                        <div className="text-sm text-muted-foreground">{t("systemDaemonExplorerNoWorkspaces")}</div>
                      )}
                    </div>

                    <div className="mt-4 flex items-center justify-between gap-3">
                      <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                        {t("systemDaemonExplorerPath")}
                      </div>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          const current = daemonPath.trim();
                          if (!current) return;
                          const parts = current.split("/").filter(Boolean);
                          parts.pop();
                          setDaemonPath(parts.join("/"));
                        }}
                        disabled={!daemonPath.trim()}
                      >
                        {t("systemDaemonExplorerBack")}
                      </Button>
                    </div>
                    <div className="mt-2 break-all font-mono text-xs text-muted-foreground">
                      {daemonPath || "/"}
                    </div>
                    <div className="mt-4 space-y-2">
                      {daemonTree.isLoading ? (
                        <div className="animate-pulse text-sm text-muted-foreground">{t("systemLoading")}</div>
                      ) : daemonTree.error ? (
                        <div className="text-sm text-destructive">{t("systemDaemonExplorerLoadFailed")}</div>
                      ) : daemonTree.data?.entries?.length ? (
                        daemonTree.data.entries.map((entry) => (
                          <button
                            key={entry.path}
                            type="button"
                            onClick={() => {
                              if (entry.is_dir) {
                                setDaemonPath(entry.path);
                                return;
                              }
                              setSelectedPreviewPath(entry.path);
                              if (selectedMachine && selectedWorkspaceId) {
                                loadDaemonFile({
                                  computer_id: selectedMachine.info.computer_id,
                                  workspace_id: selectedWorkspaceId,
                                  path: entry.path,
                                });
                              }
                            }}
                            className={cn(
                              "flex w-full items-center justify-between rounded-xl border px-3 py-2 text-left text-sm transition",
                              selectedPreviewPath === entry.path
                                ? "border-primary/60 bg-primary/10 text-foreground"
                                : "border-border/70 bg-background/80 text-muted-foreground hover:border-primary/30 hover:text-foreground",
                            )}
                          >
                            <div className="flex min-w-0 items-center gap-2">
                              {entry.is_dir ? <Folder className="h-4 w-4 shrink-0" /> : <FileText className="h-4 w-4 shrink-0" />}
                              <div className="min-w-0">
                                <div className="truncate font-medium text-foreground">{entry.name}</div>
                                <div className="mt-1 truncate text-xs">{entry.path}</div>
                              </div>
                            </div>
                            {!entry.is_dir ? <span className="ml-3 text-xs">{t("systemDaemonExplorerOpen")}</span> : null}
                          </button>
                        ))
                      ) : (
                        <div className="text-sm text-muted-foreground">{t("systemDaemonExplorerEmpty")}</div>
                      )}
                    </div>
                  </div>

                  <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
                    <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                      {t("systemDaemonExplorerPreview")}
                    </div>
                    <div className="mt-2 break-all font-mono text-xs text-muted-foreground">
                      {selectedPreviewPath || t("systemDaemonExplorerPreviewEmpty")}
                    </div>
                    <div className="mt-4 rounded-xl border border-border/70 bg-background/80 p-3">
                      {daemonFilePending ? (
                        <div className="animate-pulse text-sm text-muted-foreground">{t("systemLoading")}</div>
                      ) : daemonFileError ? (
                        <div className="text-sm text-destructive">{daemonFileError.message}</div>
                      ) : daemonFileData ? (
                        <div className="space-y-3">
                          {daemonFileData.truncated ? (
                            <div className="text-xs text-amber-600 dark:text-amber-300">
                              {t("systemDaemonExplorerTruncated")}
                            </div>
                          ) : null}
                          <pre className="max-h-80 overflow-auto whitespace-pre-wrap break-words font-mono text-xs text-foreground">
                            {daemonFileData.content || ""}
                          </pre>
                        </div>
                      ) : (
                        <div className="text-sm text-muted-foreground">{t("systemDaemonExplorerPreviewEmpty")}</div>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            )}
          </Card>

          <Card className="rounded-[24px] border-border/70 bg-card/92 p-5 shadow-sm">
            <div className="flex flex-col items-start justify-between gap-3 sm:flex-row">
              <div>
                <div className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
                  {t("systemDaemonBootstrapTitle")}
                </div>
                <h3 className="mt-2 text-lg font-semibold text-foreground">
                  {t("daemonBootstrapHeadline")}
                </h3>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  {t("systemDaemonBootstrapDescription")}
                </p>
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={() => refetchDaemonBootstrap()}
                disabled={daemonBootstrapFetching}
                className="w-full sm:w-auto"
              >
                <RefreshCw className={cn("mr-2 h-4 w-4", daemonBootstrapFetching && "animate-spin")} />
                {t("refresh")}
              </Button>
            </div>

            {daemonBootstrapLoading ? (
              <div className="animate-pulse py-6 text-center text-muted-foreground">
                {t("systemLoading")}
              </div>
            ) : daemonBootstrap ? (
              <div className="mt-4 space-y-3">
                <StatusMetric label={t("systemDaemonServerUrl")} value={daemonBootstrap.server_url || "-"} />
                <CopyBlock
                  label={t("systemDaemonCommand")}
                  value={daemonBootstrap.command || "-"}
                  onCopy={() => copyText(daemonBootstrap.command || "")}
                />
                <CopyBlock
                  label={t("systemDaemonToken")}
                  value={daemonBootstrap.daemon_token || "-"}
                  onCopy={() => copyText(daemonBootstrap.daemon_token || "")}
                />
                {nekoclientdBootstrap ? (
                  <div className="mt-3 space-y-3 rounded-2xl border border-border/70 bg-muted/35 p-4">
                    <div className="grid gap-3 md:grid-cols-2">
                      <label className="text-sm text-muted-foreground">
                        OS
                        <select
                          className="mt-1 w-full rounded-md border border-border bg-background px-3 py-2 text-foreground"
                          value={nekoclientdTargetOS}
                          onChange={(event) => setNekoclientdTargetOS(event.target.value)}
                        >
                          <option value="linux">linux</option>
                          <option value="darwin">darwin</option>
                          <option value="windows">windows</option>
                        </select>
                      </label>
                      <label className="text-sm text-muted-foreground">
                        Arch
                        <select
                          className="mt-1 w-full rounded-md border border-border bg-background px-3 py-2 text-foreground"
                          value={nekoclientdTargetArch}
                          onChange={(event) => setNekoclientdTargetArch(event.target.value)}
                        >
                          <option value="amd64">amd64</option>
                          <option value="arm64">arm64</option>
                        </select>
                      </label>
                    </div>
                    <StatusMetric label={t("systemNekoClientdDownloadUrl")} value={nekoclientdBootstrap.download_url || "-"} />
                    <StatusMetric label={t("systemNekoClientdArchiveName")} value={nekoclientdBootstrap.archive_name || "-"} />
                    <CopyBlock
                      label={t("systemNekoClientdInstallCommand")}
                      value={nekoclientdBootstrap.install_command || "-"}
                      onCopy={() => copyText(nekoclientdBootstrap.install_command || "")}
                    />
                    <CopyBlock
                      label={t("systemNekoClientdServiceInstallCommand")}
                      value={nekoclientdBootstrap.service_install_command || "-"}
                      onCopy={() => copyText(nekoclientdBootstrap.service_install_command || "")}
                    />
                  </div>
                ) : null}
              </div>
            ) : (
              <div className="mt-4 rounded-2xl border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                {t("systemDaemonBootstrapEmpty")}
              </div>
            )}
          </Card>
        </div>
      </ScrollArea>
    </div>
  );
}

function MachineInventoryCard({
  machine,
  expanded,
  onToggle,
}: {
  machine: DaemonMachineInventory;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <div className="rounded-2xl border border-border/70 bg-muted/35 overflow-hidden">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center gap-3 p-4 text-left transition hover:bg-muted/50"
      >
        <div className="shrink-0 text-muted-foreground">
          {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span
              className={cn(
                "rounded-full px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em]",
                machine.status === "online"
                  ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
                  : "bg-amber-500/15 text-amber-700 dark:text-amber-300",
              )}
            >
              {machine.status || "-"}
            </span>
            <span className="rounded-full bg-background/80 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.12em] text-foreground/80">
              {machine.info.os}/{machine.info.arch}
            </span>
            {machine.goal_run_runnable && (
              <span className="rounded-full bg-emerald-500/10 px-2.5 py-1 text-[11px] font-medium text-emerald-700 dark:text-emerald-300">
                {t("daemonGoalRunnable")}
              </span>
            )}
          </div>
          <div className="mt-2 flex items-center gap-2 text-sm font-semibold text-foreground">
            <Server className="h-4 w-4 text-primary" />
            {machine.info.display_name || machine.info.hostname}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <span>{t("systemDaemonRuntimes", String(machine.runtime_count))}</span>
            <span>{t("systemDaemonInstalledRuntimes", String(machine.installed_runtime_count))}</span>
            <span>{t("daemonHealthyRuntimes", String(machine.healthy_runtime_count))}</span>
          </div>
        </div>
        <div className="hidden shrink-0 break-all text-xs text-muted-foreground md:block md:max-w-[14rem] md:text-right">
          {machine.info.computer_id}
        </div>
      </button>

      {expanded && machine.runtimes.length > 0 && (
        <div className="border-t border-border/50 px-4 pb-4 pt-2">
          <div className="space-y-2">
            {machine.runtimes.map((rt) => (
              <RuntimeRow key={rt.runtime_id} runtime={rt} />
            ))}
          </div>
        </div>
      )}

      {expanded && machine.runtimes.length === 0 && (
        <div className="border-t border-border/50 px-4 py-3 text-xs text-muted-foreground">
          {t("daemonNoRuntimes")}
        </div>
      )}
    </div>
  );
}

function RuntimeRow({ runtime }: { runtime: DaemonRuntimeDetail }) {
  return (
    <div className="flex items-start gap-3 rounded-xl border border-border/40 bg-background/60 p-3">
      <div className="mt-0.5 shrink-0">
        <Zap
          className={cn(
            "h-4 w-4",
            runtime.installed && runtime.enabled
              ? "text-emerald-500"
              : runtime.installed
                ? "text-amber-500"
                : "text-muted-foreground",
          )}
        />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-medium text-foreground">
            {runtime.display_name || runtime.runtime_id}
          </span>
          <RuntimeStatusBadge installed={runtime.installed} healthy={runtime.healthy} enabled={runtime.enabled} />
          {runtime.kind && (
            <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
              {runtime.kind}
            </span>
          )}
        </div>
        {(runtime.provider || runtime.model) && (
          <div className="mt-1 text-xs text-muted-foreground">
            {runtime.provider}{runtime.provider && runtime.model ? " / " : ""}{runtime.model}
          </div>
        )}
        <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
          {runtime.current_task_count > 0 && (
            <span>{t("daemonTasksRunning", String(runtime.current_task_count))}</span>
          )}
          {runtime.pending_task_count > 0 && (
            <span>{t("daemonTasksPending", String(runtime.pending_task_count))}</span>
          )}
          {runtime.skill_names && runtime.skill_names.length > 0 && (
            <span>{t("daemonSkills", String(runtime.skill_names.length))}</span>
          )}
          {runtime.env_count > 0 && (
            <span>{t("daemonEnvVars", String(runtime.env_count))}</span>
          )}
        </div>
      </div>
    </div>
  );
}

function RuntimeStatusBadge({
  installed,
  healthy,
  enabled,
}: {
  installed: boolean;
  healthy: boolean;
  enabled: boolean;
}) {
  if (!installed) {
    return (
      <span className="rounded-full bg-red-500/10 px-2 py-0.5 text-[10px] font-medium text-red-600 dark:text-red-400">
        {t("daemonNotInstalled")}
      </span>
    );
  }
  if (!enabled) {
    return (
      <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
        {t("daemonDisabled")}
      </span>
    );
  }
  if (!healthy) {
    return (
      <span className="rounded-full bg-amber-500/10 px-2 py-0.5 text-[10px] font-medium text-amber-600 dark:text-amber-400">
        {t("daemonUnhealthy")}
      </span>
    );
  }
  return (
    <span className="rounded-full bg-emerald-500/10 px-2 py-0.5 text-[10px] font-medium text-emerald-600 dark:text-emerald-400">
      {t("daemonHealthy")}
    </span>
  );
}

function DaemonStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-[5.5rem] rounded-2xl border border-border/70 bg-muted/35 px-3 py-2">
      <div className="text-lg font-semibold text-foreground">{value}</div>
      <div className="mt-1 text-[11px] text-muted-foreground">{label}</div>
    </div>
  );
}

function StatusMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
      <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
        {label}
      </div>
      <div className="mt-2 break-words text-sm font-semibold leading-6 text-foreground">
        {value}
      </div>
    </div>
  );
}

function CopyBlock({
  label,
  value,
  onCopy,
}: {
  label: string;
  value: string;
  onCopy: () => void;
}) {
  return (
    <div className="rounded-2xl border border-border/70 bg-muted/35 p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
          {label}
        </div>
        <Button size="sm" variant="outline" onClick={onCopy} className="w-full sm:w-auto">
          <Copy className="mr-2 h-4 w-4" />
          {t("copyAccess")}
        </Button>
      </div>
      <div className="mt-2 whitespace-pre-wrap break-words font-mono text-sm leading-6 text-foreground">
        {value}
      </div>
    </div>
  );
}
