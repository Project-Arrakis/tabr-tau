// #1146: the found row of the List is marked as ONE row. The outline is on the row (tr), not on each of its cells, so there are no lines between the cells. The 5x5 card flash is not changed.
// jsdom has no layout: the DOM (which element carries the mark) and the style file (the rule, the colour token, forced colours) are pinned here; the pictures are in the pull request.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const CSS = fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', 'lcc', 'lcc-board.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };

// every rule with a flash selector, at any depth (a rule inside @media is found too): { sel, body }
const flashRules = () => [...CSS.matchAll(/([^{}]*lcc-k-flash[^{}]*)\{([^{}]*)\}/g)].map((m) => ({ sel: m[1].trim(), body: m[2].trim() }));

test('#1146: Find House in the List marks the row (one element) and no cell of it', async () => {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'list' } });
  await ui.settle();
  ui.w.HTMLElement.prototype.scrollIntoView = function () {};
  click(ui, '[data-lk="findbtn"]');
  const o = [...ui.doc.querySelectorAll('[data-lk="fpick"]')].find((n) => text(n) === 'Sor');
  assert.ok(o, 'the entry for Sor');
  click(ui, o);
  const marked = [...ui.doc.querySelectorAll('.lcc-k-flash')];
  assert.equal(marked.length, 1, 'one element is marked: ' + marked.map((n) => n.tagName + '.' + n.className).join(' '));
  assert.equal(marked[0].tagName, 'TR');
  assert.ok(marked[0].classList.contains('lcc-bd-row'), 'the row');
  assert.equal(marked[0].querySelectorAll('td').length > 1, true, 'a row of several cells');
  assert.equal(marked[0].querySelectorAll('td.lcc-k-flash, td [class*="lcc-k-flash"]').length, 0, 'no cell carries the mark');
});

test('#1146: the style file outlines the row, not its cells, in the focus colour token, with an outline (it survives forced colours)', () => {
  const rules = flashRules();
  const row = rules.filter((r) => r.sel === '.lcc-bd-row.lcc-k-flash');
  assert.equal(row.length, 1, 'one rule for the row: ' + JSON.stringify(rules));
  assert.match(row[0].body, /outline: 3px solid var\(--lcc-focus\)/, 'the same 3 px line in the same colour token as before');
  assert.match(row[0].body, /outline-offset: -3px/);
  assert.doesNotMatch(row[0].body, /#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i, 'no literal colour: every theme sets --lcc-focus');
  assert.doesNotMatch(row[0].body, /box-shadow|border|animation|transition/, 'an outline only: a shadow is dropped by forced colours, and there is no motion');
  for (const r of rules) assert.doesNotMatch(r.sel, /lcc-bd-row[^,]*lcc-k-flash\s*>\s*td|lcc-bd-row[^,]*lcc-k-flash\s+td|\btd\b[^,]*lcc-k-flash/, 'no rule marks the cells: ' + r.sel);
  // the grid card flash is as it was
  assert.ok(rules.some((r) => r.sel === '.lcc-bd-cell.lcc-k-flash > .lcc-hc' && /outline: 3px solid var\(--lcc-focus\); outline-offset: -3px/.test(r.body)), 'the 5x5 card flash is unchanged');
});

test('#1146: no forced-colours rule takes the outline of the found row away, and its colour token is defined for every palette', () => {
  const forced = [...CSS.matchAll(/@media \(forced-colors: active\) \{((?:[^{}]|\{[^{}]*\})*)\}/g)].map((m) => m[1]).join('\n');
  assert.doesNotMatch(forced, /lcc-k-flash/, 'forced colours keep the plain outline, which the browser draws in a system colour');
  const lcc = fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', 'lcc', 'lcc.css'), 'utf8');
  assert.match(lcc, /--lcc-focus:\s*var\(--c-focus\)/, 'the board token is the page token');
  const pal = fs.readFileSync(path.join(here, '..', 'design', 'assets', 'palettes.css'), 'utf8');
  assert.ok((pal.match(/--c-focus:/g) || []).length >= 20, 'every palette (light and dark) sets --c-focus');
});
