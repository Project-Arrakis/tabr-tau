// The cargo hold pane of a vehicle shows the room its parts give (#483): a vehicle with no storage part has no Add item row and says so (rows already in it can still be deleted or cleared),
// a vehicle with a hold reads its slots and volume on one line (#1231) and never "of 0", and a give that fits only in part says how much did.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { proseWords, longestBlock } from './uitext.mjs';

afterEach(closeAll);

const item = (id, pos) => ({ id: 9000 + id, template_id: 'DolomiteRock', stack_size: 500, quality_level: 0, position_index: pos });
const veh = (type, cargo) => ({ vehicles: [{ id: 41, name: `BP_${type}_CHOAM`, type, map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [], cargo }], hidden: 0, recovered: [], backups: [] });
const ITEMS = [{ id: 'DolomiteRock', name: 'Carbon Ore', category: 'resources' }];
const FUEL = { total: 1, notFull: 0, unknown: 0, noFuelRecord: 0 };
const NO_HOLD = { inventory_id: 70, slots: 0, volume: 0, used: 0, noHold: true, items: [] };
const SURPLUS = { ...NO_HOLD, used: 10, items: Array.from({ length: 10 }, (_, i) => item(i, i)) };
const BUGGY = { inventory_id: 81, slots: 35, volume: 3500, used: 10, volumeUsed: 700, volumeUnknown: 0, noHold: false, items: Array.from({ length: 10 }, (_, i) => item(i, i)) };

async function open(vehicles, post = {}) {
  const ui = await boot({ get: { '/api/vehicles': vehicles, '/api/vehicles/fuel': FUEL, '/api/catalog/items': ITEMS }, post });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  ui.click('[data-vhopen="41"]'); await ui.settle();
  return ui;
}
const cardText = (ui) => ui.doc.querySelector('#cargoroom').closest('.vpane').textContent.replace(/\s+/g, ' ');

test('a Sandbike with no storage part says so and has no Add item row or Clear button', async () => {
  const ui = await open(veh('Sandbike', NO_HOLD));
  assert.equal(ui.doc.querySelector('#cargoroom').textContent, 'This vehicle has no cargo hold (no storage part).');
  assert.equal(ui.doc.querySelector('[data-act="giveCargo"]'), null, 'no Add item');
  assert.equal(ui.doc.querySelector('#bq'), null);
  assert.equal(ui.doc.querySelector('[data-act="clearInv"]'), null, 'nothing to clear');
  assert.ok(!cardText(ui).includes('of 0'), cardText(ui));
  ui.dom.window.close();
});

test('the 10 stacks already in a hold with no slots are listed with N items in a hold with no slots, and Delete and Clear this container stay', async () => {
  const ui = await open(veh('Sandbike', SURPLUS));
  const t = cardText(ui);
  assert.ok(t.includes('This vehicle has no cargo hold (no storage part).') && t.includes('10 items in a hold with no slots.'), t);
  assert.ok(!t.includes('of 0'), t);
  assert.equal(ui.doc.querySelector('[data-act="giveCargo"]'), null, 'no Add item');
  assert.equal(ui.doc.querySelectorAll('.card [data-act="delItem"]').length, 10, 'a Delete for each surplus row');
  const clear = ui.doc.querySelector('[data-act="clearInv"]');
  assert.ok(clear && !clear.disabled && clear.dataset.inv === '70' && clear.dataset.all === '10');
  ui.dom.window.close();
});

test('a vehicle with a hold reads its slots and volume on one line and offers Add item for its inventory', async () => {
  const ui = await open(veh('Buggy', BUGGY), { '/api/bases/give': { ok: true, requested: 5000, given: 5000, reason: '', limit: '', volumeChecked: true, volumeNote: '' } });
  assert.equal(ui.doc.querySelector('#cargoroom').textContent, '10 of 35 slots, 700 of 3,500 volume used');
  assert.equal(ui.doc.querySelector('#cargoroom .vtight'), null, 'nothing is bold under 80 %');
  assert.ok(!cardText(ui).includes('of 0'));
  assert.equal(ui.doc.querySelector('[data-act="giveCargo"]').dataset.id, '81');
  ui.dom.window.close();
});

test('a give that fits only in part says how much, and a refusal from the server is shown as the error it is', async () => {
  let ui = await open(veh('Buggy', BUGGY), { '/api/bases/give': { ok: true, requested: 20000, given: 12500, reason: 'the container is full (volume)', limit: 'volume', volumeChecked: true, volumeNote: '' } });
  ui.doc.querySelector('#bt').value = 'Carbon Ore';
  ui.doc.querySelector('#bq').value = '20000';
  ui.click('[data-act="giveCargo"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/bases/give').map((p) => p.body), [{ inventory_id: 81, template_id: 'DolomiteRock', quantity: 20000 }]);
  const t = ui.doc.querySelector('#toast .part');
  assert.ok(t && t.textContent.includes('Gave 12,500 of 20,000 Carbon Ore: the container is full (volume).'), t && t.textContent);
  ui.dom.window.close();

  // a page that was drawn before the parts changed: the server still refuses
  ui = await open(veh('Buggy', BUGGY), { '/api/bases/give': { __fail: 'This vehicle has no cargo hold (no storage part).', __code: 'no_room', __status: 409 } });
  ui.doc.querySelector('#bt').value = 'Carbon Ore';
  ui.click('[data-act="giveCargo"]'); await ui.settle();
  const err = ui.doc.querySelector('#toast .err');
  assert.ok(err && err.textContent.includes('This vehicle has no cargo hold (no storage part).'), err && err.textContent);
  ui.dom.window.close();
});

test('a hold with no recorded limit shows the item count, as before', async () => {
  const ui = await open(veh('Sandbike', { inventory_id: 70, slots: null, volume: null, used: 1, noHold: false, items: [item(1, 0)] }));
  assert.equal(ui.doc.querySelector('#cargoroom').textContent, '1 item');
  ui.dom.window.close();
});

// The line (#1231): [slots used, slot limit, volume used, volume limit, units of unknown volume] -> the words, and which parts are bold ('+') or full ('!').
const LINES = [
  ['under 80 % of both: nothing is bold', 27, 35, 2799.9, 3500, 0, '27 of 35 slots, 2,799.9 of 3,500 volume used', []],
  ['80 % of the slots exactly', 28, 35, 640, 3500, 0, '28 of 35 slots (nearly full), 640 of 3,500 volume used', ['+28 of 35 slots (nearly full)']],
  ['80 % of the volume exactly', 5, 35, 2800, 3500, 0, '5 of 35 slots, 2,800 of 3,500 volume (nearly full) used', ['+2,800 of 3,500 volume (nearly full)']],
  ['both 80 % or more', 17, 20, 1800, 2000, 0, '17 of 20 slots (nearly full), 1,800 of 2,000 volume (nearly full) used', ['+17 of 20 slots (nearly full)', '+1,800 of 2,000 volume (nearly full)']],
  ['full by volume', 62, 150, 15000, 15000, 0, '62 of 150 slots, 15,000 of 15,000 volume (full)', ['!15,000 of 15,000 volume (full)']],
  ['full by a volume that is a hair under as a sum of floats', 1, 150, 14999.999999999998, 15000, 0, '1 of 150 slots, 15,000 of 15,000 volume (full)', ['!15,000 of 15,000 volume (full)']],
  ['a hair under full is not said to be full', 1, 150, 14999.97, 15000, 0, '1 of 150 slots, 14,999.9 of 15,000 volume (nearly full) used', ['+14,999.9 of 15,000 volume (nearly full)']],
  ['every slot used', 35, 35, 1210.4, 3500, 0, '35 of 35 slots (full), 1,210.4 of 3,500 volume', ['!35 of 35 slots (full)']],
  ['both full', 150, 150, 11300, 11250, 0, '150 of 150 slots (full), 11,300 of 11,250 volume (full)', ['!150 of 150 slots (full)', '!11,300 of 11,250 volume (full)']],
  ['an empty hold', 0, 20, 0, 2000, 0, '0 of 20 slots, 0 of 2,000 volume used', []],
  ['units of unknown volume', 8, 35, 420, 3500, 4, '8 of 35 slots, at least 420 of 3,500 volume used', []],
  ['no volume limit (0)', 12, 30, 80, 0, 0, '12 of 30 slots used', []],
  ['no volume limit (NULL)', 12, 30, 80, null, 0, '12 of 30 slots used', []],
  ['no volume figure', 12, 30, null, 3500, 0, '12 of 30 slots used', []],
  ['no slot limit', 4, null, 380, 2000, 0, '4 items, 380 of 2,000 volume used', []],
  ['no slot limit, full by volume', 1, null, 2000, 2000, 0, '1 item, 2,000 of 2,000 volume (full)', ['!2,000 of 2,000 volume (full)']],
];

test('the cargo line: bold from 80 % used, (full) in the warning colour, only the limits that are recorded', async () => {
  for (const [what, used, slots, volumeUsed, volume, volumeUnknown, words, marked] of LINES) {
    const items = Array.from({ length: used }, (_, i) => item(i, i));
    const ui = await open(veh('Buggy', { inventory_id: 81, slots, volume, used, volumeUsed, volumeUnknown, noHold: false, items }));
    const line = ui.doc.querySelector('#cargoroom');
    assert.equal(line.textContent, words, what);
    assert.deepEqual([...line.querySelectorAll('.vtight')].map((n) => (n.classList.contains('vfull') ? '!' : '+') + n.textContent), marked, what);
    const note = ui.doc.querySelector('details[data-about="vehicles.cargo"]');
    assert.equal(!!note, volumeUnknown > 0, what + ': "at least" is explained where it is said, and only there');
    ui.dom.window.close();
  }
});

test('the cards stay inside the words-to-read budget of the text sweep', async () => {
  for (const cargo of [NO_HOLD, SURPLUS, BUGGY]) {
    const ui = await open(veh('Sandbike', cargo));
    const main = ui.doc.querySelector('#main');
    assert.ok(proseWords(main) <= 100, `${proseWords(main)} words`);
    assert.ok(longestBlock(main) <= 45, `${longestBlock(main)} words in a block`);
    ui.dom.window.close();
  }
});
