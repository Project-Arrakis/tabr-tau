// Keyboard and screen-reader access (#180 UX-08, #22): the cells of the land claim grid are buttons, every field has a name, and no handler is left that nothing can reach.
// The real page code runs in jsdom over the generated fixtures (see boot.mjs).
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const appJs = fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', 'app.js'), 'utf8');
test.afterEach(() => closeAll());

const BASE7 = [{ base_id: 7, name: 'Base', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 0, placeables: 0, x: 0, y: 0, z: 0 }];
const CLAIM = [{ totem_id: 7, name: 'Totem', cells: 2, rings: 0, level: 1, irregular: true, maxRings: 5, maxLevel: 5, yaw: 90, segments: [{ x: 1, y: 0 }], grid: [{ x: 0, y: 0, in: true, n: 0 }, { x: 1, y: 0, in: true, n: 3 }] }];

async function claimEditor() {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': CLAIM } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="claim"]'); await ui.settle();
  return ui;
}
const key = (ui, el, k) => el.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }));

test('a cell of the land claim grid can be reached with Tab and pressed with Enter or Space, and keeps the focus', async () => {
  const ui = await claimEditor();
  const cell = (k) => ui.doc.querySelector(`[data-cell="${k}"]`);
  const c = cell('2,0');
  assert.equal(c.getAttribute('tabindex'), '0', 'it is in the tab order');
  assert.equal(c.getAttribute('role'), 'button');
  assert.equal(c.getAttribute('aria-pressed'), 'false');
  assert.match(c.getAttribute('aria-label'), /^Cell 2, 0: available\. Press to add it\.$/);
  c.focus();
  key(ui, c, 'Enter'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('rect.lc-s').length, 1, 'Enter added the cell');
  assert.equal(ui.doc.activeElement, cell('2,0'), 'the focus is on the same cell after the grid was drawn again');
  assert.equal(cell('2,0').getAttribute('aria-pressed'), 'true');
  assert.match(cell('2,0').getAttribute('aria-label'), /chosen to be added\. Press to take it back\./);
  key(ui, cell('2,0'), ' '); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('rect.lc-s').length, 0, 'Space took it back');
  assert.equal(ui.doc.activeElement, cell('2,0'));
  // the existing cell of the claim that holds nothing can be marked for removal the same way; one that holds pieces is not offered
  assert.equal(cell('1,0'), null, 'a cell that holds pieces cannot be pressed');
  ui.dom.window.close();
});

test('a key other than Enter or Space does not press a cell', async () => {
  const ui = await claimEditor();
  const c = ui.doc.querySelector('[data-cell="2,0"]');
  key(ui, c, 'a'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('rect.lc-s').length, 0);
  ui.dom.window.close();
});

// A name is what a screen reader says for a field: aria-label, aria-labelledby, a wrapping label or a label for it. A placeholder is not one.
const named = (el) => !!(el.getAttribute('aria-label') || el.getAttribute('aria-labelledby') || el.closest('label') || (el.id && el.ownerDocument.querySelector(`label[for="${el.id}"]`)));

test('every field on every tab and sub-view has an accessible name', async () => {
  const ui = await boot({});
  const bad = new Set();
  const scan = (where) => {
    for (const el of ui.doc.querySelectorAll('input, select, textarea')) {
      if (el.type === 'hidden' || el.type === 'button' || named(el)) continue;
      bad.add(`${where}: ${el.outerHTML.slice(0, 110)}`);
    }
  };
  for (const t of ['player', 'livemap', 'landsraad', 'config', 'db']) {
    ui.click(`[data-tab="${t}"]`); await ui.settle(); scan(t);
    if (t !== 'player') continue;
    for (const s of ['character', 'crafting', 'research', 'buildingsets', 'customizations', 'skills', 'specialization', 'journey', 'bases', 'vehicles', 'admin']) {
      ui.click(`[data-sub="player:${s}"]`); await ui.settle(); scan('player:' + s);
      for (const c of [...ui.doc.querySelectorAll('[data-sub^="char:"]')]) { ui.click(`[data-sub="${c.getAttribute('data-sub')}"]`); await ui.settle(); scan(c.getAttribute('data-sub')); }
    }
  }
  assert.deepEqual([...bad], [], 'fields without a name');
  ui.dom.window.close();
});

test('the inline cells of the config and database tables name their row', () => {
  assert.match(appJs, /<input class="cfgv" aria-label="Value of \$\{e\.key\}"/);
  assert.match(appJs, /<input class="dbc" aria-label="\$\{cols\[i\]\}, row \$\{r\[0\]\}"/);
});

test('every handler in the action table can be reached by a button, or is run by name', () => {
  const i = appJs.indexOf('\nconst A = {');
  const j = appJs.indexOf('\n};', i);
  const body = appJs.slice(i, j);
  const dead = [...body.matchAll(/^ {2}(\w+):/gm)].map((m) => m[1]).filter((k) => !appJs.includes(`data-act="${k}"`) && !new RegExp(`['"]${k}['"]`).test(appJs));
  assert.deepEqual(dead, [], 'handlers nothing can call');
});
