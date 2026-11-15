// Look-and-feel polish: the count lines of the Building sets and Cosmetics lists, the save name in the header, and the Database pager.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

test.afterEach(() => closeAll());

const open = async (ui, tab, sub) => { ui.click(`[data-tab="${tab}"]`); await ui.settle(); if (sub) { ui.click(`[data-sub="${tab}:${sub}"]`); await ui.settle(); } };
const row = (o) => ({ id: 'X', name: 'X', group: 'Atreides', groupId: 'atreides', learned: false, inInventory: false, inCatalog: true, ...o });
const count = (ui) => ui.doc.querySelector('#cthead .row > .mut').textContent;

test('Building sets: the count line ends with how many are learned, like the Crafting line', async () => {
  const sets = { rows: [row({ id: 'A', name: 'A', learned: true }), row({ id: 'B', name: 'B', inInventory: true }), row({ id: 'C', name: 'C' }), row({ id: 'D', name: 'D', learned: true })], newPieces: [] };
  const ui = await boot({ get: { '/api/player/building-sets': sets } });
  await open(ui, 'player', 'buildingsets');
  assert.equal(count(ui), '4 items; 2 of 4 learned (1 more only in an inventory).');
  ui.click('[data-ctf="missing"]'); await ui.settle();
  assert.equal(count(ui), '1 of 4 items match; 2 of 4 learned (1 more only in an inventory).');
});

test('Building sets: a set held only in an inventory is not counted as learned, and a list with none says nothing extra', async () => {
  const rows = [row({ id: 'A', name: 'A', learned: true }), row({ id: 'B', name: 'B', inInventory: true }), row({ id: 'C', name: 'C', inInventory: true })];
  let ui = await boot({ get: { '/api/player/building-sets': { rows, newPieces: [] } } });
  await open(ui, 'player', 'buildingsets');
  assert.equal(count(ui), '3 items; 1 of 3 learned (2 more only in an inventory).');
  assert.equal(ui.doc.querySelector('[data-ctf="have"]').textContent, 'Learned or in inventory (3)');
  ui.dom.window.close();
  ui = await boot({ get: { '/api/player/building-sets': { rows: [row({ id: 'A', name: 'A', learned: true }), row({ id: 'C', name: 'C' })], newPieces: [] } } });
  await open(ui, 'player', 'buildingsets');
  assert.equal(count(ui), '2 items; 1 of 2 learned.');
});

test('Building sets: a long list keeps the "first N shown" part before the learned count', async () => {
  const rows = Array.from({ length: 450 }, (_, i) => row({ id: 'S' + i, name: 'Set ' + i, learned: i < 10 }));
  const ui = await boot({ get: { '/api/player/building-sets': { rows, newPieces: [] } } });
  await open(ui, 'player', 'buildingsets');
  assert.equal(count(ui), '450 items, the first 400 shown; 10 of 450 learned.');
});

test('Cosmetics: the count line says unlocked or in inventory with a library, and only in inventory without one', async () => {
  const rows = [row({ id: 'a', name: 'a', unlocked: true }), row({ id: 'b', name: 'b', inInventory: true }), row({ id: 'c', name: 'c' })];
  const groups = [{ id: 'atreides', name: 'Atreides', count: 3, requirement: '' }];
  let ui = await boot({ get: { '/api/player/customizations': { libraryAvailable: true, libraryCount: 1, rows, groups } } });
  await open(ui, 'player', 'customizations');
  assert.equal(count(ui), '3 items; 2 of 3 unlocked or in inventory.');
  ui.dom.window.close();
  ui = await boot({ get: { '/api/player/customizations': { libraryAvailable: false, rows: rows.map((r) => ({ ...r, unlocked: false })), groups } } });
  await open(ui, 'player', 'customizations');
  assert.equal(count(ui), '3 items; 1 of 3 in inventory.');
});

test('Header: the save file name is a plain label, not a button or any other control, and keeps the full path as its tooltip', async () => {
  const ui = await boot();
  const label = ui.doc.querySelector('#savename');
  const name = ui.doc.querySelector('#path');
  assert.ok(label && name && label.contains(name));
  assert.equal(label.querySelector('.cap').textContent, 'Save:');
  assert.equal(name.tagName, 'SPAN');
  for (const el of [label, name, label.querySelector('.cap')]) {
    assert.ok(!['BUTTON', 'A', 'INPUT', 'SELECT'].includes(el.tagName), el.tagName);
    assert.equal(el.getAttribute('role'), null);
    assert.equal(el.getAttribute('tabindex'), null);
    assert.equal(el.getAttribute('onclick'), null);
  }
  assert.equal(label.querySelectorAll('button, a, input, [tabindex], [role]').length, 0, 'nothing in the label can be pressed or focused');
  assert.ok(!ui.doc.querySelector('header').querySelector('#path').closest('button'));
  assert.equal(name.textContent, 'game.db');
  assert.ok(name.title.length > name.textContent.length && name.title.endsWith('game.db'), 'the tooltip is the full path: ' + name.title);
  assert.equal(label.hidden, false);
});

const table = (total, nrows = 2) => ({ columns: ['_rowid_', 'id'], locked: null, rows: Array.from({ length: nrows }, (_, i) => [i + 1, i + 1]), total });

test('Database: a table of one row has no Previous or Next', async () => {
  const ui = await boot({ get: { '/api/db/table': table(1, 1) } });
  await open(ui, 'db');
  assert.equal(ui.doc.querySelector('[data-act="dbprev"]'), null);
  assert.equal(ui.doc.querySelector('[data-act="dbnext"]'), null);
  assert.ok(ui.doc.querySelector('#dbout .mut').textContent.startsWith('1 row, showing 1–1'), ui.doc.querySelector('#dbout .mut').textContent);
});

test('Database: 100 rows fit one page and have no pager; 101 rows have one, reachable by keyboard', async () => {
  let ui = await boot({ get: { '/api/db/table': table(100) } });
  await open(ui, 'db');
  assert.equal(ui.doc.querySelector('[data-act="dbnext"]'), null);
  ui.dom.window.close();
  ui = await boot({ get: { '/api/db/table': table(101) } });
  await open(ui, 'db');
  const prev = ui.doc.querySelector('[data-act="dbprev"]'), next = ui.doc.querySelector('[data-act="dbnext"]');
  assert.ok(prev && next);
  assert.equal(prev.tagName, 'BUTTON'); assert.equal(next.tagName, 'BUTTON');
  assert.ok(prev.disabled, 'first page: Previous is off');
  assert.ok(!next.disabled && next.tabIndex >= 0, 'Next can be reached with the keyboard');
  assert.ok(ui.doc.querySelector('#dbout .mut').textContent.startsWith('101 rows, showing 1–100'));
});
