// Move the Reserve into storage on Player > Bases (#1191): a button and two box-name fields on the Bases card, a question that lists what goes where and says it can only be undone by
// restoring a backup of the save, a write that carries the names and the fingerprint of what was shown, and names that are kept in this browser.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

test.afterEach(() => closeAll());

const HOME = { base_id: 7, name: 'Home', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 10, placeables: 2, x: 0, y: 0, z: 0 };
const PREVIEW = { ok: true, dryRun: true, changed: true, nothingToDo: false, stacks: 6, items: 2161, fingerprint: 'fp1', warnings: ['120 units in the Reserve have no known volume and were counted as none in the volume check.'],
  summary: ['Would move: 6 item stacks (2,161 items) from the Reserve into storage, whole and unchanged. Only the place of each stack changes.',
    'Box "Dump Box" (in "Home"): 6 stacks (2,161 items) would go in, 348 stacks are already there; afterwards there would be 646 slots free, volume 28608.2 free.',
    'Box "Spice Out" (in "Home"): 0 stacks (0 items) would go in, 421 stacks are already there; afterwards there would be 579 slots free, volume 12216.5 free.',
    'The Reserve would be empty afterwards.'] };
const boots = (post = {}, preview = PREVIEW, local = {}) => boot({ get: { '/api/bases/list': [HOME], '/api/bases/backups': [], '/api/bases/move-reserve-preview': preview }, post, local });
const openBases = async (ui) => {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  return ui.doc.querySelector('#main');
};
const type = (ui, id, v) => { const e = ui.doc.querySelector(id); e.value = v; e.dispatchEvent(new ui.w.Event('input', { bubbles: true })); };

test('the Bases card has the button and two box-name fields that offer DD-1 and DD-2, with an explanation that says it is not tried in the game', async () => {
  const ui = await boots();
  const main = await openBases(ui);
  const b = main.querySelector('[data-act="reserveMove"]');
  assert.ok(b && b.textContent.trim() === 'Move the Reserve into storage');
  const tipBox = main.querySelector('#' + b.getAttribute('aria-describedby'));
  assert.ok(tipBox && tipBox.getAttribute('role') === 'tooltip' && tipBox.textContent.includes('Developer storage boxes') && tipBox.textContent.includes('Not yet tried in the game'), tipBox && tipBox.textContent);
  assert.deepEqual(['#mrBox1', '#mrBox2'].map((s) => main.querySelector(s).value), ['DD-1', 'DD-2']);
  assert.ok(tipBox.textContent.includes('DD-1 and DD-2') && tipBox.textContent.includes('capitals'), tipBox.textContent);
  assert.equal(ui.posted.length, 0, 'drawing the card writes nothing');
  ui.dom.window.close();
});

test('without both names nothing is asked of the server', async () => {
  const ui = await boots();
  await openBases(ui);
  type(ui, '#mrBox1', 'Dump Box');
  type(ui, '#mrBox2', '  ');
  ui.click('[data-act="reserveMove"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Type the names of the two storage boxes'));
  assert.equal(ui.fetchedUrls.filter((u) => u.startsWith('/api/bases/move-reserve')).length, 0);
  ui.dom.window.close();
});

test('the question lists what goes where and that only a save backup undoes it; OK writes once with the names and the fingerprint; Cancel writes nothing', async () => {
  const ui = await boots({ '/api/bases/move-reserve': { ok: true, changed: true, stacks: 6, items: 2161, outcome: 'written' } });
  await openBases(ui);
  type(ui, '#mrBox1', 'Dump Box');
  type(ui, '#mrBox2', 'Spice Out');
  ui.click('[data-act="reserveMove"]'); await ui.settle();
  assert.deepEqual(ui.fetchedUrls.filter((u) => u.startsWith('/api/bases/move-reserve-preview')), ['/api/bases/move-reserve-preview?box1=Dump%20Box&box2=Spice%20Out']);
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
  assert.deepEqual(ui.posted.filter((x) => x.path === '/api/bases/move-reserve').map((x) => x.body), [{ box1: 'Dump Box', box2: 'Spice Out', fingerprint: 'fp1' }]);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Moved 6 item stacks (2,161 items) from your Reserve into "Dump Box" and "Spice Out"'), t);
  ui.dom.window.close();
});

test('the names are kept in this browser and come back', async () => {
  const ui = await boots();
  await openBases(ui);
  type(ui, '#mrBox1', 'Dump Box');
  type(ui, '#mrBox2', 'Spice Out');
  assert.equal(ui.w.localStorage.getItem('moveReserveBox1'), 'Dump Box');
  assert.equal(ui.w.localStorage.getItem('moveReserveBox2'), 'Spice Out');
  ui.dom.window.close();
  const ui2 = await boots({}, PREVIEW, { moveReserveBox1: 'Dump Box', moveReserveBox2: 'Spice Out' });
  const main = await openBases(ui2);
  assert.deepEqual(['#mrBox1', '#mrBox2'].map((s) => main.querySelector(s).value), ['Dump Box', 'Spice Out']);
  ui2.dom.window.close();
});

test('a refusal of the preview is the message of the page and no question opens; an empty Reserve is said and writes nothing', async () => {
  const refused = { __fail: 'No Developer storage container in this save is named "Nowhere". Nothing was changed.', __code: 'not_found', __status: 404 };
  const ui = await boots({}, refused);
  await openBases(ui);
  type(ui, '#mrBox1', 'Dump Box');
  type(ui, '#mrBox2', 'Nowhere');
  ui.click('[data-act="reserveMove"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('named "Nowhere"'));
  assert.ok(!ui.doc.querySelector('#ask').textContent.includes('Move the Reserve'), 'no question');
  assert.equal(ui.posted.length, 0);
  ui.dom.window.close();
  const ui2 = await boots({}, { ok: true, nothingToDo: true, changed: false, stacks: 0, summary: ['The Reserve is empty. Nothing was moved and nothing was written.'], warnings: [], fingerprint: 'e' });
  await openBases(ui2);
  type(ui2, '#mrBox1', 'A');
  type(ui2, '#mrBox2', 'B');
  ui2.click('[data-act="reserveMove"]'); await ui2.settle();
  assert.ok(ui2.doc.querySelector('#toast').textContent.includes('Your Reserve is empty. Nothing was moved.'));
  assert.ok(!ui2.doc.querySelector('#ask').textContent.includes('Move the Reserve'));
  assert.equal(ui2.posted.length, 0);
  ui2.dom.window.close();
});
