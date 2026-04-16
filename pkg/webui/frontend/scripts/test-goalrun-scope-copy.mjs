import { readFileSync } from 'node:fs';

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

const en = JSON.parse(readFileSync(new URL('../public/i18n/en.json', import.meta.url), 'utf8'));
const zh = JSON.parse(readFileSync(new URL('../public/i18n/zh-CN.json', import.meta.url), 'utf8'));
const ja = JSON.parse(readFileSync(new URL('../public/i18n/ja.json', import.meta.url), 'utf8'));

assert(en.goalRunsScopePendingSelection === 'Not confirmed yet', 'en pending-selection copy mismatch');
assert(zh.goalRunsScopePendingSelection === '尚未确认', 'zh-CN pending-selection copy mismatch');
assert(ja.goalRunsScopePendingSelection === 'まだ確認されていません', 'ja pending-selection copy mismatch');
assert(en.goalRunsScopeUnknown !== en.goalRunsScopePendingSelection, 'en unknown and pending-selection should differ');
assert(zh.goalRunsScopeUnknown !== zh.goalRunsScopePendingSelection, 'zh unknown and pending-selection should differ');
assert(ja.goalRunsScopeUnknown !== ja.goalRunsScopePendingSelection, 'ja unknown and pending-selection should differ');

console.log('PASS goal run selected-scope copy distinguishes unavailable vs pending confirmation');
