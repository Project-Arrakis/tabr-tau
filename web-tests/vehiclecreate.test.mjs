// Create a vehicle from a template (#401) on Player > Vehicles: pick a class, a tier, change parts, set the fuel, add items for the cargo hold, Preview (nothing is written) and Create
// (one request, no question asked). The result line says what was written and that the game has not loaded it yet.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const VEH = { vehicles: [{ id: 41, name: 'BP_Sandbike_CHOAM', type: 'Sandbike', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [], cargo: null }], hidden: 0, recovered: [], backups: [] };
const mod = (id, name, o = {}) => ({ id, name, grade: 6, unique: false, cargoSlots: 0, cargoVolume: 0, weapon: false, ...o });
const SLOTS = [
  { slot: 'Chassis', mandatory: true, modules: [mod('BuggyChassis_6', 'Buggy Chassis Mk6')] },
  { slot: 'Engine', mandatory: true, modules: [mod('BuggyEngine_6', 'Buggy Engine Mk6'), mod('BuggyEngine_Unique_Accelerate_06', 'Bluddshot Buggy Engine Mk6', { unique: true })] },
  { slot: 'BodyHull', mandatory: true, modules: [mod('BuggyHullBackExtra_6', 'Buggy Utility Rear Mk6')] },
  { slot: 'UtilitySlot1', mandatory: false, modules: [mod('BuggyBoost_6', 'Buggy Booster Mk6'), mod('BuggyInventory_6', 'Buggy Storage Mk6', { cargoSlots: 20, cargoVolume: 3000 }), mod('BuggyInventory_Unique_Capacity_6', 'Bigger Buggy Boot Mk6', { unique: true, cargoSlots: 35, cargoVolume: 3500 })] },
  { slot: 'UtilitySlot2', mandatory: false, modules: [mod('BuggyLauncher_6', 'Buggy Rocket Launcher Mk6'), mod('BuggyMining_Unique_YieldIncrease_06', 'Focused Buggy Cutteray Mk6', { unique: true })] },
];
const T6 = { name: 'T6_Combat', tier: 6, modules: { Chassis: 'BuggyChassis_6', Engine: 'BuggyEngine_6', BodyHull: 'BuggyHullBackExtra_6', UtilitySlot1: 'BuggyBoost_6', UtilitySlot2: 'BuggyLauncher_6' }, maxHealth: 650, fuel: 300, cargoSlots: 0, cargoVolume: 0 };
const T0 = { name: 'T0', tier: 0, modules: { Chassis: 'BuggyChassis_6', Engine: 'BuggyEngine_6', BodyHull: 'BuggyHullBackExtra_6' }, maxHealth: 650, fuel: 300, cargoSlots: 0, cargoVolume: 0 };
const BIKE = { key: 'Sandbike', label: 'Sandbike', slots: [{ slot: 'Chassis', mandatory: true, modules: [mod('SandbikeChassis_1', 'Sandbike Chassis Mk1', { grade: 1 })] }], templates: [{ name: 'T1_ExtraSeat', tier: 1, modules: { Chassis: 'SandbikeChassis_1' }, maxHealth: 400, fuel: 100, cargoSlots: 0, cargoVolume: 0 }] };
const CATALOG = { build: '1.5.15.0', classes: [BIKE, { key: 'Buggy', label: 'Buggy', slots: SLOTS, templates: [T0, T6] }] };
const ITEMS = [{ id: 'MelangeSpice', name: 'Spice Melange', category: 'resources' }, { id: 'Literjon', name: 'Literjon', category: 'tools' }];
const ANSWER = (o = {}) => ({
  ok: true, id: 1870, class: 'Buggy', label: 'Buggy', template: 'T6_Combat', tier: 6, build: '1.5.15.0', complete: true, changed: [], notes: [], dryRun: false,
  derived: { maxHealth: 650, critical: 150, fuel: 300, fuelCapacity: 300, cargoSlots: 35, cargoVolume: 3500 }, counts: { vehicle_modules: 5 },
  requested: 10, given: 10, notGiven: [], rows: { vehicle_modules: [{ slot: 'Engine', template_id: 'BuggyEngine_6', name: 'Buggy Engine Mk6', durability: 1750 }, { slot: 'Chassis', template_id: 'BuggyChassis_6', name: 'Buggy Chassis Mk6', durability: 3500 }] }, ...o,
});
const GET = { '/api/vehicles': VEH, '/api/vehicles/fuel': { total: 1, notFull: 0, unknown: 0, noFuelRecord: 0 }, '/api/vehicles/catalog': CATALOG, '/api/catalog/items': ITEMS };
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);
const set = (ui, sel, v) => { const el = ui.doc.querySelector(sel); assert.ok(el, sel); el.value = v; return el; };
const change = async (ui, sel, v) => { const el = set(ui, sel, v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle(); };
async function openVehicles(ui) { ui.click('[data-tab="player"]'); await ui.settle(); ui.click('[data-sub="player:vehicles"]'); await ui.settle(); }
async function openForm(ui) { await openVehicles(ui); ui.click('[data-act="vcOpen"]'); await ui.settle(); }

test('the Vehicles tab has a Create a vehicle button and nothing else of the form until it is pressed; the catalog is not read before', async () => {
  const ui = await boot({ get: GET });
  await openVehicles(ui);
  assert.ok(ui.doc.querySelector('[data-act="vcOpen"]'));
  assert.equal(ui.doc.querySelector('#vcClass'), null);
  assert.ok(!ui.fetched.includes('/api/vehicles/catalog'));
  ui.dom.window.close();
});

test('the form opens on the buggy at its top tier with the template\'s parts, a full tank and no cargo hold; parts are folded', async () => {
  const ui = await boot({ get: GET });
  await openForm(ui);
  assert.ok(ui.fetched.includes('/api/vehicles/catalog'));
  assert.equal(ui.doc.querySelector('#vcClass').value, 'Buggy');
  assert.equal(ui.doc.querySelector('#vcTpl').value, 'T6_Combat');
  assert.equal(ui.doc.querySelector('#vcTpl').selectedOptions[0].textContent, 'Mk6 Combat');
  assert.equal(ui.doc.querySelector('#vcFuel').value, '100');
  assert.equal(ui.doc.querySelector('details.vcparts').open, false);
  assert.equal(ui.doc.querySelector('[data-vcslot="Engine"]').value, 'BuggyEngine_6');
  assert.equal(ui.doc.querySelector('[data-vcslot="UtilitySlot1"]').value, 'BuggyBoost_6');
  assert.ok(ui.doc.querySelector('#vcnohold') && !ui.doc.querySelector('#vcItem'), 'no cargo hold, so no items to pick');
  assert.equal(ui.doc.querySelector('[data-vcslot="Chassis"] option[value=""]'), null, 'a mandatory slot cannot be emptied');
  assert.ok(ui.doc.querySelector('[data-vcslot="UtilitySlot2"] option[value=""]'), 'an optional slot can');
  assert.equal(posts(ui, '/api/vehicles/spawn').length + posts(ui, '/api/vehicles/spawn-preview').length, 0, 'opening writes nothing');
  ui.dom.window.close();
});

test('the operator\'s Mk6 buggy: uniques in three slots and items; Preview posts the request and writes nothing', async () => {
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }) } });
  await openForm(ui);
  await change(ui, '[data-vcslot="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  await change(ui, '[data-vcslot="UtilitySlot1"]', 'BuggyInventory_Unique_Capacity_6');
  await change(ui, '[data-vcslot="UtilitySlot2"]', 'BuggyMining_Unique_YieldIncrease_06');
  assert.equal(ui.doc.querySelector('[data-vcslot="Engine"]').value, 'BuggyEngine_Unique_Accelerate_06', 'the choice survives the page being drawn again');
  assert.ok(ui.doc.querySelector('#vcItem') && !ui.doc.querySelector('#vcnohold'), 'a storage part gives a cargo hold, so items can be picked');
  // a word that is not an item is refused, nothing is added
  set(ui, '#vcItem', 'Spice');
  ui.click('[data-act="vcAddItem"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('is not an item'));
  set(ui, '#vcItem', 'Spice Melange'); set(ui, '#vcQty', '10');
  ui.click('[data-act="vcAddItem"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-vcqty]').length, 1);
  set(ui, '#vcFuel', '50');
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  const p = posts(ui, '/api/vehicles/spawn-preview');
  assert.equal(p.length, 1);
  assert.deepEqual(p[0].body, {
    class: 'Buggy', template: 'T6_Combat', fuel: 0.5,
    uniques: { Engine: 'BuggyEngine_Unique_Accelerate_06', UtilitySlot1: 'BuggyInventory_Unique_Capacity_6', UtilitySlot2: 'BuggyMining_Unique_YieldIncrease_06' },
    items: [{ template_id: 'MelangeSpice', quantity: 10 }],
  });
  const box = ui.doc.querySelector('#vcpreview');
  assert.ok(box && box.textContent.includes('Would create vehicle 1870') && box.textContent.includes('Buggy, Mk6 Combat') && box.textContent.includes('a cargo hold of 35 slots and volume 3,500') && box.textContent.includes('10 of 10 items'), box && box.textContent);
  assert.ok(box.textContent.includes('Buggy Engine Mk6'), 'the parts table');
  assert.equal(posts(ui, '/api/vehicles/spawn').length, 0, 'a preview writes nothing');
  assert.ok(ui.doc.querySelector('#vcClass'), 'the form stays');
  // any change drops the preview, which no longer describes the draft
  await change(ui, '[data-vcslot="Engine"]', 'BuggyEngine_6');
  assert.equal(ui.doc.querySelector('#vcpreview'), null);
  ui.dom.window.close();
});

test('Create posts once with no question asked, the form closes, and the result line says what was written and that the game has not loaded it', async () => {
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) } });
  await openForm(ui);
  await change(ui, '[data-vcslot="UtilitySlot1"]', 'BuggyInventory_Unique_Capacity_6');
  ui.click('[data-act="vcCreate"]');
  await ui.settle();
  const p = posts(ui, '/api/vehicles/spawn');
  assert.equal(p.length, 1);
  assert.deepEqual(p[0].body, { class: 'Buggy', template: 'T6_Combat', fuel: 1, uniques: { UtilitySlot1: 'BuggyInventory_Unique_Capacity_6' } });
  assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null, 'nothing is asked first');
  assert.equal(ui.doc.querySelector('#vcClass'), null, 'the form closed');
  const r = ui.doc.querySelector('#vcresult');
  assert.ok(r, 'a result line');
  const text = r.textContent.replace(/\s+/g, ' ');
  assert.ok(text.includes('Created vehicle 1870:') && text.includes('Buggy, Mk6 Combat, 5 parts, health 650, fuel 300 of 300') && text.includes('the game has not loaded it yet'), text);
  assert.ok(!text.includes('queued'), text);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Created vehicle 1870'));
  ui.click('[data-act="vcDismiss"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#vcresult'), null);
  ui.dom.window.close();
});

test('while the game runs the edit is queued and the result line says so', async () => {
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn': ANSWER({ outcome: 'queued' }) } });
  await openForm(ui);
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  const text = ui.doc.querySelector('#vcresult').textContent.replace(/\s+/g, ' ');
  assert.ok(text.includes('queued'), text);
  ui.dom.window.close();
});

test('a class or a tier change resets the parts to the template\'s; an emptied optional slot is sent as empty; items can be removed', async () => {
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }) } });
  await openForm(ui);
  await change(ui, '[data-vcslot="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  await change(ui, '#vcTpl', 'T0');
  assert.equal(ui.doc.querySelector('[data-vcslot="Engine"]').value, 'BuggyEngine_6', 'a new tier starts from its template');
  assert.equal(ui.doc.querySelector('[data-vcslot="UtilitySlot1"]').value, '', 'T0 has no utility part');
  await change(ui, '#vcClass', 'Sandbike');
  assert.equal(ui.doc.querySelector('#vcTpl').value, 'T1_ExtraSeat');
  await change(ui, '#vcClass', 'Buggy');
  assert.equal(ui.doc.querySelector('#vcTpl').value, 'T6_Combat', 'back to the class\'s top tier');
  await change(ui, '[data-vcslot="UtilitySlot2"]', '');
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/vehicles/spawn-preview')[0].body.uniques, { UtilitySlot2: '' });
  // items
  await change(ui, '[data-vcslot="UtilitySlot1"]', 'BuggyInventory_6');
  set(ui, '#vcItem', 'Literjon'); ui.click('[data-act="vcAddItem"]'); await ui.settle();
  set(ui, '#vcItem', 'Spice Melange'); set(ui, '#vcQty', '5'); ui.click('[data-act="vcAddItem"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-vcqty]').length, 2);
  set(ui, '[data-vcqty="1"]', '7');
  ui.click('[data-act="vcDelItem"][data-id="0"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-vcqty]').length, 1);
  assert.equal(ui.doc.querySelector('[data-vcqty="0"]').value, '7', 'a quantity typed before the removal is kept');
  ui.dom.window.close();
});

test('a refused request is said in the toast, nothing is created and the draft is kept', async () => {
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn': { __fail: 'the Engine slot needs a module', __code: 'bad_argument' }, '/api/vehicles/spawn-preview': { __fail: 'no preview now', __code: 'bad_argument' } } });
  await openForm(ui);
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('no preview now'));
  assert.equal(ui.doc.querySelector('#vcpreview'), null);
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('the Engine slot needs a module'));
  assert.ok(ui.doc.querySelector('#vcClass'), 'the form is still there');
  assert.equal(ui.doc.querySelector('#vcresult'), null);
  ui.click('[data-act="vcCancel"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#vcClass'), null);
  ui.dom.window.close();
});

test('the catalog the real handler answers draws the form for every class, and the Mk6 buggy parts are there', async () => {
  const ui = await boot({ get: { '/api/vehicles': VEH, '/api/vehicles/fuel': { total: 1, notFull: 0, unknown: 0, noFuelRecord: 0 } } });
  await openForm(ui);
  const classes = [...ui.doc.querySelectorAll('#vcClass option')].map((o) => o.value);
  assert.deepEqual(classes, ['Sandbike', 'Buggy', 'OrnithopterLight', 'OrnithopterMedium', 'OrnithopterTransport', 'Sandcrawler', 'ContainerVehicle']);
  await change(ui, '#vcClass', 'Buggy');
  assert.equal(ui.doc.querySelector('#vcTpl').selectedOptions[0].textContent, 'Mk6 Combat');
  const opts = [...ui.doc.querySelectorAll('[data-vcslot="UtilitySlot1"] option')].map((o) => o.value);
  assert.ok(opts.includes('BuggyInventory_Unique_Capacity_6'), opts.join());
  for (const c of classes) { await change(ui, '#vcClass', c); assert.ok(ui.doc.querySelectorAll('[data-vcslot]').length >= 1, c); }
  ui.dom.window.close();
});

test('the explanation of Create a vehicle is in About this, not on the page; the tooltips of Preview and Create carry theirs', async () => {
  const { visibleText, movedText } = await import('./uitext.mjs');
  const ui = await boot({ get: GET });
  await openForm(ui);
  const main = ui.doc.querySelector('#main');
  const shown = visibleText(main), moved = movedText(main);
  assert.ok(!shown.includes('has not been loaded in the game yet'), 'still shown');
  assert.ok(moved.includes('has not been loaded in the game yet') && moved.includes('Mk6 is the tier 6 template'), 'lost, not moved');
  assert.ok(moved.includes('Nothing asks first') && moved.includes('Nothing is changed.'));
  assert.ok(shown.includes('Create a vehicle') && shown.includes('Change parts'));
  ui.dom.window.close();
});
