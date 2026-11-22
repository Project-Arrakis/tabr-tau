// #1136: Give item > Augments. The heading row is "Augments" with the one grade of the added augments on the same row; the slot lists sit under it, one per line, labelled Slot 1 and Slot 2.
// The same block is drawn in the Give item row, in the care package editor and in the item's Augments card, so the three share one shape. Layout only: ONE grade for all the augments
// stays (the request body is unchanged, and ui.hostile.test.mjs still posts the same body). jsdom lays nothing out, so the order and the nesting are read from
// the page and the widths from the style sheet.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const CSS = fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', 'app.css'), 'utf8');
const FIX = JSON.parse(fs.readFileSync(path.join(process.env.UI_FIXTURES_DIR || path.join(here, 'fixtures'), 'ui-fixtures.json'), 'utf8'));
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const fit = { template_id: 'Helm_1', kind: 'clothing', limit: 2, options: [{ id: 'Aug_A', name: 'Plate', effects: ['Armor +5%'] }, { id: 'Aug_B', name: 'Weave', effects: [] }], applied: [{ id: 'Aug_B', name: 'Weave', quality: '3' }] };
async function inventory() {
  const inv = JSON.parse(JSON.stringify(FIX['/api/player/inventory'].body));
  inv.items = [{ id: 7, inventory_id: 1, inventory_name: 'Backpack', position_index: 0, template_id: 'Helm_1', stack_size: 1, quality_level: 5, durability: null, max_durability: null, augments: [{ name: 'Aug_B', quality: 3 }], aug_limit: 2 }];
  inv.templates = ['Helm_1'];
  const ui = await boot({ get: { '/api/player/inventory': inv }, post: { '/api/items/augment-options': fit, '/api/items/augment': { ok: true, augments: 1, slotsUnlocked: 2 }, '/api/player/give-items': { ok: true, count: 1, itemIds: [9] } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  return ui;
}
// the shape every place must have: a heading row (the word and the one grade), then a list of slots under it with one label per slot
function checkBlock(box, { prefix, grade, slots }) {
  const block = box.querySelector('.augbox');
  assert.ok(block, 'one block for the augments');
  const [head, list] = [...block.children];
  assert.ok(head.classList.contains('aug-head') && list.classList.contains('aug-slots'), 'the heading row first, the slot list under it');
  assert.equal(text(head.querySelector('b')), 'Augments', 'the heading word');
  assert.deepEqual([...head.querySelectorAll('select')].map((s) => s.id), [grade], 'the one grade is on the heading row, and it is the only grade');
  assert.equal(list.querySelector('#' + grade), null, 'no grade among the slots: it does not belong to one slot');
  const labels = [...list.children];
  assert.equal(labels.length, slots, 'one line for each slot');
  labels.forEach((l, i) => {
    assert.equal(l.tagName, 'LABEL');
    assert.match(text(l), new RegExp('^Slot ' + (i + 1) + ' '), 'labelled Slot ' + (i + 1));
    assert.equal(l.querySelectorAll('select').length, 1);
    assert.equal(l.querySelector('select').id, prefix + i);
  });
  assert.equal(box.querySelectorAll('select').length, slots + 1, 'the slots and the one grade: nothing else');
  assert.ok(head.compareDocumentPosition(list) & 4, 'the heading row comes before the slots');
  return block;
}

test('#1136: Give item: an Augments heading row with the one grade, the two slots one under the other', async () => {
  const ui = await inventory();
  ui.doc.querySelector('#gt').value = 'Helm_1';
  ui.doc.querySelector('#gt').dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  const box = ui.doc.querySelector('#gaug');
  checkBlock(box, { prefix: 'ga', grade: 'gag', slots: 2 });
  assert.match(text(box.querySelector('#gag').closest('label')), /^Grade of added augments 1\s?2\s?3\s?4\s?5$/, 'the grade keeps its words and its choices');
  assert.equal(box.querySelector('#gag').value, '5', 'and its default');
});

test('#1136: the Augments card of an item has the same block, and the grade, the unlock box, Apply and Cancel still work', async () => {
  const ui = await inventory();
  ui.click('[data-act="augEdit"]'); await ui.settle();
  const card = ui.doc.querySelector('#au0').closest('.card');
  checkBlock(card, { prefix: 'au', grade: 'augG', slots: 2 }); // the card also holds the apply row (no select there)
  assert.ok(card.querySelector('#augU'), 'the unlock box stays');
  assert.equal(card.querySelector('#au0').value, 'Aug_B', 'the applied augment is preselected');
  ui.doc.querySelector('#au1').value = 'Aug_A'; ui.doc.querySelector('#augG').value = '4';
  ui.click('[data-act="augApply"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/items/augment').map((p) => p.body), [{ item_id: 7, augments: ['Aug_B', 'Aug_A'], grade: 4, unlock_slots: true }], 'one grade is sent for all the augments, as before');
});

test('#1136: the style sheet stacks the slots, gives every slot the same width and keeps the heading row on one line', () => {
  const rule = (sel) => { const m = CSS.split(/\r?\n/).filter((l) => l.includes(sel)); assert.ok(m.length, 'no rule for ' + sel); return m.join(' '); };
  assert.match(rule('.aug-slots{'), /\.aug-slots\{[^}]*display:grid/, 'one slot per line');
  assert.match(rule('.augbox .aug-slots label{'), /grid-template-columns:\s*[\d.]+rem minmax\(0,1fr\)/, 'the same label column, so the lists start at the same place');
  assert.match(rule('.augbox .aug-slots select{'), /width:100%/, 'and are as wide as each other');
  assert.match(rule('.aug-head{'), /\.aug-head\{[^}]*display:flex/, 'the word and the grade on one row');
});
