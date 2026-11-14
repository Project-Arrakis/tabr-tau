// The fill, refill, refuel and repair actions of the editor play an animation (#571), with Motion = Full only: the icon of the action (a drop, a ring, a can, a shield; the heart and the bolt are
// drawn too, for the fills of health and stamina) plays on the button that was pressed and on the result toast. The icon is chosen by the ACTION that was pressed (FX_OF in app.js), never by the
// words of the result. A result where something was written is green with the icon, even where the rest was already full; a result where nothing was written is amber with no icon, never red; an
// error is red with no icon. Calm and Off show the plain check. The tests below walk the whole list, and a scan of app.js fails for a fill or repair action that has no animation.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const static_ = path.join(here, '..', 'internal', 'web', 'static');
const read = (n) => fs.readFileSync(path.join(static_, n), 'utf8').replace(/\r\n/g, '\n');
const appJs = read('app.js');
const appCss = read('app.css');
const motionBoot = read('motion-boot.js');

const base = (id) => ({ base_id: id, name: 'Base ' + id, base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 3, placeables: 2, x: 0, y: 0, z: 0 });
const STRUCT = { base_id: 7, pieces: { n: 3, minh: 1000, avgh: 3833.33, sand: 50 }, types: [{ building_type: 'Wall', n: 1, minh: 1000, maxh: 1000, avgh: 1000 }], placeables: [{ id: 601, building_type: 'Totem_Placeable', health: 5000 }],
  unowned: { pieces: { n: 0, minh: 0, avgh: 0, sand: 0 }, types: [], placeables: [] }, save: { pieces: 3, placeables: 2 } };
const VEH = { vehicles: [{ id: 41, name: 'BP_Sandbike_CHOAM', type: 'Sandbike', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [], cargo: null }], hidden: 0, recovered: [], backups: [] };
const GET = {
  '/api/player/blood-sacks': { total: 4, notFull: 3, unknown: 0, holdingOther: 0 },
  '/api/player/liquids': { water: { total: 3, notFull: 2, unknown: 0 }, bloodSacks: { total: 4, notFull: 1, unknown: 0, holdingOther: 0 }, vehicles: { total: 5, notFull: 3, unknown: 0, noFuelRecord: 0 } },
  '/api/vehicles': VEH, '/api/vehicles/fuel': { total: 2, notFull: 1, unknown: 0, noFuelRecord: 0 },
  '/api/bases/list': [base(7), base(8)], '/api/bases/structures': STRUCT,
};

const go = async (ui, ...clicks) => { for (const c of clicks) { ui.click(c); await ui.settle(); } };
const character = (ui) => go(ui, '[data-tab="player"]');
const inventory = (ui) => go(ui, '[data-tab="player"]', '[data-sub="char:inventory"]');
const vehicles = (ui) => go(ui, '[data-tab="player"]', '[data-sub="player:vehicles"]');
const bases = (tab) => (ui) => go(ui, '[data-tab="player"]', '[data-sub="player:bases"]', `[data-bstab="${tab}"]`);
const structures = (ui) => go(ui, '[data-tab="player"]', '[data-sub="player:bases"]', '[data-bstab="structures"]');

// Every fill, refill, refuel and repair action of the editor: the icon it plays, where its button is, the route it posts to, and two answers of that route: one where something was written
// (the rest may have been full already) and one where nothing was written. A new action of this kind has to be added here, and to FX_OF in app.js.
const ACTIONS = [
  { act: 'fillWater', fx: 'water', open: character, route: '/api/player/hydration', wrote: { ok: true, before: 12, after: 100, outcome: 'written' }, none: { ok: true, before: 100, after: 100, outcome: 'written' } },
  { act: 'fillBlood', fx: 'water', open: character, route: '/api/player/fill-blood-sacks', wrote: { ok: true, filled: 2, alreadyFull: 1, byTemplate: {}, unknown: 0, holdingOther: 0 }, none: { ok: true, filled: 0, alreadyFull: 3, byTemplate: {}, unknown: 0, holdingOther: 0 } },
  { act: 'refill', fx: 'water', open: character, route: '/api/player/refill', wrote: { ok: true, filled: 3, alreadyFull: 1, skippedUnknown: [] }, none: { ok: true, filled: 0, alreadyFull: 4, skippedUnknown: [] } },
  { act: 'fillLiquids', fx: 'all', open: character, route: '/api/player/fill-all-liquids', wrote: { ok: true, water: { filled: 2 }, bloodSacks: { filled: 0 }, vehicles: { filled: 0 }, vehiclesIncluded: true }, none: { ok: true, water: { filled: 0 }, bloodSacks: { filled: 0 }, vehicles: { filled: 0 }, vehiclesIncluded: true } },
  { act: 'repair', fx: 'repair', open: inventory, route: '/api/player/repair', wrote: { ok: true, repaired: 4 }, none: { ok: true, repaired: 0, backpackRepaired: 0 } },
  { act: 'refuelV', fx: 'fuel', open: vehicles, route: '/api/vehicles/refuel', wrote: { ok: true, filled: 3, byClass: {}, alreadyFull: 1, total: 5, unknown: 0, skippedUnknown: [], noFuelRecord: 0 }, none: { ok: true, filled: 0, byClass: {}, alreadyFull: 2, total: 2, unknown: 0, skippedUnknown: [], noFuelRecord: 0 } },
  { act: 'repairV', fx: 'repair', open: vehicles, route: '/api/vehicles/repair', wrote: { ok: true, modules: 9, repaired: 7, withoutKnownMax: 0 }, none: { ok: true, modules: 9, repaired: 0, withoutKnownMax: 0 } },
  { act: 'repairOne', fx: 'repair', open: vehicles, route: '/api/vehicles/repair', wrote: { ok: true, modules: 3, repaired: 1, withoutKnownMax: 0 }, none: { ok: true, modules: 3, repaired: 0, withoutKnownMax: 0 } },
  { act: 'refillWater', fx: 'water', open: bases('water'), route: '/api/bases/refill-water', wrote: { ok: true, filled: 2, alreadyFull: 1, skippedUnknown: [], summary: '2 Medium Water Cisterns' }, none: { ok: true, filled: 0, alreadyFull: 3, skippedUnknown: [], summary: '' } },
  { act: 'refillGen', fx: 'fuel', open: bases('power'), route: '/api/bases/refill-generators', wrote: { ok: true, filled: 3, alreadyFull: 0, summary: '3 Windtraps' }, none: { ok: true, filled: 0, alreadyFull: 3, summary: '' } },
  { act: 'repairB', fx: 'repair', open: structures, route: '/api/bases/repair', wrote: { ok: true, pieces: 2, placeables: 1 }, none: { ok: true, pieces: 0, placeables: 0 } },
  { act: 'repairAll', fx: 'repair', open: structures, route: '/api/bases/repair-all', wrote: { ok: true, pieces: 5, placeables: 2 }, none: { ok: true, pieces: 0, placeables: 0 } },
];

// opts.motion: the Motion setting kept in the browser ('full' | 'calm' | 'off'); the real motion-boot.js runs before the page, as in the editor
async function press(a, reply, motion = 'full') {
  const ui = await boot({ get: GET, post: { [a.route]: reply }, local: { tauMotion: motion }, pre: (w) => w.eval(motionBoot) });
  assert.equal(ui.doc.documentElement.getAttribute('data-motion'), motion);
  await a.open(ui);
  const find = () => ui.doc.querySelector(`#main [data-act="${a.act}"]`);
  const b = find();
  assert.ok(b, a.act + ' has a button');
  const before = { text: b.textContent, title: b.title, aria: b.getAttribute('aria-describedby'), cls: [...b.classList].join(' ') };
  b.disabled = false; // a count on the page can be older than the press: the editor's own answer decides
  ui.click(`#main [data-act="${a.act}"]`); await ui.settle();
  const toast = ui.doc.querySelector('#toast .d-toast');
  assert.ok(toast, a.act + ' shows a toast');
  return { ui, toast, button: find(), before, overlays: () => [...ui.doc.querySelectorAll('#main .x-btnfx')] };
}

test('Motion = Full: every fill, refill, refuel and repair plays its icon on the button that was pressed and on the toast (the whole list)', async () => {
  for (const a of ACTIONS) {
    const r = await press(a, a.wrote);
    assert.ok(r.toast.classList.contains('d-toast') && !r.toast.classList.contains('warn') && !r.toast.classList.contains('bad') && !r.toast.classList.contains('err'), a.act + ': a success is green');
    assert.equal(r.toast.getAttribute('data-fx'), a.fx, a.act + ' plays ' + a.fx + ' on the toast');
    assert.ok(r.toast.querySelector('svg.d-fx.' + a.fx + ' .lvl') && !r.toast.querySelector('svg.d-i'), a.act + ': the icon, not the plain check');
    const o = r.overlays();
    assert.equal(o.length, 1, a.act + ': one overlay');
    assert.ok(o[0].matches('svg.d-fx.' + a.fx), a.act + ' plays ' + a.fx + ' on the button');
    assert.equal(o[0].parentNode, r.button, a.act + ': on the button that is on the page after the redraw');
    assert.ok(r.button.classList.contains('x-fxing'), a.act);
    assert.equal(r.button.textContent, r.before.text, a.act + ': the words of the button are as they were');
    assert.equal(r.button.title, r.before.title, a.act);
    assert.equal(r.button.getAttribute('aria-describedby'), r.before.aria, a.act);
    assert.equal(r.ui.doc.querySelectorAll('#main .x-fxing').length, 1, a.act + ': no other button plays');
    r.ui.dom.window.close();
  }
});

test('a result where something was written and the rest was already full is a success: green, with the icon', async () => {
  const a = ACTIONS.find((x) => x.act === 'refill');
  const r = await press(a, { ok: true, filled: 1, alreadyFull: 6, skippedUnknown: [] });
  assert.ok(r.toast.textContent.includes('6 already full'));
  assert.equal(r.toast.getAttribute('data-fx'), 'water');
  assert.equal(r.overlays().length, 1);
  const b = await press(ACTIONS.find((x) => x.act === 'repairB'), { ok: true, pieces: 0, placeables: 1 });
  assert.equal(b.toast.getAttribute('data-fx'), 'repair', 'only the placeables were written: still a success');
});

test('nothing written is amber with no icon, never red, and nothing plays on the button (the whole list)', async () => {
  for (const a of ACTIONS) {
    const r = await press(a, a.none);
    assert.ok(r.toast.classList.contains('warn') && r.toast.classList.contains('note'), a.act + ': amber');
    assert.ok(!r.toast.classList.contains('bad') && !r.toast.classList.contains('err') && !r.toast.querySelector('.x'), a.act + ': never red, and it leaves by itself');
    assert.equal(r.toast.getAttribute('role'), 'status', a.act);
    assert.ok(r.toast.querySelector('svg.d-i') && !r.toast.querySelector('svg.d-fx') && !r.toast.hasAttribute('data-fx'), a.act + ': a mark of its own, no fill icon');
    assert.equal(r.overlays().length, 0, a.act);
    assert.equal(r.ui.doc.querySelectorAll('.x-fxing').length, 0, a.act);
    r.ui.dom.window.close();
  }
});

test('an error is red with no icon and nothing plays on the button (the whole list)', async () => {
  for (const a of ACTIONS) {
    const r = await press(a, { __fail: 'The save is not writable.', __status: 500 });
    assert.ok(r.toast.classList.contains('bad') && r.toast.classList.contains('err'), a.act + ': red');
    assert.equal(r.toast.getAttribute('role'), 'alert', a.act);
    assert.ok(r.toast.textContent.includes('The save is not writable.'), a.act);
    assert.ok(!r.toast.querySelector('svg.d-fx') && !r.toast.hasAttribute('data-fx'), a.act);
    assert.equal(r.overlays().length, 0, a.act);
    r.ui.dom.window.close();
  }
});

test('Motion = Calm and Off: the plain check on the toast and nothing on any button (the whole list)', async () => {
  for (const motion of ['calm', 'off']) {
    for (const a of ACTIONS) {
      const r = await press(a, a.wrote, motion);
      assert.ok(!r.toast.classList.contains('warn') && !r.toast.classList.contains('bad'), motion + ' ' + a.act + ': still a success');
      assert.ok(r.toast.querySelector('svg.d-i') && !r.toast.querySelector('svg.d-fx') && !r.toast.hasAttribute('data-fx'), motion + ' ' + a.act + ': the plain check');
      assert.equal(r.overlays().length, 0, motion + ' ' + a.act);
      assert.equal(r.ui.doc.querySelectorAll('.x-fxing').length, 0, motion + ' ' + a.act);
      r.ui.dom.window.close();
    }
  }
});

test('the system asking for reduced motion, with nothing chosen, is Calm: the plain check', async () => {
  const ui = await boot({ get: GET, post: { '/api/player/refill': ACTIONS[2].wrote }, pre: (w) => { w.matchMedia = () => ({ matches: true, addEventListener() {}, removeEventListener() {} }); w.eval(motionBoot); } });
  assert.equal(ui.doc.documentElement.getAttribute('data-motion'), 'calm');
  await character(ui);
  ui.click('[data-act="refill"]'); await ui.settle();
  const t = ui.doc.querySelector('#toast .d-toast');
  assert.ok(t.querySelector('svg.d-i') && !t.querySelector('svg.d-fx'));
  assert.equal(ui.doc.querySelectorAll('.x-btnfx').length, 0);
});

test('a second press plays again on the same button, and the overlay is gone when its time is up', async () => {
  const a = ACTIONS.find((x) => x.act === 'refill');
  const r = await press(a, a.wrote);
  assert.equal(r.overlays().length, 1);
  r.ui.click('#main [data-act="refill"]'); await r.ui.settle();
  assert.equal(r.overlays().length, 1, 'one overlay, not two');
  await r.ui.wait(2400);
  assert.equal(r.overlays().length, 0, 'taken away by its own timer');
  assert.equal(r.ui.doc.querySelectorAll('.x-fxing').length, 0);
});

test('the icon of an action is the one in the table, whatever the result says: the words of a result never choose it', async () => {
  const a = ACTIONS.find((x) => x.act === 'refuelV');
  // a result whose words talk about water, repairs and a shield: the refuel still plays the can
  const r = await press(a, { ...a.wrote, byClass: { BP_Sandbike_CHOAM_C: 1 }, skippedUnknown: ['Repaired water shield x1'], unknown: 1 });
  assert.equal(r.toast.getAttribute('data-fx'), 'fuel');
  assert.ok(r.overlays()[0].matches('svg.d-fx.fuel'));
  const body = appJs.slice(appJs.indexOf('function fillToast'), appJs.indexOf('function playFx'));
  assert.ok(body.includes('{ kind: FX_OF[name] }'), 'fillToast reads the table by the action name');
  assert.ok(!/msg\.(includes|match|startsWith|search)|\.test\(msg\)|indexOf\(msg/.test(body), 'and never looks at the words of the result');
});

// ----- the list is complete: a new fill or repair action without an animation fails here -----
const tableOf = () => {
  const m = /const FX_OF = (\{[^;]*\});/.exec(appJs);
  assert.ok(m, 'FX_OF is in app.js');
  return Object.fromEntries([...m[1].matchAll(/(\w+): '(\w+)'/g)].map((x) => [x[1], x[2]]));
};
// the action table of app.js: the name of each entry and its text
const actionTable = () => {
  const start = appJs.indexOf('\nconst A = {\n');
  assert.ok(start > 0, 'the action table is in app.js');
  const end = appJs.indexOf('\n};\n', start);
  const out = {};
  let cur = null;
  for (const line of appJs.slice(start, end).split('\n')) {
    const m = /^ {2}(\w+): /.exec(line);
    if (m) { cur = m[1]; out[cur] = ''; }
    if (cur) out[cur] += line + '\n';
  }
  return out;
};
// the actions that look like a fill or a repair but are something else, with the reason (a new name here is a decision, not a way round the test)
const NOT_FILLS = {
  rsUnlock: 'Research: purchases or repairs an unlock record, nothing is filled or worn',
  autoref: 'the switch of the automatic pass',
  autoNow: 'the automatic pass: runs the jobs that are ticked and reports in its own banner (a later piece of #571)',
  launchSkip: 'ends the countdown of the automatic pass',
  launchOff: 'turns the automatic pass off',
};

test('every action of the editor that fills, refills, refuels or repairs has an animation, and the list in this file is the table of app.js', () => {
  const table = tableOf();
  const acts = actionTable();
  assert.ok(Object.keys(acts).length > 50, 'the action table was read');
  // by what they send: a route with fill, refill, refuel, repair or hydration in it
  const byRoute = Object.keys(acts).filter((k) => /\/api\/[\w/-]*(fill|repair|refuel|hydration)/.test(acts[k]));
  for (const k of byRoute) {
    if (NOT_FILLS[k]) continue;
    assert.ok(table[k], k + ' fills or repairs something (' + (/\/api\/[\w/-]+/.exec(acts[k]) || [''])[0] + '): add it to FX_OF in app.js and to ACTIONS in this file');
  }
  // by the words of its button: a button labelled Fill, Refill, Refuel or Repair
  const labelled = [...appJs.matchAll(/<button[^>]*data-act="(\w+)"[^>]*>([^<$]*)</g)].filter((m) => /\b(fill|refill|refuel|repair)/i.test(m[2])).map((m) => m[1]);
  assert.ok(labelled.length >= 8, 'the buttons were read: ' + labelled.join(','));
  for (const k of labelled) {
    if (NOT_FILLS[k]) continue;
    assert.ok(table[k], 'the button of ' + k + ' says fill, refill, refuel or repair: add it to FX_OF in app.js and to ACTIONS in this file');
  }
  // the table names real actions, each one is in the list of this file, and its icon is one the page draws
  assert.deepEqual(Object.keys(table).sort(), ACTIONS.map((a) => a.act).sort(), 'FX_OF and the list in this file are the same');
  for (const k of Object.keys(table)) assert.ok(Object.hasOwn(acts, k), k + ' is an action');
  for (const a of ACTIONS) assert.equal(table[a.act], a.fx, a.act);
  for (const k of Object.keys(NOT_FILLS)) assert.ok(!table[k], k + ' is not a fill and has no icon');
  assert.match(appJs, /const FX_SHAPE = \{ water:.*fuel:.*repair:[\s\S]*health:.*stamina:.*all:/, 'the page draws all six icons: drop, can, shield, heart, bolt and ring');
  // each icon has its colour on the toast and on the button
  for (const k of ['health', 'stamina', 'all']) assert.ok(appCss.includes('.d-fx.' + k) && appCss.includes('.x-btnfx.' + k), k + ' has its colour');
});

test('the Motion setting is where the editor keeps its other display options: the Theme menu, with Full, Calm and Off, kept in the browser', async () => {
  const ui = await boot({ pre: (w) => w.eval(motionBoot) });
  assert.deepEqual([...ui.w.TauMotion.VALUES], ['full', 'calm', 'off']);
  assert.equal(ui.w.TauMotion.KEY, 'tauMotion');
  assert.match(read('theme-menu.js'), /id: 'motion', title: 'Motion', values: \['full', 'calm', 'off'\]/);
});
