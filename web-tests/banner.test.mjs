// The banner under the header says what became of the queued changes: one banner, the queue in words, what a reload dropped (kept after the others are written), and a
// queue that cannot be written. The state is fed to the real page code (jsdom); everything else the page asks for is not found, which the banner does not need.
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
  return { text: gw.textContent.replace(/\s+/g, ' ').trim(), gw, doc: w.document, w, close: () => w.close() };
}

test('a queue is one banner with the reason said once and the changes listed in words', async () => {
  const b = await banner({
    dirty: true, saveBlocked: true, blockReason: 'could not check whether the game is running (exit status 1: ERROR: Call cancelled)',
    pending: ['solari +10000', 'xp +500'], lastWrite: { saved: false, queued: 2, why: 'could not check whether the game is running (exit status 1: ERROR: Call cancelled)' },
  });
  assert.equal(b.gw.querySelectorAll('.bn-list').length, 1);
  assert.match(b.text, /2 changes are queued/);
  assert.match(b.text, /Add 10,000 Solari to the backpack/);
  assert.match(b.text, /Add 500 XP/);
  assert.equal((b.text.match(/Could not tell whether Dune: Awakening is running/g) || []).length, 1, 'the reason is said once');
  assert.ok(!/solari \+10000/.test(b.gw.querySelector('.bn-list').textContent), 'no internal wording in the list');
  b.close();
});

test('the notice of a change a reload dropped stays after the others were written, until it is dismissed', async () => {
  const b = await banner({ restored: { applied: 1, conflicts: [{ desc: 'add row 3', why: 'the save no longer accepts it' }], rebased: true, written: true } });
  assert.match(b.text, /1 change was put back onto the newer file the game saved\. It has been written\./);
  assert.match(b.text, /Could not put back: add row 3 \(the save no longer accepts it\)\. Make it again\./);
  assert.ok(b.doc.querySelector('[data-act="dismissRestored"]'), 'it can be dismissed');
  assert.equal(b.gw.className.includes('warn'), true);
  b.close();
});

test('a reload that put nothing back says so instead of "0 changes were put back"', async () => {
  const b = await banner({ restored: { applied: 0, conflicts: [{ desc: 'add row 3', why: 'it changed 0 rows now and 1 before' }], rebased: true, written: true } });
  assert.ok(!/0 changes/.test(b.text));
  assert.match(b.text, /A queued change was not kept/);
  assert.match(b.text, /Could not put back: add row 3/);
  b.close();
});

test('a queue that was not kept and cannot be put back says it cannot be written, with the way out', async () => {
  const b = await banner({
    dirty: true, pending: ['solari +5'], journalOff: 'an edit ran a statement the journal does not keep (x)',
    lastWrite: { saved: false, queued: 1, stuck: true, why: 'the pending edits were not kept (an edit ran a statement the journal does not keep (x)): they cannot be put back onto the game\'s newer file; use Discard to read the file again' },
  });
  assert.match(b.text, /1 queued change cannot be written/);
  assert.match(b.text, /did not keep it/);
  assert.ok(!/written to the game file by itself/.test(b.text), 'the promise of a later write is not made');
  assert.ok(b.doc.querySelector('[data-act="discardQueue"]'), 'Discard queue is offered');
  b.close();
});

test('a held queue lists the conflicts and offers Discard queue, once', async () => {
  const b = await banner({ dirty: true, pending: ['Add row 3'], held: { conflicts: [{ desc: 'add row 3', why: 'the game used that id' }] }, lastWrite: { saved: false, queued: 1, why: 'x' } });
  assert.match(b.text, /1 queued change was not written/);
  assert.match(b.text, /add row 3: the game used that id/);
  assert.equal(b.doc.querySelectorAll('[data-act="discardQueue"]').length, 1);
  b.close();
});

test('a write that could not be recorded in the write log says so, and that the game file was written', async () => {
  const b = await banner({ writeLogProblem: 'the write could not be recorded in the write log (open write-log.jsonl: access is denied)' });
  assert.equal(b.gw.className, 'warn');
  assert.match(b.text, /The write could not be recorded in the write log \(open write-log\.jsonl: access is denied\)\. The game's file was written; only the record of it is missing\./);
  b.close();
});

test('with nothing wrong with the write log the banner stays hidden', async () => {
  const b = await banner({ writeLogProblem: '' });
  assert.equal(b.gw.className, 'hide');
  b.close();
});
