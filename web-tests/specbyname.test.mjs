// The Specialization tab picks a track by its name (#708, slice 1). When every track has a number, the page has no Track numbers section and no box asking for a number, and the
// raw-row form is a dropdown of names that sends the number behind the chosen name. A save whose numbers cannot be told keeps the assignment card as a small fallback.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { root } from './lcc-load.mjs';

afterEach(closeAll);

const TRACKS = ['Combat', 'Crafting', 'Exploration', 'Gathering', 'Sabotage'];
const NUMBERS = { Crafting: 1, Gathering: 2, Exploration: 3, Combat: 4, Sabotage: 5 };
const specs = (known = TRACKS) => ({
  rows: TRACKS.map((track) => ({ track, number: known.includes(track) ? NUMBERS[track] : null, xp: known.includes(track) ? 500 : 0, level: 3, keystoneOwned: 0, keystoneTotal: 2, granted: false })),
  unassigned: [], maxXp: 44182, tracks: TRACKS, resetCount: known.length,
});
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);
async function open(ui) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
}

test('all five numbers known: no Track numbers section, no box asking for a number, no note about missing numbers', async () => {
  const ui = await boot({ get: { '/api/player/specs': specs() } });
  await open(ui);
  const page = ui.doc.querySelector('#main');
  assert.ok(page.textContent.includes('Specialization tracks'), 'the tab itself is drawn');
  const heads = [...page.querySelectorAll('h3')].map((h) => h.textContent.trim());
  assert.ok(!heads.includes('Track numbers'), 'no Track numbers heading: ' + heads);
  assert.equal(page.querySelector('#spmt'), null);
  assert.equal(page.querySelector('#spmn'), null);
  assert.equal(page.querySelector('[data-act="spAssignHand"]'), null);
  assert.equal(page.querySelector('#spnone'), null);
  assert.ok(!page.textContent.includes("Known a track's number already?"));
  assert.ok(!page.textContent.includes('Assigned:'));
  assert.equal(page.querySelector('[data-about="specialization.numbers"]'), null, 'its About this goes with it');
  assert.equal(page.querySelectorAll('[data-act="spAdd"]:not([disabled])').length, 5, 'every track takes XP');
});

test('all five numbers known: the raw row asks for a specialization by name and sends the number behind it', async () => {
  const ui = await boot({ get: { '/api/player/specs': specs() }, post: { '/api/player/specs': { ok: true } } });
  await open(ui);
  const pick = ui.doc.querySelector('#st');
  assert.equal(pick.tagName, 'SELECT', 'a dropdown, not a number box');
  assert.deepEqual([...pick.options].map((o) => o.textContent.trim()), TRACKS);
  assert.ok(!ui.doc.querySelector('details').textContent.includes('Track number'), 'the form does not talk about numbers');
  pick.value = String(NUMBERS.Sabotage);
  ui.doc.querySelector('#sx').value = '500'; ui.doc.querySelector('#sl').value = '2';
  ui.click('[data-act="specSet"]'); await ui.settle();
  assert.equal(posts(ui, '/api/player/specs').length, 0, 'nothing is sent before the answer');
  const ask = ui.doc.querySelector('#ask').textContent;
  assert.ok(ask.includes('Sabotage is set to 500 XP at level 2') && !ask.includes('Track number'), ask);
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/player/specs').map((p) => p.body), [{ track_type: 5, xp: 500, level: 2 }]);
  // each name sends its own number
  for (const [name, n] of Object.entries(NUMBERS)) {
    const opt = [...pick.options].find((o) => o.textContent.trim() === name);
    assert.equal(opt.value, String(n), name);
  }
});

test('a save whose numbers cannot be told keeps the assignment card, and the raw row lists only the tracks with a number', async () => {
  const ui = await boot({ get: { '/api/player/specs': specs(['Combat', 'Crafting']) } });
  await open(ui);
  const page = ui.doc.querySelector('#main');
  const heads = [...page.querySelectorAll('h3')].map((h) => h.textContent.trim());
  assert.ok(heads.includes('Track numbers'), 'the fallback is drawn: ' + heads);
  assert.ok(page.querySelector('#spmt') && page.querySelector('#spmn') && page.querySelector('[data-act="spAssignHand"]'));
  assert.ok(page.textContent.includes("Known a track's number already?"));
  assert.deepEqual([...ui.doc.querySelector('#st').options].map((o) => o.textContent.trim()), ['Combat', 'Crafting']);
});

test('no track has a number: the dead-end note and the assignment card are drawn, and the raw row has nothing to pick', async () => {
  const ui = await boot({ get: { '/api/player/specs': specs([]) } });
  await open(ui);
  const page = ui.doc.querySelector('#main');
  assert.ok(page.querySelector('#spnone'));
  assert.ok([...page.querySelectorAll('h3')].some((h) => h.textContent.trim() === 'Track numbers'));
  assert.equal(page.querySelector('#st'), null);
  assert.equal(page.querySelector('[data-act="specSet"]'), null);
});

// ---- #853: a number assigned by hand that is not the game's own must stay correctable ----
const withNumbers = (nums, unassigned = []) => ({ ...specs(), rows: TRACKS.map((track) => ({ track, number: nums[track], xp: 500, level: 3, keystoneOwned: 0, keystoneTotal: 2, granted: false })), unassigned });

test('every track has a number and every number is the game\'s own: no Track numbers card, even when the save holds a row no track has', async () => {
  const ui = await boot({ get: { '/api/player/specs': withNumbers(NUMBERS, [{ number: 9, xp: 750, level: 6 }]) } });
  await open(ui);
  const page = ui.doc.querySelector('#main');
  assert.ok(![...page.querySelectorAll('h3')].some((h) => h.textContent.trim() === 'Track numbers'));
  assert.equal(page.querySelector('#spmt'), null);
});

test('a number assigned by hand that is not the game\'s own (Combat = 9) brings the Track numbers card back, with the hand box and the assigned list', async () => {
  const ui = await boot({ get: { '/api/player/specs': withNumbers({ ...NUMBERS, Combat: 9 }, [{ number: 4, xp: 750, level: 6 }]) }, post: { '/api/player/specs/assign': { ok: true } } });
  await open(ui);
  const page = ui.doc.querySelector('#main');
  assert.ok([...page.querySelectorAll('h3')].some((h) => h.textContent.trim() === 'Track numbers'), 'the card is drawn');
  assert.ok(page.textContent.includes('Combat = 9'), 'the list of assigned numbers shows the wrong one');
  ui.doc.querySelector('#spmt').value = 'Combat'; ui.doc.querySelector('#spmn').value = '4';
  ui.click('[data-act="spAssignHand"]'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/player/specs/assign').map((p) => p.body), [{ track: 'Combat', number: 4 }], 'Combat can be set back to its own number');
});

test('a full swap of two numbers (Combat = 1, Crafting = 4) shows the card too', async () => {
  const ui = await boot({ get: { '/api/player/specs': withNumbers({ ...NUMBERS, Combat: 1, Crafting: 4 }) } });
  await open(ui);
  assert.ok([...ui.doc.querySelectorAll('#main h3')].some((h) => h.textContent.trim() === 'Track numbers'));
});

test('the numbers the page counts as the game\'s own are the measured numbers of the Go code, and the card says how to clear a clash', async () => {
  const go = fs.readFileSync(path.join(root, 'internal', 'ops', 'landsraad_grant.go'), 'utf8');
  const m = /var measuredTrackNumbers = map\[string\]int64\{([^}]*)\}/.exec(go);
  assert.ok(m, 'the table is in landsraad_grant.go');
  const goNums = Object.fromEntries([...m[1].matchAll(/"(\w+)":\s*(\d+)/g)].map((x) => [x[1], Number(x[2])]));
  assert.deepEqual(goNums, NUMBERS);
  const app = fs.readFileSync(path.join(root, 'internal', 'web', 'static', 'app.js'), 'utf8');
  const a = /const MEASURED_TRACK_NUMBERS = \{([^}]*)\}/.exec(app);
  assert.ok(a, 'the page keeps the same table');
  assert.deepEqual(Object.fromEntries([...a[1].matchAll(/(\w+):\s*(\d+)/g)].map((x) => [x[1], Number(x[2])])), goNums);
  const ui = await boot({ get: { '/api/player/specs': withNumbers({ ...NUMBERS, Combat: 9 }) } });
  await open(ui);
  const about = ui.doc.querySelector('[data-about="specialization.numbers"]');
  assert.ok(about, 'the About this of the card is there');
  assert.match(about.textContent, /set that track again to its own number in the box above/);
  assert.match(about.textContent, /Crafting 1, Gathering 2, Exploration 3, Combat 4, Sabotage 5/);
});
