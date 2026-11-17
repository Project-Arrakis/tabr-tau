// A container opened in a base's Inventory list (Open) stays open when the page is drawn again (#731): after an item is deleted (so several can be deleted in a row), after a
// quantity is added, and after the page's own refresh. It closes with a move to another base, sub-tab or group, and when it is no longer one of this base's containers. The
// keyboard focus goes to the row that took the place of the deleted one, and what was typed in the container's boxes stays.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const row = (id) => ({ base_id: id, name: 'Base ' + id, base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 3, placeables: 2, x: 0, y: 0, z: 0 });
const ITEMS = [{ id: 'MelangeSpice', name: 'Spice Melange', category: 'resources' }, { id: 'Literjon', name: 'Literjon', category: 'tools' }];
const box = (inventory_id, name, group, items = 3) => ({ actor_id: inventory_id, base_id: 7, building_type: 'GenericContainer_Placeable', group, inventory_id, items, max_item_count: 20, name, type: 'Chest' });
const GROUPS = [{ count: 1, id: 'storage', label: 'Storage' }, { count: 0, id: 'refining', label: 'Refining' }, { count: 0, id: 'crafting', label: 'Crafting' }, { count: 1, id: 'other', label: 'Other' }];
const stack = (id, tpl, n = 1) => ({ id, inventory_id: 1, template_id: tpl, position_index: id, quality_level: 0, stack_size: n });
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);

function setup() {
  const get = {
    '/api/bases/list': [row(7), row(8)],
    '/api/bases/inventory': { containers: [box(1, 'Chest', 'storage'), box(2, 'Crate', 'other')], groups: GROUPS },
    '/api/bases/storage/items': [stack(11, 'Literjon'), stack(12, 'MelangeSpice', 5), stack(13, 'Literjon')],
    '/api/catalog/items': ITEMS,
  };
  const post = { '/api/items/delete': { ok: true }, '/api/bases/give': { ok: true, given: 1, requested: 1 } };
  return { get, post };
}
async function openChest(ui) {
  for (const s of ['[data-tab="player"]', '[data-sub="player:bases"]', '[data-bstab="inventory"]', '[data-act="openInv"]']) { ui.click(s); await ui.settle(); }
  assert.ok(ui.doc.querySelector('#inv [data-act="delItem"]'), 'the container is open');
}
const ids = (ui) => [...ui.doc.querySelectorAll('#inv [data-act="delItem"]')].map((b) => b.dataset.id);
// Delete the item of the row whose Delete button has the focus, the way a person does: Delete, then OK in the question.
async function deleteItem(ui, get, id) {
  const btn = ui.doc.querySelector(`#inv [data-act="delItem"][data-id="${id}"]`);
  assert.ok(btn, 'a Delete button for ' + id);
  btn.focus();
  ui.click(`#inv [data-act="delItem"][data-id="${id}"]`); await ui.settle();
  get['/api/bases/storage/items'] = get['/api/bases/storage/items'].filter((r) => r.id != id); // the save no longer holds it
  ui.click('[data-act="askOk"]'); await ui.settle();
}

test('deleting an item leaves the container open with the item gone, so several can be deleted in a row', async () => {
  const { get, post } = setup();
  const ui = await boot({ get, post });
  await openChest(ui);
  assert.deepEqual(ids(ui), ['11', '12', '13']);
  await deleteItem(ui, get, 12);
  assert.equal(posts(ui, '/api/items/delete').length, 1);
  assert.deepEqual(ids(ui), ['11', '13'], 'the container is still open and the item is gone');
  assert.ok(ui.doc.querySelector('#inv h3').textContent.includes('Inventory: Chest'));
  await deleteItem(ui, get, 11);
  assert.deepEqual(ids(ui), ['13'], 'a second delete in a row');
  assert.equal(posts(ui, '/api/items/delete').length, 2);
  ui.dom.window.close();
});

test('after a delete the focus goes to the Delete button of the row that took its place, then to the last one, then to the Add item box', async () => {
  const { get, post } = setup();
  const ui = await boot({ get, post });
  await openChest(ui);
  await deleteItem(ui, get, 11);
  assert.equal(ui.doc.activeElement, ui.doc.querySelector('#inv [data-act="delItem"][data-id="12"]'), 'the next row');
  await deleteItem(ui, get, 13);
  assert.equal(ui.doc.activeElement, ui.doc.querySelector('#inv [data-act="delItem"][data-id="12"]'), 'the last row when the deleted one was last');
  await deleteItem(ui, get, 12);
  assert.deepEqual(ids(ui), []);
  assert.ok(ui.doc.querySelector('#inv').textContent.includes('This container is empty.'));
  assert.equal(ui.doc.activeElement, ui.doc.querySelector('#inv #bt'), 'no row left: the box that adds an item');
  ui.dom.window.close();
});

test('a refused delete changes nothing: the container stays open and the focus stays on its button', async () => {
  const { get, post } = setup();
  post['/api/items/delete'] = { __fail: 'the save is busy', __status: 409 };
  const ui = await boot({ get, post });
  await openChest(ui);
  const btn = ui.doc.querySelector('#inv [data-act="delItem"][data-id="12"]');
  btn.focus();
  ui.click('#inv [data-act="delItem"][data-id="12"]'); await ui.settle();
  ui.click('[data-act="askOk"]'); await ui.settle();
  assert.deepEqual(ids(ui), ['11', '12', '13']);
  assert.equal(ui.doc.activeElement, ui.doc.querySelector('#inv [data-act="delItem"][data-id="12"]'));
  ui.dom.window.close();
});

test('text typed in the open container\'s boxes and its table offset stay through a delete', async () => {
  const { get, post } = setup();
  const ui = await boot({ get, post });
  await openChest(ui);
  const q = ui.doc.querySelector('#bq');
  q.value = '7';
  const pick = ui.doc.querySelector('#bt');
  pick.value = 'Spice';
  const t = ui.doc.querySelector('#inv .scroll');
  assert.ok(t, 'the items table');
  t.scrollTop = 40;
  await deleteItem(ui, get, 11);
  assert.equal(ui.doc.querySelector('#bq').value, '7');
  assert.equal(ui.doc.querySelector('#bt').value, 'Spice');
  assert.equal(ui.doc.querySelector('#inv .scroll').scrollTop, 40);
  ui.dom.window.close();
});

test('adding an item keeps the container open, and so does a redraw from the file (the page refreshing itself)', async () => {
  const { get, post } = setup();
  const ui = await boot({ get, post });
  await openChest(ui);
  ui.doc.querySelector('#bt').value = 'Literjon';
  ui.click('[data-act="giveInv"]'); await ui.settle();
  assert.equal(posts(ui, '/api/bases/give').length, 1);
  assert.deepEqual(ids(ui), ['11', '12', '13']);
  get['/api/bases/storage/items'] = [stack(11, 'Literjon'), stack(13, 'Literjon')]; // the save changed meanwhile
  await ui.w.render(true, true); // the page's own refresh from the file
  assert.deepEqual(ids(ui), ['11', '13'], 'still open, and drawn from the save');
  ui.dom.window.close();
});

test('moving to another sub-tab, group or base closes the container; so does a container that is not this base\'s any more', async () => {
  const { get, post } = setup();
  const ui = await boot({ get, post });
  await openChest(ui);
  ui.click('[data-bsgrp="other"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#inv [data-act="delItem"]'), null, 'another group: closed');
  assert.equal(ui.doc.querySelector('#inv').textContent.trim(), '');
  ui.click('[data-bsgrp="storage"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#inv').textContent.trim(), '', 'and it stays closed when the group is shown again');
  ui.click('[data-act="openInv"]'); await ui.settle();
  assert.ok(ids(ui).length);
  ui.click('[data-bstab="power"]'); await ui.settle();
  ui.click('[data-bstab="inventory"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#inv').textContent.trim(), '', 'another sub-tab and back: closed');
  ui.click('[data-act="openInv"]'); await ui.settle();
  ui.click('[data-bsopen="8"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#inv').textContent.trim(), '', 'another base: closed');
  ui.dom.window.close();
});

test('a container that the save no longer lists under this base is not drawn again after the redraw', async () => {
  const { get, post } = setup();
  const ui = await boot({ get, post });
  await openChest(ui);
  get['/api/bases/inventory'] = { containers: [box(2, 'Crate', 'other')], groups: GROUPS };
  await deleteItem(ui, get, 11);
  assert.equal(ui.doc.querySelector('#inv').textContent.trim(), '', 'closed, not an error');
  assert.equal(ui.doc.querySelector('#main .bad'), null, 'no error card');
  ui.dom.window.close();
});

test('another page and back starts with the container closed', async () => {
  const { get, post } = setup();
  const ui = await boot({ get, post });
  await openChest(ui);
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#inv').textContent.trim(), '', 'a move to another page: closed');
  ui.dom.window.close();
});

test('a redraw never shows the open container empty first: it is in the first drawing, and its Delete buttons say which item they delete', async () => {
  const { get, post } = setup();
  const ui = await boot({ get, post });
  await openChest(ui);
  assert.equal(ui.doc.querySelector('#inv [data-act="delItem"][data-id="12"]').getAttribute('aria-label'), 'Delete Spice Melange');
  // a watcher on the page: whenever the page has its container place, the container card must be in it (the redraw puts it in with the page, it does not fill it in afterwards)
  const seen = [];
  const mo = new ui.w.MutationObserver(() => { const n = ui.doc.querySelector('#inv'); if (n) seen.push(n.textContent.includes('Inventory: Chest')); });
  mo.observe(ui.doc.querySelector('#main'), { childList: true, subtree: true });
  await ui.w.render(true, true);
  await new Promise((r) => ui.w.setTimeout(r, 0));
  mo.disconnect();
  assert.ok(seen.length, 'the page was drawn');
  assert.ok(seen.every(Boolean), 'the container place was never drawn empty');
  ui.dom.window.close();
});
