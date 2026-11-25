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
  const rows = [...ui.doc.querySelectorAll('#main tr.vrow')];
  const stored = rows.find((r) => r.textContent.includes('In the vehicle backup'));
  assert.ok(stored, 'the stored vehicle carries its label');
  assert.equal(stored.querySelector('.tag').textContent, 'Stored');
  assert.equal(stored.querySelector('[data-act="bring"]').disabled, true);
  assert.equal(stored.querySelector('[data-act="dupV"]').disabled, true);
  assert.equal(stored.querySelector('[data-act="repairOne"]').disabled, false, 'Repair stays on, as before');
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

// #1230: a stored vehicle that is in the list is shown there, with a Stored tag and the server's label (one function, #528). A vehicle waiting to be recovered, which is not in
// the list, is shown in a section of its own, drawn only when there is one.
const sv = (id, kind, extra) => ({ vehicle_id: id, kind, label: kind === 'backup' ? 'In the vehicle backup' : 'Stored for recovery', type: 'Buggy', ...extra });
const card = (ui, title) => [...ui.doc.querySelectorAll('#main .card')].find((c) => c.querySelector('h3') && c.querySelector('h3').textContent.startsWith(title));
const openVehicles = async (vehicles, stored) => {
  const ui = await boot({ get: { '/api/vehicles': { vehicles, hidden: 0, recovered: [], backups: [], stored }, '/api/vehicles/fuel': { total: 1, notFull: 0, unknown: 0, noFuelRecord: 0 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  return ui;
};

test('a vehicle waiting for recovery has its own section; the backed-up and the state-only vehicle are in the list only', async () => {
  const stored = [sv(301, 'recovery', { vehicle_name: 'BP_Buggy_CHOAM', chassis_durability: 0.5, time_stored: 1700000000, reason: 1 }),
    sv(302, 'backup', { type: 'LightOrnithopter' }), sv(303, 'recovery', { type: 'Sandbike' })];
  const ui = await openVehicles([veh(302, { type: 'LightOrnithopter', stored: 'backup', storedLabel: 'In the vehicle backup' }),
    veh(303, { stored: 'recovery', storedLabel: 'Stored for recovery' })], stored);
  assert.equal(card(ui, 'Recovered and stored vehicles'), undefined, 'the old card is gone');
  const list = card(ui, 'Your vehicles');
  assert.deepEqual([...list.querySelectorAll('[id^="vstored"]')].map((n) => n.textContent),
    ['In the vehicle backup. Take it out of storage in the game first.', 'Stored for recovery. Take it out of storage in the game first.']);
  const waiting = card(ui, 'Waiting for recovery');
  assert.equal(waiting.querySelector('h3').textContent, 'Waiting for recovery');
  assert.equal(list.nextElementSibling, waiting, 'directly under the list');
  assert.deepEqual([...waiting.querySelectorAll('th')].map((h) => h.textContent), ['Vehicle', 'Chassis durability', 'Stored at'], 'no Name and no Reason code');
  const rows = [...waiting.querySelectorAll('tbody tr')];
  assert.equal(rows.length, 1, 'a vehicle that is in the list is not repeated');
  const cells = [...rows[0].querySelectorAll('td')].map((c) => c.textContent);
  assert.deepEqual(cells.slice(0, 2), ['Buggy', '0.5']);
  assert.ok(cells[2].includes('2023'), cells[2]);
  assert.ok(!waiting.textContent.includes('BP_Buggy_CHOAM'));
  assert.equal(waiting.querySelector('button'), null, 'nothing to press');
});

test('the waiting section is not drawn, and says nothing, when no vehicle waits', async () => {
  for (const stored of [[], undefined, [sv(1, 'backup')], [sv(1, 'recovery')]]) {
    const ui = await openVehicles([veh(1)], stored);
    assert.equal(card(ui, 'Waiting for recovery'), undefined, JSON.stringify(stored));
    assert.ok(!ui.doc.querySelector('#main').textContent.includes('No vehicle has been recovered or stored.'));
    ui.dom.window.close();
  }
});
