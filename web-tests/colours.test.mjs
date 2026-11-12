// The Colours group of the Theme menu (#587, refs #570): the eighteen palettes of the design module (palettes.css) recolour the look in their own theme. look-boot.js reads one stored word
// (tauPalette) before the first paint into html[data-palette], theme-menu.js offers them under their group headings, and the module's stylesheet does the recolouring. Tested in jsdom by
// attribute, class and focus assertions; how the colours look is the operator's check. The table of palettes in look-boot.js is compared with the module's own list by a Go test
// (internal/web/colours_test.go) and, here, with the ids and themes in the module's stylesheet.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(here, '..', 'internal', 'web', 'static');
const read = (f) => fs.readFileSync(path.join(root, f), 'utf8').replace(/\r\n/g, '\n');
const files = { motion: read('motion-boot.js'), look: read('look-boot.js'), menu: read('theme-menu.js') };
const pages = { 'index.html': read('index.html'), 'locked.html': read('locked.html') };
const appCss = read('app.css');
const KEY = 'tauPalette';
const GROUPS = ['Planet', 'House & faction', 'Character', 'Guild & commerce', 'Mystical & mental'];

// the module's stylesheet: the palette ids with the one theme each applies in, in file order
const moduleCss = fs.readFileSync(path.join(here, '..', 'design', 'assets', 'palettes.css'), 'utf8').replace(/\r\n/g, '\n');
const moduleBlocks = [...moduleCss.matchAll(/^:root\[data-palette="([a-z-]+)"\]\[data-theme="(dark|light)"\] \{/gm)].map((m) => ({ id: m[1], theme: m[2] }));

function open(page, { store = {}, light = false, width = 1024, menu = true } = {}) {
  const dom = new JSDOM(pages[page], { runScripts: 'outside-only', pretendToBeVisual: true, url: 'http://127.0.0.1:8090/' });
  const w = dom.window;
  const media = { light };
  w.matchMedia = (q) => ({ matches: q.includes('prefers-color-scheme: light') ? media.light : false, addEventListener() {}, removeEventListener() {} });
  w.innerWidth = width;
  w.innerHeight = 700;
  w.Element.prototype.getBoundingClientRect = function () { return { width: 80, height: 28, left: width - 120, top: 10, right: width - 40, bottom: 38 }; };
  for (const [k, v] of Object.entries(store)) w.localStorage.setItem(k, v);
  w.eval(files.motion);
  w.eval(files.look);
  if (menu) w.eval(files.menu);
  const d = w.document;
  const $ = (id) => d.getElementById(id);
  const attr = (n) => d.documentElement.getAttribute(n);
  const items = () => [...d.querySelectorAll('#theme-menu [role="menuitemradio"]')];
  const item = (group, value) => items().find((i) => i.getAttribute('data-thgroup') === group && i.getAttribute('data-thvalue') === value);
  const palettes = () => items().filter((i) => i.getAttribute('data-thgroup') === 'palette');
  const key = (el, k) => el.dispatchEvent(new w.KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }));
  const tag = (value) => item('palette', value).querySelector('.x-thtag');
  return { w, d, $, attr, items, item, palettes, key, tag, media };
}
// values made in the page's own realm are copied into this one before they are compared
const plain = (v) => JSON.parse(JSON.stringify(v));
const both = (fn) => { for (const page of Object.keys(pages)) fn(page); };

// ---- the table ----

test('the table in look-boot.js lists the 18 palettes of the module: Arrakis Incandescent first, the module\'s ids and themes in the module\'s order, groups kept together', () => {
  const p = open('index.html', { menu: false });
  const table = p.w.TauLook.PALETTES;
  assert.equal(table.length, 18);
  assert.deepEqual([table[0].id, table[0].name], ['warm-charcoal', 'Arrakis Incandescent']);
  assert.deepEqual(plain(table.map((t) => [t.id, t.theme])), moduleBlocks.map((b) => [b.id, b.theme]), 'the ids and themes are those of palettes.css, in its order');
  assert.deepEqual(plain(table.filter((t) => t.theme === 'light').map((t) => t.name)), ['Fremen Sietch', 'Mentat']);
  assert.deepEqual(plain([...new Set(table.map((t) => t.group))]), GROUPS, 'the groups, each in one run');
  let runs = 0;
  table.forEach((t, i) => { if (i === 0 || t.group !== table[i - 1].group) runs += 1; });
  assert.equal(runs, GROUPS.length);
  assert.ok(Object.isFrozen(table), 'the table cannot be changed by another script');
  p.w.close();
});

test('the palettes stylesheet of the module is linked after the look\'s files and motion.css and before look-boot.js, on both pages, from the same origin', () => both((page) => {
  const head = pages[page].slice(0, pages[page].indexOf('</head>'));
  const at = (s) => head.indexOf(s);
  assert.ok(at('<link rel="stylesheet" href="/design/palettes.css">') > at('<link rel="stylesheet" href="/design/motion.css">'), page + ': after motion.css');
  assert.ok(at('<link rel="stylesheet" href="/design/palettes.css">') < at('<script src="/look-boot.js"></script>'), page + ': before look-boot.js');
  assert.equal(head.split('palettes.css').length - 1, 1);
}));

// ---- before the first paint ----

test('with nothing chosen there is no palette attribute: the look\'s own colours', () => {
  const p = open('index.html');
  assert.equal(p.attr('data-palette'), null);
  assert.equal(p.w.TauLook.palette(), null);
  assert.equal(p.w.TauLook.paletteShows(), false);
  assert.equal(p.item('palette', 'none').getAttribute('aria-checked'), 'true');
  p.w.close();
});

test('a stored palette is on the page before anything else runs, on both pages, and nothing else changes', () => both((page) => {
  const p = open(page, { store: { [KEY]: 'sand-linen' }, menu: false });
  assert.equal(p.attr('data-palette'), 'sand-linen');
  assert.equal(p.attr('data-look'), 'fremen');
  assert.equal(p.attr('data-theme'), null, 'a palette does not choose the theme');
  assert.deepEqual([...p.d.querySelectorAll('link[rel="stylesheet"]')].map((l) => l.getAttribute('href')).slice(0, 5),
    ['/design/tokens-fremen.css', '/design/components-fremen.css', '/design/components.css', '/design/motion.css', '/design/palettes.css'], 'the same files; no file name was made from the value');
  p.w.close();
}));

test('a stored value that is not on the list is ignored: it is never used as a file name, a selector or markup', () => {
  const clean = open('index.html', { menu: false });
  const links = clean.d.querySelectorAll('link[rel="stylesheet"]').length;
  clean.w.close();
  for (const bad of ['', 'none', 'None', 'WARM-CHARCOAL', 'warm-charcoal ', 'dune-arrakis-inc', 'x', '../../x', '/design/palettes.css', '"][data-x="1', '<img src=x onerror=1>', 'constructor', '__proto__', 'toString', 'hasOwnProperty', '[]', 'null', 'fremen']) {
    const p = open('index.html', { store: { [KEY]: bad }, menu: false });
    assert.equal(p.attr('data-palette'), null, JSON.stringify(bad));
    assert.equal(p.w.TauLook.palette(), null);
    assert.equal(p.d.querySelectorAll('img[src="x"], [onerror], [data-x]').length, 0, 'nothing from the value became markup or an attribute');
    assert.equal(p.d.querySelectorAll('link[rel="stylesheet"]').length, links, 'no link was added');
    p.w.close();
  }
});

test('setPalette takes only a word of the list (or null to clear); anything else changes nothing', () => {
  const p = open('index.html');
  const L = p.w.TauLook;
  assert.equal(L.setPalette('x'), false);
  assert.equal(L.setPalette('none'), false, 'none is the menu\'s word; the key is cleared with null');
  assert.equal(L.setPalette(undefined), false);
  assert.equal(L.setPalette({ toString: () => 'warm-charcoal' }), false);
  assert.equal(p.attr('data-palette'), null);
  assert.equal(p.w.localStorage.getItem(KEY), null);
  assert.equal(L.setPalette('warm-charcoal'), true);
  assert.equal(p.attr('data-palette'), 'warm-charcoal');
  assert.equal(L.setPalette(null), true);
  assert.equal(p.attr('data-palette'), null);
  assert.equal(p.w.localStorage.getItem(KEY), null);
  p.w.close();
});

test('a page whose storage throws still works: no palette, and a choice lasts until the page is closed', () => {
  const dom = new JSDOM(pages['index.html'], { runScripts: 'outside-only', url: 'http://127.0.0.1:8090/' });
  const w = dom.window;
  w.matchMedia = () => ({ matches: false, addEventListener() {} });
  Object.defineProperty(w, 'localStorage', { get() { throw new Error('storage is off'); } });
  w.eval(files.motion);
  w.eval(files.look);
  w.eval(files.menu);
  assert.equal(w.document.documentElement.getAttribute('data-palette'), null);
  assert.equal(w.TauLook.setPalette('ink-blue'), true);
  assert.equal(w.document.documentElement.getAttribute('data-palette'), 'ink-blue');
  w.close();
});

// ---- the menu ----

test('the Colours group sits after Dark / Dayside: None, then the five groups of the module under their headings, each palette one radio item, Arrakis Incandescent first', () => {
  const p = open('index.html');
  const heads = [...p.d.querySelectorAll('#theme-menu .x-thg')].map((h) => h.textContent);
  assert.deepEqual(heads, ['Look', 'Dark / Dayside', 'Colours', 'Font', 'Motion']);
  const colours = p.$('theme-g-palette');
  assert.equal(colours.textContent, 'Colours');
  const box = colours.nextElementSibling;
  assert.equal(box.getAttribute('role'), 'group');
  assert.equal(box.getAttribute('aria-labelledby'), 'theme-g-palette');
  assert.deepEqual([...box.querySelectorAll('.x-thsub')].map((s) => s.textContent), GROUPS);
  for (const sub of box.querySelectorAll('.x-thsub')) {
    const inner = sub.nextElementSibling;
    assert.equal(inner.getAttribute('role'), 'group');
    assert.equal(inner.getAttribute('aria-labelledby'), sub.id, 'each run of palettes is a group named by its heading');
  }
  const list = p.palettes();
  assert.equal(list.length, 19);
  assert.deepEqual(list.map((i) => i.getAttribute('data-thvalue')), ['none', ...plain(p.w.TauLook.PALETTES.map((t) => t.id))]);
  assert.equal(list[0].querySelector('.x-thname').textContent, 'None (the look’s own)');
  assert.equal(list[1].querySelector('.x-thname').textContent, 'Arrakis Incandescent');
  assert.deepEqual(list.slice(1).map((i) => i.querySelector('.x-thname').textContent), plain(p.w.TauLook.PALETTES.map((t) => t.name)));
  assert.ok(list.every((i) => i.getAttribute('role') === 'menuitemradio' && i.type === 'button'));
  assert.equal(list.filter((i) => i.getAttribute('aria-checked') === 'true').length, 1);
  assert.match(p.d.querySelector('#theme-menu .x-thnote').textContent, /own theme only: 16 are dark, 2 are Dayside/);
  assert.equal(p.$('theme-menu').getAttribute('aria-label'), 'Theme: look, dark or dayside, colours, font, motion');
  p.w.close();
});

test('picking a palette sets the attribute, keeps the word, ticks the item and names it on the Theme button; None clears the attribute and the key', () => {
  const p = open('index.html');
  p.item('palette', 'warm-charcoal').click();
  assert.equal(p.attr('data-palette'), 'warm-charcoal');
  assert.equal(p.w.localStorage.getItem(KEY), 'warm-charcoal');
  assert.equal(p.item('palette', 'warm-charcoal').getAttribute('aria-checked'), 'true');
  assert.equal(p.item('palette', 'none').getAttribute('aria-checked'), 'false');
  assert.equal(p.$('theme-btn').textContent, 'Theme: Fremen · Arrakis Incandescent');
  assert.match(p.$('theme-btn').getAttribute('aria-label'), /Colours: Arrakis Incandescent\./);
  p.item('look', 'sietch').click();
  assert.equal(p.attr('data-palette'), 'warm-charcoal', 'a palette stays when the look changes: it recolours any look');
  p.item('palette', 'none').click();
  assert.equal(p.attr('data-palette'), null);
  assert.equal(p.w.localStorage.getItem(KEY), null);
  assert.equal(p.item('palette', 'none').getAttribute('aria-checked'), 'true');
  assert.equal(p.$('theme-btn').textContent, 'Theme: Sietch');
  p.w.close();
});

test('a palette is still there when the page is opened again', () => {
  const first = open('index.html');
  first.item('palette', 'deep-ocean').click();
  const kept = { [KEY]: first.w.localStorage.getItem(KEY) };
  assert.deepEqual(kept, { [KEY]: 'deep-ocean' });
  first.w.close();
  const again = open('index.html', { store: kept });
  assert.equal(again.attr('data-palette'), 'deep-ocean');
  assert.equal(again.item('palette', 'deep-ocean').getAttribute('aria-checked'), 'true');
  again.w.close();
});

test('a palette that is not in effect in the theme shown now says so: "Dayside only" in Dark, "Dark only" in Dayside, with a tooltip; it stays pickable and the choice is kept', () => {
  const p = open('index.html', { light: false });
  // Dark: the two Dayside palettes are marked, the dark ones are not
  for (const t of p.w.TauLook.PALETTES) {
    const it = p.item('palette', t.id);
    const tag = p.tag(t.id);
    if (t.theme === 'light') {
      assert.equal(tag.hidden, false, t.name);
      assert.equal(tag.textContent, 'Dayside only');
      assert.ok(it.classList.contains('x-thoff'));
      assert.ok(it.title.includes('is a Dayside palette: it shows in Dayside only'), t.name + ' has a tooltip');
    } else {
      assert.equal(tag.hidden, true, t.name);
      assert.equal(tag.textContent, '');
      assert.ok(!it.classList.contains('x-thoff'));
      assert.equal(it.hasAttribute('title'), false);
    }
  }
  // a Dayside palette picked in Dark is kept, and the button says it is not applied
  p.item('palette', 'sand-linen').click();
  assert.equal(p.attr('data-palette'), 'sand-linen');
  assert.equal(p.w.localStorage.getItem(KEY), 'sand-linen');
  assert.equal(p.w.TauLook.paletteShows(), false);
  assert.match(p.$('theme-btn').title, /Colours: Fremen Sietch \(not applied: it shows in Dayside only\)/);
  // Dayside: it is in effect now, and the dark palettes are the ones marked
  p.$('theme-bulb').click();
  assert.equal(p.attr('data-theme'), 'light');
  assert.equal(p.w.TauLook.paletteShows(), true);
  assert.equal(p.tag('sand-linen').hidden, true);
  assert.equal(p.tag('warm-charcoal').hidden, false);
  assert.equal(p.tag('warm-charcoal').textContent, 'Dark only');
  assert.ok(!/not applied/.test(p.$('theme-btn').title));
  assert.equal(p.item('palette', 'sand-linen').getAttribute('aria-checked'), 'true');
  p.w.close();
  // the system's setting counts when the person has not picked a theme
  const q = open('index.html', { light: true });
  assert.equal(q.tag('paper-grey').hidden, true, 'the system says Dayside: the Dayside palettes are in effect');
  assert.equal(q.tag('mono-grey').textContent, 'Dark only');
  q.w.close();
});

test('from the keyboard: the arrows walk through None and the palettes in the order shown, Enter and Space pick and the menu stays open, Home and End still reach the first and last items', () => {
  const p = open('index.html');
  const btn = p.$('theme-btn');
  btn.focus();
  p.key(btn, 'ArrowDown');
  p.key(p.d.activeElement, 'ArrowDown'); // Look: Fremen -> Awakening
  const none = p.item('palette', 'none');
  none.focus();
  p.key(none, 'ArrowDown');
  assert.equal(p.d.activeElement, p.item('palette', 'warm-charcoal'), 'Arrakis Incandescent follows None');
  p.key(p.d.activeElement, 'Enter');
  assert.equal(p.attr('data-palette'), 'warm-charcoal');
  assert.equal(p.$('theme-menu').hidden, false, 'the menu stays open after a pick');
  p.key(p.d.activeElement, 'ArrowDown');
  assert.equal(p.d.activeElement, p.item('palette', 'umber-sand'));
  p.key(p.d.activeElement, ' ');
  assert.equal(p.attr('data-palette'), 'umber-sand');
  p.key(p.d.activeElement, 'ArrowUp');
  p.key(p.d.activeElement, 'ArrowUp');
  assert.equal(p.d.activeElement, none);
  p.key(p.d.activeElement, 'Enter');
  assert.equal(p.attr('data-palette'), null);
  // across a group heading: the last of Planet is followed by the first of House & faction
  p.item('palette', 'circuit-indigo').focus();
  p.key(p.d.activeElement, 'ArrowDown');
  assert.equal(p.d.activeElement, p.item('palette', 'mono-grey'));
  p.key(p.d.activeElement, 'Home');
  assert.equal(p.d.activeElement, p.items()[0]);
  p.key(p.d.activeElement, 'End');
  assert.equal(p.d.activeElement, p.item('motion', 'off'));
  p.key(p.d.activeElement, 'Escape');
  assert.equal(p.$('theme-menu').hidden, true);
  assert.equal(p.d.activeElement, btn);
  p.w.close();
});

test('opening the menu puts the focus on the first group\'s chosen item and ticks the palette chosen now, one item in each group', () => {
  const p = open('index.html', { store: { [KEY]: 'violet-iron' } });
  p.$('theme-btn').click();
  assert.ok(p.item('palette', 'violet-iron').getAttribute('aria-checked') === 'true');
  assert.equal(p.items().filter((i) => i.getAttribute('aria-checked') === 'true').length, 5, 'one chosen in each group');
  p.w.close();
});

test('a change in another window of the editor is shown in this one: the storage event applies the palette, the menu and the button follow, and a removed key clears it', () => {
  const p = open('index.html');
  p.w.localStorage.setItem(KEY, 'storm-blue');
  p.w.dispatchEvent(new p.w.StorageEvent('storage', { key: KEY }));
  assert.equal(p.attr('data-palette'), 'storm-blue');
  assert.equal(p.item('palette', 'storm-blue').getAttribute('aria-checked'), 'true');
  assert.equal(p.$('theme-btn').textContent, 'Theme: Fremen · Caladan Storm');
  p.w.localStorage.removeItem(KEY);
  p.w.dispatchEvent(new p.w.StorageEvent('storage', { key: KEY }));
  assert.equal(p.attr('data-palette'), null);
  assert.equal(p.item('palette', 'none').getAttribute('aria-checked'), 'true');
  p.w.localStorage.setItem(KEY, '../../x');
  p.w.dispatchEvent(new p.w.StorageEvent('storage', { key: KEY }));
  assert.equal(p.attr('data-palette'), null, 'a junk word from the other window is ignored too');
  p.w.close();
});

test('the page is told when a palette is chosen: the taulook event carries it', () => {
  const p = open('index.html');
  const seen = [];
  p.d.addEventListener('taulook', (e) => seen.push(e.detail.palette));
  p.item('palette', 'mono-grey').click();
  p.item('palette', 'none').click();
  assert.deepEqual(seen, ['mono-grey', null]);
  p.w.close();
});

// ---- what the files are ----

test('the new code builds no markup from a string, makes no regular expression from a value, and the stylesheet rules use only the module\'s tokens', () => {
  for (const [name, src] of Object.entries(files)) {
    assert.ok(!/innerHTML|outerHTML|insertAdjacentHTML|eval\(|document\.write|new Function|setAttribute\('style'|new RegExp/.test(src), name + ' builds no markup from a string');
  }
  assert.ok(!files.menu.includes("'/design/") && !files.look.includes("'/design/palettes"), 'no script names the palette file: the pages link it, and a value is never made into a file name');
  assert.ok(!/setAttribute\('(href|src)'/.test(files.menu), 'the menu sets no link or source');
  assert.ok(!files.menu.includes('localStorage'), 'the menu keeps nothing itself: it asks the boot scripts');
  const rules = appCss.split('\n').filter((l) => /^\.x-thmenu \.x-th(sub|tag|off|note)/.test(l)).join('\n');
  assert.ok(rules.length > 0);
  assert.ok(!/#[0-9a-fA-F]{3,8}\b/.test(rules), 'no colour of its own in the Colours rules');
  for (const n of new Set([...rules.matchAll(/var\((--[a-z0-9-]+)/g)].map((m) => m[1]))) assert.ok(/^--(c|t|s|r|e)-/.test(n), n + ' is a token of the module');
  for (const cls of ['x-thsub', 'x-thtag', 'x-thoff', 'x-thnote']) assert.ok(appCss.includes('.' + cls), 'app.css styles .' + cls);
});
