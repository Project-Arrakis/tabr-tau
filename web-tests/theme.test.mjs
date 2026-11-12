// The Look, the Dark / Dayside choice, the heading font and the Theme menu (#570, slice 3): /look-boot.js keeps and applies the choices before the page draws, /theme-menu.js is the control
// in the header. The real files run in jsdom over the editor's own pages. A stored value is only ever compared with a fixed list; nothing a storage key holds can name a file.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';

const here = path.dirname(fileURLToPath(import.meta.url));
const static_ = path.join(here, '..', 'internal', 'web', 'static');
const read = (n) => fs.readFileSync(path.join(static_, n), 'utf8');

const BASE = { path: 'C:\\x\\game.db', dirty: false, pending: [], saveBlocked: false, changedOnDisk: false, gameMode: 'unknown', gameProcesses: [], journalOff: '', held: null, restored: null, lastWrite: null };

// opts.page: 'index.html' (default) or 'locked.html'; opts.store: what the browser's storage holds before the page starts; opts.light: the system asks for the light theme;
// opts.blocked: storage throws on every touch. The head scripts run in the order of the page: Motion, Look, then (index only) the menu and the page.
async function page(opts = {}) {
  const name = opts.page || 'index.html';
  const dom = new JSDOM(read(name), { url: 'http://127.0.0.1:8090/', runScripts: 'dangerously', pretendToBeVisual: true });
  const w = dom.window;
  w.fetch = async (p) => {
    const u = new URL(p, 'http://127.0.0.1:8090');
    if (u.pathname === '/api/save/state') return { ok: true, status: 200, statusText: 'OK', json: async () => ({ ...BASE }) };
    return { ok: false, status: 404, statusText: 'Not Found', json: async () => ({ error: 'not found' }) };
  };
  for (const [k, v] of Object.entries(opts.store || {})) w.localStorage.setItem(k, v);
  const mq = { matches: !!opts.light, listeners: [], addEventListener(type, f) { this.listeners.push(f); }, removeEventListener() {} };
  w.matchMedia = (q) => (q.includes('prefers-color-scheme') ? mq : { matches: false, addEventListener() {}, removeEventListener() {} });
  if (opts.blocked) Object.defineProperty(w, 'localStorage', { get() { throw new Error('storage is blocked'); } });
  if (name === 'index.html') w.eval(read('motion-boot.js'));
  w.eval(read('look-boot.js'));
  if (name === 'index.html') {
    w.eval(read('html.js'));
    w.eval(read('livemap.js'));
    w.eval(read('theme-menu.js'));
    w.eval(read('app.js'));
  }
  const wait = (ms) => new Promise((r) => w.setTimeout(r, ms));
  for (let i = 0; i < 20; i++) await wait(0);
  const html = w.document.documentElement;
  const hrefs = () => [...w.document.querySelectorAll('link[rel="stylesheet"]')].map((l) => l.getAttribute('href'));
  const key = (el, k) => el.dispatchEvent(new w.KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }));
  return { w, doc: w.document, html, wait, hrefs, key, mq, close: () => w.close() };
}
const FREMEN = ['/design/tokens-fremen.css', '/design/components-fremen.css', '/design/components.css', '/design/motion.css', '/design/palettes.css', '/app.css'];

test('with nothing chosen the page is Fremen with Dune Rise headings and follows the system theme', async () => {
  const p = await page();
  assert.equal(p.html.getAttribute('data-look'), 'fremen');
  assert.equal(p.html.getAttribute('data-heading-font'), 'dune-rise');
  assert.equal(p.html.hasAttribute('data-theme'), false);
  assert.deepEqual(p.hrefs(), FREMEN);
  assert.equal(p.w.TauLook.theme(), 'dark');
  assert.equal(p.w.TauLook.themePicked(), false);
  p.close();
  const l = await page({ light: true });
  assert.equal(l.w.TauLook.theme(), 'light', 'the system asks for Dayside');
  assert.equal(l.html.hasAttribute('data-theme'), false, 'and the page follows it without forcing anything');
  l.close();
});

test('kept choices are applied before the page draws, on the index and the locked page alike', async () => {
  for (const name of ['index.html', 'locked.html']) {
    const p = await page({ page: name, store: { tauLook: 'sietch', tauTheme: 'light', tauFont: 'system' } });
    assert.equal(p.html.getAttribute('data-look'), 'sietch', name);
    assert.equal(p.html.getAttribute('data-theme'), 'light');
    assert.equal(p.html.getAttribute('data-heading-font'), 'system');
    assert.deepEqual(p.hrefs().slice(0, 3), ['/design/tokens-sietch.css', '/design/components-sietch.css', '/design/components.css']);
    assert.equal(p.doc.querySelectorAll('link[data-look-next]').length, 0, 'before the first paint the links are changed where they stand');
    p.close();
  }
});

test('anything else in a key is ignored: a stored string never names a file, a selector or markup', async () => {
  for (const junk of ['', 'FREMEN', 'sietch ', '../app', '/design/tokens-sietch.css', 'javascript:alert(1)', '<img src=x>', 'fremen"], body{', '__proto__', '1']) {
    const p = await page({ store: { tauLook: junk, tauTheme: junk, tauFont: junk } });
    assert.equal(p.html.getAttribute('data-look'), 'fremen', JSON.stringify(junk));
    assert.equal(p.html.hasAttribute('data-theme'), false);
    assert.equal(p.html.getAttribute('data-heading-font'), 'dune-rise');
    assert.deepEqual(p.hrefs(), FREMEN);
    p.close();
  }
});

test('a switch of look puts the new links in beside the old and drops the old only when the new have loaded', async () => {
  const p = await page();
  let heard = null;
  p.doc.addEventListener('taulook', (e) => { heard = e.detail; });
  assert.equal(p.w.TauLook.setLook('awakening'), true);
  assert.equal(p.w.localStorage.getItem('tauLook'), 'awakening');
  assert.equal(p.html.getAttribute('data-look'), 'awakening');
  assert.deepEqual({ ...heard }, { look: 'awakening', theme: 'dark', picked: false, font: 'dune-rise', palette: null });
  const next = [...p.doc.querySelectorAll('link[data-look-next]')];
  assert.deepEqual(next.map((l) => l.getAttribute('href')), ['/design/tokens-awakening.css', '/design/components-awakening.css']);
  assert.ok(p.hrefs().includes('/design/tokens-fremen.css'), 'the old look is still there while the new one loads');
  next.forEach((l) => l.dispatchEvent(new p.w.Event('load')));
  assert.deepEqual(p.hrefs(), ['/design/tokens-awakening.css', '/design/components-awakening.css', '/design/components.css', '/design/motion.css', '/design/palettes.css', '/app.css']);
  assert.equal(p.doc.querySelectorAll('link[data-look-next]').length, 0);
  p.close();
});

test('a switch that is still loading is dropped for the next one, and one that cannot load leaves the old look', async () => {
  const p = await page();
  p.w.TauLook.setLook('awakening');
  p.w.TauLook.setLook('sietch');
  const next = [...p.doc.querySelectorAll('link[data-look-next]')];
  assert.deepEqual(next.map((l) => l.getAttribute('href')), ['/design/tokens-sietch.css', '/design/components-sietch.css'], 'only the last switch is loading');
  next.forEach((l) => l.dispatchEvent(new p.w.Event('error')));
  assert.deepEqual(p.hrefs(), FREMEN, 'the old look stays when the new one cannot load');
  p.close();
});

test('a word that is not in the lists is refused and changes nothing', async () => {
  const p = await page();
  for (const [fn, bad] of [['setLook', 'neon'], ['setLook', '../x'], ['setLook', null], ['setTheme', 'sepia'], ['setTheme', ''], ['setFont', 'comic'], ['setFont', undefined]]) {
    assert.equal(p.w.TauLook[fn](bad), false, fn + ' ' + bad);
  }
  assert.equal(p.w.localStorage.getItem('tauLook'), null);
  assert.equal(p.w.localStorage.getItem('tauTheme'), null);
  assert.equal(p.w.localStorage.getItem('tauFont'), null);
  assert.deepEqual(p.hrefs(), FREMEN);
  p.close();
});

test('Dark and Dayside: a pick is kept, and following the system takes it back', async () => {
  const p = await page({ light: true });
  p.w.TauLook.setTheme('dark');
  assert.equal(p.html.getAttribute('data-theme'), 'dark');
  assert.equal(p.w.TauLook.theme(), 'dark', 'a pick beats the system');
  assert.equal(p.w.TauLook.themePicked(), true);
  assert.equal(p.w.localStorage.getItem('tauTheme'), 'dark');
  p.w.TauLook.followSystem();
  assert.equal(p.html.hasAttribute('data-theme'), false);
  assert.equal(p.w.localStorage.getItem('tauTheme'), null);
  assert.equal(p.w.TauLook.theme(), 'light');
  p.w.TauLook.setFont('system');
  assert.equal(p.html.getAttribute('data-heading-font'), 'system');
  p.close();
});

test('a browser that blocks its storage still works: a choice lasts until the page is closed', async () => {
  const p = await page({ blocked: true });
  assert.equal(p.html.getAttribute('data-look'), 'fremen');
  assert.equal(p.w.TauLook.setLook('sietch'), true);
  assert.equal(p.html.getAttribute('data-look'), 'sietch');
  assert.equal(p.w.TauLook.setTheme('light'), true);
  p.w.TauLook.followSystem();
  p.close();
});

test('the Theme menu is in the header with a bulb and a button, closed, and its groups are the choices', async () => {
  const p = await page();
  const host = p.doc.querySelector('header #theme-host');
  assert.ok(host, 'the host is in the header');
  const btn = host.querySelector('#theme-btn'), bulb = host.querySelector('#theme-bulb'), menu = host.querySelector('#theme-menu');
  assert.ok(btn && bulb && menu);
  assert.equal(menu.hidden, true);
  assert.equal(btn.getAttribute('aria-expanded'), 'false');
  assert.equal(btn.getAttribute('aria-haspopup'), 'menu');
  assert.equal(btn.textContent, 'Theme: Fremen');
  assert.equal(bulb.getAttribute('aria-label'), 'Switch to Dayside');
  const groups = [...menu.querySelectorAll('.x-thg')].map((g) => g.textContent);
  assert.deepEqual(groups, ['Look', 'Dark / Dayside', 'Colours', 'Font', 'Motion']);
  const items = [...menu.querySelectorAll('[role="menuitemradio"]')];
  assert.deepEqual(items.filter((i) => i.dataset.thgroup !== 'palette').map((i) => i.dataset.thgroup + ':' + i.dataset.thvalue), ['look:fremen', 'look:awakening', 'look:sietch', 'theme:system', 'theme:dark', 'theme:light', 'font:system', 'font:dune-rise', 'motion:full', 'motion:calm', 'motion:off']);
  assert.deepEqual(items.filter((i) => i.getAttribute('aria-checked') === 'true').map((i) => i.dataset.thgroup + ':' + i.dataset.thvalue), ['look:fremen', 'theme:system', 'palette:none', 'font:dune-rise', 'motion:full']);
  assert.ok(items.every((i) => i.tabIndex === -1), 'one stop in the menu once it is open, not before');
  p.close();
});

test('opening the menu, picking an item and closing it: the page follows each pick and the keyboard works', async () => {
  const p = await page();
  const btn = p.doc.querySelector('#theme-btn'), menu = p.doc.querySelector('#theme-menu');
  const item = (g, v) => menu.querySelector(`[data-thgroup="${g}"][data-thvalue="${v}"]`);
  btn.click();
  assert.equal(menu.hidden, false);
  assert.equal(btn.getAttribute('aria-expanded'), 'true');
  assert.equal(p.doc.activeElement, item('look', 'fremen'), 'the chosen item of the first group has the focus');
  item('look', 'sietch').click();
  assert.equal(p.html.getAttribute('data-look'), 'sietch');
  assert.equal(item('look', 'sietch').getAttribute('aria-checked'), 'true');
  assert.equal(item('look', 'fremen').getAttribute('aria-checked'), 'false');
  assert.equal(btn.textContent, 'Theme: Sietch');
  assert.equal(menu.hidden, false, 'the menu stays open so that a combination can be tried');
  item('theme', 'light').click();
  assert.equal(p.html.getAttribute('data-theme'), 'light');
  assert.equal(item('theme', 'light').getAttribute('aria-checked'), 'true');
  item('theme', 'system').click();
  assert.equal(p.html.hasAttribute('data-theme'), false);
  assert.equal(item('theme', 'system').getAttribute('aria-checked'), 'true');
  item('font', 'system').click();
  assert.equal(p.html.getAttribute('data-heading-font'), 'system');
  item('motion', 'off').click();
  assert.equal(p.html.getAttribute('data-motion'), 'off');
  assert.equal(p.w.localStorage.getItem('tauMotion'), 'off');
  assert.equal(item('motion', 'off').getAttribute('aria-checked'), 'true');
  // the keyboard: arrows move, Home and End jump, Enter picks, Escape closes and gives the focus back
  p.key(item('look', 'sietch'), 'Home');
  assert.equal(p.doc.activeElement, menu.querySelector('[role="menuitemradio"]'));
  p.key(p.doc.activeElement, 'ArrowUp');
  assert.equal(p.doc.activeElement, [...menu.querySelectorAll('[role="menuitemradio"]')].pop(), 'the arrows wrap');
  p.key(p.doc.activeElement, 'End');
  p.key(p.doc.activeElement, 'ArrowDown'); // wraps to the first item
  p.key(p.doc.activeElement, 'ArrowDown');
  p.key(p.doc.activeElement, 'Enter');
  assert.equal(p.html.getAttribute('data-look'), 'awakening', 'Enter picks the item that has the focus');
  p.key(p.doc.activeElement, 'Escape');
  assert.equal(menu.hidden, true);
  assert.equal(btn.getAttribute('aria-expanded'), 'false');
  assert.equal(p.doc.activeElement, btn);
  p.close();
});

test('a click outside the menu closes it, and the bulb switches between Dark and Dayside', async () => {
  const p = await page();
  const btn = p.doc.querySelector('#theme-btn'), menu = p.doc.querySelector('#theme-menu'), bulb = p.doc.querySelector('#theme-bulb');
  btn.click();
  p.doc.body.click();
  assert.equal(menu.hidden, true);
  bulb.click();
  assert.equal(p.html.getAttribute('data-theme'), 'light');
  assert.equal(bulb.getAttribute('aria-label'), 'Switch to Dark');
  bulb.click();
  assert.equal(p.html.getAttribute('data-theme'), 'dark');
  assert.equal(bulb.getAttribute('aria-label'), 'Switch to Dayside');
  assert.equal(p.w.localStorage.getItem('tauTheme'), 'dark');
  p.close();
});

test('the menu shows what a kept choice was, and follows a change made in another window', async () => {
  const p = await page({ store: { tauLook: 'awakening', tauTheme: 'light', tauMotion: 'calm' } });
  const menu = p.doc.querySelector('#theme-menu');
  const checked = () => [...menu.querySelectorAll('[aria-checked="true"]')].map((i) => i.dataset.thgroup + ':' + i.dataset.thvalue).join(' ');
  assert.equal(checked(), 'look:awakening theme:light palette:none font:dune-rise motion:calm');
  p.w.localStorage.setItem('tauLook', 'sietch');
  p.w.dispatchEvent(new p.w.StorageEvent('storage', { key: 'tauLook' }));
  assert.equal(p.html.getAttribute('data-look'), 'sietch');
  assert.equal(checked(), 'look:sietch theme:light palette:none font:dune-rise motion:calm');
  p.close();
});

test('the editor still starts and answers when the menu cannot be built (no look script)', async () => {
  const dom = new JSDOM(read('index.html'), { url: 'http://127.0.0.1:8090/', runScripts: 'dangerously', pretendToBeVisual: true });
  const w = dom.window;
  w.fetch = async () => ({ ok: true, status: 200, statusText: 'OK', json: async () => ({ ...BASE }) });
  w.eval(read('theme-menu.js'));
  w.eval(read('html.js'));
  w.eval(read('livemap.js'));
  w.eval(read('app.js'));
  assert.equal(w.document.querySelector('#theme-host').children.length, 0, 'without the look script there is nothing to put in the header');
  w.close();
});
