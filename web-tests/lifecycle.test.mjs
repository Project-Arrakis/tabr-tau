// Lifecycle state of vehicles and bases (#403): a stored vehicle is labelled and cannot be brought or duplicated, the Saved map says how many are not shown,
// and a base picked up with the base tool is labelled.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const veh = (id, extra) => ({ id, name: 'BP_Sandbike_CHOAM', type: 'Sandbike', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [], cargo: null, ...extra });

test('a stored vehicle is labelled and its Bring and Duplicate buttons are off', async () => {
  const ui = await boot({ get: { '/api/vehicles': { vehicles: [veh(1), veh(2, { stored: 'backup', storedLabel: 'In the vehicle backup' })], hidden: 0, recovered: [], backups: [] },
    '/api/vehicles/fuel': { total: 2, notFull: 0, unknown: 0, noFuelRecord: 0 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  const rows = [...ui.doc.querySelectorAll('#main table tbody tr')];
  const stored = rows.find((r) => r.textContent.includes('In the vehicle backup'));
  assert.ok(stored, 'the stored vehicle carries its label');
  assert.equal(stored.querySelector('[data-act="bring"]').disabled, true);
  assert.equal(stored.querySelector('[data-act="dupV"]').disabled, true);
  const plain = rows.find((r) => !r.textContent.includes('In the vehicle backup'));
  assert.equal(plain.querySelector('[data-act="bring"]').disabled, false);
  assert.equal(plain.querySelector('[data-act="dupV"]').disabled, false);
});

test('the bases list labels a picked-up base', async () => {
  const base = (id, extra) => ({ base_id: id, name: 'Home ' + id, base_type: 'Sub-Fief', owner: 'Me', map: 'HaggaBasin', generators: 0, pieces: 1, placeables: 1, x: 1, y: 2, z: 3, picked_up: false, ...extra });
  const ui = await boot({ get: { '/api/bases/list': [base(1), base(2, { picked_up: true })] } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  const rows = [...ui.doc.querySelectorAll('#main table tbody tr')].map((r) => r.textContent);
  assert.ok(rows.some((r) => r.includes('Home 2') && r.includes('Picked up')), rows.join(' | '));
  assert.ok(rows.some((r) => r.includes('Home 1') && !r.includes('Picked up')));
});

test('the Saved map says how many stored vehicles it does not show', async () => {
  const cfg = { key: 'HaggaBasin', label: 'Hagga Basin', image: '/maps/hagga-basin.png', width: 4096, height: 4096, minX: -456752, maxX: 354547, minY: -450630, maxY: 353821 };
  const data = (n) => ({ config: cfg, maps: [{ key: 'HaggaBasin', label: 'Hagga Basin' }], character: { name: 'Me', x: 0, y: 0, z: 0 }, vehicles: [], bases: [], storage: [], resourceFields: [], markers: [], storedVehicles: n });
  const ui = await boot({ get: { '/api/livemap': data(2) } });
  ui.click('[data-tab="livemap"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#main').textContent.includes('2 of your vehicles are stored'));
  const none = await boot({ get: { '/api/livemap': data(0) } });
  none.click('[data-tab="livemap"]'); await none.settle();
  assert.ok(!none.doc.querySelector('#main').textContent.includes('are stored'));
});

// #528: the card lists every stored vehicle (recovered, in the vehicle backup, stored by state only) with its kind, and its count is the rows shown.
const sv = (id, kind, extra) => ({ vehicle_id: id, kind, label: kind === 'backup' ? 'In the vehicle backup' : 'Stored for recovery', type: 'Buggy', ...extra });
const storedCard = (ui) => [...ui.doc.querySelectorAll('#main .card')].find((c) => c.querySelector('h3') && c.querySelector('h3').textContent.startsWith('Recovered and stored vehicles'));

test('the stored vehicles card lists the recovered, the backed-up and the state-only vehicle, with a kind column and a count of the rows', async () => {
  const stored = [sv(301, 'recovery', { vehicle_name: 'BP_Buggy_CHOAM', chassis_durability: 0.5, time_stored: 1700000000, reason: 1 }),
    sv(302, 'backup', { type: 'LightOrnithopter' }), sv(303, 'backup', { type: 'Sandbike' })];
  const ui = await boot({ get: { '/api/vehicles': { vehicles: [veh(303, { stored: 'backup', storedLabel: 'In the vehicle backup' })], hidden: 0, recovered: [], backups: [], stored },
    '/api/vehicles/fuel': { total: 1, notFull: 0, unknown: 0, noFuelRecord: 0 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  const card = storedCard(ui);
  assert.ok(card, 'the card is drawn');
  assert.equal(card.querySelector('h3').textContent, 'Recovered and stored vehicles (3)');
  assert.ok(!card.textContent.includes('No vehicle has been recovered or stored.'));
  const heads = [...card.querySelectorAll('th')].map((h) => h.textContent);
  assert.ok(heads.includes('Kind') && heads.includes('Vehicle'), heads.join(','));
  const rows = [...card.querySelectorAll('tbody tr')];
  assert.equal(rows.length, 3, 'the title counts the rows shown');
  const kindCol = heads.indexOf('Kind');
  const cells = (r) => [...r.querySelectorAll('td')].map((c) => c.textContent);
  assert.deepEqual(rows.map((r) => cells(r)[kindCol]), ['Stored for recovery', 'In the vehicle backup', 'In the vehicle backup']);
  assert.deepEqual(rows.map((r) => cells(r)[heads.indexOf('Vehicle')]), ['Buggy', 'LightOrnithopter', 'Sandbike']);
  assert.equal(cells(rows[1])[heads.indexOf('Chassis durability')], '-', 'what the save does not have is a dash');
  assert.equal(cells(rows[0])[heads.indexOf('Chassis durability')], '0.5');
  // the vehicle table says the same about the vehicle it lists
  assert.ok([...ui.doc.querySelectorAll('#main table tbody tr')].some((r) => r.textContent.includes('In the vehicle backup') && r.querySelector('[data-act="bring"]')));
});

test('the stored vehicles card says so only when no vehicle is stored, and then counts 0', async () => {
  const ui = await boot({ get: { '/api/vehicles': { vehicles: [veh(1)], hidden: 0, recovered: [], backups: [], stored: [] }, '/api/vehicles/fuel': { total: 1, notFull: 0, unknown: 0, noFuelRecord: 0 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  const card = storedCard(ui);
  assert.equal(card.querySelector('h3').textContent, 'Recovered and stored vehicles (0)');
  assert.ok(card.textContent.includes('No vehicle has been recovered or stored.'));
  assert.equal(card.querySelectorAll('tbody tr').length, 0);
});
