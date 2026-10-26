// The text sweep (#426) on Player > Character: Inventory, Reputation and Care package. See uitext.test.mjs for the checks that hold on every page.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { visibleText, movedText } from './uitext.mjs';

test.afterEach(() => closeAll());

const open = async (ui) => { ui.click('[data-tab="player"]'); await ui.settle(); };
const itm = (id, tmpl, inv, slot, type) => ({ id, template_id: tmpl, stack_size: 1, quality_level: 0, position_index: slot, inventory_id: inv, inventory_type: type, inventory_name: 'x', durability: null, max_durability: null, augments: [], aug_limit: 0 });
const INV = { items: [itm(10, 'SolarisCoin', 1, 0, 0), itm(11, 'Knife', 1, 1, 0)], inventories: [{ id: 1, inventory_type: 0, max_item_count: 35 }], templates: ['SolarisCoin', 'Knife'] };
const LIMITS = { kits: 2, items: 50, name: 60, quantity: 1000000, xp: 100000000, skillPoints: 100000, fileBytes: 65536 };
const KIT = (name) => ({ name, items: [], xp: 900, skillPoints: 0, problem: '' });
const sub = async (ui, s) => { await open(ui); ui.click(`[data-sub="${s}"]`); await ui.settle(); };

test('Inventory: the explanations are tooltips or in About this, the pointer line is gone, and the counts and state stay', async () => {
  const ui = await boot({ get: { '/api/player/inventory': INV } });
  await sub(ui, 'char:inventory');
  const main = ui.doc.querySelector('#main');
  const shown = visibleText(main), moved = movedText(main);
  for (const s of ['Everything you wear and everything in your loadout is made as good as new', 'Pick an item by its in-game name',
    'Removes the 1 item in Backpack, with everything attached to them', 'The fill actions (Fill hydration, Fill blood sacks, Refill Literjons, Fill all liquids) are on Character > Overview']) {
    assert.ok(!shown.includes(s), 'still shown: ' + s);
    assert.ok(moved.includes(s), 'lost, not moved: ' + s);
  }
  assert.ok(!shown.includes('The fill actions are on the Character tab') && !moved.includes('The fill actions are on the Character tab'), 'the old pointer is gone');
  assert.equal(ui.doc.querySelector('#fillmoved'), null);
  for (const s of ['Inventory: 2 items carried', '2 items', '2 of 35 slots used', 'Repair all worn items and loadouts', 'Clear this container', 'Also remove currency']) assert.ok(shown.includes(s), 'missing from the default view: ' + s);
  // the way back to the fill actions is one click away, in the card's About this
  assert.ok(ui.doc.querySelector('details[data-about="inventory.card"] [data-goto="character:overview"]'));
  ui.click('details[data-about="inventory.card"] [data-goto="character:overview"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('[data-act="fillLiquids"]'), 'it goes to the Fill card');
  ui.dom.window.close();
});

test('Inventory: an empty container says it is empty (state) and has no tooltip to explain a button that is off', async () => {
  const ui = await boot({ get: { '/api/player/inventory': { items: [], inventories: [{ id: 1, inventory_type: 0, max_item_count: 35 }], templates: [] } } });
  await sub(ui, 'char:inventory');
  const btn = ui.doc.querySelector('[data-act="clearInv"]');
  assert.equal(btn.disabled, true);
  assert.equal(btn.closest('.tip'), null);
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('Backpack is empty: nothing to clear.'));
  assert.equal(btn.getAttribute('aria-describedby'), 'clrnote');
  ui.dom.window.close();
});

test('Reputation: the range is on each box, not in a permanent line', async () => {
  const ui = await boot({ get: { '/api/player/factions': [{ faction_id: 1, name: 'House Atreides', reputation: 12 }] } });
  await sub(ui, 'char:reputation');
  const main = ui.doc.querySelector('#main');
  assert.ok(!visibleText(main).includes('Range 0'), visibleText(main));
  assert.ok(!visibleText(main).includes('press Tab/Enter'));
  const box = ui.doc.querySelector('[data-faction="1"]');
  assert.equal(box.title, '0 to 12474');
  assert.equal(box.getAttribute('max'), '12474');
  assert.ok(visibleText(main).includes('House Atreides'));
  ui.dom.window.close();
});

test('Care package: the description is in About this; the full-list line is drawn only while the list is full; the editor\'s hints are tooltips', async () => {
  let ui = await boot({ get: { '/api/kits': { kits: [KIT('One')], limits: LIMITS, problem: '' } } });
  await sub(ui, 'char:carepackage');
  let main = ui.doc.querySelector('#main');
  assert.ok(!visibleText(main).includes('A care package is a named kit'));
  assert.ok(movedText(main).includes('A care package is a named kit of items, XP and skill points') && movedText(main).includes('Up to 2 kits, each with up to 50 items'));
  assert.equal(ui.doc.querySelector('#kitfull'), null, 'room for another kit: no limit line');
  assert.equal(ui.doc.querySelector('[data-act="kitNew"]').disabled, false);
  ui.click('[data-act="kitNew"]'); await ui.settle();
  main = ui.doc.querySelector('#main');
  assert.ok(!visibleText(main).includes('letters, digits, spaces') && !visibleText(main).includes('XP raises the level'));
  assert.ok(ui.doc.getElementById('kitnametip').textContent.includes('Up to 60 letters, digits, spaces and - _ . , \' ( ) + & # !'));
  assert.ok(ui.doc.getElementById('kitxptip').textContent.includes('as Give XP does'));
  assert.equal(ui.doc.querySelector('#kitName').getAttribute('aria-describedby'), 'kitnametip');
  assert.equal(ui.doc.querySelector('#kitXp').getAttribute('aria-describedby'), 'kitxptip');
  ui.dom.window.close();

  ui = await boot({ get: { '/api/kits': { kits: [KIT('One'), KIT('Two')], limits: LIMITS, problem: '' } } });
  await sub(ui, 'char:carepackage');
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('The most kits are kept (2): delete one first.'));
  assert.equal(ui.doc.querySelector('[data-act="kitNew"]').disabled, true);
  ui.dom.window.close();

  ui = await boot({ get: { '/api/kits': { kits: [], limits: LIMITS, problem: 'The settings file could not be read.' } } });
  await sub(ui, 'char:carepackage');
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('The settings file could not be read.'), 'an error stays');
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('No kits yet.'));
  ui.dom.window.close();
});
