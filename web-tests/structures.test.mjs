// Player > Bases > Structures is the open base's own (#526): the request names the base, the numbers on the page are the answer for that base,
// pieces no base owns are listed apart, the repair and the sand buttons send the base id, and the only whole-save figures and buttons are in
// their own card with their own labels.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { visibleText } from './uitext.mjs';

test.afterEach(() => closeAll());

const row = (id, over = {}) => ({ base_id: id, name: 'Base ' + id, base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 3, placeables: 2, x: 0, y: 0, z: 0, ...over });
const TWO = [row(7), row(8)];
// Base 7 has 3 pieces (lowest 1000, average 3833, sand 50); base 8 has 3 pieces of other health (lowest 2000, average 4533, sand 17); 2 pieces and 1 placeable belong to no base.
const S7 = {
  base_id: 7, pieces: { n: 3, minh: 1000, avgh: 3833.33, sand: 50 },
  types: [{ building_type: 'Foundation', n: 2, minh: 3500, maxh: 7000, avgh: 5250 }, { building_type: 'Wall', n: 1, minh: 1000, maxh: 1000, avgh: 1000 }],
  placeables: [{ id: 601, building_type: 'Totem_Placeable', health: 5000 }, { id: 851, building_type: 'WaterCistern_Placeable', health: 300 }],
  unowned: { pieces: { n: 2, minh: 100, avgh: 150, sand: 3 }, types: [{ building_type: 'Wall', n: 1, minh: 100, maxh: 100, avgh: 100 }, { building_type: 'Foundation', n: 1, minh: 200, maxh: 200, avgh: 200 }], placeables: [{ id: 854, building_type: 'LostCistern_Placeable', health: 50 }] },
  save: { pieces: 8, placeables: 6 },
};
const S8 = { ...S7, base_id: 8, pieces: { n: 3, minh: 2000, avgh: 4533.33, sand: 17 }, types: [{ building_type: 'Wall', n: 2, minh: 2000, maxh: 5600, avgh: 3800 }, { building_type: 'Foundation', n: 1, minh: 6000, maxh: 6000, avgh: 6000 }], placeables: [{ id: 602, building_type: 'Totem_Placeable', health: 4000 }, { id: 852, building_type: 'WaterCistern_Placeable', health: 1250 }, { id: 853, building_type: 'Generator_Placeable', health: 2500 }] };
const open = async (ui, id) => {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  if (id) { ui.click(`[data-bsopen="${id}"]`); await ui.settle(); }
  ui.click('[data-bstab="structures"]'); await ui.settle();
  return ui.doc.querySelector('#main');
};
const card = (main, title) => [...main.querySelectorAll('.card')].find((c) => c.querySelector('h3') && c.querySelector('h3').textContent.startsWith(title));

test('the Structures tab asks for the open base and shows that base\'s numbers, types and placeables', async () => {
  const ui = await boot({ get: { '/api/bases/list': TWO, '/api/bases/structures': S7 } });
  const main = await open(ui);
  assert.deepEqual(ui.fetchedUrls.filter((u) => u.startsWith('/api/bases/structures')), ['/api/bases/structures?base=7'], 'the request names the open base');
  const health = visibleText(card(main, 'Structure health'));
  for (const s of ['Building pieces3', 'Lowest health1,000', 'Average health3,833', 'Total sand buildup50', 'Repair this base to max', 'Clear sand on this base']) assert.ok(health.replace(/\s+/g, ' ').includes(s), s + ' | ' + health);
  const types = visibleText(card(main, 'Building piece types'));
  assert.ok(types.includes('Foundation') && types.includes('Foundation2350070005250') && types.includes('Wall1100010001000') && !types.includes('5600'), types);
  const placeables = visibleText(card(main, 'Placeables (2)'));
  assert.ok(placeables.includes('Totem_Placeable') && placeables.includes('WaterCistern_Placeable') && !placeables.includes('LostCistern_Placeable'), placeables);
  ui.dom.window.close();
});

test('opening another base asks for that base and shows its different figures; nothing of the first stays', async () => {
  const ui = await boot({ get: { '/api/bases/list': TWO, '/api/bases/structures': S7 } });
  await open(ui);
  ui.dom.window.close();
  const ui2 = await boot({ get: { '/api/bases/list': TWO, '/api/bases/structures': S8 } });
  const main = await open(ui2, 8);
  assert.deepEqual(ui2.fetchedUrls.filter((u) => u.startsWith('/api/bases/structures')), ['/api/bases/structures?base=8']);
  const health = visibleText(card(main, 'Structure health')).replace(/\s+/g, ' ');
  assert.ok(health.includes('Lowest health2,000') && health.includes('Average health4,533') && health.includes('Total sand buildup17') && !health.includes('1,000'), health);
  assert.ok(visibleText(card(main, 'Placeables (3)')).includes('Generator_Placeable'));
  ui2.dom.window.close();
});

test('pieces no base owns are listed apart and the whole-save card says where every piece is', async () => {
  const ui = await boot({ get: { '/api/bases/list': TWO, '/api/bases/structures': S7 } });
  const main = await open(ui);
  const un = visibleText(card(main, 'Not part of any base')).replace(/\s+/g, ' ');
  assert.ok(un.includes('2 building pieces and 1 placeables') && un.includes('LostCistern_Placeable'), un);
  assert.ok(!visibleText(card(main, 'Placeables (2)')).includes('LostCistern_Placeable'), 'never counted into the base');
  const all = visibleText(card(main, 'Whole save')).replace(/\s+/g, ' ');
  // 8 pieces and 6 placeables in the save: 3 and 2 here, 3 and 3 at the other base, 2 and 1 at no base
  assert.ok(all.includes('The save has 8 building pieces and 6 placeables: 3 building pieces and 2 placeables at this base, 3 and 3 at other bases, 2 and 1 at no base.'), all);
  ui.dom.window.close();
});

test('a base with no pieces says so, and no unowned card is drawn when nothing is unowned', async () => {
  const empty = { base_id: 7, pieces: { n: 0, minh: 0, avgh: 0, sand: 0 }, types: [], placeables: [], unowned: { pieces: { n: 0 }, types: [], placeables: [] }, save: { pieces: 0, placeables: 0 } };
  const ui = await boot({ get: { '/api/bases/list': TWO, '/api/bases/structures': empty } });
  const main = await open(ui);
  const t = visibleText(main);
  assert.ok(t.includes('This base has no building pieces.') && t.includes('No placeables at this base.') && t.includes('No building pieces at this base.'), t);
  assert.equal(card(main, 'Not part of any base'), undefined);
  ui.dom.window.close();
});

test('the buttons of the base send its id; the whole-save buttons are their own routes', async () => {
  const ui = await boot({ get: { '/api/bases/list': TWO, '/api/bases/structures': S8 }, post: { '/api/bases/repair': { ok: true, pieces: 2, placeables: 1 }, '/api/bases/clear-sand': { ok: true, pieces: 3 }, '/api/bases/repair-all': { ok: true, pieces: 5, placeables: 2 }, '/api/bases/clear-sand-all': { ok: true, pieces: 9 } } });
  await open(ui, 8);
  ui.click('[data-act="repairB"]'); await ui.settle();
  assert.deepEqual(ui.posted.map((p) => [p.path, p.body]), [['/api/bases/repair', { base_id: 8 }]]);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Repaired 2 pieces, 1 placeables at this base'));
  ui.click('[data-act="sand"]'); await new Promise((r) => ui.w.setTimeout(r, 450)); ui.click('[data-act="sand"]'); await ui.settle();
  assert.deepEqual(ui.posted.slice(1).map((p) => [p.path, p.body]), [['/api/bases/clear-sand', { base_id: 8 }]]);
  ui.click('[data-act="repairAll"]'); await ui.settle();
  assert.deepEqual(ui.posted.slice(2).map((p) => p.path), ['/api/bases/repair-all']);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('in the whole save'));
  ui.click('[data-act="sandAll"]'); await new Promise((r) => ui.w.setTimeout(r, 450)); ui.click('[data-act="sandAll"]'); await ui.settle();
  assert.deepEqual(ui.posted.slice(3).map((p) => p.path), ['/api/bases/clear-sand-all']);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Cleared 9 pieces in the whole save'));
  ui.dom.window.close();
});
