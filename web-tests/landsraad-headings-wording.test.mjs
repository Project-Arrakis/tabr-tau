// #1061 (leftovers) and #1063: one name for the Landsraad section, and no internal product name over the page. The page heading of the Houses is "Houses" (not "Task board" and not "Command Center board");
// the View switch of the Specialization page offers "Classic list" and "Command Center" (the Landsraad has no View switch since #1028: the board is its only view); the Command Center's
// button is "Add XP" as the Classic list's is (not "Apply XP"); the reset-all question of the Classic Specialization list says specializations, with its typed sentence; and the hint of a specialization the editor cannot
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

async function openLandsraad() {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, local: { tab: 'landsraad', 'lsb.layout': 'grid' } });
  await ui.settle();
  return ui;
}

test('Landsraad: the page heading of the Houses is "Houses", and no internal product name is a heading', async () => {
  const ui = await openLandsraad();
  assert.match(text(ui.doc.getElementById('lsb-board-h')), /^Houses \d+ houses?$/);
  const main = text(ui.doc.querySelector('#main'));
  assert.doesNotMatch(main, /Task board|Command Center board/);
  const heads = [...ui.doc.querySelectorAll('#main h1, #main h2, #main h3')].map((h) => text(h));
  assert.ok(!heads.some((h) => /board$|^Task|^Command Center board/i.test(h.replace(/\s+\d+ houses?$/, ''))), 'headings: ' + heads);
});

test('Landsraad: there is no View switch, the board is the only view of the Houses', async () => {
  const ui = await openLandsraad();
  assert.equal(ui.doc.querySelector('#main .viewsw'), null);
  assert.equal(ui.doc.querySelector('#main [data-lb="view"]'), null);
  assert.doesNotMatch(text(ui.doc.querySelector('#main')), /Classic list/);
});

test('Specialization page: there is no View switch, the Command Center view is the only view', async () => {
  const ui = await boot({ get: { '/api/player/specs': SPECS('mixed-five-tracks'), '/api/landsraad': synthetic() }, local: { tab: 'player' } });
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('.lcc-sp-viewsw'), null);
  assert.equal(ui.doc.querySelector('[data-spview]'), null);
  assert.doesNotMatch(text(ui.doc.querySelector('#main')), /Classic list/);
});

test('Add XP: the Command Center button has one name, and nothing says Apply XP', async () => {
  const cc = await boot({ get: { '/api/player/specs': SPECS('mixed-five-tracks'), '/api/landsraad': synthetic() }, local: { tab: 'player' } });
  cc.click('[data-sub="player:specialization"]'); await cc.settle();
  assert.equal(text(cc.doc.querySelector('[data-act="spAddCC"]')), 'Add XP');
  assert.doesNotMatch(text(cc.doc.querySelector('#main')), /Apply XP/);
});

test('reset-all and grant-all questions say specializations', async () => {
  const s = SPECS('unassigned-row-number-9');
  const ui = await boot({ get: { '/api/player/specs': s, '/api/landsraad': synthetic() }, local: { tab: 'player' } });
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
});
