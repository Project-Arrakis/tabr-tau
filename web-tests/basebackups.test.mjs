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

test('About this on the card names the format and says what export and import do', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [BACKUP] } });
  const main = await openBases(ui);
  const d = main.querySelector('details[data-about="bases.backups"]');
  assert.ok(d && d.textContent.includes('tabr-tau-base') && d.textContent.includes('only reads the save') && d.textContent.includes('Importing the same file twice adds two backups'));
  ui.dom.window.close();
});

// The Dune console's file (#534): Export for console on a standing base (Bases list) and on a base backup (Base backups card). Both only read the save.
const HDR = (c, notes = []) => ({ 'X-Base-Counts': JSON.stringify(c), 'X-Base-Notes': encodeURIComponent(JSON.stringify(notes)) });
const COUNTS = { actors: 4, pieces: 312, placeables: 41, storedItems: 9, entities: 40, entries: 600 };

test('every standing base on the list has Export for console, a base the game picked up does not, and the sentence is a tooltip', async () => {
  const list = [{ ...BASE7[0], base_id: 7, name: 'Home' }, { ...BASE7[0], base_id: 8, name: 'Old', picked_up: true }];
  const ui = await boot({ get: { '/api/bases/list': list, '/api/bases/backups': [] } });
  const main = await openBases(ui);
  const buttons = [...main.querySelectorAll('[data-act="baseConsoleExport"]')];
  assert.equal(buttons.length, 1);
  assert.equal(buttons[0].textContent.trim(), 'Export for console');
  assert.deepEqual([buttons[0].dataset.kind, buttons[0].dataset.id, buttons[0].dataset.name], ['base', '7', 'Home']);
  const tipBox = main.querySelector('#' + buttons[0].getAttribute('aria-describedby'));
  assert.ok(tipBox && tipBox.getAttribute('role') === 'tooltip' && tipBox.textContent.includes('Reads the save and changes nothing'));
  assert.ok(![...main.querySelectorAll('.card')][0].textContent.replace(tipBox.textContent, '').includes('Dune console'), 'the sentence is not on the page itself');
  assert.equal(ui.posted.length, 0);
  ui.dom.window.close();
});

test('Export for console on a base asks for that totem, saves the file under a safe name and repeats the counts of the file', async () => {
  const list = [{ ...BASE7[0], base_id: 970, name: '../<b>Evil</b> "base"', pieces: 1, placeables: 1 }];
  const ui = await boot({ get: { '/api/bases/list': list, '/api/bases/backups': [], '/api/bases/console-export': { __headers: HDR(COUNTS) } } });
  const s = spy(ui);
  await openBases(ui);
  ui.click('[data-act="baseConsoleExport"]'); await ui.settle();
  assert.deepEqual(s.urls.filter((u) => u.includes('console-export')), ['/api/bases/console-export?base=970']);
  assert.equal(s.blobs.length, 1);
  assert.deepEqual(s.names, ['tabr-tau-console-base-970-b_Evil_b_base.json']);
  assert.ok(!/[\/<>"]/.test(s.names[0]));
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('312 pieces') && t.includes('41 placeables') && t.includes('9 stored items') && t.includes('Nothing in the save changed.') && t.includes('Dune console'), t);
  assert.equal(ui.posted.length, 0, 'an export posts nothing');
  ui.dom.window.close();
});

test('Export for console on a base backup asks for the console format of that backup and says what the file holds', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [BACKUP], '/api/bases/backups/export': { __headers: HDR({ ...COUNTS, pieces: 1174, placeables: 43, storedItems: 53 }, ['1 references name an actor that is not in the base.']) } } });
  const s = spy(ui);
  const main = await openBases(ui);
  const card = [...main.querySelectorAll('.card')].find((c) => c.querySelector('h3') && c.querySelector('h3').textContent.startsWith('Base backups'));
  assert.equal(card.querySelectorAll('[data-act]').length >= 2, true);
  assert.equal(card.querySelector('[data-act="baseBackupExport"]').textContent.trim(), 'Export base backup', 'the editor\'s own export is still there');
  ui.click('[data-act="baseConsoleExport"][data-kind="backup"]'); await ui.settle();
  assert.deepEqual(s.urls.filter((u) => u.includes('/export')), ['/api/bases/backups/export?format=console&backup=3']);
  assert.deepEqual(s.names, ['tabr-tau-console-backup-3-Sub-Fief.json']);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok((t.includes('1,174 pieces') || t.includes('1174 pieces')) && t.includes('43 placeables') && t.includes('53 stored items') && t.includes('1 references name an actor that is not in the base.'), t);
  assert.equal(ui.posted.length, 0);
  ui.dom.window.close();
});

test('without the headers the numbers of the row are used, and a refusal shows its reason and saves nothing', async () => {
  let ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [], '/api/bases/console-export': {} } });
  let s = spy(ui);
  await openBases(ui);
  ui.click('[data-act="baseConsoleExport"]'); await ui.settle();
  assert.equal(s.blobs.length, 1);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('0 pieces'));
  ui.dom.window.close();

  ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [], '/api/bases/console-export': { __fail: 'This base has no owner (nobody holds rank 1 over it), so it cannot be saved as a base backup.', __status: 409 } } });
  s = spy(ui);
  await openBases(ui);
  ui.click('[data-act="baseConsoleExport"]'); await ui.settle();
  assert.equal(s.blobs.length, 0);
  assert.equal(s.names.length, 0);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('has no owner'));
  ui.dom.window.close();
});

test('About this on the card names the console file, what it holds, and that the Solido file is not written yet', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/backups': [BACKUP] } });
  const main = await openBases(ui);
  const d = main.querySelector('details[data-about="bases.backups"]');
  assert.ok(d && d.textContent.includes('Export for console') && d.textContent.includes('dune-base-backup') && d.textContent.includes('owner as a placeholder') && d.textContent.includes('Solido blueprint file is not written yet'));
  ui.dom.window.close();
});
