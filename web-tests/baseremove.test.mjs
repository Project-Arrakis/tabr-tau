// Back up and remove a base on Player > Bases (#1127): a button on every standing base, a question that says what happens (and that the contents are not moved), and a write that
// carries the fingerprint of what was shown.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

test.afterEach(() => closeAll());

const HOME = { base_id: 7, name: 'Home', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 73, placeables: 2, x: 0, y: 0, z: 0 };
const OLD = { ...HOME, base_id: 8, name: 'Old', picked_up: true };
const PREVIEW = { ok: true, dryRun: true, name: 'Home', pieces: 73, placeables: 1, leftStanding: 1, storedItems: 12, fingerprint: 'abc123', backupId: 4, warnings: ['1 placeables of this base have no buildable support. The game\'s pick-up does not take them, so they stay as they are.'] };
const openBases = async (ui) => {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  return ui.doc.querySelector('#main');
};
const boots = (post = {}) => boot({ get: { '/api/bases/list': [HOME, OLD], '/api/bases/backups': [], '/api/bases/remove-preview': PREVIEW }, post });

test('every standing base has Back up and remove, a base the game picked up does not, and the sentence is a tooltip', async () => {
  const ui = await boots();
  const main = await openBases(ui);
  const buttons = [...main.querySelectorAll('[data-act="baseRemove"]')];
  assert.equal(buttons.length, 1);
  assert.equal(buttons[0].textContent.trim(), 'Back up and remove');
  assert.deepEqual([buttons[0].dataset.id, buttons[0].dataset.name], ['7', 'Home']);
  const tipBox = main.querySelector('#' + buttons[0].getAttribute('aria-describedby'));
  assert.ok(tipBox && tipBox.getAttribute('role') === 'tooltip' && tipBox.textContent.includes('base backup you can place again') && tipBox.textContent.includes('stay with the backup'));
  assert.equal(ui.posted.length, 0, 'drawing the list writes nothing');
  ui.dom.window.close();
});

test('the question names the base, says what becomes of it and of its contents, and nothing is written on Cancel', async () => {
  const ui = await boots();
  await openBases(ui);
  ui.click('[data-act="baseRemove"]'); await ui.settle();
  const q = ui.doc.querySelector('#ask');
  assert.ok(q.textContent.includes('Back up and remove "Home"?'), q.textContent);
  const items = [...q.querySelectorAll('li')].map((l) => l.textContent);
  assert.ok(items[0].includes('73 building pieces') && items[0].includes('1 placeables'), items[0]);
  assert.ok(items.some((l) => l.includes('base backup that you can place again')), items.join('|'));
  assert.ok(items.some((l) => l.includes('Items stored in the base (12) are not moved') && l.includes('stay where they are, with the backup')), items.join('|'));
  assert.ok(items.some((l) => l.includes('no buildable support')), 'the placeables that stay are said');
  assert.ok(q.textContent.includes('written at once, after a backup'));
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(ui.posted.filter((x) => x.path === '/api/bases/remove').length, 0, 'Cancel writes nothing');
  ui.dom.window.close();
});

test('OK writes once, with the base and the fingerprint the question showed, and the toast says what happened', async () => {
  const ui = await boots({ '/api/bases/remove': { ...PREVIEW, dryRun: false, outcome: 'written' } });
  await openBases(ui);
  ui.click('[data-act="baseRemove"]'); await ui.settle();
  await ui.wait(450); // the gap the question keeps against a double click
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((x) => x.path === '/api/bases/remove').map((x) => x.body), [{ base_id: 7, fingerprint: 'abc123' }]);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Backed up "Home" as base backup 4') && t.includes('73 pieces') && t.includes('Base Reconstruction Tool'), t);
  ui.dom.window.close();
});

test('a refusal of the preview is the message of the page and no question opens', async () => {
  const ui = await boot({ get: { '/api/bases/list': [HOME], '/api/bases/backups': [], '/api/bases/remove-preview': { __fail: 'This base belongs to another player (or is shared), so it was not changed.', __code: 'conflict', __status: 409 } } });
  await openBases(ui);
  ui.click('[data-act="baseRemove"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('belongs to another player'));
  assert.ok(!ui.doc.querySelector('#ask').textContent.includes('Back up and remove'), 'no question');
  assert.equal(ui.posted.length, 0);
  ui.dom.window.close();
});
