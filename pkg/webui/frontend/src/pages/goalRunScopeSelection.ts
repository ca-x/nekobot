export type GoalRunScopeKind = 'server' | 'daemon';

export interface GoalRunScopeLike {
  kind: GoalRunScopeKind;
  machine_id?: string;
}

export interface GoalRunLike {
  selected_scope?: GoalRunScopeLike | null;
  recommended_scope?: GoalRunScopeLike | null;
}

export interface GoalRunDaemonOption {
  value: string;
  label: string;
}

export function preferredScope(run?: GoalRunLike | null): GoalRunScopeLike | undefined {
  return run?.selected_scope ?? run?.recommended_scope ?? undefined;
}

export function initialScopeKind(run?: GoalRunLike | null): GoalRunScopeKind {
  return preferredScope(run)?.kind ?? 'server';
}

export function initialMachineID(
  run: GoalRunLike | null | undefined,
  daemonOptions: GoalRunDaemonOption[],
): string {
  const scope = preferredScope(run);
  if (scope?.kind === 'daemon') {
    return scope.machine_id ?? '';
  }
  return daemonOptions[0]?.value ?? '';
}

export function autofillDaemonMachineID(input: {
  currentMachineID: string;
  selectedScopeKind: GoalRunScopeKind;
  run?: GoalRunLike | null;
  daemonOptions: GoalRunDaemonOption[];
}): string {
  if (input.selectedScopeKind !== 'daemon') {
    return input.currentMachineID;
  }
  if (input.currentMachineID.trim().length > 0) {
    return input.currentMachineID;
  }
  const scope = preferredScope(input.run);
  if (scope?.kind === 'daemon' && scope.machine_id) {
    return scope.machine_id;
  }
  return input.daemonOptions[0]?.value ?? '';
}

export function isRunnableDaemonMachineSelected(
  selectedMachineID: string,
  daemonOptions: GoalRunDaemonOption[],
): boolean {
  const machineID = selectedMachineID.trim();
  if (machineID.length === 0) {
    return false;
  }
  return daemonOptions.some((option) => option.value === machineID);
}
