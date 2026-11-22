// Give item picker (#1126): an item and its schematic are two rows that never look alike, and the kind is named again in the queued line and in the result toast.
// The rows are built from the catalog the editor serves; the schematic of an item is a second entry of the same name in the category "schematics".
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const CATALOG = [
  { id: 'Combat_Light_Helmet_05', name: 'Dyvetz Helm', category: 'clothing' }, // has both forms
  { id: 'Combat_Light_Helmet_05_Schematic', name: 'Dyvetz Helm', category: 'schematics' },
  { id: 'IronBar', name: 'Iron Bar', category: 'resources' }, // one form: the item
  { id: 'Flamethrower1Schematic', name: 'Scorchbolt', category: 'schematics' }, // one form: only the schematic
  { id: 'Twin_A', name: 'Twin', category: 'clothing' }, // two items of one name, and a schematic of that name
  { id: 'Twin_B', name: 'Twin', category: 'clothing' },
  { id: 'Twin_A_Schematic', name: 'Twin', category: 'schematics' },
];
const INV = { items: [], inventories: [{ id: 1, inventory_type: 0, max_item_count: 5, max_item_volume: 1000 }], templates: [] };
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);
const boot_ = (post = {}) => boot({ get: { '/api/catalog/items': CATALOG, '/api/player/inventory': INV }, post });
async function openGive(ui) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
}
const rows = (ui) => [...ui.doc.querySelectorAll('#gt-dl option')].map((o) => o.value);
const pick = (ui, text, qty = 1) => { ui.doc.querySelector('#gt').value = text; ui.doc.querySelector('#gq').value = String(qty); };
const OK = { ok: true, requested: 1, given: 1, reason: '', limit: '', outcome: 'written' };

test('Give item rows: an item with both forms has a plain row and a "(schematic)" row; an item with one form keeps one row', async () => {
  const ui = await boot_();
  await openGive(ui);
  const r = rows(ui);
  assert.ok(r.includes('Dyvetz Helm') && r.includes('Dyvetz Helm (schematic)'), r.join(' | '));
  assert.ok(r.includes('Iron Bar') && !r.includes('Iron Bar (schematic)'), 'an item with one form stays as it was');
  assert.ok(r.includes('Scorchbolt (schematic)'), 'a schematic-only entry says what it is');
  assert.equal(new Set(r).size, r.length, 'no two rows are alike');
  // several of one kind with one name still carry their template id, and the schematic row says both
  assert.ok(r.includes('Twin (Twin_A)') && r.includes('Twin (Twin_B)') && r.includes('Twin (schematic)'), r.join(' | '));
  const ids = [...ui.doc.querySelectorAll('#gt-dl option')].map((o) => o.getAttribute('label'));
  assert.ok(ids.includes('Combat_Light_Helmet_05_Schematic'), 'the template id stays on each row, so an id can still be searched');
});

test('Give item rows: each row gives the entry it names, and the old bare names, template ids and typed schematic names still resolve', async () => {
  const ui = await boot_({ '/api/player/give': OK });
  await openGive(ui);
  const given = async (text) => { pick(ui, text); ui.click('[data-act="give"]'); await ui.settle(); return posts(ui, '/api/player/give').pop().body.template_id; };
  assert.equal(await given('Dyvetz Helm'), 'Combat_Light_Helmet_05');
  assert.equal(await given('Dyvetz Helm (schematic)'), 'Combat_Light_Helmet_05_Schematic');
  assert.equal(await given('dyvetz helm (SCHEMATIC)'), 'Combat_Light_Helmet_05_Schematic');
  assert.equal(await given('Combat_Light_Helmet_05_Schematic'), 'Combat_Light_Helmet_05_Schematic');
  assert.equal(await given('Scorchbolt (schematic)'), 'Flamethrower1Schematic');
  assert.equal(await given('Scorchbolt'), 'Flamethrower1Schematic', 'a name only a schematic has is still found by that name');
  assert.equal(await given('Iron Bar'), 'IronBar');
});

test('Give item queue: the queued line names the kind of an item with both forms and of every schematic, and leaves other items alone', async () => {
  const ui = await boot_();
  await openGive(ui);
  for (const t of ['Dyvetz Helm', 'Dyvetz Helm (schematic)', 'Iron Bar', 'Scorchbolt (schematic)']) { pick(ui, t, 2); ui.click('[data-act="queueAdd"]'); await ui.settle(); }
  const tags = [...ui.doc.querySelectorAll('span.tag')].map((t) => t.textContent.replace(/\s+/g, ' ').replace(/ x$/, '').trim());
  assert.deepEqual(tags, ['2 x Dyvetz Helm (item)', '2 x Dyvetz Helm (schematic)', '2 x Iron Bar', '2 x Scorchbolt (schematic)']);
});

test('Give item result: the toast says item or schematic, a partial give names the kind too, a queue says how many schematics it held', async () => {
  let ui = await boot_({ '/api/player/give': OK });
  await openGive(ui);
  pick(ui, 'Dyvetz Helm (schematic)'); ui.click('[data-act="give"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Schematic added'), ui.doc.querySelector('#toast').textContent);
  ui.dom.window.close();

  ui = await boot_({ '/api/player/give': OK });
  await openGive(ui);
  pick(ui, 'Dyvetz Helm'); ui.click('[data-act="give"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Item added'));
  assert.ok(!ui.doc.querySelector('#toast').textContent.includes('Schematic'));
  ui.dom.window.close();

  ui = await boot_({ '/api/player/give': { ok: true, requested: 3, given: 1, reason: 'the backpack is full', limit: 'slots', outcome: 'written' } });
  await openGive(ui);
  pick(ui, 'Dyvetz Helm (schematic)', 3); ui.click('[data-act="give"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast .part').textContent.includes('Gave 1 of 3 Dyvetz Helm (schematic): the backpack is full'), ui.doc.querySelector('#toast .part').textContent);
  ui.dom.window.close();

  ui = await boot_({ '/api/player/give-items': { ok: true, count: 3, requested: 3, given: 3, reason: '', limit: '', outcome: 'written' } });
  await openGive(ui);
  for (const t of ['Dyvetz Helm', 'Dyvetz Helm (schematic)', 'Iron Bar']) { pick(ui, t); ui.click('[data-act="queueAdd"]'); await ui.settle(); }
  ui.click('[data-act="queueGive"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Added 3 items (1 schematic)'), ui.doc.querySelector('#toast').textContent);
  assert.deepEqual(posts(ui, '/api/player/give-items')[0].body.items.map((i) => i.template_id), ['Combat_Light_Helmet_05', 'Combat_Light_Helmet_05_Schematic', 'IronBar'], 'what is sent is unchanged');
});

test('Give item picker: the box is the wide one, and the style lets it grow to a whole name yet never past its row', async () => {
  const ui = await boot_();
  await openGive(ui);
  assert.ok(ui.doc.querySelector('#gt').classList.contains('pick-wide'));
  const css = fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', 'app.css'), 'utf8');
  const m = css.match(/input\.pick-wide\{([^}]*)\}/);
  assert.ok(m, 'no rule for the wide picker');
  assert.match(m[1], /width:\s*72ch/);
  assert.match(m[1], /max-width:\s*100%/, 'a phone is narrower than 72 characters: the box must not push the page wide');
});

test('Real catalog: every row is unique, schematics all say so, no item row says so, and every item with a schematic of its name gets two rows', async () => {
  const items = JSON.parse(fs.readFileSync(path.join(here, '..', 'internal', 'catalog', 'items.json'), 'utf8'));
  const ui = await boot({ get: { '/api/catalog/items': items, '/api/player/inventory': INV } });
  await openGive(ui);
  const opts = [...ui.doc.querySelectorAll('#gt-dl option')];
  const r = opts.map((o) => o.value);
  assert.equal(r.length, items.length);
  assert.equal(new Set(r.map((v) => v.toLowerCase())).size, r.length, 'two rows with the same label');
  const byId = new Map(items.map((i) => [i.id, i]));
  for (const o of opts) {
    const it = byId.get(o.getAttribute('label'));
    const sch = it.category === 'schematics';
    assert.equal(/\(schematic[,)]/.test(o.value), sch, `${o.value} (${it.id})`);
  }
  const both = new Set(items.filter((i) => i.category === 'schematics').map((i) => i.name).filter((n) => items.some((i) => i.name === n && i.category !== 'schematics')));
  assert.ok(both.size > 400, 'the catalog has hundreds of items with both forms');
  for (const n of both) assert.ok(r.includes(n + ' (schematic)') || r.some((v) => v.startsWith(n + ' (schematic, ')), n);
});
