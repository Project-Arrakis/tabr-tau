// Recycle base on Player > Bases (#1176): a button on every standing base next to the others, a question that lists the numbers, says everything goes to the Reserve and that it
// cannot be undone except by restoring a backup of the save, and a write that carries the fingerprint of what was shown. The Solido option (#1187): a box on the Bases card, a
// question that says a Solido is made first and in which device, and a write that carries the same choice.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

test.afterEach(() => closeAll());

const HOME = { base_id: 7, name: 'Home', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 2141, placeables: 253, x: 0, y: 0, z: 0 };
const OLD = { ...HOME, base_id: 8, name: 'Old', picked_up: true };
const PREVIEW = { ok: true, dryRun: true, base: 7, name: 'Home', map: 'HaggaBasin', costPercent: 100, pieces: 2141, placeables: 253, holograms: 1, devStorage: 11, itemStacksMoved: 3492, itemsMoved: 2319976,
  refund: [{ item: 'IronBar', name: 'Iron Ingot', count: 150, stacks: 1 }, { item: 'Stone', name: 'Granite Stone', count: 24632, stacks: 50 }], refundStacks: 51, waterDropped: 9401157, waterDevices: 117,
  destination: 'Reserve', fingerprint: 'abc123', warnings: ['No map marker of this base was found, so none was removed.'], summary: [] };
const openBases = async (ui) => {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  return ui.doc.querySelector('#main');
};
const boots = (post = {}, preview = PREVIEW) => boot({ get: { '/api/bases/list': [HOME, OLD], '/api/bases/backups': [], '/api/bases/recycle-preview': preview }, post });

test('every standing base has Recycle base next to the other actions, a base the game picked up does not, and the existing buttons are as they were', async () => {
  const ui = await boots();
  const main = await openBases(ui);
  const buttons = [...main.querySelectorAll('[data-act="baseRecycle"]')];
  assert.equal(buttons.length, 1);
  assert.equal(buttons[0].textContent.trim(), 'Recycle base');
  assert.deepEqual([buttons[0].dataset.id, buttons[0].dataset.name], ['7', 'Home']);
  const tipBox = main.querySelector('#' + buttons[0].getAttribute('aria-describedby'));
  assert.ok(tipBox && tipBox.getAttribute('role') === 'tooltip' && tipBox.textContent.includes('Reserve') && tipBox.textContent.includes('cannot be undone') && tipBox.textContent.includes('No base backup is made'));
  const cell = buttons[0].closest('td');
  assert.deepEqual([...cell.querySelectorAll('button')].map((b) => b.textContent.trim()).slice(-3), ['Export for console', 'Back up and remove', 'Recycle base']);
  assert.equal(main.querySelectorAll('[data-act="baseRemove"]').length, 1);
  assert.equal(main.querySelectorAll('[data-act="baseConsoleExport"]').length, 1);
  assert.equal(ui.posted.length, 0, 'drawing the list writes nothing');
  ui.dom.window.close();
});

test('the question lists the numbers, says everything goes to the Reserve and that only a save backup undoes it, and Cancel writes nothing', async () => {
  const ui = await boots();
  await openBases(ui);
  ui.click('[data-act="baseRecycle"]'); await ui.settle();
  const q = ui.doc.querySelector('#ask');
  assert.ok(q.textContent.includes('Recycle "Home"?'), q.textContent);
  const items = [...q.querySelectorAll('li')].map((l) => l.textContent);
  assert.ok(items[0].includes('2,141 building pieces') && items[0].includes('253 placeables') && items[0].includes('No base backup is made'), items[0]);
  assert.ok(items[1].includes('3,492 item stacks') && items[1].includes('2,319,976 items') && items[1].includes('move to your Reserve'), items[1]);
  assert.ok(items[2].includes('100% of the building cost') && items[2].includes('51 new stacks') && items[2].includes('150 Iron Ingot') && items[2].includes('24,632 Granite Stone'), items[2]);
  assert.ok(!items.some((l) => /respawn/.test(l)), 'no respawn line for a base that is no respawn point');
  assert.ok(items.some((l) => l.includes('1 holograms give nothing back')), items.join('|'));
  assert.ok(items.some((l) => l.includes('11 Developer storage containers give 10 Iron Ingot each')), items.join('|'));
  assert.ok(items.some((l) => l.includes('9,401,157 water') && l.includes('117 devices') && l.includes('dropped')), items.join('|'));
  assert.ok(items.some((l) => l === 'Vehicles are not touched'), items.join('|'));
  assert.ok(items.some((l) => l.includes('No map marker of this base')), 'the warnings are said');
  assert.ok(q.textContent.includes("goes to your character's Reserve"));
  assert.ok(q.textContent.includes('cannot be undone, except by restoring a backup of the save'));
  assert.ok(q.textContent.includes('A backup of the save is taken first'));
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(ui.posted.filter((x) => x.path === '/api/bases/recycle').length, 0, 'Cancel writes nothing');
  ui.dom.window.close();
});

test('a Deep Desert base says the refund is half the table cost, and lines with nothing to say are left out', async () => {
  const ui = await boots({}, { ...PREVIEW, map: 'DeepDesert', costPercent: 50, holograms: 0, devStorage: 0, waterDropped: 0, waterDevices: 0, warnings: [] });
  await openBases(ui);
  ui.click('[data-act="baseRecycle"]'); await ui.settle();
  const items = [...ui.doc.querySelectorAll('#ask li')].map((l) => l.textContent);
  assert.ok(items[2].includes('which is 50% of the table cost'), items[2]);
  assert.equal(items.length, 4, items.join('|'));
  assert.ok(!items.some((l) => /hologram|Developer|water/.test(l)), items.join('|'));
  ui.dom.window.close();
});

test('while the save cannot be written the question says the recycle is queued, that it is dropped when the game changes the base, and to close the game', async () => {
  const state = JSON.parse(fs.readFileSync(process.env.UI_FIXTURES_FILE || path.join(process.env.UI_FIXTURES_DIR || path.join(path.dirname(fileURLToPath(import.meta.url)), 'fixtures'), 'ui-fixtures.json'), 'utf8'))['/api/save/state'].body;
  const ui = await boot({ get: { '/api/bases/list': [HOME], '/api/bases/backups': [], '/api/bases/recycle-preview': PREVIEW, '/api/save/state': { ...state, saveBlocked: true, blockReason: 'The game is running.' } } });
  await openBases(ui);
  ui.click('[data-act="baseRecycle"]'); await ui.settle();
  const q = ui.doc.querySelector('#ask').textContent;
  assert.ok(q.includes('the edit is queued') && q.includes('the queued recycle is dropped and nothing is recycled') && q.includes('Close the game first'), q);
  assert.ok(!q.includes('written at once'), q);
  ui.dom.window.close();
});

test('a preview with no refund list at all still asks, and says the refund is nothing', async () => {
  const ui = await boots({}, { ...PREVIEW, refund: null, refundStacks: 0 });
  await openBases(ui);
  ui.click('[data-act="baseRecycle"]'); await ui.settle();
  const items = [...ui.doc.querySelectorAll('#ask li')].map((l) => l.textContent);
  assert.ok(items[2].includes('0 new stacks') && items[2].endsWith(': nothing'), items[2]);
  ui.dom.window.close();
});

test('a base that is a respawn point: the question says the respawn point is deleted with it', async () => {
  const ui = await boots({}, { ...PREVIEW, respawnPoints: 1 });
  await openBases(ui);
  ui.click('[data-act="baseRecycle"]'); await ui.settle();
  const items = [...ui.doc.querySelectorAll('#ask li')].map((l) => l.textContent);
  assert.ok(items.some((l) => l === '1 respawn point at this base is deleted with it: you can no longer respawn there'), items.join('|'));
  ui.dom.window.close();
});

test('OK writes once, with the base and the fingerprint the question showed, and the toast says what happened', async () => {
  const ui = await boots({ '/api/bases/recycle': { ...PREVIEW, dryRun: false, outcome: 'written' } });
  await openBases(ui);
  ui.click('[data-act="baseRecycle"]'); await ui.settle();
  await ui.wait(450); // the gap the question keeps against a double click
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((x) => x.path === '/api/bases/recycle').map((x) => x.body), [{ base_id: 7, fingerprint: 'abc123' }]);
  assert.equal(ui.posted.filter((x) => x.path === '/api/bases/remove').length, 0);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Recycled "Home"') && t.includes('2,141 pieces') && t.includes('3,492 item stacks') && t.includes('51 refund stacks') && t.includes('Reserve'), t);
  ui.dom.window.close();
});

test('a refusal of the preview is the message of the page and no question opens', async () => {
  const ui = await boot({ get: { '/api/bases/list': [HOME], '/api/bases/backups': [], '/api/bases/recycle-preview': { __fail: 'A crafting queue is running in this base (1 stations), or cannot be read.', __code: 'conflict', __status: 409 } } });
  await openBases(ui);
  ui.click('[data-act="baseRecycle"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('crafting queue is running'));
  assert.ok(!ui.doc.querySelector('#ask').textContent.includes('Recycle "Home"'), 'no question');
  assert.equal(ui.posted.length, 0);
  ui.dom.window.close();
});

const SOLIDO = { made: true, blueprint: 1, item: 61428455, itemCreated: false, pieces: 2133, placeables: 251, pentashields: 9, piecesLeftOut: 8, doorsLeftOut: 2 };
const tick = (ui, on) => { const b = ui.doc.querySelector('#rcSolido'); b.checked = on; b.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };

test('the Bases card has the Solido box, unticked, with its explanation, and a recycle without it asks for no Solido', async () => {
  const ui = await boots();
  const main = await openBases(ui);
  const box = main.querySelector('#rcSolido');
  assert.ok(box && box.type === 'checkbox' && box.checked === false);
  assert.ok(box.closest('label').textContent.includes('Make a Solido of the base first when I recycle it'));
  const tipBox = main.querySelector('#' + box.getAttribute('aria-describedby'));
  assert.ok(tipBox && tipBox.getAttribute('role') === 'tooltip' && tipBox.textContent.includes('blank Solido device') && tipBox.textContent.includes('never changed') && tipBox.textContent.includes('claim totem'), tipBox && tipBox.textContent);
  ui.click('[data-act="baseRecycle"]'); await ui.settle();
  assert.deepEqual(ui.fetchedUrls.filter((u) => u.startsWith('/api/bases/recycle-preview')), ['/api/bases/recycle-preview?base=7']);
  assert.ok(![...ui.doc.querySelectorAll('#ask li')].some((l) => /Solido/.test(l.textContent)), 'no Solido line');
  assert.equal(ui.doc.querySelector('#aok').textContent.trim(), 'Recycle base');
  ui.dom.window.close();
});

test('with the box ticked the preview is asked for a Solido, the question says it is made first and in which device, and OK writes with the same choice', async () => {
  const ui = await boots({ '/api/bases/recycle': { ...PREVIEW, dryRun: false, outcome: 'written', solido: SOLIDO } }, { ...PREVIEW, solido: SOLIDO });
  await openBases(ui);
  tick(ui, true);
  ui.click('[data-act="baseRecycle"]'); await ui.settle();
  assert.deepEqual(ui.fetchedUrls.filter((u) => u.startsWith('/api/bases/recycle-preview')), ['/api/bases/recycle-preview?base=7&solido=1']);
  const q = ui.doc.querySelector('#ask');
  const items = [...q.querySelectorAll('li')].map((l) => l.textContent);
  assert.ok(items[0].includes('A Solido of the base is made first, in the same edit') && items[0].includes('2,133 building pieces') && items[0].includes('251 placeables') && items[0].includes('9 pentashields') &&
    items[0].includes('the blank Solido device (item 61428455) that is already in your inventory') && items[0].includes('the claim totem, 2 doors, 8 pieces'), items[0]);
  assert.ok(items[1].includes('2,141 building pieces') && items[1].includes('No base backup is made'), items[1]);
  assert.ok(q.textContent.includes('A Solido of the base is made first, then the base is deleted'));
  assert.equal(ui.doc.querySelector('#aok').textContent.trim(), 'Make a Solido and recycle base');
  await ui.wait(450);
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((x) => x.path === '/api/bases/recycle').map((x) => x.body), [{ base_id: 7, fingerprint: 'abc123', solido: true }]);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Recycled "Home"') && t.includes('A Solido of it (2,133 pieces, 251 placeables) is in the blank device you already had (item 61428455)'), t);
  ui.dom.window.close();
});

test('the toast says where a new device is: the backpack, or the loadout when the backpack was full', async () => {
  for (const where of ['backpack', 'loadout']) {
    const sol = { ...SOLIDO, itemCreated: true, itemWhere: where, item: 99 };
    const ui = await boots({ '/api/bases/recycle': { ...PREVIEW, dryRun: false, outcome: 'written', solido: sol } }, { ...PREVIEW, solido: sol });
    await openBases(ui);
    tick(ui, true);
    ui.click('[data-act="baseRecycle"]'); await ui.settle();
    const items = [...ui.doc.querySelectorAll('#ask li')].map((l) => l.textContent);
    assert.ok(items[0].includes(`a new Solido device (BuildingBlueprint_CopyDevice) in your ${where}`), items[0]);
    await ui.wait(450);
    ui.click('#aok'); await ui.settle();
    const t = ui.doc.querySelector('#toast').textContent;
    assert.ok(t.includes(`is in a new device in your ${where} (item 99)`), t);
    ui.dom.window.close();
  }
});

test('a new device is named as a new device in the backpack, and a Solido refused by the preview is the message of the page', async () => {
  const ui = await boots({}, { ...PREVIEW, solido: { ...SOLIDO, itemCreated: true, doorsLeftOut: 0, piecesLeftOut: 0 } });
  await openBases(ui);
  tick(ui, true);
  ui.click('[data-act="baseRecycle"]'); await ui.settle();
  const items = [...ui.doc.querySelectorAll('#ask li')].map((l) => l.textContent);
  assert.ok(items[0].includes('a new Solido device (BuildingBlueprint_CopyDevice) in your backpack') && items[0].endsWith('the claim totem'), items[0]);
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(ui.posted.filter((x) => x.path === '/api/bases/recycle').length, 0);
  ui.dom.window.close();
  const ui2 = await boot({ get: { '/api/bases/list': [HOME], '/api/bases/backups': [], '/api/bases/recycle-preview': { __fail: 'The Solido could not be made: no free slot. Nothing was changed.', __code: 'conflict', __status: 409 } } });
  await openBases(ui2);
  tick(ui2, true);
  ui2.click('[data-act="baseRecycle"]'); await ui2.settle();
  assert.ok(ui2.doc.querySelector('#toast').textContent.includes('The Solido could not be made'));
  assert.ok(!ui2.doc.querySelector('#ask').textContent.includes('Recycle "Home"'), 'no question');
  assert.equal(ui2.posted.length, 0);
  ui2.dom.window.close();
});
