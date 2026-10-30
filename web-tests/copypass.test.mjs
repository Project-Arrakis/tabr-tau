// A copy (a save that is not the game's own) is never refilled by the automatic pass (#508): the Automatic refill card says "Not run: this is a copy", the words about a write
// say "this file" and not "the game", and the Restore list marks the backups of a copy as made from a copy. The manual Fill and Refill buttons stay.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const JOBS = [{ id: 'water', label: 'Base water', desc: 'Fills Water Cisterns to capacity.', key: 'autoRefillWater', ready: true, reason: '', on: true }];
const prefs = (extra = {}) => ({ autoRefillOnOpen: true, jobs: JOBS, last: '', lastProblem: '', problem: '', copy: false, ...extra });
const STATE = (extra = {}) => ({ path: 'C:\\x\\game.db', dirty: false, pending: [], saveBlocked: false, changedOnDisk: false, gameMode: 'none', gameProcesses: [], journalOff: '', held: null, restored: null, lastWrite: null, ...extra });
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();

async function openBases(ui) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
}

test('On a copy the Automatic refill card says the pass is not run, and keeps the Run button that explains why', async () => {
  const ui = await boot({ get: { '/api/settings': prefs({ copy: true, waiting: true, waitingWhy: 'x' }), '/api/save/state': STATE({ live: false }) } });
  await openBases(ui);
  const c = ui.doc.querySelector('#autocopy');
  assert.ok(c, 'the card says so');
  assert.ok(text(c).startsWith('Not run: this is a copy'), text(c));
  assert.equal(ui.doc.querySelector('#autowait'), null, 'a copy does not wait for the game');
  assert.ok(ui.doc.querySelector('[data-act="autoNow"]'), 'the button stays: it answers why');
});

test("On the game's own save the card says nothing about a copy", async () => {
  const ui = await boot({ get: { '/api/settings': prefs(), '/api/save/state': STATE({ live: true }) } });
  await openBases(ui);
  assert.equal(ui.doc.querySelector('#autocopy'), null);
});

test('The Restore list marks the backups of a copy as made from a copy, and the question names it', async () => {
  const rows = [{ name: 'game-1.db', size: 10, modified: '2026-10-30T10:00:00Z', copy: true }, { name: 'game-2.db', size: 10, modified: '2026-10-30T09:00:00Z', copy: false }];
  const ui = await boot({ get: { '/api/save/backups': rows, '/api/save/state': STATE({ live: false }) } });
  ui.click('[data-tab="db"]'); await ui.settle();
  ui.click('[data-sub="db:backups"]'); await ui.settle();
  const trs = [...ui.doc.querySelectorAll('tbody tr')];
  assert.equal(trs.length, 2);
  assert.ok(text(trs[0]).includes('made from a copy'), text(trs[0]));
  assert.ok(!text(trs[1]).includes('made from a copy'), text(trs[1]));
  ui.click('[data-act="restore"][data-n="game-1.db"]'); await ui.settle();
  const body = text(ui.doc.querySelector('#ask'));
  assert.ok(body.includes('made from a copy') && !body.includes('Replace game.db'), body);
});

test('Write now says "this file" for a copy and "the game file" for the game\'s own save', async () => {
  for (const [live, want] of [[false, 'Written to this file.'], [true, 'Written to the game file.']]) {
    const ui = await boot({
      get: { '/api/save/state': STATE({ live, dirty: true, pending: ['solari +5'], saveBlocked: true, blockReason: 'another program has the file open', lastWrite: { saved: false, queued: 1, why: 'another program has the file open' } }) },
      post: { '/api/game/check': { message: 'ok', failed: false }, '/api/save/flush': { saved: true } },
    });
    ui.click('[data-act="writeNow"]'); await ui.settle();
    const t = text(ui.doc.querySelector('#toast'));
    assert.ok(t.includes(want) && (live || !t.includes('game file')), `${live}: ${t}`);
    ui.dom.window.close();
  }
});
