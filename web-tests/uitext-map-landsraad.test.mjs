// The text sweep (#426) on the Saved map and the Landsraad. See uitext.test.mjs for the checks that hold on every page.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { visibleText, movedText } from './uitext.mjs';

test.afterEach(() => closeAll());

const here = path.dirname(fileURLToPath(import.meta.url));
const FIX = JSON.parse(fs.readFileSync(path.join(process.env.UI_FIXTURES_DIR || path.join(here, 'fixtures'), 'ui-fixtures.json'), 'utf8'));
const TIERS = [1200, 2400, 3600, 4800, 6000];
const house = (id, name, mine) => ({
  id, board_index: id - 900, house_name: 'DA_House' + name, goal_amount: 4000, completed: 0, contributions: [], player_contributions: [], my_contribution: mine, my_recorded: mine > 0,
  rewards: TIERS.map((threshold) => ({ threshold, amount: 1, template_id: 'Item.A', reached: mine >= threshold, delivered: false })),
});
const board = (faction) => ({
  term: { term_id: 7, start_time: 1, end_time: 2, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 }, factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  character: { player_id: 1, name: 'Tester', faction_id: faction }, decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }], rotation: [], tasks: [house(900, 'Alexin', 0), house(901, 'Thorvald', 1300)],
});
const go = async (ui, tab) => { ui.click(`[data-tab="${tab}"]`); await ui.settle(); return ui.doc.querySelector('#main'); };

test('Landsraad: the nine-sentence explanation of the scales is in About this, with its words; the faction line is drawn only when there is no faction', async () => {
  let ui = await boot({ get: { '/api/landsraad': board(1) } });
  let main = await go(ui, 'landsraad');
  const shown = visibleText(main), moved = movedText(main);
  for (const s of ['Two different scales are shown for each house', 'Whether the game hands out a tier when it loads the save after a fill is not known from a save']) { assert.ok(moved.includes(s), 'lost, not moved: ' + s); }
  assert.ok(!shown.includes('Two different scales are shown for each house'));
  assert.ok(ui.doc.querySelector('#lscales').closest('details.about'));
  assert.equal(ui.doc.querySelector('#lsnofaction'), null, 'the character has a faction: no line');
  for (const s of ['Task board (2)', 'Nothing is chosen.', 'Faction for Fill faction goal and Complete', 'Fill chosen', 'Choose all houses']) assert.ok(shown.includes(s), 'missing: ' + s);
  assert.ok(!shown.includes('Choose houses (all their tiers) or single tiers, then Fill chosen'));
  assert.ok(!shown.includes('used by the buttons below'));
  assert.ok(ui.doc.querySelector('[data-act="tfill"]').title.includes('A personal contribution and the reward tiers are not changed'), 'the sentence about Fill faction goal is on the button');
  assert.equal(ui.doc.querySelector('[data-act="tdone"]').title, 'Marks the house taken, for the chosen faction.');
  ui.dom.window.close();

  ui = await boot({ get: { '/api/landsraad': board(null) } });
  main = await go(ui, 'landsraad');
  assert.ok(visibleText(main).includes('Your character is in no faction (Neutral), so there is nothing to fill: assign Atreides or Harkonnen under Admin first.'), 'why Fill is off stays in view');
  assert.ok(!ui.doc.querySelector('#lscales').textContent.includes('Neutral'), 'it is no longer inside the explanation');
  ui.dom.window.close();
});

test('Saved map: the first sentence stays, the hint and the reasons are tooltips or in About this, the readout line is empty until a marker is hovered', async () => {
  const lm = { ...FIX['/api/livemap'].body, storedVehicles: 2 };
  const ui = await boot({ get: { '/api/livemap': lm } });
  const main = await go(ui, 'livemap');
  const shown = visibleText(main), moved = movedText(main);
  assert.ok(shown.includes('The save as of its last write, not the running game.'));
  assert.ok(shown.includes('2 of your vehicles are stored (in the vehicle backup or waiting for recovery) and not shown.'));
  for (const s of ['Markers you have not discovered yet are shown faded', 'Hover a marker for details. Drag to move, scroll or press + and - to zoom, 0 to fit.', 'the game keeps only the place a stored vehicle was last in the world'.replace('the game', 'The game')]) {
    assert.ok(!shown.includes(s), 'still shown: ' + s);
    assert.ok(moved.includes(s), 'lost, not moved: ' + s);
  }
  assert.equal(ui.doc.querySelector('#lminfo').textContent, '', 'nothing hovered, nothing said');
  assert.equal(ui.doc.querySelector('[data-lmund]').getAttribute('aria-describedby'), 'lmundtip');
  assert.ok(ui.doc.querySelector('[data-lmund]').closest('.tip'));
  ui.dom.window.close();
});

test('Saved map: the sand-field caveat is a tooltip on the Spice and Flour layer buttons, and the buttons still toggle', async () => {
  const base = FIX['/api/livemap'].body;
  const lm = { ...base, resourceFields: [{ kind: 'spice', size: 'Large', x: 1, y: 1, left: 1 }, ...(base.resourceFields || [])] };
  const ui = await boot({ get: { '/api/livemap': lm } });
  const main = await go(ui, 'livemap');
  const btns = [...ui.doc.querySelectorAll('.lm-layer')].filter((b) => /spice|flour/i.test(b.dataset.lmg));
  for (const b of btns) {
    assert.ok(b.closest('.tip'), b.dataset.lmg + ' has the caveat as a tooltip');
    assert.ok(ui.doc.getElementById(b.getAttribute('aria-describedby')).textContent.includes('have not been checked against the game for this map'));
    const was = b.getAttribute('aria-pressed');
    ui.click(`[data-lmg="${b.dataset.lmg}"]`);
    assert.notEqual(b.getAttribute('aria-pressed'), was, 'it still toggles');
  }
  assert.ok(!visibleText(main).includes('placed by decoding their ids'));
  ui.dom.window.close();
});
