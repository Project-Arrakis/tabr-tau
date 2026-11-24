// The place text of the Vehicles and Bases lists (#1175): the server says "place" for a thing in the Deep Desert ("Deep Desert, G5"), and the page shows it in place of "near <region>";
// a Hagga Basin row keeps "near <region>". Display only: nothing is written.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const FUEL = { total: 0, notFull: 0, unknown: 0, noFuelRecord: 0 };
const veh = (id, o) => ({ id, name: `BP_Buggy_${id}`, type: 'Buggy', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [], ...o });
const VEHICLES = {
  vehicles: [
    veh(41, { map: 'HaggaBasin', region: 'Hagga Rift' }),
    veh(42, { map: 'DeepDesert', place: 'Deep Desert, G5' }),
    veh(43, { map: 'DeepDesert', place: 'Deep Desert' }),
    veh(44, { map: 'DeepDesert', place: 'Deep Desert, A1', stored: 'recovery', storedLabel: 'In recovery' }),
    veh(45, { map: 'HaggaBasin' }),
  ],
  hidden: 0, recovered: [], backups: [],
};
const base = (id, o) => ({ base_id: id, name: `Base ${id}`, base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 1, placeables: 0, x: 0, y: 0, z: 0, ...o });

const locationOf = (row) => [...row.querySelectorAll('td')].map((c) => c.textContent.replace(/\s+/g, ' ').trim()).find((t) => /^-?[\d,.]+, -?[\d,.]+/.test(t));

test('the Vehicles list says the place of a Deep Desert vehicle, keeps near <region> in the Hagga Basin and shows no place text without either', async () => {
  const ui = await boot({ get: { '/api/vehicles': VEHICLES, '/api/vehicles/fuel': FUEL } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  const where = {};
  for (const b of ui.doc.querySelectorAll('[data-vhopen]')) where[b.dataset.vhopen] = locationOf(b.closest('tr'));
  assert.match(where[41], /1, 2 near Hagga Rift$/);
  assert.match(where[42], /1, 2 Deep Desert, G5$/);
  assert.ok(!where[42].includes('near'), where[42]);
  assert.match(where[43], /1, 2 Deep Desert$/);
  assert.match(where[44], /1, 2 Deep Desert, A1$/);
  assert.match(where[45], /^1, 2$/);
  const stored = ui.doc.querySelector('[data-vhopen="44"]').closest('tr');
  assert.ok(stored.textContent.includes('In recovery') || stored.querySelector('.tag'), 'the stored tag stays');
  assert.equal(ui.posted.length, 0, 'drawing the list writes nothing');
  ui.dom.window.close();
});

test('the Bases list says the place of a Deep Desert base and keeps near <region> in the Hagga Basin', async () => {
  const ui = await boot({ get: { '/api/bases/list': [base(7, { region: 'Hagga Basin South' }), base(8, { map: 'DeepDesert', place: 'Deep Desert, C2' })], '/api/bases/backups': [] } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  const text = ui.doc.querySelector('#main').textContent.replace(/\s+/g, ' ');
  assert.ok(text.includes('near Hagga Basin South'), text);
  assert.ok(text.includes('Deep Desert, C2') && !text.includes('near Deep Desert'), text);
  ui.dom.window.close();
});
