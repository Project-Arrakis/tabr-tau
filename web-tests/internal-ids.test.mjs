// The game's own numbers and keys are not shown to the player (#618): vendors, claims, bases, base backups, containers, stored vehicles and the queue list say what a thing is,
// and its number is in a tooltip or in a list of ids for support. Display only: every button still sends the id it always sent.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const goTo = async (ui, ...sels) => { for (const s of sels) { ui.click(s); await ui.settle(); } };
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();

const VENDORS = { stock: [{ vendor_id: 'VEND-AAA-111', template_id: 'SolarisCoin', amount_bought: 3 }, { vendor_id: 'VEND-AAA-111', template_id: 'Literjon', amount_bought: 1 }, { vendor_id: 'VEND-BBB-222', template_id: 'SolarisCoin', amount_bought: 2 }],
  cycles: [{ vendor_id: 'VEND-BBB-222', last_interacted_timestamp: 1790000000 }, { vendor_id: 'VEND-CCC-333', last_interacted_timestamp: 1790000100 }] };

test('vendors are Vendor 1, Vendor 2 in both tables, with the id in a tooltip and in a list of ids, and the reset still sends the id (#618)', async () => {
  const ui = await boot({ get: { '/api/vendors': VENDORS, '/api/catalog/items': [] }, post: { '/api/vendors/reset': { ok: true } } });
  await goTo(ui, '[data-tab="player"]', '[data-sub="player:admin"]');
  const main = ui.doc.querySelector('#main');
  const heads = [...main.querySelectorAll('th')].map((t) => t.textContent.trim());
  assert.ok(heads.includes('Vendor') && !heads.includes('Vendor ID'), heads.join('|'));
  const cells = [...main.querySelectorAll('td span[title^="Vendor id"]')];
  assert.deepEqual(cells.map((c) => [c.textContent, c.title]), [
    ['Vendor 1', 'Vendor id VEND-AAA-111'], ['Vendor 1', 'Vendor id VEND-AAA-111'], ['Vendor 2', 'Vendor id VEND-BBB-222'],
    ['Vendor 2', 'Vendor id VEND-BBB-222'], ['Vendor 3', 'Vendor id VEND-CCC-333']]);
  const shown = [...main.querySelectorAll('td, th, h3, button, p')].map(text).join(' ');
  assert.ok(!/VEND-/.test(shown), 'no vendor id in what is shown');
  const ids = [...main.querySelectorAll('details.ids')].find((d) => text(d.querySelector('summary')) === 'Vendor ids');
  assert.ok(ids, 'the ids are kept in a list for support');
  assert.match(text(ids), /Vendor 2s*VEND-BBB-222/);
  const one = [...main.querySelectorAll('[data-act="vreset"]')].find((b) => b.dataset.v === 'VEND-BBB-222');
  assert.equal(one.dataset.vn, 'Vendor 2');
  ui.click('[data-act="vreset"][data-v="VEND-BBB-222"]'); await ui.settle();
  const dlg = text(ui.doc.body);
  assert.ok(dlg.includes('Resets the purchase counters of Vendor 2.') && !dlg.includes('purchase counters of vendor VEND'), 'the question names the vendor as the tables do');
  ui.dom.window.close();
});

const baseRow = (id, name) => ({ base_id: id, name, base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 3, placeables: 2, x: 0, y: 0, z: 0 });

test('the Bases list has no ID column and no number in the open base heading; the number is in a tooltip (#618)', async () => {
  const ui = await boot({ get: { '/api/bases/list': [baseRow(7, 'Home'), baseRow(8, 'Outpost')], '/api/bases/backups': [] } });
  await goTo(ui, '[data-tab="player"]', '[data-sub="player:bases"]');
  const card = [...ui.doc.querySelectorAll('#main .card')].find((c) => (c.querySelector('h3') || {}).textContent === 'Bases (2)');
  assert.ok(card);
  const heads = [...card.querySelectorAll('th')].map((t) => t.textContent.trim());
  assert.ok(!heads.includes('ID'), heads.join('|'));
  assert.deepEqual([...card.querySelectorAll('tbody tr')].map((r) => r.querySelector('span[title^="Base number"]').title), ['Base number 7', 'Base number 8']);
  assert.equal(text(ui.doc.querySelector('#main h3[title="Base number 7"]')), 'Home');
  assert.ok(!/#\d/.test(text(ui.doc.querySelector('#main'))), 'no #number anywhere on the tab');
  assert.equal(ui.doc.querySelector('[data-bsopen="8"]').dataset.bsopen, '8', 'Open still sends the number');
  ui.click('[data-bsopen="8"]'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#main h3[title="Base number 8"]')), 'Outpost');
  ui.dom.window.close();
});

test('the base backups list has no ID column and the number is in the name tooltip (#618)', async () => {
  const backup = { backup_id: 3, name: 'Sub-Fief', backup_name: '##Totem_Small_Placeable', map: 'HaggaBasin', actors: 12, picked_up: 12, pieces: 1250, placeables: 9, stored_items: 7 };
  const ui = await boot({ get: { '/api/bases/list': [baseRow(7, 'Home')], '/api/bases/backups': [backup] } });
  await goTo(ui, '[data-tab="player"]', '[data-sub="player:bases"]');
  const card = [...ui.doc.querySelectorAll('#main .card')].find((c) => (c.querySelector('h3') || {}).textContent.startsWith('Base backups'));
  const heads = [...card.querySelectorAll('th')].map((t) => t.textContent.trim());
  assert.deepEqual(heads.slice(0, 2), ['Name', 'Map']);
  assert.equal(card.querySelector('tbody span[title]').title, 'Base backup number 3');
  assert.equal(card.querySelector('[data-act="baseBackupExport"]').dataset.id, '3');
  ui.dom.window.close();
});

test('a claim is "Claim", not its internal name and totem number; the totem is in a tooltip and in a list of ids (#618)', async () => {
  const claim = [{ totem_id: 7, name: 'BP_Totem_Placeable_C', cells: 2, rings: 0, level: 1, irregular: true, maxRings: 5, maxLevel: 5, yaw: 90, segments: [{ x: 1, y: 0 }], grid: [{ x: 0, y: 0, in: true, n: 0 }, { x: 1, y: 0, in: true, n: 3 }], piecesInClaim: 3, piecesOutside: 0 }];
  const ui = await boot({ get: { '/api/bases/list': [baseRow(7, 'Home')], '/api/bases/backups': [], '/api/bases/claim': claim } });
  await goTo(ui, '[data-tab="player"]', '[data-sub="player:bases"]', '[data-bstab="claim"]');
  const main = ui.doc.querySelector('#main');
  const t = text(main);
  assert.ok(!t.includes('Totem ID') && !t.includes('BP_Totem'), 'no internal name and no totem id in the card');
  const b = [...main.querySelectorAll('b')].find((x) => x.textContent === 'Claim');
  assert.ok(b && b.title === 'Totem 7');
  const ids = [...main.querySelectorAll('details.ids')].find((d) => text(d.querySelector('summary')) === 'Claim ids');
  assert.ok(ids && /Totems*7/.test(text(ids)));
  assert.equal(main.querySelector('[data-act="claimGrow"]').dataset.id, '7', 'Expand still sends the totem');
  ui.dom.window.close();
});

test('a queue line names a Journey step without its id when the Journey list is loaded, and keeps the id for the tooltip; a vehicle move shows no number (#618)', async () => {
  const ui = await boot({ get: { '/api/player/journey/browse': { story: [{ id: 'STEP_ABC_1', name: 'Find the Fremen', depth: 0, status: 'Complete', complete: true, actionable: true }], contract: [], codex: [], tutorial: [] } } });
  await goTo(ui, '[data-tab="player"]', '[data-sub="player:journey"]'); // the Journey list is loaded when the tab is opened
  const r = ui.w.eval(`[queueLine('journey STEP_ABC_1 completed (3 rows)'), queueLine('journey STEP_UNKNOWN reset (1 row)'), queueLine('moved vehicle 4711 next to player'),
     queueTip('journey STEP_ABC_1 completed (3 rows)'), queueTip('moved vehicle 4711 next to player'), queueTip('solari +5')]`);
  assert.deepEqual([...r], ['Journey step Find the Fremen: completed', 'Journey step: reset', 'Move a vehicle next to the character', 'Journey step id STEP_ABC_1', 'Vehicle number 4711', '']);
  ui.dom.window.close();
});
