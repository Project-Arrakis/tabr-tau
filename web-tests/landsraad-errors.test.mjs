// Landsraad reads and writes that fail say so (#611): a failed read of the board shows its error and no board, and a Fill faction goal that the editor refuses (a faction the character
// is not in) shows the editor's reason and no success line.
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

test('Fill faction goal for a faction the character is not in shows the editor\'s reason and no success line', async () => {
  const msg = 'your character is not in Atreides, so nothing was changed: Fill faction goal records your own contribution under the faction, and the editor writes no contribution row of yours for a faction your character is not in.';
  const board = {
    term: { term_id: 7, start_time: 1, end_time: 2, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 },
    factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
    character: { player_id: 1, name: 'Tester', faction_id: 2 },
    decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }], rotation: [],
    tasks: [{ id: 900, board_index: 0, house_name: 'DA_HouseAlexin', goal_amount: 4000, completed: 0, winning_faction_id: null, completion_time: null, contributions: [], player_contributions: [], my_contribution: 0, my_recorded: false, rewards: [] }],
  };
  const ui = await boot({ get: { '/api/landsraad': board }, post: { '/api/landsraad/faction-goal': { __fail: msg, __status: 422, __code: 'bad_argument' } } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  ui.doc.querySelector('#lf').value = '1'; // Atreides, not the character's own faction
  ui.click('[data-act="tfill"][data-id="900"]'); await ui.settle();
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('not in Atreides') && t.includes('nothing was changed') && !t.includes('completed'), t);
  const sent = ui.posted.filter((p) => p.path === '/api/landsraad/faction-goal');
  assert.equal(sent.length, 1);
  assert.equal(sent[0].body.faction_id, 1);
});
