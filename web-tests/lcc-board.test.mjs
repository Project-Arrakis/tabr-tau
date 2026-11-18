// The Command Center board on the real page (M3, stage B): the view switch, the fixed five-by-five board, the list, the House Inspector and the filters. The page code is the real app.js with the
// real lcc scripts, over the synthetic answer of M1 (no game save is read). What a click writes is tested in lcc-board-write.test.mjs.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { read } from './lcc-load.mjs';

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
const tileOf = (ui, name) => tiles(ui).find((c) => c.querySelector('.lcc-hc-name').textContent.toLowerCase() === 'house ' + name.toLowerCase());

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
  assert.deepEqual(t.map((c) => c.querySelector('.lcc-hc-pos').textContent), [...Array(25).keys()].map((i) => String(i + 1).padStart(2, '0')));
  assert.deepEqual(t.map((c) => Number(c.dataset.index)), [...Array(25).keys()]);
  for (const c of t) {
    const b = c.querySelector('button.lcc-hc-select');
    assert.equal(b.getAttribute('type'), 'button');
    assert.match(b.getAttribute('aria-label'), /^Number \d\d, /);
    assert.match(c.querySelector('.lcc-hc-owner').textContent, /Atreides|Harkonnen|Open|unreadable/);
  }
  assert.equal(text(tiles(ui)[0].querySelector('.lcc-hc-owner')), '● Harkonnen', 'position 01 is Vernius, taken by Harkonnen in the fixture');
  assert.equal(ui.doc.querySelector('.lcc-bd-tiles').getAttribute('role'), 'list');
  const ev = ui.doc.querySelector('.lcc-bd-tiles').getAttribute('aria-label');
  assert.match(ev, /five rows of five/);
});

test('the plaque and the figures are the counts of the same Houses: 12 and 4, nine open, and no won Sysselraad', async () => {
  const ui = await open();
  const p = ui.doc.querySelector('.lcc-plaque-host');
  assert.equal(p.getAttribute('data-a'), '12');
  assert.equal(p.getAttribute('data-h'), '4');
  assert.match(text(p.querySelector('.lcc-pl-standing')), /ATREIDES LEAD 12 TO 4 · 9 HOUSES OPEN/);
  const own = ui.doc.querySelector('.lcc-bd').cloneNode(true);
  own.querySelector('[data-lcc-slot="bar"]').replaceChildren(); // the page's own action bar sits in the board's grid and has its own words
  assert.doesNotMatch(text(own), /\bwon\b|won the vote/i, 'a lead in the count is not called a win: nobody holds a line of five or 13 Houses');
  const k = [...ui.doc.querySelectorAll('.lcc-bd-metric')].map((x) => [text(x.querySelector('.lcc-bd-metric-k')), text(x.querySelector('strong'))]);
  assert.deepEqual(k.map((x) => x[1]), ['CONTESTED', '9', '4', 'LV 100'], 'the term status, the open Houses and the potential lines read from the save (M4), the cap is a game rule');
  assert.deepEqual(k.map((x) => x[0].replace(/RULE$/, '')), ['TERM STATUS', 'OPEN HOUSES', 'POTENTIAL LINES', 'SPECIALIZATION CAP']);
  assert.match(text(ui.doc.querySelector('.lcc-bd-victory')), /SYSSELRAAD CONTESTED.*ATREIDES LEAD 12 TO 4 · 9 HOUSES OPEN.*Atreides 12 · Harkonnen 4 · Open 9/);
  assert.equal(text(p.querySelector('.lcc-pl-session')), 'SYSSELRAAD CONTESTED');
  assert.match(text(p.querySelector('.lcc-pl-held')), /^HELD BY\s*HOUSE ATREIDES$/);
  assert.equal(text(p.querySelector('.lcc-pl-title')), 'Exclusive Vehicle Import Rights');
  assert.equal(p.querySelectorAll('.lcc-standard-none').length, 0, 'the save holds contribution rows under both factions, so both banners list them');
  assert.match(text(p.querySelector('.lcc-f-a .lcc-standard-name')), /^You · [\d,]+$/, 'the character is the biggest contributor under Atreides in the fixture, named as You');
});

test('a House whose owner cannot be read is counted for neither side and the plaque says so', async () => {
  const a = synthetic();
  a.tasks[10].completed = 1; a.tasks[10].winning_faction_id = null; // taken with no winner: not readable
  const ui = await open(a);
  const p = ui.doc.querySelector('.lcc-plaque-host');
  assert.match(text(p.querySelector('.lcc-pl-standing')), /COUNT INCOMPLETE · 1 HOUSE UNREADABLE/);
  assert.equal(text(tiles(ui)[10].querySelector('.lcc-hc-owner')), '? Owner unreadable');
  assert.match(text(ui.doc.querySelector('.lcc-bd-headlines')), /OWNER UNREADABLE/);
  assert.match(text(ui.doc.querySelector('[data-metric="open"]')), /1 unreadable, not counted/);
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
  assert.equal(ui.doc.querySelector('.lcc-hc-pos'), null);
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

test('a card shows five numbered tier boxes and the line N/5 REACHED with the collected and waiting counts, and says them in its name for a screen reader', async () => {
  const ui = await open();
  const w = tileOf(ui, 'Wallach');
  const boxes = [...w.querySelectorAll('.lcc-hc-t')];
  assert.equal(boxes.length, 5);
  assert.deepEqual(boxes.map((b) => text(b)), ['◆', '◆', '◆', '4', '5'], 'a waiting reward is a diamond, a tier not reached keeps its number');
  assert.deepEqual(boxes.map((b) => b.classList.contains('waiting')), [true, true, true, false, false]);
  assert.equal(text(w.querySelector('.lcc-hc-count')), '3/5 REACHED ✓ 0 ◆ 3');
  assert.match(w.querySelector('button.lcc-hc-select').getAttribute('aria-label'), /^Number 08, Wallach, claimed by Atreides, path Exploration, 3 of 5 tiers reached, 0 collected, 3 rewards not collected$/);
});

test('the five tier boxes tell collected, waiting, delivery unknown and not reached apart, and delivery unknown is never read as collected', async () => {
  const a = synthetic();
  const t = a.tasks[7];
  t.rewards[0].reward_state = 'delivered';
  t.rewards[1].reward_state = 'no_row';
  const ui = await open(a);
  const w = tileOf(ui, 'Wallach');
  const boxes = [...w.querySelectorAll('.lcc-hc-t')];
  assert.deepEqual(boxes.map((b) => text(b)), ['✓', '?', '◆', '4', '5']);
  assert.deepEqual(boxes.map((b) => ['delivered', 'uncertain', 'waiting'].find((c) => b.classList.contains(c)) || 'off'), ['delivered', 'uncertain', 'waiting', 'off', 'off']);
  assert.match(boxes[1].getAttribute('aria-label'), /no delivery record$|no delivery record\./);
  assert.doesNotMatch(boxes[1].getAttribute('aria-label'), /collected/);
  assert.equal(text(w.querySelector('.lcc-hc-count')), '3/5 REACHED ✓ 1 ◆ 1 ? 1', 'the unknown one is counted on its own, in neither the collected nor the waiting figure');
  assert.match(w.querySelector('button.lcc-hc-select').getAttribute('aria-label'), /1 with no delivery record/);
});

test('each card carries the specialization badge of its mapped path in the upper right, and a House without one has no badge and says so in words', async () => {
  const ui = await open();
  const w = tileOf(ui, 'Wallach');
  const badge = w.querySelector('.lcc-hc-top .lcc-hc-spec');
  assert.ok(badge, 'the badge is in the top row, after the number');
  assert.equal(badge.dataset.track, 'exploration');
  assert.equal(badge.getAttribute('title'), 'Exploration specialization');
  assert.equal(badge.querySelector('img').getAttribute('src'), '/lcc/art/specs/exploration.png');
  assert.equal(badge.querySelector('img').getAttribute('alt'), '', 'decorative: the path is also written under the name');
  assert.equal(text(w.querySelector('.lcc-hc-track')), 'Exploration');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-tiles .lcc-hc-spec').length, 25);
  const a = synthetic();
  a.tasks[7].line = 'nothing-known';
  const un = await open(a);
  const c = tileOf(un, 'Wallach');
  assert.equal(c.querySelector('.lcc-hc-spec'), null, 'no badge when the save gives no path');
  assert.equal(text(c.querySelector('.lcc-hc-track')), 'Specialization not in the save');
  assert.match(c.querySelector('button.lcc-hc-select').getAttribute('aria-label'), /specialization not in the save/);
});

test('there is no decree preview strip: the plaque shows the active decree of the term, with no sample label, and the Term and Decrees section sits in its place (#923)', async () => {
  const posts = [];
  const ui = await open(synthetic(), { post: (url, body) => { posts.push(url); return { ok: true }; } });
  assert.equal(ui.doc.querySelector('.lcc-bd-preview'), null);
  assert.equal(ui.doc.querySelector('#lcc-bd-decree'), null);
  assert.equal(ui.doc.querySelector('[data-lcb="preview"], [data-lcb="toterm"]'), null);
  assert.doesNotMatch(text(ui.doc.querySelector('.lcc-bd')), /DECREE PREVIEW|Sample decree only|as a sample/);
  assert.equal(ui.doc.querySelector('.lcc-plaque [class*="demo"]'), null, 'no sample badge on the plaque');
  assert.equal(text(ui.doc.querySelector('.lcc-pl-title')), 'Exclusive Vehicle Import Rights', 'the plaque reads the active decree of the term');
  const slot = ui.doc.querySelector('.lcc-bd > [data-lcc-slot="term"]');
  assert.ok(slot, 'the slot is where the strip was');
  assert.equal(slot.querySelectorAll('#lcc-term').length, 1, 'the section is drawn in it');
  assert.equal(ui.doc.querySelectorAll('#lcc-term').length, 1, 'and once on the page');
  assert.equal(slot.previousElementSibling.classList.contains('lcc-plaque-host'), true, 'directly under the plaque');
  assert.equal(posts.length, 0, 'nothing was sent');
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
  const dim = tiles(ui).filter((c) => c.querySelector('.lcc-hc').dataset.dim === 'true');
  assert.equal(dim.length, 16);
  assert.equal(tiles(ui).length, 25, 'the board never loses a tile');
  assert.deepEqual(tiles(ui).map((c) => Number(c.dataset.index)), [...Array(25).keys()], 'and never moves one');
  assert.match(dim[0].querySelector('button').getAttribute('aria-label'), /not in the current filter/);
  assert.equal(ui.doc.querySelector('[data-fk="bf:hs:open"]').getAttribute('aria-pressed'), 'true');
  assert.equal(ui.doc.querySelector('[data-fk="bf:hs:any"]').getAttribute('aria-pressed'), 'false');
  const sel = ui.doc.querySelector('#lcc-bd-spec'); sel.value = 'combat'; sel.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.match(text(ui.doc.querySelector('#lcc-bd-shown')), /^\d of 25 Houses shown$/);
  for (const c of tiles(ui)) if (c.querySelector('.lcc-hc').dataset.dim !== 'true') assert.match(c.querySelector('.lcc-hc-track').textContent, /Combat/);
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
  assert.match(i.querySelector('.lcc-bd-insp-name').getAttribute('title'), /^No\. 08/, 'the position is a tooltip, not a chip');
  assert.equal(i.querySelector('.lcc-bd-no'), null);
  assert.equal(i.querySelector('.lcc-bd-status'), null, 'no sentence under the band: the approved design has none');
  assert.equal(text(i.querySelector('.lcc-bd-gopath')), 'Exploration Specialization', 'the specialization is one line, and it is the button that opens its track (#939)');
  assert.equal(i.querySelector('.lcc-bd-bloc'), null, 'no separate caption and no repeated words');
  assert.match(i.querySelector('.lcc-bd-control').getAttribute('title'), /^Claimed by Atreides \(completed, with a winner the save records\)\./);
  assert.match(text(i.querySelector('.lcc-bd-banner .lcc-bd-sr')), /^Claimed by Atreides/, 'and the words stay for a screen reader');
  const bars = [...i.querySelectorAll('.lcc-bd-fc')].map((b) => [text(b.querySelector('.lcc-bd-fc-l')), text(b.querySelector('.lcc-bd-fc-v'))]);
  assert.deepEqual(bars, [['HOUSE ATREIDES', '4,000 / 4,000'], ['HOUSE HARKONNEN', '900 / 4,000']]);
  const s1 = i.querySelector('#lcc-bd-s1');
  assert.match(s1.getAttribute('title'), /House goal: 4,000\..*Top contributors: You 3,600 · Contributor 50 1,000\. Missions: Not in the save\./, 'what the three removed lines said stays as a tooltip');
  assert.match(text(s1.parentNode.querySelector('.lcc-bd-sr')), /Top contributors: You 3,600 · Contributor 50 1,000\. Missions: Not in the save\./);
  assert.equal(s1.parentNode.querySelectorAll('.lcc-bd-cap, .lcc-bd-none').length, 0, 'no visible note lines under the two bars');
  assert.match(i.querySelector('.lcc-bd-track-me').getAttribute('aria-label'), /^Your contribution: 3,600 of 6,000 · next tier 4,800, 1,200 more\./);
  assert.equal(i.querySelector('#lcc-bd-s2').parentNode.querySelector('.lcc-bd-line'), null, 'no line under the contribution bar');
  const order = [...i.children].map((c) => c.getAttribute('aria-labelledby') || c.className.split(' ')[0]);
  assert.deepEqual(order, ['lcc-bd-insp-top', 'lcc-bd-back', 'lcc-bd-identity', 'lcc-bd-s4', 'lcc-bd-pin', 'lcc-bd-banner', 'lcc-bd-s1', 'lcc-bd-s2', 'lcc-bd-s3', 'lcc-bd-disc', 'lcc-bd-s5'], 'the order of the approved design, except that the actions block comes directly under the name (and the way back to the board is before it); the note is a disclosure');
  assert.equal(i.querySelector('.lcc-bd-disc[data-key="note"] > summary').textContent, 'Important: three separate states');
  assert.match(text(i.querySelector('.lcc-bd-note')), /^Important: A House claimed by a faction/);
  assert.doesNotMatch(text(i), /sysselraad/i, 'the Inspector holds no sysselraad data');
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
  assert.equal(i.querySelector('.lcc-bd-ledger .lcc-bd-line'), null, 'no sentence under the four counts');
  assert.match(i.querySelector('.lcc-bd-stats').getAttribute('aria-label'), /3 of 5 reached · 1 collected · 1 not collected \(waiting at the house representative\) · 1 delivery unknown · 2 not reached\./, 'the counts stay for a screen reader and as a tooltip');
  assert.match(text(i), /Delivery unknown.{1,20}means the tier is reached and the save has no delivery row/);
  const st = [...i.querySelectorAll('.lcc-bd-tier-st')].map((s) => text(s));
  assert.deepEqual(st.slice(0, 3), ['Reached, collected', 'Reached, delivery unknown', 'Reached, not collected']);
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
  const writes = [...ui.doc.querySelectorAll('.lcc-bd [data-lb]')].filter((e) => !e.closest('[data-bar]') && !e.closest('#lcc-term')).map((e) => e.dataset.lb); // the parts of the action bar the page seats in the control bar are the page's own (#914)
  assert.deepEqual([...new Set(writes)].sort(), ['map', 'pick', 'qa', 'tier']); // map: the place line of the panel is the Classic click that opens the Saved map (#939)
  assert.deepEqual([...new Set([...ui.doc.querySelectorAll('#lcc-term [data-lb]')].map((e) => e.dataset.lb))].sort(), ['tad', 'tapply', 'tflag', 'tmk', 'trf'], 'the Term and Decrees section keeps the classic panel own controls');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd [data-lcb="sel"]').length, 25);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd .lcc-bd-act[data-lb="qa"]').length, 4);
  assert.deepEqual([...ui.doc.querySelectorAll('.lcc-bd .lcc-bd-act[data-lb="qa"]')].map((b) => b.dataset.do), ['fill', 'fillgoal', 'complete', 'reopen']);
});

test('the board panel has the approved head, filters, legend, totals and the two panels beside it, in the approved order', async () => {
  const ui = await open();
  const b = ui.doc.querySelector('.lcc-bd');
  const order = ['.lcc-plaque-host', '#lcc-term', '.lcc-bd-metrics', '.lcc-bd-panel', '.lcc-bd-victory', '.lcc-bd-insp'];
  const at = order.map((q) => { const e = b.querySelector(q); assert.ok(e, q + ' is missing'); return [...b.querySelectorAll('*')].indexOf(e); });
  assert.deepEqual([...at].sort((x, y) => x - y), at, 'plaque, Term and Decrees section, term row, board panel, victory analysis, House Intelligence');
  const head = b.querySelector('.lcc-bd-panel-head');
  assert.equal(text(head.querySelector('.lcc-bd-eyebrow')), 'THE GREAT HOUSES');
  assert.equal(text(head.querySelector('.lcc-bd-h')), 'The Landsraad');
  assert.match(text(head.querySelector('.lcc-bd-subline')), /Fixed 5 × 5 board · game positions read from board_index/);
  assert.deepEqual([...head.querySelectorAll('.lcc-bd-layout button')].map((x) => text(x)), ['Grid', 'List']);
  assert.ok([...head.querySelectorAll('button')].some((x) => text(x) === 'Clear highlight'));
  assert.deepEqual([...ui.doc.querySelectorAll('#lcc-bd-spec option')].map((o) => text(o)), ['All specializations', 'Crafting', 'Gathering', 'Exploration', 'Combat', 'Sabotage']);
  assert.deepEqual([...b.querySelectorAll('.lcc-bd-headlines li')].map((x) => text(x)), ['ATREIDES CLAIMED', 'HARKONNEN CLAIMED', 'OPEN', 'WINNING HOUSES · SYSSELRAAD WON']);
  assert.deepEqual([...b.querySelectorAll('[data-lcb="filter"][data-key="hs"]')].map((x) => text(x)), ['Any', 'Open', 'Claimed']);
  assert.match(text(b.querySelector('.lcc-bd-totals')), /Your rewards on this board: 26 of 120 tiers reached/);
  assert.match(text(b.querySelector('[data-metric="lines"]')), /^POTENTIAL LINES4Unblocked five-House alignments · Atreides 4 · Harkonnen 0$/, 'the unblocked lines of the twelve, counted by the evaluator (M4)');
  assert.match(text(b.querySelector('[data-metric="cap"]')), /RULE/, 'the cap is a game rule, tagged as one, not read from the save');
});

test('the House cards use the approved system fonts only (nothing is fetched), and the reward boxes use the approved colours', () => {
  const css = read('internal', 'web', 'static', 'lcc', 'lcc.css') + read('internal', 'web', 'static', 'lcc', 'lcc-board.css');
  assert.ok(css.includes("--lcc-serif: Georgia, 'Times New Roman', serif;"));
  assert.ok(css.includes("--lcc-sans: 'Segoe UI Variable', 'Segoe UI', Tahoma, Arial, sans-serif;"));
  assert.ok(css.includes('--lcc-mono: Consolas, monospace;'));
  assert.doesNotMatch(css, /@font-face|@import|url\(/);
  const board = read('internal', 'web', 'static', 'lcc', 'lcc-board.css');
  assert.match(board, /\.lcc-hc-name \{[^}]*var\(--lcc-serif\)/, 'House names are in the serif face of the approved design');
  assert.match(board, /\.lcc-hc-count \{[^}]*var\(--lcc-sans\)/);
  for (const [k, v] of [['ok', '#68c6a8'], ['wait', '#f5c878'], ['unk', '#9b563b']]) assert.ok(css.includes('--lcc-' + k + '-bg: ' + v), k + ' box colour');
});

test('a House name with markup stays text on the card, and the tier boxes of a card without a host are pictures, not controls', async () => {
  const a = synthetic();
  a.tasks[0].house_name = '<b onclick=alert(1)>x</b>';
  const ui = await open(a);
  assert.equal(ui.doc.querySelectorAll('.lcc-hc b[onclick], .lcc-hc [onclick]').length, 0);
  const first = tiles(ui)[0];
  assert.ok(text(first.querySelector('.lcc-hc-name')).includes('<b onclick'), 'shown as the text it is');
  assert.equal(first.querySelectorAll('.lcc-hc-t').length, 5);
});

test('the board header does not say which faction the character is in, and still says why nothing can be changed for a character in none (#930)', async () => {
  const ui = await open();
  assert.doesNotMatch(text(ui.doc.querySelector('#lsb-board')), /Your character is in (Atreides|Harkonnen)/);
  assert.ok(ui.doc.querySelector('#lsb-fac option'), 'the faction box still names the faction');
  const classic = await open(synthetic(), { view: 'classic' });
  assert.ok(classic.doc.querySelector('#lsb-fac'), 'the classic list is as it was');
  const none = synthetic();
  none.character.faction_id = null;
  const ui2 = await open(none);
  assert.match(text(ui2.doc.querySelector('#lsb-board .line1')), /Your character is in no faction/);
});
