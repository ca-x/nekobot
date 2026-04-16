import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { execFileSync } from 'node:child_process';

const repoRoot = new URL('../../../../', import.meta.url);
const workspace = mkdtempSync(join(tmpdir(), 'goalrun-scope-format-'));
const outDir = join(workspace, 'out');

const translations = {
  goalRunsScopeUnknown: 'Scope unavailable',
  goalRunsScopePendingSelection: 'Not confirmed yet',
  goalRunsScopeManual: 'manual',
  goalRunsScopeAuto: 'auto',
  goalRunsScopeDaemon: 'Daemon · {0}',
  goalRunsScopeDaemonWithMachine: 'Daemon {0} · {1}',
  goalRunsScopeServer: 'WebUI server · {0}',
};

function t(key, ...args) {
  const template = translations[key];
  if (!template) {
    throw new Error(`missing translation: ${key}`);
  }
  return args.reduce((value, arg, index) => value.replace(`{${index}}`, String(arg)), template);
}

function assertEqual(actual, expected, label) {
  if (actual !== expected) {
    throw new Error(`${label}: expected ${expected}, got ${actual}`);
  }
  console.log(`PASS ${label}`);
}

try {
  execFileSync(
    resolve(repoRoot.pathname, 'pkg/webui/frontend/node_modules/.bin/tsc'),
    [
      '--target',
      'es2022',
      '--module',
      'nodenext',
      '--moduleResolution',
      'nodenext',
      '--outDir',
      outDir,
      'pkg/webui/frontend/src/pages/goalRunScopeCopy.ts',
    ],
    {
      cwd: repoRoot,
      stdio: 'inherit',
    },
  );

  const mod = await import(`file://${join(outDir, 'goalRunScopeCopy.js')}`);

  assertEqual(
    mod.formatGoalRunScopeSummary(
      { recommended_scope: { kind: 'server', source: 'auto' } },
      t,
    ),
    'WebUI server · auto',
    'summary falls back to recommended scope',
  );

  assertEqual(
    mod.formatGoalRunSelectedScope(
      { recommended_scope: { kind: 'server', source: 'auto' } },
      t,
    ),
    'Not confirmed yet',
    'selected scope shows pending copy when only recommended scope exists',
  );

  assertEqual(
    mod.formatGoalRunSelectedScope(
      { selected_scope: { kind: 'daemon', machine_id: 'machine-a', source: 'manual' } },
      t,
    ),
    'Daemon machine-a · manual',
    'selected scope prefers explicit daemon selection',
  );

  assertEqual(
    mod.formatGoalRunScope(undefined, t),
    'Scope unavailable',
    'raw formatter keeps unavailable copy for truly missing scope',
  );
} finally {
  rmSync(workspace, { recursive: true, force: true });
}
