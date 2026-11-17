// The Command Center board on the real page (M3, stage B): the view switch, the fixed five-by-five board, the list, the House Inspector and the filters. The page code is the real app.js with the
// real lcc scripts, over the synthetic answer of M1 (no game save is read). What a click writes is tested in lcc-board-write.test.mjs.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();

// the page, on the Landsraad tab, in the view asked for
async function open(answer = synthetic(), { view = 'board', layout, post } = {}) {
  const local = { tab: 'landsraad', 'lsb.view': view };
  if (layout) local['lsb.layout'] = layout;
  const ui = await boot({ get: { '/api/landsraad': answer }, post, local });
  await ui.settle();
  return ui;
}
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const tiles = (ui) => [...ui.doc.querySelectorAll('.lcc-bd-cell')];
const tileOf = (ui, name) => tiles(ui).find((c) => c.querySelector('.lcc-house-name').textContent.toLowerCase() === name.toLowerCase());

test('the classic list is the default view, and the switch offers the board', async () => {
  const ui = await open(synthetic(), { view: 'classic' });
  assert.ok(ui.doc.querySelector('#lsb-rows'), 'the classic rows are there');
  assert.equal(ui.doc.querySelector('#lsb-lcc'), null);
  const sw = [...ui.doc.querySelectorAll('.viewsw [data-lb="view"]')];
  assert.deepEqual(sw.map((b) => [b.dataset.view, b.getAttribute('aria-pressed')]), [['classic', 'true'], ['board', 'false']]);
  const fresh = await boot({ get: { '/api/landsraad': synthetic() }, local: { tab: 'landsraad' } });
  await fresh.settle();
  assert.ok(fresh.doc.querySelector('#lsb-rows'), 'with nothing remembered the classic list opens');
});

test('the view switch changes how the same Houses are drawn, and the choice is remembered', async () => {
  const ui = await open(synthetic(), { view: 'classic' });
  click(ui, '.viewsw [data-view="board"]');
  assert.equal(tiles(ui).length, 25);
  assert.equal(ui.doc.querySelector('#lsb-rows'), null, 'the classic rows are gone');
  assert.equal(ui.w.localStorage.getItem('lsb.view'), 'board');
  assert.equal(ui.doc.querySelector('.viewsw [data-view="board"]').getAttribute('aria-pressed'), 'true');
  click(ui, '.viewsw [data-view="classic"]');
  assert.ok(ui.doc.querySelector('#lsb-rows'));
  assert.equal(ui.doc.querySelector('.lcc-bd'), null);
  assert.equal(ui.w.localStorage.getItem('lsb.view'), 'classic');
});

test('the board is 25 tiles in board order, numbered 01 to 25, each a button with its owner in words', async () => {
  const ui = await open();
  const t = tiles(ui);
  assert.equal(t.length, 25);
  assert.deepEqual(t.map((c) => c.querySelector('.lcc-house-no').textContent), [...Array(25).keys()].map((i) => String(i + 1).padStart(2, '0')));
  assert.deepEqual(t.map((c) => Number(c.dataset.index)), [...Array(25).keys()]);
  for (const c of t) {
    const b = c.querySelector('button.lcc-house');
    assert.equal(b.getAttribute('type'), 'button');
    assert.match(b.getAttribute('aria-label'), /^Number \d\d, /);
    assert.match(c.querySelector('.lcc-owner').textContent, /Atreides|Harkonnen|Open|Unreadable/);
  }
  assert.equal(text(tiles(ui)[0].querySelector('.lcc-owner')), 'H Harkonnen', 'position 01 is Vernius, taken by Harkonnen in the fixture');
  assert.equal(ui.doc.querySelector('.lcc-bd-tiles').getAttribute('role'), 'list');
  const ev = ui.doc.querySelector('.lcc-bd-tiles').getAttribute('aria-label');
  assert.match(ev, /five rows of five/);
});

test('the plaque and the figures are the counts of the same Houses: 12 and 4, nine open, and never a won vote', async () => {
  const ui = await open();
  const p = ui.doc.querySelector('.lcc-plaque-host');
  assert.equal(p.getAttribute('data-a'), '12');
  assert.equal(p.getAttribute('data-h'), '4');
  assert.match(text(p.querySelector('.lcc-pl-standing')), /ATREIDES LEAD 12 TO 4 · 9 HOUSES OPEN · VOTE NOT CONFIRMED/);
  assert.doesNotMatch(text(ui.doc.querySelector('.lcc-bd')), /\bwins?\b|victory|won the vote/i);
  const k = [...ui.doc.querySelectorAll('.lcc-bd-kpi')].map((x) => [text(x.querySelector('.lcc-bd-kpi-l')), text(x.querySelector('.lcc-bd-kpi-v'))]);
  assert.deepEqual(k.slice(0, 3), [['Atreides Houses', '12'], ['Harkonnen Houses', '4'], ['Open Houses', '9']]);
  assert.match(text(p.querySelector('.lcc-pl-held')), /^HELD BY\s*HOUSE ATREIDES$/);
  assert.equal(text(p.querySelector('.lcc-pl-title')), 'Exclusive Vehicle Import Rights');
  assert.ok([...p.querySelectorAll('.lcc-standard-none')].every((n) => text(n) === 'Not in the save'), 'contributor lists are not in the save');
});

test('a House whose owner cannot be read is counted for neither side and the plaque says so', async () => {
  const a = synthetic();
  a.tasks[10].completed = 1; a.tasks[10].winning_faction_id = null; // taken with no winner: not readable
  const ui = await open(a);
  const p = ui.doc.querySelector('.lcc-plaque-host');
  assert.match(text(p.querySelector('.lcc-pl-standing')), /COUNT INCOMPLETE · 1 HOUSE UNREADABLE · VOTE NOT CONFIRMED/);
  assert.equal(text(tiles(ui)[10].querySelector('.lcc-owner')), '? Unreadable');
  const k = [...ui.doc.querySelectorAll('.lcc-bd-kpi')].map((x) => text(x.querySelector('.lcc-bd-kpi-l')));
  assert.ok(k.includes('Unreadable owner'));
});

test('a board that is not a board (24 Houses) is not drawn as one: a read-only list, the reasons, and nothing that writes', async () => {
  const a = synthetic();
  a.tasks.pop();
  const ui = await open(a);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-tiles').length, 0);
  assert.equal(ui.doc.querySelector('.lcc-bd').getAttribute('data-ready'), 'false');
  assert.match(text(ui.doc.querySelector('.lcc-bd-problems')), /24 of 25 Houses/);
  assert.match(text(ui.doc.querySelector('.lcc-bd-problems')), /classic list/);
  const rows = ui.doc.querySelectorAll('.lcc-bd-row');
  assert.equal(rows.length, 24);
  assert.ok([...rows].every((r) => text(r.querySelector('.lcc-bd-c-no')) === '—'), 'no number is given by a board that failed its checks');
  assert.equal(ui.doc.querySelectorAll('[data-lb="qa"], [data-lb="tier"], [data-lb="pick"], [data-lb="bulk"], #lsb-fac').length, 0, 'no control that writes');
  click(ui, '.lcc-bd-row .lcc-bd-namebtn');
  assert.match(text(ui.doc.querySelector('.lcc-bd-insp')), /Read-only/);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-insp [data-lb]').length <= 1, true, 'the Inspector offers no write (at most the Saved map link)');
  assert.equal(ui.doc.querySelector('[data-lcb="layout"][data-val="grid"]').getAttribute('aria-disabled'), 'true');
});

test('a duplicate board position is refused too, and the House number is never drawn from it', async () => {
  const a = synthetic();
  a.tasks[3].board_index = 2;
  const ui = await open(a);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-tiles').length, 0);
  assert.match(text(ui.doc.querySelector('.lcc-bd-problems')), /more than one House/);
  assert.equal(ui.doc.querySelector('.lcc-house-no'), null);
});

test('#768: the classic list shows the House number 01 to 25 too, from the same positions, and none when the board is not a board', async () => {
  const ui = await open(synthetic(), { view: 'classic' });
  const nos = [...ui.doc.querySelectorAll('#lsb-rows .hno')].map((n) => text(n).replace('Number ', ''));
  assert.equal(nos.length, 25);
  assert.deepEqual([...nos].sort(), [...Array(25).keys()].map((i) => String(i + 1).padStart(2, '0')));
  const row = [...ui.doc.querySelectorAll('#lsb-rows > li')].find((r) => r.querySelector('.hn b').textContent === 'Wallach');
  assert.equal(text(row.querySelector('.hno')), 'Number 08');
  assert.equal(row.querySelector('.hno').getAttribute('title'), 'position 8 of 25, row 2, column 3');
  const a = synthetic(); a.tasks.pop();
  const bad = await open(a, { view: 'classic' });
  assert.equal(bad.doc.querySelectorAll('#lsb-rows .hno').length, 0);
});

test('a tile shows the tiers reached as pips and the rewards waiting, and says both in its name for a screen reader', async () => {
  const ui = await open();
  const w = tileOf(ui, 'Wallach');
  assert.equal(w.querySelectorAll('.lcc-house-meter i').length, 5);
  assert.equal(w.querySelectorAll('.lcc-house-meter i.on').length, 3);
  assert.match(w.querySelector('.lcc-flag-wait').textContent, /3 WAITING/);
  assert.match(w.querySelector('button').getAttribute('aria-label'), /Number 08, Wallach, claimed by Atreides, path Exploration, 3 rewards waiting, 3 of 5 tiers reached$/);
});

test('names and paths from a save are only text on the board, the list and the Inspector', async () => {
  const hostile = '<img src=x onerror=alert(1)>';
  const a = synthetic();
  a.tasks[0].house_name = hostile;
  a.tasks[0].line = hostile;
  a.decrees[0].display_name = hostile;
  const ui = await open(a);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd img[src="x"], .lcc-bd [onerror]').length, 0);
  click(ui, tiles(ui)[0].querySelector('button'));
  assert.equal(ui.doc.querySelectorAll('.lcc-bd img[src="x"], .lcc-bd [onerror]').length, 0);
  assert.ok(text(ui.doc.querySelector('.lcc-bd-insp-name')).toLowerCase().includes('onerror'), 'the name is shown as the text it is');
  click(ui, '[data-lcb="layout"][data-val="list"]');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd img[src="x"], .lcc-bd [onerror]').length, 0);
});

test('Show, House and Path dim the Houses they leave out and keep the board fixed; the count says how many are shown', async () => {
  const ui = await open();
  assert.equal(text(ui.doc.querySelector('#lcc-bd-shown')), '25 of 25 Houses shown');
  click(ui, '[data-fk="bf:hs:open"]');
  assert.equal(text(ui.doc.querySelector('#lcc-bd-shown')), '9 of 25 Houses shown');
  const dim = tiles(ui).filter((c) => c.querySelector('.lcc-house').dataset.dim === 'true');
  assert.equal(dim.length, 16);
  assert.equal(tiles(ui).length, 25, 'the board never loses a tile');
  assert.deepEqual(tiles(ui).map((c) => Number(c.dataset.index)), [...Array(25).keys()], 'and never moves one');
  assert.match(dim[0].querySelector('button').getAttribute('aria-label'), /not in the current filter/);
  assert.equal(ui.doc.querySelector('[data-fk="bf:hs:open"]').getAttribute('aria-pressed'), 'true');
  assert.equal(ui.doc.querySelector('[data-fk="bf:hs:any"]').getAttribute('aria-pressed'), 'false');
  const sel = ui.doc.querySelector('#lcc-bd-spec'); sel.value = 'combat'; sel.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.match(text(ui.doc.querySelector('#lcc-bd-shown')), /^\d of 25 Houses shown$/);
  for (const c of tiles(ui)) if (c.querySelector('.lcc-house').dataset.dim !== 'true') assert.match(c.querySelector('.lcc-house-path').textContent, /Combat/);
});

test('Jump to House selects it, shows it whatever the filters were, and puts the focus on its tile', async () => {
  const ui = await open();
  click(ui, '[data-fk="bf:hs:taken"]');
  const j = ui.doc.querySelector('#lcc-bd-jump');
  const sor = [...j.options].find((o) => /Sor$/.test(o.textContent));
  assert.match(sor.textContent, /^09 · /);
  j.value = sor.value; j.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.equal(text(ui.doc.querySelector('.lcc-bd-insp-name')), 'House Sor');
  assert.equal(ui.doc.querySelector('[data-fk="bf:hs:any"]').getAttribute('aria-pressed'), 'true', 'the filter that would have hidden it is cleared');
  assert.equal(ui.doc.activeElement.getAttribute('data-fk'), 'ht:' + sor.value);
});

test('choosing a House opens the Inspector: both scales apart, the contributors and missions as "Not in the save", no sysselraad', async () => {
  const ui = await open();
  click(ui, tileOf(ui, 'Wallach').querySelector('button'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  assert.equal(text(i.querySelector('.lcc-bd-insp-name')), 'House Wallach');
  assert.equal(text(i.querySelector('.lcc-bd-no')), 'No. 08');
  assert.match(text(i.querySelector('.lcc-bd-status')), /^Taken by Atreides/);
  const bars = [...i.querySelectorAll('.lcc-bd-fbar')].map((b) => [text(b.querySelector('.lcc-bd-fbar-l')), text(b.querySelector('.lcc-bd-fbar-v'))]);
  assert.deepEqual(bars, [['Atreides', '4,000'], ['Harkonnen', '900']]);
  assert.match(text(i), /House goal: 4,000/);
  assert.match(text(i), /Top contributors: Not in the save\./);
  assert.match(text(i), /Missions: Not in the save\./);
  assert.match(text(i), /Your contribution: 3,600 · next tier 4,800, 1,200 more/);
  assert.doesNotMatch(text(ui.doc.querySelector('.lcc-bd')), /sysselraad/i);
  const tiers = [...i.querySelectorAll('.lcc-bd-tier')];
  assert.equal(tiers.length, 5);
  assert.deepEqual(tiers.map((t) => t.getAttribute('aria-pressed')), ['true', 'true', 'true', 'false', 'false']);
  assert.equal(tiers[0].dataset.lb, 'tier');
  assert.equal(tiers[0].dataset.id, '9008');
  assert.equal(ui.doc.querySelector('[data-fk="ht:9008"]').getAttribute('aria-pressed'), 'true');
  click(ui, tileOf(ui, 'Wallach').querySelector('button'));
  assert.equal(ui.doc.querySelector('.lcc-bd-insp-name'), null, 'a second click on the same tile closes the Inspector');
});

test('the reward ledger tells waiting, collected and delivery unknown apart, and says what "unknown" means', async () => {
  const a = synthetic();
  const t = a.tasks[7];
  t.rewards[0].reward_state = 'delivered';
  t.rewards[1].reward_state = 'no_row';
  const ui = await open(a);
  click(ui, tileOf(ui, 'Wallach').querySelector('button'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  assert.match(text(i), /3 of 5 reached · 1 waiting to collect · 1 collected · 1 delivery unknown\./);
  assert.match(text(i), /Delivery unknown.{1,20}means the tier is reached and the save has no delivery row/);
  const st = [...i.querySelectorAll('.lcc-bd-tier-st')].map((s) => text(s));
  assert.deepEqual(st.slice(0, 3), ['Reached, collected', 'Reached, delivery unknown', 'Reached, waiting to collect']);
});

test('the list: board order by default, sortable by every column both ways, grouped by region with the unmapped last', async () => {
  const ui = await open(synthetic(), { layout: 'list' });
  const names = () => [...ui.doc.querySelectorAll('.lcc-bd-row .lcc-bd-namebtn')].map((b) => text(b));
  const nos = () => [...ui.doc.querySelectorAll('.lcc-bd-row .lcc-bd-c-no')].map((b) => text(b));
  assert.equal(ui.doc.querySelector('.lcc-bd-tiles'), null);
  assert.deepEqual(nos(), [...Array(25).keys()].map((i) => String(i + 1).padStart(2, '0')));
  assert.equal(ui.doc.querySelector('th[aria-sort]').textContent.trim().startsWith('No.'), true);
  click(ui, '[data-fk="bs:house"]');
  assert.deepEqual(names(), [...names()].sort((x, y) => x.localeCompare(y)));
  click(ui, '[data-fk="bs:house"]');
  assert.deepEqual(names(), [...names()].sort((x, y) => y.localeCompare(x)));
  assert.equal(ui.doc.querySelector('th[aria-sort="descending"]').textContent.trim().startsWith('House'), true);
  const g = ui.doc.querySelector('#lcc-bd-group'); g.value = 'region'; g.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  const heads = [...ui.doc.querySelectorAll('.lcc-bd-grouprow th')].map((h) => text(h));
  assert.deepEqual(heads, ['Arrakeen · 1', 'Deep Desert · 1', 'Hagga Basin · 1', 'Location unknown · 22']);
  assert.equal(ui.doc.querySelector('.lcc-bd-list').getAttribute('role'), null, 'a real table, not a pile of divs');
  assert.ok(ui.doc.querySelector('.lcc-bd-list caption'));
});

test('the layout is remembered, and the list carries a Choose box and a Saved map link only where the map has a picture', async () => {
  const ui = await open();
  click(ui, '[data-fk="bf:layout:list"]');
  assert.equal(ui.w.localStorage.getItem('lsb.layout'), 'list');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-row .lcc-bd-pick').length, 25);
  const maps = [...ui.doc.querySelectorAll('.lcc-bd-row [data-lb="map"]')];
  assert.deepEqual(maps.map((m) => m.closest('tr').querySelector('.lcc-bd-namebtn').textContent).sort(), ['Alexin', 'Kenola', 'Lindaren'].filter((n) => maps.some((m) => m.closest('tr').querySelector('.lcc-bd-namebtn').textContent === n)));
  for (const m of maps) assert.match(m.getAttribute('aria-label'), /Open in the Saved map$/);
});

test('every control that changes the save carries the classic list\'s own data-lb, so one handler, one guard and one queue serve both views', async () => {
  const ui = await open();
  click(ui, tileOf(ui, 'Wallach').querySelector('button'));
  const writes = [...ui.doc.querySelectorAll('.lcc-bd [data-lb]')].map((e) => e.dataset.lb);
  assert.deepEqual([...new Set(writes)].sort(), ['pick', 'qa', 'tier']);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd [data-lcb="sel"]').length, 25);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd .lcc-bd-act[data-lb="qa"]').length, 4);
  assert.deepEqual([...ui.doc.querySelectorAll('.lcc-bd .lcc-bd-act[data-lb="qa"]')].map((b) => b.dataset.do), ['fill', 'fillgoal', 'complete', 'reopen']);
});
