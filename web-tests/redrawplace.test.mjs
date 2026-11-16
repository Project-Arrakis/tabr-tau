// A change inside a page is not a move to another page (#686, after #650): a base opened or its sub-tab or group changed, an inventory group or filter,
// the kit and augment editors, the table browser's paging and the rest keep the scroll offset and the focused control. jsdom has no layout, so the
// offset is a number the test holds, and a redraw from nothing puts it at 0 unless the page puts it back.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import nodePath from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll, GAP } from './boot.mjs';

afterEach(closeAll);

function holdWindowScroll(w, y) {
  let at = y;
  Object.defineProperty(w, 'scrollY', { get: () => at, configurable: true });
  w.scrollTo = (x, yy) => { at = yy; };
  new w.MutationObserver(() => { at = 0; }).observe(w.document.querySelector('#main'), { childList: true });
  return () => at;
}
const set = (ui, sel, v) => { const el = ui.doc.querySelector(sel); assert.ok(el, sel); el.value = v; return el; };
const row = (id) => ({ base_id: id, name: 'Base ' + id, base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 3, placeables: 2, x: 0, y: 0, z: 0 });
const ITEMS = [{ id: 'MelangeSpice', name: 'Spice Melange', category: 'resources' }, { id: 'Literjon', name: 'Literjon', category: 'tools' }];
const inv = (id, type, tpl = 'Literjon', extra = {}) => ({ id, inventory_id: 900011 + type, inventory_name: 'Backpack', inventory_type: type, template_id: tpl, position_index: id, quality_level: 0, stack_size: 1, durability: null, max_durability: null, aug_limit: 0, augments: null, ...extra });
const INV = { inventories: [{ id: 900011, inventory_type: 0, max_item_count: 20, max_item_volume: 1000 }], items: [inv(1, 0, 'Literjon', { aug_limit: 2 }), inv(2, 0, 'MelangeSpice'), inv(3, 1, 'MelangeSpice')], templates: ['Literjon', 'MelangeSpice'] };
const BASEINV = { containers: [{ actor_id: 1, base_id: 7, building_type: 'GenericContainer_Placeable', group: 'storage', inventory_id: 1, items: 1, max_item_count: 20, name: 'Chest', type: 'Chest' }, { actor_id: 2, base_id: 7, building_type: 'GenericContainer_Placeable', group: 'other', inventory_id: 2, items: 1, max_item_count: 20, name: 'Crate', type: 'Chest' }], groups: [{ count: 1, id: 'storage', label: 'Storage' }, { count: 0, id: 'refining', label: 'Refining' }, { count: 0, id: 'crafting', label: 'Crafting' }, { count: 1, id: 'other', label: 'Other' }] };
const BASES = { '/api/bases/list': [row(7), row(8)], '/api/bases/inventory': BASEINV };
const LIMITS = { kits: 20, items: 50, name: 60, quantity: 1000000, xp: 100000000, skillPoints: 100000, fileBytes: 65536 };
const KIT = { name: 'Starter', items: [{ item: 'Literjon', quantity: 3 }, { item: 'MelangeSpice', quantity: 5 }], xp: 0, skillPoints: 0, problem: '' };
const go = async (ui, ...sels) => { for (const s of sels) { ui.click(s); await ui.settle(); } };
// Presses a control the way a person does: the focus is on it, the page has been scrolled, and the redraw must leave both as they were.
async function press(ui, sel, y = 900) {
  const at = holdWindowScroll(ui.w, y);
  const el = ui.doc.querySelector(sel); assert.ok(el, sel);
  el.focus();
  ui.click(sel); await ui.settle();
  return at;
}
const focused = (ui, sel) => assert.equal(ui.doc.activeElement, ui.doc.querySelector(sel), 'the focus is on ' + sel);

test('Bases: opening another base keeps the page offset', async () => {
  const ui = await boot({ get: BASES });
  await go(ui, '[data-tab="player"]', '[data-sub="player:bases"]');
  const at = await press(ui, '[data-bsopen="8"]');
  assert.equal(at(), 900);
  assert.ok(ui.doc.querySelector('[data-bsopen="7"]'), 'base 8 is the open one now');
});

test('Bases: a sub-tab and an inventory group keep the page offset and the focus on the button pressed', async () => {
  const ui = await boot({ get: BASES });
  await go(ui, '[data-tab="player"]', '[data-sub="player:bases"]');
  let at = await press(ui, '[data-bstab="inventory"]');
  assert.equal(at(), 900, 'a base sub-tab keeps the offset');
  focused(ui, '[data-bstab="inventory"]');
  assert.ok(ui.doc.querySelector('[data-bsgrp]'), 'the sub-tab is open');
  at = await press(ui, '[data-bsgrp="other"]', 640);
  assert.equal(at(), 640, 'a base inventory group keeps the offset');
  focused(ui, '[data-bsgrp="other"]');
  assert.ok(ui.doc.querySelector('[data-bsgrp="other"]').classList.contains('on'));
});

test('Character inventory: a group, Filter and Clear keep the page offset and the focus', async () => {
  const ui = await boot({ get: { '/api/player/inventory': INV, '/api/catalog/items': ITEMS } });
  await go(ui, '[data-tab="player"]', '[data-sub="char:inventory"]');
  let at = await press(ui, '[data-invg="character"]');
  assert.equal(at(), 900, 'an inventory group keeps the offset');
  focused(ui, '[data-invg="character"]');
  assert.ok(ui.doc.querySelector('[data-invg="character"]').classList.contains('on'));
  await go(ui, '[data-invg="backpack"]');
  set(ui, '#invq', 'spice');
  at = await press(ui, '[data-act="invfilter"]', 700);
  assert.equal(at(), 700, 'Filter keeps the offset');
  focused(ui, '[data-act="invfilter"]');
  assert.equal(ui.doc.querySelector('#invq').value, 'spice', 'the filter text is drawn from the page\'s own state');
  at = await press(ui, '[data-act="invclear"]', 700);
  assert.equal(at(), 700, 'Clear keeps the offset');
  focused(ui, '[data-act="invclear"]');
  assert.equal(ui.doc.querySelector('#invq').value, '');
});

test('Character inventory: Add to the queue and opening the augments keep the page offset', async () => {
  const ui = await boot({ get: { '/api/player/inventory': INV, '/api/catalog/items': ITEMS }, post: { '/api/items/augment-options': { limit: 2, options: [{ id: 'T6_Augment_Armor10', name: 'Armor 10', effects: [] }], applied: [] } } });
  await go(ui, '[data-tab="player"]', '[data-sub="char:inventory"]');
  set(ui, '#gt', 'Spice Melange');
  let at = await press(ui, '[data-act="queueAdd"]');
  assert.equal(at(), 900, 'the add keeps the offset');
  focused(ui, '[data-act="queueAdd"]');
  assert.ok(ui.doc.querySelector('#main').textContent.includes('Queue (1)'));
  assert.equal(ui.doc.querySelector('#gt').value, '', 'the item box is empty after the add, as before');
  at = await press(ui, '[data-act="augEdit"]', 1200);
  assert.equal(at(), 1200, 'opening the augments keeps the offset');
  assert.ok(ui.doc.querySelector('[data-act="augCancel"]'), 'the augment card is open');
});

test('Care package: New kit, Edit, Add item and removing an item keep the page offset', async () => {
  const ui = await boot({ get: { '/api/kits': { kits: [KIT], limits: LIMITS, problem: '' }, '/api/catalog/items': ITEMS } });
  await go(ui, '[data-tab="player"]', '[data-sub="char:carepackage"]');
  let at = await press(ui, '[data-act="kitNew"]');
  assert.equal(at(), 900, 'New kit keeps the offset');
  assert.ok(ui.doc.querySelector('#kitName'));
  await go(ui, '[data-act="kitCancel"]');
  at = await press(ui, '[data-act="kitEdit"]', 800);
  assert.equal(at(), 800, 'Edit keeps the offset');
  assert.equal(ui.doc.querySelectorAll('[data-act="kitDelLine"]').length, 2);
  set(ui, '#kitItem', 'Spice Melange');
  at = await press(ui, '[data-act="kitAddLine"]', 800);
  assert.equal(at(), 800, 'Add item keeps the offset');
  focused(ui, '[data-act="kitAddLine"]');
  assert.equal(ui.doc.querySelectorAll('[data-act="kitDelLine"]').length, 3);
  at = await press(ui, '[data-act="kitDelLine"][data-id="2"]', 800);
  assert.equal(at(), 800, 'removing an item keeps the offset');
  assert.equal(ui.doc.querySelectorAll('[data-act="kitDelLine"]').length, 2);
});

test('Database: Search, Previous and Next keep the page offset and the focus; the table box starts at its top', async () => {
  const many = { columns: ['_rowid_', 'id'], locked: null, rows: [[1, 1], [2, 2]], total: 250 };
  const ui = await boot({ get: { '/api/db/table': many } });
  await go(ui, '[data-tab="db"]');
  const box = () => ui.doc.querySelector('#dbout .scroll');
  box().scrollTop = 300;
  let at = await press(ui, '[data-act="dbnext"]', 1100);
  assert.equal(at(), 1100, 'Next keeps the offset');
  focused(ui, '[data-act="dbnext"]');
  assert.ok(ui.fetchedUrls.some((u) => u.includes('offset=100')), 'the next page was asked for');
  assert.equal(box().scrollTop, 0, 'the new rows start at the top of their box');
  at = await press(ui, '[data-act="dbprev"]', 1100);
  assert.equal(at(), 1100, 'Previous keeps the offset');
  set(ui, '#dbq', 'abc');
  at = await press(ui, '[data-act="dbgo"]', 1100);
  assert.equal(at(), 1100, 'Search keeps the offset');
  focused(ui, '[data-act="dbgo"]');
  assert.ok(ui.fetchedUrls.some((u) => u.includes('q=abc')));
});

test('Database: an edit refused because the row changed redraws the table and keeps the page offset', async () => {
  const one = { columns: ['_rowid_', 'id', 'name'], locked: null, rows: [[1, 1, 'a']], total: 1 };
  const ui = await boot({ get: { '/api/db/table': one }, post: { '/api/db/update': { __fail: 'the row changed', __code: 'conflict', __status: 409 } } });
  await go(ui, '[data-tab="db"]');
  const at = holdWindowScroll(ui.w, 1000);
  const reads = () => ui.fetched.filter((p) => p === '/api/db/table').length;
  const before = reads();
  const el = set(ui, 'input.dbc', 'b');
  el.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  await ui.wait(20); await ui.settle();
  assert.ok(reads() > before, 'the table is read again');
  assert.equal(at(), 1000);
});

test('Skills: a reset refused because the count changed redraws and keeps the page offset', async () => {
  const skills = { total: 10, unspent: 5, spent: 4, schools: [{ key: 'Trooper', label: 'Trooper' }], modules: [{ id: 'Skills.Ability.X', name: 'X', school: 'Trooper', kind: 'Ability', rank: 1, max: 3, spent: 1, ladder: [1, 3, 6], known: true }], resetCount: 3 };
  const ui = await boot({ get: { '/api/player/skills': skills }, post: { '/api/player/skills/reset': { __fail: 'the count changed', __code: 'conflict', __status: 409 } } }); ui.freeze();
  await go(ui, '[data-tab="player"]', '[data-sub="player:skills"]');
  const at = holdWindowScroll(ui.w, 1300);
  const reads = () => ui.fetched.filter((p) => p === '/api/player/skills').length;
  const before = reads();
  ui.click('[data-act="skReset"]'); await ui.settle();
  ui.type('reset 3 skills'); ui.tick(GAP);
  ui.click('#aok'); await ui.settle();
  await ui.wait(20); await ui.settle();
  assert.ok(reads() > before, 'the page is read again');
  assert.equal(at(), 1300);
});

// The focus goes back to a control the kept offset left where it was; the page moves only when that control is off the screen or under the sticky header.
test('the refocused control is scrolled into view only when it is off the screen', async () => {
  for (const [top, want] of [[200, 800], [1500, 800 + 1530 - 768 + 8], [10, 800 + 10 - 56 - 8]]) {
    const ui = await boot({ get: BASES });
    await go(ui, '[data-tab="player"]', '[data-sub="player:bases"]');
    const proto = ui.w.HTMLElement.prototype;
    proto.getBoundingClientRect = function () {
      if (this.tagName === 'HEADER') return { top: 0, bottom: 56, width: 1000, height: 56 };
      if (this.getAttribute && this.getAttribute('data-bstab') === 'water') return { top, bottom: top + 30, width: 80, height: 30 };
      return { top: 0, bottom: 0, width: 0, height: 0 };
    };
    const at = await press(ui, '[data-bstab="water"]', 800);
    assert.equal(at(), want, 'control at ' + top);
    ui.dom.window.close();
  }
});

// A table box keeps its scroll offset across a redraw that shows the same rows, and starts at its top when the redraw shows other rows (#697).
const boxOf = (ui) => ui.doc.querySelector('#main .scroll');

test('Character inventory: another group and Filter and Clear start the table box at its top, the page offset stays', async () => {
  const ui = await boot({ get: { '/api/player/inventory': INV, '/api/catalog/items': ITEMS } });
  await go(ui, '[data-tab="player"]', '[data-sub="char:inventory"]');
  for (const [sel, y] of [['[data-invg="character"]', 900], ['[data-invg="backpack"]', 640]]) {
    assert.ok(boxOf(ui), 'there is a table box before ' + sel);
    boxOf(ui).scrollTop = 300; boxOf(ui).scrollLeft = 7;
    const at = await press(ui, sel, y);
    assert.equal(at(), y, sel + ' keeps the page offset');
    if (boxOf(ui)) assert.deepEqual([boxOf(ui).scrollTop, boxOf(ui).scrollLeft], [0, 0], sel + ': the other rows start at the top of their box');
  }
  set(ui, '#invq', 'spice');
  boxOf(ui).scrollTop = 300;
  let at = await press(ui, '[data-act="invfilter"]', 700);
  assert.equal(at(), 700);
  assert.equal(boxOf(ui).scrollTop, 0, 'Filter: the filtered rows start at the top');
  boxOf(ui).scrollTop = 300;
  at = await press(ui, '[data-act="invclear"]', 700);
  assert.equal(at(), 700);
  assert.equal(boxOf(ui).scrollTop, 0, 'Clear: the rows start at the top');
});

test('Bases: another inventory group, a sub-tab and another base start the table box at its top, the page offset stays', async () => {
  const ui = await boot({ get: BASES });
  await go(ui, '[data-tab="player"]', '[data-sub="player:bases"]', '[data-bstab="inventory"]');
  const boxIn = (title) => { const c = [...ui.doc.querySelectorAll('#main .card')].find((x) => (x.querySelector('h3') || {}).textContent === title); return c && c.querySelector('.scroll'); };
  assert.ok(boxIn('Inventory'), 'the containers are in a table box');
  boxIn('Inventory').scrollTop = 300;
  let at = await press(ui, '[data-bsgrp="other"]', 640);
  assert.equal(at(), 640);
  assert.equal(boxIn('Inventory').scrollTop, 0, 'a base inventory group: its containers start at the top');
  boxIn('Inventory').scrollTop = 300;
  at = await press(ui, '[data-bsopen="8"]', 640);
  assert.equal(at(), 640);
  assert.equal(boxIn('Inventory').scrollTop, 0, 'another base: its table starts at the top');
});

// The list the button was pressed in keeps its offset: its rows did not change, and a list that jumps back to its top hides the button that has the focus (#824).
const many = (n, f) => Array.from({ length: n }, (_, i) => f(i + 1));
const veh = (id) => ({ id, name: 'BP_Sandbike_CHOAM', type: 'Sandbike', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [], cargo: null });

test('Vehicles: Details on a low row keeps the list at its offset, the button keeps the focus and the page offset stays', async () => {
  const ui = await boot({ get: { '/api/vehicles': { vehicles: many(40, veh), hidden: 0, recovered: [], backups: [] }, '/api/vehicles/fuel': { total: 40, notFull: 0, unknown: 0, noFuelRecord: 0 } } });
  await go(ui, '[data-tab="player"]', '[data-sub="player:vehicles"]');
  boxOf(ui).scrollTop = 420; boxOf(ui).scrollLeft = 5;
  const at = await press(ui, '[data-vhopen="39"]', 800);
  assert.equal(at(), 800, 'the page offset stays');
  assert.deepEqual([boxOf(ui).scrollTop, boxOf(ui).scrollLeft], [420, 5], 'the list the button is in keeps its offset');
  focused(ui, '[data-vhopen="39"]');
  assert.ok(boxOf(ui).contains(ui.doc.activeElement), 'the focused button is inside that list');
});

test('Bases: Open on a low row keeps the list of bases at its offset', async () => {
  const ui = await boot({ get: { '/api/bases/list': many(40, row), '/api/bases/inventory': BASEINV } });
  await go(ui, '[data-tab="player"]', '[data-sub="player:bases"]');
  boxOf(ui).scrollTop = 500;
  const at = await press(ui, '[data-bsopen="38"]', 800);
  assert.equal(at(), 800);
  assert.equal(boxOf(ui).scrollTop, 500, 'the list of bases keeps its offset');
});

// The page's own refresh from the file (about every twenty seconds while the game runs) gives the focus back to the control that had it, but must never
// scroll the page to it: the person pressed a button near the top, scrolled down to read, and the refresh must not pull the page back up (#696).
const fixtureState = () => JSON.parse(fs.readFileSync(nodePath.join(nodePath.dirname(fileURLToPath(import.meta.url)), 'fixtures', 'ui-fixtures.json'), 'utf8'))['/api/save/state'].body;

test('the automatic refresh keeps the focus and does not scroll the page; a press still scrolls its control into view', async () => {
  const state = { ...fixtureState(), changedOnDisk: false, dirty: false, pending: [] };
  const ui = await boot({ get: { ...BASES, '/api/save/state': state } });
  await go(ui, '[data-tab="player"]', '[data-sub="player:bases"]');
  // the control is far below the screen, so a scroll to it would be seen as a change of offset
  const proto = ui.w.HTMLElement.prototype;
  proto.getBoundingClientRect = function () {
    if (this.tagName === 'HEADER') return { top: 0, bottom: 56, width: 1000, height: 56 };
    if (this.getAttribute && this.getAttribute('data-bstab') === 'water') return { top: 1500, bottom: 1530, width: 80, height: 30 };
    return { top: 0, bottom: 0, width: 0, height: 0 };
  };
  const scrolls = [];
  const at = holdWindowScroll(ui.w, 0);
  const hold = ui.w.scrollTo;
  ui.w.scrollTo = (x, y) => { scrolls.push(y); hold(x, y); };
  const el = ui.doc.querySelector('[data-bstab="water"]'); assert.ok(el);
  el.focus();
  // the person presses the control (still on screen for this press), then scrolls down with the wheel to read
  ui.w.scrollTo(0, 900); scrolls.length = 0;
  ui.click('[data-bstab="water"]'); await ui.settle();
  focused(ui, '[data-bstab="water"]');
  assert.equal(at(), 900 + 1530 - 768 + 8, 'the press of a control that is off the screen still brings it into view');
  ui.w.scrollTo(0, 1500); scrolls.length = 0; // the person scrolled on with the wheel
  // the game rewrote the file: the next status poll sees it and the page reloads and redraws by itself
  state.changedOnDisk = true;
  await ui.w.status(); await ui.settle();
  assert.ok(ui.posted.some((p) => p.path === '/api/save/discard'), 'the automatic refresh ran');
  focused(ui, '[data-bstab="water"]');
  assert.equal(at(), 1500, 'the page offset is the one the person left');
  assert.deepEqual(scrolls.filter((y) => y !== 1500), [], 'the refresh moved the page only to put the kept offset back');
});
