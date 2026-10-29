// Base backups on Player > Bases (#475, phase 1): the list of the bases the game's tool picked up, and the button that saves one as a file. The export only reads the save, so
// nothing is posted and nothing is queued; the file reaches the browser as a blob and is never parsed in the page.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

test.afterEach(() => closeAll());

const BASE7 = [{ base_id: 7, name: 'Base', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 0, placeables: 0, x: 0, y: 0, z: 0 }];
const BACKUP = { backup_id: 3, name: 'Sub-Fief', backup_name: '##Totem_Small_Placeable', map: 'HaggaBasin', actors: 44, picked_up: 44, pieces: 1174, placeables: 43, stored_items: 53 };
const openBases = async (ui) => {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  return ui.doc.querySelector('#main');
};
const spy = (ui) => {
  const urls = [], blobs = [], names = [];
  const real = ui.w.fetch;
  ui.w.fetch = (p, o) => { urls.push(String(p)); return real(p, o); };
  ui.w.URL.createObjectURL = (b) => { blobs.push(b); return 'blob:x'; };
  ui.w.URL.revokeObjectURL = () => {};
  ui.w.HTMLAnchorElement.prototype.click = function () { names.push(this.download); };
  return { urls, blobs, names };
};

test('the Bases tab lists the base backups with the counts the file will hold, and a button for each', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [BACKUP] } });
  const main = await openBases(ui);
  const card = [...main.querySelectorAll('.card')].find((c) => c.querySelector('h3') && c.querySelector('h3').textContent.startsWith('Base backups'));
  assert.ok(card, 'there is a Base backups card');
  assert.equal(card.querySelector('h3').textContent, 'Base backups (1)');
  const cells = [...card.querySelectorAll('tbody td')].map((td) => td.textContent.trim());
  assert.deepEqual(cells.slice(0, 7), ['3', 'Sub-Fief', 'HaggaBasin', '44', (1174).toLocaleString(), '43', '53']);
  const heads = [...card.querySelectorAll('th')].map((th) => th.textContent.trim());
  assert.deepEqual(heads.slice(0, 7), ['ID', 'Name', 'Map', 'Actors', 'Building pieces', 'Placeables', 'Stored items']);
  const b = card.querySelector('[data-act="baseBackupExport"]');
  assert.equal(b.textContent.trim(), 'Export base backup');
  assert.equal(b.dataset.id, '3');
  assert.equal(ui.posted.length, 0, 'drawing the list writes nothing');
  ui.dom.window.close();
});

test('with no base backup the card says how to make one and has no button', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [] } });
  const main = await openBases(ui);
  assert.ok(main.textContent.includes('Base backups (0)'));
  assert.ok(main.textContent.includes('No base backups in this save. Pick a base up with the game\'s base tool, then reload.'));
  assert.equal(main.querySelector('[data-act="baseBackupExport"]'), null);
  ui.dom.window.close();
});

test('Export base backup asks the editor for that backup and hands the file to the browser, named with nothing unsafe in it; nothing is written', async () => {
  const hostile = { ...BACKUP, backup_id: 5, name: '../<b>Evil</b> "base"' };
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [hostile], '/api/bases/backups/export': {} } });
  const s = spy(ui);
  await openBases(ui);
  ui.click('[data-act="baseBackupExport"]'); await ui.settle();
  assert.deepEqual(s.urls.filter((u) => u.includes('/export')), ['/api/bases/backups/export?backup=5']);
  assert.equal(s.blobs.length, 1);
  assert.deepEqual(s.names, ['tabr-tau-base-backup-5-b_Evil_b_base.json']);
  assert.ok(!/[\/<>"]/.test(s.names[0]));
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('1,174 pieces') || t.includes('1174 pieces'), t);
  assert.ok(t.includes('43 placeables') && t.includes('53 stored items') && t.includes('Nothing in the save changed.'), t);
  assert.equal(ui.posted.length, 0, 'an export posts nothing');
  assert.ok(!ui.doc.querySelector('#toast').textContent.includes('queued'), 'an export is never queued');
  ui.dom.window.close();
});

test('a refused export shows the reason and saves no file', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [BACKUP], '/api/bases/backups/export': { __fail: 'A storage of this base holds a Solido blueprint, so the base was not exported.', __status: 422 } } });
  const s = spy(ui);
  await openBases(ui);
  ui.click('[data-act="baseBackupExport"]'); await ui.settle();
  assert.equal(s.blobs.length, 0);
  assert.equal(s.names.length, 0);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('holds a Solido blueprint'));
  ui.dom.window.close();
});

test('when the list cannot be read the bases are still shown and the card says why', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': { __fail: 'could not read the base backups', __status: 500 } } });
  const main = await openBases(ui);
  assert.ok(main.textContent.includes('Bases (1)'));
  assert.ok(main.textContent.includes('could not read the base backups'));
  assert.equal(main.querySelector('[data-act="baseBackupExport"]'), null);
  ui.dom.window.close();
});

test('About this on the card names the format and says a file cannot be imported yet', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [BACKUP] } });
  const main = await openBases(ui);
  const d = main.querySelector('details[data-about="bases.backups"]');
  assert.ok(d && d.textContent.includes('tabr-tau-base') && d.textContent.includes('only reads the save') && d.textContent.includes('cannot be imported yet'));
  ui.dom.window.close();
});
