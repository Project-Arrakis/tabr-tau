// The board writes (part 2 of the tab rework): a click stages a change on the page, the rows show it as pending with an Undo, and the Write button sends the whole thing in one request.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const TH = [1200, 2400, 3600, 4800, 6000];
const OWN = ['SolarisCoin', 'T6ArmorPlating', 'T6RangeFinder', 'T6HoltzmanActuator', 'T6DiamodineBladeParts'];
// a house with the character at `mine`; states: the reward_state of each tier by threshold (default: reached ones are no_row, others not_reached)
const house = (id, name, mine, extra = {}, states = {}) => ({
  id, board_index: id - 900, house_name: 'DA_House' + name, goal_amount: 4000, completed: 0, winning_faction_id: null, completion_time: null,
  contributions: [], player_contributions: [], my_contribution: mine, my_recorded: mine > 0, swatch_held: false, location: { known: false },
  rewards: TH.map((threshold, i) => {
    const reached = mine >= threshold, top = i === 4;
    return { threshold, amount: top ? 75 : [10000, 1, 50, 1][i], template_id: OWN[i], reached, delivered: false,
      shown_template_id: top ? 'Alexin_Placeables_Swatch' : OWN[i], shown_amount: top ? 1 : [10000, 1, 50, 1][i], swatch: top,
      reward_state: states[threshold] || (reached ? 'no_row' : 'not_reached') };
  }),
  ...extra,
});
const board = (extra = {}) => ({
  term: { term_id: 7, start_time: 100, end_time: 200, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 },
  terms: [{ term_id: 7, start_time: 100, end_time: 200 }],
  earlier_terms: [],
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  character: { player_id: 1, name: 'Tester', faction_id: 1 },
  decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }], rotation: [],
  tasks: [
    house(903, 'Wayku', 6500, { completed: 1, winning_faction_id: 1, contributions: [{ faction_id: 1, amount: 4000 }, { faction_id: 2, amount: 1500 }] }, { 1200: 'delivered', 2400: 'delivered', 3600: 'waiting', 4800: 'waiting', 6000: 'waiting' }),
    house(900, 'Alexin', 0, { contributions: [{ faction_id: 2, amount: 900 }] }),
    house(901, 'Thorvald', 4700, { contributions: [{ faction_id: 1, amount: 3000 }, { faction_id: 2, amount: 1000 }] }),
    house(902, 'Hurata', 1300, { completed: 1, winning_faction_id: 2, contributions: [{ faction_id: 2, amount: 4000 }] }),
  ],
  ...extra,
});
async function open(data = board(), post = {}) {
  const ui = await boot({ get: { '/api/landsraad': data }, post });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}
const by = (ui, name) => [...ui.doc.querySelectorAll('#lsb-rows > li')].find((r) => r.querySelector('.hn b').textContent === name);
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const tier = (ui, name, i) => by(ui, name).querySelector(`.meter .mt[data-t="${i}"]`);
const tc = (ui, name, i) => by(ui, name).querySelector(`.more .tc[data-t="${i}"]`);
const quick = (ui, name, which) => by(ui, name).querySelector('.c-qa .' + which);
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const writes = (ui) => ui.posted.filter((p) => p.path === '/api/landsraad/apply');

test('a tier click stages the change: nothing is sent, the row shows it as pending with an Undo, and the counts follow', async () => {
  const ui = await open();
  assert.equal(+ui.doc.querySelector('.glance .tile.done .n').textContent, 1);
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle(); // 4,700 now: tiers 4 and 5 will be reached
  assert.deepEqual(writes(ui), [], 'a click stages, it does not write');
  const row = by(ui, 'Thorvald');
  assert.ok(row.hasAttribute('data-pend'));
  assert.ok(text(row.querySelector('.pendtag')).startsWith('Pending'));
  assert.ok(row.querySelector('.meter .mt[data-t="5"] i.add') && row.querySelector('.meter .mt[data-t="4"] i.add'));
  assert.equal(row.dataset.my, 'done');
  assert.equal(+ui.doc.querySelector('.glance .tile.done .n').textContent, 2, 'the tiles follow the staged state');
  assert.ok(text(row.querySelector('.more .pend')).includes('4,700 → 6,000 (tiers 4 to 5 will be reached)'), text(row.querySelector('.more .pend')));
  assert.equal(row.querySelectorAll('.more .padd li').length, 2, 'the opened row lists the rewards the two tiers would add');
  assert.ok(text(row.querySelector('.more .padd li:last-child')).includes('Swatch'), 'the 6,000 tier shows the swatch while it is not held');
  assert.ok(tc(ui, 'Thorvald', 5).classList.contains('add') && text(tc(ui, 'Thorvald', 5)).includes('will be reached'));
  assert.equal(ui.doc.querySelector('#lsb-pendbar').textContent.includes('1 house has pending changes'), true);
  // Undo on the row takes it back
  click(ui, row.querySelector('.pendtag .undoq')); await ui.settle();
  assert.ok(!by(ui, 'Thorvald').hasAttribute('data-pend'));
  assert.equal(ui.doc.querySelector('#lsb-pendbar'), null);
  assert.equal(+ui.doc.querySelector('.glance .tile.done .n').textContent, 1);
});

test('the highest reached tier clicked again is a removal, a lower reached tier says how to remove', async () => {
  const ui = await open();
  click(ui, tier(ui, 'Wayku', 5)); await ui.settle(); // 6,500 is above tier 5: removing tier 5 drops it to tier 4 (4,800)
  const row = by(ui, 'Wayku');
  assert.ok(row.querySelector('.meter .mt[data-t="5"] i.rem'));
  assert.ok(text(row.querySelector('.more .pend')).includes('6,500 → 4,800 (tier 5 will be removed)'), text(row.querySelector('.more .pend')));
  assert.ok(tc(ui, 'Wayku', 5).classList.contains('rem') && text(tc(ui, 'Wayku', 5)).includes('will be removed'));
  assert.equal(row.dataset.my, 'part');
  click(ui, tier(ui, 'Wayku', 2)); await ui.settle(); // a reached tier that is not the highest one
  assert.ok(text(ui.doc.querySelector('#lsb-result')).includes('click the highest reached tier (tier 4)'), text(ui.doc.querySelector('#lsb-result')));
  assert.deepEqual(writes(ui), []);
});

test('Fill stages the top tier and is off where nothing would change; a click on an off button stages nothing and says why', async () => {
  const ui = await open();
  const off = quick(ui, 'Wayku', 'qa-fill'); // 6,500 is above the top tier
  assert.equal(off.getAttribute('aria-disabled'), 'true');
  click(ui, off); await ui.settle();
  assert.ok(!by(ui, 'Wayku').hasAttribute('data-pend'));
  assert.ok(text(ui.doc.querySelector('#lsb-result')).includes('already at the top tier'));
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  const row = by(ui, 'Alexin');
  assert.equal(row.dataset.my, 'done');
  assert.ok(text(row.querySelector('.js-mine')).includes('6,000') || text(row.querySelector('.amt')).includes('6,000'));
  assert.equal(quick(ui, 'Alexin', 'qa-fill').getAttribute('aria-disabled'), 'true', 'now it is at the top tier');
  assert.ok(row.hasAttribute('data-pend') && !text(row.querySelector('.c-hs')).includes('Taken'), 'Fill does not take the house');
});

test('Complete and Reopen stage for the faction in the list, which starts on the character own faction', async () => {
  const ui = await open();
  assert.equal(ui.doc.querySelector('#lsb-fac').value, '1');
  assert.ok(text(ui.doc.querySelector('#lsb-fac').selectedOptions[0]).includes('(you)'));
  click(ui, quick(ui, 'Thorvald', 'qa-cr')); await ui.settle();
  let row = by(ui, 'Thorvald');
  assert.ok(text(row.querySelector('.c-hs')).includes('Taken · Atreides (yours)'));
  assert.ok(text(row.querySelector('.more .pend')).includes('Atreides progress 3,000 → 4,000; house taken by Atreides'), text(row.querySelector('.more .pend')));
  assert.equal(text(row.querySelector('.c-qa .qa-cr')), 'Reopen', 'the button turns into Reopen once the house is taken');
  click(ui, quick(ui, 'Thorvald', 'qa-cr')); await ui.settle(); // Reopen of what was only staged: back to the saved state
  assert.ok(!by(ui, 'Thorvald').hasAttribute('data-pend'));
  // another faction: the list is changed to Harkonnen, then Complete on an open house
  select(ui, '#lsb-fac', 2); await ui.settle();
  click(ui, quick(ui, 'Alexin', 'qa-cr')); await ui.settle();
  row = by(ui, 'Alexin');
  assert.ok(text(row.querySelector('.c-hs')).includes('Taken · Harkonnen'));
  assert.ok(row.hasAttribute('data-pend'));
  // a saved house taken by Harkonnen: Reopen stages the reopening
  click(ui, quick(ui, 'Hurata', 'qa-cr')); await ui.settle();
  assert.ok(text(by(ui, 'Hurata').querySelector('.more .pend')).includes('house reopened'));
});

test('Write sends every staged house in one request, with what the page showed, and clears the staging', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ok: true, changed: 3, houses: 3, skipped: 0, contribution_raised: 2, contribution_lowered: 0, completed: 1, reopened: 0, reward_rows: 5, events: 2 } });
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  click(ui, quick(ui, 'Thorvald', 'qa-cr')); await ui.settle();
  click(ui, ui.doc.querySelector('[data-lb=write]')); await ui.settle();
  const w = writes(ui);
  assert.equal(w.length, 1);
  assert.equal(w[0].body.term_id, 7);
  assert.deepEqual(w[0].body.houses, [
    { task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000 },
    { task_id: 901, was_mine: 4700, was_done: false, was_win: null, mine_to: 6000, complete_for: 1 },
  ]);
  assert.equal(ui.doc.querySelector('#lsb-pendbar'), null, 'the staging is cleared once it is written');
  assert.ok(ui.doc.body.textContent.includes('Board written'), 'the answer is reported');
  assert.ok(ui.doc.body.textContent.includes('5 reward rows written'), 'the reward rows are reported');
});

test('a removal is sent with the flag that allows lowering; Fill after a removal goes back to what the save holds and never below it', async () => {
  const ui = await open();
  click(ui, tier(ui, 'Wayku', 5)); await ui.settle();
  click(ui, ui.doc.querySelector('[data-lb=write]')); await ui.settle();
  assert.deepEqual(writes(ui)[0].body.houses, [{ task_id: 903, was_mine: 6500, was_done: true, was_win: 1, mine_to: 4800, lower: true }]);
  const ui2 = await open();
  click(ui2, tier(ui2, 'Wayku', 5)); await ui2.settle();
  click(ui2, quick(ui2, 'Wayku', 'qa-fill')); await ui2.settle(); // 6,500 is above the top tier: Fill cancels the removal, it does not set 6,000
  assert.ok(!by(ui2, 'Wayku').hasAttribute('data-pend'));
  assert.equal(ui2.doc.querySelector('#lsb-pendbar'), null);
});

test('a refused Write keeps the staging and shows the reason', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { __fail: 'house Thorvald was taken since you looked, so nothing was changed', __code: 'conflict', __status: 409 } });
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  click(ui, ui.doc.querySelector('[data-lb=write]')); await ui.settle();
  assert.ok(ui.doc.body.textContent.includes('was taken since you looked'));
  assert.ok(by(ui, 'Thorvald').hasAttribute('data-pend'), 'the staged change is still there');
});

test('Undo all drops every staged change', async () => {
  const ui = await open();
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  click(ui, quick(ui, 'Thorvald', 'qa-cr')); await ui.settle();
  assert.ok(ui.doc.querySelector('#lsb-pendbar').textContent.includes('2 houses have pending changes'));
  click(ui, ui.doc.querySelector('[data-lb=undoall]')); await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-pendbar'), null);
  assert.equal(ui.doc.querySelectorAll('#lsb-rows > li[data-pend]').length, 0);
});

test('the bulk bar acts on the chosen houses, says how many each button changes, and stages the same changes as the row buttons', async () => {
  const ui = await open();
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), 'Nothing is chosen.');
  click(ui, '[data-lb=pickall]');
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), 'Chosen: 4 houses. Fill chosen changes 3, Complete chosen changes 3 for Atreides, Reopen chosen changes 2.');
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-result')).startsWith('Fill chosen: 3 houses staged, 1 house left alone'), text(ui.doc.querySelector('#lsb-result')));
  assert.equal(ui.doc.querySelectorAll('#lsb-rows > li[data-pend]').length, 3);
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-result')).startsWith('Complete chosen for Atreides: 3 houses staged'), text(ui.doc.querySelector('#lsb-result')));
  click(ui, '[data-lb=bulk][data-do=reopen]'); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-result')).startsWith('Reopen chosen:'));
  click(ui, '[data-lb=pickno]');
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), 'Nothing is chosen.');
  click(ui, '[data-lb=bulk][data-do=fill]');
  assert.ok(text(ui.doc.querySelector('#lsb-result')).includes('Nothing is chosen'));
  assert.deepEqual(writes(ui), []);
});

test('the Choose boxes of rows a filter hides are cleared', async () => {
  const ui = await open();
  click(ui, '[data-lb=pickall]');
  click(ui, '.glance .tile.done'); // only Wayku stays
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).startsWith('Chosen: 1 house.'), text(ui.doc.querySelector('#lsb-sum')));
});

test('the control that was pressed keeps the focus when the board is drawn again', async () => {
  const ui = await open();
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  assert.equal(ui.doc.activeElement.dataset.fk, 'mt:901:5');
});

test('a character in no faction cannot change a contribution: the tiers and Fill are off with the reason, Complete still works', async () => {
  const ui = await open(board({ character: { player_id: 1, name: 'Tester', faction_id: null } }));
  assert.equal(tier(ui, 'Thorvald', 5).getAttribute('aria-disabled'), 'true');
  assert.ok(tier(ui, 'Thorvald', 5).title.includes('in no faction'));
  assert.equal(quick(ui, 'Thorvald', 'qa-fill').getAttribute('aria-disabled'), 'true');
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  assert.ok(!by(ui, 'Thorvald').hasAttribute('data-pend'));
  click(ui, quick(ui, 'Thorvald', 'qa-cr')); await ui.settle();
  assert.ok(by(ui, 'Thorvald').hasAttribute('data-pend'));
});

test('Fill faction goal in the opened row stages the top tier and the win; for another faction it stages the win only', async () => {
  const ui = await open();
  click(ui, by(ui, 'Alexin').querySelector('.act-fill')); await ui.settle();
  assert.ok(text(by(ui, 'Alexin').querySelector('.more .pend')).includes('0 → 6,000'));
  assert.ok(text(by(ui, 'Alexin').querySelector('.more .pend')).includes('house taken by Atreides'));
  select(ui, '#lsb-fac', 2); await ui.settle();
  click(ui, by(ui, 'Thorvald').querySelector('.act-fill')); await ui.settle();
  const p = text(by(ui, 'Thorvald').querySelector('.more .pend'));
  assert.ok(p.includes('house taken by Harkonnen') && !p.includes('your contribution'), p);
});

test('with another faction chosen, Fill and Fill chosen change nothing of yours and nothing is sent for the contribution; Complete and Fill faction goal act for that faction', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 0, contribution_lowered: 0, completed: 1, reopened: 0, reward_rows: 0, events: 0 } });
  select(ui, '#lsb-fac', 2); await ui.settle();
  const off = quick(ui, 'Alexin', 'qa-fill');
  assert.equal(off.getAttribute('aria-disabled'), 'true');
  assert.ok(off.title.includes('Off while Harkonnen is chosen') && off.title.includes('Atreides'), off.title);
  click(ui, off); await ui.settle();
  assert.ok(!by(ui, 'Alexin').hasAttribute('data-pend'), 'Fill staged nothing');
  by(ui, 'Alexin').querySelector('.pick').click(); by(ui, 'Thorvald').querySelector('.pick').click(); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).includes('Fill chosen changes 0,'), text(ui.doc.querySelector('#lsb-sum')));
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('#lsb-rows > li[data-pend]').length, 0, 'Fill chosen staged nothing');
  click(ui, by(ui, 'Alexin').querySelector('.act-fill')); await ui.settle();
  click(ui, ui.doc.querySelector('[data-lb=write]')); await ui.settle();
  const w = writes(ui);
  assert.deepEqual(w[0].body.houses, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, complete_for: 2 }], 'no mine_to: no contribution and so no reward row');
});
