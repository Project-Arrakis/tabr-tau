// The board writes at once (#641): every click on a tier, Fill, Complete, Reopen, Fill faction goal and every bulk button sends its change as one edit, shows what it wrote, and reads
// the board again from the save. There is no pending state, no Undo and no Write button. A bulk button for more than one house asks first.
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
const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 2, events: 1, outcome: 'written' };
async function open(data = board(), post = {}) {
  const ui = await boot({ get: { '/api/landsraad': data }, post });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}
const by = (ui, name) => [...ui.doc.querySelectorAll('#lsb-rows > li')].find((r) => r.querySelector('.hn b').textContent === name);
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const tier = (ui, name, i) => by(ui, name).querySelector(`.meter .mt[data-t="${i}"]`);
const quick = (ui, name, which) => by(ui, name).querySelector('.c-qa .' + which);
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const writes = (ui) => ui.posted.filter((p) => p.path === '/api/landsraad/apply');
const result = (ui) => text(ui.doc.querySelector('#lsb-result'));
const toasts = (ui) => [...ui.doc.querySelectorAll('.d-toast')].map((t) => text(t).replace(/×$/, ''));
const boardFetches = (ui) => ui.fetched.filter((p) => p === '/api/landsraad').length;
// holds the answer of the next write until release() is called, so a test can look at the page while a write runs
function hold(ui) {
  const orig = ui.w.fetch; const h = { release: null, started: 0 };
  ui.w.fetch = (p, opt = {}) => {
    if (opt.method === 'POST' && String(p).includes('/api/landsraad/apply')) { h.started++; return new Promise((res) => { h.release = () => res(orig(p, opt)); }); }
    return orig(p, opt);
  };
  return h;
}

test('a tier click writes at once: one request with what the page showed, no staged state, and the board is read again', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  const before = boardFetches(ui);
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  assert.deepEqual(writes(ui).map((w) => w.body), [{ term_id: 7, houses: [{ task_id: 901, was_mine: 4700, was_done: false, was_win: null, mine_to: 6000 }] }]);
  assert.equal(boardFetches(ui), before + 1, 'the row is read again from the save');
  assert.ok(result(ui).startsWith('House Thorvald: your contribution 4,700 → 6,000 (tiers 4 to 5 reached); 2 reward rows written'), result(ui));
  assert.ok(toasts(ui).some((t) => t.includes('House Thorvald: your contribution 4,700 → 6,000')), 'a toast says what was written');
  // nothing of the staging is left on the page
  for (const sel of ['.pendtag', '.undo', '.undoq', '[data-lb=write]', '[data-lb=undo]', '[data-lb=undoall]', '#lsb-pendbar', '.pend', '[data-pend]', '.src-write', '.tc.add', '.tc.rem']) assert.equal(ui.doc.querySelector(sel), null, sel + ' is gone');
  const shown = ui.doc.querySelector('#lsb').cloneNode(true); shown.querySelectorAll('details.about').forEach((d) => d.remove()); // the About text may say there is none
  assert.ok(!/Pending|Undo|Write [0-9]|Write button|WRITES/.test(text(shown)), "no word of the staging on the board");
  assert.equal(ui.doc.activeElement.dataset.fk, 'mt:901:5', 'the focus stays on the control that was pressed');
});

test('the highest reached tier clicked again is a removal sent with the flag that allows lowering; a lower reached tier writes nothing and says how to remove', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0 } });
  click(ui, tier(ui, 'Wayku', 2)); await ui.settle();
  assert.deepEqual(writes(ui), []);
  assert.ok(result(ui).includes('click the highest reached tier (tier 5)'), result(ui));
  click(ui, tier(ui, 'Wayku', 5)); await ui.settle(); // 6,500 is above tier 5: removing tier 5 drops it to tier 4 (4,800)
  assert.deepEqual(writes(ui).map((w) => w.body.houses), [[{ task_id: 903, was_mine: 6500, was_done: true, was_win: 1, mine_to: 4800, lower: true }]]);
  assert.ok(result(ui).includes('6,500 → 4,800 (tier 5 removed)'), result(ui));
});

test('Fill is off where nothing would change (nothing is sent, the reason is said); Fill writes the top tier and never lowers', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  const off = quick(ui, 'Wayku', 'qa-fill'); // 6,500 is above the top tier
  assert.equal(off.getAttribute('aria-disabled'), 'true');
  click(ui, off); await ui.settle();
  assert.deepEqual(writes(ui), []);
  assert.ok(result(ui).includes('already at the top tier'), result(ui));
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  assert.deepEqual(writes(ui).map((w) => w.body.houses), [[{ task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000 }]]);
});

test('Complete, Reopen and Fill faction goal write at once for the faction in the list, which starts on the character own faction', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0 } });
  assert.equal(ui.doc.querySelector('#lsb-fac').value, '1');
  click(ui, quick(ui, 'Thorvald', 'qa-cr')); await ui.settle();
  assert.deepEqual(writes(ui)[0].body.houses, [{ task_id: 901, was_mine: 4700, was_done: false, was_win: null, complete_for: 1 }]);
  assert.ok(result(ui).includes('House Thorvald: Atreides progress 3,000 → 4,000; house taken by Atreides'), result(ui));
  // a taken house shows Reopen, and it writes the reopening with the state the page showed
  assert.equal(text(quick(ui, 'Hurata', 'qa-cr')), 'Reopen');
  click(ui, quick(ui, 'Hurata', 'qa-cr')); await ui.settle();
  assert.deepEqual(writes(ui)[1].body.houses, [{ task_id: 902, was_mine: 1300, was_done: true, was_win: 2, reopen: true }]);
  assert.ok(result(ui).includes('house reopened (progress stays as it is)'), result(ui));
  // Fill faction goal in the opened row: the top tier and the win in one request
  click(ui, by(ui, 'Alexin').querySelector('.act-fill')); await ui.settle();
  assert.deepEqual(writes(ui)[2].body.houses, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000, complete_for: 1 }]);
  // another faction: the drop-down only chooses the faction of the next click, it writes nothing itself
  select(ui, '#lsb-fac', 2); await ui.settle();
  assert.equal(writes(ui).length, 3);
  click(ui, quick(ui, 'Alexin', 'qa-cr')); await ui.settle();
  assert.deepEqual(writes(ui)[3].body.houses, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, complete_for: 2 }]);
});

test('the tooltip and the About text of Reopen say it does not set back what Complete wrote', async () => {
  const ui = await open();
  const tip = quick(ui, 'Hurata', 'qa-cr').title;
  assert.ok(/progress stays at the goal/.test(tip) && /not given back/.test(tip), tip);
  assert.ok(by(ui, 'Hurata').querySelector('.act-reopen').title.includes('progress stays at the goal'));
  assert.ok(ui.doc.querySelector('[data-lb=bulk][data-do=reopen]').title.includes('does not set back what Complete wrote'));
  const about = text(ui.doc.querySelector('[data-about="lsb.fill"] .abt'));
  assert.ok(about.includes('Reopen does not undo a Complete completely') && about.includes('writes or removes no reward rows'), about);
});

test('while a write runs the control is busy and disabled, a second click is ignored with a line that says so, and the board is usable again after it', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  const h = hold(ui);
  const btn = tier(ui, 'Thorvald', 5);
  click(ui, btn);
  await new Promise((r) => ui.w.setTimeout(r, 5));
  assert.equal(h.started, 1);
  assert.equal(btn.getAttribute('aria-disabled'), 'true');
  assert.ok(btn.classList.contains('busy'));
  assert.equal(ui.doc.querySelector('#lsb').getAttribute('aria-busy'), 'true');
  assert.equal(result(ui), 'Writing…');
  assert.equal(ui.doc.querySelector('#lsb-fac').disabled, true, 'the faction cannot change mid-write');
  click(ui, btn); click(ui, quick(ui, 'Alexin', 'qa-fill')); click(ui, '[data-lb=pickall]'); click(ui, '[data-lb=bulk][data-do=complete]');
  await new Promise((r) => ui.w.setTimeout(r, 5));
  assert.equal(h.started, 1, 'a second click did not start a second write');
  assert.ok(result(ui).includes('still running'), result(ui));
  h.release(); await ui.settle();
  assert.equal(writes(ui).length, 1);
  assert.equal(ui.doc.querySelector('#lsb').getAttribute('aria-busy'), null);
  assert.equal(ui.doc.querySelector('#lsb-fac').disabled, false);
  assert.ok(result(ui).startsWith('House Thorvald:'), result(ui));
  assert.equal(tier(ui, 'Thorvald', 5).getAttribute('aria-disabled'), 'false');
});

test('a refused write shows the error text unchanged, writes nothing, and leaves the control usable', async () => {
  const msg = 'house Thorvald was taken since you looked, so nothing was changed';
  const ui = await open(board(), { '/api/landsraad/apply': { __fail: msg, __code: 'conflict', __status: 409 } });
  const before = boardFetches(ui);
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  assert.equal(result(ui), msg);
  assert.ok(ui.doc.querySelector('#lsb-result').classList.contains('bad'), 'the line is marked as an error');
  assert.equal(ui.doc.querySelector('#lsb-result').getAttribute('class'), 'bad');
  assert.ok(toasts(ui).includes(msg), 'the toast has the same words');
  assert.ok(boardFetches(ui) > before, 'after a conflict the board is read again to show what the save holds now');
  assert.equal(ui.doc.querySelector('#lsb').getAttribute('aria-busy'), null);
  assert.equal(tier(ui, 'Thorvald', 5).getAttribute('aria-disabled'), 'false');
  // another kind of refusal: no re-read, the control is back, the words are unchanged
  const ui2 = await open(board(), { '/api/landsraad/apply': { __fail: 'the game is running, so nothing was written', __code: 'game_running', __status: 409 } });
  const b2 = boardFetches(ui2);
  click(ui2, quick(ui2, 'Alexin', 'qa-fill')); await ui2.settle();
  assert.equal(result(ui2), 'the game is running, so nothing was written');
  assert.equal(boardFetches(ui2), b2);
  assert.equal(quick(ui2, 'Alexin', 'qa-fill').classList.contains('busy'), false);
  assert.equal(quick(ui2, 'Alexin', 'qa-fill').getAttribute('aria-disabled'), 'false');
  // and it can be pressed again
  click(ui2, quick(ui2, 'Alexin', 'qa-fill')); await ui2.settle();
  assert.equal(writes(ui2).length, 2);
});

test('a bulk button for more than one house asks first with the count; Cancel and Escape write nothing and put the focus back; Yes writes the houses in one edit', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 3, houses: 3, contribution_raised: 0, completed: 3, reward_rows: 0 } });
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), 'Nothing is chosen.');
  click(ui, '[data-lb=pickall]');
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), 'Chosen: 4 houses. Fill chosen changes 3, Complete chosen changes 3 for Atreides, Reopen chosen changes 2.');
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle();
  const q = ui.doc.querySelector('#lsb-confirm');
  assert.ok(q, 'the question is shown');
  assert.equal(text(q.querySelector('b')), 'Complete 3 houses for Atreides?');
  assert.deepEqual([...q.querySelectorAll('button')].map(text), ['Yes', 'Cancel']);
  assert.deepEqual(writes(ui), [], 'asking writes nothing');
  assert.equal(ui.doc.activeElement.dataset.fk, 'c-no', 'the safe answer has the focus');
  click(ui, '[data-lb=cno]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.deepEqual(writes(ui), []);
  assert.equal(result(ui), 'Cancelled. Nothing was written.');
  assert.equal(ui.doc.activeElement.dataset.fk, 'b-complete');
  // Escape does the same
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle();
  ui.doc.activeElement.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.equal(ui.doc.activeElement.dataset.fk, 'b-complete');
  assert.deepEqual(writes(ui), []);
  // a change of the choice or of the faction closes the question: it named houses that are no longer the ones chosen
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle();
  by(ui, 'Alexin').querySelector('.pick').click(); await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  by(ui, 'Alexin').querySelector('.pick').click(); await ui.settle();
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle();
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.equal(writes(ui).length, 1);
  assert.deepEqual(writes(ui)[0].body, { term_id: 7, houses: [ // only the houses that change are sent: Wayku is already complete for Atreides
    { task_id: 900, was_mine: 0, was_done: false, was_win: null, complete_for: 1 },
    { task_id: 901, was_mine: 4700, was_done: false, was_win: null, complete_for: 1 },
    { task_id: 902, was_mine: 1300, was_done: true, was_win: 2, complete_for: 1 },
  ] });
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.ok(result(ui).startsWith('Complete chosen: 3 houses completed'), result(ui));
  assert.equal(ui.doc.activeElement.dataset.fk, 'b-complete');
});

test('Fill chosen and Reopen chosen ask with their own count; one chosen house is written at once without a question', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 2, houses: 2 } });
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Fill 3 houses?');
  click(ui, '[data-lb=cno]'); await ui.settle();
  click(ui, '[data-lb=bulk][data-do=reopen]'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Reopen 2 houses?');
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.deepEqual(writes(ui)[0].body.houses, [
    { task_id: 903, was_mine: 6500, was_done: true, was_win: 1, reopen: true },
    { task_id: 902, was_mine: 1300, was_done: true, was_win: 2, reopen: true },
  ]);
  // one house chosen: no question
  click(ui, '[data-lb=pickno]');
  by(ui, 'Alexin').querySelector('.pick').click(); await ui.settle();
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.equal(writes(ui).length, 2);
  assert.deepEqual(writes(ui)[1].body.houses, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000 }]);
  // nothing chosen, or nothing to change
  click(ui, '[data-lb=pickno]');
  click(ui, '[data-lb=bulk][data-do=fill]');
  assert.ok(result(ui).includes('Nothing is chosen'));
  assert.equal(writes(ui).length, 2);
});

test('with another faction chosen, Fill and Fill chosen change nothing of yours and nothing is sent; Complete and Fill faction goal act for that faction without a contribution', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0 } });
  select(ui, '#lsb-fac', 2); await ui.settle();
  const off = quick(ui, 'Alexin', 'qa-fill');
  assert.equal(off.getAttribute('aria-disabled'), 'true');
  assert.ok(off.title.includes('Off while Harkonnen is chosen') && off.title.includes('Atreides'), off.title);
  click(ui, off); await ui.settle();
  assert.deepEqual(writes(ui), [], 'Fill wrote nothing');
  by(ui, 'Alexin').querySelector('.pick').click(); by(ui, 'Thorvald').querySelector('.pick').click(); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).includes('Fill chosen changes 0,'), text(ui.doc.querySelector('#lsb-sum')));
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  assert.deepEqual(writes(ui), [], 'Fill chosen wrote nothing');
  assert.ok(result(ui).includes('nothing to change'), result(ui));
  click(ui, by(ui, 'Alexin').querySelector('.act-fill')); await ui.settle();
  assert.deepEqual(writes(ui)[0].body.houses, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, complete_for: 2 }], 'no mine_to: no contribution and so no reward row');
});

test('a character in no faction cannot change a contribution: the tiers and Fill are off with the reason, Complete still writes', async () => {
  const ui = await open(board({ character: { player_id: 1, name: 'Tester', faction_id: null } }), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0 } });
  assert.equal(tier(ui, 'Thorvald', 5).getAttribute('aria-disabled'), 'true');
  assert.ok(tier(ui, 'Thorvald', 5).title.includes('in no faction'));
  assert.equal(quick(ui, 'Thorvald', 'qa-fill').getAttribute('aria-disabled'), 'true');
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  assert.deepEqual(writes(ui), []);
  click(ui, quick(ui, 'Thorvald', 'qa-cr')); await ui.settle();
  assert.equal(writes(ui).length, 1);
  assert.equal(writes(ui)[0].body.houses[0].mine_to, undefined);
});

test('the Choose boxes of rows a filter hides are cleared', async () => {
  const ui = await open();
  click(ui, '[data-lb=pickall]');
  click(ui, '.glance .tile.done'); // only Wayku stays
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).startsWith('Chosen: 1 house.'), text(ui.doc.querySelector('#lsb-sum')));
});

test('a save with no term writes nothing and says so', async () => {
  const ui = await open(board({ term: null }));
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  assert.deepEqual(writes(ui), []);
});

test('the term controls write at once with the same busy handling and a result line', async () => {
  const ui = await open(board(), { '/api/landsraad/term': { ok: true, outcome: 'written' }, '/api/landsraad/decree': { ok: true, outcome: 'written' } });
  ui.doc.querySelector('#lsb-tdp').open = true;
  const orig = ui.w.fetch; let release = null;
  ui.w.fetch = (p, opt = {}) => (opt.method === 'POST' && String(p).includes('/api/landsraad/term') ? new Promise((res) => { release = () => res(orig(p, opt)); }) : orig(p, opt));
  const apply = ui.doc.querySelector('[data-lb=tapply]');
  click(ui, apply);
  await new Promise((r) => ui.w.setTimeout(r, 5));
  assert.equal(apply.getAttribute('aria-disabled'), 'true');
  assert.equal(text(ui.doc.querySelector('#lsb-tresult')), 'Writing…');
  assert.equal(ui.doc.querySelector('#lsb-trf').disabled, true);
  release(); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).startsWith('Applied: active decree'), text(ui.doc.querySelector('#lsb-tresult')));
  assert.equal(ui.posted.filter((p) => p.path === '/api/landsraad/term').length, 1);
  click(ui, ui.doc.querySelector('[data-lb=tflag]')); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).startsWith('Decree 1: pool flag set to'), text(ui.doc.querySelector('#lsb-tresult')));
});

test('after a write the row is drawn from the saved data, and the focus stays on the pressed control even when Complete has turned into Reopen', async () => {
  const data = board();
  const ui = await open(data, { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0 } });
  const orig = ui.w.fetch;
  ui.w.fetch = (p, opt = {}) => { // the save changes when the write is made
    if (opt.method === 'POST' && String(p).includes('/api/landsraad/apply')) { const t = data.tasks.find((x) => x.id === 901); t.completed = 1; t.winning_faction_id = 1; t.contributions = [{ faction_id: 1, amount: 4000 }]; }
    return orig(p, opt);
  };
  assert.equal(text(quick(ui, 'Thorvald', 'qa-cr')), 'Complete');
  click(ui, quick(ui, 'Thorvald', 'qa-cr')); await ui.settle();
  assert.equal(text(quick(ui, 'Thorvald', 'qa-cr')), 'Reopen', 'the row shows what the save holds now');
  assert.ok(text(by(ui, 'Thorvald').querySelector('.c-hs')).includes('Taken · Atreides (yours)'));
  assert.equal(ui.doc.activeElement.dataset.fk, 'qc:901', 'the focus is on the same place, now the Reopen button');
});

test('Cancel is ignored while the write of the bulk question runs: it does not say "Cancelled" about a write that goes on', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 3, houses: 3, contribution_raised: 0, completed: 3, reward_rows: 0 } });
  const h = hold(ui);
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle();
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.equal(h.started, 1);
  click(ui, '[data-lb=cno]');
  assert.ok(!result(ui).startsWith('Cancelled'), result(ui));
  assert.equal(result(ui), 'A write is still running. Wait for it to finish, then click again.');
  h.release(); await ui.settle();
  assert.equal(writes(ui).length, 1);
  assert.ok(result(ui).startsWith('Complete chosen: 3 houses completed'), result(ui));
});
