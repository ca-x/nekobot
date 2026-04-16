import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { execFileSync } from 'node:child_process';

const repoRoot = new URL('../../../../', import.meta.url);
const workspace = mkdtempSync(join(tmpdir(), 'goalrun-scope-selection-'));
const outDir = join(workspace, 'out');

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
      'pkg/webui/frontend/src/pages/goalRunScopeSelection.ts',
    ],
    {
      cwd: repoRoot,
      stdio: 'inherit',
    },
  );

  const mod = await import(`file://${join(outDir, 'goalRunScopeSelection.js')}`);

  const tests = [
    {
      name: 'prefers daemon machine already stored on the run',
      run: { recommended_scope: { kind: 'daemon', machine_id: 'machine-a' } },
      currentMachineID: '',
      selectedScopeKind: 'daemon',
      daemonOptions: [{ value: 'machine-b', label: 'Machine B' }],
      expected: 'machine-a',
    },
    {
      name: 'autofills the first runnable daemon once options arrive',
      run: { recommended_scope: { kind: 'daemon' } },
      currentMachineID: '',
      selectedScopeKind: 'daemon',
      daemonOptions: [{ value: 'machine-b', label: 'Machine B' }],
      expected: 'machine-b',
    },
    {
      name: 'does not override a user-selected daemon machine',
      run: { recommended_scope: { kind: 'daemon' } },
      currentMachineID: 'machine-user',
      selectedScopeKind: 'daemon',
      daemonOptions: [{ value: 'machine-b', label: 'Machine B' }],
      expected: 'machine-user',
    },
    {
      name: 'leaves server scope untouched',
      run: { recommended_scope: { kind: 'daemon', machine_id: 'machine-a' } },
      currentMachineID: '',
      selectedScopeKind: 'server',
      daemonOptions: [{ value: 'machine-b', label: 'Machine B' }],
      expected: '',
    },
  ];

  const availabilityTests = [
    {
      name: 'accepts a currently runnable daemon machine',
      selectedMachineID: 'machine-b',
      daemonOptions: [{ value: 'machine-b', label: 'Machine B' }],
      expected: true,
    },
    {
      name: 'rejects a stale daemon machine after options refresh',
      selectedMachineID: 'machine-stale',
      daemonOptions: [{ value: 'machine-b', label: 'Machine B' }],
      expected: false,
    },
  ];

  let failed = false;
  for (const test of tests) {
    const actual = mod.autofillDaemonMachineID({
      run: test.run,
      currentMachineID: test.currentMachineID,
      selectedScopeKind: test.selectedScopeKind,
      daemonOptions: test.daemonOptions,
    });
    if (actual !== test.expected) {
      failed = true;
      console.error(`FAIL ${test.name}: expected ${test.expected}, got ${actual}`);
    } else {
      console.log(`PASS ${test.name}`);
    }
  }

  for (const test of availabilityTests) {
    const actual = mod.isRunnableDaemonMachineSelected(test.selectedMachineID, test.daemonOptions);
    if (actual !== test.expected) {
      failed = true;
      console.error(`FAIL ${test.name}: expected ${test.expected}, got ${actual}`);
    } else {
      console.log(`PASS ${test.name}`);
    }
  }

  if (failed) {
    process.exitCode = 1;
  }
} finally {
  rmSync(workspace, { recursive: true, force: true });
}
