// Tooltips made by tip() (#536). A tooltip is drawn with position:fixed from the rectangle of its control, so a scroll box around a table neither clips it nor grows a scrollbar
// for it. placeTip() is the pure part (rectangles in, position out); the rest checks that hover, focus, Escape, scroll and resize open and close the one tooltip, and that the
// style sheet draws it outside the flow. jsdom lays nothing out, so the rectangles are given by the test.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

test.afterEach(() => closeAll());

const css = fs.readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'internal', 'web', 'static', 'app.css'), 'utf8');
const rule = (sel) => { const m = css.split(/\r?\n/).find((l) => l.startsWith(sel + '{')); assert.ok(m, `no rule for ${sel}`); return m; };

const VIEW = { width: 1000, height: 700 };
const BOX = { width: 300, height: 80 };
const rect = (left, top, w = 100, h = 24) => ({ left, top, right: left + w, bottom: top + h });

test('placeTip: below the control by default, with a 2 px overlap so the pointer can travel into it', async () => {
  const ui = await boot();
  const p = ui.w.placeTip(rect(200, 100), BOX, VIEW);
  assert.deepEqual({ ...p }, { left: 200, top: 122, side: 'below' });
});

test('placeTip: flips above when it would run past the bottom and there is more room above', async () => {
  const ui = await boot();
  const p = ui.w.placeTip(rect(200, 660), BOX, VIEW); // the last row of a long table, near the bottom edge
  assert.equal(p.side, 'above');
  assert.equal(p.top, 660 + 2 - BOX.height);
  assert.ok(p.top + BOX.height <= 660 + 2, 'ends where the control starts');
});

test('placeTip: stays below when it does not fit above either and below has more room', async () => {
  const ui = await boot();
  const p = ui.w.placeTip(rect(200, 10), { width: 300, height: 900 }, VIEW); // taller than the window: the CSS max-height cuts it
  assert.equal(p.side, 'below');
  assert.equal(p.top, 8, 'clamped to the top margin');
});

test('placeTip: clamped to the right edge and to the left edge', async () => {
  const ui = await boot();
  assert.equal(ui.w.placeTip(rect(950, 100), BOX, VIEW).left, VIEW.width - BOX.width - 8, 'a control at the right edge');
  assert.equal(ui.w.placeTip(rect(-40, 100), BOX, VIEW).left, 8, 'a control scrolled partly off the left edge');
  assert.equal(ui.w.placeTip(rect(300, 100), { width: 1200, height: 40 }, VIEW).left, 8, 'a box wider than a narrow window starts at the margin');
});

test('placeTip: a narrow window keeps the whole box inside it', async () => {
  const ui = await boot();
  const narrow = { width: 320, height: 568 };
  const p = ui.w.placeTip(rect(260, 500), { width: 280, height: 120 }, narrow);
  assert.ok(p.left >= 8 && p.left + 280 <= narrow.width - 8, `left ${p.left}`);
  assert.ok(p.top >= 8 && p.top + 120 <= narrow.height - 8, `top ${p.top}`);
});

test('the style sheet draws the tooltip outside the flow of the scroll box', () => {
  const box = rule('.tipbox');
  assert.match(box, /position:fixed/, 'fixed: not clipped by overflow:auto and not part of the scroll size');
  assert.doesNotMatch(box, /position:absolute/);
  assert.doesNotMatch(css, /\.tip:hover \.tipbox|\.tip:focus-within \.tipbox/, 'the script opens it, so its place is always set first');
  assert.match(css, /\.tip\.open \.tipbox\{display:block\}/);
  assert.doesNotMatch(box, /transition|animation/, 'no motion to turn off for prefers-reduced-motion');
});

const BASE = (id, extra = {}) => ({ base_id: id, name: 'Base ' + id, base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 0, placeables: 0, x: 0, y: 0, z: 0, ...extra });
const openBases = async (get) => {
  const ui = await boot({ get });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  // the page has no layout in jsdom: the control sits where the test says, the tooltip has a fixed size
  ui.at = { left: 400, top: 300 };
  ui.w.HTMLElement.prototype.getBoundingClientRect = function () {
    if (this.classList.contains('tipbox')) return { left: 0, top: 0, right: 300, bottom: 80, width: 300, height: 80 };
    if (this.classList.contains('tip')) return { ...rect(ui.at.left, ui.at.top), width: 100, height: 24 };
    return { left: 0, top: 0, right: 0, bottom: 0, width: 0, height: 0 };
  };
  ui.w.innerWidth = 1000; ui.w.innerHeight = 700;
  return ui;
};
const fire = (ui, el, type, init = {}) => el.dispatchEvent(new ui.w.MouseEvent(type, { bubbles: true, cancelable: true, ...init }));
const ev = (ui, el, type, init = {}) => el.dispatchEvent(new ui.w.Event(type, { bubbles: true, cancelable: true, ...init }));
const LIST = [BASE(7), BASE(8), BASE(9)];

test('hovering Export for console opens its tooltip beside the button and leaving closes it; the table is not touched', async () => {
  const ui = await openBases({ '/api/bases/list': LIST, '/api/bases/backups': [] });
  const wraps = [...ui.doc.querySelectorAll('.scroll .tip')];
  assert.equal(wraps.length, 3, 'one tooltip wrapper per base, all inside the table box');
  const strip = (s) => s.replace(/ style="[^"]*"/g, '').replace(/ class="tip open"/g, ' class="tip"');
  const before = strip(ui.doc.querySelector('.scroll').innerHTML);
  const w = wraps[0], box = w.querySelector('.tipbox');
  fire(ui, w.querySelector('button'), 'mouseover');
  assert.ok(w.classList.contains('open'));
  assert.equal(box.style.left, '400px');
  assert.equal(box.style.top, '322px', 'below the button');
  assert.equal(ui.doc.querySelectorAll('.tip.open').length, 1);
  assert.equal(strip(ui.doc.querySelector('.scroll').innerHTML), before, 'only the tooltip changed; no row or cell did');
  fire(ui, w.querySelector('button'), 'mouseout', { relatedTarget: ui.doc.body });
  assert.ok(!w.classList.contains('open'));
  ui.dom.window.close();
});

test('the tooltip of the last row opens above the button when there is no room below', async () => {
  const ui = await openBases({ '/api/bases/list': LIST, '/api/bases/backups': [] });
  const w = [...ui.doc.querySelectorAll('.scroll .tip')].pop();
  ui.at = { left: 400, top: 670 };
  fire(ui, w.querySelector('button'), 'mouseover');
  assert.equal(w.querySelector('.tipbox').style.top, String(670 + 2 - 80) + 'px');
  ui.dom.window.close();
});

test('keyboard focus opens the tooltip and blur closes it; the control keeps aria-describedby', async () => {
  const ui = await openBases({ '/api/bases/list': LIST, '/api/bases/backups': [] });
  const w = ui.doc.querySelector('.scroll .tip'), b = w.querySelector('button'), box = w.querySelector('.tipbox');
  assert.equal(box.getAttribute('role'), 'tooltip');
  assert.equal(b.getAttribute('aria-describedby'), box.id);
  b.focus();
  assert.ok(w.classList.contains('open'), 'focus opens it');
  b.blur();
  assert.ok(!w.classList.contains('open'), 'blur closes it');
  ui.dom.window.close();
});

test('Escape, a scroll and a resize put the tooltip away; moving back in opens it again', async () => {
  const ui = await openBases({ '/api/bases/list': LIST, '/api/bases/backups': [] });
  const w = ui.doc.querySelector('.scroll .tip'), b = w.querySelector('button');
  fire(ui, b, 'mouseover');
  ui.doc.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.ok(!w.classList.contains('open'), 'Escape');
  fire(ui, b, 'mouseover');
  assert.ok(!w.classList.contains('open'), 'it stays away while the pointer stays on the control');
  fire(ui, b, 'mouseout', { relatedTarget: ui.doc.body });
  fire(ui, b, 'mouseover');
  assert.ok(w.classList.contains('open'), 'it returns when the pointer leaves and comes back');
  ev(ui, ui.doc.querySelector('.scroll'), 'scroll');
  assert.ok(!w.classList.contains('open'), 'the table box scrolled');
  fire(ui, b, 'mouseout', { relatedTarget: ui.doc.body });
  fire(ui, b, 'mouseover');
  assert.ok(w.classList.contains('open'));
  ui.w.dispatchEvent(new ui.w.Event('resize'));
  assert.ok(!w.classList.contains('open'), 'the window was resized');
  ui.dom.window.close();
});

test('moving from the button into its tooltip keeps it open; opening another closes the first', async () => {
  const ui = await openBases({ '/api/bases/list': LIST, '/api/bases/backups': [] });
  const [a, b] = [...ui.doc.querySelectorAll('.scroll .tip')];
  fire(ui, a.querySelector('button'), 'mouseover');
  fire(ui, a.querySelector('button'), 'mouseout', { relatedTarget: a.querySelector('.tipbox') });
  assert.ok(a.classList.contains('open'), 'the pointer is on the tooltip, which is inside the wrapper');
  fire(ui, b.querySelector('button'), 'mouseover');
  assert.ok(!a.classList.contains('open') && b.classList.contains('open'), 'one at a time');
  ui.dom.window.close();
});

test('the Base backups card works the same', async () => {
  const BACKUP = { backup_id: 3, name: 'Sub-Fief', backup_name: '##Totem_Small_Placeable', map: 'HaggaBasin', actors: 44, picked_up: 44, pieces: 1174, placeables: 43, stored_items: 53 };
  const ui = await openBases({ '/api/bases/list': LIST, '/api/bases/backups': [BACKUP] });
  const w = ui.doc.querySelector('#bbctip3').closest('.tip');
  assert.ok(w.closest('.scroll'), 'inside a scroll box');
  fire(ui, w.querySelector('button'), 'mouseover');
  assert.ok(w.classList.contains('open'));
  ui.dom.window.close();
});
