// The Command Center board on the real page (M3, stage B): the fixed five-by-five board, the list, the House Inspector and the filters. The page code is the real app.js with the
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

test('the board is the only Landsraad view and the default: nothing remembered, or the retired Classic choice remembered, draws the board, and there is no view switch (#1028)', async () => {
  for (const local of [{ tab: 'landsraad' }, { tab: 'landsraad', 'lsb.view': 'classic' }, { tab: 'landsraad', 'lsb.view': 'board' }]) {
    const ui = await boot({ get: { '/api/landsraad': synthetic() }, local });
    await ui.settle();
    assert.equal(tiles(ui).length, 25, JSON.stringify(local));
    assert.equal(ui.doc.querySelector('#lsb-rows'), null, 'no Classic rows');
    assert.equal(ui.doc.querySelector('.viewsw'), null, 'no view switch');
    assert.equal(ui.doc.querySelector('[data-lb="view"]'), null);
    closeAll();
  }
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
    assert.match(b.getAttribute('aria-label'), /^Square \d\d, /);
    assert.match(c.querySelector('.lcc-hc-owner').textContent, /Pledged to Atreides|Pledged to Harkonnen|Undecided|Wavering|Favor unreadable/);
  }
  assert.equal(text(tiles(ui)[0].querySelector('.lcc-hc-owner')), 'Pledged to Harkonnen', 'position 01 is Vernius, pledged to Harkonnen in the fixture');
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
  assert.doesNotMatch(text(own), /(?<!favor |not )\bwon\b|won the vote/i, 'a lead in the count is not called a win: nobody holds a line of five or 13 Houses');
  // the row of four figures under the plaque (term status, open Houses, potential lines, specialization cap) is gone (#935): the state of the term is on the plaque, the open Houses are in its tally line,
  // the cap is on the Specializations tab, and the lines each faction can still make are one line of the Imperial Victory Analysis
  assert.equal(ui.doc.querySelector('.lcc-bd-metrics'), null, 'no row of four figures');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-metric, [data-metric]').length, 0, 'none of the four figures');
  assert.doesNotMatch(text(ui.doc.querySelector('.lcc-bd')), /TERM STATUS|OPEN HOUSES|POTENTIAL LINES|SPECIALIZATION CAP/);
  assert.match(text(ui.doc.querySelector('.lcc-bd-victory')), /SYSSELRAAD CONTESTED.*ATREIDES LEAD 12 TO 4 · 9 HOUSES OPEN/);
  assert.doesNotMatch(text(ui.doc.querySelector('.lcc-bd-victory')), /Atreides 12 · Harkonnen 4 · Open 9|House count:/, 'the count is said once, in the headline (#1052)');
  assert.equal(text(p.querySelector('.lcc-pl-session')), 'SYSSELRAAD CONTESTED');
  assert.match(text(p.querySelector('.lcc-pl-held')), /^HELD BY\s*HOUSE ATREIDES$/);
  assert.equal(text(p.querySelector('.lcc-pl-title')), 'Exclusive Vehicle Import Rights');
  assert.equal(p.querySelectorAll('.lcc-standard-none').length, 0, 'the save holds contribution rows under both factions, so both banners list them');
  assert.match(text(p.querySelector('.lcc-f-a .lcc-standard-list > li')), /^SYNTHETIC_CHARACTER$/, 'the character is the biggest contributor under Atreides in the fixture, named by the character name of the save, with no amount (#1043)');
});

test('a House whose owner cannot be read is counted for neither side and the plaque says so', async () => {
  const a = synthetic();
  a.tasks[10].completed = 1; a.tasks[10].winning_faction_id = null; // taken with no winner: not readable
  const ui = await open(a);
  const p = ui.doc.querySelector('.lcc-plaque-host');
  assert.match(text(p.querySelector('.lcc-pl-standing')), /COUNT INCOMPLETE · 1 HOUSE UNREADABLE/);
  assert.equal(text(tiles(ui)[10].querySelector('.lcc-hc-owner')), '? Favor unreadable');
  assert.match(text(ui.doc.querySelector('.lcc-bd-headlines')), /FAVOR UNREADABLE/);
  assert.equal(ui.doc.querySelector('.lcc-bd-vlines'), null, 'with a House that cannot be read there is no count of lines and no line about it (#1052)');
});

test('a board that is not a board (24 Houses) is not drawn as one: a read-only list, the reasons, and nothing that writes', async () => {
  const a = synthetic();
  a.tasks.pop();
  const ui = await open(a);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-tiles').length, 0);
  assert.equal(ui.doc.querySelector('.lcc-bd').getAttribute('data-ready'), 'false');
  assert.match(text(ui.doc.querySelector('.lcc-bd-problems')), /24 of 25 Houses/);
  assert.doesNotMatch(text(ui.doc.querySelector('.lcc-bd-problems')), /classic list/i, 'no Classic list to send anyone to (#1028)');
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

test('a card shows five numbered tier boxes and the line N/5 EARNED with the collected and waiting counts, and says them in its name for a screen reader', async () => {
  const ui = await open();
  const w = tileOf(ui, 'Wallach');
  const boxes = [...w.querySelectorAll('.lcc-hc-t')];
  assert.equal(boxes.length, 5);
  assert.deepEqual(boxes.map((b) => text(b)), ['◆', '◆', '◆', '4', '5'], 'a waiting reward is a diamond in a gold box (the card boxes keep their look, #1082), a tier not reached keeps its number');
  assert.deepEqual(boxes.map((b) => b.classList.contains('waiting')), [true, true, true, false, false]);
  assert.equal(text(w.querySelector('.lcc-hc-count')), '3/5 EARNED', 'one line on the card, no count line under it (#1078)');
  assert.equal(w.querySelectorAll('.lcc-hc-q').length, 0);
  assert.match(w.querySelector('.lcc-hc-count b').getAttribute('title'), /Collected: 0\. Not collected \(waiting at the representative\): 3\./, 'the counts are in the hover');
  assert.match(w.querySelector('button.lcc-hc-select').getAttribute('aria-label'), /^Square 08, Wallach, pledged to Atreides, specialization Exploration, 3 of 5 tiers earned, 0 collected, 3 rewards not collected$/);
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
  assert.equal(text(w.querySelector('.lcc-hc-count')), '3/5 EARNED');
  assert.match(w.querySelector('.lcc-hc-count b').getAttribute('title'), /Collected: 1\. Not collected \(waiting at the representative\): 1\. With no delivery record, so in neither count: 1\./, 'the unknown one is counted on its own, in neither the collected nor the waiting figure');
  assert.match(w.querySelector('button.lcc-hc-select').getAttribute('aria-label'), /1 with no delivery record/);
});

test('each card carries the specialization emblem of its mapped path in the upper right (the clean emblem picture of the path, no frame), and a House without one has none and says so in words', async () => {
  const ui = await open();
  const w = tileOf(ui, 'Wallach');
  const badge = w.querySelector('.lcc-hc-top .lcc-hc-spec');
  assert.ok(badge, 'the badge is in the top row, after the number');
  assert.equal(badge.dataset.track, 'exploration');
  assert.equal(badge.getAttribute('title'), 'Exploration specialization, Level —', 'the level is a dash until the Specializations tab has read it (#646)');
  assert.equal(badge.querySelector('img').getAttribute('src'), '/lcc/art/emblems/exploration.png', 'the clean emblem cut from the banner, not the banner with its shadow and side copies (#976), not a screenshot crop (#963)');
  assert.equal(badge.querySelector('img').getAttribute('width'), '28');
  assert.equal(badge.querySelector('img').getAttribute('alt'), '', 'decorative: the path is also written under the name');
  assert.equal(text(w.querySelector('.lcc-hc-track')), 'Exploration');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-tiles .lcc-hc-spec').length, 25);
  const a = synthetic();
  a.tasks[7].line = 'nothing-known';
  const un = await open(a);
  const c = tileOf(un, 'Wallach');
  assert.equal(c.querySelector('.lcc-hc-spec'), null, 'no emblem when the save gives no path');
  assert.equal(text(c.querySelector('.lcc-hc-track')), '—', 'a dash; the reason is the hover text (#1056)');
  assert.equal(c.querySelector('.lcc-hc-track').getAttribute('title'), 'Specialization not in the save');
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
  const said = () => text(ui.doc.querySelector('[data-lk="shownsr"]'));
  assert.equal(said(), '25 of 25 Houses shown', 'the strip always says the count (tau #974, #1118: 25 of 25 when nothing narrows the board); the live region stays');
  assert.equal(ui.doc.querySelector('#lcc-bd-shown').getAttribute('role'), 'status', 'the live region stays');
  click(ui, '[data-lk="fbtn"]');
  click(ui, '[data-fk="bf:hs:open"]');
  assert.equal(said(), '9 of 25 Houses shown');
  const dim = tiles(ui).filter((c) => c.querySelector('.lcc-hc').dataset.dim === 'true');
  assert.equal(dim.length, 16);
  assert.equal(tiles(ui).length, 25, 'the board never loses a tile');
  assert.deepEqual(tiles(ui).map((c) => Number(c.dataset.index)), [...Array(25).keys()], 'and never moves one');
  assert.match(dim[0].querySelector('button').getAttribute('aria-label'), /not in the current filter/);
  assert.equal(ui.doc.querySelector('[data-fk="bf:hs:open"]').getAttribute('aria-checked'), 'true');
  assert.equal(ui.doc.querySelector('[data-fk="bf:hs:any"]').getAttribute('aria-checked'), 'false');
  click(ui, '[data-lk="cat"][data-cat="spec"]'); click(ui, '[data-lk="spick"][data-val="combat"]');
  assert.match(said(), /^\d of 25 Houses shown$/);
  for (const c of tiles(ui)) if (c.querySelector('.lcc-hc').dataset.dim !== 'true') assert.match(c.querySelector('.lcc-hc-track').textContent, /Combat/);
});

test('Find House (the jump of the control area, tau #974) finds a House the filters dim: it outlines it, keeps the filters as they were, ticks nothing and opens no panel', async () => {
  const ui = await open();
  let scrolled = 0;
  ui.w.HTMLElement.prototype.scrollIntoView = function () { scrolled++; };
  click(ui, '[data-fk="bf:hs:taken"]'); // the House filter is Pledged
  click(ui, '[data-lk="findbtn"]');
  const sor = [...ui.doc.querySelectorAll('[data-lk="fpick"]')].find((o) => text(o) === 'Sor');
  assert.ok(sor, 'the entry is the House name only, without the square number (#979)');
  click(ui, sor);
  const cell = tileOf(ui, 'Sor');
  assert.ok(scrolled >= 1, 'the board scrolled to the House');
  assert.ok(cell.classList.contains('lcc-k-flash'), 'the House is outlined whatever the filters were');
  assert.equal(ui.doc.querySelector('[data-fk="bf:hs:taken"]').getAttribute('aria-checked'), 'true', 'the filter is as it was: Find House filters nothing');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-pick:checked').length, 0, 'it ticks nothing');
  assert.match(text(ui.doc.querySelector('.lcc-bd-insp-name')), /Sor/, 'it chooses the House for the panel, as a click on the card does (#1147)');
});

test('choosing a House opens the Inspector: both scales apart, the contributors and missions as "Not in the save", no sysselraad', async () => {
  const ui = await open();
  click(ui, tileOf(ui, 'Wallach').querySelector('button'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  assert.equal(text(i.querySelector('.lcc-bd-insp-name')), 'House Wallach');
  assert.match(i.querySelector('.lcc-bd-insp-name').getAttribute('title'), /^Square 08/, 'the position is a tooltip, not a chip');
  assert.equal(i.querySelector('.lcc-bd-no'), null);
  assert.equal(i.querySelector('.lcc-bd-status'), null, 'no sentence under the band: the approved design has none');
  assert.equal(text(i.querySelector('.lcc-bd-gopath')), 'Exploration Specialization', 'the specialization is one line, and it is the button that opens its track (#939)');
  assert.equal(i.querySelector('.lcc-bd-bloc'), null, 'no separate caption and no repeated words');
  assert.match(i.querySelector('.lcc-bd-control').getAttribute('title'), /^Pledged to House Atreides \(completed, with a faction the save records\)\./);
  assert.match(text(i.querySelector('.lcc-bd-banner .lcc-bd-sr')), /^Pledged to House Atreides/, 'and the words stay for a screen reader');
  const bars = [...i.querySelectorAll('.lcc-bd-fc')].map((b) => [text(b.querySelector('.lcc-bd-fc-l')), text(b.querySelector('.lcc-bd-fc-v'))]);
  assert.deepEqual(bars, [['HOUSE ATREIDES', '4,000 / 4,000'], ['HOUSE HARKONNEN', '900 / 4,000']]);
  const s1 = i.querySelector('#lcc-bd-s1');
  assert.match(s1.getAttribute('title'), /House goal: 4,000\..*Missions: Not in the save\./, 'what the three removed lines said stays as a tooltip, with no contributor names (#1077)');
  assert.doesNotMatch(s1.getAttribute('title'), /contributors/i);
  assert.match(text(s1.parentNode.querySelector('.lcc-bd-sr')), /Missions: Not in the save\./);
  assert.equal(i.querySelector('.lcc-bd-contrib'), null, 'the Inspector has no contributors block (#1077)');
  assert.doesNotMatch(text(i), /TOP CONTRIBUTORS/);
  assert.equal(s1.parentNode.querySelectorAll('.lcc-bd-cap, .lcc-bd-none').length, 0, 'no visible note lines under the two bars');
  assert.match(i.querySelector('.lcc-bd-track-me').getAttribute('aria-label'), /^Your contribution: 3,600 of 6,000 · next tier 4,800, 1,200 more\./);
  assert.equal(i.querySelector('#lcc-bd-s2').parentNode.querySelector('.lcc-bd-line'), null, 'no line under the contribution bar');
  const order = [...i.children].map((c) => c.getAttribute('aria-labelledby') || c.className.split(' ')[0]);
  assert.deepEqual(order, ['lcc-bd-insp-top', 'lcc-bd-back', 'lcc-bd-identity', 'lcc-bd-s4', 'lcc-bd-pin', 'lcc-bd-banner', 'lcc-bd-s1', 'lcc-bd-s2', 'lcc-bd-s3', 'lcc-bd-s5'], 'the order of the approved design, except that the actions block comes directly under the name (and the way back to the board is before it); no note or disclosure under the ledger (#980)');
  assert.equal(i.querySelector('.lcc-bd-disc, .lcc-bd-note'), null, 'the note of the three separate states is in the Help tab');
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
  assert.match(i.querySelector('.lcc-bd-stats').getAttribute('aria-label'), /3 of 5 tiers earned · 1 of 3 earned tiers collected, 1 waiting at the house representative, 1 with no delivery record\./, 'the counts stay for a screen reader and as a tooltip');
  assert.match(text(i), /Delivery unknown.{1,20}means the tier is earned and the save has no delivery row/);
  const st = [...i.querySelectorAll('.lcc-bd-tier-st')].map((s) => text(s));
  assert.deepEqual(st.slice(0, 3), ['Earned, collected', 'Earned, delivery unknown', 'Earned, not collected']);
});

test('the list: House name A to Z by default (#953), sortable by every column both ways, the Square column included, grouped by region with the unmapped last', async () => {
  const ui = await open(synthetic(), { layout: 'list' });
  const names = () => [...ui.doc.querySelectorAll('.lcc-bd-row .lcc-bd-namebtn')].map((b) => text(b));
  const nos = () => [...ui.doc.querySelectorAll('.lcc-bd-row .lcc-bd-c-no')].map((b) => text(b));
  assert.equal(ui.doc.querySelector('.lcc-bd-tiles'), null);
  assert.deepEqual(names(), [...names()].sort((x, y) => x.localeCompare(y)), 'it opens sorted by the House name, A to Z');
  assert.equal(ui.doc.querySelector('th[aria-sort="ascending"]').textContent.trim().startsWith('House'), true);
  click(ui, '[data-fk="bs:house"]');
  assert.deepEqual(names(), [...names()].sort((x, y) => y.localeCompare(x)));
  click(ui, '[data-fk="bs:position"]');
  assert.deepEqual(nos(), [...Array(25).keys()].map((i) => String(i + 1).padStart(2, '0')), 'the Square column sorts by the position on the board');
  assert.equal(ui.doc.querySelector('th[aria-sort]').textContent.trim().startsWith('Square'), true);
  click(ui, '[data-fk="bs:house"]');
  assert.deepEqual(names(), [...names()].sort((x, y) => x.localeCompare(y)));
  click(ui, '[data-fk="bs:house"]');
  assert.deepEqual(names(), [...names()].sort((x, y) => y.localeCompare(x)));
  assert.equal(ui.doc.querySelector('th[aria-sort="descending"]').textContent.trim().startsWith('House'), true);
  click(ui, '[data-lk="fbtn"]'); click(ui, '[data-lk="cat"][data-cat="group"]'); click(ui, '[data-lk="gpick"][data-val="region"]'); // Group is in the Filter menu now (tau #974)
  const heads = [...ui.doc.querySelectorAll('.lcc-bd-grouprow th')].map((h) => text(h));
  assert.deepEqual(heads, ['Arrakeen · 1', 'Deep Desert · 1', 'Hagga Basin · 1', 'Place unknown · 22']);
  assert.equal(ui.doc.querySelector('.lcc-bd-list').getAttribute('role'), null, 'a real table, not a pile of divs');
  assert.ok(ui.doc.querySelector('.lcc-bd-list caption'));
});

test('the layout is remembered, and the list carries a Choose box and a Saved map link only where the map has a picture', async () => {
  const ui = await open();
  click(ui, '[data-fk="bf:layout:list"]');
  assert.equal(ui.w.localStorage.getItem('lsb.layout'), 'list');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-row .lcc-bd-pick').length, 25);
  const maps = [...ui.doc.querySelectorAll('.lcc-bd-row .lcc-bd-loc [data-lb="map"]')];
  assert.deepEqual(maps.map((m) => m.closest('tr').querySelector('.lcc-bd-namebtn').textContent).sort(), ['Alexin', 'Kenola', 'Lindaren'].filter((n) => maps.some((m) => m.closest('tr').querySelector('.lcc-bd-namebtn').textContent === n)));
  for (const m of maps) assert.match(m.getAttribute('aria-label'), /Open in the Saved map$/);
});

test('every control that changes the save carries the page\'s own data-lb, so one handler, one guard and one queue serve the board', async () => {
  const ui = await open();
  click(ui, tileOf(ui, 'Wallach').querySelector('button'));
  const writes = [...ui.doc.querySelectorAll('.lcc-bd [data-lb]')].filter((e) => !e.closest('[data-bar]') && !e.closest('#lcc-term')).map((e) => e.dataset.lb); // the parts of the action bar the page seats in the control bar are the page's own (#914)
  assert.deepEqual([...new Set(writes)].sort(), ['map', 'pick', 'qa', 'tier']); // map: the place line of the panel is the Classic click that opens the Saved map (#939)
  assert.deepEqual([...new Set([...ui.doc.querySelectorAll('#lcc-term [data-lb]')].map((e) => e.dataset.lb))].sort(), ['tad', 'tapply', 'tflag', 'tflags', 'tmk', 'trf'], 'the Term and Decrees section keeps the classic panel own controls');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd [data-lcb="sel"]').length, 25);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd .lcc-bd-act[data-lb="qa"]').length, 4);
  assert.deepEqual([...ui.doc.querySelectorAll('.lcc-bd .lcc-bd-act[data-lb="qa"]')].map((b) => b.dataset.do), ['fill', 'fillgoal', 'complete', 'reopen']);
});

test('the board panel has the approved head, filters, legend, totals and the two panels beside it, in the approved order', async () => {
  const ui = await open();
  const b = ui.doc.querySelector('.lcc-bd');
  const order = ['.lcc-plaque-host', '#lcc-term', '.lcc-bd-panel', '.lcc-bd-victory', '.lcc-bd-insp'];
  const at = order.map((q) => { const e = b.querySelector(q); assert.ok(e, q + ' is missing'); return [...b.querySelectorAll('*')].indexOf(e); });
  assert.deepEqual([...at].sort((x, y) => x - y), at, 'plaque, Term and Decrees section, board panel, victory analysis, House Intelligence');
  const head = b.querySelector('.lcc-bd-panel-head');
  assert.equal(text(head.querySelector('.lcc-bd-eyebrow')), 'THE GREAT HOUSES');
  assert.equal(text(head.querySelector('.lcc-bd-h')), 'The Landsraad');
  assert.match(text(head.querySelector('.lcc-bd-subline')), /^Term [0-9]+$/, 'only the term (#1033)');
  assert.doesNotMatch(text(head), /Fixed 5 × 5|board_index/);
  assert.deepEqual([...head.querySelectorAll('.lcc-bd-layout button')].map((x) => text(x)), ['Grid', 'List']);
  assert.ok(![...head.querySelectorAll('button')].some((x) => /Clear highlight|Strategy/.test(text(x))), 'no Clear highlight and no Sysselraad Strategy button (#958)');
  assert.deepEqual([...ui.doc.querySelectorAll('#lcc-bd-menu [data-lk="spick"]')].map((o) => text(o)), ['All', 'Crafting', 'Gathering', 'Exploration', 'Combat', 'Sabotage'], 'the Specialization choices moved from the panel head into the Filter menu');
  assert.deepEqual([...b.querySelectorAll('.lcc-bd-headlines li')].map((x) => text(x)), ['PLEDGED TO ATREIDES', 'PLEDGED TO HARKONNEN', 'UNDECIDED', 'WINNING HOUSES · SYSSELRAAD SECURED']);
  assert.deepEqual([...b.querySelectorAll('[data-lcb="filter"][data-key="hs"]')].map((x) => x.firstChild.textContent), ['All', 'Undecided', 'Pledged']);
  assert.equal(b.querySelector('.lcc-bd-totals'), null, 'no standing line under the board (#1062)');
  assert.equal(b.querySelector('.lcc-bd-foot'), null);
  assert.match(b.querySelector('#lcc-bd-h').getAttribute('title'), /Your rewards on this board: 26 of 120 tiers earned/, 'the sentence is the hover text of the heading');
  assert.equal(text(b.querySelector('.lcc-bd-victory .lcc-bd-vlines')), 'Lines of five still possible: Atreides 4 · Harkonnen 0', 'the unblocked lines of the twelve, counted by the evaluator (M4), in the Imperial Victory Analysis (#935)');
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
  const none = synthetic();
  none.character.faction_id = null;
  const ui2 = await open(none);
  assert.match(text(ui2.doc.querySelector('#lsb-board .line1')), /Your character is in no faction/);
});

test('the card count reads N/5 EARNED with a tooltip, and no card or board count says reached (#962)', async () => {
  const ui = await open();
  const w = tileOf(ui, 'Wallach');
  const b = w.querySelector('.lcc-hc-count b');
  assert.equal(text(b), '3/5 EARNED');
  assert.match(b.getAttribute('title'), /^Reward tiers of yours earned in this House: 3 of 5\./);
  for (const c of ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc')) { // #1078: every card's line is the one figure, never a count line under it
    assert.equal(c.querySelectorAll('.lcc-hc-q').length, 0, 'no count line under the figure');
    assert.match(text(c.querySelector('.lcc-hc-count')), /^(\d\/\d EARNED|NO TIERS|Pledged - no tier record in the save)$/);
  }
  for (const c of ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc')) {
    assert.doesNotMatch(text(c.querySelector('.lcc-hc-count')), /REACHED/i, 'card count');
    const said = (c.querySelector('.lcc-hc-select').getAttribute('aria-label') || '') + ' ' + [...c.querySelectorAll('.lcc-hc-count [title]')].map((x) => x.getAttribute('title')).join(' ');
    assert.doesNotMatch(said, /tiers? (is |are )?reached|reached with no delivery/, 'card screen-reader text and count tooltips');
  }
  assert.doesNotMatch(ui.doc.querySelector('#lcc-bd-h').getAttribute('title'), /reached/, 'the hover text of the heading');
});

test('earned and collected are the one pair of words on the list, the inspector, the note and the hovers; only the state word Not reached stays (#962)', async () => {
  const list = await open(synthetic(), { layout: 'list' });
  const rows = [...list.doc.querySelectorAll('.lcc-bd-row')];
  const wall = rows.find((r) => text(r.querySelector('.lcc-bd-namebtn')) === 'Wallach');
  assert.equal(text(wall.querySelector('.lcc-bd-c-tiers')), '3 of 5 tiers earned');
  const tops = rows.map((r) => text(r.querySelector('.lcc-bd-c-next'))).filter((t) => /top tier/.test(t));
  for (const t of tops) assert.equal(t, 'top tier earned');
  for (const r of rows) {
    for (const cls of ['lcc-bd-c-state', 'lcc-bd-c-tiers', 'lcc-bd-c-next']) assert.doesNotMatch([text(r.querySelector('.' + cls)), r.querySelector('.' + cls).getAttribute('title') || ''].join(' '), /tiers? reached|top tier reached|\(reached/i, cls);
  }
  const ui = await open();
  click(ui, tileOf(ui, 'Wallach').querySelector('button'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  assert.equal(i.querySelector('.lcc-bd-ledger-n'), null, 'no N OF 5 EARNED tag: the Tiers earned tile says it (#1059)');
  assert.deepEqual([...i.querySelectorAll('.lcc-bd-stat span')].map(text)[0], 'Tiers earned');
  assert.equal(i.querySelector('.lcc-bd-note'), null, 'the note of the three separate states is in the Help tab (#980)');
  const said = [text(i), ...[...i.querySelectorAll('[title], [aria-label]')].map((e) => (e.getAttribute('title') || '') + ' ' + (e.getAttribute('aria-label') || ''))].join(' ');
  assert.doesNotMatch(said.replace(/not reached|NOT REACHED|Not reached/g, ''), /\breached\b/i, 'the inspector says earned, never reached (the state word Not reached stays)');
  assert.equal(i.querySelector('.lcc-bd-legend'), null, 'the legend of the marks is in the Help tab (#980)');
});

test('#1048: a faction with no contribution row in the save reads 0 / goal in the Inspector, never "not known / goal"; the difference is only in the hover text', async () => {
  const a = synthetic();
  const t = a.tasks[7];
  t.contributions = (t.contributions || []).filter((c) => c.faction_id !== 1);
  const ui = await open(a);
  click(ui, tileOf(ui, 'Wallach').querySelector('button'));
  const i = ui.doc.querySelector('.lcc-bd-insp');
  const bars = [...i.querySelectorAll('.lcc-bd-fc')].map((b) => [text(b.querySelector('.lcc-bd-fc-l')), text(b.querySelector('.lcc-bd-fc-v'))]);
  const atreides = bars.find((b) => b[0] === 'HOUSE ATREIDES');
  assert.match(atreides[1], /^0 \/ [\d,]+$/, JSON.stringify(bars));
  assert.doesNotMatch(text(i), /not known \//);
  const v = [...i.querySelectorAll('.lcc-bd-fc-v')][0];
  assert.equal(v.getAttribute('title'), 'No record of this faction in the save for this House.');
  assert.match(v.getAttribute('aria-label'), /^HOUSE ATREIDES 0 of [\d,]+$/);
  const other = [...i.querySelectorAll('.lcc-bd-fc-v')][1];
  assert.equal(other.getAttribute('title'), null, 'a faction with a row has no such hover');
});

test('#1059: list cells stay short: an em dash for a House with no place, "N of 5" with the words for a screen reader, and no "no map picture" text on the page', async () => {
  const a = synthetic();
  const ui = await open(a, { view: 'board', layout: 'list' });
  const rows = [...ui.doc.querySelectorAll('.lcc-bd-row')];
  assert.equal(rows.length, 25);
  const css = (el) => [...el.querySelectorAll('.lcc-bd-sr')].map(text);
  for (const r of rows) {
    const t = r.querySelector('.lcc-bd-c-tiers');
    const visible = text(t).replace(/ tiers earned$/, '');
    assert.match(visible, /^(\d of \d|no tier record|no tiers)$/, 'the cell shows "N of 5": ' + text(t));
    if (/\d of \d/.test(text(t))) assert.deepEqual(css(t), ['tiers earned'], 'the words for a screen reader are kept');
    assert.match(text(t), /^(\d of \d tiers earned|no tier record|no tiers)$/, 'the cell still says the whole thing as text: ' + text(t));
  }
  for (const r of rows) {
    const l = r.querySelector('.lcc-bd-loc');
    const mut = l.querySelector('.lcc-bd-dash');
    if (mut) {
      assert.equal(mut.firstChild.textContent, '—', 'no place is an em dash');
      assert.deepEqual(css(mut), ['not mapped yet']);
      assert.match(mut.getAttribute('title'), /^Not mapped yet/);
    }
    const np = l.querySelector('.lcc-bd-nopic');
    if (np) { assert.deepEqual(css(np), ['· no map picture']); assert.match(np.getAttribute('title'), /no picture of|outside the picture of/); }
  }
});

test('#1057: no source labels (FROM THE SAVE, SAVE, SAVE + DERIVED) on the board, the empty or opened Inspector and the Term section, and no table, field or save-number words in their tooltips', async () => {
  const ui = await open();
  const check = (what) => {
    const root = ui.doc.querySelector('#lsb');
    assert.equal(root.querySelectorAll('.lcc-bd-src, .lcc-bd-source, .lcc-tm-src, .lcc-badge').length, 0, what + ': no label chip');
    assert.doesNotMatch(text(root), /FROM THE SAVE|SAVE [+] DERIVED|WORKED OUT HERE/, what + ': no source words');
    const said = [...root.querySelectorAll('[title], [aria-label], .lcc-bd-sr')].map((e) => (e.getAttribute('title') || '') + ' ' + (e.getAttribute('aria-label') || '') + ' ' + (e.classList.contains('lcc-bd-sr') ? e.textContent : '')).join(' | ');
    assert.doesNotMatch(said, /landsraad_|Read from the save|save number|identifier the save|winning_faction_id|board_index|player_contributions/, what + ': no table, field or save-number words in a tooltip');
  };
  check('the empty Inspector');
  click(ui, tileOf(ui, 'Wallach').querySelector('button'));
  assert.ok(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-sec-h'));
  check('an opened Inspector');
  for (const h of ui.doc.querySelectorAll('.lcc-bd-insp .lcc-bd-sec-h')) assert.equal(h.children.length, 0, 'a section heading is only its words: ' + text(h));
});
