// The text sweep (#426) on Config and Database. See uitext.test.mjs for the checks that hold on every page.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { visibleText, movedText } from './uitext.mjs';

test.afterEach(() => closeAll());

const S = '/Script/DuneSandbox.UserServerCustomSettings';
const FILE = {
  name: 'ServerCustomSettings.ini', text: '', gameRunning: false,
  entries: [{ line: 1, section: S, key: 'GatheringAmount', value: '4.000000' }, { line: 2, section: S, key: 'DifficultyLevel', value: 'Custom' }],
  defaults: [{ line: 1, key: 'GatheringAmount', known: true, default: '1.000000', atDefault: false, min: 0.1, max: 10 }, { line: 2, key: 'DifficultyLevel', known: false, atDefault: false }],
  resetCounts: { change: 1, atDefault: 0, noDefault: 1, otherParts: 0 },
};
const VALID = { dirExists: true, files: [{ name: 'ServerCustomSettings.ini', exists: true, missingSections: [] }, { name: 'Game.ini', exists: true, missingSections: [] }] };
const gets = (f = FILE, v = VALID) => ({ '/api/config/validate': v, '/api/config/files': [{ name: 'ServerCustomSettings.ini', empty: false }], '/api/config/file': f, '/api/config/backups': [] });
const go = async (ui, tab) => { ui.click(`[data-tab="${tab}"]`); await ui.settle(); return ui.doc.querySelector('#main'); };

test('Config: the success line is short, the closing sentence of the problem box is gone, the rewrite warning and the backup stay, the source of the defaults is in About this', async () => {
  let ui = await boot({ get: gets() });
  let main = await go(ui, 'config');
  let shown = visibleText(main);
  assert.ok(shown.includes('All expected config files are present.'));
  assert.ok(!shown.includes('ServerCustomSettings.ini, Game.ini'), 'the list of names is the choice below');
  assert.ok(shown.includes('The game rewrites its config files when it exits. Close Dune before saving changes here.') && shown.includes('tabr-tau-backups') && shown.includes('Backups list is below'), shown);
  assert.ok(!shown.includes('Dune Docker console') && movedText(main).includes('The defaults are the Dune Docker console\'s table for the game\'s settings (MIT, RedBlink)'), 'the credit moved, with its words');
  assert.ok(shown.includes('1 value differs from the default and would change.'), 'the state stays');
  assert.ok(ui.doc.getElementById('rstnote').textContent.includes('already at the default'));
  assert.ok(!ui.doc.getElementById('rstnote').textContent.includes('Dune Docker'));
  ui.dom.window.close();

  ui = await boot({ get: gets(FILE, { dirExists: true, files: [{ name: 'Game.ini', exists: false, missingSections: [] }] }) });
  main = await go(ui, 'config');
  shown = visibleText(main);
  assert.ok(shown.includes('Config check:') && shown.includes('file not found') && shown.includes('Create'), 'the problem stays');
  assert.ok(!shown.includes('Missing files are normally created by the game'));
  ui.dom.window.close();

  ui = await boot({ get: gets({ ...FILE, defaults: undefined, resetCounts: undefined }) });
  main = await go(ui, 'config');
  assert.equal(ui.doc.querySelector('details[data-about="config.defaults"]'), null, 'no table of defaults for this file: no About this about it');
  ui.dom.window.close();
});

test('Database: Write mode\'s explanation is a tooltip, the read-only columns sentence is in About this and only when some column is read-only, the backup line is short and keeps its facts', async () => {
  let ui = await boot({ get: { '/api/db/tables': [{ name: 'items', rows: 1, columns: ['id'], types: ['INTEGER'] }], '/api/db/table': { columns: ['_rowid_', 'id', 'name'], rows: [[5, 10, 'x']], total: 1, tableLocked: '', locked: { id: 'primary key' } } } });
  let main = await go(ui, 'db');
  assert.ok(!visibleText(main).includes('Plain text columns are read-only'));
  assert.ok(movedText(main).includes('Plain text columns are read-only (primary keys, links to other tables and account identity); hover a value for the reason.'));
  assert.ok(visibleText(main).includes('1 row, showing 1–1'), 'the count stays');
  ui.dom.window.close();

  ui = await boot({ get: { '/api/db/tables': [{ name: 'accounts', rows: 1, columns: ['user'], types: ['TEXT'] }], '/api/db/table': { columns: ['_rowid_', 'user'], rows: [[1, 'u']], total: 1, tableLocked: 'account identity', locked: null } } });
  main = await go(ui, 'db');
  assert.ok(visibleText(main).includes('Read-only: account identity.'), 'a read-only table says so');
  assert.equal(ui.doc.querySelector('details[data-about="db.tables"]'), null);
  ui.dom.window.close();

  ui = await boot({});
  await go(ui, 'db');
  ui.click('[data-sub="db:sql"]'); await ui.settle();
  main = ui.doc.querySelector('#main');
  assert.ok(!visibleText(main).includes('Write mode runs one or more statements atomically'));
  assert.ok(ui.doc.getElementById('sqltip').textContent.includes('queued and written by themselves when it can be'));
  assert.equal(ui.doc.querySelector('[data-act="execsql"]').getAttribute('aria-describedby'), 'sqltip');
  assert.ok(visibleText(main).includes('Run (read-only)') && visibleText(main).includes('Run as write'));
  ui.dom.window.close();

  ui = await boot({ get: { '/api/save/backups': [{ name: 'a.db', size: 2048, modified: '2026-10-26T10:00:00Z' }] } });
  await go(ui, 'db');
  ui.click('[data-sub="db:backups"]'); await ui.settle();
  main = ui.doc.querySelector('#main');
  const t = visibleText(main);
  assert.ok(t.includes('A backup is taken before changes are written and before every restore. Restore needs the game to be closed.'), t);
  assert.ok(t.includes('Restore') && t.includes('a.db'));
  assert.ok(movedText(main).includes('One backup covers the changes made within two minutes.'), 'the two-minute detail moved, it is not lost');
  ui.dom.window.close();
});
