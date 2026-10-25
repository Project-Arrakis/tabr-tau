// Region names (#406): the Saved map lists the regions the character has discovered and says which region the character, the vehicles and the base are near; the vehicle and
// base lists put the region next to the coordinates. A name from the save is text, never markup. A map with no names (the Deep Desert) shows none of this.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const CFG = { key: 'HaggaBasin', label: 'Hagga Basin', image: '/maps/hagga-basin.png', width: 4096, height: 4096, minX: -456752, maxX: 354547, minY: -450630, maxY: 353821 };
const MAP = (extra) => ({ config: CFG, maps: [{ key: 'HaggaBasin', label: 'Hagga Basin' }], character: { name: 'Me', x: 0, y: 0, z: 0, region: 'Hagga Rift' },
  vehicles: [{ id: 1, name: 'BP_Sandbike', x: 10, y: 10, region: 'Hagga Rift' }], bases: [{ id: 5, x: 20, y: 20, region: 'Sheol' }], storage: [], resourceFields: [], markers: [], ...extra });

test('the Saved map lists the discovered regions and where the things are', async () => {
  const ui = await boot({ get: { '/api/livemap': MAP({ areas: { named: true, total: 12, discovered: ['Hagga Rift', 'Sheol'], other: 1 } }) } });
  ui.click('[data-tab="livemap"]'); await ui.settle();
  const main = ui.doc.querySelector('#main');
  const h = [...main.querySelectorAll('h3')].map((x) => x.textContent);
  assert.ok(h.includes('Regions of Hagga Basin (2 of 12 discovered)'), h.join(' | '));
  const tags = [...main.querySelectorAll('.tag')].map((x) => x.textContent.trim());
  assert.deepEqual(tags, ['Hagga Rift', 'Sheol']);
  assert.ok(main.textContent.includes('and 1 more without a name'));
  const rows = [...main.querySelectorAll('table')].find((t) => t.textContent.includes('Near the region'));
  assert.ok(rows, 'a table of where things are');
  const cells = [...rows.querySelectorAll('tbody tr')].map((r) => [...r.children].map((c) => c.textContent));
  assert.deepEqual(cells, [['Me', 'Hagga Rift'], ['BP_Sandbike', 'Hagga Rift'], ['Your base', 'Sheol']]);
  assert.ok(main.textContent.includes('nearest marker'), 'the page says it is a guess from the markers');
});

test('a map with no region names shows no regions card, and a save without the field still draws', async () => {
  const none = await boot({ get: { '/api/livemap': MAP({ areas: { named: false, total: 0, discovered: [], other: 0 }, character: { name: 'Me', x: 0, y: 0, z: 0 }, vehicles: [], bases: [] }) } });
  none.click('[data-tab="livemap"]'); await none.settle();
  assert.ok(![...none.doc.querySelectorAll('#main h3')].some((x) => x.textContent.startsWith('Regions of')));
  const old = await boot({ get: { '/api/livemap': MAP({}) } });
  old.click('[data-tab="livemap"]'); await old.settle();
  assert.ok(old.doc.querySelector('#lmc'), 'the map is drawn without the areas field');
});

test('nothing discovered says so, and a hostile region name is text', async () => {
  const hostile = '<img src=x onerror=1>';
  const ui = await boot({ get: { '/api/livemap': MAP({ character: { name: 'Me', x: 0, y: 0, z: 0, region: hostile }, areas: { named: true, total: 12, discovered: [], other: 0 } }) } });
  ui.click('[data-tab="livemap"]'); await ui.settle();
  const main = ui.doc.querySelector('#main');
  assert.ok(main.textContent.includes('No region of this map is marked as discovered'));
  assert.ok(main.textContent.includes(hostile));
  assert.equal(main.querySelector('img'), null);
});

test('the vehicle list puts the region next to the location', async () => {
  const veh = { vehicles: [{ id: 41, name: 'BP_Sandbike_CHOAM', type: 'Sandbike', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, region: 'Mysa Tarill', components: [], cargo: null },
    { id: 42, name: 'BP_Buggy_CHOAM', type: 'Buggy', map: 'DeepDesert', condition: 60, fuel: 10, x: 3, y: 4, components: [], cargo: null }], hidden: 0, recovered: [], backups: [] };
  const ui = await boot({ get: { '/api/vehicles': veh, '/api/vehicles/fuel': { total: 2, notFull: 0, unknown: 0, noFuelRecord: 0 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  const rows = [...ui.doc.querySelectorAll('#main table tbody tr')].map((r) => r.textContent);
  assert.ok(rows.some((r) => r.includes('1, 2') && r.includes('near Mysa Tarill')), rows.join(' | '));
  assert.ok(rows.some((r) => r.includes('3, 4') && !r.includes('near')), 'a vehicle with no region shows only its location');
});
