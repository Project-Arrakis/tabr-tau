// The Command Center view inside the page (M5): the switch between the two views, what each draws, the chosen path surviving a write and the redraw after it, the focus coming back, the House
// links, and a Landsraad that cannot be read. Real page code with the synthetic answers of M1; no game save.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { read } from './lcc-load.mjs';

afterEach(closeAll);
const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const SPECS = (id = 'mixed-five-tracks') => JSON.parse(JSON.stringify(CASES.find((c) => c.id === id).response));
const text = (e) => (e ? e.textContent.replace(/\s+/g, ' ').trim() : null);
const POSTS = { '/api/player/specs/xp': { ok: true, before: 1, after: 1001, level: 9 }, '/api/player/specs/max': { ok: true }, '/api/player/specs/keystones/grant': { ok: true, granted: 3 } };
async function open(local = {}, get = {}) {
  const ui = await boot({ get: { '/api/player/specs': SPECS(), '/api/landsraad': synthetic(), ...get }, post: POSTS, local: { tab: 'player', ...local } });
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  return ui;
}

test('the classic list is the default: its table and buttons are there, with a switch that offers the Command Center', async () => {
  const ui = await open();
  assert.equal(ui.doc.querySelector('#lcs-root'), null);
  assert.ok(ui.doc.querySelector('table'), 'the classic table');
  assert.equal(ui.doc.querySelectorAll('[data-act="spAdd"]').length, 5);
  const sw = ui.doc.querySelector('.lcc-sp-viewsw');
  assert.ok(sw);
  assert.equal(sw.getAttribute('role'), 'group');
  assert.equal(sw.querySelector('[data-spview="classic"]').getAttribute('aria-pressed'), 'true');
  assert.equal(sw.querySelector('[data-spview="cc"]').getAttribute('aria-pressed'), 'false');
});

test('the switch moves between the two views, remembers the choice, and the classic list comes back whole', async () => {
  const ui = await open();
  ui.click('[data-spview="cc"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#lcs-root'));
  assert.equal(ui.doc.querySelector('[data-spview="cc"]').getAttribute('aria-pressed'), 'true');
  assert.equal(ui.w.localStorage.getItem('spec.view'), 'cc');
  assert.equal(ui.doc.querySelectorAll('[data-lcs-lane]').length, 5);
  assert.equal(ui.doc.querySelector('table'), null, 'the classic table is not drawn beside it');
  ui.click('[data-spview="classic"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#lcs-root'), null);
  assert.equal(ui.doc.querySelectorAll('[data-act="spAdd"]').length, 5);
  assert.equal(ui.w.localStorage.getItem('spec.view'), 'classic');
});

test('a stored choice of the Command Center opens it, and an unknown stored word opens the classic list', async () => {
  const a = await open({ 'spec.view': 'cc' });
  assert.ok(a.doc.querySelector('#lcs-root'));
  closeAll();
  const b = await open({ 'spec.view': '<script>' });
  assert.equal(b.doc.querySelector('#lcs-root'), null);
});

// the track mapping and the raw-row writer are classic-list only since #986 (the endpoints and the classic list are unchanged)
const CLASSIC_ONLY = new Set(['spAssign', 'spAssignHand', 'specSet']);
test('both views offer the same operations of the editor, except the three the Command Center view dropped (#986): every other data-act of the classic list is reachable in it', async () => {
  const classic = await open({ 'spec.view': 'classic' }, { '/api/player/specs': SPECS('conflicting-learned-number-leaves-combat-unresolved') });
  const want = new Set([...classic.doc.querySelectorAll('[data-act]')].map((b) => b.dataset.act).filter((a) => /^(sp|spec)/.test(a)));
  closeAll();
  const cc = await open({ 'spec.view': 'cc' }, { '/api/player/specs': SPECS('conflicting-learned-number-leaves-combat-unresolved') });
  const have = new Set([...cc.doc.querySelectorAll('[data-act]')].map((b) => b.dataset.act));
  have.delete('spAddCC'); have.add('spAdd'); // the Command Center's Apply XP checks the number and then runs spAdd
  for (const a of want) if (!CLASSIC_ONLY.has(a)) assert.ok(have.has(a), a + ' is reachable in the Command Center view');
  for (const a of CLASSIC_ONLY) { assert.ok(want.has(a), a + ' is still in the classic list'); assert.equal(cc.doc.querySelector('[data-act="' + a + '"]'), null, a + ' is not drawn in the Command Center view'); }
  for (const a of ['spKeyAll', 'spKeyReset', 'spResetAll', 'spMax', 'spReset']) assert.ok(cc.doc.querySelector('[data-act="' + a + '"]'), a);
});

test('the path chosen survives a write and the redraw after it, and the focus goes back to the button pressed', async () => {
  const ui = await open({ 'spec.view': 'cc' });
  ui.click('[data-lcs-lane="exploration"]');
  assert.equal(text(ui.doc.querySelector('#lcs-dos-h')), 'EXPLORATION');
  const before = ui.fetched.filter((p) => p === '/api/player/specs').length;
  const add = ui.doc.querySelector('[data-act="spAddCC"][data-t="Exploration"]');
  add.focus();
  ui.doc.querySelector('#spxExploration').value = '1000';
  add.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true }));
  await ui.settle(); await ui.wait(20); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/specs/xp').map((p) => p.body), [{ track: 'Exploration', amount: 1000 }]);
  assert.ok(ui.fetched.filter((p) => p === '/api/player/specs').length > before, 'the save is read again after the write');
  assert.equal(text(ui.doc.querySelector('#lcs-dos-h')), 'EXPLORATION', 'the same path is chosen');
  assert.equal(ui.doc.querySelector('[data-lcs-lane="exploration"]').getAttribute('aria-pressed'), 'true');
  assert.equal(ui.doc.activeElement && ui.doc.activeElement.dataset.act, 'spAddCC', 'the focus is back on the button');
  assert.equal(ui.doc.activeElement.dataset.t, 'Exploration');
});

test('the numbers on the page are the ones the answer after the write holds, not a count kept by the page', async () => {
  let level = 3;
  const ui = await open({ 'spec.view': 'cc' });
  const first = text(ui.doc.querySelector('[data-lcs-lane="crafting"] .lcc-sp-lane-nums'));
  const next = SPECS(); next.rows.find((r) => r.track === 'Crafting').xp = 12345; next.rows.find((r) => r.track === 'Crafting').level = level;
  ui.w.fetch = ((orig) => async (p, o) => { if (String(p).endsWith('/api/player/specs') && (!o || !o.method || o.method === 'GET')) return { ok: true, status: 200, json: async () => next }; return orig(p, o); })(ui.w.fetch);
  ui.click('[data-act="spKeyAll"]'); await ui.settle(); ui.click('#aok'); await ui.settle(); await ui.wait(20); await ui.settle();
  const after = text(ui.doc.querySelector('[data-lcs-lane="crafting"] .lcc-sp-lane-nums'));
  assert.notEqual(after, first);
  assert.match(after, /^12,345 XP/);
});

test('a House of a path opens the Landsraad board with that House chosen; nothing is written', async () => {
  const a = synthetic();
  const ui = await open({ 'spec.view': 'cc' });
  const house = a.tasks.find((t) => t.line === 'Crafting');
  const btn = ui.doc.querySelector('[data-lcs-go="' + house.id + '"]');
  assert.ok(btn, 'a House with the line of the chosen path is listed');
  const writes = ui.posted.length;
  btn.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true }));
  await ui.settle(); await ui.wait(20); await ui.settle();
  assert.equal(ui.w.localStorage.getItem('tab'), 'landsraad');
  assert.equal(ui.w.LCCB.state.sel, house.id);
  const insp = ui.doc.querySelector('.lcc-bd-insp');
  assert.ok(insp, 'the House Inspector is open');
  assert.equal(insp.getAttribute('data-id'), String(house.id));
  assert.equal(ui.posted.length, writes, 'no request was sent');
  assert.equal(ui.w.LCCB.state.show, 'all');
});

test('a Landsraad that cannot be read leaves the Houses "Not in the save" and the rest of the page working', async () => {
  const ui = await open({ 'spec.view': 'cc' }, { '/api/landsraad': { __fail: 'no term', __status: 400 } });
  assert.ok(ui.doc.querySelector('#lcs-root'));
  assert.match(text(ui.doc.querySelector('.lcc-sp-houses')), /could not be read, so the Houses of this path are not in the save/);
  assert.equal(ui.doc.querySelectorAll('[data-lcs-lane]').length, 5);
  assert.ok(ui.doc.querySelector('[data-act="spKeyAll"]'));
});

test('a Landsraad that answers without a task list leaves the Houses "Not in the save", not "no House is listed" (#899)', async () => {
  for (const answer of [{}, { tasks: null }, { tasks: 'x' }, { term: 3 }]) {
    const ui = await open({ 'spec.view': 'cc' }, { '/api/landsraad': answer });
    assert.match(text(ui.doc.querySelector('.lcc-sp-houses')), /could not be read, so the Houses of this path are not in the save/, JSON.stringify(answer));
    assert.doesNotMatch(text(ui.doc.querySelector('.lcc-sp-houses')), /No House of this term is listed/);
    assert.equal(text(ui.doc.querySelectorAll('.lcc-sp-fact strong')[0]), 'Not in the save');
    closeAll();
  }
  const empty = await open({ 'spec.view': 'cc' }, { '/api/landsraad': { tasks: [] } });
  assert.match(text(empty.doc.querySelector('.lcc-sp-houses')), /No House of this term is listed under this path/, 'a list that is there and empty is still "none listed"');
});

test('choosing another lane keeps what was typed in the XP fields, and drawing the view again does too (#899)', async () => {
  const ui = await open({ 'spec.view': 'cc' });
  const field = (t) => ui.doc.querySelector('#spx' + t);
  const type = (t, v) => { field(t).value = v; field(t).dispatchEvent(new ui.w.Event('input', { bubbles: true })); };
  type('Crafting', '2500');
  ui.click('[data-lcs-lane="combat"]');
  assert.equal(field('Combat').value, '1000', 'another path has its own field, untouched');
  type('Combat', '-300');
  ui.click('[data-lcs-lane="crafting"]');
  assert.equal(field('Crafting').value, '2500', 'what was typed for Crafting is still there');
  ui.click('[data-lcs-lane="combat"]');
  assert.equal(field('Combat').value, '-300');
  assert.equal(ui.posted.length, 0, 'choosing a lane sends nothing');
});

test('a save with no specialization tracks shows one message and no control', async () => {
  const ui = await open({ 'spec.view': 'cc' }, { '/api/player/specs': { rows: [], unassigned: [], maxXp: 44182, tracks: [], resetCount: 0 } });
  assert.match(text(ui.doc.querySelector('.lcc-sp-empty')), /This save has no specialization tracks\./);
  assert.equal(ui.doc.querySelectorAll('#lcs-root [data-act]').length, 0);
});

test('without the Command Center scripts the page shows the classic list and no switch', async () => {
  const ui = await boot({ get: { '/api/player/specs': SPECS() }, post: POSTS, local: { tab: 'player', 'spec.view': 'cc' }, pre: (w) => { Object.defineProperty(w, 'LCCS', { value: undefined, writable: true, configurable: true }); } });
  ui.w.LCCS = undefined;
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('.lcc-sp-viewsw'), null);
  assert.equal(ui.doc.querySelector('#lcs-root'), null);
  assert.ok(ui.doc.querySelector('table'));
});
