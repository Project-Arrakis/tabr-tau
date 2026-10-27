// Complete chosen and Reopen chosen on the Landsraad tab (#451), and the per-house Complete and Reopen (#455): no question is asked, the faction list starts on the character's own faction, the request carries the term, the
// houses, the faction and the number the page counted, and the result line says how many houses were done and how many were left alone.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const TIERS = [1200, 2400, 3600, 4800, 6000];
const house = (id, name, extra = {}) => ({
  id, board_index: id - 900, house_name: 'DA_House' + name, goal_amount: 4000, completed: 0, winning_faction_id: null, completion_time: null, contributions: [], player_contributions: [],
  my_contribution: 0, my_recorded: false,
  rewards: TIERS.map((threshold) => ({ threshold, amount: 1, template_id: 'Item.A', reached: false, delivered: false })),
  ...extra,
});
// 900 Alexin is open; 901 Thorvald is complete for faction 2 with its progress at the goal; 902 Hurata is complete for faction 2 with a progress of 1,000.
const board = (faction = 2) => ({
  term: { term_id: 7, start_time: 1, end_time: 2, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 },
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  character: { player_id: 1, name: 'Tester', faction_id: faction },
  decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }], rotation: [],
  tasks: [
    house(900, 'Alexin'),
    house(901, 'Thorvald', { completed: 1, winning_faction_id: 2, completion_time: 5, contributions: [{ faction_id: 2, amount: 4000 }] }),
    house(902, 'Hurata', { completed: 1, winning_faction_id: 2, completion_time: 5, contributions: [{ faction_id: 2, amount: 1000 }] }),
  ],
});

async function open(extra = {}, data = board()) {
  const ui = await boot({
    get: { '/api/landsraad': data },
    post: {
      '/api/landsraad/complete-many': { ok: true, changed: 2, skipped: 1, noop: false, houses: 3, outcome: 'written' },
      '/api/landsraad/reopen-many': { ok: true, changed: 2, skipped: 1, noop: false, houses: 3, outcome: 'written' },
      '/api/landsraad/progress': { ok: true, outcome: 'written' },
      ...extra,
    },
  });
  ui.freeze();
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}
const tick = (ui, sel, on = true) => { const el = ui.doc.querySelector(sel); assert.ok(el, 'no ' + sel); el.checked = on; el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const pick = (ui, id, value) => { const el = ui.doc.querySelector(id); el.value = value; el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const sent = (ui, route) => ui.posted.filter((p) => p.path === '/api/landsraad/' + route);
const toast = (ui) => ui.doc.querySelector('#toast').lastElementChild.textContent; // the latest message
const sum = (ui) => ui.doc.querySelector('#lsum').textContent;
const noAsk = (ui) => assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null, 'no question is asked');

test('the two buttons are next to Fill chosen, named, and nothing in them asks first', async () => {
  const ui = await open();
  const btns = ['lsFill', 'lsComplete', 'lsReopen'].map((a) => ui.doc.querySelector(`[data-act="${a}"]`));
  assert.deepEqual(btns.map((b) => b && b.textContent.trim()), ['Fill chosen', 'Complete chosen', 'Reopen chosen']);
  assert.ok(btns.every((b) => b.closest('[role="group"]') === btns[0].closest('[role="group"]')), 'in the same group as Fill chosen');
  assert.ok(btns.slice(1).every((b) => !b.hasAttribute('data-confirm') && !b.hasAttribute('data-ask')), 'no confirmation attribute');
  assert.equal(ui.doc.querySelector('#lsum').getAttribute('aria-live'), 'polite');
  assert.ok(ui.doc.querySelector('#lscales').closest('details.about').textContent.includes('Complete chosen marks every house chosen'), 'About this says what it does');
});

test('the faction list starts on the character own faction, also for the per-row buttons', async () => {
  let ui = await open({}, board(2));
  assert.equal(ui.doc.querySelector('#lf').value, '2');
  assert.equal(ui.doc.querySelector('#lsfacnote'), null);
  ui.click('[data-act="tfill"][data-id="900"]'); await ui.settle();
  assert.deepEqual(sent(ui, 'progress').map((p) => p.body), [{ task_id: 900, faction_id: 2, amount: 4000 }]);
  ui.w.close();
  ui = await open({}, board(1));
  assert.equal(ui.doc.querySelector('#lf').value, '1');
  ui.click('[data-act="tfill"][data-id="900"]'); await ui.settle();
  assert.deepEqual(sent(ui, 'progress').map((p) => p.body), [{ task_id: 900, faction_id: 1, amount: 4000 }]);
});

test('a character in no faction: the list is left as it is and the page says no faction is chosen for the person', async () => {
  const ui = await open({}, board(null));
  const sel = ui.doc.querySelector('#lf');
  assert.equal(sel.options.length, 2, 'the list is as it was');
  const note = ui.doc.querySelector('#lsfacnote');
  assert.ok(note && /No faction is chosen for you/.test(note.textContent), note && note.textContent);
  assert.equal(sel.getAttribute('aria-describedby'), 'lsfacnote');
});

test('Complete chosen sends the term, the chosen houses, the faction and the count, and asks nothing', async () => {
  const ui = await open();
  tick(ui, '[data-lshouse="900"]'); tick(ui, '[data-lshouse="901"]'); tick(ui, '[data-lshouse="902"]');
  assert.ok(sum(ui).includes('Complete chosen would complete 2 houses for Harkonnen'), sum(ui)); // 901 is complete with its progress at the goal
  ui.click('[data-act="lsComplete"]'); await ui.settle();
  noAsk(ui);
  assert.deepEqual(sent(ui, 'complete-many').map((p) => p.body), [{ term_id: 7, task_ids: [900, 901, 902], faction_id: 2, expected: 2 }]);
  const t = toast(ui);
  assert.ok(t.includes('Completed 2 houses for Harkonnen') && t.includes('set its progress on each to the house goal') && t.includes('1 house was skipped (already complete with the progress at the goal)'), t);
  assert.ok(!/DA_House|task_id|900|\b9\d\d\b/.test(t), 'no raw ids or prefixes in the line: ' + t);
  assert.equal(ui.doc.querySelector('[data-lshouse="900"]').checked, true, 'the choice is kept, so Reopen chosen can follow');
});

test('Complete chosen uses the faction the person picked, and the pick survives the redraw', async () => {
  const ui = await open();
  pick(ui, '#lf', '1');
  tick(ui, '[data-lshouse="900"]');
  assert.ok(sum(ui).includes('Complete chosen would complete 1 house for Atreides'), sum(ui));
  ui.click('[data-act="lsComplete"]'); await ui.settle();
  assert.deepEqual(sent(ui, 'complete-many').map((p) => p.body), [{ term_id: 7, task_ids: [900], faction_id: 1, expected: 1 }]);
  assert.equal(ui.doc.querySelector('#lf').value, '1', 'the choice is kept across the redraw');
});

test('Complete chosen with nothing to change, nothing chosen or only single tiers sends nothing and says why', async () => {
  const ui = await open();
  ui.click('[data-act="lsComplete"]'); await ui.settle();
  assert.ok(toast(ui).startsWith('Nothing is chosen'), toast(ui));
  tick(ui, '[data-lstier="1200"][data-lsid="900"]');
  ui.click('[data-act="lsComplete"]'); await ui.settle();
  assert.ok(toast(ui).includes('No whole house is chosen'), toast(ui));
  tick(ui, '[data-lstier="1200"][data-lsid="900"]', false);
  tick(ui, '[data-lshouse="901"]');
  ui.click('[data-act="lsComplete"]'); await ui.settle();
  assert.ok(toast(ui).startsWith('Nothing to change') && toast(ui).includes('Harkonnen'), toast(ui));
  noAsk(ui);
  assert.equal(sent(ui, 'complete-many').length, 0);
});

test('Complete chosen with no faction in the list names the problem and sends nothing', async () => {
  const ui = await open({}, { ...board(null), factions: [] });
  tick(ui, '[data-lshouse="900"]');
  ui.click('[data-act="lsComplete"]'); await ui.settle();
  assert.ok(toast(ui).includes('No faction is chosen'), toast(ui));
  assert.equal(sent(ui, 'complete-many').length, 0);
});

test('a refused Complete chosen (the board changed) shows the reason and keeps the choice', async () => {
  const ui = await open({ '/api/landsraad/complete-many': { __fail: 'the number of houses to complete is now 1, not the 2 you were shown, so nothing was changed: reload the page and look again', __code: 'conflict', __status: 409 } });
  tick(ui, '[data-lshouse="900"]'); tick(ui, '[data-lshouse="902"]');
  ui.click('[data-act="lsComplete"]'); await ui.settle();
  assert.ok(toast(ui).includes('nothing was changed'), toast(ui));
  assert.equal(ui.doc.querySelector('[data-lshouse="900"]').checked, true);
});

test('Reopen chosen sends the term, the houses and the count, asks nothing, and says that progress and contributions are not changed', async () => {
  const ui = await open();
  tick(ui, '[data-lshouse="900"]'); tick(ui, '[data-lshouse="901"]'); tick(ui, '[data-lshouse="902"]');
  assert.ok(sum(ui).includes('Reopen chosen would reopen 2 houses'), sum(ui));
  ui.click('[data-act="lsReopen"]'); await ui.settle();
  noAsk(ui);
  assert.deepEqual(sent(ui, 'reopen-many').map((p) => p.body), [{ term_id: 7, task_ids: [900, 901, 902], expected: 2 }]);
  const t = toast(ui);
  assert.ok(t.includes('Reopened 2 houses') && t.includes('1 house was not complete, so left alone') && t.includes('Faction progress and your own contributions were not changed'), t);
});

test('Reopen chosen with nothing complete or nothing chosen sends nothing', async () => {
  const ui = await open();
  ui.click('[data-act="lsReopen"]'); await ui.settle();
  assert.ok(toast(ui).startsWith('Nothing is chosen'), toast(ui));
  tick(ui, '[data-lshouse="900"]');
  ui.click('[data-act="lsReopen"]'); await ui.settle();
  assert.ok(toast(ui).startsWith('Nothing to change'), toast(ui));
  assert.equal(sent(ui, 'reopen-many').length, 0);
  noAsk(ui);
});

test('Choose all houses and Clear the choice still work, and the summary follows them', async () => {
  const ui = await open();
  ui.click('[data-act="lsAll"]'); await ui.settle();
  assert.ok(sum(ui).includes('3 houses') && sum(ui).includes('Reopen chosen would reopen 2 houses'), sum(ui));
  ui.click('[data-act="lsNone"]'); await ui.settle();
  assert.equal(sum(ui), 'Nothing is chosen.');
});

test('per-house Complete acts on the first press, asks nothing, and the result line names the house and the faction (#455)', async () => {
  const ui = await open({ '/api/landsraad/complete': { ok: true, outcome: 'written' } });
  const btn = ui.doc.querySelector('[data-act="tdone"][data-id="900"]');
  assert.equal(btn.textContent.trim(), 'Complete');
  ui.click('[data-act="tdone"][data-id="900"]'); await ui.settle();
  noAsk(ui);
  assert.deepEqual(sent(ui, 'complete').map((p) => p.body), [{ task_id: 900, faction_id: 2, reset: false }]);
  assert.ok(toast(ui).startsWith('Alexin completed for Harkonnen'), toast(ui));
  assert.ok(!toast(ui).includes('DA_House'), toast(ui));
});

test('per-house Complete uses the faction picked in the list', async () => {
  const ui = await open({ '/api/landsraad/complete': { ok: true, outcome: 'written' } });
  pick(ui, '#lf', '1');
  ui.click('[data-act="tdone"][data-id="900"]'); await ui.settle();
  assert.deepEqual(sent(ui, 'complete').map((p) => p.body), [{ task_id: 900, faction_id: 1, reset: false }]);
  assert.ok(toast(ui).startsWith('Alexin completed for Atreides'), toast(ui));
});

test('per-house Reopen acts on the first press, asks nothing, and says which house was reopened (#455)', async () => {
  const ui = await open({ '/api/landsraad/complete': { ok: true, outcome: 'written' } });
  const btn = ui.doc.querySelector('[data-act="tdone"][data-id="901"]');
  assert.equal(btn.textContent.trim(), 'Reopen');
  ui.click('[data-act="tdone"][data-id="901"]'); await ui.settle();
  noAsk(ui);
  assert.deepEqual(sent(ui, 'complete').map((p) => p.body), [{ task_id: 901, faction_id: 2, reset: true }]);
  assert.ok(toast(ui).startsWith('Thorvald reopened'), toast(ui));
});

test('per-house Complete refused by the editor shows the reason', async () => {
  const ui = await open({ '/api/landsraad/complete': { __fail: 'The save changed on disk.', __status: 409 } });
  ui.click('[data-act="tdone"][data-id="900"]'); await ui.settle();
  noAsk(ui);
  assert.ok(toast(ui).includes('The save changed on disk.') && !toast(ui).includes('completed'), toast(ui));
});
