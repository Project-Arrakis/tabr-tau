// The text sweep (#426) on the Saved map and the Landsraad. See uitext.test.mjs for the checks that hold on every page.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { visibleText, movedText } from './uitext.mjs';
import { synthetic } from './lcc-fixture.mjs';
const text = (e) => e.textContent.replace(/\s+/g, ' ').trim();

test.afterEach(() => closeAll());

const here = path.dirname(fileURLToPath(import.meta.url));
const FIX = JSON.parse(fs.readFileSync(path.join(process.env.UI_FIXTURES_DIR || path.join(here, 'fixtures'), 'ui-fixtures.json'), 'utf8'));
const go = async (ui, tab) => { ui.click(`[data-tab="${tab}"]`); await ui.settle(); return ui.doc.querySelector('#main'); };

test('Landsraad: the explanations of the board are tooltips of its keys, not text on the page, and the short lines and the buttons stay in view', async () => {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, local: { tab: 'landsraad' } });
  await ui.settle();
  const main = ui.doc.querySelector('#main');
  const shown = visibleText(main);
  // the Classic list's About this disclosures went with it (#1028): the board has none open or closed, its long explanations are the tooltips of its keys and the Help tab
  assert.equal(main.querySelectorAll('details.about').length, 0, 'no About this on the board');
  const tip = (fk) => ui.doc.querySelector(`[data-fk="${fk}"] .lcc-k-tip`);
  // the long sentences of the keys are in their tooltips (aria-hidden: the key's own aria-label is what a screen reader says)
  for (const [fk, s] of [['b-fill', 'The question shows the totals before anything is written.'], ['b-complete', 'Marks every chosen House complete for the faction chosen'], ['b-reopen', 'Progress and contributions stay as they are'], ['b-all', 'Chooses the Houses that are shown now.'], ['b-none', 'Clears the choice']]) {
    const t = tip(fk);
    assert.ok(t && t.textContent.includes(s), fk + ' has its explanation as a tooltip: ' + s);
    assert.equal(t.getAttribute('aria-hidden'), 'true', fk + ': the tooltip is not read twice');
  }
  // the long sentences of the Classic list are not on the page
  for (const s of ['Each click is written at once', 'Every row starts collapsed']) assert.ok(!shown.includes(s) && !text(main).includes(s), 'still shown: ' + s);
  // the buttons and their words stay in view
  assert.equal(text(ui.doc.querySelector('[data-fk="b-fill"] .lcc-k-bl')), 'Fill');
  assert.equal(text(ui.doc.querySelector('[data-fk="b-complete"] .lcc-k-bl')), 'Complete');
  assert.equal(text(ui.doc.querySelector('[data-fk="b-reopen"] .lcc-k-bl')), 'Reopen');
  assert.equal(text(ui.doc.querySelector('[data-fk="b-all"]')).startsWith('Choose shown'), true);
  assert.ok(ui.doc.querySelector('[aria-label="Faction for Fill and Complete"]'), 'the faction for Fill and Complete');
  assert.ok(ui.doc.querySelector('[data-fk="b-none"]'), 'Clear');
  ui.dom.window.close();
});

test('Saved map: the first sentence stays, the hint and the reasons are tooltips or in About this, the readout line is empty until a marker is hovered', async () => {
  const lm = { ...FIX['/api/livemap'].body, storedVehicles: 2 };
  const ui = await boot({ get: { '/api/livemap': lm } });
  const main = await go(ui, 'livemap');
  const shown = visibleText(main), moved = movedText(main);
  assert.ok(shown.includes('The save as of its last write, not the running game.'));
  assert.ok(shown.includes('2 of your vehicles are stored (in the vehicle backup or waiting for recovery) and not shown.'));
  for (const s of ['Markers you have not discovered yet are shown faded', 'Hover a marker for details. Drag to move, scroll or press + and - to zoom, 0 to fit.', 'the game keeps only the place a stored vehicle was last in the world'.replace('the game', 'The game')]) {
    assert.ok(!shown.includes(s), 'still shown: ' + s);
    assert.ok(moved.includes(s), 'lost, not moved: ' + s);
  }
  assert.equal(ui.doc.querySelector('#lminfo').textContent, '', 'nothing hovered, nothing said');
  assert.equal(ui.doc.querySelector('[data-lmund]').getAttribute('aria-describedby'), 'lmundtip');
  assert.ok(ui.doc.querySelector('[data-lmund]').closest('.tip'));
  ui.dom.window.close();
});

test('Saved map: the sand-field caveat is a tooltip on the Spice and Flour layer buttons, and the buttons still toggle', async () => {
  const base = FIX['/api/livemap'].body;
  const lm = { ...base, resourceFields: [{ kind: 'spice', size: 'Large', x: 1, y: 1, left: 1 }, ...(base.resourceFields || [])] };
  const ui = await boot({ get: { '/api/livemap': lm } });
  const main = await go(ui, 'livemap');
  const btns = [...ui.doc.querySelectorAll('.lm-layer')].filter((b) => /spice|flour/i.test(b.dataset.lmg));
  for (const b of btns) {
    assert.ok(b.closest('.tip'), b.dataset.lmg + ' has the caveat as a tooltip');
    assert.ok(ui.doc.getElementById(b.getAttribute('aria-describedby')).textContent.includes('have not been checked against the game for this map'));
    const was = b.getAttribute('aria-pressed');
    ui.click(`[data-lmg="${b.dataset.lmg}"]`);
    assert.notEqual(b.getAttribute('aria-pressed'), was, 'it still toggles');
  }
  assert.ok(!visibleText(main).includes('placed by decoding their ids'));
  ui.dom.window.close();
});
