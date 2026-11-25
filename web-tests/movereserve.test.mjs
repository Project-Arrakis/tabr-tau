// Move the Reserve into storage on Player > Bases (#1191, #1214): a button and one line that says which boxes DD-1 ... DD-6 the editor found, a question that lists what goes where and says
// it can only be undone by restoring a backup of the save, and a write that carries the fingerprint of what was shown.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

test.afterEach(() => closeAll());

const HOME = { base_id: 7, name: 'Home', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 10, placeables: 2, x: 0, y: 0, z: 0 };
const BOXES = { found: ['DD-1', 'DD-2', 'DD-3'], ignored: [], gap: 'DD-4', max: 6, text: 'Storage boxes: DD-1 ... DD-6, used in that order (found 3: DD-1, DD-2, DD-3).' };
const PREVIEW = { ok: true, dryRun: true, changed: true, nothingToDo: false, stacks: 6, items: 2161, fingerprint: 'fp1', boxNames: ['Dump Box', 'Spice Out'], autoDetected: true, warnings: ['120 units in the Reserve have no known volume and were counted as none in the volume check.'],
  summary: ['Would move: 6 item stacks (2,161 items) from the Reserve into storage, whole and unchanged. Only the place of each stack changes.',
    'Box "Dump Box" (in "Home"): 6 stacks (2,161 items) would go in, 348 stacks are already there; afterwards there would be 646 slots free, volume 28608.2 free.',
    'Box "Spice Out" (in "Home"): 0 stacks (0 items) would go in, 421 stacks are already there; afterwards there would be 579 slots free, volume 12216.5 free.',
    'The Reserve would be empty afterwards.'] };
const boots = (post = {}, preview = PREVIEW, boxes = BOXES) => boot({ get: { '/api/bases/list': [HOME], '/api/bases/backups': [], '/api/bases/move-reserve-preview': preview, '/api/bases/move-reserve-boxes': boxes }, post });
const openBases = async (ui) => {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  return ui.doc.querySelector('#main');
};

test('the Bases card has the button and one line that says which boxes were found, with an explanation that says it is not tried in the game', async () => {
  const ui = await boots();
  const main = await openBases(ui);
  const b = main.querySelector('[data-act="reserveMove"]');
  assert.ok(b && b.textContent.trim() === 'Move the Reserve into storage');
  const tipBox = main.querySelector('#' + b.getAttribute('aria-describedby'));
  assert.ok(tipBox && tipBox.getAttribute('role') === 'tooltip' && tipBox.textContent.includes('Developer storage boxes') && tipBox.textContent.includes('Not yet tried in the game'), tipBox && tipBox.textContent);
  assert.ok(tipBox.textContent.includes('DD-1, DD-2, DD-3 ... up to DD-6') && tipBox.textContent.includes('capitals'), tipBox.textContent);
  assert.equal(main.querySelector('#mrBox1'), null, 'the two name fields are gone');
  assert.equal(main.querySelector('#mrBox2'), null);
  assert.equal(main.querySelector('#mrBoxes').textContent.trim(), BOXES.text);
  assert.equal(ui.posted.length, 0, 'drawing the card writes nothing');
  ui.dom.window.close();
});

test('the line says in plain words when no box is named DD-1, and when a box behind a gap is ignored', async () => {
  const none = { found: [], ignored: [], gap: 'DD-1', max: 6, text: 'No Developer storage box named DD-1 was found. Name a Developer storage container DD-1 in the game (capitals and the hyphen exactly), then DD-2, DD-3 ... for more room.' };
  const ui = await boots({}, PREVIEW, none);
  const main = await openBases(ui);
  assert.ok(main.querySelector('#mrBoxes').textContent.includes('No Developer storage box named DD-1'));
  ui.dom.window.close();
  const gap = { found: ['DD-1', 'DD-2'], ignored: ['DD-4'], gap: 'DD-3', max: 6, text: 'Storage boxes: DD-1 ... DD-6, used in that order (found 2: DD-1, DD-2). Ignored: DD-4 (DD-3 is missing, and the boxes are used in a row from DD-1 without a gap).' };
  const ui2 = await boots({}, PREVIEW, gap);
  const main2 = await openBases(ui2);
  assert.ok(main2.querySelector('#mrBoxes').textContent.includes('Ignored: DD-4') && main2.querySelector('#mrBoxes').textContent.includes('DD-3 is missing'));
  ui2.dom.window.close();
});

test('the card still draws when the editor cannot say which boxes it found', async () => {
  const ui = await boots({}, PREVIEW, { __fail: 'no', __code: 'bad', __status: 500 });
  const main = await openBases(ui);
  assert.ok(main.querySelector('[data-act="reserveMove"]'));
  assert.equal(main.querySelector('#mrBoxes').textContent.trim(), '');
  ui.dom.window.close();
});

test('the question lists what goes where and that only a save backup undoes it; OK writes once with the fingerprint and no names; Cancel writes nothing', async () => {
  const ui = await boots({ '/api/bases/move-reserve': { ok: true, changed: true, stacks: 6, items: 2161, outcome: 'written' } });
  await openBases(ui);
  ui.click('[data-act="reserveMove"]'); await ui.settle();
  assert.deepEqual(ui.fetchedUrls.filter((u) => u.startsWith('/api/bases/move-reserve-preview')), ['/api/bases/move-reserve-preview']);
  const q = ui.doc.querySelector('#ask');
  assert.ok(q.textContent.includes('Move the Reserve into "Dump Box" and "Spice Out"?'), q.textContent);
  const items = [...q.querySelectorAll('li')].map((l) => l.textContent);
  assert.equal(items.length, 5, items.join('|'));
  assert.ok(items[0].includes('6 item stacks (2,161 items)') && items[1].includes('"Dump Box"') && items[2].includes('"Spice Out"') && items[4].includes('no known volume'), items.join('|'));
  assert.ok(q.textContent.includes('It can be undone only by restoring a backup of the save') && q.textContent.includes('A backup of the save is taken first'));
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(ui.posted.filter((x) => x.path === '/api/bases/move-reserve').length, 0, 'Cancel writes nothing');
  ui.click('[data-act="reserveMove"]'); await ui.settle();
  await ui.wait(450); // the gap the question keeps against a double click
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((x) => x.path === '/api/bases/move-reserve').map((x) => x.body), [{ fingerprint: 'fp1' }]);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Moved 6 item stacks (2,161 items) from your Reserve into "Dump Box" and "Spice Out"'), t);
  ui.dom.window.close();
});

test('three boxes are named in the question and in the answer with commas and one "and"', async () => {
  const three = { ...PREVIEW, boxNames: ['DD-1', 'DD-2', 'DD-3'] };
  const ui = await boots({ '/api/bases/move-reserve': { ok: true, changed: true, stacks: 6, items: 2161, outcome: 'written' } }, three);
  await openBases(ui);
  ui.click('[data-act="reserveMove"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('Move the Reserve into "DD-1", "DD-2" and "DD-3"?'), ui.doc.querySelector('#ask').textContent);
  await ui.wait(450);
  ui.click('#aok'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('into "DD-1", "DD-2" and "DD-3".'), ui.doc.querySelector('#toast').textContent);
  ui.dom.window.close();
});

test('a refusal of the preview is the message of the page and no question opens; an empty Reserve is said and writes nothing', async () => {
  const refused = { __fail: 'The Reserve does not fit into the boxes, so nothing was changed. Needed: 9 stacks. Short by 3 stacks that fit in none of the boxes. Free room in the boxes in the game, or add DD-3.', __code: 'no_room', __status: 409 };
  const ui = await boots({}, refused);
  await openBases(ui);
  ui.click('[data-act="reserveMove"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('or add DD-3'));
  assert.ok(!ui.doc.querySelector('#ask').textContent.includes('Move the Reserve'), 'no question');
  assert.equal(ui.posted.length, 0);
  ui.dom.window.close();
  const ui2 = await boots({}, { ok: true, nothingToDo: true, changed: false, stacks: 0, boxNames: ['DD-1'], summary: ['The Reserve is empty. Nothing was moved and nothing was written.'], warnings: [], fingerprint: 'e' });
  await openBases(ui2);
  ui2.click('[data-act="reserveMove"]'); await ui2.settle();
  assert.ok(ui2.doc.querySelector('#toast').textContent.includes('Your Reserve is empty. Nothing was moved.'));
  assert.ok(!ui2.doc.querySelector('#ask').textContent.includes('Move the Reserve'));
  assert.equal(ui2.posted.length, 0);
  ui2.dom.window.close();
});

test('the Move the Reserve question says it is not yet tried in the game (#1238)', async () => {
  const ui = await boots();
  await openBases(ui);
  ui.click('[data-act="reserveMove"]'); await ui.settle();
  const q = ui.doc.querySelector('#ask').textContent;
  assert.ok(q.includes('Not yet tried in the game: keep your save backup.'), q);
});
