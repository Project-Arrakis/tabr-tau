// Three read-only additions to the Command Center board: the summary line of the term (#440), the character's level in the specialization of a House (#646), and a list whose Place and Next tier cells
// fit (#1103). Real page code with the synthetic answers of M1; no game save. Nothing here writes, and nothing here adds a request.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { read } from './lcc-load.mjs';

afterEach(closeAll);

const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : null);
const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const SPECS = () => JSON.parse(JSON.stringify(CASES.find((c) => c.id === 'mixed-five-tracks').response));
const APPLIED = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 1, outcome: 'written', missions: 3, solari: 18000, house_scrip: 3000, specialization_xp: { Gathering: 4500 }, rewards_already_given: 0 };
async function open(answer = synthetic(), { layout = 'grid', local = {}, tab = 'landsraad' } = {}) {
  const ui = await boot({ get: { '/api/landsraad': answer, '/api/player/specs': SPECS() }, post: { '/api/landsraad/apply': APPLIED }, local: { tab, 'lsb.view': 'board', 'lsb.layout': layout, ...local } });
  await ui.settle();
  return ui;
}
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const tileOf = (ui, name) => [...ui.doc.querySelectorAll('.lcc-bd-cell')].find((c) => c.querySelector('.lcc-hc-name').textContent.toLowerCase() === 'house ' + name.toLowerCase());
const countsOf = (ui, a = synthetic()) => { const hs = ui.w.LCCM.readBoard(a).houses; const n = (k) => hs.filter((h) => ui.w.LCCM.matches(h, { show: k })).length; return { done: n('done'), part: n('part'), new: n('new') }; };

test('#440: one summary line under the term counts the Houses at top tier, in progress and not started, from the same state the Show tabs filter by', async () => {
  const ui = await open();
  const sum = ui.doc.querySelector('.lcc-bd-panel-head .lcc-bd-sum');
  assert.ok(sum, 'the summary line is in the board header');
  const c = countsOf(ui);
  assert.equal(c.done + c.part + c.new, 25, 'every House is in one of the three');
  assert.ok(c.done > 0 && c.part > 0 && c.new > 0, 'the fixture has Houses in all three states, so the line is not a trivial check');
  const keys = [...sum.querySelectorAll('button')];
  assert.deepEqual(keys.map((b) => text(b)), [c.done + ' top tier', c.part + ' in progress', c.new + ' not started']);
  assert.deepEqual(keys.map((b) => b.dataset.val), ['done', 'part', 'new'], 'each key is a value of the Show filter');
  // the wording is the Show tabs' (one name per thing)
  const tabs = Object.fromEntries([...ui.doc.querySelectorAll('[data-lcb="filter"][data-key="show"]')].filter((b) => b.closest('.lcc-k-tabs')).map((b) => [b.dataset.val, text(b)]));
  keys.forEach((b) => assert.equal(text(b).replace(/^\d+ /, ''), tabs[b.dataset.val].toLowerCase()));
  assert.ok(keys.every((b) => /Show only the \d+ Houses? at/.test(b.title) && /changes nothing in the save/.test(b.title)), 'the mouse-over says what a press does');
  assert.equal(sum.getAttribute('role'), 'group');
  assert.ok(sum.getAttribute('aria-label'));
});

test('#440: pressing a count sets that Show tab, shows exactly those Houses, and the count of Houses shown agrees', async () => {
  const ui = await open();
  const c = countsOf(ui);
  click(ui, '.lcc-bd-sum [data-val="part"]');
  assert.equal(ui.doc.querySelector('.lcc-k-tabs [data-val="part"]').getAttribute('aria-checked'), 'true', 'the Show tab In progress is on');
  assert.equal(ui.doc.querySelector('.lcc-bd-sum [data-val="part"]').getAttribute('aria-pressed'), 'true');
  assert.equal(ui.doc.querySelector('.lcc-bd-sum [data-val="done"]').getAttribute('aria-pressed'), 'false');
  assert.equal(ui.doc.querySelector('[data-lk="shown"]').textContent, String(c.part), 'the Houses shown are the ones counted');
  assert.deepEqual(countsOf(ui), c, 'the counts are of the term, not of what is shown: pressing a count does not change them');
  assert.equal(text(ui.doc.querySelector('.lcc-bd-sum [data-val="done"]')), c.done + ' top tier');
  assert.equal(ui.posted.length, 0, 'nothing is written');
});

test('#440: a character in no faction has no summary line (every House would read as not started)', async () => {
  const a = synthetic();
  a.character.faction_id = null;
  const ui = await open(a);
  assert.ok(ui.doc.querySelector('.lcc-bd-panel-head'), 'the board is drawn');
  assert.equal(ui.doc.querySelector('.lcc-bd-sum'), null);
});
