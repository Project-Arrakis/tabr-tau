// The board writes at once (#641): every click on a tier, Fill, Complete, Reopen, Fill faction goal and every bulk button sends its change as one edit, shows what it wrote, and reads
// the board again from the save. There is no pending state, no Undo and no Write button. A bulk button for more than one house asks first.
// Ported to the Command Center board when the Classic list was retired (#1028): the House is looked at (its name button), and the Inspector holds the tier boxes and the actions; the List layout rows hold
// Fill and Complete or Reopen. The results are the toasts of the page (the board keeps no result line, #943): one line per fact. The requests, the stale guard, the refusals and the questions are the
// same ones the Classic list sent and asked, which lcc-board-requests.test.mjs proves against the recorded file; this file keeps the finer cases: wording, the lock, the grant sentences.
// The board is a fixed five by five board: the four Houses of the cases are on it with 21 Houses of no contribution beside them, which no case chooses or writes.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { questionText } from './lcc-fixture.mjs';

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
const board = (extra = {}) => {
  const b = {
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
  };
  // the board is a fixed five by five board: 21 Houses with no contribution fill the other squares (no case chooses or writes them)
  if (b.tasks.length === 4) for (let i = 4; i < 25; i++) b.tasks.push(house(900 + i, 'Filler' + String.fromCharCode(65 + i), 0));
  return b;
};
const ID = { Wayku: 903, Alexin: 900, Thorvald: 901, Hurata: 902 };
const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 2, outcome: 'written' };
async function open(data = board(), post = {}, layout = 'grid') {
  const ui = await boot({ get: { '/api/landsraad': data }, post, local: { tab: 'landsraad', 'lsb.layout': layout } });
  await ui.settle();
  return ui;
}
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
// #1120: what the term panel says is a toast of the page (never a line that stays): the text of the Landsraad toast on screen
const toastOf = (ui) => [...ui.doc.querySelectorAll('#toast [data-one="lbd"]')].at(-1) || null;
const said = (ui) => { const t = toastOf(ui); return t ? t.querySelector('span').textContent.replace(/\s+/g, ' ').trim() : ''; };
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const writes = (ui) => ui.posted.filter((p) => p.path === '/api/landsraad/apply');
const boardFetches = (ui) => ui.fetched.filter((p) => p === '/api/landsraad').length;
// What the page said last: the toast of the board, one line per fact (the board keeps no result line, #943). lines: its lines; result: the same said as one text.
const toastEls = (ui) => [...ui.doc.querySelectorAll('#toast .d-toast')];
const lastToast = (ui) => toastEls(ui).pop() || null;
const lines = (ui) => { const t = lastToast(ui); return t ? t.querySelector('span:not([class])').textContent.split('\n').map((x) => x.trim()).filter(Boolean) : []; };
const result = (ui) => lines(ui).join(' ');
// the House is looked at (its name button), then the Inspector holds its tier boxes and its actions
const look = (ui, name) => { const i = ui.doc.querySelector('.lcc-bd-insp'); if (!i || i.dataset.id !== String(ID[name])) click(ui, `[data-fk="ht:${ID[name]}"]`); };
const tier = (ui, name, n) => { look(ui, name); return ui.doc.querySelector(`.lcc-bd-insp .lcc-bd-tier[data-t="${n}"]`); };
const act = (ui, name, which) => { look(ui, name); return ui.doc.querySelector(`.lcc-bd-insp .lcc-bd-act[data-do="${which}"]`); };
// a tier box of a reward with a picture mouse-over keeps its sentence in that mouse-over, not in a native title (#977)
const ttl = (ui, b) => b.getAttribute('title') || ui.w.LCCB.tipOf(b).title;
const grant = (b) => { const g = b.querySelector('.lcc-bd-tier-gr'); return g ? text(g) : null; };
// the rows of the List layout hold Fill and Complete or Reopen (rf:ID, rc:ID)
const row = (ui, name, which) => ui.doc.querySelector(`.lcc-bd-list tr.lcc-bd-row[data-id="${ID[name]}"] [data-fk="${which}:${ID[name]}"]`);
const pick = (ui, id, on = true) => { const c = ui.doc.querySelector('.lcc-bd-pick[data-id="' + id + '"]'); c.checked = on; c.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const pickFour = (ui) => { for (const n of Object.keys(ID)) pick(ui, ID[n]); }; // the four Houses of the cases (All in the Filter menu would choose the 21 others too)
const bulk = (ui, which) => click(ui, `[data-fk="b-${which}"]`);
const chooseShown = (ui) => { click(ui, '[data-lk="fbtn"]'); click(ui, '#lcc-bd-menu [data-lk="hall"]'); }; // All in the Filter menu (#1116: the Choose shown key is gone)
// the House filter (Open / Pledged) and the specialization filter hide Houses, and drop them from the choice; the Show tabs are off while Houses are chosen, so they cannot do it
// (Show tabs work only with nothing chosen)
const show = (ui, k) => click(ui, `[data-fk="bf:show:${k}"]`);
const houseFilter = (ui, k) => click(ui, `[data-fk="bf:hs:${k}"]`);
const specFilter = (ui, k) => click(ui, `[data-fk="sp:${k}"]`);
const sentIds = (ui) => writes(ui).flatMap((w) => w.body.houses.map((h) => h.task_id)).sort();
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
  assert.ok(lines(ui)[0].startsWith('House Thorvald: your contribution 4,700 → 6,000 (tiers 4 to 5 reached); 2 rewards now waiting at the house representative'), lines(ui)[0]);
  assert.equal(ui.doc.querySelector('#lsb-result'), null, 'the board keeps no result line: the toast says what was written');
  // nothing of the staging is left on the page
  for (const sel of ['.pendtag', '.undo', '.undoq', '[data-lb=write]', '[data-lb=undo]', '[data-lb=undoall]', '#lsb-pendbar', '.pend', '[data-pend]', '.src-write', '.tc.add', '.tc.rem']) assert.equal(ui.doc.querySelector(sel), null, sel + ' is gone');
  const shown = ui.doc.querySelector('#lsb').cloneNode(true); shown.querySelectorAll('details.about').forEach((d) => d.remove()); // the About text may say there is none
  assert.ok(!/Pending|Undo|Write [0-9]|Write button|WRITES/.test(text(shown)), 'no word of the staging on the board');
  assert.equal(ui.doc.activeElement.dataset.fk, 'tc:901:5', 'the focus stays on the control that was pressed');
});

test('the highest reached tier clicked again is a removal sent with the flag that allows lowering; a lower reached tier writes nothing and says how to remove', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0 } });
  click(ui, tier(ui, 'Wayku', 2)); await ui.settle();
  assert.deepEqual(writes(ui), []);
  // the board removes tiers with Set tier and Apply tier, one at a time, the highest first (the Classic list said: click the highest reached tier)
  assert.ok(result(ui).includes('To remove tiers of House Wayku, choose tier 4 in Set tier and press Apply tier'), result(ui));
  click(ui, tier(ui, 'Wayku', 5)); await ui.settle(); // 6,500 is above tier 5: removing tier 5 drops it to tier 4 (4,800)
  assert.deepEqual(writes(ui).map((w) => w.body.houses), [[{ task_id: 903, was_mine: 6500, was_done: true, was_win: 1, mine_to: 4800, lower: true }]]);
  assert.ok(result(ui).includes('6,500 → 4,800 (tier 5 removed)'), result(ui));
});

test('Fill is off where nothing would change (nothing is sent, the reason is said); Fill writes the top tier and never lowers', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  const off = act(ui, 'Wayku', 'fill'); // 6,500 is above the top tier
  assert.equal(off.getAttribute('aria-disabled'), 'true');
  click(ui, off); await ui.settle();
  assert.deepEqual(writes(ui), []);
  assert.ok(result(ui).includes('already at the top tier'), result(ui));
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
  assert.deepEqual(writes(ui).map((w) => w.body.houses), [[{ task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000 }]]);
});

test('Complete, Reopen and Fill faction goal write at once for the faction chosen, which starts on the character own faction', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0 } });
  assert.equal(ui.doc.querySelector('#lsb-fac').value, '1');
  click(ui, act(ui, 'Thorvald', 'complete')); await ui.settle();
  assert.deepEqual(writes(ui)[0].body.houses, [{ task_id: 901, was_mine: 4700, was_done: false, was_win: null, complete_for: 1 }]);
  assert.ok(result(ui).includes('House Thorvald: Atreides progress 3,000 → 4,000; house taken by Atreides'), result(ui));
  // a taken house can be reopened, and it writes the reopening with the state the page showed
  click(ui, act(ui, 'Hurata', 'reopen')); await ui.settle();
  assert.deepEqual(writes(ui)[1].body.houses, [{ task_id: 902, was_mine: 1300, was_done: true, was_win: 2, reopen: true }]);
  assert.ok(result(ui).includes('house reopened (progress stays as it is)'), result(ui));
  // Fill faction goal: the top tier and the win in one request
  click(ui, act(ui, 'Alexin', 'fillgoal')); await ui.settle();
  assert.deepEqual(writes(ui)[2].body.houses, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000, complete_for: 1 }]);
  // another faction: the faction box only chooses the faction of the next click, it writes nothing itself
  select(ui, '#lsb-fac', 2); await ui.settle();
  assert.equal(writes(ui).length, 3);
  click(ui, act(ui, 'Alexin', 'complete')); await ui.settle();
  assert.deepEqual(writes(ui)[3].body.houses, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, complete_for: 2 }]);
});

test('the List rows hold Fill and Complete or Reopen: the same requests as the Inspector, and a taken house shows Reopen', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0 } }, 'list');
  const off = row(ui, 'Wayku', 'rf'); // 6,500 is above the top tier
  assert.equal(off.getAttribute('aria-disabled'), 'true');
  click(ui, off); await ui.settle();
  assert.deepEqual(writes(ui), []);
  assert.ok(result(ui).includes('already at the top tier'), result(ui));
  assert.equal(row(ui, 'Thorvald', 'rc').getAttribute('data-do'), 'complete');
  assert.equal(row(ui, 'Hurata', 'rc').getAttribute('data-do'), 'reopen', 'a taken house shows Reopen');
  click(ui, row(ui, 'Thorvald', 'rc')); await ui.settle();
  assert.deepEqual(writes(ui)[0].body.houses, [{ task_id: 901, was_mine: 4700, was_done: false, was_win: null, complete_for: 1 }]);
  click(ui, row(ui, 'Hurata', 'rc')); await ui.settle();
  assert.deepEqual(writes(ui)[1].body.houses, [{ task_id: 902, was_mine: 1300, was_done: true, was_win: 2, reopen: true }]);
  assert.ok(result(ui).includes('house reopened (progress stays as it is)'), result(ui));
  click(ui, row(ui, 'Alexin', 'rf')); await ui.settle();
  assert.deepEqual(writes(ui)[2].body.houses, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000 }]);
});

test('the tooltip of Reopen and the Reopen chosen key say it does not set back what Complete wrote', async () => {
  const ui = await open();
  const tip = act(ui, 'Hurata', 'reopen').title;
  assert.ok(/progress stays at the goal/.test(tip) && /not given back/.test(tip), tip);
  assert.ok(text(ui.doc.querySelector('[data-fk="b-reopen"]')).includes('does not set back what Complete wrote'));
  const lst = await open(board(), {}, 'list');
  assert.ok(/progress stays at the goal/.test(row(lst, 'Hurata', 'rc').title), row(lst, 'Hurata', 'rc').title);
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
  assert.equal(ui.doc.querySelector('#lsb-fac').disabled, true, 'the faction cannot change mid-write');
  assert.equal(ui.doc.querySelector('.lcc-bd-insp #lcc-bd-fac').disabled, true, 'nor the faction box of the actions');
  click(ui, btn); click(ui, act(ui, 'Thorvald', 'fill')); chooseShown(ui); bulk(ui, 'complete');
  await new Promise((r) => ui.w.setTimeout(r, 5));
  assert.equal(h.started, 1, 'a second click did not start a second write');
  assert.ok(result(ui).includes('still running'), result(ui));
  h.release(); await ui.settle();
  assert.equal(writes(ui).length, 1);
  assert.equal(ui.doc.querySelector('#lsb').getAttribute('aria-busy'), null);
  assert.equal(ui.doc.querySelector('#lsb-fac').disabled, false);
  assert.ok(lines(ui)[0].startsWith('House Thorvald:'), result(ui));
  assert.equal(tier(ui, 'Thorvald', 5).getAttribute('aria-disabled'), 'false');
});

test('a refused write shows the error text unchanged, writes nothing, and leaves the control usable', async () => {
  const msg = 'house Thorvald was taken since you looked, so nothing was changed';
  const ui = await open(board(), { '/api/landsraad/apply': { __fail: msg, __code: 'conflict', __status: 409 } });
  const before = boardFetches(ui);
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  assert.equal(result(ui), msg);
  const t = lastToast(ui);
  assert.ok(t.classList.contains('err') && t.getAttribute('role') === 'alert', 'an error is a toast that stays until it is dismissed');
  assert.ok(t.querySelector('button.x'), 'with a dismiss control');
  assert.ok(boardFetches(ui) > before, 'after a conflict the board is read again to show what the save holds now');
  assert.equal(ui.doc.querySelector('#lsb').getAttribute('aria-busy'), null);
  assert.equal(tier(ui, 'Thorvald', 5).getAttribute('aria-disabled'), 'false');
  // another kind of refusal: no re-read, the control is back, the words are unchanged
  const ui2 = await open(board(), { '/api/landsraad/apply': { __fail: 'the game is running, so nothing was written', __code: 'game_running', __status: 409 } });
  const b2 = boardFetches(ui2);
  click(ui2, act(ui2, 'Alexin', 'fill')); await ui2.settle();
  assert.equal(result(ui2), 'the game is running, so nothing was written');
  assert.ok(lastToast(ui2).classList.contains('err'));
  assert.equal(boardFetches(ui2), b2);
  assert.equal(act(ui2, 'Alexin', 'fill').classList.contains('busy'), false);
  assert.equal(act(ui2, 'Alexin', 'fill').getAttribute('aria-disabled'), 'false');
  // and it can be pressed again
  click(ui2, act(ui2, 'Alexin', 'fill')); await ui2.settle();
  assert.equal(writes(ui2).length, 2);
});

test('a bulk button for more than one house asks first with the count; Cancel and Escape write nothing and put the focus back; Yes writes the houses in one edit', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 3, houses: 3, contribution_raised: 0, completed: 3, reward_rows: 0 } });
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), '', 'no standing sentence when nothing is chosen (#1062)');
  pickFour(ui);
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), 'Chosen: 4 houses. Fill chosen changes 3, Complete chosen changes 3 for Atreides, Complete and fill chosen changes 3, Reopen chosen changes 2, Unfill chosen changes 0.'); // this read has no unfill plan (#1013): Unfill refuses on such a read, so it counts no House
  bulk(ui, 'complete'); await ui.settle();
  const q = ui.doc.querySelector('#lsb-confirm');
  assert.ok(q, 'the question is shown');
  assert.equal(text(q.querySelector('b')), 'Complete 3 houses for Atreides?');
  assert.ok(questionText(ui.doc).endsWith('Written at once, with a backup first.'), questionText(ui.doc));
  assert.deepEqual([...q.querySelectorAll('button')].map(text), ['Complete 3 houses', 'Cancel']);
  assert.deepEqual(writes(ui), [], 'asking writes nothing');
  assert.equal(ui.doc.activeElement.dataset.fk, 'c-no', 'the safe answer has the focus');
  click(ui, '[data-fk="c-no"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.deepEqual(writes(ui), []);
  assert.equal(result(ui), 'Cancelled. Nothing was written.');
  assert.equal(ui.doc.activeElement.dataset.fk, 'b-complete');
  // Escape does the same
  bulk(ui, 'complete'); await ui.settle();
  ui.doc.activeElement.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.equal(ui.doc.activeElement.dataset.fk, 'b-complete');
  assert.deepEqual(writes(ui), []);
  // a change of the choice or of the faction closes the question: it named houses that are no longer the ones chosen
  bulk(ui, 'complete'); await ui.settle();
  pick(ui, 900, false); await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  pick(ui, 900); await ui.settle();
  bulk(ui, 'complete'); await ui.settle();
  select(ui, '#lsb-fac', 2); await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null, 'a change of the faction closes the question');
  select(ui, '#lsb-fac', 1); await ui.settle();
  bulk(ui, 'complete'); await ui.settle();
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
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
  pickFour(ui);
  bulk(ui, 'fill'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Fill 3 houses?');
  assert.ok(questionText(ui.doc).endsWith('Written at once, with a backup first.'));
  click(ui, '[data-fk="c-no"]'); await ui.settle();
  bulk(ui, 'reopen'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Reopen 2 houses?');
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.deepEqual(writes(ui)[0].body.houses, [
    { task_id: 903, was_mine: 6500, was_done: true, was_win: 1, reopen: true },
    { task_id: 902, was_mine: 1300, was_done: true, was_win: 2, reopen: true },
  ]);
  // one house chosen: no question
  click(ui, '[data-fk="b-none"]');
  pick(ui, 900); await ui.settle();
  bulk(ui, 'fill'); await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.equal(writes(ui).length, 2);
  assert.deepEqual(writes(ui)[1].body.houses, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000 }]);
  // nothing chosen, or nothing to change
  click(ui, '[data-fk="b-none"]');
  bulk(ui, 'fill');
  assert.ok(result(ui).includes('Nothing is chosen'));
  assert.equal(writes(ui).length, 2);
});

test('with another faction chosen, Fill and Fill chosen change nothing of yours and nothing is sent; Complete and Fill faction goal act for that faction without a contribution', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0 } });
  select(ui, '#lsb-fac', 2); await ui.settle();
  const off = act(ui, 'Alexin', 'fill');
  assert.equal(off.getAttribute('aria-disabled'), 'true');
  assert.ok(off.title.includes('Off while Harkonnen is chosen') && off.title.includes('Atreides'), off.title);
  click(ui, off); await ui.settle();
  assert.deepEqual(writes(ui), [], 'Fill wrote nothing');
  pick(ui, 900); pick(ui, 901); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).includes('Fill chosen changes 0,'), text(ui.doc.querySelector('#lsb-sum')));
  bulk(ui, 'fill'); await ui.settle();
  assert.deepEqual(writes(ui), [], 'Fill chosen wrote nothing');
  assert.ok(result(ui).includes('nothing to change'), result(ui));
  click(ui, act(ui, 'Alexin', 'fillgoal')); await ui.settle();
  assert.deepEqual(writes(ui)[0].body.houses, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, complete_for: 2 }], 'no mine_to: no contribution and so no reward row');
});

test('a character in no faction cannot change a contribution: the tiers and Fill are off with the reason, Complete still writes', async () => {
  const ui = await open(board({ character: { player_id: 1, name: 'Tester', faction_id: null } }), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0 } });
  assert.equal(tier(ui, 'Thorvald', 5).getAttribute('aria-disabled'), 'true');
  assert.ok(tier(ui, 'Thorvald', 5).getAttribute('data-why').includes('in no faction'));
  assert.equal(act(ui, 'Thorvald', 'fill').getAttribute('aria-disabled'), 'true');
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  assert.deepEqual(writes(ui), []);
  click(ui, act(ui, 'Thorvald', 'complete')); await ui.settle();
  assert.equal(writes(ui).length, 1);
  assert.equal(writes(ui)[0].body.houses[0].mine_to, undefined);
});

test('the Choose boxes of Houses a filter hides are cleared, and the Show tabs are off while Houses are chosen', async () => {
  const ui = await open();
  pickFour(ui);
  houseFilter(ui, 'taken'); // Alexin and Thorvald are open: hidden; Wayku and Hurata stay
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).startsWith('Chosen: 2 houses.'), text(ui.doc.querySelector('#lsb-sum')));
  assert.equal(ui.doc.querySelector('.lcc-bd-pick[data-id="900"]').checked, false);
  // the Show tabs change which Houses are shown, so they wait until the choice is cleared
  click(ui, '[data-fk="bf:show:done"]');
  assert.equal(ui.w.LCCB.state.show, 'all');
  assert.ok(result(ui).includes('Not available while Houses are chosen'), result(ui));
});

test('a save with no term writes nothing and says so: the board is not drawn for writing, and a request that got through is refused', async () => {
  const ui = await open(board({ term: null }));
  assert.equal(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-act'), null, 'no action to press');
  assert.equal(ui.doc.querySelector('[data-lb="tier"]'), null, 'no tier box to press');
  assert.ok(text(ui.doc.querySelector('#lsb')).includes('nothing can be written from this view'));
  // the guard alone: the write function itself refuses without a term
  const ok = await ui.w.lbSend(null, [{ task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000 }], 'House Alexin', new Map(), null); await ui.settle();
  assert.equal(ok, false);
  assert.deepEqual(writes(ui), []);
  assert.equal(result(ui), 'This save has no Landsraad term, so there is no board to change.');
  assert.ok(lastToast(ui).classList.contains('err'));
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
  assert.equal(ui.doc.querySelector('#lsb-tresult'), null, 'the term panel keeps no result line: what it says is a toast (#1120)');
  assert.equal(ui.doc.querySelector('#lsb-trf').disabled, true);
  release(); await ui.settle();
  assert.ok(said(ui).startsWith('Applied: active decree'), said(ui));
  assert.equal(ui.posted.filter((p) => p.path === '/api/landsraad/term').length, 1);
  click(ui, ui.doc.querySelector('[data-lb=tflag]')); await ui.settle();
  assert.ok(said(ui).startsWith('Decree 1: pool flag set to'), said(ui));
});

test('after a write the House is drawn from the saved data, and the focus stays on the pressed control even when Complete has turned into Reopen', async () => {
  const data = board();
  const ui = await open(data, { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0 } }, 'list');
  const orig = ui.w.fetch;
  ui.w.fetch = (p, opt = {}) => { // the save changes when the write is made
    if (opt.method === 'POST' && String(p).includes('/api/landsraad/apply')) { const t = data.tasks.find((x) => x.id === 901); t.completed = 1; t.winning_faction_id = 1; t.contributions = [{ faction_id: 1, amount: 4000 }]; }
    return orig(p, opt);
  };
  assert.equal(row(ui, 'Thorvald', 'rc').getAttribute('data-do'), 'complete');
  click(ui, row(ui, 'Thorvald', 'rc')); await ui.settle();
  assert.equal(row(ui, 'Thorvald', 'rc').getAttribute('data-do'), 'reopen', 'the row shows what the save holds now');
  assert.ok(text(ui.doc.querySelector('tr.lcc-bd-row[data-id="901"]')).includes('Pledged to Atreides'), 'the row says who holds the house now');
  assert.equal(ui.doc.activeElement.dataset.fk, 'rc:901', 'the focus is on the same place, now the Reopen button');
  // the same in the Inspector: the House is still looked at, and Reopen is on
  const g = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0 } });
  click(g, act(g, 'Thorvald', 'complete')); await g.settle();
  assert.equal(g.doc.querySelector('.lcc-bd-insp').dataset.id, '901');
  assert.equal(g.doc.activeElement.dataset.fk, 'ac:901', 'the focus stays on the Complete button that was pressed');
});

test('Cancel is ignored while the write of the bulk question runs: it does not say "Cancelled" about a write that goes on', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 3, houses: 3, contribution_raised: 0, completed: 3, reward_rows: 0 } });
  const h = hold(ui);
  pickFour(ui);
  bulk(ui, 'complete'); await ui.settle();
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.equal(h.started, 1);
  click(ui, '[data-fk="c-no"]');
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
  b.tasks.forEach((t) => { t.line = LINES[t.house_name.replace('DA_House', '')] || 'Crafting'; t.my_events = events[t.id] || 0; });
  b.grant = { ...GRANT, ...g };
  return b;
};
const GRANTED = { ...OK, completed: 1, reward_rows: 3, missions: 3, solari: 18000, house_scrip: 3000, specialization_xp: { Exploration: 9000 }, rewards_already_given: 0 };

test('the toast says what was written, in words that do not claim the pay-out is checked, one line per fact', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': GRANTED });
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
  const l = lines(ui), r = result(ui);
  assert.ok(l[0].includes('3 rewards now waiting at the house representative'), l[0]);
  assert.ok(l.includes('3 missions written: +18,000 Solari, +3,000 House Scrip, +9,000 Exploration XP.'), r);
  assert.ok(r.includes('house taken, its faction reached the goal'), r);
  assert.ok(r.includes('has not been checked in the game yet'), 'the pay-out is said to be unchecked: ' + r);
  assert.ok(!/granted|Warning/.test(r), r);
  const t = lastToast(ui);
  assert.ok(!t.classList.contains('warn') && !t.classList.contains('part') && !t.classList.contains('err'), 'a plain success is not drawn as a warning: ' + t.className);
  assert.equal(t.getAttribute('role'), 'status', 'the result is announced as a status');
});

test('while the game runs the result says the change is queued, never that anything was written or granted', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...GRANTED, outcome: 'queued' } });
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('3 missions to write: +18,000 Solari') && r.includes('3 rewards to leave waiting at the house representative'), r);
  assert.ok(r.includes('queued: written as soon as the save can be written'), 'the hint is in the toast: ' + r);
  assert.ok(!/granted|missions written|now waiting/.test(r), r);
  const t = lastToast(ui);
  assert.ok(t.classList.contains('part'), 'a queued change is a warning that stays until it is dismissed, not a success: ' + t.className);
});

test('a raise whose rewards were already delivered says so as a warning, and says that no missions were written', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, reward_rows: 0, missions: 0, solari: 0, house_scrip: 0, specialization_xp: {}, rewards_already_given: 2 } });
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('No missions written: your own earlier progress on this house already counts for this ground'), r);
  assert.ok(lines(ui).includes('Warning: 2 rewards were left out: earlier progress on this house already counted for them.'), r);
  assert.ok(!/granted|tiers? not written|delivered/.test(r), r);
  assert.ok(lastToast(ui).classList.contains('part'), 'the toast is a warning, not a success: ' + lastToast(ui).className);
  const one = await open(board(), { '/api/landsraad/apply': { ...OK, reward_rows: 0, missions: 0, rewards_already_given: 1 } });
  click(one, act(one, 'Alexin', 'fill')); await one.settle();
  assert.ok(result(one).includes('Warning: 1 reward was left out: earlier progress on this house already counted for it.'), result(one));
});

test('a bulk write sums the missions of every house and names each specialization line', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 2, houses: 2, contribution_raised: 2, reward_rows: 6, missions: 5, solari: 30000, house_scrip: 5000, specialization_xp: { Crafting: 3000, Gathering: 4500 }, rewards_already_given: 1 } });
  pickFour(ui);
  bulk(ui, 'fill'); await ui.settle();
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('5 missions written: +30,000 Solari, +5,000 House Scrip, +3,000 Crafting XP, +4,500 Gathering XP'), r);
  assert.ok(r.includes('Warning: 1 reward was left out'), r);
});

// what removing the highest reached tier of Thorvald (tier 3) would do to its reward, as the server's board read says it (landsraad_takeback.go)
const withBack = (outcome, why = '') => { const d = board(); const r = d.tasks.find((t) => t.id === 901).rewards[2]; r.takeback = outcome; r.takeback_why = why; r.reward_state = outcome === 'take' ? 'waiting' : outcome === 'collected' ? 'delivered' : 'no_row'; return d; };
const question = (ui) => questionText(ui.doc);
// the sentences a removal says whole (LB_REMOVE_NOTE, app.js): what stays, that raising the tier again pays no mission twice, and the way back
const NOTE = 'Rewards already collected stay, and so do the Solari, House Scrip and XP the missions of the tier paid, and raising the tier again pays no mission a second time. To go back, restore the backup (Database tab, Save backups); the editor does not restore its own record with it, so a raise after that may pay less than before.';

test('a removal with no reward waiting for its tier writes at once, and says that no reward waited and what stays', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0 } });
  const t3 = tier(ui, 'Thorvald', 3); // the highest reached tier of Thorvald (4,700): clicking it removes it
  assert.ok(ttl(ui, t3).endsWith('Remove tier 3: your contribution drops to 2,400. No reward of this tier waits at the house representative, so there is nothing to take back. ' + NOTE), ttl(ui, t3));
  assert.ok(!ttl(ui, t3).includes('takes back nothing') && !t3.getAttribute('aria-label').includes('takes back nothing'));
  click(ui, t3); await ui.settle();
  assert.equal(question(ui), null, 'nothing to say about a reward: no question');
  assert.equal(writes(ui).length, 1);
  const r = result(ui);
  assert.ok(r.includes('(tier 3 removed)') && r.includes('No reward waited for the removed tier, so there was nothing to take back') && r.includes('Rewards already collected stay') && r.includes('restore the backup'), r);
  assert.equal(lastToast(ui).classList.contains('err'), false);
});

test('a removal that takes back a waiting reward asks first, names the reward, and the answer says how many were taken back', async () => {
  const ui = await open(withBack('take'), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0, rewards_taken_back: 1 } });
  const t3 = tier(ui, 'Thorvald', 3);
  assert.ok(ttl(ui, t3).endsWith('Remove tier 3: your contribution drops to 2,400. Takes back its reward, which still waits at the house representative: 50 Precision Range Finder. Raising the tier again writes that reward once more. ' + NOTE), ttl(ui, t3));
  click(ui, t3); await ui.settle();
  assert.equal(writes(ui).length, 0, 'nothing is written before the answer');
  const q = question(ui);
  assert.ok(q.includes('Remove tier 3 on House Thorvald? Takes back its reward, which still waits at the house representative: 50 Precision Range Finder. Raising the tier again writes that reward once more. ' + NOTE + ' Written at once, with a backup first.'), q);
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.deepEqual(writes(ui).map((w) => w.body), [{ term_id: 7, houses: [{ task_id: 901, was_mine: 4700, was_done: false, was_win: null, mine_to: 2400, lower: true }] }]);
  assert.ok(result(ui).includes('(tier 3 removed)') && result(ui).includes('Took back 1 waiting reward'), result(ui));
});

test('a removal can be answered No: nothing is written', async () => {
  const ui = await open(withBack('take'), { '/api/landsraad/apply': OK });
  click(ui, tier(ui, 'Thorvald', 3)); await ui.settle();
  assert.ok(question(ui));
  click(ui, '[data-fk="c-no"]'); await ui.settle();
  assert.equal(writes(ui).length, 0);
  assert.equal(question(ui), null);
});

test('a removal names a reward that is already collected and stays, and one that stays because it cannot be tied to the tier', async () => {
  let ui = await open(withBack('collected'), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0, rewards_kept_collected: 1 } });
  click(ui, tier(ui, 'Thorvald', 3)); await ui.settle();
  assert.ok(question(ui).includes('is already collected and stays.'), question(ui));
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.ok(result(ui).includes('1 collected reward stays'), result(ui));
  closeAll();
  ui = await open(withBack('unsure', 'the same item is also the reward of another tier'), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0, rewards_kept_unsure: 1 } });
  click(ui, tier(ui, 'Thorvald', 3)); await ui.settle();
  assert.ok(question(ui).includes('stays: it cannot be tied to this tier with certainty, because the same item is also the reward of another tier.'), question(ui));
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.ok(result(ui).includes('1 reward left alone (not certain it belongs to the removed tier)'), result(ui));
});

test('the line under the highest reached tier box says what a click does to its reward, from the outcome the server gives for the tier', async () => {
  const want = { take: 'click removes it; takes back its waiting reward', collected: 'click removes it; its reward is collected and stays', earned: 'click removes it; its waiting reward stays', unsure: 'click removes it; its waiting reward stays', none: 'click removes it; no reward waits', '': 'click removes it; no reward waits' };
  for (const [outcome, line] of Object.entries(want)) {
    const ui = await open(outcome ? withBack(outcome, 'a reason') : board(), {});
    assert.equal(grant(tier(ui, 'Thorvald', 3)), line, 'outcome ' + JSON.stringify(outcome));
    closeAll();
  }
});

test('a removal names a waiting reward that stays because the character own missions cover its tier, and the answer counts it apart from the ones not certain', async () => {
  const ui = await open(withBack('earned', 'your own missions in the game cover this tier'), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, contribution_lowered: 1, reward_rows: 0, rewards_kept_earned: 1 } });
  click(ui, tier(ui, 'Thorvald', 3)); await ui.settle();
  assert.ok(question(ui).includes('A reward (50 Precision Range Finder) waits for this tier and stays: your own missions in the game cover this tier.'), question(ui));
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.ok(result(ui).includes('1 waiting reward stays (your own missions cover the tier)') && !result(ui).includes('not certain'), result(ui));
});

test('before any click the board says what Fill, Fill faction goal and each tier grant, in text tied to the control, and that it writes at once', async () => {
  const ui = await open(withGrant({}, { 901: 4700 }), {});
  // Fill and Fill faction goal: the sentence is in the mouse-over of the button, before the click
  const fill = act(ui, 'Alexin', 'fill').title, goal = act(ui, 'Alexin', 'fillgoal').title;
  assert.ok(fill.includes('Writes 6 missions: +36,000 Solari, +6,000 House Scrip, +9,000 Crafting XP, 5 rewards waiting at the house representative.') && fill.includes('Written at once.'), fill);
  assert.ok(goal.includes('Writes 6 missions: +36,000 Solari, +6,000 House Scrip, +9,000 Crafting XP, 5 rewards waiting at the house representative.') && goal.includes('Written at once.'), goal);
  // the next tier: its own box says what giving it writes
  assert.ok(ttl(ui, tier(ui, 'Alexin', 1)).includes('Writes 2 missions: +12,000 Solari, +2,000 House Scrip, +3,000 Crafting XP, 1 reward waiting at the house representative.'), ttl(ui, tier(ui, 'Alexin', 1)));
  // each tier box that would grant shows it as text, and its accessible name carries it
  assert.equal(grant(tier(ui, 'Alexin', 5)), '6 missions: +36,000 Solari');
  assert.ok(tier(ui, 'Alexin', 5).getAttribute('aria-label').includes('Writes 6 missions: +36,000 Solari'));
  assert.equal(grant(tier(ui, 'Alexin', 1)), '2 missions: +12,000 Solari');
  // the highest reached tier says what a click does; a tier the character's own events already cover grants no missions
  assert.equal(grant(tier(ui, 'Thorvald', 3)), 'click removes it; no reward waits');
  assert.equal(grant(tier(ui, 'Thorvald', 4)), 'no missions');
  // the standing line about writes is gone (#980): the Help tab says it, and the question before a write says what it writes
  assert.equal(ui.doc.querySelector('#lsb-legend'), null);
  assert.ok(text(ui.doc.querySelector('[data-fk="b-fill"]')).includes('The question shows the totals'));
});

test('the ground the character own events already cover is not counted again in what a click says it grants', async () => {
  // Thorvald is at 4,700 and its own events cover 6,000: a raise to 6,000 stands for no mission
  const ui = await open(withGrant({}, { 901: 6000 }), {});
  assert.equal(grant(tier(ui, 'Thorvald', 5)), 'no missions');
  assert.ok(act(ui, 'Thorvald', 'fill').title.includes('Writes no missions: your own earlier progress already counts for this ground'));
});

test('what the character reached under another faction is not counted again in what a click says it grants', async () => {
  // Thorvald: the character reached 6,000 under another faction (my_elsewhere), so a raise to 6,000 under this one stands for no mission
  const b = withGrant();
  b.tasks.forEach((t) => { if (t.id === 901) t.my_elsewhere = 6000; });
  const ui = await open(b, {});
  assert.equal(grant(tier(ui, 'Thorvald', 5)), 'no missions');
  assert.ok(act(ui, 'Thorvald', 'fill').title.includes('Writes no missions: your own earlier progress already counts for this ground'));
});

test('a setting the editor cannot use, or a house with no line, is said before the click as a refusal', async () => {
  const bad = 'The Landsraad amounts cannot be worked out: LandsraadContributionMultiplier in ServerCustomSettings.ini is "x", not a number from 0 to 10. Nothing was changed; correct the value in ServerCustomSettings.ini (Config tab) and try again.';
  const ui = await open(withGrant({ problem: bad }), {});
  assert.equal(grant(tier(ui, 'Alexin', 5)), 'will be refused');
  assert.ok(act(ui, 'Alexin', 'fill').title.includes('Will be refused: ' + bad));
  const b2 = withGrant(); b2.tasks[1].line = '';
  const ui2 = await open(b2, {});
  const cap = act(ui2, 'Alexin', 'fill').title;
  assert.ok(cap.includes('has no specialization line in the editor') && cap.includes('use Complete for this one'), cap);
});

test('a click on one house that would grant ten missions or more asks first, with the totals and a button named for the action; Cancel writes nothing and returns the focus', async () => {
  // a mission step of 500: Alexin 0 to 6,000 is 12 missions (72,000 Solari)
  const ui = await open(withGrant({ step: 500 }), { '/api/landsraad/apply': { ...GRANTED, missions: 12, solari: 72000, house_scrip: 12000, specialization_xp: { Crafting: 18000 } } });
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
  assert.deepEqual(writes(ui), [], 'asking writes nothing');
  const q = ui.doc.querySelector('#lsb-confirm');
  assert.equal(question(ui), 'Fill House Alexin? Writes 12 missions: +72,000 Solari, +12,000 House Scrip, +18,000 Crafting XP, 5 rewards waiting at the house representative. Written at once, with a backup first.');
  assert.deepEqual([...q.querySelectorAll('button')].map(text), ['Fill House Alexin', 'Cancel']);
  assert.equal(ui.doc.activeElement.dataset.fk, 'c-no');
  click(ui, '[data-fk="c-no"]'); await ui.settle();
  assert.deepEqual(writes(ui), []);
  assert.equal(ui.doc.activeElement.dataset.fk, 'qf:900', 'the focus goes back to the control that was pressed');
  // answering writes the request the page showed, and the question is gone
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.deepEqual(writes(ui).map((w) => w.body.houses), [[{ task_id: 900, was_mine: 0, was_done: false, was_win: null, mine_to: 6000, was_events: 0 }]]);
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  // a tier click that would grant as much asks the same way; Fill faction goal names the faction
  const ui2 = await open(withGrant({ step: 500 }), { '/api/landsraad/apply': GRANTED });
  click(ui2, tier(ui2, 'Alexin', 5)); await ui2.settle();
  assert.equal(text(ui2.doc.querySelector('#lsb-confirm b')).split('?')[0], 'Give tier 5 on House Alexin');
  click(ui2, '[data-fk="c-no"]'); await ui2.settle();
  click(ui2, act(ui2, 'Alexin', 'fillgoal')); await ui2.settle();
  assert.equal(text(ui2.doc.querySelector('#lsb-confirm b')).split('?')[0], 'Fill faction goal on House Alexin for Atreides');
  assert.deepEqual(writes(ui2), []);
  // below the threshold a click writes at once, as before
  const ui3 = await open(withGrant({}), { '/api/landsraad/apply': GRANTED });
  click(ui3, act(ui3, 'Alexin', 'fill')); await ui3.settle();
  assert.equal(ui3.doc.querySelector('#lsb-confirm'), null);
  assert.equal(writes(ui3).length, 1);
});

test('the bulk question carries the totals of the raise and names its button by the action', async () => {
  const ui = await open(withGrant({}, { 901: 4700 }), { '/api/landsraad/apply': { ...OK, changed: 3, houses: 3, contribution_raised: 3 } });
  pickFour(ui);
  bulk(ui, 'fill'); await ui.settle();
  const q = ui.doc.querySelector('#lsb-confirm');
  // Alexin 6 missions and 5 rewards, Thorvald 1 and 2, Hurata (1,300 to 6,000) 4 and 4; Wayku is above the top tier and is not asked about
  assert.equal(text(q.querySelector('b')), 'Fill 3 houses?');
  assert.ok(questionText(ui.doc).includes('Writes 11 missions: +66,000 Solari, +11,000 House Scrip, +1,500 Combat XP, +9,000 Crafting XP, +6,000 Exploration XP, 11 rewards waiting at the house representative.') && questionText(ui.doc).endsWith('Written at once, with a backup first.'), questionText(ui.doc));
  assert.deepEqual([...q.querySelectorAll('button')].map(text), ['Fill 3 houses', 'Cancel']);
  // a refusal the editor would give is in the question too
  const bad = withGrant({ problem: 'LandsraadContributionMultiplier is 0. Nothing was changed.' });
  const ui2 = await open(bad, {});
  pickFour(ui2); bulk(ui2, 'fill'); await ui2.settle();
  assert.ok(questionText(ui2.doc).includes('Will be refused: LandsraadContributionMultiplier is 0.'), questionText(ui2.doc));
});

test('the bulk question says it will be refused when the houses together stand for more missions than one change may write', async () => {
  // a step of 10: Alexin alone is 600 missions, Hurata 470, together over the 1,000 of one change
  const ui = await open(withGrant({ step: 10 }), {});
  pickFour(ui);
  bulk(ui, 'fill'); await ui.settle();
  const q = questionText(ui.doc);
  assert.ok(q.includes('Will be refused: This request stands for') && q.includes('more than the 1,000 one change may grant') && q.includes('Fill fewer houses at once'), q);
  assert.deepEqual(writes(ui), []);
});

test('with another faction chosen the tier boxes are off with the reason and write nothing; with your own faction they work again (#667)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  select(ui, '#lsb-fac', 2); await ui.settle();
  for (const i of [1, 3, 5]) {
    const b = tier(ui, 'Thorvald', i);
    assert.equal(b.getAttribute('aria-disabled'), 'true', 'tier ' + i);
    const why = b.getAttribute('data-why');
    assert.ok(why.startsWith('Off while Harkonnen is chosen') && why.includes('your own contribution') && why.includes('Atreides'), why);
    assert.ok(!why.includes('Reward') && !why.includes('Game id'), 'the reason is the reason only');
    assert.ok(b.getAttribute('aria-label').includes(', off') && !b.getAttribute('aria-label').includes('Off while'), 'the name says it is off, the reason is in the mouse-over');
    assert.equal(b.querySelector('.lcc-bd-tier-gr'), null, 'no promise of a grant on a box that does nothing');
  }
  assert.ok(act(ui, 'Thorvald', 'fill').title.startsWith('Off while Harkonnen is chosen'), 'Fill says the same');
  click(ui, tier(ui, 'Thorvald', 5)); await ui.settle();
  click(ui, tier(ui, 'Thorvald', 2)); await ui.settle(); // the highest reached tier is the one that removes
  assert.deepEqual(writes(ui), [], 'no tier click wrote anything');
  assert.ok(result(ui).startsWith('Off while Harkonnen is chosen'), result(ui));
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

test('the toast says which win a completion replaced and how many other completion times were moved (#611)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, contribution_raised: 0, completed: 1, reward_rows: 0, replaced: [{ house: 'Hurata', previous_winner: 'Harkonnen' }], repaired: 2 } });
  click(ui, act(ui, 'Thorvald', 'complete')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('; replaces the win of Harkonnen on Hurata'), r);
  assert.ok(r.includes("; 2 other completion times put back inside the term's window"), r);
});

test('a write that changed no house but moved other completion times does not say there was nothing to do (#611)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 0, noop: true, contribution_raised: 0, reward_rows: 0, replaced: [], repaired: 1 } });
  click(ui, act(ui, 'Thorvald', 'complete')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('already in that state'), r);
  assert.ok(r.includes("; 1 other completion time put back inside the term's window"), r);
  assert.ok(!r.includes('nothing to change'), 'a write that moved times does not say there was nothing to change: ' + r); // #775
  // with no time moved it still says so
  const none = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 0, noop: true, contribution_raised: 0, reward_rows: 0, replaced: [], repaired: 0 } });
  click(none, act(none, 'Thorvald', 'complete')); await none.settle();
  assert.ok(result(none).includes('nothing to change, the house was already in that state'), result(none));
});

// the notes of a write (a win taken away, other times moved) come before the warning, not inside its sentence (#775)
test('the notes of a write stand before the warning, not inside it', async () => {
  const one = await open(board(), { '/api/landsraad/apply': { ...OK, reward_rows: 0, missions: 0, rewards_already_given: 1, replaced: [{ house: 'Alexin', previous_winner: 'Harkonnen' }], repaired: 2 } });
  click(one, act(one, 'Alexin', 'fill')); await one.settle();
  const l = lines(one), warn = l.findIndex((x) => x.startsWith('Warning: 1 reward was left out')), note = l.findIndex((x) => x.includes('; replaces the win of Harkonnen on Alexin'));
  assert.ok(note > -1 && warn > -1 && note < warn, l.join(' | '));
  assert.ok(l[note].includes("; 2 other completion times put back inside the term's window"), l[note]);
  assert.ok(!l[warn].includes('replaces') && !l[warn].includes('other completion'), l[warn]);
});

// The ground the editor paid before (#777): the board read carries it (my_paid), the sentence before a click names the missions that would not be paid, and the result says so.
test('ground the editor paid before is not counted in what a click grants, and the sentence before the click says so', async () => {
  // Thorvald is at 4,700 and the editor paid 6,000 before (a tier removal took the contribution down): a raise to 6,000 stands for no mission, and one of them is named as not paid
  const b = withGrant();
  b.tasks.forEach((t) => { if (t.id === 901) { t.my_paid = 6000; t.my_paid_rows = 6000; } });
  const ui = await open(b, {});
  assert.equal(grant(tier(ui, 'Thorvald', 5)), 'no missions');
  const cap = act(ui, 'Thorvald', 'fill').title;
  assert.ok(cap.includes('Writes no missions. 1 mission not paid: the editor recorded that ground as paid in an earlier raise, and removing a tier does not take back what its missions paid.'), cap);
  assert.ok(act(ui, 'Thorvald', 'fillgoal').title.includes('Writes no missions. 1 mission not paid: the editor recorded that ground as paid in an earlier raise'), act(ui, 'Thorvald', 'fillgoal').title);
  // the rewards a removal took back are not on the board any more (my_paid_rows is below the paid ground): the raise writes them again and still pays no mission
  const bb = withGrant();
  bb.tasks.forEach((t) => { if (t.id === 901) { t.my_paid = 6000; t.my_paid_rows = 4700; } });
  const ub = await open(bb, {});
  const capb = act(ub, 'Thorvald', 'fill').title;
  assert.ok(capb.includes('Writes no missions; 2 rewards waiting at the house representative. 1 mission not paid: the editor recorded that ground as paid in an earlier raise, and removing a tier does not take back what its missions paid.'), capb);
  // paid ground below the target: only the part above it is written, and the part below is named
  const b2 = withGrant();
  b2.tasks.forEach((t) => { if (t.id === 900) { t.my_paid = 2400; t.my_paid_rows = 2400; } });
  const ui2 = await open(b2, {});
  const al = act(ui2, 'Alexin', 'fill').title;
  assert.ok(al.includes('Writes 3 missions: +18,000 Solari') && al.includes('3 missions not paid: the editor recorded that ground as paid in an earlier raise, and removing a tier does not take back what its missions paid.'), al);
  assert.equal(grant(tier(ui2, 'Alexin', 5)), '3 missions: +18,000 Solari');
});

test('the result of a write says how many missions were not paid because that ground was paid before', async () => {
  const ui = await open(withGrant(), { '/api/landsraad/apply': { ...OK, reward_rows: 0, missions: 0, solari: 0, house_scrip: 0, specialization_xp: {}, rewards_already_given: 2, missions_paid_before: 3, ground_paid_before: 3000 } });
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('No missions written. 3 missions not paid: the editor recorded that ground as paid in an earlier raise, and removing a tier does not take back what its missions paid.'), r);
  const part = await open(withGrant(), { '/api/landsraad/apply': { ...GRANTED, missions: 1, solari: 6000, house_scrip: 1000, missions_paid_before: 2, ground_paid_before: 2000 } });
  click(part, act(part, 'Alexin', 'fill')); await part.settle();
  assert.ok(result(part).includes('1 mission written: +6,000 Solari, +1,000 House Scrip') && result(part).includes('2 missions not paid: the editor recorded that ground as paid in an earlier raise'), result(part));
  // an answer that paid everything has no such sentence
  const full = await open(withGrant(), { '/api/landsraad/apply': GRANTED });
  click(full, act(full, 'Alexin', 'fill')); await full.settle();
  assert.ok(!result(full).includes('not paid'), result(full));
});

test('a swatch won in this term is not counted as a reward waiting in the sentence before the click', async () => {
  // the top tier of Alexin is the house swatch (swatch: true); with the swatch held in this term the write skips it, so 4 rewards wait, not 5
  const held = withGrant(); held.tasks.forEach((t) => { if (t.id === 900) t.swatch_held = true; });
  const ui = await open(held, {});
  const cap = act(ui, 'Alexin', 'fill').title;
  assert.ok(cap.includes('Writes 6 missions: +36,000 Solari, +6,000 House Scrip, +9,000 Crafting XP, 4 rewards waiting at the house representative.'), cap);
  const free = await open(withGrant(), {});
  assert.ok(act(free, 'Alexin', 'fill').title.includes(', 5 rewards waiting at the house representative.'), 'a swatch not held yet is delivered: 5 rewards');
});

// ---- leftovers of the review of #652 (#752, #762): the experience that did not fit, the preview near the last level, the ground the page showed

test('a track at its last level is said in the toast: the experience that did not fit is named by line (#752)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...GRANTED, specialization_xp: {}, specialization_xp_not_added: { Exploration: 9000 } } });
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
  const r = result(ui);
  assert.ok(r.includes('3 missions written: +18,000 Solari, +3,000 House Scrip; Exploration is at its last level, +9,000 XP not added'), r);
  const q = await open(board(), { '/api/landsraad/apply': { ...GRANTED, outcome: 'queued', specialization_xp: { Exploration: 1500 }, specialization_xp_not_added: { Exploration: 7500 } } });
  click(q, act(q, 'Alexin', 'fill')); await q.settle();
  assert.ok(result(q).includes('+1,500 Exploration XP; Exploration is at its last level, +7,500 XP will not be added'), result(q));
  // an answer with nothing left over (and an older answer without the field) says nothing about it
  const none = await open(board(), { '/api/landsraad/apply': { ...GRANTED, specialization_xp_not_added: {} } });
  click(none, act(none, 'Alexin', 'fill')); await none.settle();
  assert.ok(!result(none).includes('last level'), result(none));
});

test('the sentence before a click does not promise more experience than the track has room for, one house or several of one line (#762)', async () => {
  const ui = await open(withGrant({ room: { Crafting: 2000 } }, { 901: 4700 }), {});
  const cap = act(ui, 'Alexin', 'fill').title;
  assert.ok(cap.includes('Writes 6 missions: +36,000 Solari, +6,000 House Scrip, +2,000 Crafting XP'), cap);
  assert.ok(!cap.includes('+9,000'), cap);
  // two houses of one line share the track: together they add no more than its room
  const b = withGrant({ room: { Crafting: 5000 } });
  b.tasks.forEach((t) => { if (t.id === 901) t.line = 'Crafting'; });
  const two = await open(b, {});
  pickFour(two);
  bulk(two, 'fill'); await two.settle();
  const q = questionText(two.doc);
  assert.ok(q.includes('+5,000 Crafting XP') && !q.includes('10,500'), q);
});

test('a raise sends the ground the character own events covered when the page drew the house, so the server can refuse it when that changed (#762)', async () => {
  const ui = await open(withGrant({}, { 900: 1200 }), { '/api/landsraad/apply': OK });
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
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
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
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
  click(old, act(old, 'Alexin', 'fill')); await old.settle();
  const ho = writes(old)[0].body.houses[0];
  assert.ok(!('was_paid' in ho) && !('was_paid_rows' in ho) && !('was_elsewhere' in ho), JSON.stringify(ho));
});

test('a house with no specialization line is refused before the click only when the raise pays a mission, as the write refuses it (#762)', async () => {
  // a mission step of 100,000 and 100 already covered: Alexin 0 to 6,000 crosses no mission step, so no experience is written
  const b = withGrant({ step: 100000 }, { 900: 100 });
  b.tasks.forEach((t) => { if (t.id === 900) t.line = ''; });
  const ui = await open(b, {});
  const cap = act(ui, 'Alexin', 'fill').title;
  assert.ok(!cap.includes('has no specialization line') && !cap.includes('Will be refused'), cap);
  assert.equal(grant(tier(ui, 'Alexin', 5)), 'no missions');
});

// #818: a bulk question names the houses chosen when it was asked. A filter unticks the houses it hides, so the question must not write them.

test('a filter that unticks houses of an open bulk question closes it, and nothing is written for a house that is not chosen (#818)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 3, houses: 3 } });
  pickFour(ui);
  bulk(ui, 'fill'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Fill 3 houses?');
  houseFilter(ui, 'open'); await ui.settle(); // Wayku and Hurata are taken: hidden, and they lose their ticks
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null, 'the question about three houses is gone');
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).startsWith('Chosen: 2 houses.'), text(ui.doc.querySelector('#lsb-sum')));
  assert.ok(result(ui).includes('question was closed') && result(ui).includes('Nothing was written'), result(ui));
  assert.deepEqual(writes(ui), []);
  // the answer button is gone; asking again names the houses that are chosen now
  assert.equal(ui.doc.querySelector('[data-fk="c-yes"]'), null);
  bulk(ui, 'fill'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Fill 2 houses?');
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.deepEqual(sentIds(ui), [900, 901], 'Alexin and Thorvald only: Hurata is hidden and not chosen');
});

test('the House filter closes an open bulk question too (#818, #830)', async () => {
  // the Classic list had a Jump to house box as well; the board has none, its House filter is the one that hides houses by whether a faction holds them
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  pickFour(ui);
  bulk(ui, 'complete'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Complete 3 houses for Atreides?');
  click(ui, '[data-fk="bf:hs:taken"]'); await ui.settle(); // Alexin and Thorvald are open: hidden and not chosen
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.deepEqual(writes(ui), []);
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).startsWith('Chosen: 2 houses.'), text(ui.doc.querySelector('#lsb-sum')));
  // what is chosen now (Wayku, Hurata) is asked about again and written by one answer
  bulk(ui, 'reopen'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Reopen 2 houses?');
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.deepEqual(sentIds(ui), [902, 903]);
});

test('a filter that hides only houses the open bulk question does not name leaves the question open, and Yes writes the houses it named (#818)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 2, houses: 2 } });
  for (const id of [900, 901, 903]) pick(ui, id);
  bulk(ui, 'complete'); await ui.settle(); // Wayku is complete already: it is chosen but not in the question
  assert.ok(ui.doc.querySelector('#lsb-confirm'));
  houseFilter(ui, 'open'); await ui.settle(); // hides Wayku only (Hurata is taken too, but it is not chosen)
  assert.ok(text(ui.doc.querySelector('#lsb-sum')).startsWith('Chosen: 2 houses.'), text(ui.doc.querySelector('#lsb-sum')));
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Complete 2 houses for Atreides?');
  // the question said '1 other chosen house is left alone'; that is no longer so, so the first Yes asks again with the words as they are now, and writes nothing
  assert.ok(questionText(ui.doc).includes('1 other chosen house is left alone'), questionText(ui.doc));
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.deepEqual(writes(ui), [], 'the first Yes only asks again');
  assert.ok(result(ui).includes('The houses changed since the question was asked'), result(ui));
  assert.ok(!questionText(ui.doc).includes('left alone'), questionText(ui.doc));
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.deepEqual(sentIds(ui), [900, 901]);
});

test('Yes never writes a house that is not chosen: with the closing of the question out of the way, the answer asks again with the smaller number (#818)', async () => {
  // the second guard on its own: the page's closing of the question is switched off, so the filter leaves the stale question open and only the check in the answer stands
  const ui = await open(board(), { '/api/landsraad/apply': { ...OK, changed: 2, houses: 2 } });
  ui.w.lbCancelConfirm = () => {};
  pickFour(ui);
  bulk(ui, 'complete'); await ui.settle();
  houseFilter(ui, 'open'); await ui.settle(); // Hurata is hidden and no longer chosen
  assert.ok(ui.doc.querySelector('#lsb-confirm'), 'the stale question is still shown');
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.deepEqual(writes(ui), [], 'the first Yes only asks again');
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Complete 2 houses for Atreides?');
  assert.ok(result(ui).includes('The houses changed since the question was asked'), result(ui));
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.deepEqual(sentIds(ui), [900, 901], 'Hurata was no longer chosen');
  // none of the houses of the question is chosen any more: nothing is written
  const ui2 = await open(board(), { '/api/landsraad/apply': OK });
  ui2.w.lbCancelConfirm = () => {};
  for (const id of [900, 901, 903]) pick(ui2, id);
  bulk(ui2, 'complete'); await ui2.settle();
  houseFilter(ui2, 'taken'); await ui2.settle(); // Alexin and Thorvald are hidden: only Wayku stays chosen, and Wayku is not in the question
  click(ui2, '[data-fk="c-yes"]'); await ui2.settle();
  assert.deepEqual(writes(ui2), []);
  assert.equal(ui2.doc.querySelector('#lsb-confirm'), null);
  assert.ok(result(ui2).includes('Nothing to change any more'), result(ui2));
});

test('a re-read that makes a named house drop out of the filter closes the question through the page redraw: nothing is written, the focus goes back to the bulk button (#831, #832)', async () => {
  const data = board();
  const ui = await open(data, { '/api/landsraad/apply': { ...OK, changed: 2, houses: 2 } });
  show(ui, 'part'); await ui.settle(); // In progress: Thorvald and Hurata
  chooseShown(ui);
  bulk(ui, 'fill'); await ui.settle();
  assert.equal(text(ui.doc.querySelector('#lsb-confirm b')), 'Fill 2 houses?');
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

test('a question about one clicked house is not a bulk question: a filter that hides the House does not close it or change what Yes writes (#818)', async () => {
  const ui = await open(withGrant({ step: 500 }), { '/api/landsraad/apply': GRANTED });
  pick(ui, 900); // Alexin is also ticked in the choice, so the filter will untick it
  click(ui, act(ui, 'Alexin', 'fill')); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-confirm b')).startsWith('Fill House Alexin?'));
  houseFilter(ui, 'taken'); await ui.settle(); // Alexin is open: hidden and unticked
  assert.equal(ui.doc.querySelector('.lcc-bd-pick[data-id="900"]').checked, false);
  assert.ok(ui.doc.querySelector('#lsb-confirm'), 'the question names the house itself and stays');
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
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

// #881: a House the board does not show is not written by a bulk button, even when the choice still holds it (a stale choice, or a House hidden without the page being told).
test('a bulk button drops a House the board does not show from the choice: it is not in the question or in the request (#881)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  pickFour(ui);
  houseFilter(ui, 'open'); await ui.settle(); // Hurata and Wayku are taken: hidden and unticked
  ui.w.LCC_HOST.picks().add(902); // a stale choice that holds the hidden House again
  bulk(ui, 'fill'); await ui.settle();
  const q = questionText(ui.doc);
  assert.ok(q.startsWith('Fill 2 houses?') && !q.includes('Hurata'), q);
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.ok(!sentIds(ui).includes(902), 'Hurata is not written: ' + sentIds(ui));
  assert.deepEqual(sentIds(ui), [900, 901]);
});

test('Yes does not write a House the board stopped showing after the question was asked (#881)', async () => {
  const ui = await open(board(), { '/api/landsraad/apply': OK });
  pickFour(ui);
  bulk(ui, 'fill'); await ui.settle();
  assert.ok(ui.doc.querySelector('#lsb-confirm'));
  ui.w.LCCB.state.show = 'part'; // the filter changed without the page being told: Alexin is not shown now
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.ok(!sentIds(ui).includes(900), 'Alexin is hidden, so it is not written: ' + sentIds(ui));
  // #1019: the test must not pass just because nothing was sent: the question is asked again, with the count as it is now, and nothing is written until it is answered
  assert.equal(sentIds(ui).length, 0, 'nothing is written by the first Yes: ' + sentIds(ui));
  assert.ok(result(ui).includes('The houses changed since the question was asked'), result(ui));
  assert.ok(ui.doc.querySelector('#lsb-confirm'), 'the question is open again');
  assert.ok(!questionText(ui.doc).includes('Alexin'), questionText(ui.doc));
  click(ui, '[data-fk="c-yes"]'); await ui.settle();
  assert.ok(sentIds(ui).length > 0 && !sentIds(ui).includes(900), 'the second Yes writes the others only: ' + sentIds(ui));
});
