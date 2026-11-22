// #1134: the mouse-over text of Find House (and of Filter) stayed on screen after a House was chosen, because the focus returns to the key when its dialog closes and the style sheet shows the text
// for :focus-within. The key that gets the focus back is now quiet (class lcc-k-tip-quiet, drawn with no text) until the next mouse-over of that key or the next key press. jsdom draws no :hover or
// :focus, so the test reads the class that the code sets and clears, and the style sheet's rule that hides the text for that class.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const CSS = fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', 'lcc', 'lcc-board.css'), 'utf8');
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const key = (ui, el, k) => el.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }));
const over = (ui, el) => el.dispatchEvent(new ui.w.MouseEvent('mouseover', { bubbles: true }));
const QUIET = 'lcc-k-tip-quiet';
async function open() {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, post: {}, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  // jsdom lays nothing out, so offsetParent is always null; the page asks it to find the Find House key that is drawn. A node inside a hidden box is not drawn.
  Object.defineProperty(ui.w.HTMLElement.prototype, 'offsetParent', { configurable: true, get() { return this.closest('[hidden]') ? null : this.parentNode; } });
  return ui;
}
// the Find House key that is drawn and focusable (the strip's key, or the bar's small key)
const findKey = (ui) => [...ui.doc.querySelectorAll('[data-lk="findbtn"]')].find((b) => !b.closest('[hidden]')) || ui.doc.querySelector('[data-lk="findbtn"]');

test('the style sheet hides the mouse-over text of a quiet key, and the rule beats the :hover / :focus-within rule', () => {
  const lines = CSS.split(/\r?\n/);
  const show = lines.find((l) => l.startsWith('.lcc-k-tipped:hover > .lcc-k-tip'));
  const hide = lines.find((l) => l.startsWith('.lcc-k-tipped.lcc-k-tip-quiet > .lcc-k-tip'));
  assert.ok(show && show.includes(':focus-within'), 'the rule that shows the text is still there');
  assert.ok(hide, 'no rule for a quiet key');
  assert.match(hide, /display:\s*none\s*!important/);
});

test('#1134: choosing a House in Find House gives the key its focus back and the key shows no mouse-over text', async () => {
  const ui = await open();
  const k = findKey(ui);
  assert.ok(k.classList.contains('lcc-k-tipped'), 'the key has a mouse-over text');
  assert.ok(!k.classList.contains(QUIET), 'a key that was not closed over is not quiet');
  click(ui, k);
  assert.equal(ui.doc.querySelector('#lcc-bd-find').hidden, false, 'the list is open');
  assert.ok(!k.classList.contains(QUIET), 'while the list is open the key is as always');
  const pick = [...ui.doc.querySelectorAll('[data-lk="fpick"]')].find((o) => text(o) === 'Sor');
  assert.ok(pick);
  click(ui, pick);
  assert.equal(ui.doc.querySelector('#lcc-bd-find').hidden, true, 'the list closed');
  assert.equal(ui.doc.activeElement, k, 'the focus is back on the key');
  assert.ok(k.classList.contains(QUIET), 'the key is quiet: its text would stay as long as the focus is on it');
});

test('#1134: the text comes back at the next mouse-over of the key, and at the next key press', async () => {
  const ui = await open();
  const k = findKey(ui);
  click(ui, k);
  click(ui, [...ui.doc.querySelectorAll('[data-lk="fpick"]')].find((o) => text(o) === 'Sor'));
  assert.ok(k.classList.contains(QUIET));
  over(ui, ui.doc.querySelector('.lcc-k-strip .lcc-k-lens'));
  assert.ok(k.classList.contains(QUIET), 'a mouse-over of something else changes nothing');
  over(ui, k);
  assert.ok(!k.classList.contains(QUIET), 'the mouse comes over the key: the text comes back');
  // and again with the keyboard
  click(ui, k);
  click(ui, [...ui.doc.querySelectorAll('[data-lk="fpick"]')].find((o) => text(o) === 'Sor'));
  assert.ok(k.classList.contains(QUIET));
  key(ui, k, 'ArrowDown');
  assert.ok(!k.classList.contains(QUIET), 'a key press: the text comes back');
});

test('#1134: Done and Escape in Find House close it the same way; Escape from the search box keeps the key quiet', async () => {
  const ui = await open();
  const k = findKey(ui);
  click(ui, k);
  click(ui, '[data-lk="fclose"]');
  assert.equal(ui.doc.activeElement, k);
  assert.ok(k.classList.contains(QUIET), 'Done');
  key(ui, k, 'Tab');
  assert.ok(!k.classList.contains(QUIET));
  click(ui, k);
  key(ui, ui.doc.querySelector('[data-lk="fsearch"]'), 'Escape');
  assert.equal(ui.doc.querySelector('#lcc-bd-find').hidden, true);
  assert.ok(k.classList.contains(QUIET), 'Escape: the key press that closed the list must not count as the press that wakes the text');
});

test('#1134: the Filter key has the same pattern and the same fix (Done and Escape give it the focus back)', async () => {
  const ui = await open();
  const f = ui.doc.querySelector('[data-lk="fbtn"]');
  assert.ok(f.classList.contains('lcc-k-tipped'));
  click(ui, f);
  click(ui, '[data-lk="popclose"]');
  assert.equal(ui.doc.activeElement, f);
  assert.ok(f.classList.contains(QUIET), 'Done');
  over(ui, f);
  assert.ok(!f.classList.contains(QUIET));
  click(ui, f);
  key(ui, ui.doc.querySelector('#lcc-bd-menu'), 'Escape');
  assert.ok(f.classList.contains(QUIET), 'Escape');
});

test('#1134: moving the focus away clears it, so the key shows its text again when it is reached by the keyboard later', async () => {
  const ui = await open();
  const k = findKey(ui);
  click(ui, k);
  click(ui, [...ui.doc.querySelectorAll('[data-lk="fpick"]')].find((o) => text(o) === 'Sor'));
  assert.ok(k.classList.contains(QUIET));
  k.blur();
  assert.ok(!k.classList.contains(QUIET));
});
