// Milestone M6 on the real page: the Term and Decrees section, Sysselraad Strategy on the board, and the way between the board, the term section and Specializations. Real app.js and lcc scripts, the
// synthetic answer of M1 (no game save is read). What the term controls write is compared with the classic panel in lcc-term-requests.test.mjs.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir } from './lcc-load.mjs';

afterEach(closeAll);
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : '');
async function open(answer = synthetic(), local = {}) {
  const ui = await boot({ get: { '/api/landsraad': answer }, local: Object.assign({ tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' }, local) });
  await ui.settle();
  return ui;
}
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };

test('the term section tells where each fact comes from: the save, and what the save does not hold', async () => {
  const ui = await open();
  const s = ui.doc.querySelector('#lcc-term');
  assert.ok(s);
  assert.match(text(s), /FROM THE SAVE/);
  assert.match(text(s), /NOT IN THE SAVE/i);
  assert.doesNotMatch(text(s), /\bwins?\b|Dominion/i, 'no claim of a result');
  assert.equal(s.querySelector('h3#lcc-tm-h').getAttribute('tabindex'), '-1');
  assert.ok(s.querySelector('#lsb-tresult[role="status"]'));
});

test('every decree of the pool has a text alternative for its picture, its effect or the plain words that it is not in the save, and its pool flag as a button', async () => {
  const a = synthetic();
  const ui = await open(a);
  const rows = [...ui.doc.querySelectorAll('#lcc-term .lcc-tm-row')];
  assert.equal(rows.length, (a.decrees || []).length);
  for (const r of rows) {
    const img = r.querySelector('img');
    if (img) assert.ok(img.getAttribute('alt') !== null, 'an image without alt');
    assert.ok(r.querySelector('[data-lb="tflag"]'), 'the pool flag control');
    assert.ok(r.querySelector('.lcc-tm-de') && text(r.querySelector('.lcc-tm-de')).length > 3, 'an effect or the plain words');
  }
});

test('Make active is off for the decree that is active, and says so', async () => {
  const a = synthetic();
  const ui = await open(a);
  const act = ui.doc.querySelector('#lcc-term .lcc-tm-row[data-decree="7"]');
  assert.ok(act, 'the row of the active decree');
  assert.equal(act.querySelector('[data-lb="tmk"]'), null);
  assert.match(text(act.querySelector('.lcc-tm-mkon')), /Active/);
  assert.ok(ui.doc.querySelector('#lcc-term .lcc-tm-row[data-decree="4"] [data-lb="tmk"]'), 'another row can be made active');
});

test('the board has a button to the term section that moves the focus to its heading and writes nothing', async () => {
  const ui = await open();
  click(ui, '[data-lcb="toterm"]');
  assert.equal(ui.doc.activeElement.id, 'lcc-tm-h');
  assert.equal(ui.posted.length, 0);
});

test('Sysselraad Strategy is off until chosen; on, it lists twelve lines with a word for each, and a line is chosen and let go again', async () => {
  const ui = await open();
  assert.equal(ui.doc.querySelector('.lcc-bd-strat-panel'), null);
  assert.equal(ui.doc.querySelector('[data-lcb="strategy"]').getAttribute('aria-pressed'), 'false');
  click(ui, '[data-lcb="strategy"]');
  const panel = ui.doc.querySelector('.lcc-bd-strat-panel');
  assert.ok(panel);
  const rows = [...panel.querySelectorAll('[data-lcb="line"]')];
  assert.equal(rows.length, 12);
  for (const r of rows) assert.match(text(r), /FIVE IN A LINE|ONE AWAY|POTENTIAL|BLOCKED|UNREADABLE/);
  assert.doesNotMatch(text(panel), /\bwins\b|\bwon\b|Dominion (is|has)|Dominion confirmed/i);
  const id = rows[0].dataset.pline;
  click(ui, rows[0]);
  assert.equal(ui.doc.querySelector('[data-pline="' + id + '"]').getAttribute('aria-pressed'), 'true');
  assert.ok(ui.doc.querySelector('.lcc-bd-lfoot .lcc-bd-lcur'));
  click(ui, '[data-lcb="clearline"]');
  assert.equal(ui.doc.querySelector('[data-pline="' + id + '"]').getAttribute('aria-pressed'), 'false');
  assert.equal(ui.posted.length, 0, 'nothing is written by looking at a line');
});

test('a chosen line marks its Houses on the board, a House the line misses is jumped to, and nothing is sent', async () => {
  const ui = await open();
  click(ui, '[data-lcb="strategy"]');
  const pot = [...ui.doc.querySelectorAll('[data-lcb="line"]')].find((b) => b.dataset.kind === 'potential' || b.dataset.kind === 'oneaway');
  assert.ok(pot, 'a line that is not held');
  click(ui, pot);
  assert.ok(ui.doc.querySelectorAll('.lcc-hc[data-route]').length >= 1, 'the Houses of the line are marked');
  assert.ok(ui.doc.querySelector('.lcc-hc[data-route-role="missing"]'), 'a missing House is marked');
  const jump = ui.doc.querySelector('[data-lcb="jumpmissing"]');
  assert.ok(jump);
  click(ui, jump);
  assert.equal(ui.posted.length, 0);
});

test('the order of the list can be changed and the board is not reordered', async () => {
  const ui = await open();
  click(ui, '[data-lcb="strategy"]');
  const cells = () => [...ui.doc.querySelectorAll('.lcc-bd-cell')].map((c) => text(c.querySelector('.lcc-hc-name')));
  const before = cells(), ids = () => [...ui.doc.querySelectorAll('[data-lcb="line"]')].map((b) => b.dataset.pline);
  const board = ids();
  click(ui, '[data-lcb="lsort"][data-val="near"]');
  assert.deepEqual([...ids()].sort(), [...board].sort());
  assert.deepEqual(cells(), before, 'the board keeps its order');
});

test('with the classic list view nothing of the Command Center is drawn, and the classic term panel is', async () => {
  const ui = await open(synthetic(), { 'lsb.view': 'classic' });
  assert.equal(ui.doc.querySelector('#lcc-term'), null);
  assert.equal(ui.doc.querySelector('.lcc-bd-strat-panel'), null);
  assert.ok(ui.doc.querySelector('.lsb .tctl'));
});

test('the Inspector of a House with a specialization line opens that line in Specializations, and nothing is written', async () => {
  const ui = await open();
  let went = null;
  for (const c of ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc-select')) {
    click(ui, c);
    const g = ui.doc.querySelector('[data-lcb="gopath"]');
    if (g) { went = g; break; }
  }
  assert.ok(went, 'a House with a path');
  assert.match(text(went), /^Open .+ in Specializations$/);
  click(ui, went);
  await ui.settle();
  assert.equal(ui.posted.length, 0, 'only reads');
  assert.ok(ui.doc.querySelector('#lcs-mount .lcc-sp'), 'the Specializations view is drawn');
  assert.ok(ui.doc.querySelector('.lcc-sp-lane[aria-pressed="true"]'), 'a path is chosen');
});

test('no demo or mock name is in the term section or the strategy list', async () => {
  const ui = await open();
  click(ui, '[data-lcb="strategy"]');
  const t = text(ui.doc.querySelector('#lcc-term')) + ' ' + text(ui.doc.querySelector('.lcc-bd-strat-panel'));
  assert.doesNotMatch(t, /Lorem|John Doe|Warlord|Fixture|Mock/i);
});

test('the term script and stylesheet are linked by the page', () => {
  const idx = fs.readFileSync(path.join(lccDir, '..', 'index.html'), 'utf8');
  assert.match(idx, /lcc\/lcc-term\.css/);
  assert.match(idx, /lcc\/lcc-term\.js/);
});
