// The Motion setting and what the page does with it (#571): /motion-boot.js reads and keeps one of Full, Calm or Off before the page draws; the status bar bumps the count of queued
// changes and draws a check when a write went through; a toast has its result icon and leaves with a fade; an edited row lights up. The real page code runs in jsdom; the page asks for
// nothing but the save state, which each test feeds.
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

// opts.store: what the browser's storage holds before the page starts; opts.script: false leaves /motion-boot.js out (a page that never ran it); opts.reduce: the system asks for reduced
// motion; opts.blocked: storage throws on every touch.
async function page(opts = {}) {
  const dom = new JSDOM(read('index.html'), { url: 'http://127.0.0.1:8090/', runScripts: 'dangerously', pretendToBeVisual: true });
  const w = dom.window;
  const state = { ...BASE, ...(opts.state || {}) };
  const box = { state };
  w.fetch = async (p) => {
    const u = new URL(p, 'http://127.0.0.1:8090');
    if (u.pathname === '/api/save/state') return { ok: true, status: 200, statusText: 'OK', json: async () => ({ ...BASE, ...box.state }) };
    return { ok: false, status: 404, statusText: 'Not Found', json: async () => ({ error: 'not found' }) };
  };
  w.confirm = () => true;
  for (const [k, v] of Object.entries(opts.store || {})) w.localStorage.setItem(k, v);
  if (opts.reduce !== undefined) w.matchMedia = () => ({ matches: !!opts.reduce, addEventListener() {}, removeEventListener() {} });
  if (opts.blocked) Object.defineProperty(w, 'localStorage', { get() { throw new Error('storage is blocked'); } });
  if (opts.script !== false) w.eval(read('motion-boot.js'));
  w.eval(read('html.js'));
  w.eval(read('livemap.js'));
  w.eval(read('app.js'));
  const wait = (ms) => new Promise((r) => w.setTimeout(r, ms));
  for (let i = 0; i < 30; i++) await wait(0);
  const poll = async (next) => { if (next) box.state = { ...BASE, ...next }; await w.status(); };
  const html = w.document.documentElement;
  return { w, doc: w.document, html, wait, poll, close: () => w.close() };
}

test('the page reads the choice before it draws: a kept word is used, anything else is ignored', async () => {
  for (const mode of ['full', 'calm', 'off']) {
    const p = await page({ store: { tauMotion: mode } });
    assert.equal(p.html.getAttribute('data-motion'), mode);
    assert.equal(p.html.getAttribute('data-motion-src'), 'user', 'a choice of the person wins over the system setting');
    p.close();
  }
  for (const junk of ['', 'FULL', 'loud', '<img src=x>', 'full ', '1']) {
    const p = await page({ store: { tauMotion: junk } });
    assert.equal(p.html.getAttribute('data-motion'), 'full', JSON.stringify(junk));
    assert.equal(p.html.hasAttribute('data-motion-src'), false, 'nothing was chosen');
    p.close();
  }
});

test('until the person chooses, the system decides: reduced motion is Calm, anything else is Full', async () => {
  const a = await page({ reduce: true });
  assert.equal(a.html.getAttribute('data-motion'), 'calm');
  assert.equal(a.html.hasAttribute('data-motion-src'), false);
  a.close();
  const b = await page({ reduce: false });
  assert.equal(b.html.getAttribute('data-motion'), 'full');
  b.close();
  const c = await page({ reduce: true, store: { tauMotion: 'full' } });
  assert.equal(c.html.getAttribute('data-motion'), 'full', 'a choice beats the system setting');
  c.close();
});

test('set() keeps a valid word, tells the page, and refuses everything else', async () => {
  const p = await page();
  let heard = null;
  p.doc.addEventListener('taumotion', (e) => { heard = e.detail; });
  assert.equal(p.w.TauMotion.set('off'), true);
  assert.equal(p.w.localStorage.getItem('tauMotion'), 'off');
  assert.equal(p.html.getAttribute('data-motion'), 'off');
  assert.equal(p.html.getAttribute('data-motion-src'), 'user');
  assert.deepEqual({ ...heard }, { mode: 'off', picked: true });
  for (const bad of ['loud', '', null, undefined, 3, {}, ['calm']]) assert.equal(p.w.TauMotion.set(bad), false, String(bad));
  assert.equal(p.html.getAttribute('data-motion'), 'off', 'a refused word changes nothing');
  assert.equal(p.w.localStorage.getItem('tauMotion'), 'off');
  assert.equal(p.w.TauMotion.marks(), false);
  assert.equal(p.w.TauMotion.eases(), false);
  assert.equal(p.w.TauMotion.loops(), false);
  p.w.TauMotion.set('calm');
  assert.equal(p.w.TauMotion.marks(), false, 'a mark that asks for the eye is for Full');
  assert.equal(p.w.TauMotion.eases(), true);
  assert.equal(p.w.TauMotion.loops(), false, 'a loop is for Full');
  p.w.TauMotion.set('full');
  assert.equal(p.w.TauMotion.marks() && p.w.TauMotion.eases() && p.w.TauMotion.loops(), true);
  p.close();
});

test('a browser that blocks its storage still works: the choice lasts until the page is closed', async () => {
  const p = await page({ blocked: true });
  assert.equal(p.html.getAttribute('data-motion'), 'full');
  assert.equal(p.w.TauMotion.set('calm'), true);
  assert.equal(p.html.getAttribute('data-motion'), 'calm');
  p.close();
});

test('a hidden document pauses the animations: the root says so and the loops stop', async () => {
  const p = await page();
  Object.defineProperty(p.doc, 'hidden', { configurable: true, get: () => true });
  p.doc.dispatchEvent(new p.w.Event('visibilitychange'));
  assert.equal(p.html.hasAttribute('data-doc-hidden'), true);
  assert.equal(p.w.TauMotion.loops(), false);
  Object.defineProperty(p.doc, 'hidden', { configurable: true, get: () => false });
  p.doc.dispatchEvent(new p.w.Event('visibilitychange'));
  assert.equal(p.html.hasAttribute('data-doc-hidden'), false);
  p.close();
});

test('a toast has a shape as well as a colour, errors stay with a dismiss button, and the result icons are the fill icons', async () => {
  const p = await page();
  const box = p.doc.querySelector('#toast');
  assert.ok(box.classList.contains('d-toasts'));
  p.w.toast('Done', false, 5000);
  p.w.toast('Part of it', 'part');
  p.w.toast('It failed', true);
  p.w.toast('Water filled', false, 5000, { kind: 'water' });
  p.w.toast('A warning with an icon', 'part', undefined, { kind: 'repair' });
  const [ok, part, bad, water, warn] = [...box.children];
  assert.equal(ok.className, 'd-toast');
  assert.equal(ok.getAttribute('role'), 'status');
  assert.ok(ok.querySelector('svg.d-i'), 'a check');
  assert.equal(part.className, 'd-toast warn part');
  assert.equal(part.getAttribute('role'), 'status');
  assert.equal(bad.className, 'd-toast bad err');
  assert.equal(bad.getAttribute('role'), 'alert');
  assert.ok(bad.querySelector('button.x'), 'an error is dismissed by hand');
  assert.equal(ok.querySelector('button'), null);
  assert.equal(water.getAttribute('data-fx'), 'water');
  assert.ok(water.querySelector('svg.d-fx.water .lvl') && water.querySelector('.shell') && water.querySelector('.chk'));
  assert.match(water.querySelector('.lvl').style.getPropertyValue('--lvl-from'), /^70%$/);
  assert.equal(water.textContent, 'Water filled', 'the icon adds no words');
  assert.equal(warn.hasAttribute('data-fx'), false, 'a warning keeps its own mark');
  assert.ok(warn.querySelector('svg.d-i'));
  const ids = [...box.querySelectorAll('clipPath')].map((c) => c.id);
  assert.equal(new Set(ids).size, ids.length, 'each icon has its own clip');
  bad.querySelector('button.x').click();
  assert.equal(bad.isConnected, false);
  p.close();
});

test('a toast leaves with a fade, and in Off it is gone at once', async () => {
  const full = await page({ store: { tauMotion: 'full' } });
  full.w.toast('bye', false, 20);
  const t = full.doc.querySelector('#toast .d-toast');
  await full.wait(60);
  assert.equal(t.isConnected, true, 'it is still fading out');
  assert.ok(t.classList.contains('out'));
  await full.wait(400);
  assert.equal(t.isConnected, false);
  full.close();
  const off = await page({ store: { tauMotion: 'off' } });
  off.w.toast('bye', false, 20);
  const o = off.doc.querySelector('#toast .d-toast');
  await off.wait(60);
  assert.equal(o.isConnected, false, 'nothing fades in Off');
  off.close();
});

test('the count of queued changes bumps once when a change joins the queue, in Full only', async () => {
  for (const [mode, bumps] of [['full', true], ['calm', false], ['off', false]]) {
    const p = await page({ store: { tauMotion: mode }, state: { dirty: true, pending: ['xp +500'] } });
    const pill = p.doc.querySelector('#pending');
    assert.equal(pill.classList.contains('x-bump'), false, 'the first poll is no change');
    await p.poll({ dirty: true, pending: ['xp +500'] });
    assert.equal(pill.classList.contains('x-bump'), false, 'the same count is no change');
    await p.poll({ dirty: true, pending: ['xp +500', 'solari +10000'] });
    assert.equal(pill.classList.contains('x-bump'), bumps, mode);
    assert.equal(pill.textContent, '2 queued');
    await p.wait(520);
    assert.equal(pill.classList.contains('x-bump'), false, 'the mark is taken off when it has played');
    await p.poll({ dirty: true, pending: ['xp +500'] });
    assert.equal(pill.classList.contains('x-bump'), false, 'a count that falls does not bump');
    p.close();
  }
});

test('a check is drawn when a write went through since the last poll, and for nothing else', async () => {
  const p = await page({ state: { lastWrite: { saved: true, at: '2026-11-11T10:00:00Z', queued: 0 } } });
  const mark = p.doc.querySelector('#savedmark');
  assert.equal(mark.hidden, true, 'a write that was made before the page opened is no news');
  await p.poll({ lastWrite: { saved: true, at: '2026-11-11T10:00:00Z', queued: 0 } });
  assert.equal(mark.hidden, true, 'the same write is no news');
  await p.poll({ dirty: true, pending: ['xp +500'], lastWrite: { saved: false, at: '2026-11-11T10:00:30Z', queued: 1, why: 'the game is running' } });
  assert.equal(mark.hidden, true, 'a change that is only waiting is not a write');
  await p.poll({ lastWrite: { saved: true, at: '2026-11-11T10:01:00Z', queued: 0 } });
  assert.equal(mark.hidden, false, 'the queue was written');
  assert.ok(mark.querySelector('.d-fx .chk'));
  assert.match(mark.textContent, /Written/);
  p.close();
  const first = await page();
  await first.poll({ lastWrite: { saved: true, at: '2026-11-11T10:02:00Z', queued: 0 } });
  assert.equal(first.doc.querySelector('#savedmark').hidden, false, 'the first write after a page with no writes is shown');
  first.close();
});

test('an edit that was accepted lights its row once, in Full only, and one that was refused does not', async () => {
  for (const [mode, lit] of [['full', true], ['calm', false], ['off', false]]) {
    const p = await page({ store: { tauMotion: mode } });
    const tr = p.doc.createElement('tr');
    const input = p.doc.createElement('input');
    tr.append(input);
    p.doc.body.append(tr);
    await p.w.editInput(input, async () => ({}), null);
    assert.equal(tr.classList.contains('d-flash'), lit, mode);
    p.close();
  }
  const p = await page({ store: { tauMotion: 'full' } });
  const tr = p.doc.createElement('tr');
  const input = p.doc.createElement('input');
  tr.append(input);
  p.doc.body.append(tr);
  await p.w.editInput(input, async () => { throw new Error('refused'); }, null);
  assert.equal(tr.classList.contains('d-flash'), false);
  p.close();
});

test('a page that never ran the head script still works and has no limit of its own', async () => {
  const p = await page({ script: false });
  assert.equal(p.w.TauMotion, undefined);
  p.w.toast('Still works', false, 5000);
  assert.match(p.doc.querySelector('#toast').textContent, /Still works/);
  await p.poll({ dirty: true, pending: ['a'] });
  await p.poll({ dirty: true, pending: ['a', 'b'] });
  assert.equal(p.doc.querySelector('#pending').classList.contains('x-bump'), true);
  p.close();
});
