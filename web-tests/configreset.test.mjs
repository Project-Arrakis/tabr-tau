// Config: reset a setting, or all of them, to the game's default (#348). Each setting that has a verified default shows it and a Reset button (off when already the default);
// a setting without a verified default says "default unknown" and its Reset is off; Reset all asks with a typed sentence that names the file and the count, and sends the count.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll, GAP } from './boot.mjs';

afterEach(closeAll);

const S = '/Script/DuneSandbox.UserServerCustomSettings';
const FILE = {
  name: 'ServerCustomSettings.ini', text: '', gameRunning: false,
  entries: [
    { line: 1, section: S, key: 'DifficultyLevel', value: 'Custom' },
    { line: 2, section: S, key: 'GatheringAmount', value: '4.000000' },
    { line: 3, section: S, key: 'PVPMode', value: 'FullPVP' },
    { line: 4, section: S, key: 'LootRespawnSpeed', value: '1.000000' },
  ],
  defaults: [
    { line: 1, key: 'DifficultyLevel', known: false, atDefault: false },
    { line: 2, key: 'GatheringAmount', known: true, default: '1.000000', atDefault: false, min: 0.1, max: 10 },
    { line: 3, key: 'PVPMode', known: true, default: 'Limited', atDefault: false, options: ['NoPVP', 'Limited', 'FullPVP'] },
    { line: 4, key: 'LootRespawnSpeed', known: true, default: '1.000000', atDefault: true, min: 0.1, max: 10 },
  ],
  resetCounts: { change: 2, atDefault: 1, noDefault: 1, otherParts: 0 },
};
const FILES = [{ name: 'ServerCustomSettings.ini', empty: false }];
const GETS = (f = FILE) => ({ '/api/config/files': FILES, '/api/config/file': f, '/api/config/backups': [] });
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);
async function openConfig(ui) { ui.click('[data-tab="config"]'); await ui.settle(); }
const rowOf = (ui, key) => [...ui.doc.querySelectorAll('table tbody tr')].find((tr) => tr.children[1] && tr.children[1].textContent.trim() === key);

test('Config: defaults and Reset buttons per setting; at default and unknown are off with their reason', async () => {
  const ui = await boot({ get: GETS() });
  await openConfig(ui);
  const heads = [...ui.doc.querySelectorAll('table thead th')].map((th) => th.textContent.trim());
  assert.ok(heads.includes('Default') && heads.includes('Reset'), heads.join());
  const g = rowOf(ui, 'GatheringAmount');
  assert.ok(g.textContent.includes('1.000000') && g.textContent.includes('range 0.1 to 10'), g.textContent);
  const gb = g.querySelector('button');
  assert.equal(gb.disabled, false);
  assert.equal(gb.dataset.confirm, 'Yes, reset');
  assert.ok(rowOf(ui, 'PVPMode').textContent.includes('one of NoPVP, Limited, FullPVP'));
  const atDef = rowOf(ui, 'LootRespawnSpeed').querySelector('button');
  assert.ok(atDef.disabled && atDef.title === 'Already the default');
  const unk = rowOf(ui, 'DifficultyLevel');
  assert.ok(unk.textContent.includes('default unknown'));
  assert.ok(unk.querySelector('button').disabled && /No verified default/.test(unk.querySelector('button').title));
  assert.equal(posts(ui, '/api/config/reset').length, 0);
});

test('Config: Reset on a row asks with the second press, sends the line and key, and says what changed and how to undo', async () => {
  const ui = await boot({ get: GETS(), post: { '/api/config/reset': { ok: true, result: { changed: 1 } } } }); ui.freeze();
  await openConfig(ui);
  ui.click('[data-act="cfgReset"][data-key="GatheringAmount"]'); await ui.settle();
  assert.equal(posts(ui, '/api/config/reset').length, 0, 'the first press only asks');
  ui.tick(GAP);
  ui.click('[data-act="cfgReset"][data-key="GatheringAmount"]'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/config/reset').map((p) => p.body), [{ name: 'ServerCustomSettings.ini', line: 2, key: 'GatheringAmount' }]);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('GatheringAmount: 4.000000 → 1.000000') && t.includes('To undo, set it back to 4.000000'), t);
});

test('Config: Reset all names the file and the count in the typed sentence, waits out the gap, and sends the count', async () => {
  const ui = await boot({ get: GETS(), post: { '/api/config/reset': { ok: true, result: { changed: 2 } } } }); ui.freeze();
  await openConfig(ui);
  const btn = ui.doc.querySelector('[data-act="cfgResetAll"]');
  assert.ok(btn && !btn.disabled);
  const note = ui.doc.getElementById(btn.getAttribute('aria-describedby')).textContent;
  assert.ok(note.includes('2 values differ') && note.includes('1 value is already') && note.includes('1 entry has no known default'), note);
  ui.click('[data-act="cfgResetAll"]'); await ui.settle();
  const dlg = ui.doc.querySelector('#ask [role="dialog"]');
  assert.ok(dlg.textContent.includes('Reset 2 settings to the game'), dlg.textContent);
  assert.ok(dlg.textContent.includes('Type reset 2 settings in ServerCustomSettings.ini to continue'));
  assert.equal(ui.doc.querySelector('#aok').disabled, true);
  ui.type('reset 2 settings in ServerCustomSettings.ini');
  assert.equal(ui.doc.querySelector('#aok').disabled, false);
  ui.click('#aok'); await ui.settle();
  assert.equal(posts(ui, '/api/config/reset').length, 0, 'an OK sooner than the gap is not a decision');
  ui.tick(GAP);
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/config/reset').map((p) => p.body), [{ name: 'ServerCustomSettings.ini', expect: 2 }]);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Reset 2 settings in ServerCustomSettings.ini'));
});

test('Config: Reset all is off when nothing differs, a file with no table of defaults has no Reset column, and a refusal is shown', async () => {
  let ui = await boot({ get: GETS({ ...FILE, resetCounts: { change: 0, atDefault: 3, noDefault: 0, otherParts: 0 } }) });
  await openConfig(ui);
  const btn = ui.doc.querySelector('[data-act="cfgResetAll"]');
  assert.ok(btn.disabled);
  assert.ok(ui.doc.getElementById('rstnote').textContent.includes('already at it'));
  ui.dom.window.close();

  const plain = { name: 'ServerCustomSettings.ini', text: '', gameRunning: false, entries: FILE.entries };
  ui = await boot({ get: GETS(plain) });
  await openConfig(ui);
  assert.equal(ui.doc.querySelector('[data-act="cfgResetAll"]'), null);
  assert.ok(![...ui.doc.querySelectorAll('table thead th')].some((th) => th.textContent.trim() === 'Reset'));
  ui.dom.window.close();

  ui = await boot({ get: GETS(), post: { '/api/config/reset': { __fail: 'file changed; reload', __status: 400 } } }); ui.freeze();
  await openConfig(ui);
  ui.click('[data-act="cfgResetAll"]'); await ui.settle();
  ui.type('reset 2 settings in ServerCustomSettings.ini');
  ui.tick(GAP);
  ui.click('#aok'); await ui.settle();
  const err = ui.doc.querySelector('#toast .err');
  assert.ok(err && err.textContent.includes('file changed; reload'));
  ui.dom.window.close();
});
