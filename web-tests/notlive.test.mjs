// A save that is not the game's own (a copy) is called that wherever the open file is shown (#498): in the header ahead of the file name, in the tab title and in the banner,
// always, not only when something is queued. The game's own save shows nothing of the kind. The state is fed to the real page code (jsdom).
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';

const here = path.dirname(fileURLToPath(import.meta.url));
const static_ = path.join(here, '..', 'internal', 'web', 'static');
const read = (n) => fs.readFileSync(path.join(static_, n), 'utf8');

const BASE = { path: 'C:\\x\\game.db', dirty: false, pending: [], saveBlocked: false, changedOnDisk: false, gameMode: 'none', gameProcesses: [], journalOff: '', held: null, restored: null, lastWrite: null };

async function page(state) {
  const dom = new JSDOM(read('index.html'), { url: 'http://127.0.0.1:8090/', runScripts: 'dangerously', pretendToBeVisual: true });
  const w = dom.window;
  w.fetch = async (p) => {
    const u = new URL(p, 'http://127.0.0.1:8090');
    if (u.pathname === '/api/save/state') return { ok: true, status: 200, statusText: 'OK', json: async () => ({ ...BASE, ...state }) };
    return { ok: false, status: 404, statusText: 'Not Found', json: async () => ({ error: 'not found' }) };
  };
  w.eval(read('html.js'));
  w.eval(read('livemap.js'));
  w.eval(read('app.js'));
  for (let i = 0; i < 30; i++) await new Promise((r) => w.setTimeout(r, 0));
  const doc = w.document;
  const text = (sel) => doc.querySelector(sel).textContent.replace(/\s+/g, ' ').trim();
  const shown = (sel) => !doc.querySelector(sel).classList.contains('hide');
  return { doc, w, text, shown, close: () => w.close() };
}

test('a copy: the header says NOT LIVE: a copy, next to TABR TAU and before the file name, and the tab title says it', async () => {
  const p = await page({ live: false, path: 'C:\\reports\\import-test\\game.db' });
  assert.ok(p.shown('#notlive'), 'visible');
  assert.equal(p.text('#notlive'), 'NOT LIVE: a copy');
  const header = [...p.doc.querySelector('header').children].map((e) => e.id || e.tagName.toLowerCase());
  assert.ok(header.indexOf('h1') < header.indexOf('notlive') && header.indexOf('notlive') < header.indexOf('path'), 'order in the header: ' + header.join(' '));
  assert.equal(p.text('#path'), 'game.db');
  assert.equal(p.doc.querySelector('#path').title, "Not the game's save: C:\\reports\\import-test\\game.db");
  assert.ok(p.doc.title.startsWith('NOT LIVE: a copy - '), p.doc.title);
  p.close();
});

test('a copy: the banner is there with nothing queued, says the file is a copy, that edits are written at once, and gives the path', async () => {
  const p = await page({ live: false, path: 'C:\\reports\\import-test\\game.db' });
  const gw = p.doc.querySelector('#gamewarn');
  assert.ok(!gw.classList.contains('hide'), 'the banner is shown');
  const t = p.text('#gamewarn');
  assert.ok(t.includes("This is a copy, not the game's save: edits are written at once and do not reach the game unless this file is installed as its game.db."), t);
  assert.ok(t.includes('C:\\reports\\import-test\\game.db'), 'the path');
  p.close();
});

test('a copy that cannot be written says so, still as a copy, and does not call it the game file', async () => {
  const p = await page({ live: false, dirty: true, pending: ['solari +5'], saveBlocked: true, blockReason: 'another program has the file open', lastWrite: { saved: false, queued: 1, why: 'another program has the file open' } });
  const t = p.text('#gamewarn');
  assert.ok(t.includes("This is a copy, not the game's save"), t);
  assert.ok(t.includes('1 change is queued') && t.includes('It is written to the file by itself'), t);
  assert.ok(!t.includes('game file'), t);
  assert.ok(p.shown('#notlive'));
  p.close();
});

test("the game's own save: nothing in the header, the title or the banner reads as a copy", async () => {
  const p = await page({ live: true });
  assert.ok(!p.shown('#notlive'), 'the header word is hidden');
  assert.ok(!/copy|not live/i.test(p.doc.title), p.doc.title);
  assert.equal(p.doc.querySelector('#path').title, 'C:\\x\\game.db');
  assert.ok(p.doc.querySelector('#gamewarn').classList.contains('hide'), 'no banner');
  assert.ok(!/copy|not live/i.test(p.text('header') + p.text('#gamewarn')), 'no word of it');
  p.close();
});

test('a state with no live field (an older editor) is treated as the game\'s own: no copy wording', async () => {
  const p = await page({});
  assert.ok(!p.shown('#notlive'));
  assert.ok(!/copy/i.test(p.doc.title));
  p.close();
});

test('the header word is not in a hidden or tooltip-only place: it is a visible element of the header, not a title attribute', async () => {
  const p = await page({ live: false });
  const el = p.doc.querySelector('header #notlive');
  assert.ok(el && el.textContent.trim() === 'NOT LIVE: a copy' && !el.hasAttribute('hidden') && !el.hasAttribute('aria-hidden'));
  p.close();
});
