// Fill all liquids (#349): the card under Blood sacks on Player > Character > Inventory shows what is not full, has the "also refuel my vehicles" box (on by default), posts one
// request with the box's value, remembers the box after the redraw, and says what each part did.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const INV = { items: [], inventories: [{ id: 1, inventory_type: 0, max_item_count: 5, max_item_volume: 1000 }], templates: [] };
const STATUS = { water: { total: 3, notFull: 2, unknown: 0 }, bloodSacks: { total: 4, notFull: 1, unknown: 1, holdingOther: 0 }, vehicles: { total: 5, notFull: 3, unknown: 0, noFuelRecord: 0 } };
const DONE = { ok: true, vehiclesIncluded: true, water: { ok: true, filled: 2, alreadyFull: 1, skippedUnknown: [] }, bloodSacks: { ok: true, filled: 1, byTemplate: { Bloodsack_01: 1 }, alreadyFull: 0, unknown: 0, skippedUnknown: [] }, vehicles: { ok: true, filled: 3, byClass: {}, alreadyFull: 2, total: 5, unknown: 0, skippedUnknown: [] }, outcome: 'written' };
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);
const boot_ = (get, post) => boot({ get: { '/api/player/inventory': INV, ...get }, post });
async function openInventory(ui) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
}

test('Fill all liquids: the count, the box on by default, one request with the box, and a toast that names each part', async () => {
  const ui = await boot_({ '/api/player/liquids': STATUS }, { '/api/player/fill-all-liquids': DONE });
  await openInventory(ui);
  const btn = ui.doc.querySelector('[data-act="fillLiquids"]');
  assert.ok(btn, 'the Inventory tab has the button');
  assert.equal(btn.textContent.trim(), 'Fill all liquids');
  assert.equal(ui.doc.querySelectorAll('[data-act="fillLiquids"]').length, 1);
  const box = ui.doc.querySelector('#liqVeh');
  assert.ok(box && box.checked, 'the vehicles box is on by default');
  const note = ui.doc.getElementById(btn.getAttribute('aria-describedby')).textContent;
  assert.ok(note.includes('Not full: 2 water containers, 1 blood sack, 3 vehicles.'), note);
  assert.ok(ui.doc.querySelector('[data-act="refill"]') && ui.doc.querySelector('[data-act="fillBlood"]'), 'the single buttons are still there');
  assert.equal(posts(ui, '/api/player/fill-all-liquids').length, 0, 'nothing is sent before the press');

  ui.click('[data-act="fillLiquids"]'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/player/fill-all-liquids').map((p) => p.body), [{ vehicles: true }]);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Filled 2 water containers, 1 blood sack, 3 vehicles.'), t);
  assert.equal(ui.doc.querySelector('#toast .err'), null);
  ui.dom.window.close();
});

test('Fill all liquids: with the box off the request says so, the box stays off after the redraw, and the toast says the vehicles were not refuelled', async () => {
  const ui = await boot_({ '/api/player/liquids': STATUS }, { '/api/player/fill-all-liquids': { ...DONE, vehiclesIncluded: false, vehicles: undefined } });
  await openInventory(ui);
  const box = ui.doc.querySelector('#liqVeh');
  box.checked = false;
  ui.click('[data-act="fillLiquids"]'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/player/fill-all-liquids').map((p) => p.body), [{ vehicles: false }]);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Filled 2 water containers, 1 blood sack.') && t.includes('Vehicles were not refuelled.'), t);
  assert.equal(ui.doc.querySelector('#liqVeh').checked, false, 'the box is as the person left it');
  ui.dom.window.close();
});

test('Fill all liquids: nothing to fill says so, a refusal shows the editor\'s words, and the card draws without a count', async () => {
  let ui = await boot_({ '/api/player/liquids': { __fail: 'no', __status: 500 } }, { '/api/player/fill-all-liquids': { ...DONE, water: { filled: 0, skippedUnknown: [] }, bloodSacks: { filled: 0, unknown: 1 }, vehicles: { filled: 0, unknown: 0 } } });
  await openInventory(ui);
  const btn = ui.doc.querySelector('[data-act="fillLiquids"]');
  assert.ok(btn && !btn.disabled);
  assert.ok(ui.doc.getElementById(btn.getAttribute('aria-describedby')).textContent.includes('could not be counted'));
  ui.click('[data-act="fillLiquids"]'); await ui.settle();
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Nothing needed filling.') && t.includes('of an unknown kind were left alone'), t);
  ui.dom.window.close();

  ui = await boot_({ '/api/player/liquids': STATUS }, { '/api/player/fill-all-liquids': { __fail: 'Edits are held until the queue is looked at.', __code: 'queue_held', __status: 409 } });
  await openInventory(ui);
  ui.click('[data-act="fillLiquids"]'); await ui.settle();
  const err = ui.doc.querySelector('#toast .err');
  assert.ok(err && err.textContent.includes('Edits are held until the queue is looked at.'));
  ui.dom.window.close();
});
