// The TOP CONTRIBUTORS of the two banners (#972). Three things were looked at, against a made-up answer built like the one read from a real save (the guild names are the save's own
// guild_identifier values, the rows are the rows of landsraad_task_player_contributions of a term):
//   data     the ranking of a faction is the sum of each contributor's rows over all the Houses of the term, the biggest first, equal amounts by name; the character is named by the save ("You" only when no name can be read). It was right, and is
//            checked here against an independent sum of the raw rows.
//   layout   each line showed "name · amount" in one cut-off piece, so the amount was lost behind an ellipsis. A line is now a name and an amount of its own: the name wraps, the amount is whole,
//            and an entry that does not fit in the height of the cloth is not drawn at all (a second column that is out of sight), never cut through.
//   wording  the heading's mouse-over text says how the ranking is made (summed over the term, ties by name, progress without a contributor not named) and gives the three whole.
// The page code is the real app.js with the real lcc scripts (no game save is read).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir, load } from './lcc-load.mjs';

afterEach(closeAll);

const require = createRequire(import.meta.url);
const M = require(path.join(lccDir, 'lcc-model.js'));
const css = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8').replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const ME = 1; const NAME = 'Ahsan Quill'; // the character's own player id and name in the answer

// Atreides is faction 1, Harkonnen 2 (as in the synthetic answer). Four Atreides guilds gave 1,000 to one House each (a four-way tie, as in the operator's save), Bravo Squad gave 500 more to a
// fifth House (so over the term it is ahead); Harkonnen has the character with 33,600 in one House and 3,000 in another, a guild with 31,900 and one with 1,000; a guild that gave to both factions.
const ROWS = [
  [0, 28, 1, 1000, 'bravo_squad'], [1, 32, 1, 1000, 'dune_divers'], [2, 24, 1, 1000, 'fortune_hunters_inc'], [3, 20, 1, 1000, 'red_hawk_corps'], [4, 28, 1, 500, 'bravo_squad'],
  [5, ME, 2, 33600, null], [6, ME, 2, 3000, null], [7, 19, 2, 31900, 'the_wraiths_of_ginaz'], [8, 45, 2, 1000, 'tears_of_shaitan'], [9, 45, 1, 250, 'tears_of_shaitan'],
];
function answer() {
  const a = synthetic();
  a.character.name = NAME;
  for (const t of a.tasks) t.player_contributions = [];
  for (const [i, pid, fid, amount, guild] of ROWS) a.tasks[i].player_contributions.push({ player_id: pid, faction_id: fid, amount, mine: pid === ME, ...(guild ? { guild } : {}) });
  return a;
}
// the ranking worked out again from the raw rows, without the model: sum by (faction, contributor), biggest first, equal amounts by label
function independent(a, faction) {
  const by = new Map();
  for (const t of a.tasks) for (const r of t.player_contributions) if (r.faction_id === faction) by.set(r.player_id, (by.get(r.player_id) || 0) + r.amount);
  const label = (id) => {
    if (id === ME) return NAME;
    for (const t of a.tasks) for (const r of t.player_contributions) if (r.player_id === id && r.guild) return r.guild.split('_').map((w) => w[0].toUpperCase() + w.slice(1)).join(' ');
    return 'Contributor ' + id;
  };
  return [...by].map(([id, amount]) => [label(id), amount]).sort((x, y) => y[1] - x[1] || x[0].localeCompare(y[0]));
}
async function open(a) {
  const ui = await boot({ get: { '/api/landsraad': a }, local: { tab: 'landsraad', 'lsb.view': 'board' } });
  await ui.settle();
  return ui;
}

test('data: the ranking of each faction is the sum of each contributor over all the Houses, the biggest first, equal amounts by name, the character is named by the save', () => {
  const a = answer();
  const b = M.readBoard(a);
  for (const [k, fid] of [['a', 1], ['h', 2]]) {
    assert.deepEqual(b.contributors[k].map((c) => [c.label, c.amount]), independent(a, fid), 'faction ' + k);
  }
  assert.deepEqual(b.contributors.a.map((c) => [c.label, c.amount]), [['Bravo Squad', 1500], ['Dune Divers', 1000], ['Fortune Hunters Inc', 1000], ['Red Hawk Corps', 1000], ['Tears Of Shaitan', 250]], 'a tie of 1,000 is by name');
  assert.deepEqual(b.contributors.h.map((c) => [c.label, c.amount]), [[NAME, 36600], ['The Wraiths Of Ginaz', 31900], ['Tears Of Shaitan', 1000]], 'the character over two Houses is one line');
  assert.equal(b.contributors.h[0].mine, true);
  assert.equal(b.contributors.a.some((c) => c.mine), false, 'the character gave nothing under Atreides in this answer');
});

test('data: a contributor with no amount, a zero amount or another faction is not counted under this one, and nothing is made up', () => {
  const a = answer();
  a.tasks[10].player_contributions.push({ player_id: 77, faction_id: 1, amount: 0, mine: false }, { player_id: 78, faction_id: 1, amount: null, mine: false }, { player_id: 79, faction_id: 3, amount: 5000, mine: false });
  const b = M.readBoard(a);
  assert.deepEqual(b.contributors.a.map((c) => c.id).filter((id) => id >= 77), []);
  assert.deepEqual(b.contributors.h.map((c) => c.id).filter((id) => id >= 77), []);
});

test('layout: each banner line is a name and an amount of its own, the whole line in the name\'s mouse-over, nothing else on the line', async () => {
  const ui = await open(answer());
  for (const [cls, want] of [['.lcc-f-a', [['Bravo Squad', '1,500'], ['Dune Divers', '1,000'], ['Fortune Hunters Inc', '1,000']]], ['.lcc-f-h', [[NAME, '36,600'], ['The Wraiths Of Ginaz', '31,900'], ['Tears Of Shaitan', '1,000']]]]) {
    const items = [...ui.doc.querySelectorAll('.lcc-plaque ' + cls + ' .lcc-standard-list > li')];
    assert.deepEqual(items.map((li) => [text(li.querySelector('.lcc-standard-name')), text(li.querySelector('.lcc-standard-amt'))]), want);
    for (const li of items) {
      assert.deepEqual([...li.children].map((c) => c.className), ['lcc-standard-name', 'lcc-standard-amt']);
      assert.equal(li.querySelector('.lcc-standard-name').getAttribute('title'), text(li.querySelector('.lcc-standard-name')) + ' · ' + text(li.querySelector('.lcc-standard-amt')));
    }
  }
});

test('layout: a long name and a hostile one stay text and the amount stays its own whole piece', () => {
  const { L } = load();
  const std = L.standard('a', [{ name: '<img src=x onerror=alert(1)>' + 'x'.repeat(120), amount: '1,000' }, { name: 'Short', amount: '999' }]);
  assert.equal(std.querySelectorAll('img:not(.lcc-standard-img):not(.lcc-standard-tip)').length, 0);
  assert.deepEqual([...std.querySelectorAll('.lcc-standard-amt')].map((e) => e.textContent), ['1,000', '999']);
  assert.equal(std.querySelectorAll('li').length, 2);
});

test('layout: no line is cut with an ellipsis; the amount never breaks; the wide list is a column that wraps so a whole entry that does not fit is out of sight, not cut through', () => {
  for (const m of css.matchAll(/([^{}]*lcc-standard-(?:name|amt|list)[^{}]*)\{([^}]*)\}/g)) assert.doesNotMatch(m[2], /text-overflow\s*:\s*ellipsis/, m[1].trim());
  assert.match(css, /\.lcc-standard-amt \{[^}]*white-space: nowrap/);
  assert.match(css, /\.lcc-standard-name \{[^}]*overflow-wrap: anywhere/);
  assert.match(css, /\.lcc-standard-list > li \{[^}]*display: block; padding-left: 1\.5em; text-indent: -1\.5em/, 'the number hangs and the name and the amount run on as text: the amount follows the last word of a wrapped name');
  const wide = /\.lcc-pl-std-a \.lcc-standard-list, \.lcc-pl-std-h \.lcc-standard-list \{([^}]*)\}/.exec(css);
  assert.ok(wide, 'the wide list rule exists for both standards');
  assert.match(wide[1], /flex-flow: column wrap/);
  assert.match(wide[1], /overflow: hidden/);
  assert.match(wide[1], /min-height: 0/);
  assert.match(wide[1], /font-size: clamp\(11px,/, 'the 11 px text floor: the operator chose smaller text over a cut name (#972)');
  assert.match(css, /\.lcc-pl-std-a \.lcc-standard-list > li, \.lcc-pl-std-h \.lcc-standard-list > li \{[^}]*width: 100%/);
  assert.ok(css.includes('.lcc-standard-amt { color: CanvasText; }'), 'forced colours: the amount takes the text colour');
});

test('wording: the heading says how the ranking is made, gives the three whole, and an empty faction says the save holds no rows', async () => {
  const ui = await open(answer());
  const titles = [...ui.doc.querySelectorAll('.lcc-plaque .lcc-standard-h')].map((h) => h.getAttribute('title'));
  assert.equal(titles.length, 2);
  for (const t of titles) {
    assert.match(t, /worked out here: the save holds amounts, not a ranking/);
    assert.match(t, /added up over all the Houses of the term/);
    assert.match(t, /equal amounts are listed by name, A to Z/);
    assert.match(t, /Progress that the save records without a contributor is not named here/);
  }
  assert.match(titles[0], /Top 3: 1\. Bravo Squad 1,500; 2\. Dune Divers 1,000; 3\. Fortune Hunters Inc 1,000\.$/);
  assert.match(titles[1], /Top 3: 1\. Ahsan Quill 36,600; 2\. The Wraiths Of Ginaz 31,900; 3\. Tears Of Shaitan 1,000\.$/);
  const empty = answer();
  for (const t of empty.tasks) t.player_contributions = t.player_contributions.filter((r) => r.faction_id !== 1);
  const ui2 = await open(empty);
  const t2 = [...ui2.doc.querySelectorAll('.lcc-plaque .lcc-standard-h')].map((h) => h.getAttribute('title'));
  assert.match(t2[0], /holds no contribution rows under Atreides/);
  assert.equal(ui2.doc.querySelectorAll('.lcc-plaque .lcc-f-a .lcc-standard-none').length, 1);
});

// the longest simulated guild names of a real save (landsraad_simulated_guilds, 60 guilds): the full name is always on the banner, as its own text, never cut
const LONGEST = ['salvagers_association', 'the_teeth_in_the_dark', 'celestial_privateers', 'confederation_of_woe'];
const LONGEST_WORDS = ['Salvagers Association', 'The Teeth In The Dark', 'Celestial Privateers', 'Confederation Of Woe'];
test('the longest guild names of a save are on the banner whole, each with its amount, and whole in their mouse-over text', async () => {
  const a = answer();
  for (const t of a.tasks) t.player_contributions = [];
  LONGEST.forEach((g, i) => a.tasks[i].player_contributions.push({ player_id: 40 + i, faction_id: 1, amount: 4000 - i, mine: false, guild: g }));
  const ui = await open(a);
  const items = [...ui.doc.querySelectorAll('.lcc-plaque .lcc-f-a .lcc-standard-list > li')];
  assert.deepEqual(items.map((li) => li.querySelector('.lcc-standard-name').textContent), LONGEST_WORDS.slice(0, 3), 'the banner holds three, each name exactly as the save gives it');
  assert.deepEqual(items.map((li) => li.querySelector('.lcc-standard-amt').textContent), ['4,000', '3,999', '3,998']);
  assert.deepEqual(items.map((li) => li.querySelector('.lcc-standard-name').getAttribute('title')), ['Salvagers Association · 4,000', 'The Teeth In The Dark · 3,999', 'Celestial Privateers · 3,998']);
  const title = ui.doc.querySelector('.lcc-plaque .lcc-f-a .lcc-standard-h').getAttribute('title');
  assert.match(title, /Top 3: 1\. Salvagers Association 4,000; 2\. The Teeth In The Dark 3,999; 3\. Celestial Privateers 3,998\.$/);
});

test('the character is listed by the name in the save, on the banner, in the House panel and in the inspector; "You" only when the name cannot be read', async () => {
  const a = answer();
  const ui = await open(a);
  const mineLine = [...ui.doc.querySelectorAll('.lcc-plaque .lcc-f-h .lcc-standard-list > li')][0];
  assert.equal(mineLine.querySelector('.lcc-standard-name').textContent, NAME);
  const b = M.readBoard(a);
  const house = b.houses.find((h) => h.raw.id === a.tasks[5].id);
  assert.equal(house.contributors[0].label, NAME);
  const tile = [...ui.doc.querySelectorAll('.lcc-bd-cell')].find((c) => c.querySelector('.lcc-hc-name').textContent.toLowerCase() === 'house ' + house.name.toLowerCase());
  tile.querySelector('button').focus();
  tile.querySelector('button').dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true }));
  const insp = ui.doc.querySelector('.lcc-bd-insp');
  assert.match(text(insp.querySelector('.lcc-bd-contrib')), new RegExp('^Contributors to this House, from the save, the biggest first: ' + NAME + ' 33,600'));
  assert.match(insp.querySelector('#lcc-bd-s1').getAttribute('title'), new RegExp('Top contributors: ' + NAME + ' 33,600'));
  assert.doesNotMatch(text(insp), /\bYou\b/, 'no "You" next to the character name in the panel');
  for (const missing of [undefined, null, '', '   ']) {
    const c = answer();
    c.character.name = missing;
    const bb = M.readBoard(c);
    assert.equal(bb.contributors.h[0].label, 'You', 'name ' + JSON.stringify(missing) + ' falls back to You');
    assert.equal(bb.contributors.h[0].mine, true);
  }
});
