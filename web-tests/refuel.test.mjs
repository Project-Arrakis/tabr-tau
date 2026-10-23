// Refuel all my vehicles (#349): the button on Player > Vehicles shows how many vehicles are not full, posts one request, and says per kind how many were filled and
// how many were left alone; it is off when every vehicle is full, and the page still draws when the count cannot be read.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const VEH = { vehicles: [{ id: 41, name: 'BP_Sandbike_CHOAM', type: 'Sandbike', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [], cargo: null }], hidden: 0, recovered: [], backups: [] };
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);
async function openVehicles(ui) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
}

test('Refuel: the count says how many are not full, the press posts once and the toast names each kind', async () => {
  const ui = await boot({
    get: { '/api/vehicles': VEH, '/api/vehicles/fuel': { total: 5, notFull: 3, unknown: 1, noFuelRecord: 0 } },
    post: { '/api/vehicles/refuel': { ok: true, filled: 3, byClass: { BP_Sandbike_CHOAM_C: 2, BP_MediumOrnithopter_CHOAM_C: 1 }, alreadyFull: 1, total: 5, unknown: 1, skippedUnknown: ['BP_ContainerVehicle_CHOAM_C x1'], noFuelRecord: 0, outcome: 'written' } },
  });
  await openVehicles(ui);
  const btn = ui.doc.querySelector('[data-act="refuelV"]');
  assert.ok(btn, 'the Vehicles tab has the button');
  assert.equal(btn.textContent.trim(), 'Refuel all my vehicles');
  assert.equal(btn.disabled, false);
  assert.equal(ui.doc.querySelectorAll('[data-act="refuelV"]').length, 1);
  const note = ui.doc.getElementById(btn.getAttribute('aria-describedby'));
  assert.ok(note, 'the button is described by a note on the page');
  assert.ok(note.textContent.includes('3 of 5 vehicles are not full.'), note.textContent);
  assert.ok(note.textContent.includes('1 vehicle of an unknown kind will be left alone.'), note.textContent);
  assert.ok(note.textContent.includes('Sandbike 100') && note.textContent.includes('Transport Ornithopter 2500'), 'the tank sizes are on the page');
  assert.equal(posts(ui, '/api/vehicles/refuel').length, 0, 'nothing is sent before the press');

  ui.click('[data-act="refuelV"]'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/vehicles/refuel').map((p) => p.body), [{}]);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Refuelled 3 vehicles (1 x Medium Ornithopter, 2 x Sandbike); 1 already full.'), t);
  assert.ok(t.includes('1 vehicle of an unknown kind was left alone (Container Vehicle x1).'), t);
  assert.equal(ui.doc.querySelector('#toast .err'), null);
  ui.dom.window.close();
});

test('Refuel: every vehicle full turns the button off and says so; a press that fills nothing says why', async () => {
  const ui = await boot({
    get: { '/api/vehicles': VEH, '/api/vehicles/fuel': { total: 2, notFull: 0, unknown: 0, noFuelRecord: 1 } },
    post: { '/api/vehicles/refuel': { ok: true, filled: 0, byClass: {}, alreadyFull: 2, total: 3, unknown: 0, skippedUnknown: [], noFuelRecord: 1 } },
  });
  await openVehicles(ui);
  const btn = ui.doc.querySelector('[data-act="refuelV"]');
  assert.equal(btn.disabled, true);
  const note = ui.doc.getElementById(btn.getAttribute('aria-describedby')).textContent;
  assert.ok(note.includes('0 of 2 vehicles are not full.') && note.includes('1 vehicle has no fuel record and will be left alone.'), note);
  btn.disabled = false; // a stale page: the press is answered by the editor's own count
  ui.click('[data-act="refuelV"]'); await ui.settle();
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('No vehicle needed fuel (2 already full).') && t.includes('1 vehicle has no fuel record and was left alone.'), t);
  ui.dom.window.close();
});

test('Refuel: a refusal is shown as the editor words it, and the page draws when the count cannot be read', async () => {
  const ui = await boot({
    get: { '/api/vehicles': VEH, '/api/vehicles/fuel': { __fail: 'no', __status: 500 } },
    post: { '/api/vehicles/refuel': { __fail: 'Edits are held until the queue is looked at.', __code: 'queue_held', __status: 409 } },
  });
  await openVehicles(ui);
  const btn = ui.doc.querySelector('[data-act="refuelV"]');
  assert.ok(btn && !btn.disabled, 'the button stays usable without a count');
  assert.ok(ui.doc.getElementById(btn.getAttribute('aria-describedby')).textContent.includes('could not be counted'));
  ui.click('[data-act="refuelV"]'); await ui.settle();
  const err = ui.doc.querySelector('#toast .err');
  assert.ok(err && err.textContent.includes('Edits are held until the queue is looked at.'), ui.doc.querySelector('#toast').textContent);
  ui.dom.window.close();
});
