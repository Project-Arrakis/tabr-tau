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

test('the model: taken with no tier rows, taken with a ladder, open (a taken House with none of the character is no case of its own, #1022)', () => {
  const a = synthetic();
  task(a, 'Mutelli').rewards = [];
  const b = M.readBoard(a);
  const h = (n) => b.houses.find((x) => x.name === n);
  assert.equal(h('Mutelli').owner, 'a');
  assert.equal(h('Mutelli').takenNoTiers, true);
  assert.equal(h('Vernius').takenNoTiers, false);
  assert.equal(h('Vernius').tierCount, 5);
  assert.equal(h('Wallach').takenNoTiers, false);
  assert.equal(h('Sor').takenNoTiers, false, 'an open House is not taken');
  assert.equal('takenNoMine' in h('Vernius'), false, 'the flag of the removed case (taken, none of yours) is gone, #1022');
});

test('taken with no tier record in the save: the card says so, not 0/5 and not as if unreached', async () => {
  const a = synthetic();
  task(a, 'Mutelli').rewards = [];
  const ui = await open(a);
  const c = tileOf(ui, 'Mutelli');
  assert.equal(count(ui, 'Mutelli'), 'Favor won - no tier record in the save');
  assert.doesNotMatch(count(ui, 'Mutelli'), /EARNED|\/5|0\/0/i);
  assert.equal(c.querySelectorAll('.lcc-hc-q').length, 0, 'no collected and waiting ticks');
  assert.match(label(ui, 'Mutelli'), /favor won by Atreides, .*favor won, no tier record in the save/);
  assert.doesNotMatch(label(ui, 'Mutelli'), /tiers earned|no tiers in the save/);
  assert.match(c.querySelector('button.lcc-hc-select').getAttribute('data-says'), /favor is won, and the save holds no reward tiers for it/);
  assert.match(c.querySelector('.lcc-hc-count b').getAttribute('title'), /no reward tiers/);
});

test('taken with tiers in the save but none of the character: the card shows what an open House shows, 0/5 EARNED and the check and diamond counts (#1022)', async () => {
  const ui = await open(synthetic());
  assert.equal(count(ui, 'Vernius'), count(ui, 'Sor'), 'the same text as the open House Sor, which has nothing either');
  assert.match(count(ui, 'Vernius'), /^0\/5 EARNED ✓ 0 ◆ 0$/);
  assert.doesNotMatch(count(ui, 'Vernius'), /Favor won|no contribution/i);
  assert.equal(tileOf(ui, 'Vernius').querySelector('.lcc-hc-count [data-taken]'), null);
  assert.equal(tileOf(ui, 'Vernius').querySelectorAll('.lcc-hc-q').length, 2, 'the collected and waiting counts');
  assert.equal(tileOf(ui, 'Vernius').querySelectorAll('.lcc-hc-tiers .lcc-hc-t').length, 5, 'the five tiers of the ladder are still drawn, not reached');
  assert.match(tileOf(ui, 'Vernius').querySelector('.lcc-hc-owner').textContent, /Harkonnen/, 'the faction line stays');
  // still announced as won by the faction, with the plain earned count
  assert.match(label(ui, 'Vernius'), /favor won by Harkonnen, .*0 of 5 tiers earned, 0 collected/);
  assert.doesNotMatch(label(ui, 'Vernius'), /no contribution of yours|none of your/);
  assert.doesNotMatch(tileOf(ui, 'Vernius').querySelector('button.lcc-hc-select').getAttribute('data-says') || '', /no contribution of yours/);
  // the write controls and their attributes are the ones of any other card
  const t = tileOf(ui, 'Vernius').querySelector('.lcc-hc-t');
  assert.equal(t.dataset.lb, 'tier');
});

test('taken with tiers and a contribution of the character, and an open House: unchanged', async () => {
  const ui = await open(synthetic());
  assert.match(count(ui, 'Wallach'), /^3\/5 EARNED/);
  assert.doesNotMatch(count(ui, 'Wallach'), /Favor won/);
  assert.match(label(ui, 'Wallach'), /3 of 5 tiers earned/);
  assert.match(count(ui, 'Sor'), /^0\/5 EARNED/, 'an open House with nothing keeps its count');
  assert.doesNotMatch(count(ui, 'Sor'), /Favor won/);
  assert.match(label(ui, 'Sor'), /favor not won, .*0 of 5 tiers earned/);
});

test('the list and the inspector say the same of a taken House with none of the character as of an open one (#1022)', async () => {
  const ui = await open(synthetic(), 'list');
  const rows = [...ui.doc.querySelectorAll('.lcc-bd-row')];
  const row = (n) => rows.find((r) => text(r.querySelector('.lcc-bd-namebtn')) === n);
  assert.equal(text(row('Vernius').querySelector('.lcc-bd-c-state')), 'Not started');
  assert.equal(text(row('Vernius').querySelector('.lcc-bd-c-tiers')), '0 of 5 tiers earned');
  assert.equal(text(row('Sor').querySelector('.lcc-bd-c-tiers')), '0 of 5 tiers earned');
  assert.equal(text(row('Sor').querySelector('.lcc-bd-c-state')), 'Not started');
  assert.equal(text(row('Wallach').querySelector('.lcc-bd-c-tiers')), '3 of 5 tiers earned');
  click(ui, row('Vernius').querySelector('.lcc-bd-namebtn'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  assert.equal(i.querySelector('.lcc-bd-ledger-n'), null, 'no N OF 5 EARNED tag (#1059)');
  assert.equal(text(i.querySelector('.lcc-bd-stats strong')), '0 / 5', 'the summary tiles of any House');
  assert.equal(i.querySelector('.lcc-bd-taken-note'), null);
  assert.equal(i.querySelectorAll('.lcc-bd-tier').length, 5, 'the ladder rows stay, with their controls');
});

test('a taken House with no tier rows: the inspector says no tier record', async () => {
  const a = synthetic();
  task(a, 'Mutelli').rewards = [];
  const ui = await open(a);
  click(ui, tileOf(ui, 'Mutelli').querySelector('button'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  assert.equal(text(i.querySelector('.lcc-bd-ledger-n')), 'FAVOR WON · NO TIER RECORD');
  assert.match(text(i.querySelector('.lcc-bd-taken-note')), /favor is won, and the save holds no reward tiers for it/);
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
  assert.deepEqual(v.contributors.map((c) => [c.label, c.amount]), [['Fortune Hunters Inc', 2500], ['Red Hawk Corps', 1000], ['Unnamed contributor', 1000]], 'the biggest first (a tie by name), a guild by its name, a contributor with no guild as Unnamed contributor (#919)');
  assert.deepEqual(b.contributors.h.find((c) => c.id === 24 && c.label === 'Fortune Hunters Inc').amount, 2500, 'the faction list sums each contributor over the Houses of the board');
  const ui = await open(a);
  const lists = [...ui.doc.querySelectorAll('.lcc-standard')].map((s) => [...s.querySelectorAll('.lcc-standard-list > li')].map((n) => text(n)));
  assert.equal(ui.doc.querySelectorAll('.lcc-standard-none').length, 0, 'both banners have lists');
  assert.ok(lists.some((l) => l.includes('Fortune Hunters Inc')), 'the Harkonnen banner names a guild, without its amount: ' + JSON.stringify(lists));
  assert.match([...ui.doc.querySelectorAll('.lcc-standard-h')].map((h) => h.getAttribute('title')).join(' | '), /worked out here: the save holds amounts, not a ranking/);
  click(ui, tileOf(ui, 'Vernius').querySelector('button'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  assert.deepEqual([...i.querySelectorAll('.lcc-bd-contrib-l > li')].map(text), ['Fortune Hunters Inc', 'Red Hawk Corps'], 'the Inspector lists the names, the biggest first, with no amount and no Unnamed contributor row (#1058)');
  assert.match(i.querySelector('#lcc-bd-s1').getAttribute('title'), /Top contributors: Fortune Hunters Inc, Red Hawk Corps\./);
});

test('a taken House with no contributor row keeps "Not in the save" and says why; a banner with no rows says so', async () => {
  const a = synthetic();
  for (const t of a.tasks) t.player_contributions = [];
  const ui = await open(a);
  assert.equal(ui.doc.querySelectorAll('.lcc-standard-none').length, 2);
  assert.ok([...ui.doc.querySelectorAll('.lcc-standard-none')].every((n) => text(n) === '—'));
  assert.match([...ui.doc.querySelectorAll('.lcc-standard-h')].map((h) => h.getAttribute('title')).join(' | '), /holds no contribution rows under Atreides/);
  click(ui, tileOf(ui, 'Mutelli').querySelector('button'));
  const n = text(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-nosave'));
  assert.match(n, /^Top contributors: not in the save\. The favor of this House is won but the save holds no contributor row for it/);
  assert.match(ui.doc.querySelector('.lcc-bd-insp #lcc-bd-s1').getAttribute('title'), /Top contributors: Not in the save \(the save has no contributor row for this House, whose favor is won\)\. Missions: Not in the save\./);
  // an open House says no more than before
  click(ui, tileOf(ui, 'Sor').querySelector('button'));
  assert.match(ui.doc.querySelector('.lcc-bd-insp #lcc-bd-s1').getAttribute('title'), /Top contributors: Not in the save\. Missions: Not in the save\./);
  assert.doesNotMatch(text(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-nosave')), /taken/);
});
