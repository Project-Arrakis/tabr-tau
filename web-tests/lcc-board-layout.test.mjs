// Display remediation B, in the page's own code (jsdom): the whole action bar (choose, act, faction, count) is drawn in the control bar above the 5 by 5 (tau #914), the question and the result line stand
// directly under it, and all of it survives a redraw of the board, the House Intelligence panel keeps its
// scroll position and its opened notes over a redraw, the Actions block is first, the question about one House is drawn in the panel and a copy of the result line stands next to the buttons, the
// notes are disclosures, and the bar of the chosen houses for a phone follows the choice and presses the action bar's own buttons. What the controls send is in lcc-board-requests.test.mjs.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

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

test('the whole action bar is in the control bar above the 5 by 5: the choose buttons in the line of Jump to House, the actions, the faction and the count on the next line, the question and the result line under the bar (#914)', async () => {
  const ui = await open();
  const grid = ui.doc.querySelector('.lcc-bd-grid');
  assert.deepEqual([...grid.children].map((c) => c.className.split(' ')[0]), ['lcc-bd-surface', 'lcc-bd-side'], 'nothing is drawn between the board panel and the side column any more');
  const bar = ui.doc.querySelector('.lcc-bd-filters');
  assert.equal(bar.getAttribute('aria-label'), 'Filter and look at the board', 'the control bar keeps its name');
  // choose: in the line of Jump to House, one group with it
  const line = bar.querySelector('.lcc-bd-jumpline');
  assert.ok(line && line.querySelector('#lcc-bd-jump'), 'Jump to House is in the line');
  const choose = line.querySelector('[data-lcc-slot="choose"] .barpart[data-bar="choose"]');
  assert.ok(choose, 'the choose buttons are in the same group as Jump to House');
  assert.deepEqual([...choose.querySelectorAll('button')].map((b) => [text(b), b.dataset.lb, b.dataset.fk]), [['Choose all houses', 'pickall', 'b-all'], ['Clear the choice', 'pickno', 'b-none']], 'the same data attributes as before');
  assert.equal(choose.getAttribute('role'), 'group');
  // act: Fill, Complete, Reopen, the faction with its note, the count
  const act = bar.querySelector('[data-lcc-slot="act"] .barpart[data-bar="act"]');
  assert.ok(act, 'the actions are in the control bar');
  assert.deepEqual([...act.querySelectorAll('button')].map((b) => [text(b), b.dataset.lb, b.dataset.do, b.dataset.fk]), [['Fill chosen', 'bulk', 'fill', 'b-fill'], ['Complete chosen', 'bulk', 'complete', 'b-complete'], ['Complete and fill chosen', 'bulk', 'cfill', 'b-cfill'], ['Reopen chosen', 'bulk', 'reopen', 'b-reopen'], ['Unfill chosen', 'bulk', 'unfill', 'b-unfill']]);
  assert.equal(text(act.querySelector('label[for="lsb-fac"]')), 'Faction for Fill and Complete');
  const fac = act.querySelector('select#lsb-fac');
  assert.ok(fac && fac.dataset.lb === 'fac' && fac.dataset.fk === 'fac');
  assert.equal(text(fac.options[fac.selectedIndex]), 'Atreides (your faction)');
  assert.equal(act.querySelector('.note'), null, 'no standing note beside the faction box (#1045): the option says (your faction)');
  assert.equal(fac.getAttribute('title'), 'The faction of the next Fill faction goal, Complete or bulk click. Choosing it writes nothing.', 'the hover text stays, on the box');
  const sum = act.querySelector('#lsb-sum');
  assert.equal(sum.getAttribute('aria-live'), 'polite', 'the count of the chosen houses stays in a live region');
  assert.equal(text(sum), 'Nothing is chosen.');
  // the question, the result line and the note on a write: directly under the control bar, before the cards
  const slot = ui.doc.querySelector('.lcc-bd-panel > .lcc-bd-slot');
  assert.ok(slot && bar.nextElementSibling === slot, 'directly under the bar');
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
  // the focusable controls come in the order they are drawn: the filters and the jump, the choose buttons, the actions and the faction, then the cards, then the panel
  look(ui, synthetic(), 'Wallach');
  const names = ['#lcc-bd-jump', '[data-fk="b-all"]', '[data-fk="b-none"]', '[data-fk="b-fill"]', '[data-fk="b-complete"]', '[data-fk="b-reopen"]', '#lsb-fac', '.lcc-bd-pick', '.lcc-bd-insp button'];
  const nodes = names.map((n) => ui.doc.querySelector(n));
  assert.ok(nodes.every(Boolean));
  for (let i = 1; i < nodes.length; i++) assert.ok(nodes[i - 1].compareDocumentPosition(nodes[i]) & ui.w.Node.DOCUMENT_POSITION_FOLLOWING, names[i - 1] + ' comes before ' + names[i]);
  const lcc = ui.doc.querySelector('section.lcc-bd');
  assert.equal(lcc.getAttribute('aria-labelledby'), 'lcc-bd-h', 'the landmark of the board keeps its name');
});

test('the classic list keeps its action bar as it was (one bar, the About sections included)', async () => {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, local: { tab: 'landsraad', 'lsb.view': 'classic' } });
  await ui.settle();
  const bar = ui.doc.querySelector('#lsb-board > .actbar');
  assert.ok(bar, 'the bar is in the classic board');
  assert.deepEqual([...bar.querySelectorAll('button')].map((b) => b.dataset.fk), ['b-all', 'b-none', 'b-fill', 'b-complete', 'b-cfill', 'b-reopen', 'b-unfill']);
  assert.ok(bar.querySelector('#lsb-fac') && bar.querySelector('#lsb-sum'));
  assert.equal(bar.querySelector('#lsb-legend'), null);
  assert.deepEqual([...bar.querySelectorAll('details.about > summary')].map((s) => text(s)), ['About tiers', 'About Fill and Complete']);
  assert.equal(bar.querySelector('[data-bar]'), null, 'it is not drawn in parts');
  assert.equal(bar.hidden, false, 'the classic bar is shown');
  assert.equal(bar.getAttribute('role'), 'group');
  assert.equal(bar.getAttribute('aria-label'), 'Actions on the board', 'and keeps its name');
});

test('each control of the action bar in the control bar sends the request the classic list sends for the same choice', async () => {
  const a = synthetic();
  const namesOf = (kind) => (kind === 'reopen' ? ['Hurata', 'Vernius'] : ['Sor', 'Lindaren', 'Kenola']);
  const run = async (view, kind, faction) => {
    const ui = view === 'classic'
      ? await boot({ get: { '/api/landsraad': a }, post: { '/api/landsraad/apply': OK }, local: { tab: 'landsraad', 'lsb.view': 'classic' } })
      : await open(a);
    await ui.settle();
    for (const n of namesOf(kind)) { const c = ui.doc.querySelector((view === 'classic' ? '.pick' : '.lcc-bd-pick') + '[data-id="' + idOf(a, n) + '"]'); c.checked = true; c.dispatchEvent(new ui.w.Event('change', { bubbles: true })); }
    if (faction !== undefined) { const f = ui.doc.querySelector('#lsb-fac'); f.value = String(faction); f.dispatchEvent(new ui.w.Event('change', { bubbles: true })); }
    click(ui, view === 'classic' ? '#lsb-board > .actbar [data-fk="b-' + kind + '"]' : '.lcc-bd-filters [data-fk="b-' + kind + '"]');
    await ui.settle();
    assert.ok(ui.doc.querySelector('#lsb-confirm'), view + ' ' + kind + ' ' + faction + ': the question is asked before a write');
    assert.equal(ui.posted.length, 0, view + ': nothing is written before the answer');
    const q = text(ui.doc.querySelector('#lsb-confirm b'));
    click(ui, '#lsb-confirm [data-fk="c-yes"]');
    await ui.settle();
    const out = { q, posted: ui.posted.map((p) => ({ path: p.path, body: p.body })) };
    closeAll();
    return out;
  };
  const other = a.factions.find((f) => f.id !== a.character.faction_id).id;
  for (const [kind, faction] of [['fill'], ['complete'], ['reopen'], ['complete', other]]) {
    const board = await run('board', kind, faction), classic = await run('classic', kind, faction);
    assert.ok(board.posted.length >= 1, kind + ': a request is sent');
    assert.deepEqual(board.posted, classic.posted, kind + (faction !== undefined ? ' for another faction' : '') + ': the same request');
    assert.equal(board.q.split('?')[0], classic.q.split('?')[0], 'the question names the same action');
  }
  // choose all and clear the choice
  const ui = await open(a);
  click(ui, '.lcc-bd-filters [data-fk="b-all"]');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-pick:checked').length, 25, 'Choose all houses chooses the houses shown');
  assert.match(text(ui.doc.querySelector('#lsb-sum')), /^Chosen: 25 houses\./);
  click(ui, '.lcc-bd-filters [data-fk="b-none"]');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-pick:checked').length, 0);
  assert.equal(text(ui.doc.querySelector('#lsb-sum')), 'Nothing is chosen.');
  assert.equal(ui.posted.length, 0, 'choosing writes nothing');
});

test('the bar and what is chosen survive a redraw of the board (a select, a filter, a layout switch)', async () => {
  const a = synthetic();
  const ui = await open(a);
  pick(ui, a, ['Sor', 'Kenola']);
  const parts = ['choose', 'act', 'ask'].map((k) => ui.doc.querySelector('[data-bar="' + k + '"]'));
  assert.ok(parts.every(Boolean));
  click(ui, '[data-fk="bf:hs:open"]');
  look(ui, a, 'Wallach');
  click(ui, '[data-fk="bf:layout:list"]');
  click(ui, '[data-fk="bf:layout:grid"]');
  for (const k of ['choose', 'act', 'ask']) assert.equal(ui.doc.querySelectorAll('[data-bar="' + k + '"]').length, 1);
  assert.deepEqual(['choose', 'act', 'ask'].map((k) => ui.doc.querySelector('[data-bar="' + k + '"]').parentNode.getAttribute('data-lcc-slot')), ['choose', 'act', 'bar'], 'each part is in its own place of the control bar');
  assert.deepEqual(['choose', 'act', 'ask'].map((k) => ui.doc.querySelector('[data-bar="' + k + '"]')), parts, 'the same elements: nothing in them was drawn again');
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
  assert.match(text(q.querySelector('b')), /^Complete House Vernius for Atreides\? Replaces the win of Harkonnen on House Vernius\./);
  assert.equal(ui.doc.activeElement.getAttribute('data-fk'), 'c-no', 'the safe answer has the focus');
  look(ui, a, 'Kenola'); // the panel is on another House: the question is not about it
  const q2 = ui.doc.querySelector('#lsb-confirm');
  assert.ok(q2 && q2.closest('[data-bar="ask"]') && q2.closest('.lcc-bd-slot'), 'it is back under the control bar and still there to answer');
  click(ui, '[data-fk="c-no"]');
  assert.equal(ui.doc.querySelector('#lsb-confirm'), null);
  assert.equal(ui.doc.querySelector('#lcc-bd-eres'), null, 'no copy of a result is kept in the panel (#943)');
  assert.match(text([...ui.doc.querySelectorAll('#toast .d-toast')].pop()), /^Cancelled\. Nothing was written\./, 'the answer to the question is a toast');
  // several houses: the question stays in the bar
  pick(ui, a, ['Sor', 'Lindaren']);
  click(ui, '[data-fk="b-complete"]');
  await ui.settle();
  assert.ok(ui.doc.querySelector('#lsb-confirm').closest('.lcc-bd-slot'), 'a question about several Houses is the action bar\'s, drawn under the control bar');
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

test('the bar of the chosen houses for a phone follows the choice and presses the action bar\'s own buttons', async () => {
  const a = synthetic();
  const ui = await open(a);
  const bar = ui.doc.querySelector('#lsb-phonebar');
  assert.ok(bar.hidden, 'nothing is chosen: no bar');
  assert.equal(bar.getAttribute('aria-label'), 'The chosen houses');
  assert.deepEqual([...bar.querySelectorAll('button')].map((b) => text(b)), ['Fill chosen', 'Complete chosen', 'Complete and fill chosen', 'Reopen chosen', 'Unfill chosen']);
  pick(ui, a, ['Sor', 'Lindaren']);
  assert.equal(bar.hidden, false);
  assert.equal(text(bar.querySelector('.pn')), '2 chosen');
  click(ui, '[data-fk="b-none"]');
  assert.ok(bar.hidden);
  assert.equal(text(bar.querySelector('.pn')), '0 chosen');
  pick(ui, a, ['Sor', 'Lindaren', 'Kenola']);
  const pressed = [];
  for (const b of ui.doc.querySelectorAll('.lcc-bd-filters [data-lb="bulk"]')) b.addEventListener('click', () => pressed.push(b.dataset.do));
  click(ui, '[data-pb="complete"]');
  await ui.settle();
  assert.deepEqual(pressed, ['complete'], 'a press of the phone bar is a press of the action bar\'s button');
  assert.match(text(ui.doc.querySelector('#lsb-confirm b')), /^Complete 3 houses for Atreides\?/);
  assert.equal(ui.posted.length, 0, 'nothing is written before the answer');
  // a phone-bar button holds no write of its own
  assert.equal(ui.doc.querySelectorAll('#lsb-phonebar [data-lb], #lsb-phonebar [data-fk^="c-"]').length, 0);
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
