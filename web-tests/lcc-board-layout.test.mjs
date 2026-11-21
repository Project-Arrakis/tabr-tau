// Display remediation B, in the page's own code (jsdom): the whole action bar (choose, act, faction, count) is drawn in the control bar above the 5 by 5 (tau #914), the question and the result line stand
// directly under it, and all of it survives a redraw of the board, the House Intelligence panel keeps its
// scroll position and its opened notes over a redraw, the Actions block is first, the question about one House is drawn in the panel and a copy of the result line stands next to the buttons, the
// notes are disclosures, and the bar of the chosen houses for a phone follows the choice and presses the action bar's own buttons. What the controls send is in lcc-board-requests.test.mjs.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { boot, closeAll } from './boot.mjs';
import { synthetic, questionText } from './lcc-fixture.mjs';

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

test('the whole action bar is in the sticky strip above the 5 by 5 (tau #974, was #914): the filters, the count, crests, keys and Clear in the action bar, the question and the result line under the strip', async () => {
  const ui = await open();
  const grid = ui.doc.querySelector('.lcc-bd-grid');
  assert.deepEqual([...grid.children].map((c) => c.className.split(' ')[0]), ['lcc-bd-surface', 'lcc-bd-side'], 'nothing is drawn between the board panel and the side column any more');
  const bar = ui.doc.querySelector('.lcc-k-strip');
  assert.ok(bar, 'the sticky strip is the control bar');
  // no choose key in the filter row of the strip (#1116): choosing the shown Houses is All in the Filter menu
  assert.equal(bar.querySelector('[data-bar="choose"], [data-lcc-slot="choose"]'), null, 'no choose part and no choose slot');
  // act: the action bar holds the count, the two crests, Fill, Complete, Reopen (and under More the other two), Clear, the faction box behind the crests, the count sentence for a screen reader
  const act = bar.querySelector('.lcc-k-cmd [data-lcc-slot="act"] .barpart[data-bar="act"]');
  assert.ok(act, 'the actions are in the action bar');
  assert.deepEqual([...act.querySelectorAll('[data-lb="bulk"]')].map((b) => [b.dataset.lb, b.dataset.do, b.dataset.fk]), [['bulk', 'fill', 'b-fill'], ['bulk', 'complete', 'b-complete'], ['bulk', 'reopen', 'b-reopen'], ['bulk', 'cfill', 'b-cfill'], ['bulk', 'unfill', 'b-unfill']]);
  assert.deepEqual([...act.querySelectorAll('.lcc-k-bl')].map((b) => text(b)), ['Fill', 'Complete', 'Reopen', 'More']);
  const clear = act.querySelector('[data-lb="pickno"]');
  assert.ok(clear && clear.dataset.fk === 'b-none', 'Clear is the Clear the choice key (data-fk b-none)');
  assert.deepEqual([...act.querySelectorAll('.lcc-k-seats [role="radio"]')].map((b) => [b.dataset.lk, b.getAttribute('aria-checked')]), [['seat', 'true'], ['seat', 'false']], 'two crests, the character\'s faction is on');
  const fac = act.querySelector('select#lsb-fac');
  assert.ok(fac && fac.dataset.lb === 'fac' && fac.dataset.fk === 'fac', 'the faction box stays behind the crests');
  assert.equal(text(fac.options[fac.selectedIndex]), 'Atreides (your faction)');
  assert.equal(act.querySelector('.note'), null, 'no standing note beside the faction (#1045)');
  const sum = act.querySelector('#lsb-sum');
  assert.equal(sum.getAttribute('aria-live'), 'polite', 'the count of the chosen houses stays in a live region');
  assert.equal(text(sum), '', 'no standing sentence when nothing is chosen (#1062)');
  // the question, the result line and the note on a write: directly under the strip, before the cards
  const slot = ui.doc.querySelector('.lcc-bd-panel > .lcc-bd-slot');
  assert.ok(slot && bar.nextElementSibling === slot, 'directly under the strip');
  assert.equal(slot.querySelector('#lsb-result'), null, 'the board keeps no result line: a result is a toast of the page (#943)');
  assert.equal(slot.querySelector('#lsb-legend'), null, 'the standing line about writes is gone (#980): the question before a write and the Help tab say it');
  assert.ok(slot.compareDocumentPosition(ui.doc.querySelector('.lcc-bd-stage')) & ui.w.Node.DOCUMENT_POSITION_FOLLOWING, 'before the cards');
  // only the collapsed About sections of the bar are dropped
  assert.equal(ui.doc.querySelectorAll('#lsb-board details.about').length, 0, 'no About section of the action bar is drawn on the board');
  const home = ui.doc.querySelector('#lsb-board > .actbar');
  assert.equal(home.childElementCount, 0, 'nothing is left in a bar under the board');
  assert.equal(home.hidden, true, 'and the empty bar is not shown (no box, no named group for a screen reader)');
  assert.equal(home.getAttribute('role'), null, 'the empty wrapper is not a named group');
  assert.equal(home.getAttribute('aria-label'), null);
  assert.ok(/\.lsb \.actbar\[hidden\]\s*\{\s*display:\s*none/.test(fs.readFileSync(new URL('../internal/web/static/app.css', import.meta.url), 'utf8')), 'the hidden bar is display: none in the stylesheet');
  // the focusable controls come in the order they are drawn: the Show tabs, the Filter key, Find House, the action bar (crest, Fill, Complete, Reopen, Clear, faction), then the cards, then the panel
  look(ui, synthetic(), 'Wallach');
  const names = ['[data-fk="bf:show:all"]', '[data-lk="fbtn"]', '[data-lk="findbtn"]', '[data-lk="seat"]', '[data-fk="b-fill"]', '[data-fk="b-complete"]', '[data-fk="b-reopen"]', '[data-fk="b-none"]', '#lsb-fac', '.lcc-bd-pick', '.lcc-bd-insp button'];
  const nodes = names.map((n) => ui.doc.querySelector(n));
  assert.ok(nodes.every(Boolean));
  for (let i = 1; i < nodes.length; i++) assert.ok(nodes[i - 1].compareDocumentPosition(nodes[i]) & ui.w.Node.DOCUMENT_POSITION_FOLLOWING, names[i - 1] + ' comes before ' + names[i]);
  const lcc = ui.doc.querySelector('section.lcc-bd');
  assert.equal(lcc.getAttribute('aria-labelledby'), 'lcc-bd-h', 'the landmark of the board keeps its name');
});

test('each control of the action bar in the control bar asks first and then sends one request for the chosen Houses, for the faction chosen', async () => {
  // the exact bodies of these presses are the recorded ones of lcc-board-requests.test.mjs (golden file, recorded from the retired Classic list, #1028); this checks the control bar's own keys and crests
  const a = synthetic();
  const namesOf = (kind) => (kind === 'reopen' ? ['Hurata', 'Vernius'] : ['Sor', 'Lindaren', 'Kenola']);
  const run = async (kind, faction) => {
    const ui = await open(a);
    pick(ui, a, namesOf(kind));
    if (faction !== undefined) click(ui, '.lcc-k-cmd [data-lk="seat"][data-fid="' + faction + '"]'); // the board's faction is the crest (the box behind it follows)
    click(ui, '.lcc-k-cmd [data-fk="b-' + kind + '"]');
    await ui.settle();
    assert.ok(ui.doc.querySelector('#lsb-confirm'), kind + ' ' + faction + ': the question is asked before a write');
    assert.equal(ui.posted.length, 0, 'nothing is written before the answer');
    const q = questionText(ui.doc);
    click(ui, '#lsb-confirm [data-fk="c-yes"]');
    await ui.settle();
    const out = { q, posted: ui.posted.map((p) => ({ path: p.path, body: p.body })) };
    closeAll();
    return out;
  };
  const other = a.factions.find((f) => f.id !== a.character.faction_id).id;
  for (const [kind, faction] of [['fill'], ['complete'], ['reopen'], ['complete', other]]) {
    const r = await run(kind, faction);
    const label = kind + (faction !== undefined ? ' for another faction' : '');
    assert.equal(r.posted.length, 1, label + ': one request');
    assert.equal(r.posted[0].path, '/api/landsraad/apply');
    assert.equal(r.posted[0].body.term_id, 7, label + ': the term of the read, for the stale guard');
    assert.deepEqual(r.posted[0].body.houses.map((h) => h.task_id).sort(), namesOf(kind).map((n) => idOf(a, n)).sort(), label + ': the chosen Houses, all in one request');
    if (kind === 'complete') for (const h of r.posted[0].body.houses) assert.equal(h.complete_for, faction === undefined ? a.character.faction_id : faction, label + ': completed for the faction chosen');
    assert.match(r.q, kind === 'fill' ? /^Fill / : kind === 'complete' ? /^Complete / : /^Reopen /, label + ': the question names the action');
  }
  // choose all (All in the Filter menu) and clear the choice
  const ui = await open(a);
  click(ui, '.lcc-k-strip [data-lk="fbtn"]');
  click(ui, '#lcc-bd-menu [data-lk="hall"]');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-pick:checked').length, 25, 'All chooses the houses shown');
  assert.match(text(ui.doc.querySelector('#lsb-sum')), /^Chosen: 25 houses\./);
  click(ui, '.lcc-k-cmd [data-fk="b-none"]');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-pick:checked').length, 0);
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), '');
  assert.equal(ui.posted.length, 0, 'choosing writes nothing');
});

test('the bar and what is chosen survive a redraw of the board (a select, a filter, a layout switch)', async () => {
  const a = synthetic();
  const ui = await open(a);
  pick(ui, a, ['Sor', 'Kenola']);
  const parts = ['act', 'ask'].map((k) => ui.doc.querySelector('[data-bar="' + k + '"]'));
  assert.ok(parts.every(Boolean));
  click(ui, '[data-fk="bf:hs:open"]');
  look(ui, a, 'Wallach');
  click(ui, '[data-fk="bf:layout:list"]');
  click(ui, '[data-fk="bf:layout:grid"]');
  for (const k of ['act', 'ask']) assert.equal(ui.doc.querySelectorAll('[data-bar="' + k + '"]').length, 1);
  assert.deepEqual(['act', 'ask'].map((k) => ui.doc.querySelector('[data-bar="' + k + '"]').parentNode.getAttribute('data-lcc-slot')), ['act', 'askbar'], 'each part is in its own place of the strip');
  assert.deepEqual(['act', 'ask'].map((k) => ui.doc.querySelector('[data-bar="' + k + '"]')), parts, 'the same elements: nothing in them was drawn again');
  assert.match(text(ui.doc.querySelector('#lsb-sum')), /^Chosen: /);
});

test('the Actions block is first under the name and the House panel holds no disclosure or note (#980)', async () => {
  const a = synthetic();
  const ui = await open(a);
  look(ui, a, 'Lindaren');
  const kids = [...ui.doc.querySelector('.lcc-bd-insp').children].map((c) => c.getAttribute('aria-labelledby') || c.className.split(' ')[0]);
  assert.deepEqual(kids.slice(0, 5), ['lcc-bd-insp-top', 'lcc-bd-back', 'lcc-bd-identity', 'lcc-bd-s4', 'lcc-bd-pin']);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-insp details').length, 0, 'no disclosure: How each click is written, About Fill and the tiers, What the marks mean and Important: three separate states are in the Help tab');
  assert.doesNotMatch(text(ui.doc.querySelector('.lcc-bd-insp')), /Each click is written at once|three separate states|What the marks mean|About Fill/);
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
  assert.match(questionText(ui.doc), /^Complete House Vernius for Atreides\? Replaces the win of Harkonnen on House Vernius\./);
  assert.equal(ui.doc.activeElement.getAttribute('data-fk'), 'c-no', 'the safe answer has the focus');
  look(ui, a, 'Kenola'); // the panel is on another House: the question is not about it
  const q2 = ui.doc.querySelector('#lsb-confirm');
  assert.ok(q2 && q2.closest('[data-bar="ask"]') && q2.closest('.lcc-k-cmd'), 'it is back in the action bar and still there to answer');
  click(ui, '[data-fk="c-no"]');
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.equal(ui.doc.querySelector('#lcc-bd-eres'), null, 'no copy of a result is kept in the panel (#943)');
  assert.match(text([...ui.doc.querySelectorAll('#toast .d-toast')].pop()), /^Cancelled\. Nothing was written\./, 'the answer to the question is a toast');
  // several houses: the question stays in the bar
  pick(ui, a, ['Sor', 'Lindaren']);
  click(ui, '[data-fk="b-complete"]');
  await ui.settle();
  assert.ok(ui.doc.querySelector('#lsb-confirm').closest('.lcc-k-cmd'), 'a question about several Houses is the action bar\'s: it uses the whole bar');
  assert.equal(ui.doc.querySelector('.lcc-bd-ask').childElementCount, 0);
});

test('a write on the board says what it did as one toast, one result per line, and keeps no copy of it in the panel (#943)', async () => {
  const a = synthetic();
  const ui = await open(a);
  look(ui, a, 'Kenola');
  click(ui, '.lcc-bd-insp .lcc-bd-tier[data-t="3"]');
  await ui.settle();
  assert.equal(ui.doc.querySelector('#lcc-bd-eres'), null);
  assert.equal(ui.doc.querySelector('#lsb-result'), null);
  const ts = [...ui.doc.querySelectorAll('#toast .d-toast')];
  assert.equal(ts.length, 1, 'one at a time');
  assert.equal(ts[0].getAttribute('role'), 'status');
  assert.equal(ts[0].querySelector('span').textContent.split('\n')[0], 'House Kenola: your contribution 1,200 → 3,600 (tiers 2 to 3 reached).', 'what changed is the first line');
  assert.ok(ts[0].querySelector('span').textContent.split('\n').length >= 2, 'what was not written, and the notes, are lines of their own');
  assert.doesNotMatch(ts[0].textContent, /Nothing to do/);
  assert.equal(ui.doc.querySelector('#toast').getAttribute('aria-live'), 'polite');
});

test('the action bar of the chosen Houses follows the choice (one bar on every width, tau #974, in place of the old phone bar) and its keys are the page\'s own buttons', async () => {
  const a = synthetic();
  const ui = await open(a);
  const p = ui.doc.querySelector('.lcc-bd-panel'), n = () => text(p.querySelector('.lcc-k-cmd [data-lk="n"]'));
  assert.equal(ui.doc.querySelector('#lsb-phonebar'), null, 'the old phone bar is gone');
  assert.equal(p.dataset.mode, 'filters', 'nothing is chosen: the filter row, no action bar');
  assert.equal(p.querySelector('.lcc-k-cmd').getAttribute('aria-label'), 'Actions on the chosen Houses');
  pick(ui, a, ['Sor', 'Lindaren']);
  assert.equal(p.dataset.mode, 'action');
  assert.equal(n(), '2');
  click(ui, '[data-fk="b-none"]');
  assert.equal(p.dataset.mode, 'filters');
  assert.equal(n(), '0');
  pick(ui, a, ['Sor', 'Lindaren', 'Kenola']);
  const pressed = [];
  for (const b of ui.doc.querySelectorAll('.lcc-k-cmd [data-lb="bulk"]')) b.addEventListener('click', () => pressed.push(b.dataset.do));
  click(ui, '.lcc-k-cmd [data-do="complete"]');
  await ui.settle();
  assert.deepEqual(pressed, ['complete']);
  assert.match(questionText(ui.doc), /^Complete 3 houses for Atreides\?/);
  assert.equal(ui.posted.length, 0, 'nothing is written before the answer');
  // the action bar holds no write of its own: its keys are the page's data-lb buttons and the question's keys are the page's c-yes and c-no
  assert.equal(ui.doc.querySelectorAll('.lcc-k-cmd [data-pb]').length, 0);
  assert.equal(ui.doc.querySelectorAll('.lcc-k-cmd #lsb-confirm [data-fk^="c-"]').length, 2);
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
  assert.equal(ui.doc.querySelector('#lsb-result'), null, 'the board keeps no result line');
  const slot = ui.doc.querySelector('.lcc-bd-slot');
  assert.ok(slot, 'the place under the control bar stays');
});
