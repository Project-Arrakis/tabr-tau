// The board's mouse-overs and two wording fixes (tau #975, #977, #979, #978), in the page's own code (jsdom), with the synthetic answer of M1 and no game save.
// #975: a House square of the 5x5 board shows the same House hover card as the list rows (one builder, LCC_HOST.card), on the pointer and on the keyboard focus; its tier boxes keep their own mouse-over.
// #977: the reward mouse-over shows the item picture and the display name, never the internal name; pictures are local files from a fixed list; a reward with no picture shows the name only.
// #979: Find House (was Jump to House) lists House names only, A to Z. #978: the WON tag of the victory panel takes the one win outline of lcc.css (solid, never dashed).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { root as repoRoot, lccDir } from './lcc-load.mjs';

afterEach(closeAll);

const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
const src = (...p) => fs.readFileSync(path.join(repoRoot, 'internal', 'web', 'static', ...p), 'utf8').replace(/\r/g, '');
const CATALOG = [
  { id: 'SolarisCoin', name: 'Solari' }, { id: 'T6Watertube', name: 'Improved Watertube' }, { id: 'Alexin_Placeables_Swatch', name: 'House Alexin Placeables Swatch' },
  { id: 'SYNTH_SCHEMATIC_1', name: 'Synthetic Schematic' },
];
// the answer with real item ids in the tiers of House Alexin: tier 2 an item that has a picture, tier 3 a House swatch (no picture in the console), tier 1 stays Solari
function answer() {
  const a = synthetic();
  const t = a.tasks.find((x) => x.house_name === 'DA_HouseAlexin');
  for (const [i, id] of [[1, 'T6Watertube'], [2, 'Alexin_Placeables_Swatch']]) { t.rewards[i].template_id = id; t.rewards[i].shown_template_id = id; t.rewards[i].swatch = id.endsWith('Swatch'); }
  return a;
}
async function open(layout = 'grid', view = 'board', a = answer()) {
  const ui = await boot({ get: { '/api/landsraad': a, '/api/catalog/items': CATALOG }, post: {}, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': layout } });
  await ui.settle();
  return ui;
}
const cellOf = (ui, a, name) => ui.doc.querySelector('.lcc-bd-cell[data-id="' + idOf(a, name) + '"]');
const hover = (ui, el) => el.dispatchEvent(new ui.w.MouseEvent('mouseover', { bubbles: true }));
const tip = (ui) => ui.doc.querySelector('.lcc-rtip');

// ---- #975 ------------------------------------------------------------------------------------------------------------------------------------------------------------------
test('#975: every square of the board has the House hover card of the list rows, beside the card of the square, from the one builder', async () => {
  const a = answer();
  const ui = await open('grid');
  const cells = [...ui.doc.querySelectorAll('.lcc-bd-cell')];
  assert.equal(cells.length, 25);
  for (const c of cells) {
    assert.ok(c.classList.contains('hw'), 'the cell is the wrapper of the hover card, as the name cell of a list row is');
    const card = c.querySelector(':scope > .hcard');
    assert.ok(card, 'a hover card beside the square');
    assert.equal(c.querySelector('.lcc-hc .hcard'), null, 'not inside the House card: its clip path and filter would cut or move it');
    assert.ok(c.firstElementChild.classList.contains('lcc-hc'), 'the card is the first child: the page places the hover card from it');
  }
  // the same words as the list's card for the same House
  const list = await open('list', 'board', a);
  for (const name of ['Alexin', 'Kenola', 'Wallach']) {
    const row = [...list.doc.querySelectorAll('.lcc-bd-row')].find((r) => text(r.querySelector('.lcc-bd-namebtn')) === name);
    assert.equal(text(cellOf(ui, a, name).querySelector(':scope > .hcard')), text(row.querySelector('.hw > .hcard')), name + ': the square and the list row say the same');
  }
  const t = text(cellOf(ui, a, 'Alexin').querySelector(':scope > .hcard'));
  for (const w of ['House Alexin', 'Next reward', 'Not collected', 'Faction standing', 'Place']) assert.ok(t.includes(w), w + ' in ' + t);
  // the place line is the real button of the list's card (the link to the Saved map)
  assert.ok(cellOf(ui, a, 'Alexin').querySelector(':scope > .hcard .hcard-place[data-lb="map"]'));
});

test('#975: the square keeps its click (look at the House), loses its native text for the card, and the card is placed on the pointer and on the keyboard focus', async () => {
  const a = answer();
  const ui = await open('grid');
  const c = cellOf(ui, a, 'Alexin');
  const sel = c.querySelector('.lcc-hc-select');
  assert.equal(sel.getAttribute('title'), null, 'one overlay, not two: the hover card is the mouse-over');
  assert.match(sel.getAttribute('data-says'), /^House Alexin, /, 'the words stay as a data attribute');
  assert.match(sel.getAttribute('aria-label'), /Alexin/, 'and the accessible name is unchanged');
  hover(ui, sel);
  assert.ok(c.style.getPropertyValue('--fx') !== '', 'the page places the card next to the square on the pointer');
  c.style.removeProperty('--fx');
  sel.dispatchEvent(new ui.w.FocusEvent('focusin', { bubbles: true }));
  assert.ok(c.style.getPropertyValue('--fx') !== '', 'and on the keyboard focus');
  click(ui, sel);
  assert.equal(text(ui.doc.querySelector('.lcc-bd-insp-name')), 'House Alexin', 'a click still looks at the House (touch has no hover: the Inspector stays the way in)');
});

test('#975: Escape closes the card of a square, and the style sheets open it only on the pointer or the focus, and step aside for the tier boxes', async () => {
  const a = answer();
  const ui = await open('grid');
  const c = cellOf(ui, a, 'Kenola');
  const sel = c.querySelector('.lcc-hc-select');
  sel.focus();
  sel.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.ok(c.hasAttribute('data-esc'), 'the card is closed for as long as the pointer or the focus stays');
  const app = src('app.css');
  assert.match(app, /\.lsb \.hw:hover>\.hcard\{display:block\}/);
  assert.match(app, /\.lsb \.hw:has\(:focus-visible\)>\.hcard/);
  assert.match(app, /\.lsb \.hw\[data-esc\]>\.hcard\{display:none!important\}/);
  const css = src('lcc', 'lcc-board.css');
  assert.match(css, /\.lcc-bd \.lcc-bd-cell\.hw \{ display: grid; \}/, 'the cell stays a grid, so the squares keep their height');
  assert.match(css, /\.lcc-bd \.lcc-bd-cell\.hw:has\(\.lcc-hc-tiers:hover\) > \.hcard \{ display: none !important; \}/, 'the tier boxes keep their own mouse-over');
});

// ---- #977 ------------------------------------------------------------------------------------------------------------------------------------------------------------------
test('#977: a tier box of a square shows the item picture and its display name on hover, never the internal name', async () => {
  const a = answer();
  const ui = await open('grid');
  const c = cellOf(ui, a, 'Alexin');
  const boxes = [...c.querySelectorAll('.lcc-hc-t')];
  assert.equal(boxes.length, 5);
  // tier 1 is Solari: no picture mouse-over, a native text without the id
  assert.equal(boxes[0].hasAttribute('data-rtip'), false);
  assert.doesNotMatch(boxes[0].getAttribute('title'), /SolarisCoin|Game id/);
  // tier 2 has a picture
  const b = boxes[1];
  assert.equal(b.getAttribute('title'), null, 'no native text: it cannot hold a picture');
  assert.equal(b.getAttribute('data-item'), 'T6Watertube');
  assert.doesNotMatch(b.getAttribute('aria-label'), /T6Watertube|Game id/);
  assert.match(b.getAttribute('aria-label'), /Reward: 1 Improved Watertube\./);
  assert.equal(tip(ui).hidden, true, 'closed until the pointer comes');
  hover(ui, b);
  const t = tip(ui);
  assert.equal(t.hidden, false);
  const img = t.querySelector('img.lcc-rtip-img');
  assert.ok(img, 'the picture');
  assert.equal(img.getAttribute('src'), '/lcc/art/items/T6Watertube.webp', 'a file of this program, never another address');
  assert.equal(img.getAttribute('alt'), '');
  assert.equal(text(t.querySelector('.lcc-rtip-name')), 'Improved Watertube');
  assert.match(text(t), /Quantity 1/);
  assert.match(text(t), /Tier 2 needs [\d,]+: (not reached|earned, .+)/);
  assert.doesNotMatch(text(t), /T6Watertube|Game id|SYNTH_/, 'no internal name in what is shown');
  assert.equal(t.getAttribute('aria-hidden'), 'true', 'the control says the same words to a screen reader');
  // it goes away on leaving, on Escape, and on a click
  hover(ui, ui.doc.querySelector('.lcc-bd-victory'));
  assert.equal(t.hidden, true);
  hover(ui, b); assert.equal(t.hidden, false);
  b.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.equal(t.hidden, true);
});

test('#977: a reward with no picture shows the name only, and the keyboard focus shows the same mouse-over', async () => {
  const a = answer();
  const ui = await open('grid');
  const b = [...cellOf(ui, a, 'Alexin').querySelectorAll('.lcc-hc-t')][2]; // the House swatch
  assert.equal(b.getAttribute('data-item'), 'Alexin_Placeables_Swatch');
  b.dispatchEvent(new ui.w.FocusEvent('focusin', { bubbles: true }));
  const t = tip(ui);
  assert.equal(t.hidden, false, 'the focus shows it, as the pointer does');
  assert.equal(t.querySelector('img'), null, 'the 25 House swatches have no console picture: no picture and no invented one');
  assert.equal(text(t.querySelector('.lcc-rtip-name')), 'House Alexin Placeables Swatch');
  assert.match(text(t), /the house swatch/);
  assert.doesNotMatch(text(t), /Alexin_Placeables_Swatch/);
  b.dispatchEvent(new ui.w.FocusEvent('focusout', { bubbles: true }));
  assert.equal(t.hidden, true);
});

test('#977: the inspector rows and the list cells show the same picture mouse-over, and no text of the board holds an internal item name', async () => {
  const a = answer();
  const ui = await open('grid');
  click(ui, '[data-fk="ht:' + idOf(a, 'Alexin') + '"]');
  const rows = [...ui.doc.querySelectorAll('.lcc-bd-insp .lcc-bd-tier')];
  assert.equal(rows.length, 5);
  assert.equal(rows[0].hasAttribute('data-rtip'), false, 'Solari keeps no mouse-over');
  const d = ui.w.LCCB.tipOf(rows[1]);
  assert.equal(d.rows[0].name, 'Improved Watertube');
  assert.equal(rows[1].getAttribute('title'), null);
  hover(ui, rows[1]);
  assert.ok(tip(ui).querySelector('img[src="/lcc/art/items/T6Watertube.webp"]'));
  // no internal item name in any title, aria-label, data-why or visible text of the board (the data-item attribute is the one place it is kept)
  const board = ui.doc.querySelector('#lsb');
  const said = [text(board)];
  for (const e of board.querySelectorAll('*')) for (const k of ['title', 'aria-label', 'data-why', 'data-says']) if (e.getAttribute(k)) said.push(e.getAttribute(k));
  assert.doesNotMatch(said.join(' | '), /T6Watertube|Alexin_Placeables_Swatch|Game id/);
  // the list
  const list = await open('list', 'board', a);
  const row = [...list.doc.querySelectorAll('.lcc-bd-row')].find((r) => text(r.querySelector('.lcc-bd-namebtn')) === 'Alexin');
  const cell = row.querySelector('.lcc-bd-c-tiers');
  const lt = list.w.LCCB.tipOf(cell);
  assert.equal(lt.rows.length, 5, 'one row per tier');
  assert.deepEqual(lt.rows.map((r) => r.item), ['', 'T6Watertube', 'Alexin_Placeables_Swatch', lt.rows[3].item, lt.rows[4].item]);
  assert.equal(cell.getAttribute('title'), null);
  hover(list, cell);
  const t = tip(list);
  assert.equal(t.querySelectorAll('.lcc-rtip-row').length, 5);
  assert.equal(t.querySelectorAll('img').length >= 1, true);
  assert.doesNotMatch(text(t), /T6Watertube|Alexin_Placeables_Swatch|SolarisCoin/);
  assert.match(text(list.doc.querySelector('#lsb')), /\S/);
});

test('#977: every reward picture is a local file of the fixed list: the list, the manifest and the folder are the same files, and each is a real item of the editor', async () => {
  const L = (await import('./lcc-load.mjs')).default || null;
  void L;
  const ui = await open('grid');
  const pics = [...ui.w.LCC.ITEM_PICS].sort();
  const manifest = JSON.parse(fs.readFileSync(path.join(repoRoot, 'docs', 'design', 'landsraad-command-center', 'art-manifest.json'), 'utf8'));
  const listed = manifest.files.filter((f) => f.role === 'item').map((f) => f.file.replace(/^items\//, '').replace(/\.webp$/, '')).sort(); // item rows carry no key field: a long id beside a hash reads as a secret to gitleaks
  const onDisk = fs.readdirSync(path.join(lccDir, 'art', 'items')).map((f) => f.replace(/\.webp$/, '')).sort();
  assert.deepEqual(pics, listed);
  assert.deepEqual(pics, onDisk);
  assert.ok(pics.length >= 90);
  const items = new Set(JSON.parse(fs.readFileSync(path.join(repoRoot, 'internal', 'catalog', 'items.json'), 'utf8')).map((i) => i.id));
  for (const id of pics) assert.ok(items.has(id), id + ' is an item of the editor');
  for (const f of manifest.files.filter((x) => x.role === 'item')) {
    assert.equal(f.width, 64); assert.equal(f.height, 64);
    assert.match(f.file, /^items\/[A-Za-z0-9_]+\.webp$/);
    assert.match(f.source, /Dune Docker console item picture/);
  }
  // a lookup answers a path of the list or nothing, never a path built from the input
  const A = ui.w.LCC.itemAsset;
  assert.equal(A('T6Watertube').src, '/lcc/art/items/T6Watertube.webp');
  for (const bad of ['', 'SolarisCoin', '../app', 'T6Watertube/../x', 't6watertube', 'Alexin_Placeables_Swatch', null, undefined, 7, {}, ['T6Watertube'], '__proto__']) assert.equal(A(bad), null, String(bad));
  // the page never reaches for a picture outside its own address
  for (const id of pics) assert.equal(A(id).src, '/lcc/art/items/' + id + '.webp');
});

test('#977: the source says the internal name is nowhere in the mouse-overs: no Game id in the page or the board', () => {
  assert.doesNotMatch(src('app.js').split('\n').filter((l) => /^const lbTierTitle|const rid = /.test(l) || /function lbItemInfo/.test(l)).join('\n'), /Game id/);
  assert.doesNotMatch(src('app.js'), /'Game id: '/);
  assert.doesNotMatch(src('lcc', 'lcc-board.js'), /Game id/);
});

// ---- #979 ------------------------------------------------------------------------------------------------------------------------------------------------------------------
test('#979: Find House (the jump of the control area, tau #974) lists the House names only, A to Z, with no square number, and the jump still works', async () => {
  const a = answer();
  const ui = await open('grid');
  let scrolled = 0;
  ui.w.HTMLElement.prototype.scrollIntoView = function () { scrolled++; };
  click(ui, '[data-lk="findbtn"]');
  const opts = [...ui.doc.querySelectorAll('#lcc-bd-find [data-lk="fpick"]')];
  const names = opts.map((o) => text(o));
  assert.equal(names.length, 25);
  assert.deepEqual(names, [...names].sort((x, y) => x.localeCompare(y, 'en')), 'A to Z');
  for (const n of names) assert.doesNotMatch(n, /\d|·/, 'no square number: ' + n);
  const opt = opts.find((o) => text(o) === 'Wallach');
  assert.ok(opt, 'Wallach is in the list');
  assert.equal(opt.getAttribute('data-id'), String(idOf(a, 'Wallach')), 'the value is still the House');
  click(ui, opt);
  assert.ok(scrolled >= 1, 'the board scrolled to the House');
  assert.ok(cellOf(ui, a, 'Wallach').classList.contains('lcc-k-flash'), 'the House is outlined');
  // the square stays in the aria text of the card and in the list's Square column
  assert.match(cellOf(ui, a, 'Wallach').querySelector('.lcc-hc-select').getAttribute('aria-label'), /^Square 08, Wallach/);
});

// ---- #978 ------------------------------------------------------------------------------------------------------------------------------------------------------------------
const strip = (f) => fs.readFileSync(path.join(lccDir, f), 'utf8').replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
test('#978: the WON tag takes the one win outline of lcc.css (solid, with the glow and the pulse of the Houses), and nothing dashed is left on a won state', () => {
  const base = strip('lcc.css'), board = strip('lcc-board.css');
  const tags = '.lcc-bd-victory[data-state="line"] .lcc-bd-vtag, .lcc-bd-victory[data-state="count"] .lcc-bd-vtag';
  // the same rule as the winning House (one selector list, one outline token, one glow dial): no copy
  assert.ok(base.includes('.lcc-house[data-victory="win"], .lcc-sp-lane[data-victory="win"], ' + tags + ' { outline: var(--lcc-outline-win); outline-offset: 3px; box-shadow: 0 0 0 2px var(--lcc-panel), 0 0 20px 6px color-mix(in srgb, var(--lcc-gold) calc(80% * var(--lcc-glow-win)), transparent);'));
  // no dashed line for a won tag; the dashed rule that was there is gone, and the other states keep theirs
  assert.doesNotMatch(board, /data-state="count"\] \.lcc-bd-vtag \{[^}]*dashed/);
  assert.doesNotMatch(board, /data-state="line"\] \.lcc-bd-vtag \{[^}]*dashed/);
  assert.match(board, /data-state="conflict"\] \.lcc-bd-vtag \{ border-style: dashed;/);
  // the pulse: the Houses' own keyframes, only with Motion Full and no reduced-motion request; the reveal does not run on it
  assert.match(base, /@media \(prefers-reduced-motion: no-preference\) \{\n  html\[data-motion="full"\] \.lcc-bd-victory\[data-state="line"\] \.lcc-bd-vtag, html\[data-motion="full"\] \.lcc-bd-victory\[data-state="count"\] \.lcc-bd-vtag \{ animation: lcc-win-pulse 5s ease-in-out infinite; \}\n\}/);
  assert.ok(base.includes(tags + ' { animation: none; }'), 'no reveal: a still glow in every other case');
  assert.equal((base.match(/@keyframes lcc-win-pulse/g) || []).length, 1, 'one set of keyframes');
  // forced colours keep a solid double border and an outline, with no colour
  assert.match(board, /@media \(forced-colors: active\) \{[^]*\.lcc-bd-victory\[data-state="line"\] \.lcc-bd-vtag, \.lcc-bd-victory\[data-state="count"\] \.lcc-bd-vtag \{ border: 3px double CanvasText; outline: 2px solid CanvasText; outline-offset: 3px; box-shadow: none; \}/);
});

test('#978: the tag keeps its words, and only the won states carry the outline', async () => {
  for (const [state, tagText] of [['line', 'SYSSELRAAD SECURED'], ['count', 'SYSSELRAAD SECURED'], ['contested', 'SYSSELRAAD CONTESTED']]) {
    const a = answer();
    // a board in each state: a held row of five Atreides Houses (line), 13 Atreides Houses spread out (count), and the synthetic one (contested)
    const tasks = [...a.tasks].sort((x, y) => x.board_index - y.board_index);
    const setOwner = (t, f) => { t.completed = f !== 0 ? 1 : 0; t.winning_faction_id = f === 0 ? null : f; t.sysselraad = 0; };
    tasks.forEach((t) => setOwner(t, 0));
    if (state === 'line') tasks.slice(0, 5).forEach((t) => setOwner(t, 1));
    if (state === 'count') [0, 1, 2, 3, 5, 6, 8, 9, 10, 11, 13, 14, 16].forEach((i) => setOwner(tasks[i], 1));
    if (state === 'contested') tasks.slice(0, 3).forEach((t) => setOwner(t, 1));
    const ui = await open('grid', 'board', a);
    const p = ui.doc.querySelector('.lcc-bd-victory');
    assert.equal(p.getAttribute('data-state'), state);
    assert.equal(text(p.querySelector('.lcc-bd-vtag')), tagText, 'the words of WIN.TEXT, unchanged');
  }
});
