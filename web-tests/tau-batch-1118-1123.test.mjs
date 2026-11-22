// One batch of Landsraad / queue UI fixes (UAT 2026-11-21): #1119 the Actions panel of the House inspector, #1120 the term panel's result, #1123 the queue view and the waiting automatic refill.
// (#1118, the N / 25 counts, is in lcc-look-feel-1113-1116.test.mjs.) Real page code in jsdom; jsdom has no layout, so the look is checked as the rules the style sheets carry.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const here = (p) => new URL(p, import.meta.url);
const read = (p) => fs.readFileSync(here(p), 'utf8');
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : '');
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); assert.ok(el, 'no element ' + sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const toasts = (ui) => [...ui.doc.querySelectorAll('#toast .d-toast')];

// ---------------------------------------------------------------- #1119: the Actions panel

test('#1119: the Actions panel is a two-column grid of equal-width keys, Faction over the whole width, one accent key, Reopen quieter, Set tier set apart (style rules)', () => {
  const css = read('../internal/web/static/lcc/lcc-board.css');
  assert.match(css, /\.lcc-bd-egrid \{[^}]*display: grid;[^}]*grid-template-columns: repeat\(2, minmax\(0, 1fr\)\);/, 'two equal columns');
  assert.match(css, /\.lcc-bd-egrid:not\(\.lcc-bd-eset\) > \.lcc-bd-elab:first-child \{ grid-column: 1 \/ -1; \}/, 'the Faction box takes the whole width');
  assert.match(css, /\.lcc-bd-egrid > \.lcc-bd-act \{[^}]*width: 100%;/, 'every key fills its column, so every row ends at the panel edge');
  assert.match(css, /\.lcc-bd-eset \{ margin-top: 1[0-9]px; \}/, 'a small gap above Set tier');
  assert.match(css, /\.lcc-bd-act-pri \{[^}]*background: var\(--lcc-gold\);/, 'the accent key is filled');
  assert.match(css, /\.lcc-bd-act-quiet \{[^}]*background: transparent;/, 'Reopen is the quiet one');
  assert.equal((css.match(/\.lcc-bd-act-pri \{/g) || []).length, 1, 'one accent style');
  assert.doesNotMatch(css, /lcc-bd-act-danger/, 'the red mark on Reopen is gone');
});

test('#1119: the markup keeps every control, word, key and request; Fill is the only accent key, Reopen the quiet one, and Reopen still asks first', async () => {
  const a = synthetic();
  const ui = await boot({ get: { '/api/landsraad': a }, post: { '/api/landsraad/board': { ok: true } }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  const t = a.tasks.find((x) => x.house_name === 'DA_HouseKenola');
  click(ui, '[data-fk="ht:' + t.id + '"]'); await ui.settle();
  const ed = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-editor');
  assert.ok(ed, 'the inspector has the actions block');
  const rows = [...ed.querySelectorAll('.lcc-bd-erow')];
  assert.equal(rows.length, 2);
  assert.deepEqual([...rows[0].querySelectorAll('.lcc-bd-act')].map((b) => text(b)), ['Fill 5 tiers', 'Fill faction goal', 'Complete House', 'Reopen'], 'the same words, in the order the grid reads them');
  assert.deepEqual([...rows[1].querySelectorAll('label, button')].map((b) => text(b).replace(/ 0.*$/, '')), ['Set tier', 'Apply tier']);
  assert.deepEqual([...ed.querySelectorAll('.lcc-bd-act-pri')].map((b) => b.dataset.do), ['fill']);
  assert.deepEqual([...ed.querySelectorAll('.lcc-bd-act-quiet')].map((b) => b.dataset.do), ['reopen']);
  // Reopen keeps its confirmation: pressing it (on a House that is complete) writes nothing until the question is answered
  const before = ui.posted.length;
  const reopen = ed.querySelector('.lcc-bd-act[data-do="reopen"]');
  if (reopen.getAttribute('aria-disabled') !== 'true') { click(ui, reopen); await ui.settle(); assert.equal(ui.posted.length, before, 'nothing is sent before the question is answered'); }
});

// ---------------------------------------------------------------- #1120: the term panel's result is a toast

const term = async (post = {}) => {
  const a = synthetic();
  const ui = await boot({ get: { '/api/landsraad': a }, post: { '/api/landsraad/term': { ok: true, outcome: 'written' }, ...post }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  return ui;
};

test('#1120: Apply says what it did in a toast; no line of the term panel keeps the words, and nothing about it is drawn in the green of the Atreides', async () => {
  const ui = await term();
  select(ui, '#lsb-tad', 3);
  click(ui, '[data-lb="tapply"]'); await ui.settle();
  const t = toasts(ui).filter((x) => x.getAttribute('data-one') === 'lbd');
  assert.equal(t.length, 1, 'one result toast');
  assert.match(text(t[0]), /^Applied: active decree .*, reigning faction Atreides\./);
  assert.equal(ui.doc.querySelector('#lsb-tresult'), null, 'no status line in the panel');
  assert.doesNotMatch(text(ui.doc.querySelector('#lcc-term')), /Applied:/, 'the words are not left on the panel');
  assert.ok(!t[0].classList.contains('err'), 'a success is not an error: it leaves by itself');
  assert.equal(t[0].querySelector('button.x'), null, 'a success has no Dismiss button, because it goes away on its own');
  // the colour: a result of the Landsraad page is the page's gold, never the module's success green (which is the Atreides hue)
  const css = read('../internal/web/static/app.css');
  assert.match(css, /#toast \.d-toast\[data-one="lbd"\]:not\(\.warn\):not\(\.bad\) \.d-i\{color:var\(--c-gold\)\}/);
  assert.match(css, /#toast \.d-toast\[data-one="lbd"\]:not\(\.warn\):not\(\.bad\)\{border-left-color:var\(--c-gold\)\}/);
  assert.doesNotMatch(css, /lsb-tresult/, 'no style of the old line is left');
  assert.doesNotMatch(read('../internal/web/static/lcc/lcc-term.css'), /lcc-tm-result/, 'nor in the section');
});

test('#1120: while Apply runs nothing is said yet (no "Writing…" line or toast); the one result comes when the answer is in', async () => {
  const ui = await term();
  const orig = ui.w.fetch; let release = null;
  ui.w.fetch = (p, opt = {}) => (opt.method === 'POST' && String(p).includes('/api/landsraad/term') ? new Promise((res) => { release = () => res(orig(p, opt)); }) : orig(p, opt));
  select(ui, '#lsb-tad', 3);
  click(ui, '[data-lb="tapply"]');
  await new Promise((r) => ui.w.setTimeout(r, 5));
  assert.ok(release, 'the request is held');
  assert.equal(toasts(ui).length, 0, 'nothing is said while it runs: ' + toasts(ui).map(text).join(' | '));
  release(); await ui.settle();
  assert.equal(toasts(ui).filter((x) => /^Applied: /.test(text(x))).length, 1);
});

test('#1120: an error of the term panel is an error toast that stays until it is dismissed; the draft hint is not a result and stays with the choice', async () => {
  const ui = await term();
  // the draft hint: the choice differs from the term's active decree
  select(ui, '#lsb-tad', 3);
  assert.match(text(ui.doc.querySelector('#lcc-tm-pending')), /^Not applied yet: the term still has .* as its active decree\. Press Apply to set .*\.$/);
  // no faction chosen: refused, as an error toast
  select(ui, '#lsb-trf', '');
  click(ui, '[data-lb="tapply"]'); await ui.settle();
  const e = toasts(ui).find((x) => x.getAttribute('data-one') === 'lbd');
  assert.ok(e && e.classList.contains('err'), 'an error toast');
  assert.match(text(e), /^Choose a reigning faction first/);
  assert.ok(e.querySelector('button.x'), 'it can be dismissed, and stays until it is');
  assert.match(text(ui.doc.querySelector('#lcc-tm-pending')), /^Not applied yet/, 'the partial-draft hint is still there');
  // the draft is applied (this fixture answers with the same term again, so the choice still differs from it): the result is the toast, and the hint is still drawn from the choice
  select(ui, '#lsb-trf', 1);
  click(ui, '[data-lb="tapply"]'); await ui.settle();
  assert.ok(toasts(ui).some((x) => /^Applied: /.test(text(x))), 'the result is a toast');
  assert.match(text(ui.doc.querySelector('#lcc-tm-pending')), /^Not applied yet/, 'the hint follows the choice, not the toast');
  // the choice goes back to what the term holds: the hint goes
  select(ui, '#lsb-tad', 7);
  assert.equal(text(ui.doc.querySelector('#lcc-tm-pending')), '');
});

// ---------------------------------------------------------------- #1123: the queue view and the waiting automatic refill

const BASE = { path: 'C:\\x\\game.db', dirty: false, pending: [], live: true, saveBlocked: true, blockReason: 'a single-player session is active', changedOnDisk: false, gameMode: 'sp', gameProcesses: [], journalOff: '', held: null, restored: null, lastWrite: null, readOnly: '', schemaNote: '' };
const WAITING = { waiting: true, why: 'the single-player session ended 6 minutes ago but the game wrote its database 2 times after that' };
const state = (over) => ({ ...BASE, autoRefill: { waiting: false, why: '' }, ...over });
const boot2 = (st, more = {}) => boot({ get: { '/api/save/state': st, ...more }, post: { '/api/autopass/cancel': { cancelled: true } } });

test('#1123: with nothing queued the Discard queue button is not drawn (the module\'s display no longer beats [hidden])', async () => {
  const ui = await boot2(state({}));
  const b = ui.doc.querySelector('#btnDiscard');
  assert.equal(b.hidden, true, 'the page hides it with nothing listed');
  // jsdom applies the style sheets, so the cascade that showed the button in the screenshot is reproduced: the module's .d-btn display against the hidden attribute
  const st = ui.doc.createElement('style');
  st.textContent = read('../design/assets/components-fremen.css') + '\n' + read('../internal/web/static/app.css');
  ui.doc.head.appendChild(st);
  assert.equal(ui.w.getComputedStyle(b).display, 'none', 'hidden means not drawn');
  b.hidden = false;
  assert.notEqual(ui.w.getComputedStyle(b).display, 'none', 'and it is drawn when it is shown (the rule is only for [hidden])');
});

test('#1123: a refill that waits for the game is listed in What is queued with what it is, what it waits for and a Cancel; the chip counts it; Discard queue stays away', async () => {
  const ui = await boot2(state({ autoRefill: WAITING }));
  const gw = ui.doc.querySelector('#gamewarn');
  assert.equal(ui.doc.querySelector('#pending').textContent, '1 queued', 'the chip counts the refill');
  assert.match(ui.doc.querySelector('#pending').getAttribute('aria-label'), /^1 queued, show the queued changes/);
  const list = gw.querySelector('.bn-list');
  assert.ok(list, 'the queue view is there');
  assert.equal(text(list.querySelector('summary')), 'What is queued (1)');
  const li = list.querySelector('li.bn-refill');
  assert.ok(li, 'the refill is a line of the list');
  assert.match(text(li), /^Automatic refill\. The jobs switched on in Settings, run as one edit\. The single-player session ended 6 minutes ago but the game wrote its database 2 times after that\. It runs by itself, on the file as the game left it, as soon as that clears\./);
  assert.doesNotMatch(text(li), /in \d+ ?s|seconds? left|countdown/i, 'no frozen countdown: the reason says when it runs');
  assert.ok(li.querySelector('button[data-act="cancelRefill"]'), 'a way to cancel it');
  assert.equal(ui.doc.querySelector('#btnDiscard').hidden, true, 'no Discard queue: there is no edit to discard');
  assert.match(text(gw), /Changes you make now are queued/, 'the banner sentence for no edits is unchanged');
  assert.ok(gw.classList.contains('warn'));
});

test('#1123: edits and a waiting refill are one list; Discard queue is for the edits', async () => {
  const ui = await boot2(state({ dirty: true, pending: ['solari +10000'], autoRefill: WAITING }));
  const list = ui.doc.querySelector('#gamewarn .bn-list');
  assert.equal(text(list.querySelector('summary')), 'What is queued (2)');
  assert.equal(list.querySelectorAll('li').length, 2);
  assert.match(text(ui.doc.querySelector('#gamewarn')), /1 change is queued\./, 'the banner counts the edits only');
  assert.equal(ui.doc.querySelector('#pending').textContent, '2 queued');
  assert.equal(ui.doc.querySelector('#btnDiscard').hidden, false, 'there is an edit to discard');
});

test('#1123: Cancel refill asks the editor to stop the wait, says so, and the list goes', async () => {
  const st = state({ autoRefill: WAITING });
  const ui = await boot2(st);
  st.autoRefill = { waiting: false, why: '' }; // what the editor answers after the cancel
  click(ui, '[data-act="cancelRefill"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/autopass/cancel').map((p) => p.body), [{}]);
  assert.ok(toasts(ui).some((x) => /^Automatic refill cancelled\. Nothing was written\./.test(text(x))), toasts(ui).map(text).join(' | '));
  assert.equal(ui.doc.querySelector('#gamewarn .bn-list'), null, 'nothing is listed any more');
  assert.equal(ui.doc.querySelector('#pending').textContent, '');
});

test('#1123: no waiting refill and nothing queued: no list, no chip (the answer of an older editor, with no autoRefill field, is the same)', async () => {
  for (const st of [state({}), (() => { const s = state({}); delete s.autoRefill; return s; })()]) {
    const ui = await boot2(st);
    assert.equal(ui.doc.querySelector('#gamewarn .bn-list'), null);
    assert.equal(ui.doc.querySelector('#pending').textContent, '');
    closeAll();
  }
});

test('#1123: the queued-refill note is one short sentence with a neutral queued mark, not the green check; any other note is shown as it is', async () => {
  const note = 'Automatic refill queued: a single-player session is active, and the game rewrites the save about every 20 seconds while it runs. It runs by itself, on the file as the game left it, as soon as that clears.';
  const ui = await boot2(state({}), { '/api/startup': { note, warn: false } });
  const t = toasts(ui);
  assert.equal(t.length, 1);
  assert.equal(text(t[0]), 'Refill queued; it runs when the save is free.');
  assert.ok(t[0].classList.contains('queued'), 'its own kind');
  assert.ok(!t[0].classList.contains('warn') && !t[0].classList.contains('bad'));
  assert.equal(t[0].getAttribute('role'), 'status');
  const d = [...t[0].querySelectorAll('svg.d-i path')].map((p) => p.getAttribute('d'));
  assert.ok(d.length === 2 && !d.includes('M5 12.5l4.5 4.5L19 7.5'), 'a clock, not the check: ' + d.join(' | '));
  const css = read('../internal/web/static/app.css');
  assert.match(css, /#toast \.d-toast\.queued \.d-i\{color:var\(--c-dim\)\}/, 'neutral, not green');
  assert.match(css, /#toast \.d-toast\.queued\{border-left-color:var\(--c-dim\)\}/);
  closeAll();
  // the other notes keep their words (a refill that was skipped is a warning with its reason)
  const other = 'Automatic refill skipped: edits from before are still waiting to be written.';
  const ui2 = await boot2(state({}), { '/api/startup': { note: other, warn: true } });
  assert.equal(text(toasts(ui2)[0].querySelector('span')), other);
});
