// Give Blood Marks (#344): on Player > Character > Overview, in the Money card. It shows what the backpack holds, an amount box (default 100, 1 to the stack size),
// and a button that posts the amount; the editor's refusal is shown in its own words.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const player = (o = {}) => ({ controllerId: 1, pawnId: 2, accountId: 1, name: 'Tester', actor: { x: 1, y: 2, z: 3 }, journey: { done: 1, total: 2 }, solari: 100, bloodMarks: 420, bloodMarksMax: 50000, ...o });
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);
const ROUTE = '/api/player/blood-marks';

async function openOverview(ui) { ui.click('[data-tab="player"]'); await ui.settle(); }
const setBox = (ui, v) => { const el = ui.doc.querySelector('#bmAmt'); el.value = v; el.dispatchEvent(new ui.w.Event('input', { bubbles: true })); };

test('Blood Marks: the row shows the amount the backpack holds, a box that starts at 100, and a note that names the item', async () => {
  const ui = await boot({ get: { '/api/player': player() } });
  await openOverview(ui);
  assert.equal(ui.doc.querySelector('#bmHave').textContent, '420');
  const box = ui.doc.querySelector('#bmAmt');
  assert.ok(box, 'there is an amount box');
  assert.equal(box.value, '100');
  assert.equal(box.min, '1');
  assert.equal(box.max, '50000');
  const btn = ui.doc.querySelector('[data-act="bloodMarks"]');
  assert.equal(btn.textContent.trim(), 'Give Blood Marks');
  assert.ok(!btn.disabled);
  const row = btn.closest('.row');
  assert.ok(row.textContent.includes('HalloweenCoin'), 'the note names the item');
  assert.ok(row.textContent.includes('Scourge event'), 'the note says it is the event currency');
  assert.ok(!/operator|called/i.test(row.textContent), 'no hedge about what it is called');
  assert.equal(row.querySelector('b').title, 'Blood Marks (item HalloweenCoin): the currency of the Scourge event');
  assert.equal(ui.doc.querySelectorAll('[data-act="bloodMarks"]').length, 1);
  assert.equal(ui.doc.querySelectorAll('[data-act="setBloodMarks"]').length, 0, 'there is no set-amount control, as for Solari');
  ui.dom.window.close();
});

test('Blood Marks: a backpack with none shows 0, and an older answer without the figure shows 0 too', async () => {
  let ui = await boot({ get: { '/api/player': player({ bloodMarks: 0 }) } });
  await openOverview(ui);
  assert.equal(ui.doc.querySelector('#bmHave').textContent, '0');
  ui.dom.window.close();
  ui = await boot({ get: { '/api/player': (({ bloodMarks, bloodMarksMax, ...rest }) => rest)(player()) } });
  await openOverview(ui);
  assert.equal(ui.doc.querySelector('#bmHave').textContent, '0');
  assert.equal(ui.doc.querySelector('#bmAmt').max, '50000');
  ui.dom.window.close();
});

test('Blood Marks: the button posts the amount that is in the box, says what happened, and the figure is read again', async () => {
  const get = { '/api/player': player() };
  const ui = await boot({ get, post: { [ROUTE]: { ok: true, requested: 500, given: 500, reason: '', limit: '', before: 420, after: 920, stacks: 0, outcome: 'written' } } });
  await openOverview(ui);
  get['/api/player'] = player({ bloodMarks: 920 }); // what the save holds after the give
  setBox(ui, '500');
  ui.click('[data-act="bloodMarks"]'); await ui.settle();
  assert.deepEqual(posts(ui, ROUTE).map((p) => p.body), [{ amount: 500 }]);
  const msg = ui.doc.querySelector('#toast').textContent;
  assert.ok(msg.includes('Gave 500 Blood Marks') && msg.includes('420') && msg.includes('920'), msg);
  assert.ok(!ui.doc.querySelector('#toast .err'), 'it is not an error');
  await ui.wait(20); await ui.settle();
  assert.equal(ui.doc.querySelector('#bmHave').textContent, '920', 'the page shows what the save holds now');
  ui.dom.window.close();
});

test('Blood Marks: the default amount is posted as it stands, one press is one request', async () => {
  const ui = await boot({ get: { '/api/player': player() }, post: { [ROUTE]: { ok: true, requested: 100, given: 100, reason: '', limit: '', before: 420, after: 520, stacks: 0, outcome: 'queued' } } });
  await openOverview(ui);
  ui.click('[data-act="bloodMarks"]'); await ui.settle();
  assert.deepEqual(posts(ui, ROUTE).map((p) => p.body), [{ amount: 100 }]);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Gave 100 Blood Marks'));
  ui.dom.window.close();
});

test('Blood Marks: part of it fitted is a warning with the real amount and the reason', async () => {
  const ui = await boot({ get: { '/api/player': player() }, post: { [ROUTE]: { ok: true, requested: 500, given: 100, reason: 'the backpack is full', limit: 'slots', before: 49900, after: 50000, stacks: 0, outcome: 'written' } } });
  await openOverview(ui);
  setBox(ui, '500');
  ui.click('[data-act="bloodMarks"]'); await ui.settle();
  const t = ui.doc.querySelector('#toast .part');
  assert.ok(t, 'a warning, not an error');
  assert.equal(ui.doc.querySelector('#toast .err'), null);
  assert.ok(t.textContent.includes('Gave 100 of 500 Blood Marks: the backpack is full'), t.textContent);
  ui.dom.window.close();
});

test('Blood Marks: a refusal because there is no room is shown in the editor\'s words', async () => {
  const msg = 'No Blood Marks were given: the backpack is full.';
  const ui = await boot({ get: { '/api/player': player() }, post: { [ROUTE]: { __fail: msg, __code: 'no_room', __status: 409 } } });
  await openOverview(ui);
  ui.click('[data-act="bloodMarks"]'); await ui.settle();
  const err = ui.doc.querySelector('#toast .err');
  assert.ok(err && err.textContent.includes('No Blood Marks were given: the backpack is full'), 'the refusal is on the page');
  assert.ok(!ui.doc.querySelector('#toast').textContent.includes('Gave'), 'and no success is claimed');
  assert.equal(ui.doc.querySelector('#bmHave').textContent, '420');
  ui.dom.window.close();
});

test('Blood Marks: an amount the editor refuses (an empty box) is shown, not swallowed', async () => {
  const ui = await boot({ get: { '/api/player': player() }, post: { [ROUTE]: { __fail: 'amount out of range (1..50000)', __code: 'bad_argument', __status: 422 } } });
  await openOverview(ui);
  setBox(ui, '');
  ui.click('[data-act="bloodMarks"]'); await ui.settle();
  assert.deepEqual(posts(ui, ROUTE).map((p) => p.body), [{ amount: 0 }]);
  assert.ok(ui.doc.querySelector('#toast .err').textContent.includes('out of range'));
  ui.dom.window.close();
});
