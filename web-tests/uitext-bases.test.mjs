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
  moves(main, ['Fills Water Cisterns, Windtraps and Blood Purifiers to capacity', 'Fills the blood sacks you carry', 'the jobs ticked above run as one edit', 'The previous file is backed up first']);
  const shown = visibleText(main);
  for (const s of ['When the editor opens: on', 'Turn off', 'Base water', 'Fill blood sacks', 'Not available yet: arrives with the blood sack feature.', 'Last automatic save: 2026-10-23 09:00: saved 1 Water Cistern', 'The last pass failed.']) assert.ok(shown.includes(s), 'missing: ' + s);
  assert.ok(ui.doc.querySelector('details[data-about="bases.autorefill"]').textContent.includes('The same card is on the Power and Water tabs'));
  ui.dom.window.close();
});

test('Automatic refill card: the waiting line is drawn only while the master switch is on and the pass is waiting', async () => {
  let ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs() } });
  await bases(ui);
  assert.equal(ui.doc.querySelector('#autowait'), null, 'nothing is blocking: no waiting line');
  ui.dom.window.close();

  const blocked = { path: 'C:\\x\\game.db', pending: [], dirty: false, saveBlocked: true, blockReason: 'the game wrote its database 20 seconds ago and does that every 60 seconds while a world is loaded; it must be quiet for 3 minutes first (about 160 seconds more); edits stay queued until this clears, or the game closes', changedOnDisk: false };
  ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs({ waiting: true, waitingWhy: 'the game wrote its database 20 seconds ago' }), '/api/save/state': blocked } });
  await bases(ui);
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('Waiting: the game wrote its database 20 seconds ago. It runs by itself as soon as that clears.'), 'the pass is waiting, and says for what');
  ui.dom.window.close();

  ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs({ autoRefillOnOpen: false, waiting: true }), '/api/save/state': blocked } });
  await bases(ui);
  assert.equal(ui.doc.querySelector('#autowait'), null, 'switched off: nothing to wait for');
  ui.dom.window.close();

  // a pass asked for by hand (#1135) waits with the master switch off too, and the card says so
  ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs({ autoRefillOnOpen: false, waiting: true, waitingByHand: true, waitingWhy: 'the game wrote its database 20 seconds ago' }), '/api/save/state': blocked } });
  await bases(ui);
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('Waiting: the game wrote its database 20 seconds ago. It runs by itself as soon as that clears.'), 'a pass asked for by hand waits with the master switch off');
  ui.dom.window.close();

  ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs({ autoRefillOnOpen: false, waiting: true, waitingByHand: true, copy: true }), '/api/save/state': blocked } });
  await bases(ui);
  assert.equal(ui.doc.querySelector('#autowait'), null, 'a copy never waits');
  ui.dom.window.close();
});

test('Automatic refill card: the button title, the About text and the turn-on question say that a blocked pass waits for the save, not for the game to be closed (#1135, #499)', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs() } });
  await bases(ui);
  const now = ui.doc.querySelector('[data-act="autoNow"]').getAttribute('title');
  assert.ok(now.includes('queued') && now.includes('runs by itself once the save is free'), now);
  assert.ok(!now.includes('game is closed'), 'the pass does not wait for the game to close');
  const about = ui.doc.querySelector('details[data-about="bases.autorefill"]').textContent;
  assert.ok(about.includes('runs by itself once the save is free') && about.includes('it runs with the game still open'), 'the About text');
  assert.ok(!about.includes('game is closed'), 'the pass does not wait for the game to close');
  ui.dom.window.close();
  const off = await boot({ get: { '/api/bases/list': BASE7, '/api/settings': prefs({ autoRefillOnOpen: false }) } });
  await bases(off);
  off.click('[data-act="autoref"]'); await off.settle();
  const ask = off.doc.querySelector('#ask').textContent;
  assert.ok(ask.includes('with no world loaded, it runs with the game still open') && !ask.includes('game has closed'), ask);
  off.dom.window.close();
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
  moves(main, ['Raises the pieces and placeables of this base to the highest health seen for their type on standing pieces of your bases', 'Raises every building piece and placeable in the save, at every base and at no base', 'Pieces that no base owns are listed apart']);
  const shown = visibleText(main);
  assert.ok(shown.includes('Repair this base to max') && shown.includes('Clear sand on this base'), 'the scope of the open base is in the labels');
  assert.ok(shown.includes('Whole save') && shown.includes('Repair every base to max') && shown.includes('Clear sand on every base'), 'the whole-save controls are apart, in their own card, with their own labels');
  assert.ok(!shown.includes('These two act on all of your bases'));
  ui.dom.window.close();
});

test('Structures: both repair tips and the About text say that placeable holograms are left as they are (#812)', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7 } });
  const main = await bases(ui, 'structures');
  const tips = [...main.querySelectorAll('.tipbox')].map((t) => t.textContent.replace(/\s+/g, ' ').trim());
  const repair = tips.filter((t) => t.startsWith('Raises'));
  assert.equal(repair.length, 2, 'the two repair tips: ' + tips.join(' | '));
  for (const t of repair) assert.ok(t.includes('Placeable holograms (unfinished placements) are left as they are'), t);
  const named = repair.filter((t) => t.includes('a base backup'));
  assert.ok(named.length === 1 && named[0].includes('every building piece'), 'only the save-wide tip names base backups (#815): ' + repair.join(' | '));
  const about = [...main.querySelectorAll('details.about')].map((d) => d.textContent.replace(/\s+/g, ' ')).join(' ');
  assert.ok(about.includes('both repairs leave placeable holograms (unfinished placements) as they are'), about);
  ui.dom.window.close();
});

const STAKING = 'Bypasses the game\'s staking checks. Building there is not guaranteed.';
const GROW = [{ ...CLAIM[0], level: 1, rings: 0 }];

test('Resize by square says on the card and in its question that the staking checks are bypassed (#1238)', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': GROW } });
  const main = await bases(ui, 'claim');
  assert.ok(visibleText(main).includes(STAKING), 'on the card, outside About this');
  const level = ui.doc.querySelector('#cv7');
  level.value = '2';
  ui.click('[data-act="claimGrow"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes(STAKING), ui.doc.querySelector('#ask').textContent);
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(ui.posted.length, 0);
});

test('a vertical level change alone shows the staking warning, and the Apply question says it (#1238)', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': CLAIM } });
  await bases(ui, 'claim');
  assert.equal(ui.doc.querySelector('#lcbypass'), null, 'nothing pending: no warning');
  const sel = ui.doc.querySelector('#lcLvl');
  sel.value = '3'; sel.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  const warn = ui.doc.querySelector('#lcbypass');
  assert.ok(warn && warn.textContent.includes('normal staking checks'), 'the warning is drawn for a level change');
  ui.click('[data-act="lcApply"]'); await ui.settle();
  const q = ui.doc.querySelector('#ask').textContent;
  assert.ok(q.includes('Raise the vertical level from 1 to 3.') && q.includes(STAKING), q);
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(ui.posted.length, 0);
});

test('a vertical level change alone names no cells, in the Apply question and in its result (#1240)', async () => {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': CLAIM }, post: { '/api/bases/claim/apply': { ok: true, added: 0, level: 3, levelRaised: true } } });
  await bases(ui, 'claim');
  const sel = ui.doc.querySelector('#lcLvl');
  sel.value = '3'; sel.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  ui.click('[data-act="lcApply"]'); await ui.settle();
  const q = ui.doc.querySelector('#ask').textContent;
  assert.ok(!q.includes('horizontal') && !q.includes('Add '), q);
  ui.click('[data-act="askOk"]'); await ui.settle();
  assert.deepEqual(ui.posted[0].body, { totem_id: 7, cells: [], level: 3 }, 'the request is as before');
  const toast = ui.doc.querySelector('#toast').textContent;
  assert.ok(toast.includes('Land claim: vertical level raised to 3') && !toast.includes('0 cells'), toast);
});

test('the Apply question with cells chosen, alone and with a level change (#1240)', async () => {
  for (const [level, want] of [[null, 'Add 1 horizontal cell.'], ['3', 'Add 1 horizontal cell and raise the vertical level from 1 to 3.']]) {
    const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': CLAIM } });
    await bases(ui, 'claim');
    const cell = ui.doc.querySelector('[data-cell="-1,0"]');
    cell.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true })); await ui.settle();
    if (level) { const sel = ui.doc.querySelector('#lcLvl'); sel.value = level; sel.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle(); }
    ui.click('[data-act="lcApply"]'); await ui.settle();
    const q = ui.doc.querySelector('#ask').textContent;
    assert.ok(q.includes(want), q);
    ui.click('[data-act="askNo"]'); await ui.settle();
    assert.equal(ui.posted.length, 0);
    ui.dom.window.close();
  }
});

test('the Expand question reads as a sentence for a square, a level and both (#1240)', async () => {
  for (const [rings, level, want] of [['2', '1', 'Adds every missing cell up to a 5 x 5 square around the totem.'], ['0', '3', 'Raises the vertical level to 3.'], ['2', '3', 'Adds every missing cell up to a 5 x 5 square around the totem and raises the vertical level to 3.']]) {
    const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': GROW } });
    await bases(ui, 'claim');
    ui.doc.querySelector('#cr7').value = rings;
    ui.doc.querySelector('#cv7').value = level;
    ui.click('[data-act="claimGrow"]'); await ui.settle();
    const q = ui.doc.querySelector('#ask').textContent;
    assert.ok(q.includes(want), `${want} :: ${q}`);
    ui.click('[data-act="askNo"]'); await ui.settle();
    assert.equal(ui.posted.length, 0);
    ui.dom.window.close();
  }
});
