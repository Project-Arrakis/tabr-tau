// The text sweep (#426) on Player > Bases: Power, Water, Inventory, Land claim editor, Structures and the Automatic refill and repair card. See uitext.test.mjs for the checks that hold on every page.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { visibleText, movedText } from './uitext.mjs';

test.afterEach(() => closeAll());

const BASE7 = [{ base_id: 7, name: 'Base', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 0, placeables: 0, x: 0, y: 0, z: 0 }];
const CLAIM = [{ totem_id: 7, name: 'Totem', cells: 2, rings: 0, level: 1, irregular: true, maxRings: 5, maxLevel: 5, yaw: 90, segments: [{ x: 1, y: 0 }], grid: [{ x: 0, y: 0, in: true, n: 0 }, { x: 1, y: 0, in: true, n: 3 }], piecesInClaim: 3, piecesOutside: 0 }];
const JOBS = [
  { id: 'water', label: 'Base water', desc: 'Fills Water Cisterns, Windtraps and Blood Purifiers to capacity.', key: 'autoRefillWater', ready: true, reason: '', on: true },
  { id: 'fill-blood', label: 'Fill blood sacks', desc: 'Fills the blood sacks you carry.', key: 'autoFillBloodSacks', ready: false, reason: 'arrives with the blood sack feature', on: true },
];
const prefs = (extra = {}) => ({ autoRefillOnOpen: true, jobs: JOBS, last: '', lastProblem: '', problem: '', ...extra });
const bases = async (ui, tab) => {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  if (tab) { ui.click(`[data-bstab="${tab}"]`); await ui.settle(); }
  return ui.doc.querySelector('#main');
};
const moves = (main, list) => {
  const shown = visibleText(main), moved = movedText(main);
  for (const s of list) { assert.ok(!shown.includes(s), 'still shown: ' + s); assert.ok(moved.includes(s), 'lost, not moved: ' + s); }
};

test('Power and Water: what the refill buttons do is a tooltip; the "every base" scope stays in the button\'s own words', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs() } });
  let main = await bases(ui);
  moves(main, ['Tops up Fuel-Powered and Spice-Powered Generators (499)', 'Buildings that are only placeholders are left alone']);
  assert.ok(visibleText(main).includes('Refill power (every base)'));
  assert.equal(ui.doc.querySelector('[data-act="refillGen"]').dataset.confirm, undefined, 'a top-up does not ask first (#450)');
  main = await bases(ui, 'water');
  moves(main, ['Fills every Water Cistern, Medium Water Cistern', 'The blood in a Blood Purifier is not touched']);
  assert.ok(visibleText(main).includes('Refill base water (every base)'));
  assert.equal(ui.doc.querySelector('[data-act="refillWater"]').dataset.confirm, undefined, 'a top-up does not ask first (#450)');
  ui.dom.window.close();
});

test('Automatic refill card: the job descriptions are tooltips, the reason a job is off, the last result and the problems stay, the long paragraph is in About this', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs({ last: '2026-10-23 09:00: saved 1 Water Cistern', lastProblem: 'The last pass failed.' }) } });
  const main = await bases(ui);
  moves(main, ['Fills Water Cisterns, Windtraps and Blood Purifiers to capacity', 'Fills the blood sacks you carry', 'opening the editor runs the jobs ticked above as one edit', 'The previous file is backed up first']);
  const shown = visibleText(main);
  for (const s of ['When the editor opens: on', 'Turn off', 'Base water', 'Fill blood sacks', 'Not available yet: arrives with the blood sack feature.', 'Last automatic save: 2026-10-23 09:00: saved 1 Water Cistern', 'The last pass failed.']) assert.ok(shown.includes(s), 'missing: ' + s);
  assert.ok(ui.doc.querySelector('details[data-about="bases.autorefill"]').textContent.includes('The same card is on the Power and Water tabs'));
  ui.dom.window.close();
});

test('Automatic refill card: the waiting line is drawn only while the master switch is on and the pass is waiting for the game to close', async () => {
  let ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs() } });
  await bases(ui);
  assert.equal(ui.doc.querySelector('#autowait'), null, 'nothing is blocking: no waiting line');
  ui.dom.window.close();

  const blocked = { path: 'C:\\x\\game.db', pending: [], dirty: false, saveBlocked: true, blockReason: 'the game wrote its database 20 seconds ago and does that every 60 seconds while a world is loaded; it must be quiet for 3 minutes first (about 160 seconds more); edits stay queued until this clears, or the game closes', changedOnDisk: false };
  ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs({ waiting: true }), '/api/save/state': blocked } });
  await bases(ui);
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('Waiting: the game is open, and this pass never writes while it is'), 'the pass is waiting');
  ui.dom.window.close();

  ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs({ autoRefillOnOpen: false, waiting: true }), '/api/save/state': blocked } });
  await bases(ui);
  assert.equal(ui.doc.querySelector('#autowait'), null, 'switched off: nothing to wait for');
  ui.dom.window.close();
});

test('Land claim editor: the how-to and the orientation are in About this; the bypass warning is drawn only while cells are chosen; the legend and the pending state stay', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': CLAIM } });
  let main = await bases(ui, 'claim');
  moves(main, ['Click a dotted cell to add it', 'North is up. Column +x runs', 'World-oriented view', 'Click an existing cell to mark it', 'The edit is written at once, after the usual backup']);
  const shown = visibleText(main);
  for (const s of ['Totem', 'Existing', 'New', 'Available', 'To remove', 'Pending horizontal', 'Pending vertical', 'Marked for removal', '1 / 5']) assert.ok(shown.includes(s), 'missing: ' + s);
  assert.equal(ui.doc.querySelector('#lcbypass'), null, 'no cell chosen: no warning');
  assert.ok(ui.doc.getElementById('lcleveltip').textContent.includes('The game caps it at level 5'));
  ui.doc.querySelector('[data-cell="2,0"]').focus();
  ui.doc.querySelector('[data-cell="2,0"]').dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })); await ui.settle();
  main = ui.doc.querySelector('#main');
  assert.ok(ui.doc.querySelector('#lcbypass'), 'a cell is chosen: the warning appears next to Apply');
  assert.ok(visibleText(main).includes('This editor bypasses the game\'s normal staking checks'));
  assert.ok(ui.doc.querySelector('details[data-about="bases.claim"]').textContent.includes('bypasses the game\'s normal staking checks'), 'and it is always in About this');
  ui.dom.window.close();
});

test('Resize by square, Inventory and Structures: limits are tooltips or in About this, the counts and scope stay', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': CLAIM } });
  let main = await bases(ui, 'claim');
  moves(main, ['Cells that hold building pieces are never removed', 'One cell is 10 x 10 foundations']);
  assert.ok(visibleText(main).includes('3 pieces inside the claim, 0 outside.'));
  main = await bases(ui, 'inventory');
  moves(main, ['Only containers that belong to this base are listed']);
  assert.ok(!visibleText(main).includes('Slots used is the items held out of'), 'the column says 5 of 20');
  assert.ok(!movedText(main).includes('Slots used is the items held out of'));
  main = await bases(ui, 'structures');
  moves(main, ['Repair sets each piece to the highest health seen for its type']);
  const shown = visibleText(main);
  assert.ok(shown.includes('Repair every base to max') && shown.includes('Clear sand on every base'), 'the scope is in the labels');
  assert.ok(!shown.includes('These two act on all of your bases'));
  ui.dom.window.close();
});
