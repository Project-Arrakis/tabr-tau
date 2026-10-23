// The game check buttons (#366): Check again and Write now make the check at once and say what it found; a banner about a check that could not be made offers both, and
// goes away when the next state read says the check works.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const state = (over = {}) => ({
  path: 'C:/x/game.db', dirty: false, pending: [], gameRunning: false, gameProcesses: [], gameCheckError: '', saveBlocked: false, blockReason: '', gameMode: 'none',
  readOnly: false, changedOnDisk: false, locationUnconfirmed: false, restored: null, journalOff: '', writeLogProblem: '', lastWrite: null, held: null, ...over,
});
const failing = state({ gameCheckError: 'process snapshot: Access is denied.', saveBlocked: true, blockReason: 'could not check whether the game is running (process snapshot: Access is denied.)', gameMode: 'unknown' });
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);
const toastText = (ui) => ui.doc.querySelector('#toast').textContent.replace(/×$/, '').trim(); // the close mark is part of the toast's text

test('a check that could not be made offers Check again and Write now and says the editor tries again by itself', async () => {
  const ui = await boot({ get: { '/api/save/state': failing } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.textContent.includes('Changes cannot be written') && gw.textContent.includes('checks again by itself'), gw.textContent);
  assert.ok(gw.querySelector('[data-act="checkAgain"]'), 'Check again is offered');
  assert.ok(gw.querySelector('[data-act="writeNow"]'), 'Write now is still offered');
});

test('Check again posts the check, says the result in a toast, and reads the state again', async () => {
  const ui = await boot({
    get: { '/api/save/state': failing },
    post: { '/api/game/check': { running: false, failed: true, blocked: true, processes: [], checkError: 'process snapshot: Access is denied.', message: 'the check failed again: process snapshot: Access is denied.' } },
  });
  const before = ui.fetched.filter((p) => p === '/api/save/state').length;
  ui.click('[data-act="checkAgain"]'); await ui.settle();
  assert.equal(posts(ui, '/api/game/check').length, 1, 'the check is asked for once');
  assert.equal(posts(ui, '/api/save/flush').length, 0, 'Check again writes nothing itself');
  assert.equal(toastText(ui), 'The check failed again: process snapshot: Access is denied.');
  assert.ok(ui.fetched.filter((p) => p === '/api/save/state').length > before, 'the banner is redrawn from a fresh state read');
});

test('the banner is gone when the state read after a check that worked says so', async () => {
  const get = { '/api/save/state': failing };
  const ui = await boot({ get, post: { '/api/game/check': { running: false, failed: false, blocked: false, processes: [], checkError: '', message: 'the game is not running: edits are written at once' } } });
  assert.ok(ui.doc.querySelector('#gamewarn').textContent.includes('Changes cannot be written'));
  get['/api/save/state'] = state(); // the next check works
  ui.click('[data-act="checkAgain"]'); await ui.settle();
  assert.equal(toastText(ui), 'The game is not running: edits are written at once');
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.classList.contains('hide') && !gw.textContent.includes('Changes cannot be written'), 'the banner cleared without a restart: ' + gw.className + ' ' + gw.textContent);
});

test('Write now with nothing waiting runs the check and says its result, never just "Nothing was waiting"', async () => {
  const ui = await boot({
    get: { '/api/save/state': failing },
    post: {
      '/api/game/check': { running: false, failed: true, blocked: true, processes: [], checkError: 'boom', message: 'the check failed again: boom' },
      '/api/save/flush': { queued: 0, saved: false },
    },
  });
  ui.click('[data-act="writeNow"]'); await ui.settle();
  assert.equal(posts(ui, '/api/game/check').length, 1, 'Write now makes the check at once');
  assert.equal(posts(ui, '/api/save/flush').length, 1, 'and still asks for another attempt to write');
  assert.equal(toastText(ui), 'The check failed again: boom');
});

test('Write now with edits waiting names the failed check as the reason', async () => {
  const q = { ...failing, dirty: true, pending: ['solari +250'] };
  const ui = await boot({
    get: { '/api/save/state': q },
    post: {
      '/api/game/check': { running: false, failed: true, blocked: true, processes: [], checkError: 'boom', message: 'the check failed again: boom' },
      '/api/save/flush': { queued: 1, saved: false, why: 'could not check whether the game is running (boom)' },
    },
  });
  ui.click('[data-act="writeNow"]'); await ui.settle();
  assert.equal(toastText(ui), 'Still waiting: the check failed again: boom');
});

test('Write now when the game runs in a session says the edits will queue', async () => {
  const q = state({ gameRunning: true, gameProcesses: ['DuneSandbox.exe'], saveBlocked: true, blockReason: 'a single-player session is active (DuneSandbox.exe)', gameMode: 'single_player', dirty: true, pending: ['solari +250'] });
  const ui = await boot({
    get: { '/api/save/state': q },
    post: {
      '/api/game/check': { running: true, failed: false, blocked: true, processes: ['DuneSandbox.exe'], checkError: '', message: 'the game is running: edits will queue' },
      '/api/save/flush': { queued: 1, saved: false, why: 'a single-player session is active (DuneSandbox.exe)' },
    },
  });
  ui.click('[data-act="writeNow"]'); await ui.settle();
  assert.ok(toastText(ui).startsWith('Still waiting: a single-player session is active'), toastText(ui));
});
