export type GoalRunScopeKind = 'server' | 'daemon';

export interface GoalRunScopeLike {
  kind: GoalRunScopeKind;
  machine_id?: string;
  source: string;
}

export interface GoalRunLike {
  selected_scope?: GoalRunScopeLike | null;
  recommended_scope?: GoalRunScopeLike | null;
}

type Translate = (key: string, ...args: Array<string | number>) => string;

export function formatGoalRunScope(scope: GoalRunScopeLike | null | undefined, t: Translate): string {
  if (!scope) {
    return t('goalRunsScopeUnknown');
  }
  const source = scope.source === 'manual' ? t('goalRunsScopeManual') : t('goalRunsScopeAuto');
  if (scope.kind === 'daemon') {
    return scope.machine_id
      ? t('goalRunsScopeDaemonWithMachine', scope.machine_id, source)
      : t('goalRunsScopeDaemon', source);
  }
  return t('goalRunsScopeServer', source);
}

export function formatGoalRunSelectedScope(run: GoalRunLike, t: Translate): string {
  if (run.selected_scope) {
    return formatGoalRunScope(run.selected_scope, t);
  }
  if (run.recommended_scope) {
    return t('goalRunsScopePendingSelection');
  }
  return t('goalRunsScopeUnknown');
}

export function formatGoalRunScopeSummary(run: GoalRunLike, t: Translate): string {
  return formatGoalRunScope(run.selected_scope ?? run.recommended_scope, t);
}
