// #925: the decree on the plaque follows the term's active decree after every write on the Term and decrees panel, and after the page's own refresh.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : '');
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const pick = (ui, sel, v) => { const s = ui.doc.querySelector(sel); assert.ok(s, 'no ' + sel); s.value = String(v); s.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const title = (ui) => text(ui.doc.querySelector('.lcc-pl-title'));
const plaqueText = (ui) => text(ui.doc.querySelector('.lcc-plaque'));
// the save answers the next read with the term as the write left it
async function open(local = {}) {
  const answer = synthetic();
  const post = { '/api/landsraad/term': { get __fail() { return undefined; }, ok: true } };
  const ui = await boot({ get: { '/api/landsraad': answer }, post, local: Object.assign({ tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' }, local) });
  await ui.settle();
  ui.answer = answer;
  // the write changes the saved term, as the server does
  Object.defineProperty(post['/api/landsraad/term'], '__fail', { get() { const b = ui.lastBody(); answer.term.active_decree_id = +b.active_decree_id; if (b.reigning_faction_id !== undefined) answer.term.reigning_faction_id = b.reigning_faction_id === '' ? null : +b.reigning_faction_id; return undefined; } });
  ui.lastBody = () => ui.posted[ui.posted.length - 1].body;
  return ui;
}
const decName = (ui, id) => text(ui.doc.querySelector('#lsb-tad option[value="' + id + '"]'));

test('the plaque shows the active decree of the term at first', async () => {
  const ui = await open();
  assert.match(title(ui), /Exclusive Vehicle|Vehicle/i);
});

test('Active decree drop-down and Apply: the plaque shows the decree that was applied', async () => {
  const ui = await open();
  const before = title(ui);
  pick(ui, '#lsb-tad', 6);
  pick(ui, '#lsb-trf', 2);
  click(ui, '[data-lb="tapply"]');
  await ui.settle();
  assert.equal(ui.posted.length, 1);
  assert.equal(ui.answer.term.active_decree_id, 6);
  assert.notEqual(title(ui), before);
  assert.match(plaqueText(ui), /Repair/i);
  assert.match(text(ui.doc.querySelector('.lcc-pl-held')), /Harkonnen/i);
});

test('Make active on a row of the pool: the plaque shows that decree', async () => {
  const ui = await open();
  click(ui, '#lcc-term .lcc-tm-row[data-decree="4"] [data-lb="tmk"]');
  await ui.settle();
  assert.equal(ui.answer.term.active_decree_id, 4);
  assert.match(plaqueText(ui), /Crafting/i);
});

test('Make active again on another row, with the board redrawn in between', async () => {
  const ui = await open();
  click(ui, '#lcc-term .lcc-tm-row[data-decree="4"] [data-lb="tmk"]');
  await ui.settle();
  click(ui, '#lcc-term .lcc-tm-row[data-decree="2"] [data-lb="tmk"]');
  await ui.settle();
  assert.match(plaqueText(ui), /Ranged/i);
});

test('there is no sample: choosing a decree in the section does not change the plaque until it is written', async () => {
  const ui = await open();
  const before = title(ui);
  pick(ui, '#lsb-tad', 6);
  assert.equal(title(ui), before, 'a draft does not reach the plaque');
  assert.equal(ui.doc.querySelector('.lcc-plaque [class*="demo"]'), null);
  assert.equal(ui.doc.querySelector('#lcc-bd-decree'), null);
  click(ui, '[data-lb="tapply"]');
  await ui.settle();
  assert.match(plaqueText(ui), /Repair/i, 'written: the plaque follows');
});

test('the term section says a draft is not applied yet, and the plaque keeps the decree of the term until it is written', async () => {
  const ui = await open();
  const hint = () => text(ui.doc.querySelector('#lcc-tm-pending'));
  assert.equal(hint(), '', 'no draft: no hint');
  pick(ui, '#lsb-trf', 2); // a redraw of the board; the term did not change
  assert.match(title(ui), /Exclusive Vehicle|Vehicle/i, 'the plaque still shows the decree of the term');
  pick(ui, '#lsb-tad', 4);
  assert.match(hint(), /Not applied yet/);
  assert.match(hint(), /Exclusive Vehicle/i, 'names the decree the term still has');
  assert.match(hint(), /Production/i, "names the decree the choice would set");
  assert.equal(ui.posted.length, 0, 'a draft writes nothing');
  assert.ok(ui.doc.querySelector('#lcc-tm-pending[role="status"]'));
  pick(ui, '#lsb-tad', 7);
  assert.equal(hint(), '', 'back to the term: no hint');
  pick(ui, '#lsb-tad', 4);
  pick(ui, '#lsb-trf', 2); // a redraw keeps the draft and its hint
  assert.match(hint(), /Not applied yet/);
  click(ui, '[data-lb="tapply"]');
  await ui.settle();
  assert.equal(hint(), '', 'written: no hint');
});
