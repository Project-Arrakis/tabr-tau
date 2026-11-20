// #1069: the quick peek of a House's place (the hover of the place in the Classic list and the Command Center list, the place peek under the Place line of the House card, the Inspector's place peek and the
// "Where" part of an opened Classic row) draws the REAL picture of the House's map, not the generic schematic octagon: a window of the picture around the pin, with the whole map small in a corner and
// the window marked on it. The pin is the same fraction of the picture as on the Saved map (x * Width, y * Height), so the test works the pin back out of what the page drew and compares it with the place
// the read gives, using the picture and the map space read from internal/ops/livemap.go (not copied here). The schematic stays only for a place outside the frame of its picture and for a map the editor has
// no picture of, and says why. The House hover card gets the large House symbol, the name and the region. Display only: nothing is written, no request is made for a picture until the peek is shown (the
// pictures are lazy, from this program's own address, never inlined). jsdom has no layout, so what is measured is what the page draws; the real browser look is in the headless sweep and the screenshots of the PR.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { root as repoRoot } from './lcc-load.mjs';

afterEach(closeAll);

const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const read = (...p) => fs.readFileSync(path.join(repoRoot, ...p), 'utf8').replace(/\r\n/g, '\n');

// the maps of the Saved map, read from the Go file that holds them: key, picture, map space
const FRAMES = Object.fromEntries([...read('internal', 'ops', 'livemap.go').matchAll(/\{Key: "(\w+)", Label: "[^"]*", Image: "([^"]+)", Width: (\d+), Height: (\d+)/g)].map((m) => [m[1], { image: m[2], w: Number(m[3]), h: Number(m[4]) }]));
const LABEL = { HaggaBasin: 'Hagga Basin', DeepDesert: 'Deep Desert', Arrakeen: 'Arrakeen', HarkoVillage: 'Harko Village' };
// a House on each map, and two near the corner of a picture: [house, map key, x, y]
const PLACES = [['Kenola', 'HaggaBasin', 0.25, 0.5], ['Vernius', 'DeepDesert', 0.6, 0.4], ['Alexin', 'Arrakeen', 0.6965799039003683, 0.37830772138395463], ['Richese', 'HarkoVillage', 0.5842211055276382, 0.567219430485],
  ['Lindaren', 'HaggaBasin', 0.97, 0.93], ['Mutelli', 'Arrakeen', 0.02, 0.03]];
function answer() {
  const a = synthetic();
  for (const t of a.tasks) t.location = { known: false };
  for (const [name, key, x, y] of PLACES) a.tasks.find((t) => t.house_name === 'DA_House' + name).location = { known: true, map: LABEL[key], map_key: key, area: LABEL[key], place: '', cell: '', unconfirmed: false, x, y, from_you_cm: 150000 };
  // outside the frame of the picture of a map the editor has (no map_key, no pin); a map the editor has no picture of; a key the page has no picture for
  a.tasks.find((t) => t.house_name === 'DA_HouseThorvald').location = { known: true, map: 'Hagga Basin', map_key: '', area: 'Hagga Basin', place: '', cell: '', unconfirmed: false };
  a.tasks.find((t) => t.house_name === 'DA_HouseSor').location = { known: true, map: 'Caladan', map_key: '', area: 'Caladan', place: '', cell: '', unconfirmed: false };
  a.tasks.find((t) => t.house_name === 'DA_HouseWallach').location = { known: true, map: 'Somewhere', map_key: 'Somewhere', area: 'Somewhere', place: '', cell: '', unconfirmed: false, x: 0.5, y: 0.5 };
  return a;
}
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
async function open(view, layout) {
  const ui = await boot({ get: { '/api/landsraad': answer() }, post: {}, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': layout } });
  await ui.settle();
  return ui;
}
const pct = (v) => parseFloat(String(v)) / 100;

// the pin the page drew in one peek, worked back to a place in the picture: the window's left and top in the picture (from the offset and the width of the picture in the window) plus the pin's place in the window
function pinOf(peek, frame) {
  const img = peek.querySelector('.mwin .mimg'), pin = peek.querySelector('.mwin .pin');
  assert.ok(img && pin, 'a picture window with a pin');
  const zoom = 100 / parseFloat(img.style.width);                   // the share of the map's width that the window shows
  const wl = -parseFloat(img.style.left) / parseFloat(img.style.width), fh = zoom * (frame.w / frame.h) / (16 / 9), wt = -parseFloat(img.style.top) / 100 * fh;
  return { x: wl + pct(pin.style.left) * zoom, y: wt + pct(pin.style.top) * fh, zoom, fh, left: wl, top: wt };
}

// ---- the real picture, on the right map, with the pin where the Saved map puts it ----
for (const [view, layout] of [['classic', 'grid'], ['board', 'list']]) {
  for (const [name, key, fx, fy] of PLACES.slice(0, 4)) {
    test(`#1069: ${view} view, a House on ${LABEL[key]}: the place peek draws the picture of that map with the pin at the Saved map's place`, async () => {
      const a = answer(), id = idOf(a, name), frame = FRAMES[key];
      assert.ok(frame, 'the map is in livemap.go');
      const ui = await open(view, layout);
      const sel = view === 'classic' ? `#lsrow-${id} .c-loc .loc > .pop` : `.lcc-bd-row[data-id="${id}"] td.lcc-bd-loc .loc > .pop`;
      const peek = ui.doc.querySelector(sel);
      assert.ok(peek, 'the peek: ' + sel);
      const m = peek.querySelector('.minimap');
      assert.ok(m && m.classList.contains('mpic'), 'a picture, not the schematic');
      assert.equal(peek.querySelector('.minimap svg'), null, 'no octagon');
      // the picture is the Saved map's own file, from this program's address, and lazy
      const imgs = [...m.querySelectorAll('img')];
      assert.equal(imgs.length, 2, 'the window and the small whole map');
      for (const i of imgs) { assert.equal(i.getAttribute('src'), frame.image); assert.equal(i.getAttribute('loading'), 'lazy'); assert.ok(!/^(data:|[a-z]+:)/i.test(i.getAttribute('src')), 'a path of this program, never a data URL or another address'); }
      // the pin is the same fraction of the picture as on the Saved map
      const p = pinOf(m, frame);
      assert.ok(Math.abs(p.x - fx) < 0.002 && Math.abs(p.y - fy) < 0.002, `pin at ${p.x.toFixed(4)}, ${p.y.toFixed(4)}; the Saved map puts it at ${fx}, ${fy}`);
      // the window is inside the picture and shows a part of it, not the whole
      assert.ok(p.left >= -1e-9 && p.left + p.zoom <= 1 + 1e-9 && p.top >= -1e-9 && p.top + p.fh <= 1 + 1e-9, 'the window stays inside the picture');
      assert.ok(p.zoom < 0.5, 'zoomed in');
      // the small whole map has the place itself and marks the window
      const dot = m.querySelector('.mfull .mdot'), box = m.querySelector('.mfull .mview');
      assert.ok(Math.abs(pct(dot.style.left) - fx) < 0.002 && Math.abs(pct(dot.style.top) - fy) < 0.002, 'the whole map carries the same place');
      assert.ok(Math.abs(pct(box.style.left) - p.left) < 0.002 && Math.abs(pct(box.style.top) - p.top) < 0.002 && Math.abs(pct(box.style.width) - p.zoom) < 0.002, 'and marks the window');
      // the lines stay: the Region and From you, the Saved map hint
      assert.match(text(peek), /Region/);
      assert.match(text(peek), name === 'Alexin' ? /Region\s*Arrakeen/ : /From you\s*about 1\.5 km/);
      assert.match(text(peek), /Click the place to open the Saved map/);
      assert.equal(peek.getAttribute('aria-hidden'), 'true');
      assert.match(m.getAttribute('aria-label'), new RegExp('Map of ' + LABEL[key]));
    });
  }
}

test('#1069: a place near a corner of its picture is not centred: the window stays inside the picture and the small map moves away from the pin', async () => {
  const a = answer(), ui = await open('classic', 'grid');
  for (const [name, key, fx, fy, corner] of [['Lindaren', 'HaggaBasin', 0.97, 0.93, 'c-lb'], ['Mutelli', 'Arrakeen', 0.02, 0.03, 'c-rb']]) {
    const m = ui.doc.querySelector(`#lsrow-${idOf(a, name)} .c-loc .loc > .pop .minimap`);
    const p = pinOf(m, FRAMES[key]);
    assert.ok(Math.abs(p.x - fx) < 0.002 && Math.abs(p.y - fy) < 0.002, name + ' pin');
    assert.ok(p.left >= -1e-9 && p.left + p.zoom <= 1 + 1e-9 && p.top >= -1e-9 && p.top + p.fh <= 1 + 1e-9, name + ': inside the picture');
    assert.ok(m.querySelector('.mwin .pin').style.left !== '50%' || m.querySelector('.mwin .pin').style.top !== '50%', name + ': not centred, the picture ends there');
    assert.ok(m.querySelector('.mfull').classList.contains(corner), name + ' small map in ' + corner + ': ' + m.querySelector('.mfull').className);
  }
});

// ---- the place peek is the same picture everywhere ----
test('#1069: the Place line of the House card, the opened Classic row and the Inspector draw the same picture, and a click on the picture still opens the place (#1025)', async () => {
  const a = answer(), id = idOf(a, 'Vernius');
  let ui = await open('classic', 'grid');
  const card = ui.doc.querySelector(`#lsrow-${id} .hcard .loc > .pop .minimap.mpic`), where = ui.doc.querySelector(`#lsrow-${id} .more .minimap.mpic`);
  assert.ok(card && where, 'the card peek and the opened row both have the picture');
  assert.equal(card.querySelector('.mimg').getAttribute('src'), '/maps/deep-desert.png');
  ui = await open('board', 'list');
  click(ui, `[data-fk="ht:${id}"]`); await ui.settle();
  const insp = ui.doc.querySelector('.lcc-bd-insp .loc > .pop .minimap.mpic');
  assert.ok(insp, 'the Inspector place peek is the picture');
  assert.equal(ui.doc.querySelector('.lcc-bd-insp').querySelectorAll('.minimap').length, 1, 'the one picture, in the mouse-over');
  const grid = await open('board', 'grid');
  assert.ok(grid.doc.querySelector(`li.lcc-bd-cell[data-id="${id}"] .hcard .loc > .pop .minimap.mpic`), 'and the grid card');
  // a click on any part of the picture is a click on the place
  for (const part of ['.mwin', '.mwin .pin', '.mfull', '.mfull img']) {
    const u = await open('classic', 'grid');
    click(u, u.doc.querySelector(`#lsrow-${id} .c-loc .loc > .pop ${part}`)); await u.settle();
    assert.equal(u.w.liveMapState().focus.label, 'Vernius', part);
  }
});

// ---- the schematic stays only where there is no picture to show, and says why ----
test('#1069: the schematic octagon is only for a place outside its picture and for a map with no picture, says why, and draws no pin', async () => {
  const a = answer(), ui = await open('classic', 'grid');
  for (const [name, why] of [['Thorvald', /outside the frame of the picture, so there is no pin/], ['Sor', /no picture of Caladan yet/], ['Wallach', /no picture of Somewhere that this place can be pinned on/]]) {
    const peek = ui.doc.querySelector(`#lsrow-${idOf(a, name)} .c-loc .loc > .pop`);
    assert.ok(peek, name + ': the peek');
    const m = peek.querySelector('.minimap');
    assert.ok(m.querySelector('svg'), name + ': the schematic');
    assert.ok(!m.classList.contains('mpic') && !m.querySelector('img') && !m.querySelector('.pin'), name + ': no picture, no pin');
    assert.match(text(m.querySelector('.mwhy')), /^Schematic only, not to scale: /, name);
    assert.match(text(m.querySelector('.mwhy')), why, name);
    assert.doesNotMatch(m.querySelector('svg').outerHTML, /schematic, not to scale/, 'the 3 px label inside the drawing is gone: the reason is real text');
  }
  assert.match(text(ui.doc.querySelector(`#lsrow-${idOf(a, 'Thorvald')} .more`)), /This place is outside the frame of the picture of Hagga Basin\. There is no pin or map link for this house\./);
});

// ---- nothing is fetched until the peek is shown, and nothing leaves the program ----
test('#1069: every picture of a peek is lazy and from this program; the page holds no inlined picture and no other address', async () => {
  const ui = await open('classic', 'grid');
  const imgs = [...ui.doc.querySelectorAll('.lsb .pop img, .lsb .hcard img.hsym, .lsb .more .minimap img')];
  assert.ok(imgs.length > 10, 'the pictures are on the page');
  for (const i of imgs) {
    assert.equal(i.getAttribute('loading'), 'lazy', i.outerHTML.slice(0, 80));
    assert.match(i.getAttribute('src'), /^\/(maps|lcc\/art\/houses)\/[a-z-]+\.(png|webp)$/, 'a file of this program, by a path in its fixed list');
  }
  assert.ok(!/data:image/.test(ui.doc.querySelector('#lsb').innerHTML), 'no inlined picture');
  const css = read('internal', 'web', 'static', 'app.css');
  assert.ok(!/(url\(["']?(https?:)?\/\/)/.test(css), 'no address outside the program in the style sheet');
  assert.deepEqual(ui.posted, [], 'nothing is written');
});

// ---- the hover works for the keyboard too, and can be put away ----
test('#1069: the peek opens on keyboard focus as well as on hover (the style sheet), and Escape puts it away while the focus stays', async () => {
  const css = read('internal', 'web', 'static', 'app.css');
  assert.match(css, /\.lsb \.loc:hover>\.pop\{display:block!important\}/, 'hover');
  assert.match(css, /\.lsb \.loc:has\(:focus-visible\)>\.pop\{display:block!important\}/, 'keyboard focus on the place');
  assert.match(css, /\.lsb \.loc\[data-esc\]>\.pop\{display:none!important\}/, 'Escape');
  const a = answer(), id = idOf(a, 'Kenola'), ui = await open('classic', 'grid');
  const btn = ui.doc.querySelector(`#lsrow-${id} .c-loc .loc-t`), loc = btn.closest('.loc');
  btn.focus();
  btn.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
  assert.ok(loc.hasAttribute('data-esc'), 'the peek is put away');
  assert.equal(ui.doc.activeElement, btn, 'the focus stays on the link');
  const other = ui.doc.querySelector(`#lsrow-${idOf(a, 'Vernius')} .c-loc .loc-t`);
  other.focus(); other.dispatchEvent(new ui.w.FocusEvent('focusin', { bubbles: true }));
  assert.ok(!loc.hasAttribute('data-esc'), 'and it comes back when the focus is somewhere else');
  // the narrow window shows the peek as a sheet over the page, as before (#1025)
  assert.match(css, /@media \(max-width:640px\)\{[^@]*\.lsb \.pop\{position:fixed;left:12px;right:12px;bottom:12px;top:auto;width:auto;margin:0\}/);
});

// ---- the House hover card: the large symbol, the name and the region ----
for (const [view, layout, sel] of [['classic', 'grid', (id) => `#lsrow-${id} .hcard`], ['board', 'list', (id) => `.lcc-bd-row[data-id="${id}"] .hcard`], ['board', 'grid', (id) => `li.lcc-bd-cell[data-id="${id}"] .hcard`]]) {
  test(`#1069: ${view} ${layout}: the House hover card starts with the large House symbol, the name and the region`, async () => {
    const a = answer(), ui = await open(view, layout);
    const card = ui.doc.querySelector(sel(idOf(a, 'Vernius')));
    assert.ok(card, 'the card');
    const img = card.querySelector('.hhead img.hsym');
    assert.ok(img, 'the symbol');
    assert.equal(img.getAttribute('src'), '/lcc/art/houses/vernius.webp', 'the 128 px symbol of the Command Center art, from this program');
    assert.equal(img.getAttribute('loading'), 'lazy');
    assert.equal(img.getAttribute('alt'), '', 'the name beside it names the House');
    assert.equal(img.getAttribute('width'), '64');
    assert.equal(text(card.querySelector('.hhead h4')), 'House Vernius');
    assert.equal(text(card.querySelector('.hhead .hreg')), 'Deep Desert');
    assert.ok(card.querySelector('.hhead').nextElementSibling.tagName === 'DL', 'the existing lines follow');
    // a House that is not mapped says so, and the card keeps its other lines
    const un = ui.doc.querySelector(sel(idOf(a, 'Hagal')));
    assert.equal(text(un.querySelector('.hhead .hreg')), 'not mapped yet');
    assert.match(text(un), /Next reward/);
  });
}

test('#1069: the House symbol file is one of the Command Center art files, found in a fixed list, never built from a name', async () => {
  const a = answer(), ui = await open('classic', 'grid');
  const files = new Set(fs.readdirSync(path.join(repoRoot, 'internal', 'web', 'static', 'lcc', 'art', 'houses')));
  const srcs = [...ui.doc.querySelectorAll('.lsb .hcard img.hsym')].map((i) => i.getAttribute('src'));
  assert.equal(srcs.length, 25, 'every House has its symbol');
  for (const s of srcs) assert.ok(files.has(s.split('/').pop()), s + ' is a file of the art folder');
  void a;
});

// ---- the words stay at or over the text floor ----
test('#1069: the new lines of the peek and the card are 12 px or more', () => {
  const css = read('internal', 'web', 'static', 'app.css');
  assert.match(css, /:root\[data-theme="light"\] \.lsb \.hcard \.hsym\{filter:brightness\(0\)/, 'the light drawings of the symbols are dark on the light look');
  for (const re of [/\.lsb \.mwhy\{[^}]*font-size:var\(--t-sm\)/, /\.lsb \.hcard \.hreg\{[^}]*font-size:var\(--t-sm\)/, /\.lsb \.pop \.fine\{[^}]*font-size:var\(--t-sm\)/, /\.lsb \.hcard \.fine\{[^}]*font-size:var\(--t-sm\)/]) assert.match(css, re);
});
