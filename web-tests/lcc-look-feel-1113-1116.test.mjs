// Four small look-and-feel fixes on the Landsraad Command Center board (UAT build 510; display only, no request, save field or count changes):
//   #1113 the mouse-over text of a key in the control strip opens under the whole strip, never under its own key, and keeps to the strip's width (wraps)
//   #1114 the Filter menu's House list has an All choice; the pledge row says All where it said Any
//   #1115 the Inspector section and the list column say Location, not Place
//   #1116 the strip has no Choose shown key (All in the Filter menu chooses every shown House); #1118 the count line is always "N / 25 shown" and the action bar "N / 25 chosen"
// jsdom has no layout: the numbers of #1113 are checked as the rule the code applies (inline style on the tip); the real placement is shown by the headless pictures in the pull request.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const over = (ui, el) => { el.dispatchEvent(new ui.w.MouseEvent('mouseover', { bubbles: true, cancelable: true })); };
const picks = (ui) => [...ui.doc.querySelectorAll('.lcc-bd-pick')];
const ticked = (ui) => picks(ui).filter((p) => p.checked).length;
async function open(layout = 'grid') {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, post: {}, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': layout } });
  await ui.settle();
  return ui;
}
const tipOf = (el) => [...el.children].find((c) => c.classList.contains('lcc-k-tip'));

test('#1113: a key of the strip opens its text under the whole strip and keeps to the strip width', async () => {
  const ui = await open();
  const lens = ui.doc.querySelector('.lcc-k-lens');
  for (const sel of ['[data-lk="fbtn"]', '[data-lk="findbtn"]', '#lcc-bd-shown', '.lcc-k-gl']) {
    const host = lens.querySelector(sel);
    assert.ok(host, 'no ' + sel);
    const tip = tipOf(host);
    assert.ok(tip, 'no tip on ' + sel);
    over(ui, host);
    assert.equal(tip.style.top !== '' && tip.style.top !== 'auto', true, sel + ': the tip is placed from the strip bottom (top set)');
    assert.equal(tip.style.bottom, 'auto', sel + ': not above its key');
    assert.equal(tip.style.transform, 'none', sel + ': no centring transform');
    assert.equal(tip.style.whiteSpace, 'normal', sel + ': the text wraps');
    assert.match(tip.style.maxWidth, /^\d+px$/, sel + ': the width is bounded by the strip');
    assert.ok(parseInt(tip.style.maxWidth, 10) <= 352, sel + ': never wider than the tip of the kit');
  }
});

test('#1113: the keys inside the Filter menu keep their own place', async () => {
  const ui = await open();
  click(ui, '[data-lk="fbtn"]');
  const open_ = ui.doc.querySelector('#lcc-bd-menu [data-lcb="filter"][data-val="open"]');
  const tip = tipOf(open_);
  assert.ok(tip, 'the Undecided key has its text');
  over(ui, open_);
  assert.equal(tip.style.top, '', 'no strip placement inside the menu');
  assert.equal(tip.style.whiteSpace, '');
});

test('#1113: the rule is in the style sheet too: a tip is out of the flow and the strip-bound width is the only inline override', async () => {
  const ui = await open();
  const host = ui.doc.querySelector('.lcc-k-lens [data-lk="findbtn"]'), tip = tipOf(host);
  over(ui, host);
  for (const p of ['left', 'top']) assert.match(tip.style[p], /^-?\d+px$/, p + ' is a plain number of pixels');
  assert.equal(tip.style.right, 'auto');
});

test('#1114: the House list starts with All; it ticks every shown House and, ticked, clears the choice; no request is made', async () => {
  const ui = await open();
  click(ui, '[data-lk="fbtn"]');
  const list = ui.doc.querySelector('#lcc-bd-menu .lcc-k-p5-panel[data-cat="house"] .lcc-k-p5-list');
  const first = list.querySelector('.lcc-k-opt');
  assert.equal(text(first), 'All');
  assert.equal(first.getAttribute('data-lk'), 'hall');
  assert.equal(first.getAttribute('aria-checked'), 'false');
  assert.equal(list.querySelectorAll('[data-lk="hpick"]').length, 25, 'the 25 Houses follow');
  click(ui, first);
  assert.equal(ticked(ui), 25, 'every House is ticked');
  assert.equal(ui.doc.querySelector('#lcc-bd-menu').hidden, false, 'the menu stays open');
  assert.equal(first.getAttribute('aria-checked'), 'true');
  assert.equal(text(ui.doc.querySelector('[data-lk="hn"]')), '25 Houses, 25 chosen', 'the All key is not a House in the count');
  click(ui, first);
  assert.equal(ticked(ui), 0, 'ticked again it clears the choice');
  assert.equal(first.getAttribute('aria-checked'), 'false');
  assert.equal(ui.posted.length, 0, 'nothing was written');
});

test('#1114: with a filter on, All ticks only the Houses that are shown; the House search never hides it', async () => {
  const ui = await open();
  click(ui, '[data-lk="fbtn"]');
  click(ui, '#lcc-bd-menu [data-lcb="filter"][data-key="hs"][data-val="taken"]');
  const shown = Number(ui.doc.querySelector('[data-lk="shown"]').textContent);
  assert.ok(shown > 0 && shown < 25, 'the synthetic board has pledged Houses and others: ' + shown);
  const all = ui.doc.querySelector('#lcc-bd-menu [data-lk="hall"]');
  click(ui, all);
  assert.equal(ticked(ui), shown, 'only the shown Houses are ticked');
  const search = ui.doc.querySelector('#lcc-bd-menu [data-lk="hsearch"]');
  search.value = 'zzzz-no-house'; search.dispatchEvent(new ui.w.Event('input', { bubbles: true }));
  assert.equal(all.hidden, false, 'All stays while the search matches no House');
  assert.equal(text(ui.doc.querySelector('[data-lk="hn"]')), 'No House matches the search.');
});

test('#1114: one name: the pledge row, the Specialization row and the House list all say All; the Group row keeps None', async () => {
  const ui = await open('list');
  assert.deepEqual([...ui.doc.querySelectorAll('[data-lcb="filter"][data-key="hs"]')].map((b) => b.firstChild.textContent), ['All', 'Undecided', 'Pledged']);
  assert.equal(ui.w.LCCM.HOUSE_FILTERS[0][0], 'any', 'the stored key of the choice is unchanged');
  assert.equal(text(ui.doc.querySelector('[data-lk="spick"][data-val="all"]')), 'All');
  assert.equal(text(ui.doc.querySelector('#lcc-bd-menu [data-lk="hall"]')), 'All');
  assert.deepEqual([...ui.doc.querySelectorAll('[data-lk="gpick"]')].map((b) => text(b)), ['None', 'By region', 'By specialization']);
  assert.doesNotMatch(text(ui.doc.querySelector('#lcc-bd-menu')), /\bAny\b/, 'no Any left in the menu');
});

test('#1115, #1144: the list column says Location, and the Inspector has no LOCATION section (the place is the button under the House name)', async () => {
  const ui = await open('list');
  const heads = [...ui.doc.querySelectorAll('.lcc-bd-list th')].map((t) => text(t));
  assert.ok(heads.some((h) => /^Location\b/.test(h)), 'the list column is Location: ' + heads.join(' | '));
  assert.ok(!heads.some((h) => /^Place\b/.test(h)), 'no Place column: ' + heads.join(' | '));
  const grid = await open();
  const a = synthetic(), id = a.tasks.find((t) => t.house_name === 'DA_HouseKenola').id;
  click(grid, '[data-fk="ht:' + id + '"]');
  const heads2 = [...grid.doc.querySelectorAll('.lcc-bd-insp .lcc-bd-sec-h')].map((h) => text(h));
  assert.ok(!heads2.includes('LOCATION'), '#1144: no LOCATION section: ' + heads2.join(' | '));
  assert.ok(grid.doc.querySelector('.lcc-bd-insp .lcc-bd-loc[data-lb=map]'), '#1144: the place button under the House name stays');
  assert.equal(grid.doc.querySelector('.lcc-bd-insp .lcc-bd-act[data-lb=map]'), null, '#1144: no Open in the Saved map action');
  assert.ok(!heads2.includes('PLACE'), 'no PLACE heading: ' + heads2.join(' | '));
});

test('#1118: the count line is always "N / 25 shown", 25 / 25 when nothing narrows the board; the action bar says "N / 25 chosen"', async () => {
  const ui = await open();
  const count = () => text(ui.doc.querySelector('#lcc-bd-shown [data-lk="nums"]'));
  assert.equal(count(), '25 / 25 shown');
  assert.doesNotMatch(count(), /\bAll\b/, 'no All special case');
  assert.equal(text(ui.doc.querySelector('[data-lk="shownsr"]')), '25 of 25 Houses shown');
  click(ui, '[data-lcb="filter"][data-key="show"][data-val="part"]');
  const n = Number(ui.doc.querySelector('[data-lk="shown"]').textContent);
  assert.ok(n > 0 && n < 25, 'a filter narrows the board: ' + n);
  assert.equal(count(), n + ' / 25 shown');
  assert.equal(text(ui.doc.querySelector('[data-lk="shownsr"]')), n + ' of 25 Houses shown');
  click(ui, '[data-lcb="filter"][data-key="show"][data-val="all"]');
  assert.equal(count(), '25 / 25 shown', 'and back');
  // the action bar: the number of chosen Houses against all of them
  const first = ui.doc.querySelector('.lcc-bd-pick');
  assert.ok(first, 'a House can be chosen');
  click(ui, first);
  const bar = ui.doc.querySelector('.lcc-k-cmd');
  assert.equal(text(bar.querySelector('[data-lk="n"]')), '1');
  assert.equal(text(bar.querySelector('.lcc-k-chosen')), '/ 25 chosen', 'the bar reads 1 / 25 chosen');
  assert.equal(text(bar.querySelector('[data-lk="nsr"]')), '1 of 25 Houses chosen');
  assert.equal(text(ui.doc.querySelector('#lcc-bd-shown [data-lk="nums"]')), '25 / 25 shown', 'the count line is not the choice');
});

test('#1116: the filter-mode strip has no Choose shown key; All in the Filter menu is the one way to choose every shown House (and it clears when all are ticked)', async () => {
  const ui = await open();
  const strip = ui.doc.querySelector('.lcc-k-strip');
  // nothing of the removed key is left: its words, its data attributes, its part and its slot
  assert.doesNotMatch(text(ui.doc.body), /Choose shown/, 'no Choose shown anywhere on the page');
  assert.equal(strip.querySelector('[data-lb="pickall"], [data-fk="b-all"], [data-lk="chooselabel"], .bp-choose, [data-bar="choose"], [data-lcc-slot="choose"]'), null, 'no key, part or slot of Choose shown');
  assert.equal(ui.doc.querySelector('.lcc-k-lens [data-lb]'), null, 'the filter row has no key of the page that writes or chooses');
  // the count line is as it was
  assert.equal(text(strip.querySelector('#lcc-bd-shown [data-lk="nums"]')), '25 / 25 shown');
  // the Filter menu's All is reachable and named, and chooses the shown Houses with no request
  click(ui, '[data-lk="fbtn"]');
  const first = ui.doc.querySelector('#lcc-bd-menu [data-lk="hall"]');
  assert.equal(text(first), 'All');
  assert.equal(first.getAttribute('role'), 'checkbox', 'named and announced as a check box');
  assert.equal(first.getAttribute('aria-checked'), 'false');
  click(ui, '[data-lcb="filter"][data-key="show"][data-val="part"]');
  const shown = Number(ui.doc.querySelector('[data-lk="shown"]').textContent);
  assert.ok(shown > 0 && shown < 25, 'a filter narrows the board: ' + shown);
  const all = ui.doc.querySelector('#lcc-bd-menu [data-lk="hall"]'); // the menu is drawn again with the filter
  click(ui, all);
  assert.equal(ticked(ui), shown, 'All chooses the shown Houses only');
  assert.equal(all.getAttribute('aria-checked'), 'true', 'and reads as ticked when every shown House is ticked');
  click(ui, all);
  assert.equal(ticked(ui), 0, 'ticked again it clears the choice');
  assert.equal(ui.posted.length, 0, 'nothing was written');
});
