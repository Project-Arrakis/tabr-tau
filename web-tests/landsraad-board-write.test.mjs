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
const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 2, outcome: 'written' };
async function open(data = board(), post = {}) {
  const ui = await boot({ get: { '/api/landsraad': data }, post });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}
const by = (ui, name) => [...ui.doc.querySelectorAll('#lsb-rows > li')].find((r) => r.querySelector('.hn b').textContent === name);
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
// the tier boxes are in the opened row: open it, as a person does
const tier = (ui, name, i) => { const r = by(ui, name), x = r.querySelector('.xc'); if (!x.checked) { x.checked = true; x.dispatchEvent(new ui.w.Event('change', { bubbles: true })); } return r.querySelector(`.tc[data-t="${i}"]`); };
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
  assert.ok(result(ui).startsWith('House Thorvald: your contribution 4,700 → 6,000 (tiers 4 to 5 reached); 2 rewards now waiting at the house representative'), result(ui));
  assert.ok(toasts(ui).some((t) => t.includes('House Thorvald: your contribution 4,700 → 6,000')), 'a toast says what was written');
  // nothing of the staging is left on the page
  for (const sel of ['.pendtag', '.undo', '.undoq', '[data-lb=write]', '[data-lb=undo]', '[data-lb=undoall]', '#lsb-pendbar', '.pend', '[data-pend]', '.src-write', '.tc.add', '.tc.rem']) assert.equal(ui.doc.querySelector(sel), null, sel + ' is gone');
  const shown = ui.doc.querySelector('#lsb').cloneNode(true); shown.querySelectorAll('details.about').forEach((d) => d.remove()); // the About text may say there is none
  assert.ok(!/Pending|Undo|Write [0-9]|Write button|WRITES/.test(text(shown)), "no word of the staging on the board");
  assert.equal(ui.doc.activeElement.dataset.fk, 'tc:901:5', 'the focus stays on the control that was pressed');
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
  assert.ok(about.includes('It does not undo a Complete completely') && about.includes('writes or removes no reward'), about);
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
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), '', 'no standing sentence when nothing is chosen (#1062)');
  click(ui, '[data-lb=pickall]');
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), 'Chosen: 4 houses. Fill chosen changes 3, Complete chosen changes 3 for Atreides, Complete and fill chosen changes 3, Reopen chosen changes 2, Unfill chosen changes 0.'); // this read has no unfill plan (#1013): Unfill refuses on such a read, so it counts no House
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle();
  const q = ui.doc.querySelector('#lsb-confirm');
  assert.ok(q, 'the question is shown');
  assert.equal(text(q.querySelector('b')), 'Complete 3 houses for Atreides? Written at once, with a backup first.');
  assert.deepEqual([...q.querySelectorAll('button')].map(text), ['Complete 3 houses', 'Cancel']);
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
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Fill 3 houses? Written at once, with a backup first.');
  click(ui, '[data-lb=cno]'); await ui.settle();
  click(ui, '[data-lb=bulk][data-do=reopen]'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Reopen 2 houses? Written at once, with a backup first.');
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
  assert.ok(text(by(ui, 'Thorvald').querySelector('.c-hs')).includes('Pledged to Atreides (your faction)'));
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

// What a raise grants (refs #609, #652): said before the click, written in the result, and the warning when rewards were left out because they were already delivered (#656).
const GRANT = { step: 1000, solari: 6000, scrip: 1000, xp: 1500, max_missions: 1000, problem: '' };
const LINES = { Wayku: 'Gathering', Alexin: 'Crafting', Thorvald: 'Combat', Hurata: 'Exploration' };
// the board read as the editor sends it: every house with its line and the contribution its own events cover, and the grant object
const withGrant = (g = {}, events = {}) => {
  const b = board();
  b.tasks.forEach((t) => { t.line = LINES[t.house_name.replace('DA_House', '')]; t.my_events = events[t.id] || 0; });
  b.grant = { ...GRANT, ...g };
  return b;
};
const GRANTED = { ...OK, completed: 1, reward_rows: 3, missions: 3, solari: 18000, house_scrip: 3000, specialization_xp: { Exploration: 9000 }, rewards_already_given: 0 };

test('the result line says what was written, in words that do not claim the pay-out is checked, and the toast is one short line', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': GRANTED });
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('3 rewards now waiting at the house representative; 3 missions written: +18,000 Solari, +3,000 House Scrip, +9,000 Exploration XP'), r);
  assert.ok(r.includes('house taken, its faction reached the goal'), r);
  assert.ok(r.includes('has not been checked in the game yet'), 'the pay-out is said to be unchecked: ' + r);
  assert.ok(!/granted|Warning/.test(r), r);
  assert.deepEqual(toasts(ui), ['House Alexin: 3 missions written.']);
  assert.equal(ui.doc.querySelector('#lsb-result').getAttribute('class'), null, 'a plain success is not drawn as a warning');
  assert.equal(ui.doc.querySelector('#sr').textContent, r, 'the result is announced');
});

test('while the game runs the result says the change is queued, never that anything was written or granted', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...GRANTED, outcome: 'queued' } });
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('3 missions to write: +18,000 Solari') && r.includes('3 rewards to leave waiting at the house representative'), r);
  assert.ok(r.includes('queued: written as soon as the save can be written'), 'the hint is in the result line, not only in the toast: ' + r);
  assert.ok(!/granted|missions written|now waiting/.test(r), r);
  assert.equal(ui.doc.querySelector('#lsb-result').className, 'warn', 'a queued change is not drawn as a success');
  const t = ui.doc.querySelector('.d-toast');
  assert.ok(t.classList.contains('note') && t.textContent.includes('queued') && !t.textContent.includes('written.'), t.className + ' ' + t.textContent);
});

test('a raise whose rewards were already delivered says so as a warning with a next step, and says that no missions were written', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, reward_rows: 0, missions: 0, solari: 0, house_scrip: 0, specialization_xp: {}, rewards_already_given: 2 } });
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('No missions written: your own earlier progress on this house already counts for this ground'), r);
  assert.ok(r.includes('Warning: 2 rewards were left out: earlier progress on this house already counted for them. Nothing to do'), r);
  assert.ok(!/granted|tiers? not written|delivered/.test(r), r);
  assert.equal(ui.doc.querySelector('#lsb-result').className, 'warn');
  const t = ui.doc.querySelector('.d-toast');
  assert.ok(t.classList.contains('note') && t.textContent.includes('Warning: 2 rewards were left out'), 'the toast is a warning, not a success: ' + t.className + ' ' + t.textContent);
  const one = await open(board(), { '/api/landsraad/apply': { ...OK, reward_rows: 0, missions: 0, rewards_already_given: 1 } });
  click(one, quick(one, 'Alexin', 'qa-fill')); await one.settle();
  assert.ok(result(one).includes('Warning: 1 reward was left out: earlier progress on this house already counted for it.'), result(one));
});

test('a bulk write sums the missions of every house and names each specialization line', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 2, houses: 2, contribution_raised: 2, reward_rows: 6, missions: 5, solari: 30000, house_scrip: 5000, specialization_xp: { Crafting: 3000, Gathering: 4500 }, rewards_already_given: 1 } });
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  click(ui, '[data-lb=cyes]'); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('5 missions written: +30,000 Solari, +5,000 House Scrip, +3,000 Crafting XP, +4,500 Gathering XP'), r);
  assert.ok(r.includes('Warning: 1 reward was left out'), r);
});

// what removing the highest reached tier of Thorvald (tier 3) would do to its reward, as the server's board read says it (landsraad_takeback.go)
const withBack = (outcome, why = '') => { const d = board(); const r = d.tasks.find((t) => t.id === 901).rewards[2]; r.takeback = outcome; r.takeback_why = why; r.reward_state = outcome === 'take' ? 'waiting' : outcome === 'collected' ? 'delivered' : 'no_row'; return d; };
const question = (ui) => { const c = ui.doc.querySelector('#lsb-confirm'); return c ? text(c) : null; };
// the sentences a removal says whole (LB_REMOVE_NOTE, app.js): what stays, that raising the tier again pays no mission twice, and the way back
const NOTE = 'Rewards already collected stay, and so do the Solari, House Scrip and XP the missions of the tier paid, and raising the tier again pays no mission a second time. To go back, restore the backup (Database tab, Save backups); the editor does not restore its own record with it, so a raise after that may pay less than before.';

test('a removal with no reward waiting for its tier writes at once, and says that no reward waited and what stays', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0 } });
  const t3 = tier(ui, 'Thorvald', 3); // the highest reached tier of Thorvald (4,700): clicking it removes it
  assert.ok(t3.title.endsWith('Remove tier 3: your contribution drops to 2,400. No reward of this tier waits at the house representative, so there is nothing to take back. ' + NOTE), t3.title);
  assert.ok(!t3.title.includes('takes back nothing') && !t3.getAttribute('aria-label').includes('takes back nothing'));
  click(ui, t3); await ui.settle();
  assert.equal(question(ui), null, 'nothing to say about a reward: no question');
  assert.equal(writes(ui).length, 1);
  assert.ok(result(ui).includes('(tier 3 removed)') && result(ui).includes('no reward waited for the removed tier, so there was nothing to take back') && result(ui).includes('Rewards already collected stay') && result(ui).includes('restore the backup'), result(ui));
  assert.deepEqual(toasts(ui), ['House Thorvald: tier removed; no reward waited for the removed tier, so there was nothing to take back.']);
});

test('a removal that takes back a waiting reward asks first, names the reward, and the answer says how many were taken back', async () => {
  const ui = await open(withBack('take'), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0, rewards_taken_back: 1 } });
  const t3 = tier(ui, 'Thorvald', 3);
  assert.ok(t3.title.endsWith('Remove tier 3: your contribution drops to 2,400. Takes back its reward, which still waits at the house representative: 50 Precision Range Finder. Raising the tier again writes that reward once more. ' + NOTE), t3.title);
  click(ui, t3); await ui.settle();
  assert.equal(writes(ui).length, 0, 'nothing is written before the answer');
  const q = question(ui);
  assert.ok(q.includes('Remove tier 3 on House Thorvald? Takes back its reward, which still waits at the house representative: 50 Precision Range Finder. Raising the tier again writes that reward once more. ' + NOTE + ' Written at once, with a backup first.'), q);
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.deepEqual(writes(ui).map((w) => w.body), [{ term_id: 7, houses: [{ task_id: 901, was_mine: 4700, was_done: false, was_win: null, mine_to: 2400, lower: true }] }]);
  assert.ok(result(ui).includes('(tier 3 removed)') && result(ui).includes('took back 1 waiting reward'), result(ui));
  assert.deepEqual(toasts(ui), ['House Thorvald: tier removed; took back 1 waiting reward.']);
});

test('a removal can be answered No: nothing is written', async () => {
  const ui = await open(withBack('take'), { '/api/landsraad/apply': OK });
  click(ui, tier(ui, 'Thorvald', 3)); await ui.settle();
  assert.ok(question(ui));
  click(ui, '[data-lb=cno]'); await ui.settle();
  assert.equal(writes(ui).length, 0);
  assert.equal(question(ui), null);
});

test('a removal names a reward that is already collected and stays, and one that stays because it cannot be tied to the tier', async () => {
  let ui = await open(withBack('collected'), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0, rewards_kept_collected: 1 } });
  click(ui, tier(ui, 'Thorvald', 3)); await ui.settle();
  assert.ok(question(ui).includes('is already collected and stays.'), question(ui));
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.ok(result(ui).includes('1 collected reward stays'), result(ui));
  closeAll();
  ui = await open(withBack('unsure', 'the same item is also the reward of another tier'), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0, rewards_kept_unsure: 1 } });
  click(ui, tier(ui, 'Thorvald', 3)); await ui.settle();
  assert.ok(question(ui).includes('stays: it cannot be tied to this tier with certainty, because the same item is also the reward of another tier.'), question(ui));
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.ok(result(ui).includes('1 reward left alone (not certain it belongs to the removed tier)'), result(ui));
});

test('the line under the highest reached tier box says what a click does to its reward, from the outcome the server gives for the tier', async () => {
  const want = { take: 'click removes it; takes back its waiting reward', collected: 'click removes it; its reward is collected and stays', earned: 'click removes it; its waiting reward stays', unsure: 'click removes it; its waiting reward stays', none: 'click removes it; no reward waits', '': 'click removes it; no reward waits' };
  for (const [outcome, line] of Object.entries(want)) {
    const ui = await open(outcome ? withBack(outcome, 'a reason') : board(), {});
    assert.equal(text(tier(ui, 'Thorvald', 3).querySelector('.gr')), line, 'outcome ' + JSON.stringify(outcome));
    closeAll();
  }
});

test('a removal names a waiting reward that stays because the character own missions cover its tier, and the answer counts it apart from the ones not certain', async () => {
  const ui = await open(withBack('earned', 'your own missions in the game cover this tier'), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0, rewards_kept_earned: 1 } });
  click(ui, tier(ui, 'Thorvald', 3)); await ui.settle();
  assert.ok(question(ui).includes('A reward (50 Precision Range Finder) waits for this tier and stays: your own missions in the game cover this tier.'), question(ui));
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.ok(result(ui).includes('1 waiting reward stays (your own missions cover the tier)') && !result(ui).includes('not certain'), result(ui));
});

test('before any click the board says what Fill, Fill faction goal and each tier grant, in text tied to the control, and that it writes at once and is not checked in the game', async () => {
  const ui = await open(withGrant({}, { 901: 4700 }), {});
  const al = by(ui, 'Alexin');
  assert.equal(text(al.querySelector('.qg')), '6 missions: +36,000 Solari', 'a visible line under the quick buttons');
  assert.equal(quick(ui, 'Alexin', 'qa-fill').getAttribute('aria-describedby'), 'qg-900');
  const cap = text(al.querySelector('#lsg-900'));
  assert.ok(cap.includes('Fill and Fill faction goal: Writes 6 missions: +36,000 Solari, +6,000 House Scrip, +9,000 Crafting XP, 5 rewards waiting at the house representative.'), cap);
  assert.ok(cap.includes('Next tier (tier 1): Writes 2 missions: +12,000 Solari, +2,000 House Scrip, +3,000 Crafting XP, 1 reward waiting at the house representative.'), cap);
  assert.ok(cap.includes('Written at once.') && cap.includes('not been checked in the game yet'), cap);
  assert.equal(al.querySelector('.act-fill').getAttribute('aria-describedby'), 'lsg-900');
  // each tier box that would grant shows it as text, and its accessible name carries it
  assert.equal(text(tier(ui, 'Alexin', 5).querySelector('.gr')), '6 missions: +36,000 Solari');
  assert.ok(tier(ui, 'Alexin', 5).getAttribute('aria-label').includes('Writes 6 missions: +36,000 Solari'));
  assert.equal(tier(ui, 'Alexin', 1).querySelector('.gr').textContent, '2 missions: +12,000 Solari');
  // the highest reached tier says what a click does; a tier the character's own events already cover grants no missions
  assert.equal(text(tier(ui, 'Thorvald', 3).querySelector('.gr')), 'click removes it; no reward waits');
  assert.equal(text(tier(ui, 'Thorvald', 4).querySelector('.gr')), 'no missions');
  // the standing line about writes is gone (#980): the Help tab says it, and the question before a write says what it writes
  assert.equal(ui.doc.querySelector('#lsb-legend'), null);
  assert.ok(ui.doc.querySelector('[data-lb=bulk][data-do=fill]').title.includes('The question shows the totals'));
  // the closed row's meter is a picture, not five small buttons
  assert.equal(ui.doc.querySelector('.meter button'), null);
  assert.equal(ui.doc.querySelector('.meter').getAttribute('role'), 'img');
});

test('the ground the character own events already cover is not counted again in what a click says it grants', async () => {
  // Thorvald is at 4,700 and its own events cover 6,000: a raise to 6,000 stands for no mission
  const ui = await open(withGrant({}, { 901: 6000 }), {});
  assert.equal(text(by(ui, 'Thorvald').querySelector('.qg')), 'no missions');
  assert.ok(text(by(ui, 'Thorvald').querySelector('#lsg-901')).includes('Writes no missions: your own earlier progress already counts for this ground'));
});

test('what the character reached under another faction is not counted again in what a click says it grants', async () => {
  // Thorvald: the character reached 6,000 under another faction (my_elsewhere), so a raise to 6,000 under this one stands for no mission
  const b = withGrant();
  b.tasks.forEach((t) => { if (t.id === 901) t.my_elsewhere = 6000; });
  const ui = await open(b, {});
  assert.equal(text(by(ui, 'Thorvald').querySelector('.qg')), 'no missions');
  assert.ok(text(by(ui, 'Thorvald').querySelector('#lsg-901')).includes('Writes no missions: your own earlier progress already counts for this ground'));
});

test('a setting the editor cannot use, or a house with no line, is said before the click as a refusal', async () => {
  const bad = 'The Landsraad amounts cannot be worked out: LandsraadContributionMultiplier in ServerCustomSettings.ini is "x", not a number from 0 to 10. Nothing was changed; correct the value in ServerCustomSettings.ini (Config tab) and try again.';
  const ui = await open(withGrant({ problem: bad }), {});
  assert.equal(text(by(ui, 'Alexin').querySelector('.qg')), 'will be refused');
  assert.ok(text(by(ui, 'Alexin').querySelector('#lsg-900')).includes('Will be refused: ' + bad));
  const b2 = withGrant(); b2.tasks[1].line = '';
  const ui2 = await open(b2, {});
  const cap = text(by(ui2, 'Alexin').querySelector('#lsg-900'));
  assert.ok(cap.includes('has no specialization line in the editor') && cap.includes('use Complete for this one'), cap);
});

test('a click on one house that would grant ten missions or more asks first, with the totals and a button named for the action; Cancel writes nothing and returns the focus', async () => {
  // a mission step of 500: Alexin 0 to 6,000 is 12 missions (72,000 Solari)
  const ui = await open(withGrant({ step: 500 }), { '/api/landsraad/apply': { ...GRANTED, missions: 12, solari: 72000, house_scrip: 12000, specialization_xp: { Crafting: 18000 } } });
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  assert.deepEqual(writes(ui), [], 'asking writes nothing');
  const q = ui.doc.querySelector('#lsb-confirm');
  assert.equal(text(q.querySelector('b')), 'Fill House Alexin? Writes 12 missions: +72,000 Solari, +12,000 House Scrip, +18,000 Crafting XP, 5 rewards waiting at the house representative. Written at once, with a backup first.');
  assert.deepEqual([...q.querySelectorAll('button')].map(text), ['Fill House Alexin', 'Cancel']);
  assert.equal(ui.doc.activeElement.dataset.fk, 'c-no');
  click(ui, '[data-lb=cno]'); await ui.settle();
  assert.deepEqual(writes(ui), []);
  assert.equal(ui.doc.activeElement.dataset.fk, 'qf:900', 'the focus goes back to the control that was pressed');
  // answering writes the request the page showed, and the question is gone
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.deepEqual(writes(ui).map((w) => w.body.houses), [[{ task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000, was_events: 0 }]]);
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  // a tier click that would grant as much asks the same way; Fill faction goal names the faction
  const ui2 = await open(withGrant({ step: 500 }), { '/api/landsraad/apply': GRANTED });
  click(ui2, tier(ui2, 'Alexin', 5)); await ui2.settle();
  assert.equal(text(ui2.doc.querySelector('#lsb-confirm b')).split('?')[0], 'Give tier 5 on House Alexin');
  click(ui2, '[data-lb=cno]'); await ui2.settle();
  click(ui2, by(ui2, 'Alexin').querySelector('.act-fill')); await ui2.settle();
  assert.equal(text(ui2.doc.querySelector('#lsb-confirm b')).split('?')[0], 'Fill faction goal on House Alexin for Atreides');
  assert.deepEqual(writes(ui2), []);
  // below the threshold a click writes at once, as before
  const ui3 = await open(withGrant({}), { '/api/landsraad/apply': GRANTED });
  click(ui3, quick(ui3, 'Alexin', 'qa-fill')); await ui3.settle();
  assert.equal(ui3.doc.querySelector('#lsb-confirm'), null);
  assert.equal(writes(ui3).length, 1);
});

test('the bulk question carries the totals of the raise and names its button by the action', async () => {
  const ui = await open(withGrant({}, { 901: 4700 }), { '/api/landsraad/apply': { ...OK, changed: 3, houses: 3, contribution_raised: 3 } });
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  const q = ui.doc.querySelector('#lsb-confirm');
  // Alexin 6 missions and 5 rewards, Thorvald 1 and 2, Hurata (1,300 to 6,000) 4 and 4; Wayku is above the top tier and is not asked about
  assert.equal(text(q.querySelector('b')), 'Fill 3 houses? Writes 11 missions: +66,000 Solari, +11,000 House Scrip, +1,500 Combat XP, +9,000 Crafting XP, +6,000 Exploration XP, 11 rewards waiting at the house representative. Written at once, with a backup first.');
  assert.deepEqual([...q.querySelectorAll('button')].map(text), ['Fill 3 houses', 'Cancel']);
  // a refusal the editor would give is in the question too
  const bad = withGrant({ problem: 'LandsraadContributionMultiplier is 0. Nothing was changed.' });
  const ui2 = await open(bad, {});
  click(ui2, '[data-lb=pickall]'); click(ui2, '[data-lb=bulk][data-do=fill]'); await ui2.settle();
  assert.ok(text(ui2.doc.querySelector('#lsb-confirm b')).includes('Will be refused: LandsraadContributionMultiplier is 0.'));
});

test('the About text says what a raise writes and what it does not, names both settings, that the mission limits are not applied, that the pay-out is not checked, and the way back', async () => {
  const ui = await open();
  const about = text(ui.doc.querySelector('[data-about="lsb.tiers"] .abt'));
  for (const s of ['A tier click pays what its missions pay', '6,000 Solari and 1,000 House Scrip in your wallet, and 1,500 XP on the specialization line of the house', 'LandsraadContributionMultiplier', 'LandsraadSpecializationXpMultiplier',
    'The daily and weekly mission limits of the game are not applied', 'No progress event is written', 'A second Fill with nothing above your contribution writes nothing', 'raising a removed tier again pays no mission a second time', 'your character\'s own XP and your faction standing', 'takes back the reward of the removed tier if the editor wrote it and it still waits at the house representative',
    'The Solari, House Scrip, XP and faction progress the missions of the tier paid are not taken back.', 'What the raise does write again is the reward a removal took back; a collected reward is not written again.',
    'clicking the highest reached tier twice removes it and gives it again, and the second click pays no mission']) assert.ok(about.includes(s), s);
  for (const s of ['pays once', 'pays nothing again', 'takes back nothing', 'is paid again by a raise']) assert.ok(!about.includes(s), 'the About text still says: ' + s);
  assert.ok(ui.doc.querySelectorAll('[data-about="lsb.tiers"] .abt p').length >= 4 && ui.doc.querySelector('[data-about="lsb.tiers"] .abt ul'), 'the long paragraph is split into a list and short paragraphs');
  assert.ok(text(ui.doc.querySelector('[data-about="lsb.fill"] .abt')).includes('asks first'));
  assert.ok(!about.includes('Not checked in the game yet') && !about.includes('Restore on the Database tab'), 'the standing "not checked" and "going back" paragraph is in the Help tab (#980)');
});

test('the bulk question says it will be refused when the houses together stand for more missions than one change may write', async () => {
  // a step of 10: Alexin alone is 600 missions, Hurata 470, together over the 1,000 of one change
  const ui = await open(withGrant({ step: 10 }), {});
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  const q = text(ui.doc.querySelector('#lsb-confirm b'));
  assert.ok(q.includes('Will be refused: This request stands for') && q.includes('more than the 1,000 one change may grant') && q.includes('Fill fewer houses at once'), q);
  assert.deepEqual(writes(ui), []);
});

test('with another faction chosen the tier boxes are off with the reason and write nothing; with your own faction they work again (#667)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  select(ui, '#lsb-fac', 2); await ui.settle();
  for (const i of [1, 3, 5]) {
    const b = tier(ui, 'Thorvald', i);
    assert.equal(b.getAttribute('aria-disabled'), 'true', 'tier ' + i);
    assert.ok(b.title.startsWith('Off while Harkonnen is chosen') && b.title.includes('your own contribution') && b.title.includes('Atreides'), b.title);
    assert.ok(!b.title.includes('Reward') && !b.title.includes('Game id'), 'the title is the reason only');
    assert.ok(b.getAttribute('aria-label').endsWith(', off') && !b.getAttribute('aria-label').includes('Off while'), 'the name is short, the reason is in the caption');
    assert.equal(b.getAttribute('aria-describedby'), 'lsg-901');
    assert.equal(b.querySelector('.gr'), null, 'no promise of a grant on a box that does nothing');
  }
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  click(ui, tier(ui, 'Thorvald', 2)); await ui.settle(); // the highest reached tier is the one that removes
  assert.deepEqual(writes(ui), [], 'no tier click wrote anything');
  assert.ok(result(ui).startsWith('Off while Harkonnen is chosen'), result(ui));
  assert.ok(text(ui.doc.querySelector('#lsg-901')).includes('Fill and the tiers are off while Harkonnen is chosen'), text(ui.doc.querySelector('#lsg-901')));
  // the guard alone: even if a click got through, the request builder sends nothing for a tier of another faction
  const data = board(), h = ui.w.lbHouse(data.tasks.find((x) => x.id === 901), data);
  assert.equal(ui.w.lbItem(h, data, 'tier', 2, 5), null);
  assert.notEqual(ui.w.lbItem(h, data, 'tier', 1, 5), null, 'own faction: the same click is a request');
  select(ui, '#lsb-fac', 1); await ui.settle();
  assert.equal(tier(ui, 'Thorvald', 5).getAttribute('aria-disabled'), 'false');
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  assert.equal(writes(ui).length, 1);
  assert.equal(writes(ui)[0].body.houses[0].mine_to, 6000);
});

test('the result line says which win a completion replaced and how many other completion times were moved (#611)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0, replaced: [{ house: 'Hurata', previous_winner: 'Harkonnen' }], repaired: 2 } });
  click(ui, quick(ui, 'Thorvald', 'qa-cr')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('; replaces the win of Harkonnen on Hurata'), r);
  assert.ok(r.includes("; 2 other completion times put back inside the term's window"), r);
});

test('a write that changed no house but moved other completion times does not say there was nothing to do (#611)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 0, noop: true, contribution_raised: 0, reward_rows: 0, replaced: [], repaired: 1 } });
  click(ui, quick(ui, 'Thorvald', 'qa-cr')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('already in that state'), r);
  assert.ok(r.includes("; 1 other completion time put back inside the term's window"), r);
  assert.ok(!r.includes('nothing to change'), 'a write that moved times does not say there was nothing to change: ' + r); // #775
  // with no time moved it still says so
  const none = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 0, noop: true, contribution_raised: 0, reward_rows: 0, replaced: [], repaired: 0 } });
  click(none, quick(none, 'Thorvald', 'qa-cr')); await none.settle();
  assert.ok(result(none).includes('nothing to change, the house was already in that state'), result(none));
});

// the notes of a write (a win taken away, other times moved) come before the warning, not inside its sentence (#775)
test('the notes of a write stand before the warning, not inside it', async () => {
  const one = await open(board(), { '/api/landsraad/apply': { ...OK, reward_rows: 0, missions: 0, rewards_already_given: 1, replaced: [{ house: 'Alexin', previous_winner: 'Harkonnen' }], repaired: 2 } });
  click(one, quick(one, 'Alexin', 'qa-fill')); await one.settle();
  const r = result(one), warn = r.indexOf('Warning: 1 reward was left out'), note = r.indexOf('; replaces the win of Harkonnen on Alexin');
  assert.ok(note > -1 && warn > -1 && note < warn, r);
  assert.ok(r.indexOf("; 2 other completion times put back inside the term's window") < warn, r);
  assert.ok(/Nothing to do[.;]?s*$|Nothing to do.?/.test(r.slice(warn)) && !r.slice(warn).includes('replaces'), r.slice(warn));
});

// The ground the editor paid before (#777): the board read carries it (my_paid), the line before a click names the missions that would not be paid, and the result says so.
test('ground the editor paid before is not counted in what a click grants, and the line before the click says so', async () => {
  // Thorvald is at 4,700 and the editor paid 6,000 before (a tier removal took the contribution down): a raise to 6,000 stands for no mission, and one of them is named as not paid
  const b = withGrant();
  b.tasks.forEach((t) => { if (t.id === 901) { t.my_paid = 6000; t.my_paid_rows = 6000; } });
  const ui = await open(b, {});
  assert.equal(text(by(ui, 'Thorvald').querySelector('.qg')), 'no missions');
  const cap = text(by(ui, 'Thorvald').querySelector('#lsg-901'));
  assert.ok(cap.includes('Fill and Fill faction goal: Writes no missions. 1 mission not paid: the editor recorded that ground as paid in an earlier raise, and removing a tier does not take back what its missions paid.'), cap);
  // the rewards a removal took back are not on the board any more (my_paid_rows is below the paid ground): the raise writes them again and still pays no mission
  const bb = withGrant();
  bb.tasks.forEach((t) => { if (t.id === 901) { t.my_paid = 6000; t.my_paid_rows = 4700; } });
  const ub = await open(bb, {});
  const capb = text(by(ub, 'Thorvald').querySelector('#lsg-901'));
  assert.ok(capb.includes('Fill and Fill faction goal: Writes no missions; 2 rewards waiting at the house representative. 1 mission not paid: the editor recorded that ground as paid in an earlier raise, and removing a tier does not take back what its missions paid.'), capb);
  // paid ground below the target: only the part above it is written, and the part below is named
  const b2 = withGrant();
  b2.tasks.forEach((t) => { if (t.id === 900) { t.my_paid = 2400; t.my_paid_rows = 2400; } });
  const ui2 = await open(b2, {});
  const al = text(by(ui2, 'Alexin').querySelector('#lsg-900'));
  assert.ok(al.includes('Fill and Fill faction goal: Writes 3 missions: +18,000 Solari') && al.includes('3 missions not paid: the editor recorded that ground as paid in an earlier raise, and removing a tier does not take back what its missions paid.'), al);
  assert.equal(text(by(ui2, 'Alexin').querySelector('.qg')), '3 missions: +18,000 Solari');
});

test('the result of a write says how many missions were not paid because that ground was paid before', async () => {
  const ui = await open(withGrant(), { '/api/landsraad/apply': { ...OK, reward_rows: 0, missions: 0, solari: 0, house_scrip: 0, specialization_xp: {}, rewards_already_given: 2, missions_paid_before: 3, ground_paid_before: 3000 } });
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('No missions written. 3 missions not paid: the editor recorded that ground as paid in an earlier raise, and removing a tier does not take back what its missions paid.'), r);
  const part = await open(withGrant(), { '/api/landsraad/apply': { ...GRANTED, missions: 1, solari: 6000, house_scrip: 1000, missions_paid_before: 2, ground_paid_before: 2000 } });
  click(part, quick(part, 'Alexin', 'qa-fill')); await part.settle();
  assert.ok(result(part).includes('1 mission written: +6,000 Solari, +1,000 House Scrip') && result(part).includes('2 missions not paid: the editor recorded that ground as paid in an earlier raise'), result(part));
  // an answer that paid everything has no such sentence
  const full = await open(withGrant(), { '/api/landsraad/apply': GRANTED });
  click(full, quick(full, 'Alexin', 'qa-fill')); await full.settle();
  assert.ok(!result(full).includes('not paid'), result(full));
});

test('a swatch won in this term is not counted as a reward waiting in the line before the click', async () => {
  // the top tier of Alexin is the house swatch (swatch: true); with the swatch held in this term the write skips it, so 4 rewards wait, not 5
  const held = withGrant(); held.tasks.forEach((t) => { if (t.id === 900) t.swatch_held = true; });
  const ui = await open(held, {});
  const cap = text(by(ui, 'Alexin').querySelector('#lsg-900'));
  assert.ok(cap.includes('Writes 6 missions: +36,000 Solari, +6,000 House Scrip, +9,000 Crafting XP, 4 rewards waiting at the house representative.'), cap);
  const free = await open(withGrant(), {});
  assert.ok(text(by(free, 'Alexin').querySelector('#lsg-900')).includes(', 5 rewards waiting at the house representative.'), 'a swatch not held yet is delivered: 5 rewards');
});

// ---- leftovers of the review of #652 (#752, #762): the experience that did not fit, the preview near the last level, the ground the page showed

test('a track at its last level is said in the result line and the toast: the experience that did not fit is named by line (#752)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...GRANTED, specialization_xp: {}, specialization_xp_not_added: { Exploration: 9000 } } });
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('3 missions written: +18,000 Solari, +3,000 House Scrip; Exploration is at its last level, +9,000 XP not added'), r);
  assert.deepEqual(toasts(ui), ['House Alexin: 3 missions written. Some XP not added: a specialization is at its last level.']);
  const q = await open(board(), { '/api/landsraad/apply': { ...GRANTED, outcome: 'queued', specialization_xp: { Exploration: 1500 }, specialization_xp_not_added: { Exploration: 7500 } } });
  click(q, quick(q, 'Alexin', 'qa-fill')); await q.settle();
  assert.ok(result(q).includes('+1,500 Exploration XP; Exploration is at its last level, +7,500 XP will not be added'), result(q));
  // an answer with nothing left over (and an older answer without the field) says nothing about it
  const none = await open(board(), { '/api/landsraad/apply': { ...GRANTED, specialization_xp_not_added: {} } });
  click(none, quick(none, 'Alexin', 'qa-fill')); await none.settle();
  assert.ok(!result(none).includes('last level'), result(none));
});

test('the line before a click does not promise more experience than the track has room for, one house or several of one line (#762)', async () => {
  const ui = await open(withGrant({ room: { Crafting: 2000 } }, { 901: 4700 }), {});
  const cap = text(by(ui, 'Alexin').querySelector('#lsg-900'));
  assert.ok(cap.includes('Fill and Fill faction goal: Writes 6 missions: +36,000 Solari, +6,000 House Scrip, +2,000 Crafting XP'), cap);
  assert.ok(!cap.includes('+9,000'), cap);
  // two houses of one line share the track: together they add no more than its room
  const b = withGrant({ room: { Crafting: 5000 } });
  b.tasks.forEach((t) => { if (t.id === 901) t.line = 'Crafting'; });
  const two = await open(b, {});
  click(two, '[data-lb=pickall]');
  click(two, '[data-lb=bulk][data-do=fill]'); await two.settle();
  const q = text(two.doc.querySelector('#lsb-confirm b'));
  assert.ok(q.includes('+5,000 Crafting XP') && !q.includes('10,500'), q);
});

test('a raise sends the ground the character own events covered when the page drew the house, so the server can refuse it when that changed (#762)', async () => {
  const ui = await open(withGrant({}, { 900: 1200 }), { '/api/landsraad/apply': OK });
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  assert.equal(writes(ui)[0].body.houses[0].was_events, 1200);
  // a removal sends the number too (the page sends it for every contribution it writes); the server compares it for a raise only, so it never refuses a removal for it
  const rm = await open(withGrant({}, { 901: 0 }), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1 } });
  click(rm, tier(rm, 'Thorvald', 3)); await rm.settle();
  assert.equal(writes(rm)[0].body.houses[0].lower, true);
  assert.equal(writes(rm)[0].body.houses[0].was_events, 0);
});

// #805: a raise also sends the other two grounds its sentence was worked out from, so the server can refuse it when either changed
test('a raise sends the ground the editor paid before and the ground reached under another faction as the page drew them; a removal does not, and a board without them sends nothing extra (#805)', async () => {
  const b = withGrant({}, { 900: 1200 });
  b.tasks.forEach((t) => { if (t.id === 900) { t.my_paid = 2400; t.my_paid_rows = 1200; t.my_elsewhere = 3600; } });
  const ui = await open(b, { '/api/landsraad/apply': OK });
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  const h = writes(ui)[0].body.houses[0];
  assert.equal(h.was_paid, 2400);
  assert.equal(h.was_paid_rows, 1200);
  assert.equal(h.was_elsewhere, 3600);
  assert.equal(h.was_events, 1200);
  // a tier click that raises carries them too
  const t5 = await open(b, { '/api/landsraad/apply': OK });
  click(t5, tier(t5, 'Alexin', 5)); await t5.settle();
  const h5 = writes(t5)[0].body.houses[0];
  assert.deepEqual([h5.was_paid, h5.was_paid_rows, h5.was_elsewhere], [2400, 1200, 3600]);
  // a removal does not carry them: the server never compares them for one
  const b2 = withGrant({}, { 901: 0 });
  b2.tasks.forEach((t) => { if (t.id === 901) { t.my_paid = 6000; t.my_paid_rows = 6000; t.my_elsewhere = 100; } });
  const rm = await open(b2, { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1 } });
  click(rm, tier(rm, 'Thorvald', 3)); await rm.settle();
  const hr = writes(rm)[0].body.houses[0];
  assert.equal(hr.lower, true);
  assert.ok(!('was_paid' in hr) && !('was_paid_rows' in hr) && !('was_elsewhere' in hr), JSON.stringify(hr));
  // a board that does not carry the numbers (an older server) sends none
  const old = await open(withGrant({}, { 900: 0 }), { '/api/landsraad/apply': OK });
  click(old, quick(old, 'Alexin', 'qa-fill')); await old.settle();
  const ho = writes(old)[0].body.houses[0];
  assert.ok(!('was_paid' in ho) && !('was_paid_rows' in ho) && !('was_elsewhere' in ho), JSON.stringify(ho));
});

test('a house with no specialization line is refused before the click only when the raise pays a mission, as the write refuses it (#762)', async () => {
  // a mission step of 100,000 and 100 already covered: Alexin 0 to 6,000 crosses no mission step, so no experience is written
  const b = withGrant({ step: 100000 }, { 900: 100 });
  b.tasks.forEach((t) => { if (t.id === 900) t.line = ''; });
  const ui = await open(b, {});
  const cap = text(by(ui, 'Alexin').querySelector('#lsg-900'));
  assert.ok(!cap.includes('has no specialization line') && !cap.includes('Will be refused'), cap);
  assert.equal(text(by(ui, 'Alexin').querySelector('.qg')), 'no missions');
});

// #818: a bulk question names the houses chosen when it was asked. A filter or a jump unticks the rows it hides, so the question must not write them.
const show = (ui, k) => click(ui, `[data-lb=show][data-show=${k}]`);
const jump = (ui, id) => { const j = ui.doc.querySelector('#lsb-jump'); j.value = String(id); j.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const sentIds = (ui) => writes(ui).flatMap((w) => w.body.houses.map((h) => h.task_id)).sort();

test('a filter that unticks houses of an open bulk question closes it, and nothing is written for a house that is not chosen (#818)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 3, houses: 3 } });
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Fill 3 houses? Written at once, with a backup first.');
  show(ui, 'part'); await ui.settle(); // Alexin (not started) and Wayku (top tier) are hidden and lose their ticks
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null, 'the question about three houses is gone');
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).startsWith('Chosen: 2 houses.'), text(ui.doc.querySelector('#lsb-sum')));
  assert.ok(result(ui).includes('question was closed') && result(ui).includes('Nothing was written'), result(ui));
  assert.deepEqual(writes(ui), []);
  // the answer button is gone; asking again names the houses that are chosen now
  assert.equal(ui.doc.querySelector('[data-lb=cyes]'), null);
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Fill 2 houses? Written at once, with a backup first.');
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.deepEqual(sentIds(ui), [901, 902], 'Thorvald and Hurata only: Alexin is hidden and not chosen');
});

test('a jump to one house closes an open bulk question, and the House: filter does the same (#818, #830)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Complete 3 houses for Atreides? Written at once, with a backup first.');
  jump(ui, 901); await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.deepEqual(writes(ui), []);
  // the choice is only Thorvald now; one house is written at once, with no question and no other house
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle();
  assert.deepEqual(sentIds(ui), [901]);
  // the House: Open / Taken filter hides rows by whether a faction holds the house
  const ui2 = await open(board(), { '/api/landsraad/apply': OK });
  click(ui2, '[data-lb=pickall]');
  click(ui2, '[data-lb=bulk][data-do=complete]'); await ui2.settle();
  const taken = ui2.doc.querySelector('.lsb [data-lb=hs][value=taken]'); taken.checked = true; taken.dispatchEvent(new ui2.w.Event('change', { bubbles: true })); await ui2.settle();
  assert.equal(ui2.doc.querySelector('#lsb-confirm'), null);
  assert.deepEqual(writes(ui2), []);
});

test('a filter that hides only houses the open bulk question does not name leaves the question open, and Yes writes the houses it named (#818)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 3, houses: 3 } });
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle(); // Wayku is complete already: it is chosen but not in the question
  assert.ok(ui.doc.querySelector('#lsb-confirm'));
  show(ui, 'left'); await ui.settle(); // hides Wayku only
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).startsWith('Chosen: 3 houses.'), text(ui.doc.querySelector('#lsb-sum')));
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Complete 3 houses for Atreides? Written at once, with a backup first.');
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.deepEqual(sentIds(ui), [900, 901, 902]);
});

test('Yes never writes a house that is not chosen: with the closing of the question out of the way, the answer asks again with the smaller number (#818)', async () => {
  // the second guard on its own: the page's closing of the question is switched off, so the filter leaves the stale question open and only the check in the answer stands
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 2, houses: 2 } });
  ui.w.lbCancelConfirm = () => {};
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=complete]'); await ui.settle();
  show(ui, 'part'); await ui.settle(); // Alexin is hidden and no longer chosen
  assert.ok(ui.doc.querySelector('#lsb-confirm'), 'the stale question is still shown');
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.deepEqual(writes(ui), [], 'the first Yes only asks again');
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Complete 2 houses for Atreides? Written at once, with a backup first.');
  assert.ok(result(ui).includes('The houses changed since the question was asked'), result(ui));
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.deepEqual(sentIds(ui), [901, 902], 'Alexin was no longer chosen');
  // none of the houses of the question is chosen any more: nothing is written
  const ui2 = await open(board(), { '/api/landsraad/apply': OK });
  ui2.w.lbCancelConfirm = () => {};
  click(ui2, '[data-lb=pickall]');
  click(ui2, '[data-lb=bulk][data-do=complete]'); await ui2.settle();
  show(ui2, 'done'); await ui2.settle(); // only Wayku stays, and Wayku is not in the question
  click(ui2, '[data-lb=cyes]'); await ui2.settle();
  assert.deepEqual(writes(ui2), []);
  assert.equal(ui2.doc.querySelector('#lsb-confirm'), null);
  assert.ok(result(ui2).includes('Nothing to change any more'), result(ui2));
});

test('a re-read that makes a named house drop out of the filter closes the question through the page redraw: nothing is written, the focus goes back to the bulk button (#831, #832)', async () => {
  const data = board();
  const ui = await open(data, { '/api/landsraad/apply': { ...OK, changed: 2, houses: 2 } });
  show(ui, 'part'); await ui.settle(); // In progress: Thorvald and Hurata
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Fill 2 houses? Written at once, with a backup first.');
  assert.equal(ui.doc.activeElement.getAttribute('data-fk'), 'c-no', 'the safe answer has the focus');
  // the save changes meanwhile: Thorvald is at the top tier now, so the In progress filter hides it on the next read
  data.tasks = data.tasks.map((t) => (t.id === 901 ? house(901, 'Thorvald', 6200, { contributions: t.contributions }) : t));
  const before = boardFetches(ui);
  await ui.w.render(true); await ui.settle();
  assert.ok(boardFetches(ui) > before, 'the board was read again');
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null, 'the question about a house that is hidden now is closed');
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).startsWith('Chosen: 1 house.'), text(ui.doc.querySelector('#lsb-sum')));
  assert.ok(result(ui).includes('question was closed') && result(ui).includes('Nothing was written'), result(ui));
  assert.deepEqual(writes(ui), [], 'nothing was posted');
  assert.equal(ui.doc.activeElement.getAttribute('data-fk'), 'b-fill', 'the focus is on the bulk button that asked, not on the body: ' + ui.doc.activeElement.tagName);
});

test('a question about one clicked house is not a bulk question: a filter that hides the row does not close it or change what Yes writes (#818)', async () => {
  const ui = await open(withGrant({ step: 500 }), { '/api/landsraad/apply': GRANTED });
  by(ui, 'Alexin').querySelector('.pick').click(); // Alexin is also ticked in the choice, so the filter will untick it
  click(ui, quick(ui, 'Alexin', 'qa-fill')); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-confirm b')).startsWith('Fill House Alexin?'));
  show(ui, 'part'); await ui.settle(); // Alexin is hidden and unticked
  assert.ok(ui.doc.querySelector('#lsb-confirm'), 'the question names the house itself and stays');
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.deepEqual(sentIds(ui), [900]);
});

test('no click asks for a change of your own contribution together with a completion for another faction, so one house never refuses the whole request (#657)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  const data = board();
  for (const t of data.tasks) {
    const h = ui.w.lbHouse(t, data);
    for (const F of [1, 2]) {
      for (const kind of ['fill', 'complete', 'reopen', 'fillgoal', 'tier']) {
        for (const i of kind === 'tier' ? [1, 2, 3, 4, 5] : [undefined]) {
          const it = ui.w.lbItem(h, data, kind, F, i);
          if (it && it.mine_to !== undefined && it.complete_for !== undefined) {
            assert.equal(it.complete_for, data.character.faction_id, `${t.house_name} ${kind} ${i} for faction ${F}: the contribution and the completion are for one faction`);
          }
        }
      }
    }
  }
});

// #881: in the classic list too, a House whose row is hidden is not written by a bulk button, even when the choice still holds it (a stale choice, or a row hidden without the page being told).
test('a bulk button drops a House whose row is hidden from the choice: it is not in the question or in the request (#881)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  click(ui, '[data-lb=pickall]');
  show(ui, 'left'); await ui.settle(); // Wayku (top tier, completed) is hidden and unticked
  ui.w.LCC_HOST.picks().add(903); // a stale choice that holds the hidden House again
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  const q = text(ui.doc.querySelector('#lsb-confirm b'));
  assert.ok(!q.includes('Wayku'), q);
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.ok(!sentIds(ui).includes(903), 'Wayku is not written: ' + sentIds(ui));
  assert.ok(sentIds(ui).length > 0);
});

test('Yes does not write a House whose row was hidden after the question was asked (#881)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  click(ui, '[data-lb=pickall]');
  click(ui, '[data-lb=bulk][data-do=fill]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#lsb-confirm'));
  ui.doc.querySelector('#lsrow-900').hidden = true; // hidden without a filter change: the page's choice list is not told
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.ok(!sentIds(ui).includes(900), 'Alexin is hidden, so it is not written: ' + sentIds(ui));
  // #1019: the test must not pass just because nothing was sent: the question is asked again, with the count as it is now, and nothing is written until it is answered
  assert.equal(sentIds(ui).length, 0, 'nothing is written by the first Yes: ' + sentIds(ui));
  assert.ok(result(ui).includes('The houses changed since the question was asked'), result(ui));
  assert.ok(ui.doc.querySelector('#lsb-confirm'), 'the question is open again');
  assert.ok(!text(ui.doc.querySelector('#lsb-confirm b')).includes('Alexin'), text(ui.doc.querySelector('#lsb-confirm b')));
  click(ui, '[data-lb=cyes]'); await ui.settle();
  assert.ok(sentIds(ui).length > 0 && !sentIds(ui).includes(900), 'the second Yes writes the others only: ' + sentIds(ui));
});
