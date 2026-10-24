// The page keeps its place across a redraw and across a refresh (#365): the scroll offset of the page and of a scrolling table, a faction chosen in a box, a ticked box,
// text typed in a filter box and an opened list. A move to another tab starts at the top again.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const landsraad = () => ({
  term: { term_id: 7, start_time: 1, end_time: 2, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 },
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }],
  rotation: [],
  tasks: Array.from({ length: 30 }, (_, i) => ({ id: 100 + i, board_index: i + 1, house_name: 'DA_HouseX' + i, goal_amount: 50, contributions: [], completed: 0, rewards: [{ threshold: 10, amount: 3, template_id: 'Item.A' }] })),
});

// jsdom has no layout, so the page's own scroll offset is a number the test holds and the calls to scrollTo move it
function holdWindowScroll(w, y) {
  let at = y;
  Object.defineProperty(w, 'scrollY', { get: () => at, configurable: true });
  w.scrollTo = (x, yy) => { at = yy; };
  return () => at;
}

async function openLandsraad(extra = {}) {
  const ui = await boot({ get: { '/api/landsraad': landsraad() }, post: { '/api/landsraad/progress': { ok: true } }, ...extra });
  if (!ui.doc.querySelector('#lf')) { ui.click('[data-tab="landsraad"]'); await ui.settle(); }
  assert.ok(ui.doc.querySelector('#lf'), 'the Landsraad tab is open');
  return ui;
}

test('an action on the Landsraad tab keeps the scroll offsets, the chosen faction and the opened list', async () => {
  const ui = await openLandsraad();
  const y = holdWindowScroll(ui.w, 640);
  const box = [...ui.doc.querySelectorAll('#main .scroll')].pop();
  box.scrollTop = 175;
  ui.doc.querySelector('#lf').value = '2';
  const det = ui.doc.querySelector('#main details'); det.open = true;
  const before = ui.doc.querySelector('#lf');

  ui.click('[data-act="tfill"][data-id="105"]'); await ui.settle();
  const sent = ui.posted.filter((p) => p.path === '/api/landsraad/progress');
  assert.equal(sent.length, 1);
  assert.equal(sent[0].body.faction_id, 2, 'the faction that was chosen is the one sent');
  assert.notEqual(ui.doc.querySelector('#lf'), before, 'the tab was drawn again');
  assert.equal(ui.doc.querySelector('#lf').value, '2', 'the faction stays chosen');
  assert.equal([...ui.doc.querySelectorAll('#main .scroll')].pop().scrollTop, 175, 'the table keeps its offset');
  assert.equal(y(), 640, 'the page keeps its offset');
  assert.equal(ui.doc.querySelector('#main details').open, true, 'the opened list stays open');
});

test('a ticked box and typed text are put back on a redrawn page, but a value the save changed is shown', async () => {
  const ui = await openLandsraad();
  const main = ui.doc.querySelector('#main');
  const page = (stack) => `<div class="card"><label><input type="checkbox" id="tk"> tick</label><input type="text" id="tx" value="old">
    <input type="number" data-item="9" data-field="stack_size" value="${stack}" aria-label="Stack"><input type="number" data-item="9" data-field="quality" value="1" aria-label="Quality">
    <select id="sl"><option value="a" selected>A</option><option value="b">B</option></select></div>`;
  main.innerHTML = page(5);
  ui.doc.querySelector('#tk').checked = true;
  ui.doc.querySelector('#tx').value = 'typed';
  ui.doc.querySelector('[aria-label="Stack"]').value = '7'; // typed, not yet saved
  ui.doc.querySelector('[aria-label="Quality"]').value = '3';
  ui.doc.querySelector('#sl').value = 'b';
  const snap = ui.w.viewSnapshot();
  main.innerHTML = page(9); // the save now holds 9 in the stack box
  ui.w.viewApply(snap);
  assert.equal(ui.doc.querySelector('#tk').checked, true);
  assert.equal(ui.doc.querySelector('#tx').value, 'typed');
  assert.equal(ui.doc.querySelector('#sl').value, 'b');
  assert.equal(ui.doc.querySelector('[aria-label="Quality"]').value, '3', 'the box is told apart from the one beside it');
  assert.equal(ui.doc.querySelector('[aria-label="Stack"]').value, '9', 'a box whose value the save changed shows the new value');
});

test('a move to another tab starts at the top', async () => {
  const ui = await openLandsraad();
  holdWindowScroll(ui.w, 500);
  [...ui.doc.querySelectorAll('#main .scroll')].pop().scrollTop = 99;
  ui.click('[data-tab="config"]'); await ui.settle();
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  assert.equal([...ui.doc.querySelectorAll('#main .scroll')].pop().scrollTop, 0, 'a tab opened again starts at the top');
});

test('a refresh of the page comes back to the same tab, offsets and choices', async () => {
  const first = await openLandsraad();
  holdWindowScroll(first.w, 880);
  [...first.doc.querySelectorAll('#main .scroll')].pop().scrollTop = 210;
  first.doc.querySelector('#lf').value = '2';
  first.doc.querySelector('#main details').open = true;
  first.w.dispatchEvent(new first.w.Event('pagehide'));
  const saved = first.w.sessionStorage.getItem('tabr-tau-view');
  assert.ok(saved, 'the place was kept when the page was left');
  const tabKept = first.w.localStorage.getItem('tab');
  closeAll();

  const w2 = { session: { 'tabr-tau-view': saved }, local: { tab: tabKept } };
  let scrolledTo = null;
  const ui = await boot({ get: { '/api/landsraad': landsraad() }, ...w2, pre: (w) => { w.scrollTo = (x, y) => { scrolledTo = y; }; } });
  assert.equal(scrolledTo, 880, 'the page is scrolled to where it was');
  assert.ok(ui.doc.querySelector('#lf'), 'the same tab is open after the refresh');
  assert.equal(ui.doc.querySelector('#lf').value, '2', 'the chosen faction is back');
  assert.equal([...ui.doc.querySelectorAll('#main .scroll')].pop().scrollTop, 210, 'the table is where it was');
  assert.equal(ui.doc.querySelector('#main details').open, true);
});

test('a place in the session storage that is not of the right shape is ignored', async () => {
  for (const bad of ['not json', '{"tab":"landsraad"}', '{"tab":"landsraad","y":"x","boxes":[],"ctl":{},"det":{},"sub":{}}', '[]', 'null']) {
    const ui = await boot({ get: { '/api/landsraad': landsraad() }, session: { 'tabr-tau-view': bad }, local: { tab: 'landsraad' } });
    assert.ok(ui.doc.querySelector('#lf'), 'the page starts as usual: ' + bad);
    assert.equal(ui.w.sessionStorage.getItem('tabr-tau-view'), null, 'the stored place is used once');
    closeAll();
  }
});
