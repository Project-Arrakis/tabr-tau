// The limit that keeps the text sweep (#426) from wearing off: no page may show more than PAGE_WORDS words of text to read before any "About this" is opened or any control is hovered, and no single
// block of it may run past BLOCK_WORDS words. A longer explanation has a place to go: a tooltip on its control (tip()) or the card's closed "About this" (about()). Tables, choices, buttons, labels,
// headings and the summary tiles are the page's data and its way to act and are not counted (see proseWords in uitext.mjs). The pages are drawn from the same generated fixtures as the other page tests, so
// the numbers are the same on every machine; the figures of the sweep itself are in docs/design/2026-10-26-ui-text-sweep.md ("Words on each page, before and after").
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { proseWords, longestBlock, eachPage } from './uitext.mjs';

test.afterEach(() => closeAll());

// The most words of reading on one page (the worst page today shows about 86: the specialization page of a save whose tracks are not known, every line of which says why a control is off), and the longest single block (42 there).
const PAGE_WORDS = 100;
const BLOCK_WORDS = 45;

const KITS = { kits: [{ name: 'Starter', items: [{ item: 'SpicedFuelCell', quantity: 100 }], xp: 5000, skillPoints: 3, problem: '' }], limits: { kits: 20, items: 50, name: 60, quantity: 1000000, xp: 100000000, skillPoints: 100000, fileBytes: 65536 }, problem: '' };

test(`no page shows more than ${PAGE_WORDS} words to read, or a block of more than ${BLOCK_WORDS}, before an About this is opened`, async () => {
  const ui = await boot({ get: { '/api/kits': KITS } });
  const seen = [];
  await eachPage(ui, (label) => {
    const main = ui.doc.querySelector('#main');
    const words = proseWords(main), block = longestBlock(main);
    seen.push(label);
    assert.ok(words <= PAGE_WORDS, `${label} shows ${words} words to read (limit ${PAGE_WORDS}): move the explanation into a tooltip or the card's About this`);
    assert.ok(block <= BLOCK_WORDS, `${label} has a block of ${block} words (limit ${BLOCK_WORDS}): move the explanation into a tooltip or the card's About this`);
    // nothing counted lives in a disclosure that is open on a fresh load
    for (const d of ui.doc.querySelectorAll('details.about')) assert.equal(d.open, false, `${label}: ${d.dataset.about} is open on a fresh load`);
  });
  for (const l of ['player:character', 'char:carepackage', 'player:specialization', 'bases:power', 'player:vehicles', 'player:admin', 'landsraad', 'livemap', 'config', 'db:sql']) assert.ok(seen.includes(l), 'page not measured: ' + l);
  ui.dom.window.close();
});

// The pages above are drawn in their default state. A note that is drawn only while it applies (#426) must fit the same limits in the state where it IS drawn, or it could grow without the test noticing.
const BASE7 = [{ base_id: 7, name: 'Base', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 0, placeables: 0, x: 0, y: 0, z: 0 }];
const CLAIM = [{ totem_id: 7, name: 'Totem', cells: 2, rings: 0, level: 1, irregular: true, maxRings: 5, maxLevel: 5, yaw: 90, segments: [{ x: 1, y: 0 }], grid: [{ x: 0, y: 0, in: true, n: 0 }, { x: 1, y: 0, in: true, n: 3 }], piecesInClaim: 3, piecesOutside: 0 }];
const within = (ui, label) => {
  const main = ui.doc.querySelector('#main');
  const words = proseWords(main), block = longestBlock(main);
  assert.ok(words <= PAGE_WORDS, `${label} shows ${words} words to read (limit ${PAGE_WORDS})`);
  assert.ok(block <= BLOCK_WORDS, `${label} has a block of ${block} words (limit ${BLOCK_WORDS})`);
};
const goPlayer = async (ui, sub) => { ui.click('[data-tab="player"]'); await ui.settle(); ui.click(`[data-sub="player:${sub}"]`); await ui.settle(); };

test('a note drawn only while it applies stays inside the limits in that state', async () => {
  // Admin, another map chosen: the move-between-maps warning appears
  let ui = await boot({});
  await goPlayer(ui, 'admin');
  const sel = ui.doc.querySelector('#tmap');
  sel.value = 'DeepDesert'; sel.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.ok(ui.doc.querySelector('#tmapwarn'), 'the warning is drawn');
  within(ui, 'Admin with another map chosen');
  ui.dom.window.close();

  // Land claim editor, a cell chosen: the staking-check warning appears
  ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': CLAIM } });
  await goPlayer(ui, 'bases');
  ui.click('[data-bstab="claim"]'); await ui.settle();
  ui.doc.querySelector('[data-cell="2,0"]').dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })); await ui.settle();
  assert.ok(ui.doc.querySelector('#lcbypass'), 'the warning is drawn');
  within(ui, 'Land claim editor with a cell chosen');
  ui.dom.window.close();

  // Care package, the list full and a settings problem, and the kit editor open
  ui = await boot({ get: { '/api/kits': { ...KITS, limits: { ...KITS.limits, kits: 1 }, problem: 'The settings file could not be read.' } } });
  await goPlayer(ui, 'character');
  ui.click('[data-sub="char:carepackage"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#kitfull'), 'the full-list line is drawn');
  within(ui, 'Care package with the list full');
  ui.dom.window.close();
});

test('opening an About this shows more words, and the words are the same ones that were hidden (the counter sees disclosures)', async () => {
  const ui = await boot({});
  ui.click('[data-tab="player"]'); await ui.settle();
  const main = () => ui.doc.querySelector('#main');
  const before = proseWords(main());
  const d = ui.doc.querySelector('details.about');
  assert.ok(d, 'the Overview has an About this');
  d.open = true;
  assert.ok(proseWords(main()) > before, 'an open About this counts');
  ui.dom.window.close();
});

test('the page the browser loads first has almost no text of its own: the rest is drawn by app.js and counted above', () => {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const html = fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', 'index.html'), 'utf8');
  const text = html.replace(/<script[\s\S]*?<\/script>|<style[\s\S]*?<\/style>|<svg[\s\S]*?<\/svg>|<head[\s\S]*?<\/head>/g, ' ').replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
  assert.ok(text.split(' ').length <= 15, `index.html carries ${text.split(' ').length} words of its own: ${text}`);
});
