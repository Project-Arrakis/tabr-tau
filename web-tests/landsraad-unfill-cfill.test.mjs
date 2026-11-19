// Unfill chosen and Complete and fill chosen (#932), in the Command Center board and in the Classic list. Real page code, the synthetic answer of the Landsraad read, no game save.
//  - Unfill chosen sets the character's own contribution to 0 on each chosen House that has one, always asks first, names what it takes back and what stays, and sends was_taken (the number of waiting
//    rewards the question said it takes back) so the server can refuse a stale question. It is off with a reason while another faction is chosen.
//  - Complete and fill chosen is Fill faction goal on each chosen House: the same request items a click on one House sends. It is off with a reason while another faction is chosen.
// The request shapes of the routes are the old ones; only the items below are new (see lcc-board-requests.golden.json for the recorded scenarios).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 0, contribution_lowered: 1, completed: 0, reopened: 0, reward_rows: 0, rewards_taken_back: 1, rewards_kept_collected: 1, outcome: 'written' };
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : null);
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const task = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name);
const writes = (ui) => ui.posted.filter((p) => p.path === '/api/landsraad/apply');
const toasts = (ui) => [...ui.doc.querySelectorAll('.d-toast')].map((t) => text(t));

async function open(view, mutate) {
  const a = synthetic();
  if (mutate) mutate(a);
  const ui = await boot({ get: { '/api/landsraad': a }, post: { '/api/landsraad/apply': OK }, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': 'grid' } });
  await ui.settle();
  return { ui, a };
}
// a tick in the Choose box of a House, in the view that is open
const pick = (ui, a, view, names) => {
  for (const n of names) {
    const c = ui.doc.querySelector(view === 'board' ? '.lcc-bd-pick[data-id="' + task(a, n).id + '"]' : '#lsrow-' + task(a, n).id + ' .pick');
    assert.ok(c, 'no Choose box for ' + n);
    c.checked = true; c.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  }
};
const ask = (ui) => text(ui.doc.querySelector('#lsb-confirm b'));
const said = (ui, view) => (view === 'board' ? toasts(ui).join(' | ') : text(ui.doc.querySelector('#lsb-result')));

for (const view of ['board', 'classic']) {
  const V = ' (' + view + ')';

  test('Unfill chosen always asks, even for one House, names what it takes back and what stays, and writes nothing until Yes' + V, async () => {
    const { ui, a } = await open(view);
    pick(ui, a, view, ['Alexin']); // 2,500: tier 1 collected (stays), tier 2 waits and the editor wrote it (taken back)
    click(ui, '[data-fk="b-unfill"]'); await ui.settle();
    const q = ask(ui);
    assert.ok(q, 'a question is asked');
    assert.match(q, /^Unfill 1 house\? Your contribution goes to 0 on 1 house: every tier you have reached there comes off\./);
    assert.match(q, /Takes back 1 waiting reward: 1 SYNTH_SCHEMATIC_1 on House Alexin\./);
    assert.match(q, /1 reward already collected stays: 10,000 Solari on House Alexin\./);
    assert.match(q, /A House that is taken stays taken, and so does your faction.s progress\./);
    assert.match(q, /Solari, House Scrip and XP the missions of the tier paid, and raising the tier again pays no mission a second time/);
    assert.match(q, /Written at once, with a backup first\.$/);
    assert.deepEqual([...ui.doc.querySelectorAll('#lsb-confirm button')].map(text), ['Unfill 1 house', 'Cancel']);
    assert.equal(writes(ui).length, 0, 'asking writes nothing');
    assert.equal(ui.doc.activeElement.dataset.fk, 'c-no', 'the safe answer has the focus');
    click(ui, '[data-fk="c-no"]'); await ui.settle();
    assert.equal(writes(ui).length, 0, 'Cancel writes nothing');
    click(ui, '[data-fk="b-unfill"]'); await ui.settle();
    click(ui, '[data-fk="c-yes"]'); await ui.settle();
    assert.deepEqual(writes(ui).map((w) => w.body), [{ term_id: 7, houses: [{ task_id: task(a, 'Alexin').id, was_mine: 2500, was_done: true, was_win: 1, mine_to: 0, lower: true, was_taken: 1, was_events: 2500 }] }]);
    assert.match(said(ui, view), /Unfill chosen: your contribution 2,500 → 0 \(tiers 1 to 2 removed\)/);
  });

  test('Unfill chosen on several Houses sends each House set to 0 with its own number of rewards, leaves out a House with no contribution of yours, and says what it leaves alone' + V, async () => {
    const { ui, a } = await open(view);
    pick(ui, a, view, ['Alexin', 'Wallach', 'Moritani', 'Sor']); // Sor has no contribution of yours
    click(ui, '[data-fk="b-unfill"]'); await ui.settle();
    const q = ask(ui);
    assert.match(q, /^Unfill 3 houses\? Your contribution goes to 0 on 3 houses/);
    assert.match(q, /Takes back 5 waiting rewards: /, 'one from Alexin, one from Moritani, three from Wallach');
    assert.match(q, /1 other chosen house is left alone: nothing to change for it\./);
    click(ui, '[data-fk="c-yes"]'); await ui.settle();
    const sent = writes(ui);
    assert.equal(sent.length, 1, 'one request, one edit');
    const items = sent[0].body.houses;
    assert.deepEqual(items.map((i) => [i.task_id, i.mine_to, i.lower, i.was_taken]).sort(), [[task(a, 'Alexin').id, 0, true, 1], [task(a, 'Moritani').id, 0, true, 1], [task(a, 'Wallach').id, 0, true, 3]].sort());
    for (const i of items) assert.ok(!('complete_for' in i) && !('reopen' in i), 'an unfill never completes or reopens: ' + JSON.stringify(i));
  });

  test('Unfill chosen is off with a reason while another faction is chosen, and writes nothing' + V, async () => {
    const { ui, a } = await open(view);
    select(ui, '#lsb-fac', 2); await ui.settle();
    const b = ui.doc.querySelector('[data-fk="b-unfill"]');
    assert.equal(b.getAttribute('aria-disabled'), 'true');
    assert.match(b.getAttribute('data-why'), /^Unfill chosen is off while Harkonnen is chosen: it changes your own contribution, which counts for Atreides\. Choose Atreides above\.$/);
    pick(ui, a, view, ['Alexin', 'Wallach']);
    click(ui, b); await ui.settle();
    assert.equal(ui.doc.querySelector('#lsb-confirm'), null, 'no question');
    assert.equal(writes(ui).length, 0, 'no request');
    assert.match(said(ui, view), /Unfill chosen is off while Harkonnen is chosen/);
    // Complete chosen still acts for the other faction
    click(ui, '[data-fk="b-complete"]'); await ui.settle();
    assert.ok(ask(ui), 'Complete chosen asks as before');
  });

  test('Unfill chosen refuses, and writes nothing, when the read does not say what it would do to the rewards' + V, async () => {
    const { ui, a } = await open(view, (d) => { delete task(d, 'Alexin').unfill; });
    pick(ui, a, view, ['Alexin']);
    click(ui, '[data-fk="b-unfill"]'); await ui.settle();
    assert.equal(ui.doc.querySelector('#lsb-confirm'), null, 'no question: it could not be honest');
    assert.equal(writes(ui).length, 0);
    assert.match(said(ui, view), /Unfill chosen cannot say what it would do to the rewards, so nothing was changed/);
  });

  test('Unfill chosen asks again, and writes nothing, when what it would take back changed since the question' + V, async () => {
    const { ui, a } = await open(view);
    pick(ui, a, view, ['Alexin']);
    click(ui, '[data-fk="b-unfill"]'); await ui.settle();
    const q = ask(ui);
    task(a, 'Alexin').unfill.rows[1].outcome = 'collected'; // the board was read again meanwhile: the reward was collected, so nothing would be taken back now
    click(ui, '[data-fk="c-yes"]'); await ui.settle();
    assert.equal(writes(ui).length, 0, 'nothing is written on a question that is not the one shown');
    assert.notEqual(ask(ui), q, 'the question is asked again with the number as it is now');
    assert.doesNotMatch(ask(ui), /Takes back/);
    assert.match(ask(ui), /2 rewards already collected stay/);
  });

  test('Unfill chosen says nothing about a House that is not chosen: a House that has no contribution is not in the request at all' + V, async () => {
    const { ui, a } = await open(view);
    pick(ui, a, view, ['Sor']);
    click(ui, '[data-fk="b-unfill"]'); await ui.settle();
    assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
    assert.equal(writes(ui).length, 0);
    assert.match(said(ui, view), /Unfill chosen: nothing to change on the chosen houses\./);
  });

  test('Complete and fill chosen asks, says what it raises and what it takes, and sends the same item for each House as Fill faction goal on that House' + V, async () => {
    const { ui, a } = await open(view);
    pick(ui, a, view, ['Lindaren', 'Sor', 'Kenola', 'Hagal']); // Hagal is at the top tier and already complete for Atreides: nothing to change, so it is left alone
    click(ui, '[data-fk="b-cfill"]'); await ui.settle();
    const q = ask(ui);
    assert.match(q, /^Complete and fill 3 houses for Atreides\? Raises your contribution to the top tier on 3 houses \(never lowered\) and takes 3 houses for Atreides: its progress goes to the goal\./);
    assert.match(q, /Writes \d+ missions?: /, 'the missions the fill pays are said, as for Fill chosen');
    assert.match(q, /1 other chosen house is left alone/);
    assert.equal(writes(ui).length, 0);
    click(ui, '[data-fk="c-yes"]'); await ui.settle();
    assert.equal(writes(ui).length, 1, 'one write, one backup, one result line');
    const items = writes(ui)[0].body.houses;
    const goal = (name) => { const t = task(a, name); return { task_id: t.id, was_mine: t.my_contribution, was_done: t.completed === 1, was_win: t.winning_faction_id, mine_to: 6000, was_events: t.my_events, was_paid: t.my_paid, was_paid_rows: t.my_paid_rows, was_elsewhere: t.my_elsewhere, complete_for: 1 }; };
    const byId = (x, y) => x.task_id - y.task_id;
    assert.deepEqual([...items].sort(byId), ['Lindaren', 'Sor', 'Kenola'].map(goal).sort(byId), 'each item is the one Fill faction goal sends for that House, with the stale-ground fields of a raise');
    for (const i of items) assert.ok('was_paid' in i && 'was_paid_rows' in i && 'was_elsewhere' in i && 'was_events' in i);
  });

  test('Complete and fill chosen is off with a reason while another faction is chosen, and Complete chosen still completes for that faction' + V, async () => {
    const { ui, a } = await open(view);
    select(ui, '#lsb-fac', 2); await ui.settle();
    const b = ui.doc.querySelector('[data-fk="b-cfill"]');
    assert.equal(b.getAttribute('aria-disabled'), 'true');
    assert.match(b.getAttribute('data-why'), /^Complete and fill chosen is off while Harkonnen is chosen: it changes your own contribution, which counts for Atreides\. Choose Atreides above, or use Complete chosen for Harkonnen\.$/);
    pick(ui, a, view, ['Lindaren', 'Sor']);
    click(ui, b); await ui.settle();
    assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
    assert.equal(writes(ui).length, 0, 'it neither fills nor completes');
    assert.match(said(ui, view), /Complete and fill chosen is off while Harkonnen is chosen/);
    click(ui, '[data-fk="b-complete"]'); await ui.settle();
    click(ui, '[data-fk="c-yes"]'); await ui.settle();
    assert.deepEqual(writes(ui)[0].body.houses.map((i) => [i.complete_for, 'mine_to' in i]), [[2, false], [2, false]], 'Complete chosen acts for Harkonnen and touches no contribution');
  });

  test('Complete and fill chosen names the win it replaces, in both views' + V, async () => {
    const { ui, a } = await open(view);
    pick(ui, a, view, ['Vernius', 'Lindaren']); // Vernius was taken by Harkonnen
    click(ui, '[data-fk="b-cfill"]'); await ui.settle();
    const q = ask(ui);
    assert.match(q, /Replaces the win of Harkonnen on House Vernius\./);
    assert.match(q, /Raises your contribution to the top tier on 2 houses/);
    assert.match(q, /takes 2 houses for Atreides/);
  });

  test('Complete and fill chosen leaves a House alone when it is complete for the faction and at the top tier' + V, async () => {
    const { ui, a } = await open(view);
    pick(ui, a, view, ['Hagal', 'Tseida']); // Hagal: 6,000 and complete for Atreides; Tseida: complete for Atreides, none of yours: only the fill is left
    click(ui, '[data-fk="b-cfill"]'); await ui.settle();
    const q = ask(ui);
    assert.match(q, /^Complete and fill 1 house for Atreides\?/);
    assert.match(q, /Raises your contribution to the top tier on 1 house \(never lowered\) and takes 0 houses for Atreides/);
    assert.match(q, /1 other chosen house is left alone/);
  });
}

test('the bar of the chosen Houses for a phone has the two buttons, presses the action bar\'s own, and shows them off with the same rule', async () => {
  const { ui, a } = await open('board');
  const bar = ui.doc.querySelector('#lsb-phonebar');
  assert.deepEqual([...bar.querySelectorAll('button')].map((b) => [text(b), b.dataset.pb]), [['Fill chosen', 'fill'], ['Complete chosen', 'complete'], ['Complete and fill chosen', 'cfill'], ['Reopen chosen', 'reopen'], ['Unfill chosen', 'unfill']]);
  pick(ui, a, 'board', ['Alexin']);
  click(ui, '[data-pb="unfill"]'); await ui.settle();
  assert.match(ask(ui), /^Unfill 1 house\?/);
  assert.equal(writes(ui).length, 0);
  click(ui, '[data-fk="c-no"]'); await ui.settle();
  select(ui, '#lsb-fac', 2); await ui.settle();
  for (const k of ['cfill', 'unfill']) assert.equal(ui.doc.querySelector('[data-pb="' + k + '"]').getAttribute('aria-disabled'), 'true', k + ' is off on the phone bar too');
  for (const k of ['fill', 'complete', 'reopen']) assert.equal(ui.doc.querySelector('[data-pb="' + k + '"]').getAttribute('aria-disabled'), null);
});

test('the count line says how many Houses each button changes, the new ones included', async () => {
  const { ui, a } = await open('classic');
  pick(ui, a, 'classic', ['Alexin', 'Sor', 'Hagal']);
  assert.match(text(ui.doc.querySelector('#lsb-sum')), /^Chosen: 3 houses\. Fill chosen changes 2, Complete chosen changes 1 for Atreides, Complete and fill chosen changes 2, Reopen chosen changes 2, Unfill chosen changes 2\.$/);
});

test('a character in no faction: both buttons are off and say why', async () => {
  const { ui } = await open('board', (d) => { d.character.faction_id = null; });
  for (const k of ['cfill', 'unfill']) {
    const b = ui.doc.querySelector('[data-fk="b-' + k + '"]');
    assert.equal(b.getAttribute('aria-disabled'), 'true');
    assert.equal(b.getAttribute('data-why'), 'Your character is in no faction, so there is no contribution of yours to change.');
  }
});
