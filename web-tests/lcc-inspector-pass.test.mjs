// The Command Center inspector and cards pass (tau #943, #936, #938, #939, #942, #945, #948), in the page's own code (jsdom), with the synthetic answer of M1 and no game save.
// #943: every click on a tier square answers in a toast (one at a time, timed, the Command Center's own words), and the board keeps no result line and no copy of one.
// #936: Not reached / Not collected / Collected in both views. #938: the cards carry the Classic mouse-over text of a tier, built once. #939: the place line and the one-line specialization.
// #942: the four counts are quiet read-only figures. #945: no caption paragraph under the board.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 0, outcome: 'written' };
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
const toasts = (ui) => [...ui.doc.querySelectorAll('#toast .d-toast')];
const last = (ui) => toasts(ui).pop();
async function open(view = 'board', post = { '/api/landsraad/apply': OK }, answer = synthetic()) {
  const ui = await boot({ get: { '/api/landsraad': answer }, post, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': 'grid' } });
  await ui.settle();
  return ui;
}
const look = (ui, a, name) => click(ui, '[data-fk="ht:' + idOf(a, name) + '"]');
const card = (ui, a, name, n) => ui.doc.querySelector('.lcc-hc-t[data-id="' + idOf(a, name) + '"][data-t="' + n + '"]');
const row = (ui, n) => ui.doc.querySelector('.lcc-bd-insp .lcc-bd-tier[data-t="' + n + '"]');
// Thorvald has 6,500 contributed: tier 5 is reached, so tiers 1 to 4 are lower reached tiers that a click cannot remove
const HINT = /^To remove tiers of House Thorvald, choose tier 4 in Set tier and press Apply tier: tiers come off one at a time, the highest first\.$/;

// ---- #943 (A): the silent click -----------------------------------------------------------------------------------------------------------------------------------------
test('#943 A: every click on a lower reached tier square of a card answers with the Command Center\'s words in a toast, every time, one at a time', async () => {
  const a = synthetic();
  const ui = await open();
  assert.equal(toasts(ui).length, 0);
  for (const n of [1, 2, 3, 4, 1, 2]) { // the same square twice, and the others: each click is answered
    click(ui, card(ui, a, 'Thorvald', n));
    assert.equal(toasts(ui).length, 1, 'one at a time, click on tier ' + n);
    assert.match(text(last(ui)), HINT);
  }
  assert.equal(ui.posted.length, 0, 'nothing is written');
  assert.equal(ui.doc.querySelector('#lsb-result'), null, 'and no result line is kept on the page');
  assert.doesNotMatch(text(last(ui)), /click the highest reached tier/, 'the Classic rule is not told on the board');
});

test('#943 A: the same click in the panel of the House, with a faction chosen that is not the character\'s, and after a write, is answered every time', async () => {
  const a = synthetic();
  const ui = await open();
  look(ui, a, 'Thorvald');
  for (const n of [1, 2, 3, 4]) { click(ui, row(ui, n)); assert.match(text(last(ui)), HINT, 'panel row ' + n); }
  assert.equal(toasts(ui).length, 1);
  // another faction chosen: the boxes are off and say why, again and again
  select(ui, '#lsb-fac', 2); await ui.settle();
  for (let i = 0; i < 3; i++) {
    click(ui, card(ui, a, 'Thorvald', 2));
    assert.equal(toasts(ui).length, 1);
    assert.match(text(last(ui)), /^Off while Harkonnen is chosen\. Tiers change your own contribution/);
  }
  click(ui, row(ui, 3));
  assert.match(text(last(ui)), /^Off while Harkonnen is chosen/);
  select(ui, '#lsb-fac', 1); await ui.settle();
  // a write, then a lower square again
  click(ui, row(ui, 5)); await ui.settle(); // removes tier 5 (the highest reached)
  assert.equal(ui.posted.filter((p) => p.path === '/api/landsraad/apply').length, 1);
  click(ui, card(ui, a, 'Hagal', 1));
  assert.match(text(last(ui)), /^To remove tiers of House Hagal, choose tier 4 in Set tier/, 'a click after a write is answered');
  assert.equal(toasts(ui).length, 1, 'the toast of the write is replaced, not stacked');
});

test('#943 A: the Classic list keeps its own wording and its result line', async () => {
  const a = synthetic();
  const ui = await open('classic');
  const r = ui.doc.querySelector('#lsrow-' + idOf(a, 'Thorvald'));
  const x = r.querySelector('.xc'); x.checked = true; x.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  click(ui, r.querySelector('.tc[data-t="2"]'));
  assert.equal(text(ui.doc.querySelector('#lsb-result')), 'To remove tiers of House Thorvald, click the highest reached tier (tier 5).');
  assert.match(r.querySelector('.tc[data-t="2"]').getAttribute('aria-label'), /To remove tiers, click the highest reached tier \(tier 5\)\./);
});

test('#943: the stale wording is gone from the board: the tooltips and names of the lower boxes name Set tier and Apply tier', async () => {
  const a = synthetic();
  const ui = await open();
  look(ui, a, 'Thorvald');
  for (const b of [card(ui, a, 'Thorvald', 2), row(ui, 2)]) {
    assert.ok((b.getAttribute('aria-label') || '').includes('To remove tiers, choose tier 4 in Set tier and press Apply tier'), b.getAttribute('aria-label'));
    assert.ok(b.getAttribute('title').includes('choose tier 4 in Set tier'), b.getAttribute('title'));
  }
  const board = text(ui.doc.querySelector('#lsb'));
  const labels = [...ui.doc.querySelectorAll('#lsb [aria-label], #lsb [title]')].map((e) => (e.getAttribute('aria-label') || '') + ' ' + (e.getAttribute('title') || '')).join(' ');
  assert.doesNotMatch(board + ' ' + labels, /click the highest reached tier/);
});

// ---- #943 (B): the results are toasts -----------------------------------------------------------------------------------------------------------------------------------
test('#943 B: a write result is one toast of lines (changed / not written and why / notes), with no "Nothing to do." after a write that did something', async () => {
  const a = synthetic();
  const ui = await open('board', { '/api/landsraad/apply': { ...OK, rewards_already_given: 1, missions: 0 } });
  look(ui, a, 'Kenola');
  click(ui, row(ui, 3)); await ui.settle();
  const lines = last(ui).querySelector('span').textContent.split('\n');
  assert.equal(lines[0], 'House Kenola: your contribution 1,200 → 3,600 (tiers 2 to 3 reached).');
  assert.match(lines[1], /^No missions written: your own earlier progress on this house already counts for this ground\.$/);
  assert.equal(lines[2], 'Warning: 1 reward was left out: earlier progress on this house already counted for it.');
  assert.equal(lines.length, 3);
  assert.doesNotMatch(last(ui).textContent, /Nothing to do/);
  assert.ok(last(ui).classList.contains('part') && last(ui).querySelector('button.x'), 'a warning is not a plain success: it stays until it is dismissed');
  assert.equal(last(ui).getAttribute('data-lines'), 'true');
});

test('#943 B: a result goes away by itself, a long one stays longer, an error stays until it is dismissed, and a new result takes the place of the last', async () => {
  const a = synthetic();
  const ui = await open();
  const timers = [];
  const real = ui.w.setTimeout.bind(ui.w);
  ui.w.setTimeout = (fn, ms, ...rest) => { if (ms >= 3000) { timers.push({ fn, ms }); return 0; } return real(fn, ms, ...rest); };
  click(ui, card(ui, a, 'Thorvald', 1)); // a hint: a notice that leaves by itself
  const hint = timers.splice(0);
  assert.equal(hint.length, 1);
  assert.ok(hint[0].ms >= 6000, 'a notice stays at least six seconds');
  click(ui, '[data-fk="b-fill"]'); // nothing is chosen
  assert.equal(toasts(ui).length, 1, 'the new one replaced the old');
  assert.match(text(last(ui)), /^Nothing is chosen, so there is nothing to fill\./);
  const short = timers.splice(0)[0].ms;
  look(ui, a, 'Thorvald');
  click(ui, row(ui, 2));
  const long = timers.splice(0)[0].ms;
  assert.ok(long > short, 'the longer text stays longer: ' + long + ' > ' + short);
  assert.ok(long <= 20000);
  hint[0].fn(); // the time of the first is over: it is already gone, so nothing happens to the new one
  assert.equal(toasts(ui).length, 1);
  // an error: a toast with a dismiss control and no timer
  const bad = await open('board', { '/api/landsraad/apply': { __fail: 'House Sor changed since it was shown.', __code: 'conflict', __status: 409 } });
  const t2 = [];
  const real2 = bad.w.setTimeout.bind(bad.w);
  bad.w.setTimeout = (fn, ms, ...rest) => { if (ms >= 3000) { t2.push(ms); return 0; } return real2(fn, ms, ...rest); };
  look(bad, a, 'Sor');
  click(bad, '.lcc-bd-insp .lcc-bd-act[data-do="complete"]'); await bad.settle();
  const e = last(bad);
  assert.ok(e.classList.contains('err') && e.getAttribute('role') === 'alert');
  assert.deepEqual(t2, [], 'no timer takes an error away');
  e.querySelector('button.x').click();
  assert.equal(toasts(bad).length, 0, 'dismissed');
});

test('#943 B: the panel keeps no line of results: no copy, no status line of Set tier; the question that waits for an answer stays', async () => {
  const a = synthetic();
  const ui = await open();
  look(ui, a, 'Kenola');
  assert.equal(ui.doc.querySelector('#lcc-bd-eres'), null);
  assert.equal(ui.doc.querySelector('#lcc-bd-eset'), null);
  assert.equal(ui.doc.querySelector('.lcc-bd-pin #lsb-result'), null);
  select(ui, '#lcc-bd-tierset', 0);
  click(ui, '.lcc-bd-insp [data-lcb="settier"]'); // tier 1 is the highest reached: this is a removal, so it writes
  await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/landsraad/apply').length, 1);
  const src = fs.readFileSync(new URL('../internal/web/static/app.js', import.meta.url), 'utf8');
  assert.doesNotMatch(src, /lbMirror/, 'the mirror of the result line is gone from the page');
  // the confirm question is still a question in the page
  pickTwo(ui, a);
  click(ui, '[data-fk="b-complete"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#lsb-confirm'), 'a question waiting for an answer is not a toast');
});
function pickTwo(ui, a) { for (const n of ['Sor', 'Lindaren']) { const c = ui.doc.querySelector('.lcc-bd-pick[data-id="' + idOf(a, n) + '"]'); c.checked = true; c.dispatchEvent(new ui.w.Event('change', { bubbles: true })); } }

test('#943: the focus of a write that took away its control goes to the heading of the board, not to the top of the page', async () => {
  const a = synthetic();
  const ui = await open();
  pickTwo(ui, a);
  click(ui, '[data-fk="b-complete"]'); await ui.settle();
  click(ui, '#lsb-confirm [data-fk="c-yes"]'); await ui.settle();
  const f = ui.doc.activeElement;
  assert.ok(f && f !== ui.doc.body, 'the focus is somewhere');
});

// ---- #936: the three words ----------------------------------------------------------------------------------------------------------------------------------------------
test('#936: Not reached / Not collected / Collected on the cards, in the panel and in the Classic list, and "To collect" is nowhere', async () => {
  const a = synthetic();
  const ui = await open();
  look(ui, a, 'Wallach');
  const i = ui.doc.querySelector('.lcc-bd-insp');
  const states = [...i.querySelectorAll('.lcc-bd-tier-s')].map(text);
  assert.ok(states.some((s) => s.endsWith('NOT COLLECTED')) && states.some((s) => s.endsWith('NOT REACHED')), states.join('|'));
  assert.deepEqual([...i.querySelectorAll('.lcc-bd-stat span')].map(text), ['Tiers reached', 'Collected', 'Not collected', 'Not reached']);
  const legend = [...i.querySelectorAll('.lcc-bd-legend span')].map(text);
  assert.deepEqual(legend, ['✓ Collected', '◆ Not collected', '○ Not reached', '? No delivery record']);
  const all = text(ui.doc.querySelector('#lsb')) + ' ' + [...ui.doc.querySelectorAll('#lsb [aria-label], #lsb [title]')].map((e) => (e.getAttribute('aria-label') || '') + ' ' + (e.getAttribute('title') || '')).join(' ');
  assert.doesNotMatch(all, /to collect|Waiting at representative|Waiting to collect/i);
  const classic = await open('classic');
  const r = classic.doc.querySelector('#lsrow-' + idOf(a, 'Wallach'));
  const x = r.querySelector('.xc'); x.checked = true; x.dispatchEvent(new classic.w.Event('change', { bubbles: true }));
  assert.deepEqual([...r.querySelectorAll('.tc .pl')].map(text), ['Not collected', 'Not collected', 'Not collected', 'Not reached', 'Not reached']);
  const classicAll = text(classic.doc.querySelector('#lsb')) + ' ' + [...classic.doc.querySelectorAll('#lsb [aria-label], #lsb [title]')].map((e) => (e.getAttribute('aria-label') || '') + ' ' + (e.getAttribute('title') || '')).join(' ');
  assert.doesNotMatch(classicAll, /waiting to collect|Waiting to collect/);
  assert.match(classicAll, /reached, not collected \(waiting at the house representative\)/, 'the longer explanation stays in the screen-reader text');
});

// ---- #938: the Classic mouse-over on the cards -------------------------------------------------------------------------------------------------------------------------
test('#938: every tier square of a card has the Classic list\'s mouse-over text for the same tier, built once, with a name for a screen reader and the focus path', async () => {
  const a = synthetic();
  const ui = await open();
  const classic = await open('classic');
  for (const name of ['Thorvald', 'Wallach', 'Kenola', 'Sor']) {
    const r = classic.doc.querySelector('#lsrow-' + idOf(a, name));
    const x = r.querySelector('.xc'); x.checked = true; x.dispatchEvent(new classic.w.Event('change', { bubbles: true }));
    for (let n = 1; n <= 5; n++) {
      const board = card(ui, a, name, n), old = r.querySelector('.tc[data-t="' + n + '"]');
      // the one difference is the way to remove a lower reached tier: the board names Set tier and Apply tier, the Classic list says to click the highest reached tier
      const cut = (s) => s.split(' To remove tiers, ')[0];
      assert.equal(cut(board.getAttribute('title')), cut(old.getAttribute('title')), name + ' tier ' + n + ': the mouse-over is the Classic text');
      if (old.getAttribute('title').includes('click the highest reached tier')) assert.match(board.getAttribute('title'), /To remove tiers, choose tier \d in Set tier and press Apply tier/);
      else assert.equal(board.getAttribute('title'), old.getAttribute('title'));
      assert.ok(board.getAttribute('title').length > 20);
      assert.ok(board.getAttribute('aria-label').startsWith('House ' + name + ', tier ' + n), board.getAttribute('aria-label'));
      assert.ok(board.getAttribute('data-fk').startsWith('ct:'), 'the focus path stays');
      assert.equal(board.tagName, 'BUTTON', 'a button: the keyboard and touch reach it, not only the pointer');
    }
  }
  assert.ok(card(ui, a, 'Thorvald', 3).getAttribute('title').startsWith('Reward: '), 'the reward is in the text');
  select(ui, '#lsb-fac', 2); await ui.settle();
  assert.equal(card(ui, a, 'Thorvald', 3).getAttribute('title'), card(ui, a, 'Thorvald', 3).getAttribute('data-why'), 'a box that is off says why');
});

// ---- #939: the place line and the specialization ------------------------------------------------------------------------------------------------------------------------
test('#939: the place is under the House name, a link with the Classic hint, and the specialization is one line that is the button', async () => {
  const a = synthetic();
  const ui = await open();
  look(ui, a, 'Kenola');
  const i = ui.doc.querySelector('.lcc-bd-insp');
  const t = i.querySelector('.lcc-bd-insp-title');
  const kids = [...t.children].map((c) => c.className.split(' ')[0]);
  assert.deepEqual(kids, ['lcc-bd-insp-name', 'lcc-bd-loc', 'lcc-bd-linkb'], 'name, place, specialization: in this order');
  const loc = t.querySelector('.lcc-bd-loc');
  assert.equal(loc.getAttribute('data-lb'), 'map', 'the Classic click');
  assert.equal(loc.getAttribute('title'), 'Click the place to open the Saved map.');
  assert.equal(text(loc.querySelector('.lcc-bd-loc-a')), 'SYNTHETIC AREA 1');
  assert.equal(text(loc.querySelector('.lcc-bd-loc-s')), 'Hagga Basin');
  const classic = await open('classic');
  assert.equal(loc.getAttribute('aria-label'), classic.doc.querySelector('#lsrow-' + idOf(a, 'Kenola') + ' .loc-t').getAttribute('aria-label'), 'the same name for a screen reader as the Classic list');
  assert.equal(i.querySelector('.lcc-bd-where'), null, 'the old place line of the band is gone');
  const gp = t.querySelector('.lcc-bd-gopath');
  assert.match(text(gp), /^[A-Z][a-z]+ Specialization$/);
  assert.match(gp.getAttribute('aria-label'), /Open [A-Z][a-z]+ in Specializations/);
  assert.equal(gp.getAttribute('title'), null, 'no mouse-over but the place and the rewards (the panel adds none)');
  assert.equal(i.querySelector('.lcc-bd-bloc'), null);
  assert.doesNotMatch(text(i), /Associated specialization|Open [A-Z][a-z]+ in Specializations$/);
  const calls = []; const real = ui.w.liveMapShow; ui.w.liveMapShow = (...x) => { calls.push(x); real(...x); };
  click(ui, loc); // the same click as the Classic list: the Saved map opens at the House
  assert.deepEqual(calls.map((c) => [c[0], c[1], c[2], c[3]]), [['HaggaBasin', 0.25, 0.5, 'Kenola']]);
});

test('#939: a place the Saved map has no picture of, and a House not mapped, say why in the Classic words and a click says it as a toast', async () => {
  const a = synthetic();
  const ui = await open();
  look(ui, a, 'Alexin'); // Arrakeen: a name with no picture
  let loc = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-loc');
  assert.equal(loc.getAttribute('aria-disabled'), 'true');
  assert.equal(loc.getAttribute('title'), 'The Saved map has no picture of Arrakeen yet.');
  assert.match(text(loc), /Arrakeen.*no map picture/);
  click(ui, loc);
  assert.equal(text(last(ui)), 'The Saved map has no picture of Arrakeen yet.');
  look(ui, a, 'Thorvald'); // not mapped
  loc = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-loc');
  assert.equal(text(loc), 'Representative location not mapped yet');
  assert.equal(loc.getAttribute('title'), 'This house is not mapped yet: the save holds no map marker for its representative and the editor has no fixed place for it.');
  click(ui, loc);
  assert.match(text(last(ui)), /^This house is not mapped yet/);
  assert.equal(toasts(ui).length, 1);
  look(ui, a, 'Lindaren'); // a Deep Desert place with a grid cell and a name
  loc = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-loc');
  assert.equal(text(loc.querySelector('.lcc-bd-loc-s')), 'Grid cell B8 · SYNTHETIC PLACE');
  assert.match(text(ui.doc.querySelector('.lcc-bd-insp')), /From you: /, 'the distance Classic shows is in the Where section');
});

// ---- #942: the four counts ---------------------------------------------------------------------------------------------------------------------------------------------
test('#942: the four counts are read-only figures: no button, no tab stop, no role of a tab, and they match the legend and the ledger', async () => {
  const a = synthetic();
  const ui = await open();
  look(ui, a, 'Wallach');
  const stats = ui.doc.querySelector('.lcc-bd-stats');
  assert.equal(stats.querySelectorAll('button, a, input, select, [tabindex], [role="tab"], [role="button"], [onclick]').length, 0);
  assert.equal(stats.getAttribute('role'), 'group');
  const nums = [...stats.querySelectorAll('.lcc-bd-stat')].map((s) => text(s.querySelector('strong')));
  const h = ui.w.LCCM.readBoard(a).byId.get(idOf(a, 'Wallach'));
  assert.deepEqual(nums, [h.reached + '/' + h.tierCount, String(h.collected), String(h.waiting), String(h.notReached)]);
  const kinds = [...ui.doc.querySelectorAll('.lcc-bd-insp .lcc-bd-tier')].map((b) => b.className.split(' ')[1]);
  assert.equal(kinds.filter((k) => k === 'delivered').length, h.collected, 'Collected matches the rows');
  assert.equal(kinds.filter((k) => k === 'waiting').length, h.waiting, 'Not collected matches the rows');
  assert.equal(kinds.filter((k) => k === 'not_reached').length, h.notReached, 'Not reached matches the rows');
  assert.equal(h.collected + h.waiting + h.unknownDelivery, h.reached, 'the counts add up to the tiers reached');
  const css = fs.readFileSync(new URL('../internal/web/static/lcc/lcc-board.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
  const stat = css.match(/\.lcc-bd-stat \{[^}]*\}/)[0];
  assert.doesNotMatch(stat, /border: 1px solid/, 'no box edge of its own');
  assert.match(stat, /padding: 10px 8px/, 'the caption has room on every side');
  assert.match(css, /\.lcc-bd-stat strong \{ font: 700 22px/);
  assert.match(css, /\.lcc-bd-stat span \{ font-size: 11px; \}/);
  assert.doesNotMatch(css, /\.lcc-bd-pin[^{]*\{[^}]*position: sticky/, 'nothing sticks to the top edge of the panel');
});

// ---- #945 and #948 ----------------------------------------------------------------------------------------------------------------------------------------------------
test('#945: no caption paragraph under the board; the check against the save stays in the names of the tier boxes', async () => {
  const a = synthetic();
  const ui = await open();
  assert.equal(ui.doc.querySelector('.lcc-bd-caption'), null);
  assert.doesNotMatch(text(ui.doc.querySelector('#lsb')), /Select a House to inspect|are the same controls as the tiers in the House Intelligence panel/);
  assert.match(card(ui, a, 'Kenola', 3).getAttribute('aria-label'), /Each click is checked against the save first\./);
  assert.match(ui.doc.querySelector('.lcc-hc-tiers').getAttribute('aria-label'), /Each click is checked against the save first\./);
});


// ---- review of #955: what a plain click and a stray toast may take away -------------------------------------------------------------------------------------------------------------------
const CONFLICT = { __fail: 'House Sor changed since it was shown.', __code: 'conflict', __status: 409 };
const noTimers = (ui) => { const t = []; const real = ui.w.setTimeout.bind(ui.w); ui.w.setTimeout = (fn, ms, ...rest) => { if (ms >= 3000) { t.push(ms); return 0; } return real(fn, ms, ...rest); }; return t; };

test('#943 review: a plain click on a tier square (a note) never takes away an error that is still showing: the note stands next to it', async () => {
  const a = synthetic();
  const ui = await open('board', { '/api/landsraad/apply': CONFLICT });
  look(ui, a, 'Sor');
  click(ui, '.lcc-bd-insp .lcc-bd-act[data-do="complete"]'); await ui.settle();
  assert.equal(toasts(ui).length, 1);
  assert.ok(last(ui).classList.contains('err'));
  click(ui, card(ui, a, 'Thorvald', 1)); // a lower reached tier: it says how to remove it, and writes nothing
  assert.equal(toasts(ui).length, 2, 'the error is still there and the note is shown next to it');
  assert.ok(toasts(ui)[0].classList.contains('err'), 'the unread error was not removed');
  assert.match(text(toasts(ui)[1]), /^To remove tiers of House Thorvald/);
  click(ui, card(ui, a, 'Thorvald', 2));
  assert.equal(toasts(ui).length, 2, 'a second note takes the place of the first note, not of the error');
  assert.ok(toasts(ui)[0].classList.contains('err'));
  toasts(ui)[0].querySelector('button.x').click();
  assert.equal(toasts(ui).length, 1);
});

test('#943 review: a plain click does not take away a warning that is still showing, and a warning stays until it is dismissed', async () => {
  const a = synthetic();
  const ui = await open('board', { '/api/landsraad/apply': { ...OK, rewards_already_given: 1, missions: 0 } });
  const timers = noTimers(ui);
  look(ui, a, 'Kenola');
  click(ui, row(ui, 3)); await ui.settle();
  const warn = last(ui);
  assert.ok(warn.classList.contains('part'), 'a warning result is a toast that stays');
  assert.ok(warn.querySelector('button.x'), 'with a Dismiss control');
  assert.deepEqual(timers, [], 'no timer takes it away (a long text used to leave after 9 to 20 seconds)');
  click(ui, card(ui, a, 'Thorvald', 1));
  assert.ok(toasts(ui).includes(warn) && warn.isConnected, 'the warning is still there after a click that wrote nothing');
  assert.equal(toasts(ui).length, 2);
  warn.querySelector('button.x').click();
  assert.equal(toasts(ui).length, 1, 'dismissed: only the note is left');
});

test('#943 review: a result that is only queued (the game is running) is a warning too and stays until it is dismissed', async () => {
  const a = synthetic();
  const ui = await boot({ get: { '/api/landsraad': a }, post: { '/api/landsraad/apply': { ...OK, outcome: 'queued' } }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  const timers = noTimers(ui);
  ui.w.eval('NOT_SAVED = "Queued: it is written as soon as the save can be written."');
  look(ui, a, 'Kenola');
  click(ui, row(ui, 3)); await ui.settle();
  assert.ok(last(ui).classList.contains('part'));
  assert.match(text(last(ui)), /written as soon as the save can be written/);
  assert.deepEqual(timers, []);
});

test('#943 review: an error replaces an older note, and a new write result replaces the last result', async () => {
  const a = synthetic();
  const ui = await open('board', { '/api/landsraad/apply': CONFLICT });
  click(ui, card(ui, a, 'Thorvald', 1));
  assert.equal(toasts(ui).length, 1);
  look(ui, a, 'Sor');
  click(ui, '.lcc-bd-insp .lcc-bd-act[data-do="complete"]'); await ui.settle();
  assert.equal(toasts(ui).length, 1, 'the error took the place of the note');
  assert.ok(last(ui).classList.contains('err'));
});

test('#943 review: a toast that something else raises while a write is running (a poll, a lost connection) is not taken for the write\'s and is not removed by its result', async () => {
  const a = synthetic();
  const ui = await open('board', { '/api/landsraad/apply': OK });
  const realFetch = ui.w.fetch;
  let release; const gate = new Promise((r) => { release = r; });
  ui.w.fetch = async (p, opt = {}) => { if ((opt.method || 'GET') === 'POST') await gate; return realFetch(p, opt); };
  look(ui, a, 'Kenola');
  click(ui, row(ui, 3)); // the write is now waiting for the answer
  await new Promise((r) => ui.w.setTimeout(r, 20));
  ui.w.toast('Lost the connection to the editor.', true); // raised by something else, in the window of the write
  assert.equal(toasts(ui).length, 1);
  assert.equal(toasts(ui)[0].getAttribute('data-one'), null, 'it is not tagged as the write\'s');
  release(); await ui.settle();
  const t = toasts(ui);
  assert.equal(t.length, 2, 'the result of the write and the stray error are both on screen');
  assert.ok(t.some((x) => /Lost the connection/.test(x.textContent)), 'the stray error was not eaten by the write\'s result');
  assert.ok(t.some((x) => /House Kenola: your contribution/.test(x.textContent)));
  assert.equal(ui.w.eval('typeof toastOneKey'), 'undefined', 'no global key is held across the await');
});
