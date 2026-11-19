// The board on a touch screen (#880, #906, #905). A tap of a tier box on a House card only chooses the House on a coarse pointer (nothing is written; the panel of the House shows what each tier grants and
// writes through its own buttons); a mouse click and a keyboard activation still write, with the requests of before (web-tests/lcc-board-requests.test.mjs holds those). The hit areas of the controls
// keep the agreed floors (44 px at phone widths, 36 px for touch above that), and a jump to the Term and Decrees section stops below the sticky header. Real page code, the synthetic answer of M1.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir } from './lcc-load.mjs';

afterEach(closeAll);

const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 0, outcome: 'written' };
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
const writes = (ui) => ui.posted.filter((p) => p.path === '/api/landsraad/apply');
const media = (coarse) => (w) => { w.matchMedia = (q) => ({ matches: coarse && /\(pointer:\s*coarse\)/.test(q), media: q, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} }); };
async function open(coarse) {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, post: { '/api/landsraad/apply': OK }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' }, pre: media(coarse) });
  await ui.settle();
  return ui;
}
// a click as a finger or a mouse makes it (detail 1: one press), and as the keyboard makes it (detail 0)
const press = (ui, el, detail) => { el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true, detail })); };
const box = (ui, a, name, n) => ui.doc.querySelector('[data-fk="ct:' + idOf(a, name) + ':' + n + '"]');

test('#880: on a coarse pointer a tap of a tier box chooses the House and writes nothing', async () => {
  const a = synthetic();
  const ui = await open(true);
  const b = box(ui, a, 'Kenola', 3);
  assert.ok(b, 'the tier box');
  assert.equal(ui.doc.querySelector('.lcc-bd-insp').dataset.id === String(idOf(a, 'Kenola')), false, 'the House is not chosen yet');
  press(ui, b, 1);
  await ui.settle();
  assert.equal(writes(ui).length, 0, 'a tap wrote');
  assert.equal(ui.doc.querySelector('[data-fk="c-yes"]'), null, 'a tap asked a question of a write');
  assert.equal(ui.doc.querySelector('.lcc-bd-insp').dataset.id, String(idOf(a, 'Kenola')), 'the panel of the House is open');
  assert.ok(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-tier[data-t="3"]'), 'the panel has the tier rows that say what each tier grants');
  // a second tap on a box of the same House keeps it chosen (it does not let it go), still without a write
  press(ui, box(ui, a, 'Kenola', 1), 1);
  await ui.settle();
  assert.equal(ui.doc.querySelector('.lcc-bd-insp').dataset.id, String(idOf(a, 'Kenola')));
  assert.equal(writes(ui).length, 0);
  // a tap of a box of another House moves the panel to that House
  press(ui, box(ui, a, 'Lindaren', 2), 1);
  await ui.settle();
  assert.equal(ui.doc.querySelector('.lcc-bd-insp').dataset.id, String(idOf(a, 'Lindaren')));
  assert.equal(writes(ui).length, 0);
});

test('#880: the panel of the House still writes through its own buttons on a coarse pointer', async () => {
  const a = synthetic();
  const ui = await open(true);
  press(ui, box(ui, a, 'Kenola', 3), 1);
  await ui.settle();
  press(ui, ui.doc.querySelector('.lcc-bd-insp .lcc-bd-tier[data-t="3"]'), 1);
  await ui.settle();
  assert.equal(writes(ui).length, 1, 'the tier row of the panel writes');
});

test('#880: a keyboard activation of a tier box (a click with detail 0) writes on a coarse pointer, as before', async () => {
  const a = synthetic();
  const ui = await open(true);
  press(ui, box(ui, a, 'Kenola', 3), 0);
  await ui.settle();
  assert.equal(writes(ui).length, 1);
});

test('#880: a mouse click of a tier box (a fine pointer) writes at once, as before', async () => {
  const a = synthetic();
  const ui = await open(false);
  press(ui, box(ui, a, 'Kenola', 3), 1);
  await ui.settle();
  assert.equal(writes(ui).length, 1);
});

test('#880: the tap rule is in the one click handler of the board and takes the click away from the page handler that writes', () => {
  const js = fs.readFileSync(path.join(lccDir, 'lcc-board.js'), 'utf8').replace(/\r/g, '');
  const at = js.indexOf('function onClick(e)');
  assert.ok(at > 0);
  const head = js.slice(at, at + 700);
  assert.match(head, /\.lcc-hc-t\[data-lb="tier"\]/);
  assert.match(head, /e\.detail > 0 && coarsePointer\(\)/);
  assert.match(head, /e\.stopPropagation\(\)/);
  assert.match(js, /matchMedia\('\(pointer: coarse\)'\)/);
});

// ---- the hit areas (#906) ----
const css = fs.readFileSync(path.join(lccDir, 'lcc-board.css'), 'utf8').replace(/\r/g, '');
const block = css.slice(css.indexOf('==== Touch target floors (#906)'));
const termCss = fs.readFileSync(path.join(lccDir, 'lcc-term.css'), 'utf8').replace(/\r/g, '');

test('#906: the touch floors are written once, at the end of the board stylesheet, for 36 px (touch or 900 px) and 44 px (640 px)', () => {
  assert.ok(block.length > 200, 'the block is there');
  const touch = /@media \(pointer: coarse\), \(max-width: 900px\) \{([\s\S]*?)\n\}\n@media \(max-width: 640px\) \{([\s\S]*?)\n\}/.exec(block);
  assert.ok(touch, 'the two media blocks');
  const [, t36, t44] = touch;
  // a tier box is 24 px with a 1 px border: its padding box is 22 px, and the invisible extension adds to that
  const ext = (s) => { const m = /\.lcc-hc-t::after \{ top: (-?\d+)px; bottom: (-?\d+)px; left: (-?\d+)px; right: (-?\d+)px; \}/.exec(s); return m && m.slice(1).map(Number); };
  const e36 = ext(t36);
  assert.ok(e36, 'the 36 px extension');
  assert.equal(22 - e36[0] - e36[1], 36, 'a tier box has a 36 px high hit area on touch');
  assert.equal(22 - e36[2] - e36[3], 26, 'and 26 px wide: the 24 px box and the 2 px gap, so two neighbours never overlap');
  const m44 = /\.lcc-hc-t::after \{ top: (-?\d+)px; bottom: (-?\d+)px; \}/.exec(t44);
  assert.ok(m44, 'the 44 px extension');
  assert.equal(22 - Number(m44[1]) - Number(m44[2]), 44, 'a tier box has a 44 px high hit area at phone widths');
  // the House choice box: 36 px on touch, 44 px on a phone, the box itself stays where it was drawn
  assert.match(t36, /\.lcc-bd-pickl \{[^}]*width: 36px; height: 36px;/);
  assert.match(t44, /\.lcc-bd-pickl \{ width: 44px; height: 44px;/);
  assert.match(t44, /\.lcc-bd-c-pick \.lcc-bd-pickl \{ width: 44px; height: 44px; \}/);
  // every control class named by the issue has the floor
  for (const c of ['.lcc-bd-layout button', '.lcc-bd-btn', '.lcc-bd-act', '.lcc-bd-back', '.lcc-bd-eselect', '.lcc-bd-select']) {
    assert.ok(t36.includes(c) && /min-height: 36px/.test(t36), c + ' 36');
    assert.ok(t44.includes(c) && /min-height: 44px/.test(t44), c + ' 44');
  }
  for (const c of ['.lcc-bd-chip', '.lcc-bd-lbl', '.lcc-bd-sortb', '.lcc-bd-linkb']) assert.ok(t44.includes(c), c + ' 44');
  assert.match(t44, /\.lcc-bd-namebtn \{ min-width: 44px; \}/);
});

test('#906: the tier boxes keep their drawn size (24 px) on touch: only the hit area grows', () => {
  assert.match(css, /@media \(pointer: coarse\), \(max-width: 900px\) \{\n  \.lcc-hc-t \{ flex: 0 0 24px; width: 24px; max-width: 24px; height: 24px;/);
  assert.doesNotMatch(block, /\.lcc-hc-t \{[^}]*\b(width|height):/, 'the floors block never resizes the box itself');
});

// ---- #905 ----
test('#905: the Term and Decrees section and its heading stop below the sticky header and the warning bar', () => {
  assert.match(termCss, /\.lcc-tm, \.lcc-tm \.lcc-tm-h \{ scroll-margin-top: calc\(var\(--hdr-h, 56px\) \+ 12px \+ var\(--lsb-warn-room, 0px\)\); \}/);
});
