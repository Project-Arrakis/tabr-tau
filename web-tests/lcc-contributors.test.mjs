// The TOP CONTRIBUTORS of the two banners (#972). Three things were looked at, against a made-up answer built like the one read from a real save (the guild names are the save's own
// guild_identifier values, the rows are the rows of landsraad_task_player_contributions of a term):
//   data     the ranking of a faction is the sum of each contributor's rows over all the Houses of the term, the biggest first, equal amounts by name; the character is named by the save ("You" only when no name can be read). It was right, and is
//            checked here against an independent sum of the raw rows.
//   layout   each line showed "name · amount" in one cut-off piece (#972 then gave the amount a piece of its own). The operator wants the names only (#1043), so a line is the name: it wraps, and an entry that does not fit
//            in the height of the cloth is not drawn at all (a second column that is out of sight), never cut through. The amounts are still in the data and in the Inspector of a House, never on the banner.
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
    return 'Unnamed contributor';
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

test('layout: each banner line is the name only (#1043), no amount on the line or in its mouse-over, nothing else on the line', async () => {
  const ui = await open(answer());
  for (const [cls, want] of [['.lcc-f-a', ['Bravo Squad', 'Dune Divers', 'Fortune Hunters Inc']], ['.lcc-f-h', [NAME, 'The Wraiths Of Ginaz', 'Tears Of Shaitan']]]) {
    const items = [...ui.doc.querySelectorAll('.lcc-plaque ' + cls + ' .lcc-standard-list > li')];
    assert.deepEqual(items.map((li) => text(li)), want, 'the line is the name and nothing else (the order is the biggest first)');
    for (const li of items) {
      assert.deepEqual([...li.children].map((c) => c.className), ['lcc-standard-name']);
      assert.equal(li.querySelector('.lcc-standard-name').getAttribute('title'), text(li.querySelector('.lcc-standard-name')), 'the mouse-over is the whole name, with no amount');
      assert.doesNotMatch(li.outerHTML, /\d[\d,]{2,}/, 'no number on the line');
    }
    assert.equal(ui.doc.querySelectorAll('.lcc-plaque ' + cls + ' .lcc-standard-amt').length, 0);
  }
});

test('layout: a long name and a hostile one stay text; an entry that still carries an amount is drawn as its name only (#1043)', () => {
  const { L } = load();
  const std = L.standard('a', [{ name: '<img src=x onerror=alert(1)>' + 'x'.repeat(120), amount: '1,000' }, { name: 'Short', amount: '999' }]);
  assert.equal(std.querySelectorAll('img:not(.lcc-standard-img):not(.lcc-standard-tip)').length, 0);
  assert.equal(std.querySelectorAll('.lcc-standard-amt').length, 0);
  assert.doesNotMatch(std.textContent, /1,000|999/, 'the amount is not drawn');
  assert.doesNotMatch(std.outerHTML, /1,000|999/, 'nor in a mouse-over');
  assert.equal(std.querySelectorAll('li').length, 2);
});

test('layout: no line is cut with an ellipsis; the wide list is a column that wraps so a whole entry that does not fit is out of sight, not cut through', () => {
  for (const m of css.matchAll(/([^{}]*lcc-standard-(?:name|amt|list)[^{}]*)\{([^}]*)\}/g)) assert.doesNotMatch(m[2], /text-overflow\s*:\s*ellipsis/, m[1].trim());
  assert.doesNotMatch(css, /lcc-standard-amt/, 'no rule is left for an amount (#1043)');
  assert.match(css, /\.lcc-standard-name \{[^}]*overflow-wrap: anywhere/);
  assert.match(css, /\.lcc-standard-list > li \{[^}]*display: flex;/, 'a line is a row: the number, then the name in a column of its own, so they stay side by side');
  const wide = /\.lcc-pl-std-a \.lcc-standard-list, \.lcc-pl-std-h \.lcc-standard-list \{([^}]*)\}/.exec(css);
  assert.ok(wide, 'the wide list rule exists for both standards');
  assert.match(wide[1], /flex-flow: column wrap/);
  assert.match(wide[1], /overflow: hidden/);
  assert.match(wide[1], /min-height: 0/);
  assert.match(wide[1], /font-size: clamp\(12px,/, 'the names are never under 12 px (#918): a whole entry that does not fit is out of sight (#972), the text is not made smaller');
  assert.match(css, /\.lcc-pl-std-a \.lcc-standard-h, \.lcc-pl-std-h \.lcc-standard-h \{[^}]*font-size: clamp\(11px,/, 'the heading TOP CONTRIBUTORS is other text and keeps its 11 px floor');
  assert.match(css, /\.lcc-pl-std-a \.lcc-standard-list > li, \.lcc-pl-std-h \.lcc-standard-list > li \{[^}]*width: 100%/);
});

test('layout: the rank numerals 1. to 5. are drawn before each name, beside it, in both layouts, at the size of the name, and the list stays an ordered list (#1066, restoring what #992 removed, by the operator decision)', () => {
  const { L } = load();
  const std = L.standard('h', ['a', 'b', 'c', 'd', 'e', 'f', 'g']);
  const ol = std.querySelector('.lcc-standard-list');
  assert.equal(ol.tagName, 'OL', 'the order is announced: it stays an ordered list');
  assert.equal(ol.getAttribute('role'), 'list', 'a list with no marker of its own keeps its list role');
  assert.equal(ol.children.length, 5, 'up to five names');
  assert.deepEqual([...ol.children].map((li) => li.tagName), ['LI', 'LI', 'LI', 'LI', 'LI']);
  assert.doesNotMatch(std.textContent, /\d/, 'the numeral is drawn by the stylesheet, so a name is never changed by it and no amount or number is in the text');
  const list = /\.lcc-standard-list \{([^}]*)\}/.exec(css)[1];
  assert.match(list, /counter-reset: lcc-n/, 'the ordered list starts the rank counter');
  const li = /\.lcc-standard-list > li \{([^}]*)\}/.exec(css)[1];
  assert.match(li, /counter-increment: lcc-n/, 'each line counts one');
  assert.match(li, /display: flex/, 'the number and the name are side by side whatever the name does (a long unbroken name breaks inside its own column, it is never pushed under the number)');
  assert.doesNotMatch(li, /flex-(?:wrap|flow)|padding|text-indent|flex-direction: column/, 'the line is one row');
  const before = /\.lcc-standard-list > li::before \{([^}]*)\}/.exec(css);
  assert.ok(before, 'the numeral rule exists');
  assert.match(before[1], /content: counter\(lcc-n\) '\.'/, 'the numeral is 1. 2. 3. 4. 5.');
  assert.match(before[1], /flex: none/, 'the number never shrinks');
  assert.match(before[1], /width: 1\.5em/, 'as wide as the room the line leaves');
  assert.match(css, /\.lcc-standard-name \{[^}]*min-width: 0[^}]*\}/, 'the name column may shrink to the room that is left and breaks inside itself');
  assert.doesNotMatch(before[1], /font-size|display: none|visibility|opacity|position: absolute|float/, 'the numeral is the size of the name (12 px or more) and sits in the line');
  // nothing in the wide or the narrow layout hides it, shrinks it or takes its room away: every rule for the list and its lines is looked at
  for (const m of css.matchAll(/([^{}]*lcc-standard-list[^{}]*)\{([^}]*)\}/g)) {
    const sel = m[1].trim();
    if (/::(?:before|after)/.test(sel)) assert.ok(sel === '.lcc-standard-list > li::before' || /^\.lcc-pl-decree-l,.*\.lcc-standard-list > li::before$/.test(sel), 'a rule for the numeral that is neither its own nor the forced-colours colour: ' + sel);
    if (/\bli\b/.test(sel) && !/::before/.test(sel)) assert.doesNotMatch(m[2], /padding(?:-left)?\s*:|text-indent|display: *(?:block|inline)/, 'a rule that would take the room of the number: ' + sel);
    assert.doesNotMatch(m[2], /display: none|visibility: hidden|list-style\s*:\s*[^n;\s]/, sel);
  }
  // the names are never under 12 px (wide and narrow) and the numeral takes the same size, being text of the line
  assert.match(/\.lcc-pl-std-a \.lcc-standard-list, \.lcc-pl-std-h \.lcc-standard-list \{([^}]*)\}/.exec(css)[1], /font-size: clamp\(12px,/);
  assert.match(css, /\.lcc-standard-list \{ font-size: 12px;/, 'narrow: 12 px');
  // the wide line is a full-width block; box-sizing keeps the room of the number inside the 100%
  assert.match(css, /\.lcc-pl-std-a \.lcc-standard-list > li, \.lcc-pl-std-h \.lcc-standard-list > li \{[^}]*box-sizing: border-box/);
  // forced colours: the numeral takes the system text colour like the name (it is gold on the cloth otherwise)
  assert.match(css, /\.lcc-standard-list, \.lcc-standard-list > li::before \{ color: CanvasText;/);
});

test('wording: the heading says how the ranking is made, gives the three whole, and an empty faction says the save holds no rows', async () => {
  const ui = await open(answer());
  const titles = [...ui.doc.querySelectorAll('.lcc-plaque .lcc-standard-h')].map((h) => h.getAttribute('title'));
  assert.equal(titles.length, 2);
  for (const t of titles) {
    assert.match(t, /^The biggest contributors under [A-Za-z]+ this term[.]/);
    assert.doesNotMatch(t, /in the save|worked out here|rows/, 'plain words: no save or table talk (#1095)');
    assert.match(t, /added up over all the Houses of the term/);
    assert.match(t, /equal amounts are listed by name, A to Z/);
    assert.match(t, /Progress with no contributor recorded is not named here/);
  }
  assert.match(titles[0], /Top 3: 1\. Bravo Squad; 2\. Dune Divers; 3\. Fortune Hunters Inc\.$/);
  assert.match(titles[1], /Top 3: 1\. Ahsan Quill; 2\. The Wraiths Of Ginaz; 3\. Tears Of Shaitan\.$/);
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
test('the longest guild names of a save are on the banner whole, by name only, and whole in their mouse-over text (#1043)', async () => {
  const a = answer();
  for (const t of a.tasks) t.player_contributions = [];
  LONGEST.forEach((g, i) => a.tasks[i].player_contributions.push({ player_id: 40 + i, faction_id: 1, amount: 4000 - i, mine: false, guild: g }));
  const ui = await open(a);
  const items = [...ui.doc.querySelectorAll('.lcc-plaque .lcc-f-a .lcc-standard-list > li')];
  assert.deepEqual(items.map((li) => li.querySelector('.lcc-standard-name').textContent), LONGEST_WORDS.slice(0, 3), 'the banner holds three, each name exactly as the save gives it');
  assert.equal(ui.doc.querySelectorAll('.lcc-plaque .lcc-standard-amt').length, 0, 'no amount');
  assert.deepEqual(items.map((li) => li.querySelector('.lcc-standard-name').getAttribute('title')), ['Salvagers Association', 'The Teeth In The Dark', 'Celestial Privateers']);
  const title = ui.doc.querySelector('.lcc-plaque .lcc-f-a .lcc-standard-h').getAttribute('title');
  assert.match(title, /Top 3: 1\. Salvagers Association; 2\. The Teeth In The Dark; 3\. Celestial Privateers\.$/);
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
  assert.equal(insp.querySelector('.lcc-bd-contrib'), null, 'the Inspector lists no contributors (#1077); the plaque does');
  assert.doesNotMatch(insp.querySelector('#lcc-bd-s1').getAttribute('title'), /contributors/i);
  assert.doesNotMatch(text(insp), /\bYou\b/, 'no "You" next to the character name in the panel');
  for (const missing of [undefined, null, '', '   ']) {
    const c = answer();
    c.character.name = missing;
    const bb = M.readBoard(c);
    assert.equal(bb.contributors.h[0].label, 'You', 'name ' + JSON.stringify(missing) + ' falls back to You');
    assert.equal(bb.contributors.h[0].mine, true);
  }
});
