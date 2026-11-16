// The Specialization tab picks a track by its name (#708, slice 1). When every track has a number, the page has no Track numbers section and no box asking for a number, and the
// raw-row form is a dropdown of names that sends the number behind the chosen name. A save whose numbers cannot be told keeps the assignment card as a small fallback.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

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
