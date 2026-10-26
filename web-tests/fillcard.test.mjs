// The Fill card on Player > Character (#415): Fill hydration, Fill blood sacks, Refill Literjons and Fill all liquids moved here from Player > Inventory. The data they need
// (the sacks-not-full count, the liquids note) is read when the Character tab is drawn, and the tab still draws without a hydration record, without sacks or when a count fails.
// Inventory keeps the item table, its Liquid column and Give item, and says where the fill actions went.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const INV = { items: [], inventories: [{ id: 1, inventory_type: 0, max_item_count: 5, max_item_volume: 1000 }], templates: [] };
const LIQ = { water: { total: 3, notFull: 2, unknown: 0 }, bloodSacks: { total: 4, notFull: 1, unknown: 0, holdingOther: 0 }, vehicles: { total: 5, notFull: 3, unknown: 0, noFuelRecord: 0 } };
const SUMMARY = (vitals) => ({ name: 'Tester', status: 'Alive', map: 'Survival_1', faction: '', level: 3, xp: 100, skillPoints: null, intel: null, vitals, identity: {}, ids: {}, currency: [] });
const ACTS = ['fillWater', 'fillBlood', 'refill', 'fillLiquids'];
const open = async (ui) => { ui.click('[data-tab="player"]'); await ui.settle(); };
const card = (ui) => [...ui.doc.querySelectorAll('.card')].find((c) => c.querySelector('h3') && c.querySelector('h3').textContent.trim() === 'Fill');

test('Player > Character shows one Fill card with the four actions and their notes, and the sack count is read when the tab is drawn', async () => {
  const ui = await boot({ get: { '/api/player/blood-sacks': { total: 4, notFull: 3, unknown: 2, holdingOther: 0 }, '/api/player/liquids': LIQ, '/api/player/summary': SUMMARY({ health: 100, healthMinMax: 100, hydration: 52.5, maxHydration: 100, spiceAddiction: 0, maxSpiceAddiction: 10 }) } });
  await open(ui);
  assert.ok(ui.fetched.includes('/api/player/blood-sacks') && ui.fetched.includes('/api/player/liquids'), 'both counts are read without opening Inventory');
  assert.ok(!ui.fetched.includes('/api/player/inventory'), 'Inventory was not opened');
  const c = card(ui);
  assert.ok(c, 'the Fill card is on the Character tab');
  assert.deepEqual([...c.querySelectorAll('[data-act]')].map((b) => b.dataset.act), ACTS);
  for (const a of ACTS) assert.equal(ui.doc.querySelectorAll(`[data-act="${a}"]`).length, 1, a + ' appears once');
  assert.deepEqual([...c.querySelectorAll('button')].map((b) => b.textContent.trim()), ['Fill hydration', 'Fill blood sacks', 'Refill Literjons', 'Fill all liquids']);
  assert.equal(ui.doc.querySelector('#hydnote'), null, 'the hydration number is in the summary tile above, so the Fill card does not repeat it');
  const blood = ui.doc.querySelector('#bloodnote').textContent;
  assert.ok(blood.includes('3 of 4 blood sacks are not full') && blood.includes('2 blood sacks of an unknown kind will be left alone'), blood);
  assert.equal(ui.doc.querySelector('[data-act="fillBlood"]').disabled, false);
  assert.ok(ui.doc.querySelector('#liqnote').textContent.includes('Not full: 2 water containers, 1 blood sack, 3 vehicles.'));
  assert.equal(ui.doc.querySelector('#liqVeh').checked, true, 'the vehicles box is on by default');
  assert.ok(ui.doc.getElementById('hydtip').textContent.includes('will not help a thirsty character right now'), 'the tooltip of Fill hydration says what a queued fill does');
  ui.dom.window.close();
});

test('Fill card: with every sack full Fill blood sacks is switched off, the others stay usable', async () => {
  const ui = await boot({ get: { '/api/player/blood-sacks': { total: 2, notFull: 0, unknown: 0, holdingOther: 0 }, '/api/player/liquids': LIQ } });
  await open(ui);
  assert.equal(ui.doc.querySelector('[data-act="fillBlood"]').disabled, true);
  assert.ok(ui.doc.querySelector('#bloodnote').textContent.includes('0 of 2 blood sacks are not full'));
  for (const a of ['fillWater', 'refill', 'fillLiquids']) assert.equal(ui.doc.querySelector(`[data-act="${a}"]`).disabled, false, a);
  ui.dom.window.close();
});

test('Fill card: a save with no sacks and no hydration record, and counts that cannot be read, still draws the Character tab', async () => {
  const none = { total: 0, notFull: 0, unknown: 0 };
  let ui = await boot({ get: { '/api/player/blood-sacks': { ...none, holdingOther: 0 }, '/api/player/liquids': { water: none, bloodSacks: none, vehicles: none },
    '/api/player/summary': SUMMARY({ health: 100, healthMinMax: 100, hydration: null, maxHydration: 100, spiceAddiction: null, maxSpiceAddiction: 10 }) } });
  await open(ui);
  assert.ok(card(ui), 'the card draws');
  assert.ok(ui.doc.querySelector('#bloodnote').textContent.includes('No blood sacks in the backpack, worn gear or loadout.'));
  assert.equal(ui.doc.querySelector('[data-act="fillBlood"]').disabled, true);
  assert.ok(ui.doc.querySelector('#hydnote').textContent.includes('no hydration'), ui.doc.querySelector('#hydnote').textContent);
  assert.ok(ui.doc.querySelector('[data-act="solari"]'), 'the rest of the Character tab drew too');
  ui.dom.window.close();

  ui = await boot({ get: { '/api/player/blood-sacks': { __fail: 'no', __status: 500 }, '/api/player/liquids': { __fail: 'no', __status: 500 }, '/api/player/summary': { __fail: 'no', __status: 500 } } });
  await open(ui);
  assert.ok(card(ui) && ui.doc.querySelector('[data-act="solari"]'), 'the tab and the card draw');
  assert.ok(ui.doc.querySelector('#bloodnote').textContent.includes('could not be counted'));
  assert.ok(ui.doc.querySelector('#liqnote').textContent.includes('could not be counted'));
  assert.equal(ui.doc.querySelector('[data-act="fillBlood"]').disabled, false);
  ui.dom.window.close();
});

test('Fill card: Fill hydration queued while the game runs is a warning that stays, and written at once is the short line', async () => {
  let ui = await boot({ post: { '/api/player/hydration': { ok: true, before: 0, after: 100, outcome: 'queued' } } });
  await open(ui);
  ui.click('[data-act="fillWater"]'); await ui.settle();
  const warn = ui.doc.querySelector('#toast .part');
  assert.ok(warn, 'a warning: ' + ui.doc.querySelector('#toast').innerHTML);
  assert.ok(warn.textContent.includes('queued, not applied') && warn.textContent.includes('will not help right now'), warn.textContent);
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/hydration').length, 1);
  assert.ok(card(ui), 'the card is still drawn after the edit');
  ui.dom.window.close();

  ui = await boot({ post: { '/api/player/hydration': { ok: true, before: 12.34, after: 100, outcome: 'written' } } });
  await open(ui);
  ui.click('[data-act="fillWater"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#toast .part'), null);
  assert.equal(ui.doc.querySelector('#toast').textContent.trim(), 'Hydration 12.3 → 100');
  ui.dom.window.close();
});

test('Fill card: Refill Literjons posts once and reports', async () => {
  const ui = await boot({ post: { '/api/player/refill': { ok: true, filled: 3, alreadyFull: 1, skippedUnknown: [] } } });
  await open(ui);
  ui.click('[data-act="refill"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Filled 3 Literjons (1 already full)'));
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/refill').length, 1);
  ui.dom.window.close();
});

test('Inventory keeps the item table, its Liquid column and Give item, drops the fill actions and points to the Character tab', async () => {
  const item = { id: 1, template_id: 'Literjon', stack_size: 1, quality_level: 1, position_index: 1, inventory_id: 1, inventory_type: 0, inventory_name: 'Backpack', durability: null, max_durability: null, augments: [], aug_limit: 0, fill: { kind: 'Water', amount: 500, state: 'partial', capacity: 1000, percent: 50 } };
  const ui = await boot({ get: { '/api/player/inventory': { ...INV, items: [item] } } });
  await open(ui);
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  const heads = [...ui.doc.querySelectorAll('table thead th')].map((th) => th.textContent.trim());
  assert.ok(heads.includes('Liquid'), heads.join());
  assert.ok(ui.doc.querySelector('[data-act="give"]') && ui.doc.querySelector('[data-act="queueAdd"]'), 'Give item stays');
  assert.ok(ui.doc.querySelector('[data-act="repair"]'), 'the other cards stay');
  for (const a of ACTS) assert.equal(ui.doc.querySelector(`[data-act="${a}"]`), null, a + ' is not on Inventory');
  assert.equal(ui.doc.querySelector('#liqVeh'), null);
  assert.equal(ui.doc.querySelector('#fillmoved'), null, 'the permanent pointer line is gone (#426); the way back is in the About this of the card');
  const after = ui.fetched.slice(ui.fetched.lastIndexOf('/api/player/inventory'));
  assert.ok(!after.includes('/api/player/blood-sacks') && !after.includes('/api/player/liquids'), 'Inventory does not read the fill counts');
  ui.click('details[data-about="inventory.card"] [data-goto]'); await ui.settle();
  assert.ok(card(ui), 'the button switches to the Character tab, where the Fill card is');
  ui.dom.window.close();
});
