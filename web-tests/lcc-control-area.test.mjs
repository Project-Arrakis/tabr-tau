// The Command Center control area of the 5x5 board (tau #974, "E6 v2"): one sticky strip with two modes in the same place. Nothing chosen: the Show tabs, the Filter key, the Find House key, one chip per
// active filter category and the count. Houses chosen: the action bar (count, the two crests, Fill, Complete, Reopen, Clear). Real page code and the synthetic answer of M1; layout is not
// measured here (jsdom has none), the sizes are read from the style file and the headless pictures in the pull request.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const CSS = fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', 'lcc', 'lcc-board.css'), 'utf8');
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const key = (ui, el, k) => { el.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true })); };
const panel = (ui) => ui.doc.querySelector('.lcc-bd-panel');
const picks = (ui) => [...ui.doc.querySelectorAll('.lcc-bd-pick')];
const ticked = (ui) => picks(ui).filter((p) => p.checked).length;
const chips = (ui) => [...ui.doc.querySelectorAll('.lcc-k-strip [data-lk="chips"] .lcc-k-chip')].map((c) => text(c).trim());
async function open(layout = 'grid') {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, post: {}, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': layout } });
  await ui.settle();
  return ui;
}
const tick = (ui, n) => { for (let i = 0; i < n; i++) { const b = picks(ui).filter((p) => !p.disabled && !p.checked)[0]; assert.ok(b, 'no box to tick'); click(ui, b); } };
const openMenu = (ui) => click(ui, '[data-lk="fbtn"]');
const done = (ui) => click(ui, '[data-lk="popclose"]');

test('nothing chosen: the strip shows Show tabs, Filter, Find House and the count, and no action bar', async () => {
  const ui = await open();
  const p = panel(ui);
  assert.equal(p.dataset.mode, 'filters');
  assert.equal(p.dataset.state, 'idle');
  const strip = p.querySelector('.lcc-k-strip');
  assert.ok(strip, 'no strip');
  assert.deepEqual([...strip.querySelectorAll('[role="radio"][data-key="show"]')].map((b) => text(b)), ['All', 'What is left', 'Not started', 'In progress', 'Top tier']);
  assert.ok(strip.querySelector('[data-lk="fbtn"]'), 'no Filter key');
  assert.ok(strip.querySelector('[data-lk="findbtn"]'), 'no Find House key');
  assert.equal(text(strip.querySelector('#lcc-bd-shown [data-lk="nums"]')), 'All 25 shown', 'nothing narrows the board: no fraction that looks like a choice (#1116)');
  assert.equal(ui.doc.querySelectorAll('.lcc-k-strip [data-lk="chips"] .lcc-k-chip').length, 0, 'a chip with nothing filtered');
});

test('one square ticked: the strip becomes the action bar; Clear swaps back with the filters exactly as they were', async () => {
  const ui = await open();
  openMenu(ui);
  click(ui, '[data-lcb="filter"][data-key="hs"][data-val="open"]');
  done(ui);
  const before = chips(ui);
  assert.deepEqual(before, ['House: Undecided']);
  tick(ui, 1);
  const p = panel(ui);
  assert.equal(p.dataset.mode, 'action');
  assert.equal(p.dataset.state, 'chosen');
  const bar = p.querySelector('.lcc-k-cmd');
  assert.equal(text(bar.querySelector('[data-lk="n"]')), '1');
  assert.match(text(bar.querySelector('.lcc-k-chosen')), /chosen/);
  assert.equal(bar.querySelectorAll('.lcc-k-seats [role="radio"]').length, 2, 'two faction crests');
  for (const d of ['fill', 'complete', 'reopen']) assert.ok(bar.querySelector('[data-lb="bulk"][data-do="' + d + '"]'), 'no ' + d + ' key');
  assert.ok(bar.querySelector('[data-lb="pickno"]'), 'no Clear key');
  assert.ok(p.querySelector('.lcc-k-strip [data-lk="findbtn"]') || p.querySelector('.lcc-k-cmd [data-lk="findbtn"]'), 'Find House stays as a small key');
  click(ui, bar.querySelector('[data-lb="pickno"]'));
  assert.equal(p.dataset.mode, 'filters');
  assert.equal(ticked(ui), 0);
  assert.deepEqual(chips(ui), before, 'the filters are as they were');
});

test('Escape on the action bar clears the choice and swaps back', async () => {
  const ui = await open();
  tick(ui, 2);
  assert.equal(panel(ui).dataset.mode, 'action');
  key(ui, picks(ui)[0], 'Escape');
  assert.equal(panel(ui).dataset.mode, 'filters');
  assert.equal(ticked(ui), 0);
});

test('one chip per active filter category, each with its category word', async () => {
  const ui = await open();
  openMenu(ui);
  click(ui, '[data-lcb="filter"][data-key="hs"][data-val="open"]');
  assert.deepEqual(chips(ui), ['House: Undecided']);
  click(ui, '[data-lcb="filter"][data-key="hs"][data-val="taken"]');
  assert.deepEqual(chips(ui), ['House: Pledged']);
  click(ui, '[data-lk="cat"][data-cat="spec"]');
  click(ui, '[data-lk="spick"][data-val="combat"]');
  assert.deepEqual(chips(ui), ['House: Pledged', 'Specialization: Combat']);
  click(ui, '[data-lk="spick"][data-val="crafting"]');
  assert.equal(chips(ui)[1], 'Specialization: Crafting + Combat');
  click(ui, '[data-lk="spick"][data-val="exploration"]');
  assert.equal(chips(ui)[1], 'Specialization: 3 chosen', 'three or more say how many');
  assert.equal(chips(ui).length, 2, 'one chip per category, not one per item');
  click(ui, '[data-lk="spick"][data-val="all"]');
  assert.deepEqual(chips(ui), ['House: Pledged']);
});

test('Group is off in the grid and says why; in the list layout it gives a chip', async () => {
  const ui = await open();
  openMenu(ui);
  click(ui, '[data-lk="cat"][data-cat="group"]');
  assert.match(text(ui.doc.querySelector('#lcc-bd-menu')), /List layout only/);
  closeAll();
  const lu = await open('list');
  openMenu(lu);
  click(lu, '[data-lk="cat"][data-cat="group"]');
  click(lu, '[data-lk="gpick"][data-val="region"]');
  assert.deepEqual(chips(lu), ['Group: By region']);
});

test('the Houses in the Filter menu tick squares and make no chip', async () => {
  const ui = await open();
  openMenu(ui);
  const items = [...ui.doc.querySelectorAll('#lcc-bd-menu [data-lk="hpick"]')];
  assert.equal(items.length, 25, '25 Houses as tickable items');
  click(ui, items[0]);
  click(ui, items[1]);
  assert.equal(ticked(ui), 2, 'two squares ticked on the board');
  assert.equal(chips(ui).length, 0, 'no chip for a House');
  assert.equal(ui.doc.querySelector('[data-lk="fcnt"]').hidden, true, 'the Filter key counts no filter');
  const p = panel(ui);
  assert.equal(p.dataset.mode, 'filters', 'the strip swaps when the menu closes, not before');
  done(ui);
  assert.equal(p.dataset.mode, 'action');
  assert.equal(text(p.querySelector('.lcc-k-cmd [data-lk="n"]')), '2');
});

test('Find House scrolls to a House and outlines it; it ticks nothing and changes no filter', async () => {
  const ui = await open();
  let scrolled = 0;
  ui.w.HTMLElement.prototype.scrollIntoView = function () { scrolled++; };
  click(ui, '[data-lk="findbtn"]');
  const pick = ui.doc.querySelector('[data-lk="fpick"]');
  assert.ok(pick, 'no list');
  const id = pick.getAttribute('data-id');
  click(ui, pick);
  assert.equal(ticked(ui), 0, 'nothing ticked');
  assert.equal(chips(ui).length, 0, 'no filter');
  assert.equal(panel(ui).dataset.mode, 'filters');
  assert.ok(scrolled >= 1, 'the board scrolled to the House');
  const cell = ui.doc.querySelector('.lcc-bd-pick[data-id="' + id + '"]').closest('.lcc-bd-cell');
  assert.ok(cell.classList.contains('lcc-k-flash'), 'the House is outlined');
});

test('the outline of Find House is an outline only (no motion) and goes after 1.8 s', () => {
  assert.match(CSS, /\.lcc-bd-cell\.lcc-k-flash > \.lcc-hc[^{]*\{[^}]*outline: 3px solid/);
  assert.doesNotMatch(CSS.match(/\.lcc-bd-cell\.lcc-k-flash[^{]*\{[^}]*\}/g).join(''), /animation|transition/);
});

test('a filter picked in the menu drops the chosen Houses it hides (the rule of #881 and #1019); a stale choice the page was not told about is counted as not shown and is not acted on', async () => {
  const ui = await open();
  tick(ui, 3);
  const p = panel(ui);
  assert.equal(text(p.querySelector('.lcc-k-cmd [data-lk="n"]')), '3');
  // a filter picked in the Filter menu: the Houses it hides leave the choice, so the count is the Houses that are chosen and shown
  click(ui, '[data-lk="fbtn"]');
  click(ui, '[data-lcb="filter"][data-key="hs"][data-val="taken"]');
  const left = [...ui.doc.querySelectorAll('.lcc-bd-pick:checked:not(:disabled)')].length;
  assert.ok(left < 3, 'the filter hid none of the chosen Houses; the test needs a filter that does');
  assert.equal(ui.w.LCC_HOST.picks().size, left, 'a hidden House is out of the choice');
  done(ui);
  // a choice the page was not told about (the filter changed behind its back): the strip says how many are not shown and the count holds only the shown ones
  const ui2 = await open();
  tick(ui2, 3);
  ui2.w.LCCB.state.hs = 'taken';
  ui2.w.LCCB.redraw();
  const shown = [...ui2.doc.querySelectorAll('.lcc-bd-pick:checked:not(:disabled)')].length;
  const hidden = ui2.w.LCC_HOST.picks().size - shown;
  assert.ok(hidden > 0, 'the filter hid none of the stale choice');
  assert.equal(text(panel(ui2).querySelector('.lcc-k-cmd [data-lk="n"]')), String(shown), 'only the Houses that are chosen and shown are counted');
  assert.match(text(panel(ui2).querySelector('.lcc-k-strip')), new RegExp(hidden + ' (chosen, )?not shown'));
});

test('25 squares are one tab stop; the arrow keys move it', async () => {
  const ui = await open();
  const boxes = picks(ui).filter((b) => !b.disabled);
  assert.ok(boxes.length >= 5);
  assert.equal(boxes.filter((b) => b.tabIndex === 0).length, 1, 'exactly one box in the tab order');
  assert.equal(picks(ui).filter((b) => b.tabIndex !== 0 && b.getAttribute('tabindex') !== '-1').length, 0, 'every other box is -1');
  const first = boxes.find((b) => b.tabIndex === 0);
  key(ui, first, 'ArrowRight');
  const now = boxes.find((b) => b.tabIndex === 0);
  assert.notEqual(now, first, 'the stop moved');
  assert.equal(boxes.filter((b) => b.tabIndex === 0).length, 1);
  key(ui, now, 'ArrowLeft');
  assert.equal(boxes.find((b) => b.tabIndex === 0), first);
});

test('the words of the control area are the agreed ones', async () => {
  const ui = await open();
  openMenu(ui);
  const menu = text(ui.doc.querySelector('#lcc-bd-menu'));
  for (const w of ['House', 'Pledge', 'All', 'Undecided', 'Pledged', 'Specialization', 'Group', 'Done']) assert.ok(menu.includes(w), 'the menu lacks ' + w);
  assert.ok(ui.doc.querySelector('#lcc-bd-menu input[type="search"]'), 'no Search Houses box');
});

test('style: tick boxes are 16-18 px in a 36 px hit area (44 px on a phone) and have a 2 px border of the text colour', () => {
  assert.match(CSS, /\.lcc-bd-pick \{[^}]*width: 18px; height: 18px[^}]*border: 2px solid var\(--lcc-gold-text\)/);
  assert.match(CSS, /\.lcc-bd-tiles \.lcc-bd-pickl[^{]*\{[^}]*width: 36px; height: 36px/);
  assert.match(CSS, /@media \(max-width: 640px\), \(pointer: coarse\) \{ \.lcc-bd-tiles \.lcc-bd-pickl[^{]*\{[^}]*width: 44px; height: 44px/);
});

test('style: the control area sets no text under 11 px (12 px for names), in the kit rules or the app rules', () => {
  // the kit rules start at .lcc-k-page; a rule of the List layout that only mentions a kit icon (the Fill and Complete keys of a row) is not the kit
  const kit = CSS.slice(CSS.indexOf('.lcc-k-page {'));
  const sizes = [];
  for (const m of kit.matchAll(/(?:font-size:\s*|font:\s*(?:\d{3}\s+|italic\s+|normal\s+)*)(\d+(?:\.\d+)?)px/g)) sizes.push(Number(m[1]));
  assert.ok(sizes.length > 20, 'the sizes were not read');
  assert.ok(Math.min(...sizes) >= 11, 'a text under 11 px: ' + Math.min(...sizes));
  for (const m of kit.matchAll(/\.lcc-k-(?:chip|tab|opt|btn|fbtn|bl)\b[^{,]*\{[^}]*?(?:font-size:\s*|font:\s*(?:\d{3}\s+)?)(\d+(?:\.\d+)?)px/g)) assert.ok(Number(m[1]) >= 12, 'a name under 12 px: ' + m[0].slice(0, 80));
});

test('style: the phone has a sticky filter row of about 56 px and an action bar of about 104 px, and the Show tabs scroll away', () => {
  assert.match(CSS, /a phone: the sticky part is one 56 px row/);
  assert.match(CSS, /\.lcc-k-e6 \.lcc-k-tray-out \{[^}]*margin-top/);
  assert.match(CSS, /\.lcc-k-e6 \.lcc-k-strip \{ top: calc\(var\(--hdr-h, 56px\)/);
});

test('style: the 320 px rule that wrapped the label of Choose shown is gone with the key (#1116)', () => {
  assert.doesNotMatch(CSS, /@media \(max-width: 380px\)/);
  assert.doesNotMatch(CSS, /lcc-k-cg \.lcc-k-link/);
});

// ---- the review of the real-browser pass (tau #974): Cancel, the whole question, the Show tabs while Houses are chosen, a fresh board, the More disclosure ----
const state = (ui) => panel(ui).dataset.state;
const fkOf = (ui) => (ui.doc.activeElement && ui.doc.activeElement.getAttribute ? ui.doc.activeElement.getAttribute('data-fk') : '');
const chosenIds = (ui) => [...ui.w.LCC_HOST.picks()];

test('Cancel, Escape and a stale question give the strip back its action keys at once, and the focus goes to the key that asked (Fill, or More for Complete and fill and Unfill)', async () => {
  const ui = await open();
  tick(ui, 3);
  click(ui, '.lcc-k-cmd [data-do="fill"]');
  await ui.settle();
  assert.equal(state(ui), 'confirm');
  click(ui, '[data-fk="c-no"]');
  assert.equal(state(ui), 'chosen', 'the strip is synced when the question is cancelled (the keys are hidden while it says confirm)');
  assert.equal(fkOf(ui), 'b-fill');
  // Complete and fill sits in the closed More menu: the focus goes to the More key
  click(ui, '.lcc-k-cmd [data-lk="more"]');
  assert.equal(ui.doc.querySelector('.lcc-k-moremenu').hidden, false);
  click(ui, '.lcc-k-cmd [data-do="cfill"]');
  await ui.settle();
  assert.equal(state(ui), 'confirm');
  assert.equal(ui.doc.querySelector('.lcc-k-moremenu').hidden, true, 'a key of the More disclosure closes it');
  click(ui, '[data-fk="c-no"]');
  assert.equal(state(ui), 'chosen');
  assert.equal(fkOf(ui), 'b-more');
  assert.equal(ui.doc.querySelector('.lcc-k-cmd [data-lk="more"]').getAttribute('aria-expanded'), 'false');
  // Escape on a question
  click(ui, '.lcc-k-cmd [data-do="complete"]');
  await ui.settle();
  assert.equal(state(ui), 'confirm');
  ui.doc.activeElement.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
  assert.equal(state(ui), 'chosen');
  assert.equal(fkOf(ui), 'b-complete');
  assert.equal(ui.posted.length, 0, 'cancelling writes nothing');
});

test('the question is not clamped: the Houses, the Houses left alone and the backup sentence are all in the text under the heading, and no style clamps the lines', async () => {
  const ui = await open();
  tick(ui, 6);
  click(ui, '.lcc-k-cmd [data-do="fill"]');
  await ui.settle();
  const qd = ui.doc.querySelector('.lcc-k-ask .lcc-k-qd');
  assert.ok(qd, 'the detail of the question');
  assert.match(text(qd), /Houses: /);
  assert.match(text(qd), /Written at once, with a backup first\./);
  const rule = CSS.match(/\.lcc-k-e6 \.lcc-k-ask \.lcc-k-qd \{[^}]*\}/)[0];
  assert.doesNotMatch(rule, /line-clamp|display: -webkit-box|overflow: hidden/);
  assert.match(rule, /overflow-y: auto/, 'a very long list scrolls in its own box instead of being cut');
});

test('the Show tabs are off while Houses are chosen, on every width, with the reason in the title, and a press changes nothing', async () => {
  const ui = await open();
  const tabs = () => [...ui.doc.querySelectorAll('.lcc-k-tab[data-key="show"]')];
  assert.ok(tabs().every((t) => t.getAttribute('aria-disabled') === null));
  tick(ui, 4);
  assert.ok(tabs().every((t) => t.getAttribute('aria-disabled') === 'true'), 'off in the action bar');
  assert.match(tabs()[0].getAttribute('title'), /Not available while Houses are chosen/);
  const was = chosenIds(ui).length;
  click(ui, tabs().find((t) => t.getAttribute('aria-checked') === 'false'));
  assert.equal(chosenIds(ui).length, was, 'the choice is not dropped by a press on an off tab');
  assert.equal(ui.w.LCCB.state.show, 'all');
  click(ui, '.lcc-k-cmd [data-lb="pickno"]');
  assert.ok(tabs().every((t) => t.getAttribute('aria-disabled') === null && t.getAttribute('title') === null), 'back on when the choice is cleared');
  click(ui, tabs().find((t) => t.getAttribute('aria-checked') === 'false'));
  assert.notEqual(ui.w.LCCB.state.show, 'all', 'and a press works again');
});

test('a returning board starts with the Filter menu and Find House closed (the state belongs to one board)', async () => {
  const ui = await open();
  openMenu(ui);
  assert.equal(ui.doc.querySelector('#lcc-bd-menu').hidden, false);
  click(ui, '[data-tab="player"]'); await ui.settle();
  click(ui, '[data-tab="landsraad"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#lcc-bd-menu').hidden, true, 'the menu does not pop open again');
  tick(ui, 1);
  assert.equal(panel(ui).dataset.mode, 'action', 'and Houses chosen are not forced back into the filter row by a stale open menu');
});

test('More is a disclosure (aria-expanded, aria-controls, ordinary buttons), not a menu; Escape closes it and gives the focus back to More', async () => {
  const ui = await open();
  tick(ui, 2);
  const more = ui.doc.querySelector('.lcc-k-cmd [data-lk="more"]'), menu = ui.doc.querySelector('.lcc-k-moremenu');
  assert.equal(more.getAttribute('aria-haspopup'), null);
  assert.equal(more.getAttribute('aria-controls'), menu.id);
  assert.equal(menu.getAttribute('role'), 'group');
  assert.equal(menu.querySelectorAll('[role="menuitem"]').length, 0);
  click(ui, more);
  assert.equal(more.getAttribute('aria-expanded'), 'true');
  const first = menu.querySelector('button');
  first.focus();
  key(ui, first, 'Escape');
  assert.equal(menu.hidden, true);
  assert.equal(more.getAttribute('aria-expanded'), 'false');
  assert.equal(ui.doc.activeElement, more);
});

test('style: Clear keeps its word at the panel width of the design (about 860 px) and is the X alone only below 780 px', () => {
  assert.match(CSS, /@container kA \(max-width: 779px\) \{ \.lcc-k-e6 \.lcc-k-cmd \[data-lb="pickno"\] > span:not\(\.lcc-k-tip\)/);
  assert.doesNotMatch(CSS, /@container kA \(max-width: 959px\) \{ \.lcc-k-e6 \.lcc-k-cmd \[data-lb="pickno"\]/);
});
