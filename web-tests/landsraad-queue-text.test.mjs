// A queued Landsraad edit says which houses and which faction it is about (D1-6, #611), and the message of a held queue says that the changes have to be made again (D1-8).
// The state is fed to the real page code (jsdom).
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
  const gw = w.document.querySelector('#gamewarn');
  return { text: gw.textContent.replace(/\s+/g, ' ').trim(), close: () => w.close() };
}

test('queued Landsraad edits are listed with their houses and factions', async () => {
  const b = await banner({
    dirty: true, saveBlocked: true, blockReason: 'could not check whether the game is running (exit status 1: ERROR: Call cancelled)',
    pending: [
      'landsraad Kenola: completed for Harkonnen; replaces the win of Atreides on Kenola',
      'landsraad: 3 houses completed for Atreides; replaces 2 wins (Harkonnen: Maros, Kenola); 2 other completion times moved; houses: Maros, Kenola, Vernius',
      'landsraad: 1 house reopened; houses: Kenola',
      'landsraad: 4 houses changed from the board; houses: A, B, C, D',
      'landsraad Vernius: reopened',
      'landsraad Kenola: your contribution 1300 -> 6000',
      'landsraad: 2 other completion times moved',
      'landsraad term 7: end_time = 250',
    ],
    lastWrite: { saved: false, queued: 8, why: 'could not check whether the game is running (exit status 1: ERROR: Call cancelled)' },
  });
  for (const want of [
    'Kenola (Landsraad): Completed for Harkonnen; replaces the win of Atreides on Kenola',
    'Complete 3 Landsraad houses for Atreides: Maros, Kenola, Vernius; replaces 2 wins (Harkonnen: Maros, Kenola); 2 other completion times moved',
    'Reopen 1 Landsraad house: Kenola',
    'Change 4 Landsraad houses from the board: A, B, C, D',
    'Vernius (Landsraad): Reopened',
    'Kenola (Landsraad): Your contribution 1300 -> 6000',
    'Landsraad: 2 other completion times moved',
    'Landsraad term 7: end_time = 250',
  ]) assert.ok(b.text.includes(want), want + ' in: ' + b.text);
  assert.ok(!b.text.includes('houses: '), 'the internal marker is not shown');
  b.close();
});

test('a held queue says the changes have to be made again', async () => {
  const b = await banner({
    dirty: true, pending: ['landsraad Kenola: completed for Harkonnen'],
    held: { conflicts: [{ desc: 'landsraad Kenola: completed for Harkonnen', why: 'the game changed what it was worked out from' }] },
  });
  assert.match(b.text, /was not written/);
  assert.match(b.text, /Discard the queue and make the change again\./);
  b.close();
});

test('the held and the restored banners show the conflicts in words too', async () => {
  const d = 'landsraad: 2 houses completed for Atreides; houses: Maros, Kenola';
  const h = await banner({ dirty: true, pending: [d], held: { conflicts: [{ desc: d, why: 'the game changed what it was worked out from' }] } });
  assert.ok(h.text.includes('Complete 2 Landsraad houses for Atreides: Maros, Kenola: the game changed'), h.text);
  assert.ok(!h.text.includes('houses: '), h.text);
  h.close();
  const r = await banner({ restored: { applied: 0, conflicts: [{ desc: d, why: 'the save no longer accepts it' }], rebased: true, written: false } });
  assert.ok(r.text.includes('Could not put back: Complete 2 Landsraad houses for Atreides: Maros, Kenola (the save no longer accepts it)'), r.text);
  r.close();
});
