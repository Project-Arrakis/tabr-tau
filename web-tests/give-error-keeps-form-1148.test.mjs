// #1148: Give item keeps everything that was entered. After an error from Give (or Add to queue) the item, the quantity, the item's grade, the grade of the added augments and every slot
// are as they were, so the mistake can be fixed and Give pressed again. The Augments block is drawn by showGiveAugments when the picker changes, never by the card, so a redraw of the page
// (the page's own refresh while the game saves, or any action that redraws) used to give back the item with an empty block; the same for the care package editor's item row.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const FIX = JSON.parse(fs.readFileSync(path.join(process.env.UI_FIXTURES_DIR || path.join(here, 'fixtures'), 'ui-fixtures.json'), 'utf8'));
const fit = { template_id: 'Helm_1', kind: 'clothing', limit: 3, options: [{ id: 'Aug_A', name: 'Plate', effects: ['Armor +5%'] }, { id: 'Aug_B', name: 'Weave', effects: [] }], applied: [] };
const SAME = 'the same augment twice: pick each augment once';
async function inventory(post = {}) {
  const inv = JSON.parse(JSON.stringify(FIX['/api/player/inventory'].body));
  inv.items = [{ id: 7, inventory_id: 1, inventory_name: 'Backpack', position_index: 0, template_id: 'Helm_1', stack_size: 1, quality_level: 5, durability: null, max_durability: null, augments: [], aug_limit: 3 }];
  inv.templates = ['Helm_1'];
  const ui = await boot({ get: { '/api/player/inventory': inv }, post: { '/api/items/augment-options': fit, '/api/player/give-items': { __fail: SAME, __status: 400 }, ...post } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  return ui;
}
const q = (ui, sel) => ui.doc.querySelector(sel);
const set = (ui, sel, v) => { const el = q(ui, sel); assert.ok(el, sel); el.value = v; return el; };
// what the person sees in the form: the picker, the quantity, the item's grade and, when the block is there, the grade of the augments and each slot
const form = (ui) => ({ gt: q(ui, '#gt').value, gq: q(ui, '#gq').value, gg: q(ui, '#gg').value, block: !!q(ui, '#gaug .augbox'), gag: q(ui, '#gag') && q(ui, '#gag').value, slots: ['#ga0', '#ga1', '#ga2'].map((s) => (q(ui, s) ? q(ui, s).value : null)) });
async function fill(ui) {
  set(ui, '#gt', 'Helm_1');
  q(ui, '#gt').dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  set(ui, '#gq', '3'); set(ui, '#gg', '2'); set(ui, '#gag', '3'); set(ui, '#ga0', 'Aug_A'); set(ui, '#ga1', 'Aug_A');
  return { gt: 'Helm_1', gq: '3', gg: '2', block: true, gag: '3', slots: ['Aug_A', 'Aug_A', ''] };
}

test('#1148: Give with the same augment twice is refused and every value is as it was', async () => {
  const ui = await inventory();
  const want = await fill(ui);
  assert.deepEqual(form(ui), want);
  ui.click('[data-act="give"]'); await ui.settle();
  assert.ok(q(ui, '#toast').textContent.includes('the same augment twice'), q(ui, '#toast').textContent);
  assert.deepEqual(form(ui), want, 'the item, the quantity, both grades and the slots are as they were');
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/give-items').length, 1);
  // the mistake is fixed and Give is pressed again, with nothing set up again
  set(ui, '#ga1', 'Aug_B');
  ui.click('[data-act="give"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/give-items')[1].body, { items: [{ template_id: 'Helm_1', quantity: 3, quality: 2, augments: ['Aug_A', 'Aug_B'], grade: 3 }] });
});

test('#1148: a redraw after the error (the page\'s refresh while the game saves) gives the Augments block back with its slots and grades', async () => {
  const ui = await inventory();
  const want = await fill(ui);
  ui.click('[data-act="give"]'); await ui.settle();
  await ui.w.render(true, true); await ui.settle(); // what autoRefresh does after a poll that sees the game's new file
  assert.deepEqual(form(ui), want, 'the block is drawn again, and every control holds what it held');
  const posts = ui.posted.filter((p) => p.path === '/api/items/augment-options');
  assert.ok(posts.length >= 2, 'the options of the item were asked again for the redrawn block');
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/give-items').length, 1, 'the redraw gave nothing');
});

test('#1148: an item grade of 0 and an untouched block stay as they are after a redraw (nothing changes by itself)', async () => {
  const ui = await inventory();
  set(ui, '#gt', 'Helm_1');
  q(ui, '#gt').dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  const before = form(ui);
  assert.equal(before.gg, '0');
  assert.equal(before.gag, '5');
  await ui.w.render(true, true); await ui.settle();
  assert.deepEqual(form(ui), before);
});

test('#1148: a redraw with no item chosen draws no block, and Add to queue (which queues the line and starts the card afresh) is as it was', async () => {
  const ui = await inventory();
  await ui.w.render(true, true); await ui.settle();
  assert.equal(form(ui).block, false);
  await fill(ui);
  ui.click('[data-act="queueAdd"]'); await ui.settle();
  assert.match(q(ui, '#main').textContent, /Queue \(1\)\s*3 x Helm_1 \(grade 2\)/, 'the line is queued as it was entered');
  assert.deepEqual(form(ui), { gt: '', gq: '1', gg: '0', block: false, gag: null, slots: [null, null, null] }, 'a queued line starts the card afresh, as before');
});

test('#1148: Add to queue with no item says so and changes nothing that was entered', async () => {
  const ui = await inventory();
  set(ui, '#gq', '4'); set(ui, '#gg', '3');
  ui.click('[data-act="queueAdd"]'); await ui.settle();
  assert.ok(q(ui, '#toast').textContent.includes('Pick an item first'), q(ui, '#toast').textContent);
  assert.deepEqual(form(ui), { gt: '', gq: '4', gg: '3', block: false, gag: null, slots: [null, null, null] });
});

test('#1148: the care package editor\'s item row keeps its Augments block through a redraw as well', async () => {
  const LIMITS = { kits: 20, items: 50, name: 60, quantity: 1000000, xp: 100000000, skillPoints: 100000, fileBytes: 65536 };
  const ui = await boot({ get: { '/api/kits': { kits: [], limits: LIMITS, problem: '' } }, post: { '/api/items/augment-options': fit } });
  ui.click('[data-tab="player"]'); await ui.settle(); ui.click('[data-sub="char:carepackage"]'); await ui.settle();
  ui.click('[data-act="kitNew"]'); await ui.settle();
  set(ui, '#kitItem', 'Combat_Heavy_Unique_Reinforced_Helmet_06');
  q(ui, '#kitItem').dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  set(ui, '#kitGrade', '4'); set(ui, '#kag', '2'); set(ui, '#ka0', 'Aug_B'); set(ui, '#ka2', 'Aug_A');
  const row = () => ({ item: q(ui, '#kitItem').value, grade: q(ui, '#kitGrade').value, block: !!q(ui, '#kitAug .augbox'), kag: q(ui, '#kag') && q(ui, '#kag').value, slots: ['#ka0', '#ka1', '#ka2'].map((s) => (q(ui, s) ? q(ui, s).value : null)) });
  const want = { item: 'Combat_Heavy_Unique_Reinforced_Helmet_06', grade: '4', block: true, kag: '2', slots: ['Aug_B', '', 'Aug_A'] };
  assert.deepEqual(row(), want);
  await ui.w.render(true, true); await ui.settle();
  assert.deepEqual(row(), want);
});
