// Landsraad page of #611: a failed read of the board shows its error and no board; Fill faction goal for a faction the character is not in says that the faction won and
// that the personal contribution was not recorded, and the faction list starts on the character's own faction.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const REASON = 'The Landsraad page could not read the reward tiers from the save: SQL logic error: no such table: landsraad_task_rewards (1)';

test('a Landsraad read that fails shows its error in place of the board, with a way to try again', async () => {
  const ui = await boot({ get: { '/api/landsraad': { __fail: REASON, __status: 500 } } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  const main = ui.doc.querySelector('#main');
  assert.ok(main.textContent.includes('could not read the reward tiers'), main.textContent);
  assert.ok(main.querySelector('.bad'), 'the error is drawn as an error');
  assert.ok(main.querySelector('[data-act="retry"]'), 'Try again is offered');
  assert.equal(main.querySelectorAll('[data-act="tfill"]').length, 0, 'no Fill or Complete is offered against a board that was not read');
  assert.ok(!/This save has no Landsraad tasks/.test(main.textContent), 'it does not read as an empty board');
});
