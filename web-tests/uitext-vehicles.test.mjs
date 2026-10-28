// The text sweep (#426) on Player > Vehicles. See uitext.test.mjs for the checks that hold on every page.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { visibleText, movedText } from './uitext.mjs';

test.afterEach(() => closeAll());

const VEH = (o = {}) => ({ vehicles: [{ id: 41, name: 'BP_Sandbike_CHOAM', type: 'Sandbike', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [{ template_id: 'Engine_1', percent: 80, current: 80, max: 100 }], cargo: null }], hidden: 0, recovered: [], backups: [], ...o });
const FUEL = { total: 5, notFull: 3, unknown: 1, noFuelRecord: 0 };
const open = async (ui) => { ui.click('[data-tab="player"]'); await ui.settle(); ui.click('[data-sub="player:vehicles"]'); await ui.settle(); return ui.doc.querySelector('#main'); };

test('Vehicles: the repair and refuel explanations are tooltips, the tank sizes and the module note are in About this, the counts and state stay', async () => {
  const ui = await boot({ get: { '/api/vehicles': VEH(), '/api/vehicles/fuel': FUEL } });
  const main = await open(ui);
  const shown = visibleText(main), moved = movedText(main);
  for (const s of ['Makes every module as good as new: full durability', 'A module whose full durability is not known is raised to the maximum it records',
    'Fills the fuel of your own vehicles to a full tank', 'Vehicles of other owners and the world\'s vehicles are not touched', 'A tank follows the vehicle\'s chassis grade', 'Sandbike 90 to 150, Buggy 220 to 300',
    'Condition is each module\'s durability against its current maximum']) {
    assert.ok(!shown.includes(s), 'still shown: ' + s);
    assert.ok(moved.includes(s), 'lost, not moved: ' + s);
  }
  for (const s of ['Your vehicles (1)', 'Repair all my vehicles', 'Only modules below', 'Refuel all my vehicles', '3 of 5 vehicles are not full.', '1 vehicle of an unknown kind will be left alone.', 'Sandbike: 1 components']) assert.ok(shown.includes(s), 'missing from the default view: ' + s);
  ui.dom.window.close();
});

test('Vehicles: the line about vehicles that are not yours is in About this and only drawn when there are some; the duplicate empty line is gone', async () => {
  let ui = await boot({ get: { '/api/vehicles': VEH({ hidden: 2 }), '/api/vehicles/fuel': FUEL } });
  let main = await open(ui);
  assert.ok(!visibleText(main).includes('other vehicles in the world'));
  assert.ok(movedText(main).includes('2 other vehicles in the world (not yours) are not shown.'));
  ui.dom.window.close();

  ui = await boot({ get: { '/api/vehicles': VEH({ hidden: 0 }), '/api/vehicles/fuel': FUEL } });
  main = await open(ui);
  assert.ok(!movedText(main).includes('not yours'), 'none hidden: no line');
  assert.equal(ui.doc.querySelector('details[data-about="vehicles.list"]'), null);
  ui.dom.window.close();

  ui = await boot({ get: { '/api/vehicles': VEH({ vehicles: [] }), '/api/vehicles/fuel': { total: 0, notFull: 0, unknown: 0, noFuelRecord: 0 } } });
  main = await open(ui);
  const t = visibleText(main);
  assert.ok(t.includes('You have no vehicles in this save yet.'));
  assert.ok(!t.includes('None of your vehicles are in this save yet.'), 'said once, not twice');
  ui.dom.window.close();
});
