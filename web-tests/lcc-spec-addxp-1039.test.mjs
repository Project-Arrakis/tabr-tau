// #1039: the Add XP box of the Command Center's Specialization page. (1) The refusal is drawn above the box and its tip, so the tip that opens while the box has focus never covers it. (2) Ctrl+minus and
// Cmd+minus (the browser's zoom) are not swallowed; the plain minus key still is. (3) The refusal is a live region that is always in the page and never unhidden. Nothing about the rule (XP from 1 up) or
// the request is touched here: the rule and the request are held by lcc-spec-view / lcc-spec-requests. Real lcc-spec.js in jsdom, and the style file read as text.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { lccDir, read, load } from './lcc-load.mjs';

const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const byId = (id) => JSON.parse(JSON.stringify(CASES.find((c) => c.id === id).response));

function draw() {
  const { dom, w } = load();
  dom.window.eval(fs.readFileSync(path.join(lccDir, 'lcc-spec.js'), 'utf8'));
  const S = w.LCCS;
  S.state.sel = 'combat';
  const box = w.document.createElement('div');
  w.document.body.appendChild(box);
  S.mount(box, S.readSpecs(byId('mixed-five-tracks'), []), {});
  return { w, box };
}
const type = (d, inp, v) => { inp.value = v; inp.dispatchEvent(new d.w.Event('input', { bubbles: true })); };

test('the refusal sits above the box and its tip: it is before the row of the box in the page, never inside it', () => {
  const d = draw();
  const inp = d.box.querySelector('#spxCombat'), err = d.box.querySelector('#lcs-xperr'), row = inp.closest('.lcc-tt');
  assert.ok(row && row.querySelector('.lcc-tip'), 'the box shares its row with the tip');
  assert.equal(row.contains(err), false, 'the note is not inside the row that holds the tip');
  assert.ok(err.compareDocumentPosition(row) & d.w.Node.DOCUMENT_POSITION_FOLLOWING, 'the note comes before the row');
  assert.equal(err.parentNode, row.parentNode, 'both are in the Add XP group');
  assert.ok(d.box.querySelector('label[for="spxCombat"]').compareDocumentPosition(err) & d.w.Node.DOCUMENT_POSITION_FOLLOWING, 'the label comes first');
});

test('Ctrl+minus and Cmd+minus reach the browser; the plain minus key is still refused', () => {
  const d = draw();
  const inp = d.box.querySelector('#spxCombat');
  const press = (init) => { const e = new d.w.KeyboardEvent('keydown', Object.assign({ bubbles: true, cancelable: true }, init)); inp.dispatchEvent(e); return e.defaultPrevented; };
  assert.equal(press({ key: '-' }), true, 'minus');
  assert.equal(press({ key: 'Subtract' }), true, 'numpad minus');
  assert.equal(press({ key: '-', ctrlKey: true }), false, 'Ctrl+minus (zoom out)');
  assert.equal(press({ key: '-', metaKey: true }), false, 'Cmd+minus (zoom out)');
  assert.equal(press({ key: 'Subtract', ctrlKey: true }), false, 'Ctrl+numpad minus');
  assert.equal(press({ key: '5' }), false, 'a digit');
});

test('the refusal is a live region that stays in the page: never hidden, empty when the number is fine, filled for a bad one', () => {
  const d = draw();
  const inp = d.box.querySelector('#spxCombat'), err = d.box.querySelector('#lcs-xperr');
  const present = (why) => { assert.equal(err.isConnected, true, why); assert.equal(err.hidden, false, why + ': not hidden'); assert.equal(err.hasAttribute('hidden'), false, why + ': no hidden attribute'); assert.equal(err.getAttribute('aria-live'), 'polite', why); };
  present('at first');
  assert.equal(err.textContent, '');
  for (const v of ['0', '-5', '']) { type(d, inp, v); present(v); assert.match(err.textContent, /^Type a whole number of XP from 1 to 44,182\./); assert.equal(inp.getAttribute('aria-invalid'), 'true'); }
  type(d, inp, '500'); present('fixed'); assert.equal(err.textContent, ''); assert.equal(inp.getAttribute('aria-invalid'), 'false');
  assert.match(inp.getAttribute('aria-describedby'), /lcs-xperr/);
});

test('the style takes no room and draws no mark for the empty note, and the script never toggles hidden on it', () => {
  const css = read('internal', 'web', 'static', 'lcc', 'lcc-spec.css');
  assert.match(css, /\.lcc-sp-note-live:empty\s*\{[^}]*margin:\s*0[^}]*padding:\s*0[^}]*border:\s*0/);
  assert.match(css, /\.lcc-sp-note-live:empty::before\s*\{\s*content:\s*none/);
  const js = read('internal', 'web', 'static', 'lcc', 'lcc-spec.js');
  assert.doesNotMatch(js, /\.hidden\s*=\s*!/, 'no note is hidden and unhidden');
});
