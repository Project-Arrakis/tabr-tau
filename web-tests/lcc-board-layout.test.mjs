// Display remediation B, in the page's own code (jsdom): the bulk-edit bar sits in the board's grid right after the board panel and survives a redraw of the board, the House Intelligence panel keeps its
// scroll position and its opened notes over a redraw, the Actions block is first, the question about one House is drawn in the panel and a copy of the result line stands next to the buttons, the
// notes are disclosures, and the bar of the chosen houses for a phone follows the choice and presses the action bar's own buttons. What the controls send is in lcc-board-requests.test.mjs.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 0, outcome: 'written' };
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
async function open(answer = synthetic()) {
  const ui = await boot({ get: { '/api/landsraad': answer }, post: { '/api/landsraad/apply': OK }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  return ui;
}
const look = (ui, a, name) => click(ui, '[data-fk="ht:' + idOf(a, name) + '"]');
const pick = (ui, a, names) => { for (const n of names) { const c = ui.doc.querySelector('.lcc-bd-pick[data-id="' + idOf(a, n) + '"]'); c.checked = true; c.dispatchEvent(new ui.w.Event('change', { bubbles: true })); } };

test('the bulk-edit bar sits in the board grid right after the board panel, before the victory analysis and the panel, so the order of the keys is the order of the eyes', async () => {
  const ui = await open();
  const grid = ui.doc.querySelector('.lcc-bd-grid');
  assert.deepEqual([...grid.children].map((c) => c.className.split(' ')[0]), ['lcc-bd-surface', 'lcc-bd-slot', 'lcc-bd-side']);
  const slot = grid.querySelector('.lcc-bd-slot');
  assert.equal(slot.firstElementChild.className, 'actbar');
  assert.equal(slot.firstElementChild.getAttribute('aria-label'), 'Actions on the board', 'the group keeps its name');
  assert.equal(ui.doc.querySelector('#lsb-board > .actbar'), null, 'and it is not left after the board');
  // the focusable controls come in the order they are drawn: the cards, the bar, then the panel
  look(ui, synthetic(), 'Wallach');
  const order = [...ui.doc.querySelectorAll('.lcc-hc-select, .lcc-bd-pick, .actbar button, .actbar select, .lcc-bd-insp button, .lcc-bd-insp select')];
  const area = (e) => (e.closest('.lcc-bd-insp') ? 2 : e.closest('.actbar') ? 1 : 0);
  assert.deepEqual(order.map(area), [...order.map(area)].sort(), 'cards, then the bar, then the panel');
  assert.ok(order.some((e) => area(e) === 1) && order.some((e) => area(e) === 2));
  const lcc = ui.doc.querySelector('section.lcc-bd');
  assert.equal(lcc.getAttribute('aria-labelledby'), 'lcc-bd-h', 'the landmark of the board keeps its name');
});

test('the bar and what is chosen survive a redraw of the board (a select, a filter, a layout switch)', async () => {
  const a = synthetic();
  const ui = await open(a);
  pick(ui, a, ['Sor', 'Kenola']);
  const bar = ui.doc.querySelector('.actbar');
  click(ui, '[data-fk="bf:hs:open"]');
  look(ui, a, 'Wallach');
  click(ui, '[data-fk="bf:layout:list"]');
  click(ui, '[data-fk="bf:layout:grid"]');
  assert.equal(ui.doc.querySelectorAll('.actbar').length, 1);
  assert.equal(ui.doc.querySelector('.actbar').parentNode.getAttribute('data-lcc-slot'), 'bar');
  assert.equal(ui.doc.querySelector('.actbar'), bar, 'the same element: nothing in it was drawn again');
  assert.match(text(ui.doc.querySelector('#lsb-sum')), /^Chosen: /);
});

test('the Actions block is first under the name, the notes are disclosures with a plain summary, and what is opened stays open over a redraw', async () => {
  const a = synthetic();
  const ui = await open(a);
  look(ui, a, 'Lindaren');
  const kids = [...ui.doc.querySelector('.lcc-bd-insp').children].map((c) => c.getAttribute('aria-labelledby') || c.className.split(' ')[0]);
  assert.deepEqual(kids.slice(0, 5), ['lcc-bd-insp-top', 'lcc-bd-back', 'lcc-bd-identity', 'lcc-bd-s4', 'lcc-bd-pin']);
  const ds = [...ui.doc.querySelectorAll('.lcc-bd-insp details.lcc-bd-disc')];
  assert.deepEqual(ds.map((d) => d.querySelector('summary').textContent), ['How each click is written', 'About Fill and the tiers', 'What the marks mean', 'Important: three separate states']);
  assert.ok(ds.every((d) => !d.open), 'closed until asked for');
  assert.match(text(ds[0]), /Each click is written at once with a backup first, and refused if this House is no longer as shown\./, 'the words are the ones that were drawn open before');
  assert.match(text(ds[3]), /^Important: three separate statesImportant: A House claimed by a faction/);
  assert.match(text(ds[1]), /Fill and Fill faction goal: Writes 6 missions/);
  assert.match(text(ds[2]), /✓ Collected.*◆ Waiting at representative.*○ Not reached.*\? No delivery record/);
  ds[2].open = true;
  ds[2].dispatchEvent(new ui.w.Event('toggle'));
  look(ui, a, 'Kenola');
  const now = [...ui.doc.querySelectorAll('.lcc-bd-insp details.lcc-bd-disc')];
  assert.deepEqual(now.map((d) => d.open), [false, false, true, false], 'the one that was opened is still open for the next House');
});

test('the panel keeps its scroll position for the House it is on over a redraw, and starts at the top for another House', async () => {
  const a = synthetic();
  const ui = await open(a);
  look(ui, a, 'Wallach');
  let insp = ui.doc.querySelector('.lcc-bd-insp');
  insp.scrollTop = 240;
  insp.dispatchEvent(new ui.w.Event('scroll'));
  click(ui, '[data-fk="bf:hs:any"]'); // the board is drawn again
  insp = ui.doc.querySelector('.lcc-bd-insp');
  assert.equal(insp.scrollTop, 240, 'the same House: the panel is where it was');
  click(ui, '[data-fk="bf:show:all"]');
  assert.equal(ui.doc.querySelector('.lcc-bd-insp').scrollTop, 240);
  look(ui, a, 'Kenola');
  assert.equal(ui.doc.querySelector('.lcc-bd-insp').scrollTop, 0, 'another House starts at the top');
});

test('the question about one House is drawn in the panel next to the buttons; a question about several Houses stays in the bar; both go back to the bar when the panel moves to another House', async () => {
  const a = synthetic();
  const ui = await open(a);
  look(ui, a, 'Vernius');
  click(ui, '.lcc-bd-insp .lcc-bd-act[data-do="complete"]');
  await ui.settle();
  const q = ui.doc.querySelector('#lsb-confirm');
  assert.ok(q, 'the question is asked, as before');
  assert.equal(q.parentNode.getAttribute('data-lcc-slot'), 'ask', 'in the panel');
  assert.ok(q.closest('.lcc-bd-pin'));
  assert.match(text(q.querySelector('b')), /^Complete House Vernius for Atreides\? Replaces the win of Harkonnen on House Vernius\./);
  assert.equal(ui.doc.activeElement.getAttribute('data-fk'), 'c-no', 'the safe answer has the focus');
  look(ui, a, 'Kenola'); // the panel is on another House: the question is not about it
  const q2 = ui.doc.querySelector('#lsb-confirm');
  assert.ok(q2 && q2.closest('.actbar'), 'it is back in the action bar and still there to answer');
  click(ui, '[data-fk="c-no"]');
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.match(text(ui.doc.querySelector('#lcc-bd-eres')), /^Cancelled\. Nothing was written\.$/, 'the copy of the result line says the same as the result line');
  assert.equal(text(ui.doc.querySelector('#lcc-bd-eres')), text(ui.doc.querySelector('#lsb-result')));
  // several houses: the question stays in the bar
  pick(ui, a, ['Sor', 'Lindaren']);
  click(ui, '[data-fk="b-complete"]');
  await ui.settle();
  assert.ok(ui.doc.querySelector('#lsb-confirm').closest('.actbar'), 'a question about several Houses is the action bar\'s');
  assert.equal(ui.doc.querySelector('.lcc-bd-ask').childElementCount, 0);
});

test('the copy of the result line is for the eye only (the result line is the one that is announced) and carries the same words and kind after a write', async () => {
  const a = synthetic();
  const ui = await open(a);
  look(ui, a, 'Kenola');
  click(ui, '.lcc-bd-insp .lcc-bd-tier[data-t="3"]');
  await ui.settle();
  const m = ui.doc.querySelector('#lcc-bd-eres');
  assert.equal(m.getAttribute('aria-hidden'), 'true');
  assert.equal(ui.doc.querySelector('#lsb-result').getAttribute('role'), 'status');
  assert.equal(text(m), text(ui.doc.querySelector('#lsb-result')));
  assert.match(text(m), /^House Kenola: your contribution 1,200 → 3,600/);
});

test('the bar of the chosen houses for a phone follows the choice and presses the action bar\'s own buttons', async () => {
  const a = synthetic();
  const ui = await open(a);
  const bar = ui.doc.querySelector('#lsb-phonebar');
  assert.ok(bar.hidden, 'nothing is chosen: no bar');
  assert.equal(bar.getAttribute('aria-label'), 'The chosen houses');
  assert.deepEqual([...bar.querySelectorAll('button')].map((b) => text(b)), ['Fill chosen', 'Complete chosen', 'Reopen chosen']);
  pick(ui, a, ['Sor', 'Lindaren']);
  assert.equal(bar.hidden, false);
  assert.equal(text(bar.querySelector('.pn')), '2 chosen');
  click(ui, '[data-fk="b-none"]');
  assert.ok(bar.hidden);
  assert.equal(text(bar.querySelector('.pn')), '0 chosen');
  pick(ui, a, ['Sor', 'Lindaren', 'Kenola']);
  const pressed = [];
  for (const b of ui.doc.querySelectorAll('.actbar [data-lb="bulk"]')) b.addEventListener('click', () => pressed.push(b.dataset.do));
  click(ui, '[data-pb="complete"]');
  await ui.settle();
  assert.deepEqual(pressed, ['complete'], 'a press of the phone bar is a press of the action bar\'s button');
  assert.match(text(ui.doc.querySelector('#lsb-confirm b')), /^Complete 3 houses for Atreides\?/);
  assert.equal(ui.posted.length, 0, 'nothing is written before the answer');
  // a phone-bar button holds no write of its own
  assert.equal(ui.doc.querySelectorAll('#lsb-phonebar [data-lb], #lsb-phonebar [data-fk^="c-"]').length, 0);
});

test('the way back to the board scrolls to the card of the House and puts the focus on it', async () => {
  const a = synthetic();
  const ui = await open(a);
  look(ui, a, 'Wallach');
  const card = ui.doc.querySelector('[data-fk="ht:' + idOf(a, 'Wallach') + '"]');
  let scrolled = null;
  card.scrollIntoView = (o) => { scrolled = o; };
  const back = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-back');
  assert.equal(text(back), 'Back to board');
  click(ui, back);
  assert.equal(scrolled.block, 'center');
  assert.equal(ui.doc.activeElement, card);
});

test('a board that cannot be checked is read-only: its notice comes first, and no bar of the chosen houses is drawn', async () => {
  const a = synthetic();
  a.tasks = a.tasks.slice(0, 7); // not a fixed five by five board
  const ui = await open(a);
  assert.equal(ui.doc.querySelector('#lsb-phonebar'), null);
  assert.equal(ui.doc.querySelector('.actbar'), null);
  assert.ok(ui.doc.querySelector('#lsb-result'));
  const slot = ui.doc.querySelector('.lcc-bd-slot');
  assert.ok(slot && slot.querySelector('#lsb-result'), 'the result line stands in the same place');
});
