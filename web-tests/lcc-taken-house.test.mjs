// A House the save says is taken must not read as "nothing happened" (#911), and the contributors the save records are shown (#913). The page code is the real app.js with the real lcc scripts,
// over the synthetic answer of M1 (no game save is read). Houses of that answer used here: Vernius (taken by Harkonnen, no row of the character, one guild row), Mutelli (taken by Atreides,
// no row of the character, no contributor row at all), Wallach (taken by Atreides, the character at 3,600) and Sor (open, nothing).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir } from './lcc-load.mjs';

afterEach(closeAll);

const require = createRequire(import.meta.url);
const M = require(path.join(lccDir, 'lcc-model.js'));
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const task = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name);
async function open(answer, layout) {
  const local = { tab: 'landsraad', 'lsb.view': 'board' };
  if (layout) local['lsb.layout'] = layout;
  const ui = await boot({ get: { '/api/landsraad': answer }, local });
  await ui.settle();
  return ui;
}
const tileOf = (ui, name) => [...ui.doc.querySelectorAll('.lcc-bd-cell')].find((c) => c.querySelector('.lcc-hc-name').textContent.toLowerCase() === 'house ' + name.toLowerCase());
const click = (ui, el) => { el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const count = (ui, name) => text(tileOf(ui, name).querySelector('.lcc-hc-count'));
const label = (ui, name) => tileOf(ui, name).querySelector('button.lcc-hc-select').getAttribute('aria-label');

test('the model: taken with no tier rows, taken with a ladder and none of the character, taken with the character, open', () => {
  const a = synthetic();
  task(a, 'Mutelli').rewards = [];
  const b = M.readBoard(a);
  const h = (n) => b.houses.find((x) => x.name === n);
  assert.equal(h('Mutelli').owner, 'a');
  assert.equal(h('Mutelli').takenNoTiers, true);
  assert.equal(h('Mutelli').takenNoMine, false, 'with no tiers it is the no-tier case, not the other');
  assert.equal(h('Vernius').takenNoMine, true);
  assert.equal(h('Vernius').takenNoTiers, false);
  assert.equal(h('Wallach').takenNoMine, false);
  assert.equal(h('Wallach').takenNoTiers, false);
  assert.equal(h('Sor').takenNoMine, false, 'an open House is not taken');
  assert.equal(h('Sor').takenNoTiers, false);
  task(a, 'Wallach').my_recorded = false;
  task(a, 'Wallach').my_contribution = 0;
  assert.equal(M.readBoard(a).houses.find((x) => x.name === 'Wallach').takenNoMine, true, 'no row of the character is no contribution of the character');
});

test('taken with no tier record in the save: the card says so, not 0/5 and not as if unreached', async () => {
  const a = synthetic();
  task(a, 'Mutelli').rewards = [];
  const ui = await open(a);
  const c = tileOf(ui, 'Mutelli');
  assert.equal(count(ui, 'Mutelli'), 'Taken - no tier record in the save');
  assert.doesNotMatch(count(ui, 'Mutelli'), /EARNED|\/5|0\/0/i);
  assert.equal(c.querySelectorAll('.lcc-hc-q').length, 0, 'no collected and waiting ticks');
  assert.match(label(ui, 'Mutelli'), /claimed by Atreides, .*taken, no tier record in the save/);
  assert.doesNotMatch(label(ui, 'Mutelli'), /tiers earned|no tiers in the save/);
  assert.match(c.querySelector('button.lcc-hc-select').getAttribute('title'), /taken, and the save holds no reward tiers for it/);
  assert.match(c.querySelector('.lcc-hc-count b').getAttribute('title'), /no reward tiers/);
});

test('taken with tiers in the save but none of the character: the card says so, not 0/5 EARNED', async () => {
  const ui = await open(synthetic());
  assert.equal(count(ui, 'Vernius'), 'Taken - no contribution of yours');
  assert.doesNotMatch(count(ui, 'Vernius'), /EARNED/);
  assert.equal(tileOf(ui, 'Vernius').querySelectorAll('.lcc-hc-q').length, 0);
  assert.equal(tileOf(ui, 'Vernius').querySelectorAll('.lcc-hc-tiers .lcc-hc-t').length, 5, 'the five tiers of the ladder are still drawn, not reached');
  assert.match(label(ui, 'Vernius'), /claimed by Harkonnen, .*taken, no contribution of yours in the save, so none of your tiers is earned/);
  assert.doesNotMatch(label(ui, 'Vernius'), /0 of 5 tiers earned/);
  assert.match(tileOf(ui, 'Vernius').querySelector('.lcc-hc-count b').getAttribute('title'), /holds no contribution of yours to it/);
  // the write controls and their attributes are the ones of any other card
  const t = tileOf(ui, 'Vernius').querySelector('.lcc-hc-t');
  assert.equal(t.dataset.lb, 'tier');
});

test('taken with tiers and a contribution of the character, and an open House: unchanged', async () => {
  const ui = await open(synthetic());
  assert.match(count(ui, 'Wallach'), /^3\/5 EARNED/);
  assert.doesNotMatch(count(ui, 'Wallach'), /Taken/);
  assert.match(label(ui, 'Wallach'), /3 of 5 tiers earned/);
  assert.match(count(ui, 'Sor'), /^0\/5 EARNED/, 'an open House with nothing keeps its count');
  assert.doesNotMatch(count(ui, 'Sor'), /Taken/);
  assert.match(label(ui, 'Sor'), /open, .*0 of 5 tiers earned/);
});

test('the list and the inspector say the same of a taken House with none of the character', async () => {
  const ui = await open(synthetic(), 'list');
  const rows = [...ui.doc.querySelectorAll('.lcc-bd-row')];
  const row = (n) => rows.find((r) => text(r.querySelector('.lcc-bd-namebtn')) === n);
  assert.equal(text(row('Vernius').querySelector('.lcc-bd-c-state')), 'Taken, none of yours');
  assert.equal(text(row('Vernius').querySelector('.lcc-bd-c-tiers')), 'none of yours');
  assert.equal(text(row('Sor').querySelector('.lcc-bd-c-tiers')), '0 of 5');
  assert.equal(text(row('Sor').querySelector('.lcc-bd-c-state')), 'Not started');
  assert.equal(text(row('Wallach').querySelector('.lcc-bd-c-tiers')), '3 of 5');
  click(ui, row('Vernius').querySelector('.lcc-bd-namebtn'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  assert.equal(text(i.querySelector('.lcc-bd-ledger-n')), 'TAKEN · NO CONTRIBUTION OF YOURS');
  assert.equal(i.querySelector('.lcc-bd-stats'), null, 'no 0/5 summary tiles');
  assert.match(text(i.querySelector('.lcc-bd-taken-note')), /This House is taken\. The save holds no contribution of yours/);
  assert.equal(i.querySelectorAll('.lcc-bd-tier').length, 5, 'the ladder rows stay, with their controls');
});

test('a taken House with no tier rows: the inspector says no tier record', async () => {
  const a = synthetic();
  task(a, 'Mutelli').rewards = [];
  const ui = await open(a);
  click(ui, tileOf(ui, 'Mutelli').querySelector('button'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  assert.equal(text(i.querySelector('.lcc-bd-ledger-n')), 'TAKEN · NO TIER RECORD IN THE SAVE');
  assert.match(text(i.querySelector('.lcc-bd-taken-note')), /taken, and the save holds no reward tiers for it/);
});

test('the contributors the save records are shown: on the banners and for the House, with a guild by its name', async () => {
  const a = synthetic();
  const t = task(a, 'Vernius');
  t.player_contributions = [
    { player_id: 20, faction_id: 2, amount: 1000, mine: false, guild: 'red_hawk_corps' },
    { player_id: 24, faction_id: 2, amount: 2500, mine: false, guild: 'fortune_hunters_inc' },
    { player_id: 28, faction_id: 2, amount: 1000, mine: false },
  ];
  const b = M.readBoard(a);
  const v = b.houses.find((x) => x.name === 'Vernius');
  assert.deepEqual(v.contributors.map((c) => [c.label, c.amount]), [['Fortune Hunters Inc', 2500], ['Contributor 28', 1000], ['Red Hawk Corps', 1000]], 'the biggest first (a tie by name), a guild by its name, any other by its id');
  assert.deepEqual(b.contributors.h.find((c) => c.id === 24 && c.label === 'Fortune Hunters Inc').amount, 2500, 'the faction list sums each contributor over the Houses of the board');
  const ui = await open(a);
  const lists = [...ui.doc.querySelectorAll('.lcc-standard')].map((s) => [...s.querySelectorAll('.lcc-standard-name')].map((n) => text(n)));
  assert.equal(ui.doc.querySelectorAll('.lcc-standard-none').length, 0, 'both banners have lists');
  assert.ok(lists.some((l) => l.includes('Fortune Hunters Inc · 2,500')), 'the Harkonnen banner names a guild and its amount: ' + JSON.stringify(lists));
  assert.match([...ui.doc.querySelectorAll('.lcc-standard-h')].map((h) => h.getAttribute('title')).join(' | '), /worked out here: the save holds amounts, not a ranking/);
  click(ui, tileOf(ui, 'Vernius').querySelector('button'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  assert.match(text(i.querySelector('.lcc-bd-contrib')), /^Contributors to this House, from the save, the biggest first: Fortune Hunters Inc 2,500 · Contributor 28 1,000 · Red Hawk Corps 1,000.$/);
  assert.match(i.querySelector('#lcc-bd-s1').getAttribute('title'), /Top contributors: Fortune Hunters Inc 2,500 · Contributor 28 1,000/);
});

test('a taken House with no contributor row keeps "Not in the save" and says why; a banner with no rows says so', async () => {
  const a = synthetic();
  for (const t of a.tasks) t.player_contributions = [];
  const ui = await open(a);
  assert.equal(ui.doc.querySelectorAll('.lcc-standard-none').length, 2);
  assert.ok([...ui.doc.querySelectorAll('.lcc-standard-none')].every((n) => text(n) === 'Not in the save'));
  assert.match([...ui.doc.querySelectorAll('.lcc-standard-h')].map((h) => h.getAttribute('title')).join(' | '), /holds no contribution rows under Atreides/);
  click(ui, tileOf(ui, 'Mutelli').querySelector('button'));
  const n = text(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-nosave'));
  assert.match(n, /^Top contributors: not in the save\. This House is taken but the save holds no contributor row for it/);
  assert.match(ui.doc.querySelector('.lcc-bd-insp #lcc-bd-s1').getAttribute('title'), /Top contributors: Not in the save \(the save has no contributor row for this taken House\)\. Missions: Not in the save\./);
  // an open House says no more than before
  click(ui, tileOf(ui, 'Sor').querySelector('button'));
  assert.match(ui.doc.querySelector('.lcc-bd-insp #lcc-bd-s1').getAttribute('title'), /Top contributors: Not in the save\. Missions: Not in the save\./);
  assert.doesNotMatch(text(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-nosave')), /taken/);
});
