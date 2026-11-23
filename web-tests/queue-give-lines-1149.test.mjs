// #1149: the list "What is queued" names a queued Give: the item, its quantity, its grade and its augments, one line to an item when several were queued. "Give 1 queued items" is gone.
// The state is fed to the real page code (jsdom), as the other banner tests do.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';

const here = path.dirname(fileURLToPath(import.meta.url));
const static_ = path.join(here, '..', 'internal', 'web', 'static');
const read = (n) => fs.readFileSync(path.join(static_, n), 'utf8');

const BASE = { path: 'C:/x/game.db', dirty: false, pending: [], saveBlocked: false, changedOnDisk: false, gameMode: 'unknown', gameProcesses: [], journalOff: '', held: null, restored: null, lastWrite: null };

async function banner(state) {
  const dom = new JSDOM(read('index.html'), { url: 'http://127.0.0.1:8090/', runScripts: 'dangerously', pretendToBeVisual: true });
  const w = dom.window;
  w.fetch = async (p) => {
    const u = new URL(p, 'http://127.0.0.1:8090');
    if (u.pathname === '/api/save/state') return { ok: true, status: 200, statusText: 'OK', json: async () => ({ ...BASE, ...state }) };
    return { ok: false, status: 404, statusText: 'Not Found', json: async () => ({ error: 'not found' }) };
  };
  w.confirm = () => true;
  w.eval(read('html.js'));
  w.eval(read('livemap.js'));
  w.eval(read('app.js'));
  for (let i = 0; i < 30; i++) await new Promise((r) => w.setTimeout(r, 0));
  const items = [...w.document.querySelectorAll('#gamewarn .bn-list li')].map((li) => ({ text: li.innerHTML.replace(/<br>/g, ' ').replace(/<[^>]*>/g, '').replace(/\s+/g, ' ').trim(), title: li.getAttribute('title'), lines: li.querySelectorAll('br').length }));
  return { items, close: () => w.close() };
}
const blocked = { dirty: true, saveBlocked: true, blockReason: 'the game is running' };

test('a queued Give of one item names it, its quantity, grade and augments', async () => {
  const one = 'Give 1 x Ambition (grade 5, augments: Plate, Weave at grade 3)';
  const b = await banner({ ...blocked, pending: [one], lastWrite: { saved: false, queued: 1, why: 'the game is running' } });
  assert.equal(b.items.length, 1);
  assert.equal(b.items[0].text, one);
  assert.ok(!b.items[0].text.includes('queued items'));
  b.close();
});

test('a queued Give of several items lists each item on its own line, and the tooltip says it all on one line', async () => {
  const many = 'Give 3 items: 2 x Ambition (grade 5, augments: Plate at grade 3); 500 x Iron Bar (grade 0); 1 x Spice Melange (grade 2)';
  const b = await banner({ ...blocked, pending: [many, 'xp +50'], lastWrite: { saved: false, queued: 2, why: 'the game is running' } });
  assert.equal(b.items.length, 2, 'one list entry for each edit');
  assert.equal(b.items[0].lines, 3, 'one line to an item');
  assert.equal(b.items[0].text, 'Give 3 items: 2 x Ambition (grade 5, augments: Plate at grade 3) 500 x Iron Bar (grade 0) 1 x Spice Melange (grade 2)');
  assert.equal(b.items[0].title, many);
  assert.equal(b.items[1].text, 'Add 50 XP', 'the other edits are as they were');
  assert.equal(b.items[1].lines, 0);
  b.close();
});

test('a queued Give that only partly fitted says which line fell short', async () => {
  const short = 'Give 2 items: 340 of 500 x Iron Bar (grade 0); 2 x Spice Melange (grade 2)';
  const b = await banner({ ...blocked, pending: [short], lastWrite: { saved: false, queued: 1, why: 'the game is running' } });
  assert.match(b.items[0].text, /^Give 2 items: 340 of 500 x Iron Bar \(grade 0\) 2 x Spice Melange \(grade 2\)$/);
  b.close();
});
