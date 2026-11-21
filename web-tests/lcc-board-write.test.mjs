// The Command Center board writes through the editor's one route (M3, stages C and D). Every click of the Inspector is the classic list's own click: the same request, the same stale guard, the same
// busy lock, the same queued wording. This file proves it by doing the same thing in both views and comparing what was sent and what was said, and tests what only the board adds: the question
// before a win is replaced, the names in the question about several Houses, and the redraw that keeps the House and the focus. Real page code, the synthetic answer of M1, no game save.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic, questionText } from './lcc-fixture.mjs';

afterEach(closeAll);

const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 0, outcome: 'written' };
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const writes = (ui) => ui.posted.filter((p) => p.path === '/api/landsraad/apply');
// what the page said last: the Classic list's result line, or the toast of the board (the board keeps no result line, #943)
const toastEls = (ui) => [...ui.doc.querySelectorAll('#toast .d-toast')];
const toastText = (ui) => { const t = toastEls(ui).pop(); return t ? text(t.querySelector('span')) : ''; };
const result = (ui) => (ui.doc.querySelector('#lsb-result') ? text(ui.doc.querySelector('#lsb-result')) : toastText(ui));
// the same words without the punctuation that joins them: the Classic sentence and the board's lines say the same things
const same = (s) => s.replace(/[.;,:]/g, '').replace(/\s+/g, ' ').trim();
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
async function open(view, answer = synthetic(), post = { '/api/landsraad/apply': OK }) {
  const ui = await boot({ get: { '/api/landsraad': answer }, post, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': 'grid' } });
  await ui.settle();
  return ui;
}
// the same control in the two views: the classic list's opened row, or the board's Inspector
const control = {
  classic: {
    tier: (ui, a, name, i) => { const r = ui.doc.querySelector('#lsrow-' + idOf(a, name)); const x = r.querySelector('.xc'); if (!x.checked) { x.checked = true; x.dispatchEvent(new ui.w.Event('change', { bubbles: true })); } return r.querySelector('.tc[data-t="' + i + '"]'); },
    // in the classic row Fill is the quick action of the summary, and the opened row holds Fill faction goal, Complete and Reopen
    act: (ui, a, name, which) => { const r = ui.doc.querySelector('#lsrow-' + idOf(a, name)); const x = r.querySelector('.xc'); if (!x.checked) { x.checked = true; x.dispatchEvent(new ui.w.Event('change', { bubbles: true })); } return r.querySelector({ fill: '.c-qa .qa-fill', fillgoal: '.acts .act-fill', complete: '.acts .act-complete', reopen: '.acts .act-reopen' }[which]); },
  },
  board: {
    tier: (ui, a, name, i) => { select_(ui, a, name); return ui.doc.querySelector('.lcc-bd-insp .lcc-bd-tier[data-t="' + i + '"]'); },
    act: (ui, a, name, which) => { select_(ui, a, name); return ui.doc.querySelector('.lcc-bd-insp .lcc-bd-act[data-do="' + { fill: 'fill', fillgoal: 'fillgoal', complete: 'complete', reopen: 'reopen' }[which] + '"]'); },
  },
};
function select_(ui, a, name) { const sel = ui.doc.querySelector('.lcc-bd-insp'); if (!sel || sel.dataset.id !== String(idOf(a, name))) click(ui, ui.doc.querySelector('[data-fk="ht:' + idOf(a, name) + '"]')); }

// what each scenario does, in both views, on the same House
const SCENARIOS = [
  ['a tier raise', 'Kenola', (c, ui, a) => c.tier(ui, a, 'Kenola', 3)],
  ['the removal of the highest reached tier', 'Kenola', (c, ui, a) => c.tier(ui, a, 'Kenola', 1)],
  ['Fill', 'Lindaren', (c, ui, a) => c.act(ui, a, 'Lindaren', 'fill')],
  ['Fill faction goal', 'Lindaren', (c, ui, a) => c.act(ui, a, 'Lindaren', 'fillgoal')],
  ['Complete', 'Sor', (c, ui, a) => c.act(ui, a, 'Sor', 'complete')],
  ['Reopen', 'Hurata', (c, ui, a) => c.act(ui, a, 'Hurata', 'reopen')],
];
for (const [label, , pick] of SCENARIOS) {
  test('parity: ' + label + ' sends the same request and says the same in the classic list and on the board', async () => {
    const a = synthetic();
    const got = {};
    for (const view of ['classic', 'board']) {
      const ui = await open(view, synthetic());
      click(ui, pick(control[view], ui, a));
      await ui.settle();
      got[view] = { sent: writes(ui).map((w) => w.body), said: same(result(ui)) };
      closeAll();
    }
    assert.equal(got.classic.sent.length, 1, 'one request: ' + JSON.stringify(got.classic.sent));
    assert.deepEqual(got.board.sent, got.classic.sent, 'the board sends exactly what the classic list sends');
    assert.equal(got.board.said, got.classic.said, 'and says the same things, one result per line on the board');
    assert.equal(got.board.sent[0].term_id, 7, 'the term of the read is in the request, for the stale guard');
    const h = got.board.sent[0].houses[0];
    assert.ok('was_mine' in h && 'was_done' in h && 'was_win' in h, 'every request carries what the page showed (the stale guard)');
  });
}

test('the stale guard: a conflict from the save is shown as the save said it, and the board is read again', async () => {
  const msg = 'House Sor changed since it was shown: it is not as the page showed.';
  const ui = await open('board', synthetic(), { '/api/landsraad/apply': { __fail: msg, __code: 'conflict', __status: 409 } });
  const a = synthetic();
  const before = ui.fetched.filter((p) => p === '/api/landsraad').length;
  click(ui, control.board.act(ui, a, 'Sor', 'complete'));
  await ui.settle();
  assert.equal(result(ui), msg);
  assert.equal(ui.doc.querySelector('#lsb-result'), null, 'the board keeps no result line');
  const t = toastEls(ui).pop();
  assert.ok(t.classList.contains('err') && t.getAttribute('role') === 'alert', 'an error is a toast that stays until it is dismissed');
  assert.ok(t.querySelector('button.x'), 'with a dismiss control');
  assert.ok(ui.fetched.filter((p) => p === '/api/landsraad').length > before, 'the board is read again');
  assert.ok(ui.doc.querySelector('.lcc-bd-insp'), 'the House that was looked at is still looked at');
});

test('the write lock: a second click while a write runs sends nothing, says to wait, and the board is marked busy', async () => {
  const ui = await open('board');
  const a = synthetic();
  const orig = ui.w.fetch; let release = null, started = 0;
  ui.w.fetch = (p, opt = {}) => (opt.method === 'POST' && String(p).includes('/api/landsraad/apply') ? (started++, new Promise((res) => { release = () => res(orig(p, opt)); })) : orig(p, opt));
  const b = control.board.act(ui, a, 'Sor', 'complete');
  click(ui, b);
  await ui.settle();
  assert.equal(started, 1);
  assert.equal(ui.doc.querySelector('#lsb').getAttribute('aria-busy'), 'true');
  assert.equal(b.getAttribute('aria-disabled'), 'true');
  click(ui, b);
  click(ui, control.board.act(ui, a, 'Sor', 'fill'));
  await ui.settle();
  assert.equal(started, 1, 'nothing more was sent');
  assert.match(result(ui), /still running/);
  release(); await ui.settle();
  assert.equal(writes(ui).length, 1);
  assert.equal(ui.doc.querySelector('#lsb').getAttribute('aria-busy'), null);
});

// The busy state builds the faction and Set tier selects of a panel drawn during a write off (#1015); the write ends, they are on again. A plain write redraws the board while it still runs,
// so the new panel is built off and only the end of the write turns it on (a regression of #1015 on main: the selects stayed off after every write).
function holdWrite(ui) {
  const orig = ui.w.fetch, h = { release: null, started: 0 };
  ui.w.fetch = (p, opt = {}) => (opt.method === 'POST' && String(p).includes('/api/landsraad/apply') ? (h.started++, new Promise((res) => { h.release = () => res(orig(p, opt)); })) : orig(p, opt));
  return h;
}
const selectsOff = (ui) => ['#lcc-bd-fac', '#lcc-bd-tierset'].map((q) => ui.doc.querySelector('.lcc-bd-insp ' + q).disabled);

test('a plain successful write leaves the faction and Set tier selects on, with no other redraw', async () => {
  const ui = await open('board');
  const a = synthetic();
  select_(ui, a, 'Kenola');
  assert.deepEqual(selectsOff(ui), [false, false], 'on before the write');
  click(ui, control.board.tier(ui, a, 'Kenola', 3));
  await ui.settle();
  assert.equal(writes(ui).length, 1);
  assert.deepEqual(selectsOff(ui), [false, false], 'on again after the write and the re-read of the board');
});

test('a panel drawn while a write runs has its selects off, and they are on again when the write is over', async () => {
  const ui = await open('board');
  const a = synthetic();
  const h = holdWrite(ui);
  click(ui, control.board.act(ui, a, 'Sor', 'complete'));
  await ui.settle();
  assert.equal(h.started, 1);
  assert.deepEqual(selectsOff(ui), [true, true], 'the drawn panel is off while the write runs');
  click(ui, ui.doc.querySelector('[data-fk="ht:' + idOf(a, 'Kenola') + '"]')); // another House looked at: the panel is built again during the write
  assert.equal(ui.doc.querySelector('.lcc-bd-insp').dataset.id, String(idOf(a, 'Kenola')));
  assert.deepEqual(selectsOff(ui), [true, true], 'a panel drawn during the write is built off');
  h.release(); await ui.settle();
  assert.deepEqual(selectsOff(ui), [false, false], 'they are on again when the write is over');
});

test('a House with no tiers keeps Set tier off after a write', async () => {
  const a = synthetic();
  a.tasks.find((t) => t.house_name === 'DA_HouseKenola').rewards = [];
  const ui = await open('board', a);
  const h = holdWrite(ui);
  click(ui, control.board.act(ui, a, 'Sor', 'complete'));
  await ui.settle();
  click(ui, ui.doc.querySelector('[data-fk="ht:' + idOf(a, 'Kenola') + '"]'));
  assert.equal(ui.doc.querySelector('.lcc-bd-insp #lcc-bd-tierset').options.length, 1, 'the House has no tiers');
  h.release(); await ui.settle();
  assert.deepEqual(selectsOff(ui), [false, true], 'the faction box is on again, Set tier stays off');
});

test('after a write the same House is still looked at and the focus is on the control that was pressed', async () => {
  const ui = await open('board');
  const a = synthetic();
  click(ui, control.board.tier(ui, a, 'Kenola', 3));
  await ui.settle();
  assert.equal(ui.doc.querySelector('.lcc-bd-insp').dataset.id, String(idOf(a, 'Kenola')));
  assert.equal(ui.doc.activeElement.getAttribute('data-fk'), 'tc:' + idOf(a, 'Kenola') + ':3');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell').length, 25);
});

test('written and queued are said apart: a queued change is never called saved, in the result line and on the board', async () => {
  const ui = await open('board', synthetic(), { '/api/landsraad/apply': { ...OK, outcome: 'queued' } });
  click(ui, control.board.act(ui, synthetic(), 'Sor', 'complete'));
  await ui.settle();
  assert.match(result(ui), /queued: written as soon as the save can be written/);
  assert.ok(toastEls(ui).pop().classList.contains('part'), 'a queued change is a warning that stays until it is dismissed, not a success');
  assert.doesNotMatch(result(ui), /\bsaved\b/i);
  const written = await open('board');
  click(written, control.board.act(written, synthetic(), 'Sor', 'complete'));
  await written.settle();
  assert.doesNotMatch(result(written), /queued/);
});

test('another faction chosen: the tiers and Fill are off with the reason, a click sends nothing, and Complete acts for that faction', async () => {
  const ui = await open('board');
  const a = synthetic();
  select(ui, '#lsb-fac', 2);
  await ui.settle();
  click(ui, ui.doc.querySelector('[data-fk="ht:' + idOf(a, 'Lindaren') + '"]'));
  const tier = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-tier[data-t="2"]');
  assert.equal(tier.getAttribute('aria-disabled'), 'true');
  assert.match(tier.getAttribute('data-why'), /Off while Harkonnen is chosen/);
  assert.equal(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-act[data-do="fill"]').getAttribute('aria-disabled'), 'true');
  const fac = ui.doc.querySelector('.lcc-bd-insp #lcc-bd-fac');
  assert.equal(fac.value, '2', 'the faction box of the actions follows the chosen faction');
  assert.match(fac.getAttribute('title'), /Acting for Harkonnen/);
  click(ui, tier);
  await ui.settle();
  assert.equal(writes(ui).length, 0);
  assert.match(result(ui), /Off while Harkonnen is chosen/);
  click(ui, ui.doc.querySelector('.lcc-bd-insp .lcc-bd-act[data-do="complete"]'));
  await ui.settle();
  assert.equal(writes(ui)[0].body.houses[0].complete_for, 2);
});

test('Complete on a House another faction took asks first, names the faction whose win is replaced, and Cancel writes nothing', async () => {
  const ui = await open('board');
  const a = synthetic();
  const vernius = a.tasks.find((t) => t.house_name === 'DA_HouseVernius');
  assert.equal(vernius.winning_faction_id, 2, 'the fixture has Vernius taken by Harkonnen');
  const b = control.board.act(ui, a, 'Vernius', 'complete');
  click(ui, b);
  await ui.settle();
  assert.equal(writes(ui).length, 0, 'nothing is written before the answer');
  const q = ui.doc.querySelector('#lsb-confirm');
  assert.ok(q, 'the question is asked');
  assert.match(questionText(ui.doc), /^Complete House Vernius for Atreides\? Replaces the win of Harkonnen on House Vernius\. Written at once, with a backup first\.$/);
  assert.equal(ui.doc.activeElement.getAttribute('data-fk'), 'c-no', 'the safe answer has the focus');
  click(ui, '[data-fk="c-no"]');
  await ui.settle();
  assert.equal(writes(ui).length, 0);
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.match(result(ui), /Cancelled\. Nothing was written\./);
  assert.equal(ui.doc.activeElement.getAttribute('data-fk'), 'ac:' + vernius.id, 'the focus goes back to the control that was pressed');
  click(ui, control.board.act(ui, a, 'Vernius', 'complete'));
  click(ui, '[data-fk="c-yes"]');
  await ui.settle();
  assert.deepEqual(writes(ui).map((w) => w.body.houses), [[{ task_id: vernius.id, was_mine: vernius.my_contribution, was_done: true, was_win: 2, complete_for: 1 }]]);
});

test('the classic list is not changed by this: Complete on a taken House still writes at once there', async () => {
  const ui = await open('classic');
  const a = synthetic();
  click(ui, control.classic.act(ui, a, 'Vernius', 'complete'));
  await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.equal(writes(ui).length, 1);
});

test('several Houses: the question names them, the faction and how many change, and one request carries them all', async () => {
  const ui = await open('board');
  const a = synthetic();
  const names = ['Sor', 'Lindaren', 'Kenola'];
  for (const n of names) { const c = ui.doc.querySelector('.lcc-bd-pick[data-id="' + idOf(a, n) + '"]'); c.checked = true; c.dispatchEvent(new ui.w.Event('change', { bubbles: true })); }
  assert.match(text(ui.doc.querySelector('#lsb-sum')), /^Chosen: 3 houses\./);
  click(ui, '[data-fk="b-complete"]');
  await ui.settle();
  const q = ui.doc.querySelector('#lsb-confirm');
  assert.ok(q);
  const t = questionText(ui.doc);
  assert.match(t, /^Complete 3 houses for Atreides\?/);
  for (const n of names) assert.ok(t.includes(n), n + ' is named in: ' + t);
  assert.match(t, /Written at once, with a backup first\.$/);
  assert.equal(writes(ui).length, 0);
  click(ui, '[data-fk="c-yes"]');
  await ui.settle();
  assert.equal(writes(ui).length, 1, 'one request for all of them: all or none');
  assert.deepEqual(writes(ui)[0].body.houses.map((h) => h.task_id).sort(), names.map((n) => idOf(a, n)).sort());
  for (const h of writes(ui)[0].body.houses) assert.equal(h.complete_for, 1);
});

test('several Houses: Houses that would not change are left out and the question says how many, and a replaced win is named', async () => {
  const ui = await open('board');
  const a = synthetic();
  // Vernius is taken by Harkonnen (Complete replaces that win), Tseida is already Atreides's (nothing to change)
  for (const n of ['Vernius', 'Tseida', 'Sor']) { const c = ui.doc.querySelector('.lcc-bd-pick[data-id="' + idOf(a, n) + '"]'); c.checked = true; c.dispatchEvent(new ui.w.Event('change', { bubbles: true })); }
  click(ui, '[data-fk="b-complete"]');
  const t = questionText(ui.doc);
  assert.match(t, /^Complete 2 houses for Atreides\?/);
  assert.match(t, /Replaces the win of Harkonnen on House Vernius\./);
  assert.match(t, /1 other chosen house is left alone: nothing to change for it\./);
  assert.ok(!t.includes('Tseida'), 'a House that does not change is not named as one that does');
});

test('a filter drops the Houses it hides from the choice, and a question about them closes: what is not shown is never written', async () => {
  const ui = await open('board');
  const a = synthetic();
  for (const n of ['Sor', 'Vernius']) { const c = ui.doc.querySelector('.lcc-bd-pick[data-id="' + idOf(a, n) + '"]'); c.checked = true; c.dispatchEvent(new ui.w.Event('change', { bubbles: true })); }
  click(ui, '[data-fk="b-fill"]');
  assert.ok(ui.doc.querySelector('#lsb-confirm'));
  click(ui, '[data-fk="bf:hs:open"]'); // Vernius is taken: the filter hides it
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null, 'the question about a House that is no longer shown is closed');
  assert.match(text(ui.doc.querySelector('#lsb-sum')), /^Chosen: 1 house\./);
  assert.equal(ui.doc.querySelector('.lcc-bd-pick[data-id="' + idOf(a, 'Vernius') + '"]').checked, false);
  assert.equal(writes(ui).length, 0);
});

test('Choose all houses chooses what the board shows, not what a filter dims; Clear the choice clears the boxes', async () => {
  const ui = await open('board');
  click(ui, '[data-fk="bf:hs:open"]');
  click(ui, '[data-fk="b-all"]');
  const boxes = [...ui.doc.querySelectorAll('.lcc-bd-pick')];
  assert.equal(boxes.filter((b) => b.checked).length, 9, 'the nine open Houses');
  assert.match(text(ui.doc.querySelector('#lsb-sum')), /^Chosen: 9 houses\./);
  click(ui, '[data-fk="b-none"]');
  assert.equal([...ui.doc.querySelectorAll('.lcc-bd-pick')].filter((b) => b.checked).length, 0);
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), '');
});

test('the grant before the click: the Inspector says what a raise would write, and a raise over the limit is shown as refused', async () => {
  const a = synthetic();
  const ui = await open('board', a);
  click(ui, ui.doc.querySelector('[data-fk="ht:' + idOf(a, 'Lindaren') + '"]'));
  const t = text(ui.doc.querySelector('.lcc-bd-insp'));
  // the caption under the ledger is gone (#980): what a click grants is in the tooltip of the button and of the tier box, before the click
  assert.match(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-act[data-do="fill"]').getAttribute('title'), /Writes 6 missions: \+36,000 Solari, \+6,000 House Scrip/);
  assert.doesNotMatch(t, /Written at once\. The pay-out of a mission|has not been checked in the game yet/);
  assert.equal(ui.doc.querySelector('.lcc-bd-insp .lcc-badge-unverified'), null, 'no badge in the Actions block (#948)');
  assert.equal(ui.doc.querySelector('.lcc-bd-insp #lcc-bd-s4').getAttribute('title'), null, 'no heading tooltip: the facts text is in the Help tab (#980)');
  const b = synthetic();
  b.grant.max_missions = 3;
  const over = await open('board', b);
  click(over, over.doc.querySelector('[data-fk="ht:' + idOf(b, 'Lindaren') + '"]'));
  // the reason is said before the click, in the tooltip of the button and of the tier boxes, and each tier row says it will be refused (the caption line under the ledger is gone, #980)
  assert.match(over.doc.querySelector('.lcc-bd-insp .lcc-bd-act[data-do="fill"]').getAttribute('title'), /Will be refused: House Lindaren: this click stands for 6 missions, more than the 3 one change may grant/);
  assert.match(text(over.doc.querySelector('.lcc-bd-insp')), /will be refused/);
});

test('the actions block has the approved layout: Faction and Fill 5 tiers, the House actions with Reopen marked red, Set tier and Apply tier', async () => {
  const ui = await open('board');
  const a = synthetic();
  select_(ui, a, 'Kenola');
  const ed = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-editor');
  const rows = [...ed.querySelectorAll('.lcc-bd-erow')];
  assert.equal(rows.length, 2);
  assert.deepEqual([...rows[0].querySelectorAll('label, .lcc-bd-act')].map((x) => text(x).replace(/ (Atreides|Harkonnen).*$/, '')), ['Faction', 'Fill 5 tiers', 'Fill faction goal', 'Complete House', 'Reopen']);
  assert.deepEqual([...rows[0].querySelectorAll('.lcc-bd-act')].map((x) => x.dataset.do), ['fill', 'fillgoal', 'complete', 'reopen']);
  assert.ok(rows[0].querySelector('.lcc-bd-act[data-do="reopen"]').classList.contains('lcc-bd-act-danger'));
  assert.deepEqual([...rows[1].querySelectorAll('label, button')].map((x) => text(x).replace(/ 0.*$/, '')), ['Set tier', 'Apply tier']);
  assert.deepEqual([...ed.querySelectorAll('#lcc-bd-tierset option')].map((o) => text(o)), ['0 — none', '1 / 5', '2 / 5', '3 / 5', '4 / 5', '5 / 5']);
  // #948, #980: no badge, no note, no tooltip and no screen-reader sentence in the block: the facts it carried are in the Help tab
  assert.equal(ed.querySelector('.lcc-badge, .lcc-badge-unverified'), null);
  assert.equal(ed.querySelector('#lcc-bd-s4d'), null);
  assert.equal(ed.getAttribute('aria-describedby'), null);
  assert.deepEqual([...ed.querySelectorAll('p')].map((p) => p.className), [], 'no paragraph is left in the block');
  assert.doesNotMatch(text(ed), /written at once with a backup first|Each click is written at once/);
  assert.equal(ed.querySelector('#lcc-bd-fac').value, '1', 'it starts on the faction of the character');
});

test('the faction box of the actions drives the board\'s own faction box and writes nothing', async () => {
  const ui = await open('board');
  const a = synthetic();
  select_(ui, a, 'Lindaren');
  select(ui, '.lcc-bd-insp #lcc-bd-fac', 2);
  await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-fac').value, '2');
  assert.equal(ui.doc.querySelector('.lcc-bd-insp #lcc-bd-fac').value, '2');
  assert.equal(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-act[data-do="fill"]').getAttribute('aria-disabled'), 'true');
  assert.equal(writes(ui).length, 0);
  assert.equal(ui.posted.length, 0);
});

test('Apply tier is the click of the numbered box: same request and same words for a raise and for the removal of the highest tier', async () => {
  for (const [label, tierPick, n] of [['raise', 3, 3], ['removal', 1, 0]]) {
    const a = synthetic();
    const viaBox = await open('board');
    click(viaBox, control.board.tier(viaBox, a, 'Kenola', tierPick));
    await viaBox.settle();
    const box = { sent: writes(viaBox).map((w) => w.body), said: same(result(viaBox)) };
    closeAll();
    const viaApply = await open('board');
    select_(viaApply, a, 'Kenola');
    select(viaApply, '#lcc-bd-tierset', n);
    click(viaApply, '.lcc-bd-insp [data-lcb="settier"]');
    await viaApply.settle();
    const apply = { sent: writes(viaApply).map((w) => w.body), said: same(result(viaApply)) };
    closeAll();
    assert.equal(box.sent.length, 1, label);
    assert.deepEqual(apply.sent, box.sent, label + ': the same request');
    assert.equal(apply.said, box.said, label + ': the same words');
  }
});

test('Apply tier changes nothing it cannot do as one click: the same tier, a drop of several tiers at once, and another faction chosen', async () => {
  const ui = await open('board');
  const a = synthetic();
  select_(ui, a, 'Wallach'); // three tiers reached
  const say = () => toastText(ui); // a reason is a toast of the page, not a line kept in the panel (#943)
  assert.equal(ui.doc.querySelector('#lcc-bd-eset'), null, 'no status line is kept in the panel');
  select(ui, '#lcc-bd-tierset', 3);
  click(ui, '.lcc-bd-insp [data-lcb="settier"]');
  assert.match(say(), /already the highest reached tier/);
  select(ui, '#lcc-bd-tierset', 0);
  click(ui, '.lcc-bd-insp [data-lcb="settier"]');
  assert.match(say(), /one at a time, the highest first: choose 2 to remove tier 3/);
  await ui.settle();
  assert.equal(writes(ui).length, 0);
  select(ui, '#lcc-bd-fac', 2);
  await ui.settle();
  select(ui, '#lcc-bd-tierset', 5);
  click(ui, '.lcc-bd-insp [data-lcb="settier"]');
  await ui.settle();
  assert.equal(writes(ui).length, 0, 'with another faction chosen the tier boxes are off, so is Apply tier');
  assert.match(result(ui), /Off while Harkonnen is chosen/);
});

// #881: a card the filter dims cannot be chosen for the actions on several Houses, and the write never acts on a House the board does not show.
const tick = (ui, id) => { const c = ui.doc.querySelector('.lcc-bd-pick[data-id="' + id + '"]'); c.checked = true; c.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };

test('a dimmed card has its choose box off, says why, and a box turned on by a script chooses nothing (#881)', async () => {
  const ui = await open('board');
  const a = synthetic();
  click(ui, '[data-fk="bf:hs:open"]'); // Vernius is taken: its card is dimmed
  const vid = idOf(a, 'Vernius'), box = () => ui.doc.querySelector('.lcc-bd-pick[data-id="' + vid + '"]');
  assert.equal(box().closest('.lcc-hc').getAttribute('data-dim'), 'true');
  assert.equal(box().disabled, true, 'the box of a dimmed card is off');
  assert.match(box().getAttribute('aria-label'), /not in the current filter/);
  assert.match(box().closest('label').getAttribute('title'), /cannot be chosen/);
  assert.equal(ui.doc.querySelector('.lcc-bd-pick[data-id="' + idOf(a, 'Sor') + '"]').disabled, false, 'a card the filter shows keeps its box on');
  box().disabled = false; // a script (or a browser) that turns the box on and ticks it
  tick(ui, vid);
  assert.equal(box().checked, false, 'the tick is taken back');
  assert.equal(ui.w.LCC_HOST.picks().has(vid), false);
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), '');
  click(ui, '[data-fk="b-fill"]');
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.equal(writes(ui).length, 0);
});

test('a House in the choice that the board no longer shows is dropped when a bulk button is pressed: it is not in the question or in the request (#881)', async () => {
  const ui = await open('board');
  const a = synthetic();
  const sor = idOf(a, 'Sor'), ken = idOf(a, 'Kenola'), ver = idOf(a, 'Vernius');
  tick(ui, sor); tick(ui, ken);
  ui.w.LCC_HOST.picks().add(ver); // a stale choice: the filter below does not tell the page's choice list
  ui.w.LCCB.state.hs = 'open';
  click(ui, '[data-fk="b-fill"]');
  const q = questionText(ui.doc);
  assert.match(q, /^Fill 2 houses?/);
  assert.ok(q.includes('Sor') && q.includes('Kenola') && !q.includes('Vernius'), q);
  click(ui, '[data-fk="c-yes"]');
  await ui.settle();
  assert.equal(writes(ui).length, 1);
  assert.deepEqual(writes(ui)[0].body.houses.map((h) => h.task_id).sort(), [sor, ken].sort());
});

test('with only House(s) the board does not show chosen, a bulk button writes nothing and asks nothing (#881)', async () => {
  const ui = await open('board');
  const a = synthetic();
  ui.w.LCC_HOST.picks().add(idOf(a, 'Vernius'));
  ui.w.LCCB.state.hs = 'open';
  click(ui, '[data-fk="b-complete"]');
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.match(toastText(ui), /not in the current filter/);
  assert.equal(writes(ui).length, 0);
  assert.equal(ui.w.LCC_HOST.picks().size, 0);
});

test('Yes does not write a House that the board stopped showing after the question was asked, even when the choice still holds it (#881)', async () => {
  const ui = await open('board');
  const a = synthetic();
  const sor = idOf(a, 'Sor'), ver = idOf(a, 'Vernius');
  tick(ui, sor); tick(ui, ver);
  click(ui, '[data-fk="b-complete"]');
  assert.match(questionText(ui.doc), /^Complete 2 houses/);
  ui.w.LCCB.state.hs = 'open'; // the filter changed without the page being told: Vernius is not shown now
  click(ui, '[data-fk="c-yes"]');
  await ui.settle();
  const sent = writes(ui).flatMap((w) => w.body.houses.map((h) => h.task_id));
  assert.ok(!sent.includes(ver), 'the House the board does not show is not written: ' + sent);
  // #1019: not a pass by sending nothing: the first Yes only asks again (count as it is now) and writes nothing; the second Yes writes the House that is shown
  assert.equal(sent.length, 0, 'the first Yes writes nothing: ' + sent);
  assert.match(toastText(ui), /The houses changed since the question was asked/);
  assert.match(questionText(ui.doc), /^Complete 1 house/);
  click(ui, '[data-fk="c-yes"]');
  await ui.settle();
  const sent2 = writes(ui).flatMap((w) => w.body.houses.map((h) => h.task_id));
  assert.deepEqual(sent2, [sor], 'the second Yes writes the House that is shown, and only it');
});

// #1019: after a write and the re-read the choice is pruned against what the board shows, so "N chosen" and the action bar never count a dimmed House.
test('after a re-read the choice drops a House the board does not show, and the count follows (#1019)', async () => {
  const ui = await open('board');
  const a = synthetic();
  const sor = idOf(a, 'Sor'), ver = idOf(a, 'Vernius');
  tick(ui, sor); tick(ui, ver);
  assert.equal(ui.w.LCC_HOST.picks().size, 2);
  ui.w.LCCB.state.hs = 'open'; // the filter changed without the page being told (what a write that takes Vernius out of the open Houses amounts to)
  await ui.w.landsraadView(); await ui.settle();
  assert.deepEqual([...ui.w.LCC_HOST.picks()], [sor], 'Vernius is not shown now, so it is not chosen');
  assert.equal(text(ui.doc.querySelector('.lcc-k-cmd [data-lk="n"]')), '1', 'the count of the action bar follows (the old phone bar is gone, tau #974)');
});
