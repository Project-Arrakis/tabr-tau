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

test('Landsraad: the long explanations of the board are in About this, with their words, and the short lines and the buttons stay in view', async () => {
  const ui = await boot({ get: { '/api/landsraad': board(1) } });
  const main = await go(ui, 'landsraad');
  const shown = visibleText(main), moved = movedText(main);
  for (const s of ['contribution is cumulative, so', 'Each click is written at once']) { assert.ok(moved.includes(s), 'lost, not moved: ' + s); assert.ok(!shown.includes(s), 'still shown: ' + s); }
  for (const s of ['Nothing is chosen.', 'Faction for Fill and Complete', 'Fill chosen', 'Complete chosen', 'Reopen chosen', 'Choose all houses', 'Clear the choice']) assert.ok(shown.includes(s), 'missing: ' + s);
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
