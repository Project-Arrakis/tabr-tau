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

test('Fill faction goal for a faction the character is not in says the faction won and that the personal contribution was not recorded', async () => {
  const board = {
    term: { term_id: 7, start_time: 1, end_time: 2, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 },
    factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
    character: { player_id: 1, name: 'Tester', faction_id: 2 },
    decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }], rotation: [],
    tasks: [{ id: 900, board_index: 0, house_name: 'DA_HouseAlexin', goal_amount: 4000, completed: 0, winning_faction_id: null, completion_time: null, contributions: [], player_contributions: [], my_contribution: 0, my_recorded: false, rewards: [] }],
  };
  const answer = { ok: true, changed: 1, noop: false, house: 'Alexin', faction: 'Atreides', tier: 6000, progress_changed: true, completed_changed: true, contribution_changed: false, contribution_before: 0, has_character: true, personal_skipped: true, outcome: 'written' };
  const ui = await boot({ get: { '/api/landsraad': board }, post: { '/api/landsraad/faction-goal': answer } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  const sel = ui.doc.querySelector('#lf');
  assert.equal(sel.value, '2', 'the list starts on the character\'s own faction (Harkonnen)');
  const note = ui.doc.querySelector('#lsfaceffect').textContent;
  assert.ok(/own faction also records your contribution/.test(note) && /another faction it records that faction's win and progress and skips your own contribution/.test(note), note);
  sel.value = '1'; // Atreides, not the character's own faction
  ui.click('[data-act="tfill"][data-id="900"]'); await ui.settle();
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Atreides won this house; your own contribution was not recorded because your character is not in Atreides') && !t.includes('your contribution 0'), t);
  const sent = ui.posted.filter((p) => p.path === '/api/landsraad/faction-goal');
  assert.equal(sent.length, 1);
  assert.equal(sent[0].body.faction_id, 1);
});
