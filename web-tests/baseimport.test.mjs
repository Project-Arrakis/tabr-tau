// Import a base file on Player > Bases > Base backups (#475, phase 2): choose a file, see what would be added (a preview that changes nothing), press Import. The file is read in the
// browser as text and posted; the import asks no question because it only adds rows.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

test.afterEach(() => closeAll());

const BASE7 = [{ base_id: 7, name: 'Base', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 0, placeables: 0, x: 0, y: 0, z: 0 }];
const PV = {
  ok: true, dryRun: true, changed: true, name: 'Advanced Sub-Fief', state: 5, fileHash: 'abc123', patchesDiffer: false,
  summary: { actors: 56, pieces: 179, placeables: 55, storedItems: 99, entities: 86, rows: 736 },
  owner: { controller: 1, character: 'Shiya', rows: 233 }, warnings: [], ids: { backup: { first: 3, last: 3, count: 1 } },
};
const open = async (ui) => {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  return ui.doc.querySelector('#main');
};
// chooses a file the way the browser does: the input's files hold it and a change event fires
const choose = async (ui, name, text, size) => {
  const input = ui.doc.querySelector('#baseFile');
  assert.ok(input, 'there is a file input');
  Object.defineProperty(input, 'files', { value: [{ name, size: size ?? text.length, text: async () => text }], configurable: true });
  input.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  await ui.settle();
};
const boots = (post = {}) => boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [] }, post });

test('the card has a file picker and the explanation is only in About this', async () => {
  const ui = await boots();
  const main = await open(ui);
  const label = [...main.querySelectorAll('label')].find((l) => l.textContent.includes('Import a base file'));
  assert.ok(label && label.querySelector('input[type=file]#baseFile'));
  assert.equal(ui.doc.querySelector('#baseImportPanel'), null, 'nothing is shown before a file is chosen');
  const about = [...main.querySelectorAll('details.about')].find((d) => d.textContent.includes('Import reads a file'));
  assert.ok(about && !about.open, 'the explanation is in a closed About this');
  assert.equal(ui.posted.length, 0);
  ui.dom.window.close();
});

test('choosing a file posts it for a preview, shows the counts and the warnings, and writes nothing', async () => {
  const pv = { ...PV, warnings: ['1 references to an actor that is not in the file (a crafting queue) were set to none.', '<b>markup</b> stays text'] };
  const ui = await boots({ '/api/bases/backups/import-preview': pv });
  await open(ui);
  await choose(ui, 'base.json', '{"format":"tabr-tau-base"}');
  assert.deepEqual(ui.posted.map((p) => p.path), ['/api/bases/backups/import-preview']);
  assert.deepEqual(ui.posted[0].body, { text: '{"format":"tabr-tau-base"}' });
  const panel = ui.doc.querySelector('#baseImportPanel');
  assert.ok(panel, 'a preview panel');
  const t = panel.textContent;
  assert.ok(t.includes('56 actors') && t.includes('179 building pieces') && t.includes('55 placeables') && t.includes('99 stored items') && t.includes('736 rows') && t.includes('Shiya'), t);
  assert.ok(t.includes('Nothing in the save has changed.'), t);
  assert.equal(panel.querySelectorAll('li').length, 2);
  assert.equal(panel.querySelector('li b'), null, 'a warning is text, not markup');
  assert.equal(panel.querySelector('[data-act="baseImportGo"]').textContent.trim(), 'Import');
  assert.equal(ui.doc.querySelector('#baseFile').value, '', 'the picker is cleared so the same file can be chosen again');
  ui.dom.window.close();
});

test('Import sends the previewed text with its hash, asks nothing, and says where the base goes', async () => {
  const ui = await boots({ '/api/bases/backups/import-preview': PV, '/api/bases/backups/import': { ok: true, dryRun: false, name: 'Advanced Sub-Fief', summary: PV.summary, ids: PV.ids, outcome: 'written' } });
  await open(ui);
  await choose(ui, 'base.json', '{"x":1}');
  ui.click('[data-act="baseImportGo"]'); await ui.settle();
  assert.deepEqual(ui.posted.map((p) => p.path), ['/api/bases/backups/import-preview', '/api/bases/backups/import']);
  assert.deepEqual(ui.posted[1].body, { text: '{"x":1}', fileHash: 'abc123' });
  assert.equal(ui.doc.querySelector('#atype'), null, 'no typed confirmation');
  const toast = ui.doc.querySelector('#toast').textContent;
  assert.ok(toast.includes('Imported "Advanced Sub-Fief" as base backup 3') && toast.includes('179 pieces') && toast.includes('Base Reconstruction Tool'), toast);
  assert.ok(!toast.includes('queued'), toast);
  assert.equal(ui.doc.querySelector('#baseImportPanel'), null, 'the panel is gone after the import');
  ui.dom.window.close();
});

test('a queued import says so', async () => {
  const ui = await boots({ '/api/bases/backups/import-preview': PV, '/api/bases/backups/import': { ok: true, name: 'X', summary: PV.summary, ids: PV.ids, outcome: 'queued' } });
  await open(ui);
  await choose(ui, 'base.json', '{}');
  ui.click('[data-act="baseImportGo"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('queued'));
  ui.dom.window.close();
});

test('a file the editor refuses shows the reason in the panel, offers no Import, and Dismiss clears it', async () => {
  const ui = await boots({ '/api/bases/backups/import-preview': { __fail: 'This file cannot be imported: row 1 of items names inventory 99999 that is not in the file. Nothing was written.', __status: 422 } });
  await open(ui);
  await choose(ui, '<i>evil</i>.json', 'junk');
  const panel = ui.doc.querySelector('#baseImportPanel');
  assert.ok(panel.textContent.includes('inventory 99999 that is not in the file'));
  assert.equal(panel.querySelector('[data-act="baseImportGo"]'), null, 'nothing to import');
  assert.equal(panel.querySelector('i'), null, 'the file name is text');
  assert.equal(ui.posted.length, 1, 'only the preview was posted');
  ui.click('[data-act="baseImportCancel"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#baseImportPanel'), null);
  ui.dom.window.close();
});

test('a different set of game patches changes the button to Import anyway; Cancel keeps nothing', async () => {
  const ui = await boots({ '/api/bases/backups/import-preview': { ...PV, patchesDiffer: true, warnings: ['The save the file came from had 28 game patches applied; this save has 27.'] } });
  await open(ui);
  await choose(ui, 'base.json', '{}');
  assert.equal(ui.doc.querySelector('[data-act="baseImportGo"]').textContent.trim(), 'Import anyway');
  ui.click('[data-act="baseImportCancel"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#baseImportPanel'), null);
  assert.equal(ui.posted.length, 1, 'cancelling posts nothing');
  ui.dom.window.close();
});

test('a file over 16 MiB is not read or sent', async () => {
  const ui = await boots();
  await open(ui);
  await choose(ui, 'huge.json', 'x', 17 * 1024 * 1024);
  assert.equal(ui.posted.length, 0);
  assert.ok(ui.doc.querySelector('#baseImportPanel').textContent.includes('larger than 16 MiB'));
  ui.dom.window.close();
});

test('a refused import keeps the panel so the person can read why', async () => {
  const ui = await boots({ '/api/bases/backups/import-preview': PV, '/api/bases/backups/import': { __fail: 'This is not the file that was previewed. Nothing was imported. Choose the file again.', __status: 409 } });
  await open(ui);
  await choose(ui, 'base.json', '{}');
  ui.click('[data-act="baseImportGo"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('not the file that was previewed'));
  assert.ok(ui.doc.querySelector('#baseImportPanel'), 'the panel stays');
  ui.dom.window.close();
});

test('a file of the Dune console says so in the preview, with its name, map and counts, as text', async () => {
  const from = 'Read from a file of the Dune console (v1.4.52), base backup "<i>My Base</i>", map HaggaBasin, 811 pieces, 204 placeables, 21 stored items, exported 2026-10-25T18:02:57.884Z.';
  const ui = await boots({ '/api/bases/backups/import-preview': { ...PV, from, file: { origin: 'console' } } });
  await open(ui);
  await choose(ui, 'base-backup.json', '{"format":"dune-base-backup"}');
  const panel = ui.doc.querySelector('#baseImportPanel');
  const t = panel.textContent;
  assert.ok(t.includes('the Dune console (v1.4.52)') && t.includes('HaggaBasin') && t.includes('811 pieces') && t.includes('204 placeables') && t.includes('21 stored items'), t);
  assert.equal(panel.querySelector('p i'), null, 'the name from the file is text');
  assert.ok(panel.querySelector('[data-act="baseImportGo"]'));
  ui.dom.window.close();
});

test('a file of the editor has no console line', async () => {
  const ui = await boots({ '/api/bases/backups/import-preview': PV });
  await open(ui);
  await choose(ui, 'base.json', '{}');
  assert.ok(!ui.doc.querySelector('#baseImportPanel').textContent.includes('Dune console'));
  ui.dom.window.close();
});
