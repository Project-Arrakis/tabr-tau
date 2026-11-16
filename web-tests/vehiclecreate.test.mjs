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
const noId = (x) => { const { requestId, ...b } = x.body; return b; }; // the request without the id Create gives each form (#740)
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
  assert.deepEqual(noId(p[0]), { class: 'Buggy', template: 'T6_Combat', fuel: 1, uniques: { UtilitySlot1: 'BuggyInventory_Unique_Capacity_6' } });
  assert.match(p[0].body.requestId, /^[A-Za-z0-9_-]{1,64}$/, 'Create gives the form an id the editor can use to know a repeat');
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

test('the preview, the result line and the toast say where the vehicle goes and what it moved away from (#480)', async () => {
  const place = { map: 'HaggaBasin', x: 1, y: 2, z: 3, words: 'outside your base, 14 m south of you, outside the land claim (base pieces at the first spot)', moved: true, inBase: true, blocked: ['pieces'], claim: 'outside', distance: 1400, nextToCharacter: true };
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true, place }), '/api/vehicles/spawn': ANSWER({ outcome: 'written', place }) } });
  await openForm(ui);
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  const pv = ui.doc.querySelector('#vcpreview').textContent.replace(/\s+/g, ' ');
  assert.ok(pv.includes('It goes outside your base, 14 m south of you, outside the land claim (base pieces at the first spot).'), pv);
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  const text = ui.doc.querySelector('#vcresult').textContent.replace(/\s+/g, ' ');
  assert.ok(text.includes('It goes outside your base, 14 m south of you'), text);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Buggy outside your base, 14 m south of you'), ui.doc.querySelector('#toast').textContent);
  ui.dom.window.close();
});

test('an aircraft in the air says how far ahead and how high, and that the game settles it (#540)', async () => {
  const place = { map: 'HaggaBasin', x: 1, y: 2, z: 3, words: '30 m south of you and 7 m above you; it starts in the air and the game settles it', moved: false, inBase: false, blocked: [], claim: '', distance: 3000, up: 700, inAir: true, nextToCharacter: true };
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true, place }), '/api/vehicles/spawn': ANSWER({ outcome: 'written', place }) } });
  await openForm(ui);
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#vcpreview').textContent.replace(/\s+/g, ' ').includes('It goes 30 m south of you and 7 m above you; it starts in the air and the game settles it.'));
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#vcresult').textContent.replace(/\s+/g, ' ').includes('It goes 30 m south of you and 7 m above you; it starts in the air and the game settles it.'));
  ui.dom.window.close();
});

test('an answer with no place still reads "next to you"', async () => {
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }) } });
  await openForm(ui);
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#vcpreview').textContent.replace(/\s+/g, ' ').includes('It goes next to you.'));
  ui.dom.window.close();
});

// jsdom has no layout, so the offset is a number the test holds. A change inside the page is not a move to another page (#650): the scroll offset stays and the box that was changed keeps the focus.
function holdWindowScroll(w, y) {
  let at = y;
  Object.defineProperty(w, 'scrollY', { get: () => at, configurable: true });
  w.scrollTo = (x, yy) => { at = yy; };
  // a browser that draws the page again from nothing starts it at the top, unless the page puts the offset back
  new w.MutationObserver(() => { at = 0; }).observe(w.document.querySelector('#main'), { childList: true });
  return () => at;
}
const pick = async (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.focus(); await change(ui, sel, v); };

test('changing a part, the tier or the class keeps the page offset and the focus on the box that was changed', async () => {
  const ui = await boot({ get: GET });
  await openForm(ui);
  ui.doc.querySelector('details.vcparts').open = true;
  const y = holdWindowScroll(ui.w, 1545);
  await pick(ui, '[data-vcslot="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  assert.equal(y(), 1545, 'a part swap keeps the offset');
  assert.equal(ui.doc.activeElement, ui.doc.querySelector('[data-vcslot="Engine"]'), 'the focus is on the part box');
  assert.equal(ui.doc.querySelector('[data-vcslot="Engine"]').value, 'BuggyEngine_Unique_Accelerate_06');
  assert.equal(ui.doc.querySelector('details.vcparts').open, true, 'the parts stay unfolded');
  await pick(ui, '#vcTpl', 'T0');
  assert.equal(y(), 1545, 'a tier change keeps the offset');
  assert.equal(ui.doc.activeElement, ui.doc.querySelector('#vcTpl'));
  await pick(ui, '#vcClass', 'Sandbike');
  assert.equal(y(), 1545, 'a class change keeps the offset');
  assert.equal(ui.doc.activeElement, ui.doc.querySelector('#vcClass'));
  assert.equal(ui.doc.querySelector('#vcClass').value, 'Sandbike');
  ui.dom.window.close();
});

test('adding an item, previewing and opening another vehicle keep the page offset; the item box is empty after the add', async () => {
  const two = { ...VEH, vehicles: [...VEH.vehicles, { ...VEH.vehicles[0], id: 42 }] };
  const ui = await boot({ get: { ...GET, '/api/vehicles': two }, post: { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }) } });
  await openForm(ui);
  await change(ui, '[data-vcslot="UtilitySlot1"]', 'BuggyInventory_6');
  const y = holdWindowScroll(ui.w, 900);
  set(ui, '#vcItem', 'Spice Melange');
  ui.click('[data-act="vcAddItem"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-vcqty]').length, 1);
  assert.equal(ui.doc.querySelector('#vcItem').value, '', 'the item box is empty again');
  assert.equal(y(), 900);
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#vcpreview'));
  assert.equal(y(), 900);
  ui.click('[data-vhopen="42"]'); await ui.settle();
  assert.equal(y(), 900, 'opening another vehicle keeps the offset');
  assert.ok(ui.doc.querySelector('[data-vcqty]'), 'the draft is kept');
  ui.dom.window.close();
});

// ---------- Replace parts in the preview table (#672)
const SWAPPED = (o = {}) => ANSWER({
  dryRun: true, derived: { maxHealth: 700, critical: 160, fuel: 300, fuelCapacity: 300, cargoSlots: 20, cargoVolume: 3000 }, requested: 0, given: 0,
  rows: { vehicle_modules: [{ slot: 'Engine', template_id: 'BuggyEngine_Unique_Accelerate_06', name: 'Bluddshot Buggy Engine Mk6', durability: 2100 }, { slot: 'Chassis', template_id: 'BuggyChassis_6', name: 'Buggy Chassis Mk6', durability: 3500 }, { slot: 'UtilitySlot1', template_id: 'BuggyInventory_6', name: 'Buggy Storage Mk6', durability: 900 }] }, ...o,
});
const optionIds = (ui, sel) => [...ui.doc.querySelectorAll(sel + ' option')].map((o) => o.value);
async function openPreview(ui) { await openForm(ui); ui.click('[data-act="vcPreview"]'); await ui.settle(); assert.ok(ui.doc.querySelector('#vcpreview')); }
// the page waits 600 ms after the last pick before it asks, so the keys of a list can be pressed more than once
const WAIT = 750;
const fire = (ui, sel, v) => { const el = ui.doc.querySelector(sel); assert.ok(el, sel); el.focus(); el.value = v; el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const pickIn = async (ui, sel, v) => { fire(ui, sel, v); await ui.wait(WAIT); await ui.settle(); };
// holds the answers to the preview until release() is called; `once` holds only the first
function gatePreview(ui, once = false) {
  const real = ui.w.fetch;
  let release, held = 0;
  const gate = new Promise((r) => { release = r; });
  ui.w.fetch = (p, o) => (String(p).includes('spawn-preview') && !(once && held++) ? gate.then(() => real(p, o)) : real(p, o));
  return release;
}
// what the table shows for each slot, and what Create would write
const tableParts = (ui) => Object.fromEntries([...ui.doc.querySelectorAll('#vcpreview [data-vcprev]')].map((e) => [e.dataset.vcprev, e.value]));

test('each row of the preview table has a part box with only the parts that fit its slot, labelled as in Change parts; an empty optional slot gets a row too, in the class order', async () => {
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }) } });
  await openPreview(ui);
  const engine = ui.doc.querySelector('#vcpreview select[data-vcprev="Engine"]');
  assert.ok(engine, 'the Engine row has a part box');
  assert.equal(engine.getAttribute('aria-label'), 'Part for Engine');
  assert.equal(engine.getAttribute('title'), null, 'no game id as a tooltip');
  assert.equal(engine.value, 'BuggyEngine_6');
  assert.deepEqual(optionIds(ui, '#vcpreview [data-vcprev="Engine"]'), ['BuggyEngine_6', 'BuggyEngine_Unique_Accelerate_06'], 'only the engines of the buggy');
  assert.equal(engine.selectedOptions[0].textContent, 'Buggy Engine Mk6');
  assert.ok([...engine.options].some((o) => o.textContent === 'Bluddshot Buggy Engine Mk6 (unique)'));
  assert.equal(ui.doc.querySelector('#vcpreview [data-vcprev="Chassis"] option[value=""]'), null, 'a mandatory slot cannot be emptied');
  const util = ui.doc.querySelector('#vcpreview [data-vcprev="UtilitySlot2"]');
  assert.ok(util, 'an optional slot with no part in the answer still has a row');
  assert.ok(util.querySelector('option[value=""]'));
  assert.deepEqual(optionIds(ui, '#vcpreview [data-vcprev="UtilitySlot2"]'), ['', 'BuggyLauncher_6', 'BuggyMining_Unique_YieldIncrease_06']);
  assert.ok([...ui.doc.querySelector('#vcpreview [data-vcprev="UtilitySlot1"]').options].some((o) => o.textContent === 'Buggy Storage Mk6 (20 slots)'), 'a part that gives a cargo hold is labelled');
  assert.deepEqual([...ui.doc.querySelectorAll('#vcpreview [data-vcprev]')].map((e) => e.dataset.vcprev), ['Chassis', 'Engine', 'UtilitySlot1', 'UtilitySlot2'], 'the rows follow the class\'s slot order');
  assert.ok(ui.doc.querySelector('#vcpreview').textContent.includes('Pick a part in the Part column to swap it'), 'the table says it can be changed');
  assert.equal(ui.doc.querySelector('#vcwait').hidden, true);
  assert.equal(posts(ui, '/api/vehicles/spawn').length, 0);
  ui.dom.window.close();
});

test('picking a part in the preview asks for the preview again, redraws the sentence and the table in place, says the change in a toast, and keeps the offset and the focus', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }) };
  const ui = await boot({ get: GET, post });
  await openPreview(ui);
  post['/api/vehicles/spawn-preview'] = SWAPPED();
  const y = holdWindowScroll(ui.w, 1300);
  fire(ui, '#vcpreview [data-vcprev="UtilitySlot1"]', 'BuggyInventory_6');
  await ui.wait(50);
  assert.equal(ui.doc.querySelector('#vcwait').hidden, false, 'a line says why Create waits');
  assert.equal(ui.doc.querySelector('[data-act="vcCreate"]').disabled, true);
  assert.ok(ui.doc.querySelector('[data-act="vcCreate"]').getAttribute('aria-describedby').includes('vcwait'));
  assert.equal(ui.doc.querySelector('[data-vcslot="UtilitySlot1"]').value, 'BuggyInventory_6', 'Change parts shows the pick at once');
  assert.equal(posts(ui, '/api/vehicles/spawn-preview').length, 1, 'nothing is asked before the wait is over');
  await ui.wait(WAIT); await ui.settle();
  const asks = posts(ui, '/api/vehicles/spawn-preview');
  assert.equal(asks.length, 2, 'the preview was asked for again');
  assert.deepEqual(asks[1].body, { class: 'Buggy', template: 'T6_Combat', fuel: 1, uniques: { UtilitySlot1: 'BuggyInventory_6' } });
  const text = ui.doc.querySelector('#vcpreview').textContent.replace(/\s+/g, ' ');
  assert.ok(text.includes('health 700') && text.includes('a cargo hold of 20 slots and volume 3,000'), text);
  assert.ok(ui.doc.querySelector('#vcpreview').textContent.includes('2,100'), 'the durability column is the new answer');
  assert.equal(ui.doc.querySelector('[data-vcprev="UtilitySlot1"]').value, 'BuggyInventory_6');
  assert.equal(ui.doc.activeElement, ui.doc.querySelector('[data-vcprev="UtilitySlot1"]'), 'the focus stays on the box that was changed');
  assert.equal(y(), 1300, 'the offset stays');
  assert.equal(ui.doc.querySelector('[data-act="vcCreate"]').disabled, false);
  assert.equal(ui.doc.querySelector('#vcwait').hidden, true);
  assert.ok(ui.doc.querySelector('#vcwait').closest('[role="status"]'), 'the line is in the live region');
  const toastText = ui.doc.querySelector('#toast').textContent;
  assert.ok(toastText.includes('Utility 1 is now Buggy Storage Mk6 (20 slots)') && toastText.includes('cargo hold of 20 slots'), toastText);
  assert.ok(ui.doc.querySelector('#vcItem') && !ui.doc.querySelector('#vcnohold'), 'the cargo hold of the new part lets items be picked');
  assert.equal(ui.doc.querySelector('#vcpartnote'), null);
  assert.equal(posts(ui, '/api/vehicles/spawn').length, 0, 'nothing is written by a pick');
  ui.dom.window.close();
});

test('the arrow keys on a closed list change the value at every press: one request is asked, for the last value, and no error is shown', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }) };
  const ui = await boot({ get: GET, post });
  await openPreview(ui);
  post['/api/vehicles/spawn-preview'] = SWAPPED();
  fire(ui, '#vcpreview [data-vcprev="UtilitySlot1"]', 'BuggyInventory_6');
  await ui.wait(100);
  fire(ui, '#vcpreview [data-vcprev="UtilitySlot1"]', 'BuggyInventory_Unique_Capacity_6');
  await ui.wait(100);
  fire(ui, '#vcpreview [data-vcprev="UtilitySlot1"]', 'BuggyBoost_6');
  fire(ui, '#vcpreview [data-vcprev="UtilitySlot1"]', 'BuggyInventory_Unique_Capacity_6');
  assert.equal(posts(ui, '/api/vehicles/spawn-preview').length, 1, 'no request for the stops on the way');
  await ui.wait(WAIT); await ui.settle();
  const asks = posts(ui, '/api/vehicles/spawn-preview');
  assert.equal(asks.length, 2);
  assert.deepEqual(asks[1].body.uniques, { UtilitySlot1: 'BuggyInventory_Unique_Capacity_6' });
  assert.equal(ui.doc.querySelector('.d-toast.bad, .d-toast.err'), null, 'no error toast');
  assert.equal(ui.doc.querySelector('[data-vcprev="UtilitySlot1"]').value, 'BuggyInventory_Unique_Capacity_6');
  ui.dom.window.close();
});

test('two parts picked within the wait are asked together; a pick made while an answer is on its way is asked after it and the late answer is not shown', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }) };
  const ui = await boot({ get: GET, post });
  await openPreview(ui);
  post['/api/vehicles/spawn-preview'] = SWAPPED();
  fire(ui, '#vcpreview [data-vcprev="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  fire(ui, '#vcpreview [data-vcprev="UtilitySlot2"]', '');
  await ui.wait(WAIT); await ui.settle();
  assert.deepEqual(posts(ui, '/api/vehicles/spawn-preview')[1].body.uniques, { Engine: 'BuggyEngine_Unique_Accelerate_06', UtilitySlot2: '' });
  // a request is on its way (held) and another pick is made
  const release = gatePreview(ui, true);
  fire(ui, '#vcpreview [data-vcprev="UtilitySlot1"]', 'BuggyInventory_6');
  await ui.wait(WAIT); // the request is out and held: the page records it when it is let go
  assert.equal(posts(ui, '/api/vehicles/spawn-preview').length, 2);
  fire(ui, '#vcpreview [data-vcprev="UtilitySlot2"]', 'BuggyMining_Unique_YieldIncrease_06');
  await ui.wait(WAIT); await ui.settle();
  const asks = posts(ui, '/api/vehicles/spawn-preview');
  assert.equal(asks.length, 3, 'the second pick was asked without waiting for the first');
  release(); await ui.settle();
  assert.equal(posts(ui, '/api/vehicles/spawn-preview').length, 4);
  assert.equal(ui.doc.querySelector('[data-act="vcCreate"]').disabled, false);
  assert.equal(ui.doc.querySelector('#vcwait').hidden, true);
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  const made = posts(ui, '/api/vehicles/spawn');
  assert.equal(made.length, 1);
  assert.deepEqual(noId(made[0]), asks[2].body, 'Create writes the request of the answer that was used, not of the late one');
  ui.dom.window.close();
});

test('a part picked in Change parts shows in the preview table after the next Preview, and the two boxes of a slot always agree', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }) };
  const ui = await boot({ get: GET, post });
  await openPreview(ui);
  await change(ui, '[data-vcslot="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  assert.equal(ui.doc.querySelector('#vcpreview'), null, 'a change in Change parts still drops the preview');
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('[data-vcprev="Engine"]').value, 'BuggyEngine_Unique_Accelerate_06');
  ui.dom.window.close();
});

test('a part the server refuses is put back, the old preview stays and the reason is said with the box; nothing is written', async () => {
  const withStorage = ANSWER({ dryRun: true, rows: { vehicle_modules: [{ slot: 'Engine', template_id: 'BuggyEngine_6', name: 'Buggy Engine Mk6', durability: 1750 }, { slot: 'UtilitySlot1', template_id: 'BuggyInventory_6', name: 'Buggy Storage Mk6', durability: 900 }] } });
  const post = { '/api/vehicles/spawn-preview': withStorage };
  const ui = await boot({ get: GET, post });
  await openForm(ui);
  await change(ui, '[data-vcslot="UtilitySlot1"]', 'BuggyInventory_6');
  set(ui, '#vcItem', 'Spice Melange'); set(ui, '#vcQty', '10');
  ui.click('[data-act="vcAddItem"]'); await ui.settle();
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  const before = ui.doc.querySelector('#vcpreview').textContent;
  post['/api/vehicles/spawn-preview'] = { __fail: 'this buggy has no cargo hold, so items cannot be put in it. Give it a storage part, or remove the items', __code: 'bad_argument' };
  await pickIn(ui, '#vcpreview [data-vcprev="UtilitySlot1"]', 'BuggyBoost_6');
  const note = ui.doc.querySelector('#vcpartnote');
  assert.ok(note && note.getAttribute('role') === 'alert', 'the reason is an alert');
  assert.equal(note.textContent, 'Utility 1 was not changed. this buggy has no cargo hold, so items cannot be put in it. Give it a storage part, or remove the items');
  const box = ui.doc.querySelector('[data-vcprev="UtilitySlot1"]');
  assert.equal(box.value, 'BuggyInventory_6', 'the old part is back');
  assert.equal(box.getAttribute('aria-invalid'), 'true');
  assert.equal(box.getAttribute('aria-describedby'), 'vcpartnote');
  assert.equal(ui.doc.querySelector('[data-vcslot="UtilitySlot1"]').value, 'BuggyInventory_6');
  assert.equal(ui.doc.activeElement, box, 'the focus stays');
  assert.ok(ui.doc.querySelector('#vcpreview').textContent.startsWith(before.slice(0, 40)), 'the old preview is still there');
  assert.equal(ui.doc.querySelector('[data-act="vcCreate"]').disabled, false);
  assert.equal(posts(ui, '/api/vehicles/spawn').length, 0);
  // the next pick clears the note
  post['/api/vehicles/spawn-preview'] = SWAPPED();
  await pickIn(ui, '#vcpreview [data-vcprev="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  assert.equal(ui.doc.querySelector('#vcpartnote'), null);
  ui.dom.window.close();
});

test('Create writes exactly the parts the preview table shows, and the same request the last preview was asked with', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
  const ui = await boot({ get: GET, post });
  await openPreview(ui);
  post['/api/vehicles/spawn-preview'] = SWAPPED();
  await pickIn(ui, '#vcpreview [data-vcprev="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  await pickIn(ui, '#vcpreview [data-vcprev="UtilitySlot1"]', 'BuggyInventory_6');
  const shown = tableParts(ui);
  const asks = posts(ui, '/api/vehicles/spawn-preview');
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  const made = posts(ui, '/api/vehicles/spawn');
  assert.equal(made.length, 1);
  assert.deepEqual(noId(made[0]), asks[asks.length - 1].body, 'the request of Create is the request of the last preview');
  assert.deepEqual(made[0].body.uniques, { Engine: 'BuggyEngine_Unique_Accelerate_06', UtilitySlot1: 'BuggyInventory_6' });
  for (const [slot, id] of Object.entries(made[0].body.uniques)) assert.equal(shown[slot], id, slot + ' is what the table shows');
  assert.ok(ui.doc.querySelector('#vcresult'));
  ui.dom.window.close();
});

test('Create does not go while a pick is waiting or its preview is on its way, and says why in a note, not an error', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
  const ui = await boot({ get: GET, post });
  await openPreview(ui);
  post['/api/vehicles/spawn-preview'] = SWAPPED();
  fire(ui, '#vcpreview [data-vcprev="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  await ui.wait(20);
  ui.doc.querySelector('[data-act="vcCreate"]').disabled = false; // even a Create that gets through the disabled button does nothing
  ui.click('[data-act="vcCreate"]'); await ui.wait(20);
  assert.equal(posts(ui, '/api/vehicles/spawn').length, 0, 'Create waits');
  const t = [...ui.doc.querySelectorAll('#toast .d-toast')].pop();
  assert.ok(t && t.textContent.includes('The preview is updating') && t.classList.contains('note') && !t.classList.contains('err'), t && t.className);
  await ui.wait(WAIT); await ui.settle();
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/vehicles/spawn')[0].body.uniques, { Engine: 'BuggyEngine_Unique_Accelerate_06' });
  ui.dom.window.close();
});

// A late answer must never leave Create writing something other than the table shows (#672). Each case changes the form while a pick's request is held, releases the answer and then checks the one rule:
// either there is no table (and so nothing to disagree with) or Create's request is the request the table was asked with.
async function lateAnswer(ui, post, between) {
  await openForm(ui);
  await change(ui, '[data-vcslot="UtilitySlot1"]', 'BuggyInventory_6');
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  post['/api/vehicles/spawn-preview'] = SWAPPED();
  const release = gatePreview(ui);
  fire(ui, '#vcpreview [data-vcprev="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  await ui.wait(WAIT); // the request is out and held
  await between(ui);
  release(); await ui.settle();
}
const noStaleTable = (ui) => {
  const table = ui.doc.querySelector('#vcpreview');
  const asks = posts(ui, '/api/vehicles/spawn-preview');
  if (table) {
    ui.click('[data-act="vcCreate"]');
    return ui.settle().then(() => {
      const made = posts(ui, '/api/vehicles/spawn');
      if (made.length) assert.deepEqual(noId(made[0]), asks[asks.length - 1].body, 'Create writes the request the table was asked with');
    });
  }
  return Promise.resolve();
};

test('late answer: an item added while the pick is on its way drops the table; the answer does not bring it back', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
  const ui = await boot({ get: GET, post });
  await lateAnswer(ui, post, async (u) => { set(u, '#vcItem', 'Spice Melange'); set(u, '#vcQty', '5'); u.click('[data-act="vcAddItem"]'); await u.settle(); });
  assert.equal(ui.doc.querySelector('#vcpreview'), null, 'no table for a draft it was not asked for');
  assert.equal(ui.doc.querySelector('[data-vcslot="Engine"]').value, 'BuggyEngine_Unique_Accelerate_06', 'the pick is kept in the draft');
  await noStaleTable(ui);
  ui.dom.window.close();
});

test('late answer: a part changed in Change parts, an item removed, or a tier or class changed while the pick is on its way drops the table', async () => {
  for (const between of [
    async (u) => { await change(u, '[data-vcslot="UtilitySlot2"]', ''); },
    async (u) => { await change(u, '#vcTpl', 'T0'); },
    async (u) => { await change(u, '#vcClass', 'Sandbike'); },
  ]) {
    const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
    const ui = await boot({ get: GET, post });
    await lateAnswer(ui, post, between);
    assert.equal(ui.doc.querySelector('#vcpreview'), null);
    assert.equal(ui.doc.querySelector('[data-act="vcCreate"]').disabled, false, 'Create is not left disabled');
    assert.equal(ui.doc.querySelector('#vcpartnote'), null);
    await noStaleTable(ui);
    ui.dom.window.close();
  }
});

test('late answer: a class changed while a refused pick is on its way does not put the old class\'s part into the new class', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
  const ui = await boot({ get: GET, post });
  await openForm(ui);
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  post['/api/vehicles/spawn-preview'] = { __fail: 'no', __code: 'bad_argument' };
  const release = gatePreview(ui);
  fire(ui, '#vcpreview [data-vcprev="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  await ui.wait(WAIT);
  await change(ui, '#vcClass', 'Sandbike');
  release(); await ui.settle();
  assert.equal(ui.doc.querySelector('#vcpartnote'), null);
  assert.equal(ui.doc.querySelector('[data-vcslot="Chassis"]').value, 'SandbikeChassis_1');
  assert.equal(ui.doc.querySelector('#vcpreview'), null);
  ui.dom.window.close();
});

test('late answer: Preview pressed while a pick is on its way shows the answer of the Preview, not the late one', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
  const ui = await boot({ get: GET, post });
  await lateAnswer(ui, post, async (u) => { post['/api/vehicles/spawn-preview'] = SWAPPED({ id: 1999 }); u.click('[data-act="vcPreview"]'); await u.settle(); });
  const t = ui.doc.querySelector('#vcpreview');
  assert.ok(t && t.textContent.includes('Would create vehicle 1999'), t && t.textContent);
  assert.equal(ui.doc.querySelector('[data-act="vcCreate"]').disabled, false);
  await noStaleTable(ui);
  ui.dom.window.close();
});

test('Cancel and Open while a pick is on its way: the new form is not left with Create disabled and shows no table', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
  const ui = await boot({ get: GET, post });
  await lateAnswer(ui, post, async (u) => { u.click('[data-act="vcCancel"]'); await u.settle(); u.click('[data-act="vcOpen"]'); await u.settle(); });
  assert.equal(ui.doc.querySelector('#vcpreview'), null);
  assert.equal(ui.doc.querySelector('[data-vcslot="Engine"]').value, 'BuggyEngine_6');
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('[data-act="vcCreate"]').disabled, false);
  ui.dom.window.close();
});

test('Create refuses, in a note, when the fuel or a quantity was edited after the preview, drops the table and writes nothing; after Preview again it writes the new figure', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
  const ui = await boot({ get: GET, post });
  await openPreview(ui);
  set(ui, '#vcFuel', '40');
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  assert.equal(posts(ui, '/api/vehicles/spawn').length, 0, 'the sentence said a full tank');
  const t = [...ui.doc.querySelectorAll('#toast .d-toast')].pop();
  assert.ok(t && t.textContent.includes('The form changed after the preview') && t.classList.contains('note'), t && t.className);
  assert.equal(ui.doc.querySelector('#vcpreview'), null);
  assert.equal(ui.doc.querySelector('#vcFuel').value, '40', 'the edit is kept');
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  assert.equal(posts(ui, '/api/vehicles/spawn-preview').pop().body.fuel, 0.4);
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  assert.equal(posts(ui, '/api/vehicles/spawn')[0].body.fuel, 0.4);
  ui.dom.window.close();
});

test('Cancel discards the parts picked in the preview: the next form starts from the template again', async () => {
  const post = { '/api/vehicles/spawn-preview': ANSWER({ dryRun: true }) };
  const ui = await boot({ get: GET, post });
  await openPreview(ui);
  post['/api/vehicles/spawn-preview'] = SWAPPED();
  await pickIn(ui, '#vcpreview [data-vcprev="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  ui.click('[data-act="vcCancel"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#vcpreview'), null);
  assert.equal(posts(ui, '/api/vehicles/spawn').length, 0);
  ui.click('[data-act="vcOpen"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('[data-vcslot="Engine"]').value, 'BuggyEngine_6');
  assert.equal(ui.doc.querySelector('#vcpreview'), null);
  ui.dom.window.close();
});

// A double click on Create wrote two vehicles (#740): the second press came before the first answer, and nothing stopped it.
function gateSpawn(ui) {
  const real = ui.w.fetch;
  let release;
  const gate = new Promise((r) => { release = r; });
  ui.w.fetch = (p, o) => (/\/api\/vehicles\/spawn$/.test(String(p)) ? gate.then(() => real(p, o)) : real(p, o));
  return release;
}

test('a double click on Create writes one vehicle: the button is off while the write is on its way and a second press does nothing', async () => {
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) } });
  await openForm(ui);
  const release = gateSpawn(ui);
  ui.click('[data-act="vcCreate"]');
  ui.click('[data-act="vcCreate"]');
  ui.click('[data-act="vcCreate"]');
  await ui.wait(20);
  assert.equal(ui.doc.querySelector('[data-act="vcCreate"]').disabled, true, 'Create is off while the answer is not in');
  release(); await ui.settle();
  assert.equal(posts(ui, '/api/vehicles/spawn').length, 1, 'one press, one request');
  assert.ok(ui.doc.querySelector('#vcresult'), 'the result is shown');
  ui.dom.window.close();
});

test('a Create that is refused turns the button back on, and the next press is a new request with the same form id', async () => {
  const post = { '/api/vehicles/spawn': { __fail: 'the save is busy', __status: 409 } };
  const ui = await boot({ get: GET, post });
  await openForm(ui);
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('[data-act="vcCreate"]').disabled, false, 'Create is on again after a refusal');
  assert.ok(ui.doc.querySelector('#vcClass'), 'the form stays');
  post['/api/vehicles/spawn'] = ANSWER({ outcome: 'written' });
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  const made = posts(ui, '/api/vehicles/spawn');
  assert.equal(made.length, 2);
  assert.match(made[0].body.requestId, /^[A-Za-z0-9_-]{1,64}$/, 'the id is there');
  assert.equal(made[0].body.requestId, made[1].body.requestId, 'one form, one id: the editor can tell a repeat of it');
  assert.ok(ui.doc.querySelector('#vcresult'));
  ui.dom.window.close();
});

test('a second form gets its own id, so a vehicle made after another one is a new vehicle', async () => {
  const ui = await boot({ get: GET, post: { '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) } });
  await openForm(ui);
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  ui.click('[data-act="vcOpen"]'); await ui.settle();
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  const made = posts(ui, '/api/vehicles/spawn');
  assert.equal(made.length, 2);
  assert.notEqual(made[0].body.requestId, made[1].body.requestId);
  ui.dom.window.close();
});

// ---------- Text typed while a pick in the preview table is on its way (#738)
// A form with a cargo hold and its preview shown; the next answer to the preview is held until release() is called.
async function holdForm(ui, post, next) {
  await openForm(ui);
  await change(ui, '[data-vcslot="UtilitySlot1"]', 'BuggyInventory_6');
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#vcpreview') && ui.doc.querySelector('#vcItem'), 'a preview and a cargo hold');
  post['/api/vehicles/spawn-preview'] = next;
  const release = gatePreview(ui, true);
  fire(ui, '#vcpreview [data-vcprev="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  await ui.wait(WAIT); // the request is out and held
  assert.equal(ui.doc.querySelector('[data-act="vcCreate"]').disabled, true, 'the answer is not in yet');
  return release;
}
// types into a box the way a person does, with the caret where it stands after the last key
function typeIn(ui, sel, text, caret = text.length) {
  const el = ui.doc.querySelector(sel);
  assert.ok(el, sel);
  el.focus();
  el.value = text;
  if (el.type === 'text') el.setSelectionRange(caret, caret);
  el.dispatchEvent(new ui.w.Event('input', { bubbles: true }));
  return el;
}

test('text typed in the item, quantity and fuel boxes while a pick is on its way is still there when the answer is drawn, and the caret stays where it was', async () => {
  const post = { '/api/vehicles/spawn-preview': SWAPPED(), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
  const ui = await boot({ get: GET, post });
  const release = await holdForm(ui, post, SWAPPED({ id: 1871 }));
  typeIn(ui, '#vcQty', '5');
  typeIn(ui, '#vcFuel', '40');
  typeIn(ui, '#vcItem', 'Spice', 2); // the caret is after "Sp"
  release(); await ui.settle();
  assert.ok(ui.doc.querySelector('#vcpreview').textContent.includes('Would create vehicle 1871'), 'the answer was drawn');
  assert.equal(ui.doc.querySelector('#vcItem').value, 'Spice');
  assert.equal(ui.doc.querySelector('#vcQty').value, '5');
  assert.equal(ui.doc.querySelector('#vcFuel').value, '40');
  assert.equal(ui.doc.activeElement, ui.doc.querySelector('#vcItem'), 'the focus stays in the box being typed in');
  assert.equal(ui.doc.querySelector('#vcItem').selectionStart, 2, 'the caret stays');
  assert.equal(ui.doc.querySelector('#vcItem').selectionEnd, 2);
  // the fuel is in the draft, so Create sees that the preview was asked for another figure and writes nothing
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  assert.equal(posts(ui, '/api/vehicles/spawn').length, 0, 'nothing is written');
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('The form changed after the preview'));
  ui.dom.window.close();
});

test('a refused pick keeps the text typed meanwhile too, and a typed quantity of a line already in the list', async () => {
  const post = { '/api/vehicles/spawn-preview': SWAPPED(), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
  const ui = await boot({ get: GET, post });
  await openForm(ui);
  await change(ui, '[data-vcslot="UtilitySlot1"]', 'BuggyInventory_6');
  set(ui, '#vcItem', 'Spice Melange');
  ui.click('[data-act="vcAddItem"]'); await ui.settle();
  ui.click('[data-act="vcPreview"]'); await ui.settle();
  post['/api/vehicles/spawn-preview'] = { __fail: 'no', __code: 'bad_argument' };
  const release = gatePreview(ui, true);
  fire(ui, '#vcpreview [data-vcprev="Engine"]', 'BuggyEngine_Unique_Accelerate_06');
  await ui.wait(WAIT);
  typeIn(ui, '[data-vcqty="0"]', '12');
  typeIn(ui, '#vcItem', 'Lit');
  release(); await ui.settle();
  assert.ok(ui.doc.querySelector('#vcpartnote'), 'the refusal is said');
  assert.equal(ui.doc.querySelector('[data-vcprev="Engine"]').value, 'BuggyEngine_Unique_Accelerate_06', 'the parts went back to the ones the old preview shows');
  assert.equal(ui.doc.querySelector('#vcItem').value, 'Lit');
  assert.equal(ui.doc.querySelector('[data-vcqty="0"]').value, '12');
  ui.dom.window.close();
});

test('boxes nobody typed in are drawn from the draft as before: an answer changes nothing in them', async () => {
  const post = { '/api/vehicles/spawn-preview': SWAPPED(), '/api/vehicles/spawn': ANSWER({ outcome: 'written' }) };
  const ui = await boot({ get: GET, post });
  const release = await holdForm(ui, post, SWAPPED({ id: 1872 }));
  release(); await ui.settle();
  assert.equal(ui.doc.querySelector('#vcItem').value, '');
  assert.equal(ui.doc.querySelector('#vcQty').value, '1');
  assert.equal(ui.doc.querySelector('#vcFuel').value, '100');
  ui.click('[data-act="vcCreate"]'); await ui.settle();
  assert.equal(posts(ui, '/api/vehicles/spawn').length, 1, 'with nothing typed the preview still matches and Create writes');
  ui.dom.window.close();
});
