// #1061 (leftovers) and #1063: one name for the Landsraad section and the two views, and no internal product name over the page. The page heading of the Houses is "Houses" in the Classic list and in the
// Command Center (not "Task board" and not "Command Center board"); the View switch of the Landsraad and of the Specialization page offers "Classic list" and "Command Center"; the Command Center's
// button is "Add XP" as the Classic list's is (not "Apply XP"); the reset-all question of the Classic list says specializations, with its typed sentence; and the hint of a specialization the editor cannot
// match carries no "number" or "track". Real app.js and lcc scripts over the synthetic answers.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { read } from './lcc-load.mjs';

afterEach(closeAll);
const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const SPECS = (id) => JSON.parse(JSON.stringify(CASES.find((c) => c.id === id).response));
const text = (e) => (e ? e.textContent.replace(/\s+/g, ' ').trim() : '');

async function openLandsraad(view) {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': 'grid' } });
  await ui.settle();
  return ui;
}

for (const view of ['board', 'classic']) {
  test('Landsraad, ' + view + ' view: the page heading of the Houses is "Houses", and no internal product name is a heading', async () => {
    const ui = await openLandsraad(view);
    assert.match(text(ui.doc.getElementById('lsb-board-h')), /^Houses \d+ houses?$/);
    const main = text(ui.doc.querySelector('#main'));
    assert.doesNotMatch(main, /Task board|Command Center board/);
    const heads = [...ui.doc.querySelectorAll('#main h1, #main h2, #main h3')].map((h) => text(h));
    assert.ok(!heads.some((h) => /board$|^Task|^Command Center board/i.test(h.replace(/\s+\d+ houses?$/, ''))), 'headings: ' + heads);
  });
  test('Landsraad, ' + view + ' view: the View switch offers Classic list and Command Center', async () => {
    const ui = await openLandsraad(view);
    assert.deepEqual([...ui.doc.querySelectorAll('.viewsw button')].map((b) => text(b)), ['Classic list', 'Command Center']);
  });
}

test('Specialization page: the View switch says the same two names', async () => {
  const ui = await boot({ get: { '/api/player/specs': SPECS('mixed-five-tracks'), '/api/landsraad': synthetic() }, local: { tab: 'player', 'spec.view': 'cc' } });
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  assert.deepEqual([...ui.doc.querySelectorAll('.lcc-sp-viewsw button')].map((b) => text(b)), ['Classic list', 'Command Center']);
});

test('Add XP: the Command Center button and the Classic button have one name, and nothing says Apply XP', async () => {
  const cc = await boot({ get: { '/api/player/specs': SPECS('mixed-five-tracks'), '/api/landsraad': synthetic() }, local: { tab: 'player', 'spec.view': 'cc' } });
  cc.click('[data-sub="player:specialization"]'); await cc.settle();
  const ccBtn = cc.doc.querySelector('[data-act="spAddCC"]');
  assert.equal(text(ccBtn), 'Add XP');
  assert.doesNotMatch(text(cc.doc.querySelector('#main')), /Apply XP/);
  closeAll();
  const cl = await boot({ get: { '/api/player/specs': SPECS('mixed-five-tracks'), '/api/landsraad': synthetic() }, local: { tab: 'player', 'spec.view': 'classic' } });
  cl.click('[data-sub="player:specialization"]'); await cl.settle();
  assert.equal(text(cl.doc.querySelector('[data-act="spAdd"]')), 'Add XP');
  assert.doesNotMatch(text(cl.doc.querySelector('#main')), /Apply XP/);
});

test('Classic reset-all and grant-all questions say specializations, the hint of an unmatched one says it plainly', async () => {
  const s = SPECS('unassigned-row-number-9');
  const ui = await boot({ get: { '/api/player/specs': s, '/api/landsraad': synthetic() }, local: { tab: 'player', 'spec.view': 'classic' } });
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  ui.click('[data-act="spResetAll"]'); await ui.settle();
  const q = text(ui.doc.querySelector('#ask'));
  assert.match(q, /Reset the XP of \d+ specializations?\?/);
  assert.match(q, /Type reset \d+ specializations? to continue/);
  assert.doesNotMatch(q, /track/i);
  ui.click('[data-act="askNo"]'); await ui.settle();
  ui.click('[data-act="spKeyAll"]'); await ui.settle();
  assert.doesNotMatch(text(ui.doc.querySelector('#ask')), /track/i);
  assert.match(text(ui.doc.querySelector('#ask')), /Every keystone of every specialization is bought/);
  closeAll();
  const none = SPECS('mixed-five-tracks'); for (const r of none.rows) r.number = null; none.unassigned = [{ number: 7, xp: 900, level: 4 }]; none.resetCount = 0;
  const u2 = await boot({ get: { '/api/player/specs': none, '/api/landsraad': synthetic() }, local: { tab: 'player', 'spec.view': 'classic' } });
  u2.click('[data-sub="player:specialization"]'); await u2.settle();
  const hint = text(u2.doc.querySelector('#spnumCombat'));
  assert.equal(hint, 'The editor cannot match this specialization yet: see Advanced below.');
  assert.doesNotMatch(hint, /number|track/i);
});
